package transfer

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestNewProgressReporterUsesStructuredModeWhenStdoutIsPiped(t *testing.T) {
	origStdout := progressStdout
	origStderr := progressStderr
	origDetector := progressTTYDetector
	t.Cleanup(func() {
		progressStdout = origStdout
		progressStderr = origStderr
		progressTTYDetector = origDetector
	})

	stdoutFile, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatalf("CreateTemp stdout: %v", err)
	}
	defer stdoutFile.Close()

	progressStdout = stdoutFile
	progressStderr = os.Stderr
	progressTTYDetector = func(f *os.File) bool {
		return f == os.Stderr
	}

	reporter := newProgressReporter("get", 128)
	if reporter.interactive {
		t.Fatal("interactive = true, want false when stdout is piped")
	}
}

func TestNewProgressReporterUsesStructuredModeWhenStderrIsPiped(t *testing.T) {
	origStdout := progressStdout
	origStderr := progressStderr
	origDetector := progressTTYDetector
	t.Cleanup(func() {
		progressStdout = origStdout
		progressStderr = origStderr
		progressTTYDetector = origDetector
	})

	stderrFile, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatalf("CreateTemp stderr: %v", err)
	}
	defer stderrFile.Close()

	progressStdout = os.Stdout
	progressStderr = stderrFile
	progressTTYDetector = func(f *os.File) bool {
		return f == os.Stdout
	}

	reporter := newProgressReporter("put", 128)
	if reporter.interactive {
		t.Fatal("interactive = true, want false when stderr is piped")
	}
}

func TestProgressReporterPrintStructuredUsesJSONLines(t *testing.T) {
	var buf bytes.Buffer
	reporter := &progressReporter{
		label: "get",
		out:   &buf,
		total: 200,
		start: time.Now().Add(-2 * time.Second),
	}
	reporter.current.Store(100)

	reporter.printStructured(false)

	var event struct {
		Status         string  `json:"status"`
		Label          string  `json:"label"`
		Bytes          int64   `json:"bytes"`
		Total          int64   `json:"total"`
		Pct            float64 `json:"pct"`
		ElapsedSeconds float64 `json:"elapsed_seconds"`
		ETASeconds     float64 `json:"eta_seconds"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &event); err != nil {
		t.Fatalf("Unmarshal structured progress: %v", err)
	}
	if event.Status != "progress" || event.Label != "get" {
		t.Fatalf("status/label = %q/%q, want progress/get", event.Status, event.Label)
	}
	if event.Bytes != 100 || event.Total != 200 {
		t.Fatalf("bytes/total = %d/%d, want 100/200", event.Bytes, event.Total)
	}
	if event.Pct <= 0 || event.ElapsedSeconds <= 0 {
		t.Fatalf("pct/elapsed = %v/%v, want positive values", event.Pct, event.ElapsedSeconds)
	}
	if event.ETASeconds < 0 {
		t.Fatalf("eta_seconds = %v, want non-negative", event.ETASeconds)
	}
	if bytes.Contains(buf.Bytes(), []byte("\r")) {
		t.Fatalf("structured progress contains carriage return: %q", buf.String())
	}
}
