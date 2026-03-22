package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPhase5LocalSmokeWorkflow(t *testing.T) {
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "source")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("MkdirAll source: %v", err)
	}
	src := filepath.Join(srcDir, "orders.psv")
	if err := os.WriteFile(src, []byte("c1|c2\n1|2\n3|4\n"), 0o644); err != nil {
		t.Fatalf("WriteFile source: %v", err)
	}
	dst := filepath.Join(tmp, "downloaded.psv")
	shardDir := filepath.Join(tmp, "shards")
	if err := os.MkdirAll(shardDir, 0o755); err != nil {
		t.Fatalf("MkdirAll shards: %v", err)
	}

	doctorOut, doctorErr := runRootCommand(t, "doctor", src)
	if doctorErr != nil {
		t.Fatalf("doctor error = %v", doctorErr)
	}
	if !strings.Contains(doctorOut, "provider: local") || !strings.Contains(doctorOut, "name: orders.psv") {
		t.Fatalf("doctor output = %q, want local provider metadata", doctorOut)
	}

	lsOut, lsErr := runRootCommand(t, "ls", srcDir)
	if lsErr != nil {
		t.Fatalf("ls error = %v", lsErr)
	}
	if !strings.Contains(lsOut, "orders.psv") {
		t.Fatalf("ls output = %q, want source file", lsOut)
	}

	if _, getErr := runRootCommand(t, "get", src, dst); getErr != nil {
		t.Fatalf("get error = %v", getErr)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile downloaded: %v", err)
	}
	if string(data) != "c1|c2\n1|2\n3|4\n" {
		t.Fatalf("downloaded file = %q, want original content", string(data))
	}

	inspectOut, inspectErr := runRootCommand(t, "inspect", "--wc", dst)
	if inspectErr != nil {
		t.Fatalf("inspect --wc error = %v", inspectErr)
	}
	if !strings.Contains(inspectOut, "lines: 3") {
		t.Fatalf("inspect output = %q, want line count", inspectOut)
	}

	splitOut, splitErr := runRootCommand(t, "split", "--rows", "1", "--header", "--out-dir", shardDir, dst)
	if splitErr != nil {
		t.Fatalf("split error = %v", splitErr)
	}
	if !strings.Contains(splitOut, "mode: stream") || !strings.Contains(splitOut, "shards: 2") {
		t.Fatalf("split output = %q, want stream split summary", splitOut)
	}
	assertFileText(t, filepath.Join(shardDir, "downloaded_shard_0001.psv"), "c1|c2\n1|2\n")
	assertFileText(t, filepath.Join(shardDir, "downloaded_shard_0002.psv"), "c1|c2\n3|4\n")
	if _, err := os.Stat(filepath.Join(shardDir, "downloaded_manifest.jsonl")); err != nil {
		t.Fatalf("manifest stat error = %v", err)
	}
}

func runRootCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := rootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	return stdout.String(), err
}

func assertFileText(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("ReadFile(%q) = %q, want %q", path, string(data), want)
	}
}
