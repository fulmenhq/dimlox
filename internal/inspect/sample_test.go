package inspect

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"iter"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/fulmenhq/dimlox/internal/provider"
	"github.com/fulmenhq/dimlox/internal/uri"
)

func TestHeadLocalText(t *testing.T) {
	path := writeLinesFile(t, "sample.txt", []string{"header", "a", "b", "c", "d"})
	res, err := Head(context.Background(), path, 3, ProviderOptions{})
	if err != nil {
		t.Fatalf("Head() error = %v", err)
	}
	want := []string{"header", "a", "b"}
	if !reflect.DeepEqual(res.Lines, want) {
		t.Fatalf("Lines = %#v, want %#v", res.Lines, want)
	}
}

func TestTailLocalText(t *testing.T) {
	path := writeLinesFile(t, "sample.txt", []string{"h", "1", "2", "3", "4"})
	res, err := Tail(context.Background(), path, 2, ProviderOptions{})
	if err != nil {
		t.Fatalf("Tail() error = %v", err)
	}
	want := []string{"3", "4"}
	if !reflect.DeepEqual(res.Lines, want) {
		t.Fatalf("Lines = %#v, want %#v", res.Lines, want)
	}
	if res.Strategy != "backward-local" {
		t.Fatalf("Strategy = %q, want backward-local", res.Strategy)
	}
}

func TestMidLocalText(t *testing.T) {
	path := writeLinesFile(t, "sample.txt", []string{"header", "a", "b", "c", "d", "e", "f"})
	res, err := Mid(context.Background(), path, 2, ProviderOptions{})
	if err != nil {
		t.Fatalf("Mid() error = %v", err)
	}
	if len(res.Lines) != 2 {
		t.Fatalf("line count = %d, want 2", len(res.Lines))
	}
	if res.Strategy != "midpoint-seek" {
		t.Fatalf("Strategy = %q, want midpoint-seek", res.Strategy)
	}
}

func TestMidLocalBoundaryAlignedOffset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(path, []byte("ab\ncd\n12\n34\n"), 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}
	res, err := Mid(context.Background(), path, 1, ProviderOptions{})
	if err != nil {
		t.Fatalf("Mid() error = %v", err)
	}
	want := []string{"12"}
	if !reflect.DeepEqual(res.Lines, want) {
		t.Fatalf("Lines = %#v, want %#v", res.Lines, want)
	}
}

func TestTailCompressedFallsBackToForwardStream(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.txt.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create gzip file: %v", err)
	}
	gz := gzip.NewWriter(f)
	_, _ = gz.Write([]byte("h\n1\n2\n3\n4\n"))
	_ = gz.Close()
	_ = f.Close()

	res, err := Tail(context.Background(), path, 2, ProviderOptions{})
	if err != nil {
		t.Fatalf("Tail() error = %v", err)
	}
	want := []string{"3", "4"}
	if !reflect.DeepEqual(res.Lines, want) {
		t.Fatalf("Lines = %#v, want %#v", res.Lines, want)
	}
	if !res.Compressed {
		t.Fatal("Compressed = false, want true")
	}
	if res.Strategy != "forward-stream-fallback" {
		t.Fatalf("Strategy = %q, want forward-stream-fallback", res.Strategy)
	}
}

