package inspect

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fulmenhq/dimlox/internal/provider"
	"github.com/fulmenhq/dimlox/internal/uri"
)

const sampleChunkSize int64 = 4 * 1024 * 1024

type SampleMode string

const (
	SampleHead SampleMode = "head"
	SampleMid  SampleMode = "mid"
	SampleTail SampleMode = "tail"
)

type SampleResult struct {
	URI        string     `json:"uri"`
	Mode       SampleMode `json:"mode"`
	Count      int        `json:"count"`
	Lines      []string   `json:"lines"`
	Compressed bool       `json:"compressed"`
	Strategy   string     `json:"strategy"`
}

func Head(ctx context.Context, rawURI string, count int, opts ProviderOptions) (*SampleResult, error) {
	if count <= 0 {
		return nil, fmt.Errorf("head count must be > 0")
	}
	src, _, err := providerForURI(ctx, rawURI, opts)
	if err != nil {
		return nil, err
	}
	meta, err := src.Stat(ctx, rawURI)
	if err != nil {
		return nil, err
	}
	r, compressed, err := openStream(ctx, src, rawURI, meta)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	lines, err := readFirstNLines(r, count)
	if err != nil {
		return nil, err
	}
	return &SampleResult{URI: rawURI, Mode: SampleHead, Count: count, Lines: lines, Compressed: compressed, Strategy: "forward-stream"}, nil
}

func Tail(ctx context.Context, rawURI string, count int, opts ProviderOptions) (*SampleResult, error) {
	if count <= 0 {
		return nil, fmt.Errorf("tail count must be > 0")
	}
	src, parsed, err := providerForURI(ctx, rawURI, opts)
	if err != nil {
		return nil, err
	}
	meta, err := src.Stat(ctx, rawURI)
	if err != nil {
		return nil, err
	}
	if isCompressed(rawURI, meta) {
		lines, err := tailFromForwardStream(ctx, src, rawURI, meta, count)
		if err != nil {
			return nil, err
		}
		return &SampleResult{URI: rawURI, Mode: SampleTail, Count: count, Lines: lines, Compressed: true, Strategy: "forward-stream-fallback"}, nil
	}
	if parsed != nil && parsed.Provider == uri.ProviderLocal {
		lines, err := tailLocal(parsed.LocalPath, count)
		if err != nil {
			return nil, err
		}
		return &SampleResult{URI: rawURI, Mode: SampleTail, Count: count, Lines: lines, Strategy: "backward-local"}, nil
	}
	lines, err := tailCloud(ctx, src, rawURI, meta.Size, count)
	if err != nil {
		return nil, err
	}
	return &SampleResult{URI: rawURI, Mode: SampleTail, Count: count, Lines: lines, Strategy: "backward-range"}, nil
}

func Mid(ctx context.Context, rawURI string, count int, opts ProviderOptions) (*SampleResult, error) {
	if count <= 0 {
		return nil, fmt.Errorf("mid count must be > 0")
	}
	src, parsed, err := providerForURI(ctx, rawURI, opts)
	if err != nil {
		return nil, err
	}
	meta, err := src.Stat(ctx, rawURI)
	if err != nil {
		return nil, err
	}
	if isCompressed(rawURI, meta) {
		lines, err := midCompressed(ctx, src, rawURI, meta, count)
		if err != nil {
			return nil, err
		}
		return &SampleResult{URI: rawURI, Mode: SampleMid, Count: count, Lines: lines, Compressed: true, Strategy: "forward-stream-fallback"}, nil
	}
	strategy := "midpoint-range"
	if parsed != nil && parsed.Provider == uri.ProviderLocal {
		lines, err := midLocal(parsed.LocalPath, meta.Size/2, count)
		if err != nil {
			return nil, err
		}
		strategy = "midpoint-seek"
		return &SampleResult{URI: rawURI, Mode: SampleMid, Count: count, Lines: lines, Strategy: strategy}, nil
	}
	lines, err := midCloud(ctx, src, rawURI, meta.Size/2, count)
	if err != nil {
		return nil, err
	}
	return &SampleResult{URI: rawURI, Mode: SampleMid, Count: count, Lines: lines, Strategy: strategy}, nil
}

func readFirstNLines(r io.Reader, count int) ([]string, error) {
	br := bufio.NewReaderSize(r, int(sampleChunkSize))
	lines := make([]string, 0, count)
	for len(lines) < count {
		line, err := br.ReadString('\n')
		if err == io.EOF {
			if line != "" {
				lines = append(lines, trimLineEnding(line))
			}
			return lines, nil
		}
		if err != nil {
			return nil, err
		}
		lines = append(lines, trimLineEnding(line))
	}
	return lines, nil
}

func tailFromForwardStream(ctx context.Context, src provider.StorageProvider, rawURI string, meta *provider.ObjectMeta, count int) ([]string, error) {
	r, _, err := openStream(ctx, src, rawURI, meta)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return collectTailLines(r, count)
}

func collectTailLines(r io.Reader, count int) ([]string, error) {
	br := bufio.NewReaderSize(r, int(sampleChunkSize))
	ring := make([]string, 0, count)
	for {
		line, err := br.ReadString('\n')
		if err == io.EOF {
			if line != "" {
				ring = appendRing(ring, trimLineEnding(line), count)
			}
			return ring, nil
		}
		if err != nil {
			return nil, err
		}
		ring = appendRing(ring, trimLineEnding(line), count)
	}
}

