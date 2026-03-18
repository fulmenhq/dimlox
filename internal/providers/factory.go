package providers

import (
	"context"
	"fmt"

	"github.com/fulmenhq/dimlox/internal/provider"
	providerazblob "github.com/fulmenhq/dimlox/internal/provider/azblob"
	providergcs "github.com/fulmenhq/dimlox/internal/provider/gcs"
	providerlocal "github.com/fulmenhq/dimlox/internal/provider/local"
	"github.com/fulmenhq/dimlox/internal/uri"
)

type Options struct {
	AZProfile  string
	GCPProject string
}

func ForURI(ctx context.Context, rawURI string, opts Options) (provider.StorageProvider, *uri.ParsedURI, error) {
	parsed, err := uri.Parse(rawURI)
	if err != nil {
		return nil, nil, err
	}

	switch parsed.Provider {
	case uri.ProviderAZBlob:
		p, err := providerazblob.NewAZBlobProvider(ctx, opts.AZProfile)
		if err != nil {
			return nil, nil, err
		}
		return p, parsed, nil
	case uri.ProviderGCS:
		p, err := providergcs.NewGCSProvider(ctx, opts.GCPProject)
		if err != nil {
			return nil, nil, err
		}
		return p, parsed, nil
	case uri.ProviderLocal:
		return providerlocal.NewLocalProvider(), parsed, nil
	default:
		return nil, nil, fmt.Errorf("unsupported provider: %s", parsed.Provider)
	}
}
