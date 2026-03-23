package azblob

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
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

func TestResolveProfileUsesAzureProfilesDirEnv(t *testing.T) {
	base := t.TempDir()
	t.Setenv("AZURE_PROFILES_DIR", base)

	resolution, err := ResolveProfile("client-a")
	if err != nil {
		t.Fatalf("ResolveProfile() error = %v", err)
	}
	want := filepath.Join(base, "client-a")
	if len(resolution.Candidates) != 1 || resolution.Candidates[0] != want {
		t.Fatalf("Candidates = %#v, want [%q]", resolution.Candidates, want)
	}
	if resolution.Resolved != want {
		t.Fatalf("Resolved = %q, want %q", resolution.Resolved, want)
	}
	if resolution.Exists {
		t.Fatal("Exists = true, want false")
	}
}

func TestResolveProfilePrefersExistingDotAzureProfiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("AZURE_PROFILES_DIR", "")
	preferred := filepath.Join(home, ".azure-profiles", "client-a")
	if err := os.MkdirAll(preferred, 0o755); err != nil {
		t.Fatalf("mkdir preferred profile: %v", err)
	}

	resolution, err := ResolveProfile("client-a")
	if err != nil {
		t.Fatalf("ResolveProfile() error = %v", err)
	}
	if !resolution.Exists {
		t.Fatal("Exists = false, want true")
	}
	if resolution.Resolved != preferred {
		t.Fatalf("Resolved = %q, want %q", resolution.Resolved, preferred)
	}
	if got, want := len(resolution.Candidates), 2; got != want {
		t.Fatalf("len(Candidates) = %d, want %d", got, want)
	}
}

func TestResolveProfileFallsBackToLegacyCandidate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("AZURE_PROFILES_DIR", "")
	legacy := filepath.Join(home, ".azure", "profiles", "legacy")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatalf("mkdir legacy profile: %v", err)
	}

	resolution, err := ResolveProfile("legacy")
	if err != nil {
		t.Fatalf("ResolveProfile() error = %v", err)
	}
	if !resolution.Exists {
		t.Fatal("Exists = false, want true")
	}
	if resolution.Resolved != legacy {
		t.Fatalf("Resolved = %q, want %q", resolution.Resolved, legacy)
	}
}

func TestProbeAuthReturnsTokenExpiry(t *testing.T) {
	origGetToken := getAzureAccessToken
	t.Cleanup(func() { getAzureAccessToken = origGetToken })

	wantExpiry := time.Date(2026, 3, 22, 16, 30, 0, 0, time.UTC)
	getAzureAccessToken = func(context.Context, string) (azcore.AccessToken, error) {
		return azcore.AccessToken{Token: "redacted", ExpiresOn: wantExpiry}, nil
	}

	details, err := ProbeAuth(context.Background(), "client-a")
	if err != nil {
		t.Fatalf("ProbeAuth() error = %v", err)
	}
	if details == nil || !details.TokenExpiry.Equal(wantExpiry) {
		t.Fatalf("ProbeAuth() expiry = %v, want %v", details.TokenExpiry, wantExpiry)
	}
}
