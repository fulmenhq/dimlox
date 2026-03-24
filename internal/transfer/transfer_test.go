package transfer

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulmenhq/dimlox/internal/provider"
	"github.com/fulmenhq/dimlox/internal/providers"
	"github.com/fulmenhq/dimlox/internal/uri"
)

func TestDownloadLocalFile(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source.txt")
	if err := os.WriteFile(src, []byte("alpha\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	dst := filepath.Join(tmp, "download.txt")

	res, err := Download(context.Background(), src, DownloadOptions{DestinationPath: dst, Overwrite: true})
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if res.Destination != dst {
		t.Fatalf("Destination = %q, want %q", res.Destination, dst)
	}
	assertFileContent(t, dst, "alpha\n")
}

func TestDownloadCompressLocalFile(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source.txt")
	if err := os.WriteFile(src, []byte("bravo\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	res, err := Download(context.Background(), src, DownloadOptions{LandingDir: tmp, Compress: true, Overwrite: true})
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if filepath.Ext(res.Destination) != ".gz" {
		t.Fatalf("Destination ext = %q, want .gz", filepath.Ext(res.Destination))
	}
	assertGzipContent(t, res.Destination, "bravo\n")
}

func TestDownloadCompressSkipsChecksumUnlessVerifyRequested(t *testing.T) {
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "download.txt.gz")
	providerResolver = func(ctx context.Context, rawURI string, opts providers.Options) (provider.StorageProvider, *uri.ParsedURI, error) {
		return fakeProvider{
			meta: &provider.ObjectMeta{Name: "source.txt", ContentType: "text/plain", MD5: []byte{0x00}},
			body: []byte("echo\n"),
		}, &uri.ParsedURI{Provider: uri.ProviderGCS, GCSBucket: "bucket", GCSObject: "source.txt"}, nil
	}
	defer func() { providerResolver = providers.ForURI }()

	res, err := Download(context.Background(), "gs://bucket/source.txt", DownloadOptions{DestinationPath: dst, Compress: true, Overwrite: true, Verify: false})
	if err != nil {
		t.Fatalf("Download() error = %v, want nil", err)
	}
	if res.Destination != dst {
		t.Fatalf("Destination = %q, want %q", res.Destination, dst)
	}
	assertGzipContent(t, dst, "echo\n")

	_, err = Download(context.Background(), "gs://bucket/source.txt", DownloadOptions{DestinationPath: filepath.Join(tmp, "verify.txt.gz"), Compress: true, Overwrite: true, Verify: true})
	if err == nil {
		t.Fatal("Download() error = nil, want checksum mismatch")
	}
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("Download() error = %v, want ErrChecksumMismatch", err)
	}
	if _, statErr := os.Stat(filepath.Join(tmp, "verify.txt.gz.part")); !os.IsNotExist(statErr) {
		t.Fatalf("verify temp file still exists or stat err=%v", statErr)
	}
}

func TestDownloadFinalizeFailureRemovesPartFile(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source.txt")
	if err := os.WriteFile(src, []byte("golf\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	dst := filepath.Join(tmp, "download.txt")
	origFinalize := finalizeDownloadedOutput
	t.Cleanup(func() { finalizeDownloadedOutput = origFinalize })
	finalizeDownloadedOutput = func(tempPath, finalPath string, overwrite bool) error {
		return fmt.Errorf("rename %s -> %s: forced failure", tempPath, finalPath)
	}

	_, err := Download(context.Background(), src, DownloadOptions{DestinationPath: dst, Overwrite: true})
	if err == nil || !strings.Contains(err.Error(), "forced failure") {
		t.Fatalf("Download() error = %v, want forced finalize failure", err)
	}
	if _, statErr := os.Stat(dst + ".part"); !os.IsNotExist(statErr) {
		t.Fatalf("part file still exists or stat err=%v", statErr)
	}
}

func TestUploadLocalFile(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source.txt")
	if err := os.WriteFile(src, []byte("charlie\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	dst := filepath.Join(tmp, "nested", "upload.txt")

	_, err := Upload(context.Background(), UploadOptions{SourcePath: src, Destination: dst})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	assertFileContent(t, dst, "charlie\n")
}

func TestCopyLocalFileCleansLanding(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source.txt")
	if err := os.WriteFile(src, []byte("delta\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	dst := filepath.Join(tmp, "copied.txt")
	landing := filepath.Join(tmp, "landing")

	res, err := Copy(context.Background(), src, dst, CopyOptions{LandingDir: landing})
	if err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	assertFileContent(t, dst, "delta\n")
	if _, err := os.Stat(res.LandingPath); !os.IsNotExist(err) {
		t.Fatalf("landing path %q still exists or stat err=%v", res.LandingPath, err)
	}
}

func TestDownloadCancellationRemovesPartFile(t *testing.T) {
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "download.txt")
	ctx, cancel := context.WithCancel(context.Background())

	providerResolver = func(context.Context, string, providers.Options) (provider.StorageProvider, *uri.ParsedURI, error) {
		return cancelingDownloadProvider{cancel: cancel}, &uri.ParsedURI{Provider: uri.ProviderGCS, GCSBucket: "bucket", GCSObject: "source.txt"}, nil
	}
	defer func() { providerResolver = providers.ForURI }()

	_, err := Download(ctx, "gs://bucket/source.txt", DownloadOptions{DestinationPath: dst, Overwrite: true})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Download() error = %v, want context.Canceled", err)
	}
	if _, statErr := os.Stat(dst + ".part"); !os.IsNotExist(statErr) {
		t.Fatalf("part file still exists or stat err=%v", statErr)
	}
}

func TestUploadCancellationRemovesCompressedTempFile(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source.txt")
	if err := os.WriteFile(src, []byte("echo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())

	providerResolver = func(context.Context, string, providers.Options) (provider.StorageProvider, *uri.ParsedURI, error) {
		return cancelingUploadProvider{cancel: cancel}, &uri.ParsedURI{Provider: uri.ProviderGCS, GCSBucket: "bucket", GCSObject: "target.txt.gz"}, nil
	}
	defer func() { providerResolver = providers.ForURI }()

	_, err := Upload(ctx, UploadOptions{SourcePath: src, Destination: "gs://bucket/target.txt.gz", Compress: true, LandingDir: tmp})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Upload() error = %v, want context.Canceled", err)
	}
	if matches, globErr := filepath.Glob(filepath.Join(tmp, "dimlox-upload-*.gz")); globErr != nil {
		t.Fatalf("Glob: %v", globErr)
	} else if len(matches) != 0 {
		t.Fatalf("compressed temp files = %v, want none", matches)
	}
}

func TestCopyCancellationRemovesLandingFile(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source.txt")
	if err := os.WriteFile(src, []byte("foxtrot\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	landing := filepath.Join(tmp, "landing")
	ctx, cancel := context.WithCancel(context.Background())

	providerResolver = func(_ context.Context, rawURI string, _ providers.Options) (provider.StorageProvider, *uri.ParsedURI, error) {
		if strings.HasPrefix(rawURI, "gs://") {
			return cancelingUploadProvider{cancel: cancel}, &uri.ParsedURI{Provider: uri.ProviderGCS, GCSBucket: "bucket", GCSObject: "target.txt"}, nil
		}
		return providers.ForURI(context.Background(), rawURI, providers.Options{})
	}
	defer func() { providerResolver = providers.ForURI }()

	_, err := Copy(ctx, src, "gs://bucket/target.txt", CopyOptions{LandingDir: landing})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Copy() error = %v, want context.Canceled", err)
	}
	if matches, globErr := filepath.Glob(filepath.Join(landing, "*.part")); globErr != nil {
		t.Fatalf("Glob: %v", globErr)
	} else if len(matches) != 0 {
		t.Fatalf("landing part files = %v, want none", matches)
	}
	if entries, readErr := os.ReadDir(landing); readErr == nil && len(entries) != 0 {
		t.Fatalf("landing dir entries = %v, want none", entries)
	}
}

func assertFileContent(t *testing.T, path string, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("ReadFile(%q) = %q, want %q", path, string(data), want)
	}
}

func assertGzipContent(t *testing.T, path string, want string) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open gzip file: %v", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	defer gz.Close()
	data, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(data) != want {
		t.Fatalf("gzip content = %q, want %q", string(data), want)
	}
}

type fakeProvider struct {
	meta *provider.ObjectMeta
	body []byte
}

func (f fakeProvider) Name() string                                               { return "fake" }
func (f fakeProvider) Stat(context.Context, string) (*provider.ObjectMeta, error) { return f.meta, nil }
func (f fakeProvider) List(context.Context, string, provider.ListOptions) iter.Seq2[*provider.ObjectMeta, error] {
	return func(yield func(*provider.ObjectMeta, error) bool) {}
}
func (f fakeProvider) OpenReader(context.Context, string, int64, int64) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(string(f.body))), nil
}
func (f fakeProvider) DownloadFile(context.Context, string, *os.File, provider.DownloadOptions) error {
	return nil
}
func (f fakeProvider) UploadFile(context.Context, *os.File, string, provider.UploadOptions) error {
	return nil
}

type cancelingDownloadProvider struct {
	cancel context.CancelFunc
}

func (c cancelingDownloadProvider) Name() string { return "cancel-download" }
func (c cancelingDownloadProvider) Stat(context.Context, string) (*provider.ObjectMeta, error) {
	return &provider.ObjectMeta{Name: "source.txt", Size: 7, ContentType: "text/plain"}, nil
}
func (c cancelingDownloadProvider) List(context.Context, string, provider.ListOptions) iter.Seq2[*provider.ObjectMeta, error] {
	return func(yield func(*provider.ObjectMeta, error) bool) {}
}
func (c cancelingDownloadProvider) OpenReader(context.Context, string, int64, int64) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("ignored")), nil
}
func (c cancelingDownloadProvider) DownloadFile(ctx context.Context, _ string, file *os.File, _ provider.DownloadOptions) error {
	if _, err := file.WriteString("partial"); err != nil {
		return err
	}
	c.cancel()
	<-ctx.Done()
	return ctx.Err()
}
func (c cancelingDownloadProvider) UploadFile(context.Context, *os.File, string, provider.UploadOptions) error {
	return fmt.Errorf("not implemented")
}

type cancelingUploadProvider struct {
	cancel context.CancelFunc
}

func (c cancelingUploadProvider) Name() string { return "cancel-upload" }
func (c cancelingUploadProvider) Stat(context.Context, string) (*provider.ObjectMeta, error) {
	return &provider.ObjectMeta{Name: "target.txt", Size: 0, ContentType: "text/plain"}, nil
}
func (c cancelingUploadProvider) List(context.Context, string, provider.ListOptions) iter.Seq2[*provider.ObjectMeta, error] {
	return func(yield func(*provider.ObjectMeta, error) bool) {}
}
func (c cancelingUploadProvider) OpenReader(context.Context, string, int64, int64) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not implemented")
}
func (c cancelingUploadProvider) DownloadFile(context.Context, string, *os.File, provider.DownloadOptions) error {
	return fmt.Errorf("not implemented")
}
func (c cancelingUploadProvider) UploadFile(ctx context.Context, file *os.File, _ string, _ provider.UploadOptions) error {
	buf := make([]byte, 16)
	_, _ = file.Read(buf)
	c.cancel()
	<-ctx.Done()
	return ctx.Err()
}
