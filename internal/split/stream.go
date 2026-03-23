package split

import (
	"bufio"
	"context"
	"errors"
	"io"

	"github.com/fulmenhq/dimlox/internal/provider"
	"github.com/fulmenhq/dimlox/internal/uri"
)

const splitReadBufferSize = 4 * 1024 * 1024

func Stream(ctx context.Context, rawURI string, src provider.StorageProvider, parsed *uri.ParsedURI, meta *provider.ObjectMeta, outDir string, opts Options) (*Result, error) {
	delimiter, encoding, detected, err := resolveDetection(ctx, rawURI, opts)
	if err != nil {
		return nil, err
	}
	r, compressed, err := openStream(ctx, src, rawURI, meta)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = r.Close()
	}()
	compressOut, err := resolveOutputCompression(compressed, opts.OutFmt)
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
			_ = manifest.Abort()
		}
	}()

	result := &Result{
		SourceURI:    rawURI,
		Mode:         ModeStream,
		OutDir:       outDir,
		ManifestPath: manifest.Path(),
		DryRun:       opts.DryRun,
		Delimiter:    delimiter,
		Encoding:     encoding,
		HeaderCopied: opts.Header,
		Detected:     detected,
	}
	if opts.DryRun && compressOut {
		result.Notes = append(result.Notes, compressedDryRunBytesNote)
	}

	br := bufio.NewReaderSize(r, splitReadBufferSize)
	var headerLine []byte
	var shard *shardWriter
	defer func() {
		if shard != nil {
			_ = shard.Abort()
		}
	}()
	var dry dryShard
	index := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line, readErr := readLine(br)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return nil, readErr
		}
		if len(line) == 0 && errors.Is(readErr, io.EOF) {
			break
		}
		if headerLine == nil && opts.Header {
			headerLine = append([]byte(nil), line...)
			if errors.Is(readErr, io.EOF) {
				break
			}
			continue
		}
		needShard := false
		if opts.DryRun {
			needShard = dry.empty()
		} else {
			needShard = shard == nil
		}
		if needShard {
			index++
			path := textShardPath(outDir, baseName, index, compressOut)
			if opts.DryRun {
				dry = dryShard{path: path}
				if len(headerLine) > 0 {
					dry.bytes += int64(len(headerLine))
				}
			} else {
				shard, err = newShardWriter(path, compressOut)
				if err != nil {
					return nil, err
				}
				if len(headerLine) > 0 {
					if err := shard.WriteHeader(headerLine); err != nil {
						return nil, err
					}
				}
			}
		}

		if opts.DryRun {
			dry.rows++
			dry.bytes += int64(len(line))
		} else {
			if err := shard.WriteRow(line); err != nil {
				return nil, err
			}
		}

		if shouldRotateStreamShard(opts, shard, dry) {
			entry, err := closeTextShard(rawURI, meta, manifest, opts, delimiter, encoding, ModeStream, index, compressOut, shard, dry)
			if err != nil {
				return nil, err
			}
			result.Shards = append(result.Shards, entry)
			shard = nil
			dry = dryShard{}
		}

		if errors.Is(readErr, io.EOF) {
			break
		}
	}

	if opts.DryRun {
		if !dry.empty() {
			entry, err := closeTextShard(rawURI, meta, manifest, opts, delimiter, encoding, ModeStream, index, compressOut, nil, dry)
			if err != nil {
				return nil, err
			}
			result.Shards = append(result.Shards, entry)
		}
	} else if shard != nil {
		entry, err := closeTextShard(rawURI, meta, manifest, opts, delimiter, encoding, ModeStream, index, compressOut, shard, dryShard{})
		if err != nil {
			return nil, err
		}
		result.Shards = append(result.Shards, entry)
	}

	if manifest != nil {
		if err := manifest.Close(); err != nil {
			return nil, err
		}
		manifest = nil
	}
	return result, nil
}

type dryShard struct {
	path  string
	rows  int64
	bytes int64
}

func (d dryShard) empty() bool {
	return d.path == ""
}

func shouldRotateStreamShard(opts Options, shard *shardWriter, dry dryShard) bool {
	rows := dry.rows
	bytes := dry.bytes
	if shard != nil {
		rows = shard.rows
		bytes = shard.writer.n
	}
	if opts.Rows > 0 && rows >= opts.Rows {
		return true
	}
	if opts.Bytes > 0 && bytes >= opts.Bytes {
		return true
	}
	return false
}

func closeTextShard(rawURI string, meta *provider.ObjectMeta, manifest *manifestWriter, opts Options, delimiter, encoding string, mode Mode, index int, compressOut bool, shard *shardWriter, dry dryShard) (ManifestEntry, error) {
	if opts.DryRun {
		stats := shardStats{rows: dry.rows}
		if compressOut {
			stats.logicalBytes = dry.bytes
			stats.bytesNote = compressedDryRunBytesNote
		} else {
			stats.bytes = dry.bytes
		}
		entry := buildManifestEntry(rawURI, meta, opts.OutDir, dry.path, index, stats, mode, delimiter, encoding, opts.Header)
		if err := manifest.Write(entry); err != nil {
			return ManifestEntry{}, err
		}
		return entry, nil
	}
	stats, err := shard.Close()
	if err != nil {
		return ManifestEntry{}, err
	}
	finalPath := shard.finalPath
	entry := buildManifestEntry(rawURI, meta, opts.OutDir, finalPath, index, stats, mode, delimiter, encoding, opts.Header)
	if err := manifest.Write(entry); err != nil {
		return ManifestEntry{}, err
	}
	return entry, nil
}

func readLine(br *bufio.Reader) ([]byte, error) {
	var line []byte
	for {
		chunk, err := br.ReadBytes('\n')
		line = append(line, chunk...)
		switch {
		case err == nil:
			return line, nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF):
			if len(line) == 0 {
				return nil, io.EOF
			}
			return line, io.EOF
		default:
			return nil, err
		}
	}
}
