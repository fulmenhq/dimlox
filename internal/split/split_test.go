package split

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

func TestStreamSplitLocalWithHeaderAndManifest(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	src := filepath.Join(t.TempDir(), "sample.psv")
	content := strings.Join([]string{
		"c1|c2",
		"1|2",
		"3|4",
		"5|6",
		"7|8",
	}, "\n") + "\n"
	if err := os.WriteFile(src, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	res, err := Split(context.Background(), src, Options{Rows: 2, Header: true, OutDir: outDir, Manifest: true})
	if err != nil {
		t.Fatalf("Split() error = %v", err)
	}
	if res.Mode != ModeStream {
		t.Fatalf("Mode = %q, want stream", res.Mode)
	}
	if res.Delimiter != "|" {
		t.Fatalf("Delimiter = %q, want |", res.Delimiter)
	}
	if len(res.Shards) != 2 {
		t.Fatalf("len(Shards) = %d, want 2", len(res.Shards))
	}
	first, err := os.ReadFile(filepath.Join(outDir, "sample_shard_0001.psv"))
	if err != nil {
		t.Fatalf("ReadFile shard1: %v", err)
	}
	second, err := os.ReadFile(filepath.Join(outDir, "sample_shard_0002.psv"))
	if err != nil {
		t.Fatalf("ReadFile shard2: %v", err)
	}
	if string(first) != "c1|c2\n1|2\n3|4\n" {
		t.Fatalf("shard1 = %q", string(first))
	}
	if string(second) != "c1|c2\n5|6\n7|8\n" {
		t.Fatalf("shard2 = %q", string(second))
	}
	manifestData, err := os.ReadFile(res.ManifestPath)
	if err != nil {
		t.Fatalf("ReadFile manifest: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(manifestData), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("manifest lines = %d, want 2", len(lines))
	}
	var entry ManifestEntry
	if err := json.Unmarshal(lines[0], &entry); err != nil {
		t.Fatalf("Unmarshal manifest: %v", err)
	}
	if entry.ShardRows != 2 || !entry.HeaderCopied || entry.Delimiter != "|" {
		t.Fatalf("manifest entry = %+v", entry)
	}
	if matches, err := filepath.Glob(filepath.Join(outDir, "*.part")); err != nil {
		t.Fatalf("Glob: %v", err)
	} else if len(matches) != 0 {
		t.Fatalf("part files = %v, want none", matches)
	}
}

func TestStreamSplitDryRunWritesNoFiles(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	src := filepath.Join(t.TempDir(), "sample.psv")
	if err := os.WriteFile(src, []byte("c1|c2\n1|2\n3|4\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	res, err := Split(context.Background(), src, Options{Rows: 1, Header: true, OutDir: outDir, Manifest: true, DryRun: true})
	if err != nil {
		t.Fatalf("Split() error = %v", err)
	}
	if !res.DryRun {
		t.Fatal("DryRun = false, want true")
	}
	if len(res.Shards) != 2 {
		t.Fatalf("len(Shards) = %d, want 2", len(res.Shards))
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("outDir entries = %d, want 0", len(entries))
	}
}

func TestBinarySplitExactByteBoundaries(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	src := filepath.Join(t.TempDir(), "sample.bin")
	if err := os.WriteFile(src, []byte("abcdefghij"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	res, err := Split(context.Background(), src, Options{Mode: ModeBinary, Bytes: 4, OutDir: outDir, Manifest: true})
	if err != nil {
		t.Fatalf("Split() error = %v", err)
	}
	if len(res.Shards) != 3 {
		t.Fatalf("len(Shards) = %d, want 3", len(res.Shards))
	}
	got := make([]string, 0, 3)
	for i := 1; i <= 3; i++ {
		data, err := os.ReadFile(filepath.Join(outDir, fmt.Sprintf("sample_part_%04d.bin", i)))
		if err != nil {
			t.Fatalf("ReadFile part %d: %v", i, err)
		}
		got = append(got, string(data))
	}
	want := []string{"abcd", "efgh", "ij"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parts = %#v, want %#v", got, want)
	}
}

func TestRangeSplitCloudWithHeaderAndManifest(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	parsed, err := uri.Parse("gs://bucket/test.psv")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	fake := &stubSplitProvider{
		meta: &provider.ObjectMeta{Name: "test.psv", Size: int64(len("c1|c2\n1|2\n3|4\n5|6\n7|8\n")), ContentType: "text/plain"},
		data: []byte("c1|c2\n1|2\n3|4\n5|6\n7|8\n"),
	}
	oldResolver := providerResolver
	oldBlockSize := rangeReadBlockSize
	providerResolver = func(context.Context, string, ProviderOptions) (provider.StorageProvider, *uri.ParsedURI, error) {
		return fake, parsed, nil
	}
	rangeReadBlockSize = 5
	defer func() {
		providerResolver = oldResolver
		rangeReadBlockSize = oldBlockSize
	}()

	res, err := Split(context.Background(), "gs://bucket/test.psv", Options{Mode: ModeRange, Rows: 2, Header: true, OutDir: outDir, Manifest: true, Delimiter: "|", Encoding: "UTF-8"})
	if err != nil {
		t.Fatalf("Split() error = %v", err)
	}
	if res.Mode != ModeRange {
		t.Fatalf("Mode = %q, want range", res.Mode)
	}
	if len(res.Shards) != 2 {
		t.Fatalf("len(Shards) = %d, want 2", len(res.Shards))
	}
	if res.Shards[0].SplitMode != ModeRange || res.Shards[1].SplitMode != ModeRange {
		t.Fatalf("split modes = %q, %q; want range", res.Shards[0].SplitMode, res.Shards[1].SplitMode)
	}
	first, err := os.ReadFile(filepath.Join(outDir, "test_shard_0001.psv"))
	if err != nil {
		t.Fatalf("ReadFile shard1: %v", err)
	}
	second, err := os.ReadFile(filepath.Join(outDir, "test_shard_0002.psv"))
	if err != nil {
		t.Fatalf("ReadFile shard2: %v", err)
	}
	if string(first) != "c1|c2\n1|2\n3|4\n" {
		t.Fatalf("shard1 = %q", string(first))
	}
	if string(second) != "c1|c2\n5|6\n7|8\n" {
		t.Fatalf("shard2 = %q", string(second))
	}
	if len(fake.requests) < 3 {
		t.Fatalf("range requests = %d, want multiple", len(fake.requests))
	}
	manifestData, err := os.ReadFile(res.ManifestPath)
	if err != nil {
		t.Fatalf("ReadFile manifest: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(manifestData), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("manifest lines = %d, want 2", len(lines))
	}
	var entry ManifestEntry
	if err := json.Unmarshal(lines[0], &entry); err != nil {
		t.Fatalf("Unmarshal manifest: %v", err)
	}
	if entry.SplitMode != ModeRange {
		t.Fatalf("manifest split_mode = %q, want range", entry.SplitMode)
	}
}

func TestRangeSplitKeepsFinalLineWithoutTrailingNewline(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	parsed, err := uri.Parse("gs://bucket/test.psv")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	fake := &stubSplitProvider{
		meta: &provider.ObjectMeta{Name: "test.psv", Size: int64(len("a|b\n1|2\n3|4")), ContentType: "text/plain"},
		data: []byte("a|b\n1|2\n3|4"),
	}
	oldResolver := providerResolver
	oldBlockSize := rangeReadBlockSize
	providerResolver = func(context.Context, string, ProviderOptions) (provider.StorageProvider, *uri.ParsedURI, error) {
		return fake, parsed, nil
	}
	rangeReadBlockSize = 4
	defer func() {
		providerResolver = oldResolver
		rangeReadBlockSize = oldBlockSize
	}()

	res, err := Split(context.Background(), "gs://bucket/test.psv", Options{Mode: ModeRange, Rows: 1, OutDir: outDir, Delimiter: "|", Encoding: "UTF-8"})
	if err != nil {
		t.Fatalf("Split() error = %v", err)
	}
	if len(res.Shards) != 3 {
		t.Fatalf("len(Shards) = %d, want 3", len(res.Shards))
	}
	last, err := os.ReadFile(filepath.Join(outDir, "test_shard_0003.psv"))
	if err != nil {
		t.Fatalf("ReadFile shard3: %v", err)
	}
	if string(last) != "3|4" {
		t.Fatalf("shard3 = %q, want final unterminated row", string(last))
	}
}

func TestSplitAutoUsesRangeForCloudText(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	parsed, err := uri.Parse("gs://bucket/test.psv")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	fake := &stubSplitProvider{
		meta: &provider.ObjectMeta{Name: "test.psv", Size: 11, ContentType: "text/plain"},
		data: []byte("a|b|c\n1|2|3"),
	}
	oldResolver := providerResolver
	providerResolver = func(context.Context, string, ProviderOptions) (provider.StorageProvider, *uri.ParsedURI, error) {
		return fake, parsed, nil
	}
	defer func() { providerResolver = oldResolver }()

	res, err := Split(context.Background(), "gs://bucket/test.psv", Options{Rows: 1, OutDir: outDir, DryRun: true, Delimiter: "|", Encoding: "UTF-8"})
	if err != nil {
		t.Fatalf("Split() error = %v", err)
	}
	if res.Mode != ModeRange {
		t.Fatalf("Mode = %q, want range", res.Mode)
	}
	if len(res.Shards) != 2 {
		t.Fatalf("len(Shards) = %d, want 2", len(res.Shards))
	}
	if res.Shards[0].SplitMode != ModeRange || res.Shards[1].SplitMode != ModeRange {
		t.Fatalf("dry-run split modes = %q, %q; want range", res.Shards[0].SplitMode, res.Shards[1].SplitMode)
	}
	if fake.openCalls == 0 {
		t.Fatal("openCalls = 0, want range reader use")
	}
}

func TestRangeSplitRejectsLocalSource(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	src := filepath.Join(t.TempDir(), "sample.psv")
	if err := os.WriteFile(src, []byte("a|b\n1|2\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := Split(context.Background(), src, Options{Mode: ModeRange, Rows: 1, OutDir: outDir, Delimiter: "|", Encoding: "UTF-8"})
	if err == nil || !strings.Contains(err.Error(), "requires an uncompressed cloud text source") {
		t.Fatalf("Split() error = %v, want local range rejection", err)
	}
}

type openRequest struct {
	offset int64
	length int64
}

type stubSplitProvider struct {
	meta      *provider.ObjectMeta
	data      []byte
	statCalls int
	openCalls int
	requests  []openRequest
}

func (s *stubSplitProvider) Name() string { return "stub" }

func (s *stubSplitProvider) Stat(context.Context, string) (*provider.ObjectMeta, error) {
	s.statCalls++
	return s.meta, nil
}

func (s *stubSplitProvider) List(context.Context, string, provider.ListOptions) iter.Seq2[*provider.ObjectMeta, error] {
	return func(func(*provider.ObjectMeta, error) bool) {}
}

func (s *stubSplitProvider) OpenReader(_ context.Context, _ string, offset, length int64) (io.ReadCloser, error) {
	return s.openReader(offset, length)
}

func (s *stubSplitProvider) openReader(offset, length int64) (io.ReadCloser, error) {
	s.openCalls++
	s.requests = append(s.requests, openRequest{offset: offset, length: length})
	if offset < 0 {
		offset = 0
	}
	if offset > int64(len(s.data)) {
		offset = int64(len(s.data))
	}
	end := int64(len(s.data))
	if length >= 0 && offset+length < end {
		end = offset + length
	}
	return io.NopCloser(bytes.NewReader(s.data[offset:end])), nil
}

func (s *stubSplitProvider) DownloadFile(context.Context, string, *os.File, provider.DownloadOptions) error {
	return nil
}

func (s *stubSplitProvider) UploadFile(context.Context, *os.File, string, provider.UploadOptions) error {
	return nil
}
