package inspect

import (
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"
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