func appendRing(lines []string, line string, count int) []string {
	if count <= 0 {
		return lines
	}
	if len(lines) < count {
		return append(lines, line)
	}
	copy(lines, lines[1:])
	lines[count-1] = line
	return lines
}

func tailLocal(path string, count int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	size, err := localFileSize(path)
	if err != nil {
		return nil, err
	}
	start, err := tailStartOffset(func(offset, length int64) ([]byte, error) {
		buf := make([]byte, length)
		if _, err := f.ReadAt(buf, offset); err != nil && err != io.EOF {
			return nil, err
		}
		return buf, nil
	}, size, count)
	if err != nil {
		return nil, err
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	return readLinesFromOffset(f, false, count)
}

func tailCloud(ctx context.Context, src provider.StorageProvider, rawURI string, size int64, count int) ([]string, error) {
	start, err := tailStartOffset(func(offset, length int64) ([]byte, error) {
		reader, err := src.OpenReader(ctx, rawURI, offset, length)
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return io.ReadAll(reader)
	}, size, count)
	if err != nil {
		return nil, err
	}
	reader, err := src.OpenReader(ctx, rawURI, start, -1)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return readLinesFromOffset(reader, false, count)
}

func tailStartOffset(readChunk func(offset, length int64) ([]byte, error), size int64, count int) (int64, error) {
	if size <= 0 {
		return 0, nil
	}
	end := size
	var buffer []byte
	for end > 0 {
		start := end - sampleChunkSize
		if start < 0 {
			start = 0
		}
		chunk, err := readChunk(start, end-start)
		if err != nil {
			return 0, err
		}
		buffer = append(chunk, buffer...)
		if bytes.Count(buffer, []byte{'\n'}) >= count+1 || start == 0 {
			return locateTailStart(start, buffer, count), nil
		}
		end = start
	}
	return 0, nil
}

func locateTailStart(base int64, buffer []byte, count int) int64 {
	if count <= 0 || len(buffer) == 0 {
		return base
	}
	needed := count
	i := len(buffer) - 1
	if i >= 0 && buffer[i] == '\n' {
		i--
	}
	for ; i >= 0; i-- {
		if buffer[i] == '\n' {
			needed--
			if needed == 0 {
				return base + int64(i+1)
			}
		}
	}
	return base
}

func midCompressed(ctx context.Context, src provider.StorageProvider, rawURI string, meta *provider.ObjectMeta, count int) ([]string, error) {
	raw, err := openRawStream(ctx, src, rawURI)
	if err != nil {
		return nil, err
	}
	defer raw.Close()
	var compressedRead int64
	counted := provider.NewCountingReader(raw, func(n int) { compressedRead += int64(n) })
	gz, err := gzip.NewReader(counted)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	br := bufio.NewReaderSize(gz, int(sampleChunkSize))
	target := meta.Size / 2
	for compressedRead < target {
		if _, err := br.ReadString('\n'); err != nil {
			if err == io.EOF {
				return nil, nil
			}
			return nil, err
		}
	}
	return readFirstNLines(br, count)
}

func midFromReader(r io.Reader, count int, aligned bool) ([]string, error) {
	br := bufio.NewReaderSize(r, int(sampleChunkSize))
	if !aligned {
		if _, err := br.ReadString('\n'); err != nil && err != io.EOF {
			return nil, err
		}
	}
	return readFirstNLines(br, count)
}

func midLocal(path string, offset int64, count int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	aligned, err := isLineBoundaryLocal(f, offset)
	if err != nil {
		return nil, err
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	return midFromReader(f, count, aligned)
}

func midCloud(ctx context.Context, src provider.StorageProvider, rawURI string, offset int64, count int) ([]string, error) {
	aligned := true
	readOffset := offset
	length := sampleChunkSize
	if offset > 0 {
		aligned = false
		readOffset = offset - 1
		length++
		probe, err := src.OpenReader(ctx, rawURI, offset-1, 1)
		if err != nil {
			return nil, err
		}
		buf, err := io.ReadAll(probe)
		_ = probe.Close()
		if err != nil {
			return nil, err
		}
		aligned = len(buf) == 1 && buf[0] == '\n'
	}
	r, err := src.OpenReader(ctx, rawURI, readOffset, length)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	if offset > 0 {
		br := bufio.NewReaderSize(r, int(sampleChunkSize))
		if _, err := br.ReadByte(); err != nil {
			return nil, err
		}
		return midFromReader(br, count, aligned)
	}
	return midFromReader(r, count, true)
}

func isLineBoundaryLocal(f *os.File, offset int64) (bool, error) {
	if offset <= 0 {
		return true, nil
	}
	buf := make([]byte, 1)
	if _, err := f.ReadAt(buf, offset-1); err != nil {
		return false, err
	}
	return buf[0] == '\n', nil
}

func readLinesFromOffset(r io.Reader, discardPartial bool, count int) ([]string, error) {
	br := bufio.NewReaderSize(r, int(sampleChunkSize))
	if discardPartial {
		if _, err := br.ReadString('\n'); err != nil && err != io.EOF {
			return nil, err
		}
	}
	return readFirstNLines(br, count)
}

func trimLineEnding(line string) string {
	return strings.TrimRight(line, "\r\n")
}
