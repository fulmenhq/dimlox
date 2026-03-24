package transfer

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/fulmenhq/dimlox/internal/provider"
	"github.com/fulmenhq/dimlox/internal/uri"
)

func expandGlob(ctx context.Context, raw string, opts ProviderOptions, maxSources int) ([]resolvedSource, error) {
	parsed, err := uri.Parse(raw)
	if err != nil {
		return nil, err
	}
	if err := validateGlobLocation(parsed); err != nil {
		return nil, err
	}
	storageProvider, _, err := providerForURI(ctx, raw, opts)
	if err != nil {
		return nil, err
	}
	listURI, matchPattern, trimPrefix, err := globListURI(parsed)
	if err != nil {
		return nil, err
	}
	if _, err := path.Match(matchPattern, ""); err != nil {
		return nil, fmt.Errorf("invalid glob pattern %q: %w", raw, err)
	}

	matches := make([]resolvedSource, 0)
	for meta, err := range storageProvider.List(ctx, listURI, provider.ListOptions{Recursive: true}) {
		if err != nil {
			return nil, err
		}
		if meta == nil || meta.IsPrefix {
			continue
		}
		candidate, ok := globCandidate(parsed.Provider, meta, trimPrefix)
		if !ok {
			continue
		}
		matched, err := path.Match(matchPattern, candidate)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", raw, err)
		}
		if !matched {
			continue
		}
		base := path.Base(meta.Name)
		if base == "" || base == "." || base == "/" {
			continue
		}
		if maxSources > 0 && len(matches) >= maxSources {
			return nil, fmt.Errorf("glob source expansion exceeded --max-sources=%d for %s", maxSources, raw)
		}
		matches = append(matches, resolvedSource{Source: userFacingSourceURI(meta.URI), Basename: base})
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("glob source matched no files: %s", raw)
	}
	return matches, nil
}

func validateGlobLocation(parsed *uri.ParsedURI) error {
	if parsed == nil {
		return fmt.Errorf("glob parse result was nil")
	}
	if hasGlobMeta(parsed.AZAccount) || hasGlobMeta(parsed.AZContainer) || hasGlobMeta(parsed.GCSBucket) {
		return fmt.Errorf("glob characters are only supported in object paths, not provider or container names")
	}
	return nil
}

func globListURI(parsed *uri.ParsedURI) (string, string, string, error) {
	switch parsed.Provider {
	case uri.ProviderAZBlob:
		literal := globLiteralPrefix(parsed.AZBlobPath)
		return azblobPrefixURI(parsed, literal), parsed.AZBlobPath[len(literal):], literal, nil
	case uri.ProviderGCS:
		literal := globLiteralPrefix(parsed.GCSObject)
		return gcsPrefixURI(parsed, literal), parsed.GCSObject[len(literal):], literal, nil
	case uri.ProviderLocal:
		pattern := filepath.ToSlash(parsed.LocalPath)
		literal := globLiteralPrefix(pattern)
		rootDir := localGlobRoot(pattern, literal)
		matchPattern := strings.TrimPrefix(pattern, rootDir)
		matchPattern = strings.TrimPrefix(matchPattern, "/")
		return filepath.FromSlash(rootDir), matchPattern, "", nil
	default:
		return "", "", "", fmt.Errorf("unsupported provider for glob expansion: %s", parsed.Provider)
	}
}

func globLiteralPrefix(pattern string) string {
	idx := strings.IndexAny(pattern, "*?[")
	if idx < 0 {
		return pattern
	}
	return pattern[:idx]
}

func azblobPrefixURI(parsed *uri.ParsedURI, prefix string) string {
	base := fmt.Sprintf("azblob://%s/%s", parsed.AZAccount, parsed.AZContainer)
	if prefix == "" {
		return base + "/"
	}
	return base + "/" + prefix
}

func gcsPrefixURI(parsed *uri.ParsedURI, prefix string) string {
	base := fmt.Sprintf("gcs://%s", parsed.GCSBucket)
	if prefix == "" {
		return base + "/"
	}
	return base + "/" + prefix
}

func localGlobRoot(pattern, literal string) string {
	if strings.HasSuffix(literal, "/") {
		return strings.TrimSuffix(literal, "/")
	}
	root := path.Dir(literal)
	if root == "." {
		return path.Dir(pattern)
	}
	return root
}

func globCandidate(providerType uri.Provider, meta *provider.ObjectMeta, trimPrefix string) (string, bool) {
	if providerType == uri.ProviderLocal {
		return filepath.ToSlash(meta.Name), true
	}
	if trimPrefix == "" {
		return meta.Name, true
	}
	if !strings.HasPrefix(meta.Name, trimPrefix) {
		return "", false
	}
	return strings.TrimPrefix(meta.Name, trimPrefix), true
}

func userFacingSourceURI(raw string) string {
	if strings.HasPrefix(raw, "gcs://") {
		return "gs://" + strings.TrimPrefix(raw, "gcs://")
	}
	return raw
}
