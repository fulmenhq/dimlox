package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulmenhq/dimlox/internal/inspect"
)

func TestInspectWCJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(path, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}
	cmd := rootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"inspect", "--wc", "--format", "json", path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var got inspect.WCResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.Lines != 2 {
		t.Fatalf("Lines = %d, want 2", got.Lines)
	}
}

func TestInspectRequiresMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(path, []byte("a\n"), 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}
	cmd := rootCmd()
	cmd.SetArgs([]string{"inspect", path})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "inspect requires one of") {
		t.Fatalf("error = %q, want inspect mode guidance", err.Error())
	}
	if got := exitCodeFor(err); got != exitBadURI {
		t.Fatalf("exitCodeFor(inspect missing mode) = %d, want %d", got, exitBadURI)
	}
}

func TestInspectInvalidFormatExitCode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(path, []byte("a\n"), 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}
	cmd := rootCmd()
	cmd.SetArgs([]string{"inspect", "--wc", "--format", "yaml", path})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if got := exitCodeFor(err); got != exitBadURI {
		t.Fatalf("exitCodeFor(inspect invalid format) = %d, want %d", got, exitBadURI)
	}
}

func TestInspectHeadTextOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(path, []byte("header\na\nb\n"), 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}
	cmd := rootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"inspect", "--head", "2", path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "mode: head") || !strings.Contains(out, "header") || !strings.Contains(out, "a") {
		t.Fatalf("output = %q, want head sample content", out)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestInspectDetectJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.psv")
	if err := os.WriteFile(path, []byte("c1|c2|c3\n1|2|3\n4|5|6\n"), 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}
	cmd := rootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"inspect", "--detect", "--format", "json", "--sample-bytes", "64", path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var got inspect.DetectResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.Delimiter != "|" {
		t.Fatalf("Delimiter = %q, want |", got.Delimiter)
	}
	if got.Encoding != "UTF-8" {
		t.Fatalf("Encoding = %q, want UTF-8", got.Encoding)
	}
	if got.FieldsPerRow != 3 {
		t.Fatalf("FieldsPerRow = %d, want 3", got.FieldsPerRow)
	}
}

func TestInspectForceStreamIsNoOpForLocalCompressedTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.txt.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create gzip file: %v", err)
	}
	gz := gzip.NewWriter(f)
	if _, err := gz.Write([]byte("h\n1\n2\n3\n4\n")); err != nil {
		t.Fatalf("write gzip payload: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close gzip file: %v", err)
	}

	cmd := rootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"inspect", "--tail", "2", "--force-stream", path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "strategy: forward-stream-fallback") || !strings.Contains(out, "3") || !strings.Contains(out, "4") {
		t.Fatalf("output = %q, want forced tail sample content", out)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
