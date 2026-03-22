package split

import (
	"context"
	"errors"
	"io"

	"github.com/fulmenhq/dimlox/internal/provider"
	"github.com/fulmenhq/dimlox/internal/uri"
)

var binaryCopyBufferSize int64 = 4 * 1024 * 1024

func Binary(ctx context.Context, rawURI string, src provider.StorageProvider, parsed *uri.ParsedURI, meta *provider.ObjectMeta, outDir string, opts Options) (*Result, error) {
	r, err := openRawStream(ctx, src, rawURI)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = r.Close()
	}()
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

	result := &Result{SourceURI: rawURI, Mode: ModeBinary, OutDir: outDir, ManifestPath: manifest.Path(), DryRun: opts.DryRun}
	buf := make([]byte, binaryReadBufferSize(opts.Bytes))
	index := 0
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		nextIndex := index + 1
		path := binaryShardPath(outDir, baseName, nextIndex)
		var entry ManifestEntry
		var stats shardStats
		reachedEOF := false
		if opts.DryRun {
			stats, reachedEOF, err = readBinaryShard(ctx, r, opts.Bytes, buf, nil)
			if err != nil {
				return nil, err
			}
			if stats.bytes == 0 {
				break
			}
			index = nextIndex
			entry = buildManifestEntry(rawURI, meta, outDir, path, index, stats, ModeBinary, "", "", false)
		} else {
			shard, err := newShardWriter(path, false)
			if err != nil {
				return nil, err
			}
			stats, reachedEOF, err = readBinaryShard(ctx, r, opts.Bytes, buf, shard)
			if err != nil {
				_ = shard.Abort()
				return nil, err
			}
			if stats.bytes == 0 {
				_ = shard.Abort()
				break
			}
			index = nextIndex
			stats, err = shard.Close()
			if err != nil {
				return nil, err
			}
			entry = buildManifestEntry(rawURI, meta, outDir, path, index, stats, ModeBinary, "", "", false)
		}
		if err := manifest.Write(entry); err != nil {
			return nil, err
		}
		result.Shards = append(result.Shards, entry)
		if reachedEOF {
			break
		}
	}
	if manifest != nil {
		if err := manifest.Close(); err != nil {
			return nil, err
		}
		manifest = nil
	}
	return result, nil
}

func readBinaryShard(ctx context.Context, r io.Reader, shardBytes int64, buf []byte, shard *shardWriter) (shardStats, bool, error) {
	remaining := shardBytes
	var stats shardStats
	for remaining > 0 {
		if err := ctx.Err(); err != nil {
			return shardStats{}, false, err
		}
		chunk := buf
		if int64(len(chunk)) > remaining {
			chunk = chunk[:remaining]
		}
		n, err := r.Read(chunk)
		if n > 0 {
			piece := chunk[:n]
			if shard != nil {
				if _, writeErr := shard.Write(piece); writeErr != nil {
					return shardStats{}, false, writeErr
				}
			}
			stats.bytes += int64(n)
			remaining -= int64(n)
		}
		switch {
		case err == nil:
			continue
		case errors.Is(err, io.EOF):
			return stats, true, nil
		default:
			return shardStats{}, false, err
		}
	}
	return stats, false, nil
}

func binaryReadBufferSize(shardBytes int64) int {
	size := binaryCopyBufferSize
	if shardBytes > 0 && shardBytes < size {
		size = shardBytes
	}
	if size <= 0 {
		size = 64 * 1024
	}
	return int(size)
}
