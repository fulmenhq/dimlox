package inspect

import (
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
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
