package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/tabwriter"
	"time"

	"github.com/fulmenhq/dimlox/internal/provider"
)

func TestLSJSONLinesOutput(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "alpha.txt")
	if err := os.WriteFile(file, []byte("alpha"), 0o644); err != nil {
		t.Fatalf("write alpha: %v", err)
	}

	cmd := rootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"ls", "--format", "json", root})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(ls --format json) error = %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("json line count = %d, want 1; output=%q", len(lines), stdout.String())
	}

	var meta provider.ObjectMeta
	if err := json.Unmarshal([]byte(lines[0]), &meta); err != nil {
		t.Fatalf("Unmarshal(json line) error = %v", err)
	}
	if meta.Name != "alpha.txt" {
		t.Fatalf("meta.Name = %q, want %q", meta.Name, "alpha.txt")
	}
	if meta.IsPrefix {
		t.Fatal("meta.IsPrefix = true, want false")
	}
}

func TestLSRecursiveLocalOutput(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "sub")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cmd := rootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"ls", "--recursive", root})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(ls --recursive) error = %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	got := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(got) != 2 {
		t.Fatalf("line count = %d, want 2; output=%q", len(got), stdout.String())
	}
	if got[0] != "sub/" {
		t.Fatalf("line 1 = %q, want %q", got[0], "sub/")
	}
	if got[1] != "sub/file.txt" {
		t.Fatalf("line 2 = %q, want %q", got[1], "sub/file.txt")
	}
}

func TestWriteLSRowFormatsHash(t *testing.T) {
	buf := new(bytes.Buffer)
	tw := tabwriter.NewWriter(buf, 0, 4, 2, ' ', 0)

	meta := &provider.ObjectMeta{
		Name:         "artifact.bin",
		Size:         42,
		ContentType:  "application/octet-stream",
		ETag:         "etag-1",
		MD5:          []byte{0xde, 0xad, 0xbe, 0xef},
		LastModified: time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
	}

	writeLSRow(tw, meta, true, true)
	if err := tw.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "artifact.bin") {
		t.Fatalf("output missing name: %q", out)
	}
	if !strings.Contains(out, hex.EncodeToString(meta.MD5)) {
		t.Fatalf("output missing md5 hash: %q", out)
	}
	if !strings.Contains(out, "etag-1") {
		t.Fatalf("output missing etag: %q", out)
	}
}
