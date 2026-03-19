package transfer

import (
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/fulmenhq/dimlox/internal/provider"
)

type UploadResult struct {
	Source string
	Target string
}

func Upload(ctx context.Context, opts UploadOptions) (*UploadResult, error) {
	srcFile, cleanup, contentType, err := prepareUploadSource(opts)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}
	dstProvider, _, err := providerForURI(ctx, opts.Destination, opts.ProviderOptions)
	if err != nil {
		return nil, err
	}
	info, err := srcFile.Stat()
	if err != nil {
		return nil, err
	}
	reporter := newProgressReporter("put", info.Size())
	reporter.Start()
	defer reporter.Finish()
	if err := dstProvider.UploadFile(ctx, srcFile, opts.Destination, provider.UploadOptions{
		BlockSize:   opts.BlockSize,
		Concurrency: opts.Concurrency,
		ContentType: contentType,
		Progress:    reporter.Observe,
	}); err != nil {
		return nil, err
	}
	return &UploadResult{Source: opts.SourcePath, Target: opts.Destination}, nil
}

func prepareUploadSource(opts UploadOptions) (*os.File, func(), string, error) {
	file, err := os.Open(opts.SourcePath)
	if err != nil {
		return nil, nil, "", err
	}
	cleanup := func() { _ = file.Close() }
	contentType := opts.ContentType
	if contentType == "" {
		contentType = detectContentType(opts.SourcePath)
	}
	if !opts.Compress || !shouldCompress(nil, filepath.Base(opts.SourcePath)) {
		return file, cleanup, contentType, nil
	}
	landingDir, err := resolveLandingDir(opts.LandingDir)
	if err != nil {
		cleanup()
		return nil, nil, "", err
	}
	tempFile, err := os.CreateTemp(landingDir, "dimlox-upload-*.gz")
	if err != nil {
		cleanup()
		return nil, nil, "", err
	}
	gz := gzip.NewWriter(tempFile)
	if _, err := io.Copy(gz, file); err != nil {
		cleanup()
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
		return nil, nil, "", err
	}
	cleanup()
	if err := gz.Close(); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
		return nil, nil, "", err
	}
	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
		return nil, nil, "", err
	}
	return tempFile, func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}, "application/gzip", nil
}
