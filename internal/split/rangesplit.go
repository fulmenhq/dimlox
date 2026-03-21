package split

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/fulmenhq/dimlox/internal/provider"
	"github.com/fulmenhq/dimlox/internal/uri"
)

var rangeReadBlockSize int64 = 32 * 1024 * 1024

func Range(ctx context.Context, rawURI string, src provider.StorageProvider, parsed *uri.ParsedURI, meta *provider.ObjectMeta, outDir string, opts Options) (*Result, error) {
	if parsed == nil || parsed.Provider == uri.ProviderLocal {
		return nil, fmt.Errorf("split mode %q requires an uncompressed cloud text source", ModeRange)
	}
	if isCompressed(rawURI, meta) {
		return nil, fmt.Errorf("split mode %q does not support compressed sources", ModeRange)
	}
	if !isTextLike(rawURI, meta) {
		return nil, fmt.Errorf("split mode %q requires a text-like source", ModeRange)
	}

	delimiter, encoding, detected, err := resolveDetection(ctx, rawURI, opts)
	if err != nil {
		return nil, err
	}
	compressOut, err := resolveOutputCompression(false, opts.OutFmt)
	if err != nil {
		return nil, err
	}
	baseName := sourceBaseName(rawURI, parsed, meta)
	manifest, err := newManifestWriter(manifestPath(outDir, baseName), opts.Manifest, opts.DryRun)
	if err != nil {
		return nil, err
	}
	defer func() {
		if manifest != nil {
			_ = manifest.Close()
		}
	}()

	result := &Result{
		SourceURI:    rawURI,
		Mode:         ModeRange,
		OutDir:       outDir,
		ManifestPath: manifest.Path(),
		DryRun:       opts.DryRun,
		Delimiter:    delimiter,
		Encoding:     encoding,
		HeaderCopied: opts.Header,
		Detected:     detected,
	}

	headerLine, offset, err := readRangeHeader(ctx, src, rawURI, opts.Header)
	if err != nil {
		return nil, err
	}

	var shard *shardWriter
	var dry dryShard
	index := 0
	startShard := func() error {
		index++
		path := textShardPath(outDir, baseName, index, compressOut)
		if opts.DryRun {
			dry = dryShard{path: path}
			if len(headerLine) > 0 {
				dry.bytes += int64(len(headerLine))
			}
			return nil
		}
		shard, err = newShardWriter(path, compressOut)
		if err != nil {
			return err
		}
		if len(headerLine) > 0 {
			if err := shard.WriteHeader(headerLine); err != nil {
				return err
			}
		}
		return nil
	}
	closeShard := func() error {
		if opts.DryRun {
			if dry.empty() {
				return nil
			}
			entry, err := closeTextShard(rawURI, meta, manifest, opts, delimiter, encoding, ModeRange, index, nil, dry)
			if err != nil {
				return err
			}
			result.Shards = append(result.Shards, entry)
			dry = dryShard{}
			return nil
		}
		if shard == nil {
			return nil
		}
		entry, err := closeTextShard(rawURI, meta, manifest, opts, delimiter, encoding, ModeRange, index, shard, dryShard{})
		if err != nil {
			return err
		}
		result.Shards = append(result.Shards, entry)
		shard = nil
		return nil
	}
	writeLine := func(line []byte) error {
		needShard := false
		if opts.DryRun {
			needShard = dry.empty()
		} else {
			needShard = shard == nil
		}
		if needShard {
			if err := startShard(); err != nil {
				return err
			}
		}
		if opts.DryRun {
			dry.rows++
			dry.bytes += int64(len(line))
		} else {
			if err := shard.WriteRow(line); err != nil {
				return err
			}
		}
		if shouldRotateStreamShard(opts, shard, dry) {
			if err := closeShard(); err != nil {
				return err
			}
		}
		return nil
	}

	var carry []byte
	for offset < meta.Size {
		length := minInt64(rangeReadBlockSize, meta.Size-offset)
		chunk, err := readRangeChunk(ctx, src, rawURI, offset, length)
		if err != nil {
			return nil, err
		}
		if len(chunk) == 0 {
			break
		}
		offset += int64(len(chunk))
		combined := chunk
		if len(carry) > 0 {
			combined = append(carry, chunk...)
		}
		carry, err = processRangeChunk(combined, offset >= meta.Size, writeLine)
		if err != nil {
			return nil, err
		}
	}

	if len(carry) > 0 {
		if err := writeLine(carry); err != nil {
			return nil, err
		}
	}
	if err := closeShard(); err != nil {
		return nil, err
	}
	if manifest != nil {
		if err := manifest.Close(); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func readRangeHeader(ctx context.Context, src provider.StorageProvider, rawURI string, header bool) ([]byte, int64, error) {
	if !header {
		return nil, 0, nil
	}
	r, err := src.OpenReader(ctx, rawURI, 0, -1)
	if err != nil {
		return nil, 0, err
	}
	defer r.Close()
	line, err := readLine(bufio.NewReaderSize(r, splitReadBufferSize))
	if err != nil && err != io.EOF {
		return nil, 0, err
	}
	if len(line) == 0 {
		return nil, 0, nil
	}
	return append([]byte(nil), line...), int64(len(line)), nil
}

func readRangeChunk(ctx context.Context, src provider.StorageProvider, rawURI string, offset, length int64) ([]byte, error) {
	r, err := src.OpenReader(ctx, rawURI, offset, length)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func processRangeChunk(chunk []byte, eof bool, writeLine func([]byte) error) ([]byte, error) {
	start := 0
	for start < len(chunk) {
		idx := bytes.IndexByte(chunk[start:], '\n')
		if idx < 0 {
			break
		}
		end := start + idx + 1
		if err := writeLine(chunk[start:end]); err != nil {
			return nil, err
		}
		start = end
	}
	if start >= len(chunk) {
		return nil, nil
	}
	if eof {
		return nil, writeLine(chunk[start:])
	}
	return append([]byte(nil), chunk[start:]...), nil
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
