package transfer

import (
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/fulmenhq/dimlox/internal/provider"
	"github.com/fulmenhq/dimlox/internal/providers"
	"github.com/fulmenhq/dimlox/internal/uri"
)

var ErrChecksumMismatch = errors.New("checksum mismatch")

type ProviderOptions = providers.Options

type DownloadOptions struct {
	ProviderOptions
	DestinationPath string
	LandingDir      string
	BlockSize       int64
	Concurrency     int
	Compress        bool
	Overwrite       bool
	Verify          bool
}

type UploadOptions struct {
	ProviderOptions
	SourcePath  string
	Destination string
	BlockSize   int64
	Concurrency int
	Compress    bool
	ContentType string
	LandingDir  string
}

type CopyOptions struct {
	ProviderOptions
	LandingDir  string
	BlockSize   int64
	Concurrency int
	Compress    bool
	KeepLanding bool
	Verify      bool
}

var providerResolver = providers.ForURI

func providerForURI(ctx context.Context, rawURI string, opts ProviderOptions) (provider.StorageProvider, *uri.ParsedURI, error) {
	return providerResolver(ctx, rawURI, providers.Options(opts))
}

func resolveLandingDir(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if env := os.Getenv("DIMLOX_LANDING_DIR"); env != "" {
		return env, nil
	}
	return os.Getwd()
}

func defaultDownloadPath(meta *provider.ObjectMeta, parsed *uri.ParsedURI, landingDir string, compress bool) (string, error) {
	base := basenameForTarget(meta, parsed)
	if base == "" {
		return "", fmt.Errorf("cannot derive destination filename from source")
	}
	if compress && !strings.HasSuffix(base, ".gz") {
		base += ".gz"
	}
	return filepath.Join(landingDir, base), nil
}

func basenameForTarget(meta *provider.ObjectMeta, parsed *uri.ParsedURI) string {
	if meta != nil && meta.Name != "" {
		return path.Base(meta.Name)
	}
	if parsed == nil {
		return ""
	}
	switch parsed.Provider {
	case uri.ProviderAZBlob:
		if parsed.AZBlobPath != "" {
			return path.Base(parsed.AZBlobPath)
		}
		return parsed.AZContainer
	case uri.ProviderGCS:
		if parsed.GCSObject != "" {
			return path.Base(parsed.GCSObject)
		}
		return parsed.GCSBucket
	case uri.ProviderLocal:
		return filepath.Base(parsed.LocalPath)
	default:
		return ""
	}
}

func shouldCompress(meta *provider.ObjectMeta, sourceName string) bool {
	if isCompressedName(sourceName) {
		return false
	}
	if meta != nil && strings.HasPrefix(meta.ContentType, "text/") {
		return true
	}
	switch strings.ToLower(filepath.Ext(sourceName)) {
	case ".csv", ".psv", ".tsv", ".json", ".txt":
		return true
	default:
		return false
	}
}

func isCompressedName(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".gz", ".zst", ".bz2", ".zip":
		return true
	default:
		return false
	}
}

func ensureParentDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}

func prepareOutputFile(finalPath string, overwrite bool) (*os.File, string, error) {
	if err := ensureParentDir(finalPath); err != nil {
		return nil, "", err
	}
	if !overwrite {
		if _, err := os.Stat(finalPath); err == nil {
			return nil, "", fmt.Errorf("destination exists: %s", finalPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, "", err
		}
	}
	tempPath := finalPath + ".part"
	if overwrite {
		_ = os.Remove(tempPath)
	}
	f, err := os.OpenFile(tempPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, "", err
	}
	return f, tempPath, nil
}

func finalizeOutput(tempPath, finalPath string, overwrite bool) error {
	if overwrite {
		_ = os.Remove(finalPath)
	}
	return os.Rename(tempPath, finalPath)
}

func checksumVerifier(meta *provider.ObjectMeta) (hash.Hash, []byte, bool) {
	if meta == nil {
		return nil, nil, false
	}
	if len(meta.MD5) > 0 {
		want := make([]byte, len(meta.MD5))
		copy(want, meta.MD5)
		return md5.New(), want, true
	}
	if meta.CRC32C != 0 {
		table := crc32.MakeTable(crc32.Castagnoli)
		want := make([]byte, 4)
		binary.BigEndian.PutUint32(want, meta.CRC32C)
		return crc32.New(table), want, true
	}
	return nil, nil, false
}

func verifyFile(path string, meta *provider.ObjectMeta) error {
	h, want, ok := checksumVerifier(meta)
	if !ok {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := h.Sum(nil)
	if !equalBytes(got, want) {
		return fmt.Errorf("%w: got %x want %x", ErrChecksumMismatch, got, want)
	}
	return nil
}

func verifySum(sum []byte, meta *provider.ObjectMeta) error {
	_, want, ok := checksumVerifier(meta)
	if !ok {
		return nil
	}
	if !equalBytes(sum, want) {
		return fmt.Errorf("%w: got %x want %x", ErrChecksumMismatch, sum, want)
	}
	return nil
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func streamCompressed(ctx context.Context, src provider.StorageProvider, rawURI string, dst *os.File, meta *provider.ObjectMeta, verify bool, progress func(int64)) error {
	reader, err := src.OpenReader(ctx, rawURI, 0, -1)
	if err != nil {
		return err
	}
	defer reader.Close()
	if _, err := dst.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := provider.ResetFile(dst, -1); err != nil {
		return err
	}
	gz := gzip.NewWriter(dst)
	defer gz.Close()
	var stream io.Reader = reader
	var verifier hash.Hash
	if verify {
		if h, _, ok := checksumVerifier(meta); ok {
			verifier = h
			stream = io.TeeReader(stream, h)
		}
	}
	if progress != nil {
		stream = provider.NewProgressReader(stream, progress)
	}
	if _, err := io.Copy(gz, stream); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	if verifier != nil {
		if err := verifySum(verifier.Sum(nil), meta); err != nil {
			return err
		}
	}
	return nil
}
func detectContentType(path string) string {
	if ct := mime.TypeByExtension(filepath.Ext(path)); ct != "" {
		return ct
	}
	return "application/octet-stream"
}
