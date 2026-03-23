// Package uri parses cloud and local storage URIs into a normalized internal
// form and detects the storage provider. This is Phase 0 of the dimlox plan.
//
// Supported input forms:
//
//	https://<account>.blob.core.windows.net/<container>/<path>  → AZBlob
//	azblob://<account>/<container>/<path>                       → AZBlob
//	https://storage.googleapis.com/<bucket>/<path>              → GCS
//	gs://<bucket>/<path>                                        → GCS
//	/absolute/path  ./relative/path  file:///path               → Local
//
// Unsupported schemes (e.g. s3://) return ErrUnsupportedScheme.
package uri

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
)

// Provider identifies the storage backend for a parsed URI.
type Provider int

const (
	ProviderUnknown Provider = iota
	ProviderAZBlob
	ProviderGCS
	ProviderLocal
)

func (p Provider) String() string {
	switch p {
	case ProviderAZBlob:
		return "azblob"
	case ProviderGCS:
		return "gcs"
	case ProviderLocal:
		return "local"
	default:
		return "unknown"
	}
}

// ParsedURI holds the normalized form of a storage URI.
type ParsedURI struct {
	// Provider is the detected storage backend.
	Provider Provider

	// Normalized is the canonical internal URI form:
	//   azblob://<account>/<container>/<path>
	//   gcs://<bucket>/<path>
	//   file:///absolute/path
	Normalized string

	// For AZBlob: account, container, blob path.
	AZAccount   string
	AZContainer string
	AZBlobPath  string

	// For GCS: bucket and object path.
	GCSBucket string
	GCSObject string

	// For local: absolute path.
	LocalPath string
}

// ErrUnsupportedScheme is returned when the URI scheme is not AZBlob, GCS, or local.
type ErrUnsupportedScheme struct {
	Scheme string
	Input  string
}

func (e *ErrUnsupportedScheme) Error() string {
	return fmt.Sprintf("unsupported URI scheme %q in %q (supported: azblob, gcs, gs, file, local path)", e.Scheme, e.Input)
}

var ErrEmptyURI = errors.New("URI must not be empty")

// Parse parses raw into a ParsedURI. It accepts all supported input forms
// described in the package doc and returns ErrUnsupportedScheme for anything else.
func Parse(raw string) (*ParsedURI, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, ErrEmptyURI
	}

	// Local path shortcuts: absolute or relative paths, no scheme.
	if strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "../") || isWindowsDrivePath(raw) {
		return parseLocal(raw)
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URI %q: %w", raw, err)
	}

	switch {
	case u.Scheme == "file":
		p := u.Path
		// On Windows, url.Parse("file:///C:/path") yields Path="/C:/path".
		// Strip the leading slash so filepath.Abs sees "C:/path".
		if runtime.GOOS == "windows" && len(p) >= 3 && p[0] == '/' && p[2] == ':' {
			p = p[1:]
		}
		return parseLocal(p)

	case u.Scheme == "azblob":
		return parseAZBlobNative(u, raw)

	case u.Scheme == "gs":
		return parseGCSNative(u, raw)

	case u.Scheme == "https" && strings.HasSuffix(u.Host, ".blob.core.windows.net"):
		return parseAZBlobHTTPS(u, raw)

	case u.Scheme == "https" && u.Host == "storage.googleapis.com":
		return parseGCSHTTPS(u, raw)

	case u.Scheme == "https":
		// Unknown HTTPS host — not AZBlob or GCS.
		return nil, &ErrUnsupportedScheme{Scheme: u.Scheme + "://" + u.Host, Input: raw}

	default:
		return nil, &ErrUnsupportedScheme{Scheme: u.Scheme, Input: raw}
	}
}

func parseLocal(path string) (*ParsedURI, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve local path %q: %w", path, err)
	}
	// Build a proper file URI: file:///path (forward slashes, no backslashes).
	uriPath := filepath.ToSlash(abs)
	if !strings.HasPrefix(uriPath, "/") {
		// Windows absolute paths like C:/... need a leading slash in file URIs.
		uriPath = "/" + uriPath
	}
	return &ParsedURI{
		Provider:   ProviderLocal,
		Normalized: "file://" + uriPath,
		LocalPath:  abs,
	}, nil
}

