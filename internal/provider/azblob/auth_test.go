package azblob

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAzureConfigDirUsesAzureProfilesDirEnv(t *testing.T) {
	base := t.TempDir()
	t.Setenv("AZURE_PROFILES_DIR", base)

	got, err := azureConfigDir("client-a")
	if err != nil {
		t.Fatalf("azureConfigDir() error = %v", err)
	}
	want := filepath.Join(base, "client-a")
	if got != want {
		t.Fatalf("azureConfigDir() = %q, want %q", got, want)
	}
}

func TestAzureConfigDirPrefersDotAzureProfiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("AZURE_PROFILES_DIR", "")
	preferred := filepath.Join(home, ".azure-profiles", "client-a")
	if err := os.MkdirAll(preferred, 0o755); err != nil {
		t.Fatalf("mkdir preferred profile: %v", err)
	}

	got, err := azureConfigDir("client-a")
	if err != nil {
		t.Fatalf("azureConfigDir() error = %v", err)
	}
	want := preferred
	if got != want {
		t.Fatalf("azureConfigDir() = %q, want %q", got, want)
	}
}

func TestAzureConfigDirFallsBackToLegacyWhenPreferredProfileMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("AZURE_PROFILES_DIR", "")
	if err := os.MkdirAll(filepath.Join(home, ".azure-profiles"), 0o755); err != nil {
		t.Fatalf("mkdir .azure-profiles root: %v", err)
	}
	legacy := filepath.Join(home, ".azure", "profiles", "legacy")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatalf("mkdir legacy profile: %v", err)
	}

	got, err := azureConfigDir("legacy")
	if err != nil {
		t.Fatalf("azureConfigDir() error = %v", err)
	}
	if got != legacy {
		t.Fatalf("azureConfigDir() = %q, want %q", got, legacy)
	}
}

func TestAzureConfigDirFallsBackToLegacyPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("AZURE_PROFILES_DIR", "")

	got, err := azureConfigDir("legacy")
	if err != nil {
		t.Fatalf("azureConfigDir() error = %v", err)
	}
	want := filepath.Join(home, ".azure", "profiles", "legacy")
	if got != want {
		t.Fatalf("azureConfigDir() = %q, want %q", got, want)
	}
}
