package split

import (
	"context"
	"errors"
	"io"

	"github.com/fulmenhq/dimlox/internal/provider"
	"github.com/fulmenhq/dimlox/internal/uri"
)

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
			_ = manifest.Close()
		}
	}()

	result := &Result{SourceURI: rawURI, Mode: ModeBinary, OutDir: outDir, ManifestPath: manifest.Path(), DryRun: opts.DryRun}
	buf := make([]byte, opts.Bytes)
	index := 0
	for {
		n, readErr := io.ReadFull(r, buf)
		if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
			return nil, readErr
		}
		if n == 0 {
			break
		}
		index++
		path := binaryShardPath(outDir, baseName, index)
		var entry ManifestEntry
		if opts.DryRun {
			entry = buildManifestEntry(rawURI, meta, path, index, shardStats{bytes: int64(n)}, ModeBinary, "", "", false)
		} else {
			shard, err := newShardWriter(path, false)
			if err != nil {
				return nil, err
			}
			if err := shard.write(buf[:n]); err != nil {
				return nil, err
			}
			stats, err := shard.Close()
			if err != nil {
				return nil, err
			}
			entry = buildManifestEntry(rawURI, meta, path, index, stats, ModeBinary, "", "", false)
		}
		if err := manifest.Write(entry); err != nil {
			return nil, err
		}
		result.Shards = append(result.Shards, entry)
		if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
			break
		}
	}
	if manifest != nil {
		if err := manifest.Close(); err != nil {
			return nil, err
		}
	}
	return result, nil
}
