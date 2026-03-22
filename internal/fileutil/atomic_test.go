package fileutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicRenameRemovesExistingOnWindows(t *testing.T) {
	origGOOS := runtimeGOOS
	origRemove := removeFile
	origRename := renameFile
	t.Cleanup(func() {
		runtimeGOOS = origGOOS
		removeFile = origRemove
		renameFile = origRename
	})

	runtimeGOOS = "windows"
	removed := false
	removeFile = func(path string) error {
		removed = true
		return nil
	}
	renameFile = func(string, string) error { return nil }

	if err := AtomicRename("from.part", "to.txt", true); err != nil {
		t.Fatalf("AtomicRename() error = %v", err)
	}
	if !removed {
		t.Fatal("removeFile was not called on Windows replace")
	}
}

func TestAtomicRenameIgnoresMissingTargetOnWindows(t *testing.T) {
	origGOOS := runtimeGOOS
	origRemove := removeFile
	origRename := renameFile
	t.Cleanup(func() {
		runtimeGOOS = origGOOS
		removeFile = origRemove
		renameFile = origRename
	})

	runtimeGOOS = "windows"
	removeFile = func(string) error { return os.ErrNotExist }
	renameFile = func(string, string) error { return nil }

	if err := AtomicRename("from.part", "to.txt", true); err != nil {
		t.Fatalf("AtomicRename() error = %v", err)
	}
}

func TestAtomicRenameReplacesDestinationFile(t *testing.T) {
	origGOOS := runtimeGOOS
	origRemove := removeFile
	origRename := renameFile
	t.Cleanup(func() {
		runtimeGOOS = origGOOS
		removeFile = origRemove
		renameFile = origRename
	})

	runtimeGOOS = "windows"
	removeFile = os.Remove
	renameFile = os.Rename

	dir := t.TempDir()
	src := filepath.Join(dir, "source.part")
	dst := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(src, []byte("new"), 0o644); err != nil {
		t.Fatalf("WriteFile src: %v", err)
	}
	if err := os.WriteFile(dst, []byte("old"), 0o644); err != nil {
		t.Fatalf("WriteFile dst: %v", err)
	}

	if err := AtomicRename(src, dst, true); err != nil {
		t.Fatalf("AtomicRename() error = %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile dst: %v", err)
	}
	if string(data) != "new" {
		t.Fatalf("destination = %q, want new", string(data))
	}
	if _, err := os.Stat(src); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("source stat error = %v, want not exist", err)
	}
}
