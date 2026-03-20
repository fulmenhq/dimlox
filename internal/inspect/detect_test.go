package inspect

import (
	"compress/gzip"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"
)

func TestDetectLocalUTF8Pipe(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.psv")
	content := "col1|col2|col3\na|b|c\nd|e|f\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}

	res, err := Detect(context.Background(), path, 64, ProviderOptions{})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if res.Encoding != "UTF-8" {
		t.Fatalf("Encoding = %q, want UTF-8", res.Encoding)
	}
	if res.BOM {
		t.Fatal("BOM = true, want false")
	}
	if res.LineEnding != "LF" {
		t.Fatalf("LineEnding = %q, want LF", res.LineEnding)
	}
	if res.Delimiter != "|" {
		t.Fatalf("Delimiter = %q, want |", res.Delimiter)
	}
	if res.FieldsPerRow != 3 {
		t.Fatalf("FieldsPerRow = %d, want 3", res.FieldsPerRow)
	}
	if res.DelimiterConfidence < 0.95 {
		t.Fatalf("DelimiterConfidence = %.3f, want >= 0.95", res.DelimiterConfidence)
	}
	if res.SampleRows != 3 {
		t.Fatalf("SampleRows = %d, want 3", res.SampleRows)
	}
}

func TestDetectLocalGzip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.psv.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create gzip file: %v", err)
	}
	gz := gzip.NewWriter(f)
	if _, err := gz.Write([]byte("c1|c2\n1|2\n3|4\n")); err != nil {
		t.Fatalf("write gzip payload: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close gzip file: %v", err)
	}

	res, err := Detect(context.Background(), path, 64, ProviderOptions{})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if !res.Compressed {
		t.Fatal("Compressed = false, want true")
	}
	if res.Delimiter != "|" {
		t.Fatalf("Delimiter = %q, want |", res.Delimiter)
	}
	if res.FieldsPerRow != 2 {
		t.Fatalf("FieldsPerRow = %d, want 2", res.FieldsPerRow)
	}
}

func TestDetectIgnoresTruncatedTrailingSampleRow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partial.psv")
	content := "a|b|c\n1|2|3\n4|5|6\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}

	res, err := Detect(context.Background(), path, 15, ProviderOptions{})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if res.Delimiter != "|" {
		t.Fatalf("Delimiter = %q, want |", res.Delimiter)
	}
	if res.FieldsPerRow != 3 {
		t.Fatalf("FieldsPerRow = %d, want 3", res.FieldsPerRow)
	}
	if res.SampleRows != 2 {
		t.Fatalf("SampleRows = %d, want 2 complete rows", res.SampleRows)
	}
}

func TestDetectKeepsSingleEOFRowWithoutTrailingNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one-line.psv")
	content := "a|b|c"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}

	res, err := Detect(context.Background(), path, 64, ProviderOptions{})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if res.Delimiter != "|" {
		t.Fatalf("Delimiter = %q, want |", res.Delimiter)
	}
	if res.SampleRows != 1 {
		t.Fatalf("SampleRows = %d, want 1", res.SampleRows)
	}
}

func TestDetectKeepsFinalEOFRowWithoutTrailingNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "two-lines.psv")
	content := "a|b|c\n1|2|3"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}

	res, err := Detect(context.Background(), path, 64, ProviderOptions{})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if res.Delimiter != "|" {
		t.Fatalf("Delimiter = %q, want |", res.Delimiter)
	}
	if res.SampleRows != 2 {
		t.Fatalf("SampleRows = %d, want 2", res.SampleRows)
	}
}

func TestDetectUTF16LEWithBOM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.csv")
	utf16Content := utf16.Encode([]rune("c1,c2\r\n1,2\r\n3,4\r\n"))
	buf := make([]byte, 2+len(utf16Content)*2)
	buf[0] = 0xFF
	buf[1] = 0xFE
	for i, r := range utf16Content {
		binary.LittleEndian.PutUint16(buf[2+i*2:], r)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}

	res, err := Detect(context.Background(), path, 128, ProviderOptions{})
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if res.Encoding != "UTF-16LE" {
		t.Fatalf("Encoding = %q, want UTF-16LE", res.Encoding)
	}
	if !res.BOM {
		t.Fatal("BOM = false, want true")
	}
	if res.LineEnding != "CRLF" {
		t.Fatalf("LineEnding = %q, want CRLF", res.LineEnding)
	}
	if res.Delimiter != "," {
		t.Fatalf("Delimiter = %q, want comma", res.Delimiter)
	}
	if res.FieldsPerRow != 2 {
		t.Fatalf("FieldsPerRow = %d, want 2", res.FieldsPerRow)
	}
}
