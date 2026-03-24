package inspect

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"iter"
	"os"
	"path/filepath"
	"testing"

	"github.com/fulmenhq/dimlox/internal/provider"
	"github.com/fulmenhq/dimlox/internal/uri"
)

func TestWCLocalText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(path, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}
	res, err := WC(context.Background(), path, ProviderOptions{})
	if err != nil {
		t.Fatalf("WC() error = %v", err)
	}
	if res.Lines != 3 {
		t.Fatalf("Lines = %d, want 3", res.Lines)
	}
	if res.Compressed {
		t.Fatal("Compressed = true, want false")
	}
}

func TestWCLocalGzip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.txt.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create gzip file: %v", err)
	}
	gz := gzip.NewWriter(f)
	if _, err := gz.Write([]byte("row1\nrow2\nrow3")); err != nil {
		t.Fatalf("write gzip payload: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close gzip file: %v", err)
	}
	res, err := WC(context.Background(), path, ProviderOptions{})
	if err != nil {
		t.Fatalf("WC() error = %v", err)
	}
	if res.Lines != 3 {
		t.Fatalf("Lines = %d, want 3", res.Lines)
	}
	if !res.Compressed {
		t.Fatal("Compressed = false, want true")
	}
}

func TestWCCancelReturnsContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	oldResolver := providerResolver
	t.Cleanup(func() { providerResolver = oldResolver })

	providerResolver = func(context.Context, string, ProviderOptions) (provider.StorageProvider, *uri.ParsedURI, error) {
		return wcCancelProvider{cancel: cancel}, &uri.ParsedURI{Provider: uri.ProviderGCS, GCSBucket: "bucket", GCSObject: "sample.txt"}, nil
	}

	_, err := WC(ctx, "gs://bucket/sample.txt", ProviderOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WC() error = %v, want context.Canceled", err)
	}
}

type wcCancelProvider struct {
	cancel context.CancelFunc
}

func (w wcCancelProvider) Name() string { return "wc-cancel" }
func (w wcCancelProvider) Stat(context.Context, string) (*provider.ObjectMeta, error) {
	return &provider.ObjectMeta{Name: "sample.txt", Size: 16, ContentType: "text/plain"}, nil
}
func (w wcCancelProvider) List(context.Context, string, provider.ListOptions) iter.Seq2[*provider.ObjectMeta, error] {
	return func(yield func(*provider.ObjectMeta, error) bool) {}
}
func (w wcCancelProvider) OpenReader(ctx context.Context, _ string, _ int64, _ int64) (io.ReadCloser, error) {
	return &cancelAfterReadCloser{cancel: w.cancel, ctx: ctx}, nil
}
func (w wcCancelProvider) DownloadFile(context.Context, string, *os.File, provider.DownloadOptions) error {
	return nil
}
func (w wcCancelProvider) UploadFile(context.Context, *os.File, string, provider.UploadOptions) error {
	return nil
}

type cancelAfterReadCloser struct {
	cancel context.CancelFunc
	ctx    context.Context
	fired  bool
}

func (c *cancelAfterReadCloser) Read(p []byte) (int, error) {
	if !c.fired {
		copy(p, []byte("row1\n"))
		c.fired = true
		c.cancel()
		<-c.ctx.Done()
		return 5, context.Canceled
	}
	return 0, context.Canceled
}

func (c *cancelAfterReadCloser) Close() error { return nil }
