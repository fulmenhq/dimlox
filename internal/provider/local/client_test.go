package local

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fulmenhq/dimlox/internal/provider"
)

func TestListNonRecursiveIncludesChildren(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "alpha.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatalf("write alpha: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	p := NewLocalProvider()
	seq := p.List(context.Background(), root, provider.ListOptions{})

	seen := map[string]bool{}
	for meta, err := range seq {
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		seen[meta.Name] = true
	}

	if !seen["alpha.txt"] {
		t.Fatalf("alpha.txt not listed: %#v", seen)
	}
	if !seen["nested"] {
		t.Fatalf("nested dir not listed: %#v", seen)
	}
}

func TestListRecursiveIncludesNestedChildren(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "beta.txt"), []byte("beta"), 0o644); err != nil {
		t.Fatalf("write beta: %v", err)
	}

	p := NewLocalProvider()
	seq := p.List(context.Background(), root, provider.ListOptions{Recursive: true})

	seen := map[string]bool{}
	for meta, err := range seq {
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		seen[meta.Name] = true
	}

	if !seen["nested"] {
		t.Fatalf("nested dir not listed: %#v", seen)
	}
	if !seen["nested/beta.txt"] {
		t.Fatalf("nested file not listed: %#v", seen)
	}
}
