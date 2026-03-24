package transfer

import (
	"bytes"
	"context"
	"fmt"
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

func TestBuildCopyPlanPositionalMultiSource(t *testing.T) {
	tmp := t.TempDir()
	src1 := filepath.Join(tmp, "alpha.txt")
	src2 := filepath.Join(tmp, "bravo.txt")
	for _, file := range []string{src1, src2} {
		if err := os.WriteFile(file, []byte(filepath.Base(file)), 0o644); err != nil {
			t.Fatalf("WriteFile(%q): %v", file, err)
		}
	}

	plan, err := BuildCopyPlan(context.Background(), []string{src1, src2, filepath.Join(tmp, "out") + "/"}, CopyPlanOptions{})
	if err != nil {
		t.Fatalf("BuildCopyPlan() error = %v", err)
	}
	if len(plan.Items) != 2 {
		t.Fatalf("len(plan.Items) = %d, want 2", len(plan.Items))
	}
	if !strings.HasSuffix(plan.Items[0].Destination, "/alpha.txt") {
		t.Fatalf("destination[0] = %q, want suffix /alpha.txt", plan.Items[0].Destination)
	}
	if !strings.HasSuffix(plan.Items[1].Destination, "/bravo.txt") {
		t.Fatalf("destination[1] = %q, want suffix /bravo.txt", plan.Items[1].Destination)
	}
}

func TestBuildCopyPlanFromFileSkipsComments(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "transfers.jsonl")
	content := strings.Join([]string{
		"# comment",
		"// comment",
		`{"src":"gs://bucket/a.txt","dst":"azblob://acct/container/a.txt"}`,
		"",
	}, "\n")
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	plan, err := BuildCopyPlan(context.Background(), nil, CopyPlanOptions{FromFile: file})
	if err != nil {
		t.Fatalf("BuildCopyPlan() error = %v", err)
	}
	if len(plan.Items) != 1 {
		t.Fatalf("len(plan.Items) = %d, want 1", len(plan.Items))
	}
	if plan.Items[0].Source != "gs://bucket/a.txt" {
		t.Fatalf("source = %q, want gs://bucket/a.txt", plan.Items[0].Source)
	}
}

func TestBuildCopyPlanGlobExpandsMatches(t *testing.T) {
	origResolver := providerResolver
	t.Cleanup(func() { providerResolver = origResolver })

	providerResolver = func(context.Context, string, providers.Options) (provider.StorageProvider, *uri.ParsedURI, error) {
		return fakeListProvider{items: []*provider.ObjectMeta{
			{URI: "gcs://bucket/data/orders_2024.psv", Name: "data/orders_2024.psv"},
			{URI: "gcs://bucket/data/orders_2025.psv", Name: "data/orders_2025.psv"},
			{URI: "gcs://bucket/data/ignore.txt", Name: "data/ignore.txt"},
		}}, &uri.ParsedURI{Provider: uri.ProviderGCS, GCSBucket: "bucket", GCSObject: "data/orders_*.psv"}, nil
	}

	plan, err := BuildCopyPlan(context.Background(), []string{"gs://bucket/data/orders_*.psv", "azblob://acct/container/out/"}, CopyPlanOptions{MaxSources: 10})
	if err != nil {
		t.Fatalf("BuildCopyPlan() error = %v", err)
	}
	if len(plan.Items) != 2 {
		t.Fatalf("len(plan.Items) = %d, want 2", len(plan.Items))
	}
}

func TestBuildCopyPlanGlobCollisionFails(t *testing.T) {
	origResolver := providerResolver
	t.Cleanup(func() { providerResolver = origResolver })

	providerResolver = func(context.Context, string, providers.Options) (provider.StorageProvider, *uri.ParsedURI, error) {
		return fakeListProvider{items: []*provider.ObjectMeta{
			{URI: "gcs://bucket/data/a/orders.psv", Name: "data/a/orders.psv"},
			{URI: "gcs://bucket/data/b/orders.psv", Name: "data/b/orders.psv"},
		}}, &uri.ParsedURI{Provider: uri.ProviderGCS, GCSBucket: "bucket", GCSObject: "data/*/orders.psv"}, nil
	}

	_, err := BuildCopyPlan(context.Background(), []string{"gs://bucket/data/*/orders.psv", "azblob://acct/container/out/"}, CopyPlanOptions{MaxSources: 10})
	if err == nil || !strings.Contains(err.Error(), "destination collision") {
		t.Fatalf("BuildCopyPlan() error = %v, want destination collision", err)
	}
}

func TestExecuteCopyPlanContinueOnError(t *testing.T) {
	tmp := t.TempDir()
	src1 := filepath.Join(tmp, "ok.txt")
	if err := os.WriteFile(src1, []byte("alpha\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	missing := filepath.Join(tmp, "missing.txt")
	dstDir := filepath.Join(tmp, "out")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	plan := &CopyPlan{Items: []CopyPlanItem{
		{Source: src1, Destination: filepath.Join(dstDir, "ok.txt")},
		{Source: missing, Destination: filepath.Join(dstDir, "missing.txt")},
	}}
	stderr := new(bytes.Buffer)
	result, err := ExecuteCopyPlan(context.Background(), plan, ExecuteCopyPlanOptions{
		CopyOptions:     CopyOptions{LandingDir: filepath.Join(tmp, "landing")},
		ContinueOnError: true,
		SummaryWriter:   stderr,
	})
	if err == nil {
		t.Fatal("ExecuteCopyPlan() error = nil, want failure")
	}
	if result.Transferred != 1 || result.Failed != 1 || result.Skipped != 0 {
		t.Fatalf("result = %+v, want transferred=1 failed=1 skipped=0", result)
	}
	if !strings.Contains(stderr.String(), "cp summary: transferred=1 failed=1 skipped=0") {
		t.Fatalf("summary = %q, want batch summary", stderr.String())
	}
	data, readErr := os.ReadFile(filepath.Join(dstDir, "ok.txt"))
	if readErr != nil {
		t.Fatalf("ReadFile(ok.txt): %v", readErr)
	}
	if string(data) != "alpha\n" {
		t.Fatalf("ok.txt = %q, want alpha", string(data))
	}
}

type fakeListProvider struct {
	items []*provider.ObjectMeta
}

func (f fakeListProvider) Name() string { return "fake" }

func (f fakeListProvider) Stat(context.Context, string) (*provider.ObjectMeta, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f fakeListProvider) List(context.Context, string, provider.ListOptions) iter.Seq2[*provider.ObjectMeta, error] {
	return func(yield func(*provider.ObjectMeta, error) bool) {
		for _, item := range f.items {
			if !yield(item, nil) {
				return
			}
		}
	}
}

func (f fakeListProvider) OpenReader(context.Context, string, int64, int64) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f fakeListProvider) DownloadFile(context.Context, string, *os.File, provider.DownloadOptions) error {
	return fmt.Errorf("not implemented")
}

func (f fakeListProvider) UploadFile(context.Context, *os.File, string, provider.UploadOptions) error {
	return fmt.Errorf("not implemented")
}
