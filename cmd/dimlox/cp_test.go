package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulmenhq/gofulmen/foundry"
)

func TestCPDryRunPrintsTransferPlan(t *testing.T) {
	tmp := t.TempDir()
	src1 := filepath.Join(tmp, "a.txt")
	src2 := filepath.Join(tmp, "b.txt")
	for _, file := range []string{src1, src2} {
		if err := os.WriteFile(file, []byte(filepath.Base(file)), 0o644); err != nil {
			t.Fatalf("WriteFile(%q): %v", file, err)
		}
	}
	dst := filepath.Join(tmp, "out") + "/"

	cmd := rootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"cp", "--dry-run", src1, src2, dst})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "transfer plan (2 file(s)):") {
		t.Fatalf("stdout = %q, want transfer plan header", got)
	}
	if !strings.Contains(got, filepath.Join(tmp, "out")+"/a.txt") || !strings.Contains(got, filepath.Join(tmp, "out")+"/b.txt") {
		t.Fatalf("stdout = %q, want mapped destinations", got)
	}
}

func TestCPDryRunLocalGlobPrintsTransferPlan(t *testing.T) {
	tmp := t.TempDir()
	dataDir := filepath.Join(tmp, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	for _, name := range []string{"orders_a.psv", "orders_b.psv"} {
		if err := os.WriteFile(filepath.Join(dataDir, name), []byte(name), 0o644); err != nil {
			t.Fatalf("WriteFile(%q): %v", name, err)
		}
	}

	cmd := rootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"cp", "--dry-run", filepath.Join(dataDir, "orders_*.psv"), filepath.Join(tmp, "out") + "/"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	got := stdout.String()
	if !strings.Contains(got, "transfer plan (2 file(s)):") {
		t.Fatalf("stdout = %q, want transfer plan header", got)
	}
}

func TestCPRejectsParallelAboveOne(t *testing.T) {
	cmd := rootCmd()
	cmd.SetArgs([]string{"cp", "--parallel", "2", "/tmp/src.txt", "/tmp/dst.txt"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if got := exitCodeFor(err); got != foundry.ExitInvalidArgument {
		t.Fatalf("exitCodeFor(cp --parallel 2) = %d, want %d (err=%v)", got, foundry.ExitInvalidArgument, err)
	}
}

func TestCPRejectsMissingPerLegGCSCredentialFileDuringPreflight(t *testing.T) {
	cmd := rootCmd()
	cmd.SetArgs([]string{"cp", "--dry-run", "--gcp-creds-file-src", "/tmp/missing-sa.json", "gs://bucket/orders.psv", "/tmp/out.psv"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if got := exitCodeFor(err); got != foundry.ExitInvalidArgument {
		t.Fatalf("exitCodeFor(cp missing gcs creds) = %d, want %d (err=%v)", got, foundry.ExitInvalidArgument, err)
	}
	if !strings.Contains(err.Error(), "source GCS auth preflight") {
		t.Fatalf("err = %v, want source preflight context", err)
	}
}

func TestCPSingleFileRegressionStillCopies(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "source.txt")
	dst := filepath.Join(tmp, "copied.txt")
	if err := os.WriteFile(src, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cmd := rootCmd()
	cmd.SetArgs([]string{"cp", src, dst})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", dst, err)
	}
	if string(data) != "hello\n" {
		t.Fatalf("copied data = %q, want hello", string(data))
	}
}

func TestWorstBatchExitCodePrefersAuthOverDataAndDisk(t *testing.T) {
	errs := []error{
		withExitCode(foundry.ExitFailure, "%v", contextCanceledErr{}),
		withExitCode(foundry.ExitDataCorrupt, "%v", errors.New("checksum mismatch")),
		withExitCode(foundry.ExitAuthenticationFailed, "%v", errors.New("auth failed")),
	}
	if got := worstBatchExitCode(errs); got != foundry.ExitAuthenticationFailed {
		t.Fatalf("worstBatchExitCode() = %d, want %d", got, foundry.ExitAuthenticationFailed)
	}
}

type contextCanceledErr struct{}

func (contextCanceledErr) Error() string { return "context canceled" }
