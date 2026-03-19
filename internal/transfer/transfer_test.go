package transfer

import (
	"compress/gzip"
	"context"
	"errors"
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
