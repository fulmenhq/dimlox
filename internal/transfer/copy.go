package transfer

import (
	"context"
	"os"
	"path/filepath"
)

type CopyResult struct {
	LandingPath string
	Target      string
}

func Copy(ctx context.Context, srcURI, dstURI string, opts CopyOptions) (*CopyResult, error) {
	landingDir, err := resolveLandingDir(opts.LandingDir)
	if err != nil {
		return nil, err
	}
	srcProviderOptions := mergeProviderOptions(opts.ProviderOptions, opts.SourceProviderOptions)
	dstProviderOptions := mergeProviderOptions(opts.ProviderOptions, opts.DestinationProviderOptions)
	getResult, err := Download(ctx, srcURI, DownloadOptions{
		ProviderOptions: srcProviderOptions,
		LandingDir:      landingDir,
		BlockSize:       opts.BlockSize,
		Concurrency:     opts.Concurrency,
		Compress:        opts.Compress,
		Overwrite:       true,
		Verify:          opts.Verify,
	})
	if err != nil {
		return nil, err
	}
	landingPath := getResult.Destination
	if _, err := Upload(ctx, UploadOptions{
		ProviderOptions: dstProviderOptions,
		SourcePath:      landingPath,
		Destination:     dstURI,
		BlockSize:       opts.BlockSize,
		Concurrency:     opts.Concurrency,
	}); err != nil {
		return nil, err
	}
	if !opts.KeepLanding {
		_ = os.Remove(landingPath)
	}
	return &CopyResult{LandingPath: filepath.Clean(landingPath), Target: dstURI}, nil
}
