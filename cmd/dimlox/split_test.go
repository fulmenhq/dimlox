package main

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitCommandDryRun(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	src := filepath.Join(t.TempDir(), "sample.psv")
	if err := os.WriteFile(src, []byte("c1|c2\n1|2\n3|4\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cmd := rootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"split", "--rows", "1", "--header", "--dry-run", "--out-dir", outDir, src})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "mode: stream") || !strings.Contains(out, "dry-run: true") || !strings.Contains(out, "delimiter: |") {
		t.Fatalf("output = %q, want split summary", out)
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("outDir entries = %d, want 0", len(entries))
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestSplitCommandDryRunCompressedShowsGuidance(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	src := filepath.Join(t.TempDir(), "sample.psv.gz")
	var payload bytes.Buffer
	gz := gzip.NewWriter(&payload)
	if _, err := gz.Write([]byte("c1|c2\n1|2\n3|4\n")); err != nil {
		t.Fatalf("gz.Write: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gz.Close: %v", err)
	}
	if err := os.WriteFile(src, payload.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cmd := rootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"split", "--rows", "1", "--header", "--dry-run", "--out-dir", outDir, src})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "note: Compressed shard bytes are not predicted in dry-run.") {
		t.Fatalf("output = %q, want compressed dry-run note", out)
	}
	if !strings.Contains(out, "\"logical_bytes\":") {
		t.Fatalf("output = %q, want logical_bytes in shard JSON", out)
	}
	if strings.Contains(out, "\"shard_bytes\":") {
		t.Fatalf("output = %q, do not want exact shard_bytes for compressed dry-run", out)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