func TestRefuseCompressedCloudTailReturnsAdvisoryWithoutOpen(t *testing.T) {
	fake := &stubStorageProvider{meta: &provider.ObjectMeta{Size: 128, ContentType: "application/gzip"}}
	parsed, err := uri.Parse("azblob://acct/ctr/file.gz")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	oldResolver := providerResolver
	providerResolver = func(context.Context, string, ProviderOptions) (provider.StorageProvider, *uri.ParsedURI, error) {
		return fake, parsed, nil
	}
	defer func() { providerResolver = oldResolver }()

	err = RefuseCompressedCloudSample(context.Background(), "azblob://acct/ctr/file.gz", SampleTail, 5, ProviderOptions{})
	if err == nil {
		t.Fatal("RefuseCompressedCloudSample() error = nil, want advisory")
	}
	msg := err.Error()
	if !strings.Contains(msg, "--tail on a compressed cloud source") {
		t.Fatalf("error = %q, want tail refusal guidance", msg)
	}
	wantLocal := filepath.Join(os.TempDir(), "file.gz")
	if !strings.Contains(msg, "dimlox get azblob://acct/ctr/file.gz "+wantLocal) {
		t.Fatalf("error = %q, want concrete get recipe", msg)
	}
	if !strings.Contains(msg, "dimlox inspect --tail 5 "+wantLocal) {
		t.Fatalf("error = %q, want counted local inspect recipe", msg)
	}
	if !strings.Contains(msg, "To stream anyway: add --force-stream") {
		t.Fatalf("error = %q, want force-stream guidance", msg)
	}
	if fake.statCalls != 1 {
		t.Fatalf("statCalls = %d, want 1", fake.statCalls)
	}
	if fake.openCalls != 0 {
		t.Fatalf("openCalls = %d, want 0", fake.openCalls)
	}
	if fake.lastStatURI != "azblob://acct/ctr/file.gz" {
		t.Fatalf("lastStatURI = %q, want original URI", fake.lastStatURI)
	}
}

func TestTailCompressedCloudProceedsWhenForcedPathCalled(t *testing.T) {
	data := gzipBytes(t, "header\n1\n2\n3\n4\n")
	fake := &stubStorageProvider{meta: &provider.ObjectMeta{Size: int64(len(data)), ContentType: "application/gzip"}, readerBytes: data}
	parsed, err := uri.Parse("gs://bucket/file.gz")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	oldResolver := providerResolver
	providerResolver = func(context.Context, string, ProviderOptions) (provider.StorageProvider, *uri.ParsedURI, error) {
		return fake, parsed, nil
	}
	defer func() { providerResolver = oldResolver }()

	res, err := Tail(context.Background(), "gs://bucket/file.gz", 2, ProviderOptions{})
	if err != nil {
		t.Fatalf("Tail() error = %v", err)
	}
	want := []string{"3", "4"}
	if !reflect.DeepEqual(res.Lines, want) {
		t.Fatalf("Lines = %#v, want %#v", res.Lines, want)
	}
	if res.Strategy != "forward-stream-fallback" {
		t.Fatalf("Strategy = %q, want forward-stream-fallback", res.Strategy)
	}
	if fake.openCalls == 0 {
		t.Fatal("openCalls = 0, want stream open")
	}
}

type stubStorageProvider struct {
	meta        *provider.ObjectMeta
	readerBytes []byte
	statCalls   int
	openCalls   int
	lastStatURI string
}

func (s *stubStorageProvider) Name() string { return "stub" }

func (s *stubStorageProvider) Stat(_ context.Context, uri string) (*provider.ObjectMeta, error) {
	s.statCalls++
	s.lastStatURI = uri
	return s.meta, nil
}

func (s *stubStorageProvider) List(context.Context, string, provider.ListOptions) iter.Seq2[*provider.ObjectMeta, error] {
	return func(func(*provider.ObjectMeta, error) bool) {}
}

func (s *stubStorageProvider) OpenReader(context.Context, string, int64, int64) (io.ReadCloser, error) {
	s.openCalls++
	return io.NopCloser(bytes.NewReader(s.readerBytes)), nil
}

func (s *stubStorageProvider) DownloadFile(context.Context, string, *os.File, provider.DownloadOptions) error {
	return nil
}

func (s *stubStorageProvider) UploadFile(context.Context, *os.File, string, provider.UploadOptions) error {
	return nil
}

func gzipBytes(t *testing.T, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(content)); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

func writeLinesFile(t *testing.T, name string, lines []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	content := ""
	for _, line := range lines {
		content += line + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}
	return path
}