func parseAZBlobNative(u *url.URL, raw string) (*ParsedURI, error) {
	// azblob://<account>/<container>/<path>
	account := u.Host
	if account == "" {
		return nil, fmt.Errorf("azblob URI missing account name: %q", raw)
	}
	rawPath := strings.TrimPrefix(escapedPath(u), "/")
	parts := strings.SplitN(rawPath, "/", 2)
	if len(parts) < 1 || parts[0] == "" {
		return nil, fmt.Errorf("azblob URI missing container: %q", raw)
	}
	container := parts[0]
	blobPath := ""
	if len(parts) == 2 {
		blobPath = normalizeObjectPath(parts[1])
	}
	normalized := fmt.Sprintf("azblob://%s/%s", account, container)
	if blobPath != "" {
		normalized += "/" + blobPath
	} else if hasContainerOnlyTrailingSlash(rawPath) {
		normalized += "/"
	}
	return &ParsedURI{
		Provider:    ProviderAZBlob,
		Normalized:  normalized,
		AZAccount:   account,
		AZContainer: container,
		AZBlobPath:  blobPath,
	}, nil
}

func parseAZBlobHTTPS(u *url.URL, raw string) (*ParsedURI, error) {
	// https://<account>.blob.core.windows.net/<container>/<path>
	account := strings.TrimSuffix(u.Host, ".blob.core.windows.net")
	rawPath := strings.TrimPrefix(escapedPath(u), "/")
	parts := strings.SplitN(rawPath, "/", 2)
	if len(parts) < 1 || parts[0] == "" {
		return nil, fmt.Errorf("AZBlob HTTPS URI missing container: %q", raw)
	}
	container := parts[0]
	blobPath := ""
	if len(parts) == 2 {
		blobPath = normalizeObjectPath(parts[1])
	}
	normalized := fmt.Sprintf("azblob://%s/%s", account, container)
	if blobPath != "" {
		normalized += "/" + blobPath
	} else if hasContainerOnlyTrailingSlash(rawPath) {
		normalized += "/"
	}
	return &ParsedURI{
		Provider:    ProviderAZBlob,
		Normalized:  normalized,
		AZAccount:   account,
		AZContainer: container,
		AZBlobPath:  blobPath,
	}, nil
}

func parseGCSNative(u *url.URL, raw string) (*ParsedURI, error) {
	// gs://<bucket>/<object>
	bucket := u.Host
	if bucket == "" {
		return nil, fmt.Errorf("GCS URI missing bucket name: %q", raw)
	}
	rawPath := strings.TrimPrefix(escapedPath(u), "/")
	object := normalizeObjectPath(rawPath)
	normalized := fmt.Sprintf("gcs://%s", bucket)
	if object != "" {
		normalized += "/" + object
	} else if rawPath == "" && strings.HasSuffix(escapedPath(u), "/") {
		normalized += "/"
	}
	return &ParsedURI{
		Provider:   ProviderGCS,
		Normalized: normalized,
		GCSBucket:  bucket,
		GCSObject:  object,
	}, nil
}

func parseGCSHTTPS(u *url.URL, raw string) (*ParsedURI, error) {
	// https://storage.googleapis.com/<bucket>/<object>
	rawPath := strings.TrimPrefix(escapedPath(u), "/")
	parts := strings.SplitN(rawPath, "/", 2)
	if len(parts) < 1 || parts[0] == "" {
		return nil, fmt.Errorf("GCS HTTPS URI missing bucket: %q", raw)
	}
	bucket := parts[0]
	object := ""
	if len(parts) == 2 {
		object = normalizeObjectPath(parts[1])
	}
	normalized := fmt.Sprintf("gcs://%s", bucket)
	if object != "" {
		normalized += "/" + object
	} else if hasContainerOnlyTrailingSlash(rawPath) {
		normalized += "/"
	}
	return &ParsedURI{
		Provider:   ProviderGCS,
		Normalized: normalized,
		GCSBucket:  bucket,
		GCSObject:  object,
	}, nil
}

func escapedPath(u *url.URL) string {
	if path := u.EscapedPath(); path != "" {
		return path
	}
	return u.Path
}

func normalizeObjectPath(path string) string {
	return strings.TrimRight(path, "/")
}

func hasContainerOnlyTrailingSlash(path string) bool {
	trimmed := strings.TrimSuffix(path, "/")
	return path != "" && strings.HasSuffix(path, "/") && trimmed != "" && !strings.Contains(trimmed, "/")
}

// isWindowsDrivePath returns true for paths like "C:\...", "D:/...", "C:file".
// Only matches on Windows to avoid treating single-letter URI schemes as paths.
func isWindowsDrivePath(raw string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	if len(raw) < 2 {
		return false
	}
	letter := raw[0]
	if !((letter >= 'A' && letter <= 'Z') || (letter >= 'a' && letter <= 'z')) {
		return false
	}
	return raw[1] == ':'
}
