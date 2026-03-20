package inspect

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fulmenhq/dimlox/internal/provider"
	"github.com/fulmenhq/dimlox/internal/providers"
	"github.com/fulmenhq/dimlox/internal/uri"
)

type ProviderOptions = providers.Options

var providerResolver = providers.ForURI

func providerForURI(ctx context.Context, rawURI string, opts ProviderOptions) (provider.StorageProvider, *uri.ParsedURI, error) {
	return providerResolver(ctx, rawURI, providers.Options(opts))
}

func openStream(ctx context.Context, src provider.StorageProvider, rawURI string, meta *provider.ObjectMeta) (io.ReadCloser, bool, error) {
	r, err := src.OpenReader(ctx, rawURI, 0, -1)
	if err != nil {
		return nil, false, err
	}
	if !isCompressed(rawURI, meta) {
		return r, false, nil
	}
	gz, err := gzip.NewReader(r)
	if err != nil {
		_ = r.Close()
		return nil, true, err
	}
	return &combinedReadCloser{Reader: gz, closers: []io.Closer{gz, r}}, true, nil
}

func openRawStream(ctx context.Context, src provider.StorageProvider, rawURI string) (io.ReadCloser, error) {
	return src.OpenReader(ctx, rawURI, 0, -1)
}

func isCompressed(rawURI string, meta *provider.ObjectMeta) bool {
	if meta != nil && strings.EqualFold(meta.ContentType, "application/gzip") {
		return true
	}
	parsed, err := uri.Parse(rawURI)
	if err == nil && parsed.Provider == uri.ProviderLocal {
		return strings.EqualFold(filepath.Ext(parsed.LocalPath), ".gz")
	}
	return strings.EqualFold(filepath.Ext(rawURI), ".gz")
}

type combinedReadCloser struct {
	io.Reader
	closers []io.Closer
}

func (c *combinedReadCloser) Close() error {
	var firstErr error
	for _, closer := range c.closers {
		if err := closer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func basename(rawURI string, parsed *uri.ParsedURI) string {
	if parsed == nil {
		return filepath.Base(rawURI)
	}
	switch parsed.Provider {
	case uri.ProviderAZBlob:
		return filepath.Base(parsed.AZBlobPath)
	case uri.ProviderGCS:
		return filepath.Base(parsed.GCSObject)
	case uri.ProviderLocal:
		return filepath.Base(parsed.LocalPath)
	default:
		return filepath.Base(rawURI)
	}
}

func RefuseCompressedCloudSample(ctx context.Context, rawURI string, mode SampleMode, count int, opts ProviderOptions) error {
	src, parsed, err := providerForURI(ctx, rawURI, opts)
	if err != nil {
		return err
	}
	meta, err := src.Stat(ctx, rawURI)
	if err != nil {
		return err
	}
	if !isCompressedCloud(rawURI, parsed, meta) {
		return nil
	}
	localPath := filepath.Join("/tmp", basename(rawURI, parsed))
	return fmt.Errorf("--%s on a compressed cloud source requires decompressing the entire file over the network. This is typically slower than downloading the file first.\n\nRecommended workflow:\n\n  dimlox get %s %s\n  dimlox inspect --%s %d %s\n\nIf local disk space allows, downloading first is faster and lets you run multiple inspect operations without re-streaming.\n\nTo stream anyway: add --force-stream", mode, rawURI, localPath, mode, count, localPath)
}

func isCompressedCloud(rawURI string, parsed *uri.ParsedURI, meta *provider.ObjectMeta) bool {
	if parsed == nil || parsed.Provider == uri.ProviderLocal {
		return false
	}
	return isCompressed(rawURI, meta)
}

func UnsupportedInspectError(name string) error {
	return fmt.Errorf("inspect %s is not implemented yet in this Phase 3 slice", name)
}

func localFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
