package transfer

import (
	"context"
	"fmt"

	"github.com/fulmenhq/dimlox/internal/provider"
)

type DownloadResult struct {
	Destination string
	Meta        *provider.ObjectMeta
}

func Download(ctx context.Context, srcURI string, opts DownloadOptions) (*DownloadResult, error) {
	srcProvider, parsed, err := providerForURI(ctx, srcURI, opts.ProviderOptions)
	if err != nil {
		return nil, err
	}
	meta, err := srcProvider.Stat(ctx, srcURI)
	if err != nil {
		return nil, err
	}
	compress := opts.Compress && shouldCompress(meta, basenameForTarget(meta, parsed))
	destination := opts.DestinationPath
	if destination == "" {
		landingDir, err := resolveLandingDir(opts.LandingDir)
		if err != nil {
			return nil, err
		}
		destination, err = defaultDownloadPath(meta, parsed, landingDir, compress)
		if err != nil {
			return nil, err
		}
	}
	dst, tempPath, err := prepareOutputFile(destination, opts.Overwrite)
	if err != nil {
		return nil, err
	}
	defer dst.Close()
	reporter := newProgressReporter("get", meta.Size)
	reporter.Start()
	defer reporter.Finish()
	if compress {
		err = streamCompressed(ctx, srcProvider, srcURI, dst, meta, opts.Verify, reporter.Observe)
	} else {
		err = srcProvider.DownloadFile(ctx, srcURI, dst, provider.DownloadOptions{
			BlockSize:   opts.BlockSize,
			Concurrency: opts.Concurrency,
			Progress:    reporter.Observe,
		})
	}
	if err != nil {
		return nil, err
	}
	if err := dst.Close(); err != nil {
		return nil, err
	}
	if opts.Verify && !compress {
		if err := verifyFile(tempPath, meta); err != nil {
			return nil, err
		}
	}
	if err := finalizeOutput(tempPath, destination, opts.Overwrite); err != nil {
		return nil, err
	}
	return &DownloadResult{Destination: destination, Meta: meta}, nil
}

func DownloadToPath(ctx context.Context, srcURI, dstPath string, opts DownloadOptions) (*DownloadResult, error) {
	opts.DestinationPath = dstPath
	return Download(ctx, srcURI, opts)
}

func DownloadToLanding(ctx context.Context, srcURI string, opts DownloadOptions) (*DownloadResult, error) {
	res, err := Download(ctx, srcURI, opts)
	if err != nil {
		return nil, err
	}
	if res.Destination == "" {
		return nil, fmt.Errorf("download destination was empty")
	}
	return res, nil
}
