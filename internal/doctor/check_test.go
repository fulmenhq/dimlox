package doctor

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	providerazblob "github.com/fulmenhq/dimlox/internal/provider/azblob"
	providergcs "github.com/fulmenhq/dimlox/internal/provider/gcs"
	"github.com/fulmenhq/dimlox/internal/uri"
)

func TestShouldUseListProbe(t *testing.T) {
	tests := []struct {
		name   string
		target string
		parsed *uri.ParsedURI
		want   bool
	}{
		{
			name:   "gcs bucket without slash uses list probe",
			target: "gs://bucket",
			parsed: &uri.ParsedURI{Provider: uri.ProviderGCS, GCSBucket: "bucket", GCSObject: ""},
			want:   true,
		},
		{
			name:   "gcs bucket with slash uses list probe",
			target: "gs://bucket/",
			parsed: &uri.ParsedURI{Provider: uri.ProviderGCS, GCSBucket: "bucket", GCSObject: ""},
			want:   true,
		},
		{
			name:   "gcs object uses stat probe",
			target: "gs://bucket/object.txt",
			parsed: &uri.ParsedURI{Provider: uri.ProviderGCS, GCSBucket: "bucket", GCSObject: "object.txt"},
			want:   false,
		},
		{
			name:   "azblob container without slash uses list probe",
			target: "azblob://acct/container",
			parsed: &uri.ParsedURI{Provider: uri.ProviderAZBlob, AZAccount: "acct", AZContainer: "container", AZBlobPath: ""},
			want:   true,
		},
		{
			name:   "azblob blob uses stat probe",
			target: "azblob://acct/container/path/file.csv",
			parsed: &uri.ParsedURI{Provider: uri.ProviderAZBlob, AZAccount: "acct", AZContainer: "container", AZBlobPath: "path/file.csv"},
			want:   false,
		},
		{
			name:   "local path uses stat probe",
			target: "/tmp/example",
			parsed: &uri.ParsedURI{Provider: uri.ProviderLocal, LocalPath: "/tmp/example"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldUseListProbe(tt.target, tt.parsed)
			if got != tt.want {
				t.Fatalf("shouldUseListProbe(%q) = %v, want %v", tt.target, got, tt.want)
			}
		})
	}
}

func TestProbeAzureIncludesTokenValidity(t *testing.T) {
	origProbe := probeAzureAuth
	origResolve := resolveAzureProfile
	origNow := nowFunc
	t.Cleanup(func() {
		probeAzureAuth = origProbe
		resolveAzureProfile = origResolve
		nowFunc = origNow
	})

	now := time.Date(2026, 3, 22, 15, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return now }
	resolveAzureProfile = func(string) (*providerazblob.ProfileResolution, error) {
		return &providerazblob.ProfileResolution{Exists: true}, nil
	}
	probeAzureAuth = func(context.Context, string) (*providerazblob.AuthDetails, error) {
		return &providerazblob.AuthDetails{TokenExpiry: now.Add(72 * time.Minute)}, nil
	}

	status := probeAzure(context.Background(), "client-a")
	if !status.OK {
		t.Fatalf("status.OK = false, want true")
	}
	if !strings.Contains(status.Detail, "DefaultAzureCredential token acquired") {
		t.Fatalf("detail = %q, want credential message", status.Detail)
	}
	if !strings.Contains(status.Detail, "az-profile=client-a") {
		t.Fatalf("detail = %q, want profile detail", status.Detail)
	}
	if !strings.Contains(status.Detail, "valid for 1h12m") {
		t.Fatalf("detail = %q, want token validity", status.Detail)
	}
}

func TestProbeAzureReturnsSetupGuidanceWhenProfileMissing(t *testing.T) {
	origResolve := resolveAzureProfile
	t.Cleanup(func() {
		resolveAzureProfile = origResolve
	})

	resolveAzureProfile = func(string) (*providerazblob.ProfileResolution, error) {
		return &providerazblob.ProfileResolution{
			Candidates: []string{"/home/test/.azure-profiles/client-a", "/home/test/.azure/profiles/client-a"},
			Resolved:   "/home/test/.azure/profiles/client-a",
			Exists:     false,
		}, nil
	}

	status := probeAzure(context.Background(), "client-a")
	if status.OK {
		t.Fatalf("status.OK = true, want false")
	}
	if status.Kind != "setup" {
		t.Fatalf("status.Kind = %q, want setup", status.Kind)
	}
	if !strings.Contains(status.Detail, `az-profile "client-a" not found`) {
		t.Fatalf("detail = %q, want profile guidance", status.Detail)
	}
	if !strings.Contains(status.Detail, "Then retry: dimlox doctor --az-profile client-a") {
		t.Fatalf("detail = %q, want retry hint", status.Detail)
	}
}

func TestProbeAzureReturnsSetupGuidanceUsingResolvedProfileDir(t *testing.T) {
	origResolve := resolveAzureProfile
	t.Cleanup(func() {
		resolveAzureProfile = origResolve
	})

	resolveAzureProfile = func(string) (*providerazblob.ProfileResolution, error) {
		return &providerazblob.ProfileResolution{
			Candidates: []string{"/custom/profiles/client-a"},
			Resolved:   "/custom/profiles/client-a",
			Exists:     false,
		}, nil
	}

	status := probeAzure(context.Background(), "client-a")
	if status.OK {
		t.Fatalf("status.OK = true, want false")
	}
	if !strings.Contains(status.Detail, "export AZURE_CONFIG_DIR=\"/custom/profiles/client-a\"") {
		t.Fatalf("detail = %q, want resolved profile directory guidance", status.Detail)
	}
	if strings.Contains(status.Detail, ".azure-profiles/client-a") {
		t.Fatalf("detail = %q, do not want hardcoded default profile path", status.Detail)
	}
}

func TestProbeAzureReturnsLoginGuidanceWhenCLIProfileNotLoggedIn(t *testing.T) {
	origResolve := resolveAzureProfile
	origProbe := probeAzureAuth
	t.Cleanup(func() {
		resolveAzureProfile = origResolve
		probeAzureAuth = origProbe
	})

	resolveAzureProfile = func(string) (*providerazblob.ProfileResolution, error) {
		return &providerazblob.ProfileResolution{
			Candidates: []string{"/home/test/.azure-profiles/client-a"},
			Resolved:   "/home/test/.azure-profiles/client-a",
			Exists:     true,
		}, nil
	}
	probeAzureAuth = func(context.Context, string) (*providerazblob.AuthDetails, error) {
		return nil, errors.New("DefaultAzureCredential: failed to acquire a token. AzureCLICredential: ERROR: Please run 'az login' to setup account.")
	}

	status := probeAzure(context.Background(), "client-a")
	if status.OK {
		t.Fatalf("status.OK = true, want false")
	}
	if status.Kind != "setup" {
		t.Fatalf("status.Kind = %q, want setup", status.Kind)
	}
	if !strings.Contains(status.Detail, "Azure CLI profile not logged in") {
		t.Fatalf("detail = %q, want login guidance", status.Detail)
	}
	if !strings.Contains(status.Detail, "AZURE_CONFIG_DIR") {
		t.Fatalf("detail = %q, want AZURE_CONFIG_DIR guidance", status.Detail)
	}
	if !strings.Contains(status.Detail, "az login") {
		t.Fatalf("detail = %q, want az login guidance", status.Detail)
	}
}

func TestProbeAzureReturnsDefaultLoginGuidanceWithoutProfilePath(t *testing.T) {
	origProbe := probeAzureAuth
	t.Cleanup(func() {
		probeAzureAuth = origProbe
	})

	probeAzureAuth = func(context.Context, string) (*providerazblob.AuthDetails, error) {
		return nil, errors.New("DefaultAzureCredential: failed to acquire a token. AzureCLICredential: ERROR: Please run 'az login' to setup account.")
	}

	status := probeAzure(context.Background(), "")
	if status.OK {
		t.Fatalf("status.OK = true, want false")
	}
	if !strings.Contains(status.Detail, "Then retry: dimlox doctor") {
		t.Fatalf("detail = %q, want default retry hint", status.Detail)
	}
	if strings.Contains(status.Detail, ".azure-profiles") {
		t.Fatalf("detail = %q, do not want profile-directory guidance for default Azure CLI login", status.Detail)
	}
	if strings.Contains(status.Detail, "--az-profile") {
		t.Fatalf("detail = %q, do not want az-profile retry guidance", status.Detail)
	}
}

func TestProbeGCSIncludesTokenValidity(t *testing.T) {
	origProbe := probeGCSAuth
	origDescribe := describeGCSAuthSource
	origNow := nowFunc
	t.Cleanup(func() {
		probeGCSAuth = origProbe
		describeGCSAuthSource = origDescribe
		nowFunc = origNow
	})

	now := time.Date(2026, 3, 22, 15, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return now }
	probeGCSAuth = func(context.Context, providergcs.Options) (*providergcs.AuthDetails, error) {
		return &providergcs.AuthDetails{TokenExpiry: now.Add(45 * time.Minute)}, nil
	}
	describeGCSAuthSource = func(providergcs.Options) (string, error) {
		return "ADC via local ADC file (~/.config/gcloud/application_default_credentials.json), quota-project=<none>", nil
	}

	status := probeGCS(context.Background(), "", "")
	if !status.OK {
		t.Fatalf("status.OK = false, want true")
	}
	if !strings.Contains(status.Detail, "ADC via local ADC file") {
		t.Fatalf("detail = %q, want auth source detail", status.Detail)
	}
	if !strings.Contains(status.Detail, "valid for 45m") {
		t.Fatalf("detail = %q, want token validity", status.Detail)
	}
}

func TestProbeGCSFallsBackWhenDescribeFails(t *testing.T) {
	origProbe := probeGCSAuth
	origDescribe := describeGCSAuthSource
	origNow := nowFunc
	t.Cleanup(func() {
		probeGCSAuth = origProbe
		describeGCSAuthSource = origDescribe
		nowFunc = origNow
	})

	now := time.Date(2026, 3, 22, 15, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return now }
	probeGCSAuth = func(context.Context, providergcs.Options) (*providergcs.AuthDetails, error) {
		return &providergcs.AuthDetails{TokenExpiry: now.Add(2 * time.Hour)}, nil
	}
	describeGCSAuthSource = func(providergcs.Options) (string, error) {
		return "", errors.New("unavailable")
	}

	status := probeGCS(context.Background(), "", "")
	if status.Detail != "ADC token acquired (valid for 2h)" {
		t.Fatalf("detail = %q, want fallback detail with token validity", status.Detail)
	}
}

func TestRunTargetedAzureReturnsSetupStatusWhenProfileMissing(t *testing.T) {
	origResolve := resolveAzureProfile
	t.Cleanup(func() {
		resolveAzureProfile = origResolve
	})

	resolveAzureProfile = func(string) (*providerazblob.ProfileResolution, error) {
		return &providerazblob.ProfileResolution{
			Candidates: []string{"/tmp/.azure-profiles/client-a", "/tmp/.azure/profiles/client-a"},
			Resolved:   "/tmp/.azure/profiles/client-a",
			Exists:     false,
		}, nil
	}

	result, err := Run(context.Background(), "azblob://acct/container/blob.txt", Options{AZProfile: "client-a", Version: "v0.1.0"})
	if err == nil {
		t.Fatal("Run() error = nil, want doctor checks failed")
	}
	if result == nil || result.Status == nil {
		t.Fatalf("result.Status = nil, want setup status")
	}
	if result.Status.Kind != "setup" {
		t.Fatalf("result.Status.Kind = %q, want setup", result.Status.Kind)
	}
	if result.ProviderName != "azblob" {
		t.Fatalf("result.ProviderName = %q, want azblob", result.ProviderName)
	}
	if !strings.Contains(result.Status.Detail, `az-profile "client-a" not found`) {
		t.Fatalf("detail = %q, want profile guidance", result.Status.Detail)
	}
}

func TestRunTargetedAzureReturnsLoginGuidanceWhenProfileNeedsLogin(t *testing.T) {
	origResolve := resolveAzureProfile
	origProbe := probeAzureAuth
	t.Cleanup(func() {
		resolveAzureProfile = origResolve
		probeAzureAuth = origProbe
	})

	resolveAzureProfile = func(string) (*providerazblob.ProfileResolution, error) {
		return &providerazblob.ProfileResolution{
			Candidates: []string{"/tmp/.azure-profiles/client-a"},
			Resolved:   "/tmp/.azure-profiles/client-a",
			Exists:     true,
		}, nil
	}
	probeAzureAuth = func(context.Context, string) (*providerazblob.AuthDetails, error) {
		return nil, errors.New("AzureCLICredential: ERROR: Please run 'az login' to setup account.")
	}

	result, err := Run(context.Background(), "azblob://acct/container/blob.txt", Options{AZProfile: "client-a", Version: "v0.1.0"})
	if err == nil {
		t.Fatal("Run() error = nil, want doctor checks failed")
	}
	if result == nil || result.Status == nil {
		t.Fatalf("result.Status = nil, want setup status")
	}
	if result.Status.Kind != "setup" {
		t.Fatalf("result.Status.Kind = %q, want setup", result.Status.Kind)
	}
	if !strings.Contains(result.Status.Detail, "Azure CLI profile not logged in") {
		t.Fatalf("detail = %q, want login guidance", result.Status.Detail)
	}
	if !strings.Contains(result.Status.Detail, "AZURE_CONFIG_DIR") {
		t.Fatalf("detail = %q, want AZURE_CONFIG_DIR guidance", result.Status.Detail)
	}
}

func TestRunUntargetedScopesToAzureWhenAZProfileProvided(t *testing.T) {
	origResolve := resolveAzureProfile
	origProbe := probeAzureAuth
	origGCS := probeGCSAuth
	t.Cleanup(func() {
		resolveAzureProfile = origResolve
		probeAzureAuth = origProbe
		probeGCSAuth = origGCS
	})

	resolveAzureProfile = func(string) (*providerazblob.ProfileResolution, error) {
		return &providerazblob.ProfileResolution{Exists: true}, nil
	}
	probeAzureAuth = func(context.Context, string) (*providerazblob.AuthDetails, error) {
		return &providerazblob.AuthDetails{}, nil
	}
	probeGCSAuth = func(context.Context, providergcs.Options) (*providergcs.AuthDetails, error) {
		t.Fatal("probeGCSAuth should not be called when az-profile scopes doctor to Azure")
		return nil, nil
	}

	result, err := Run(context.Background(), "", Options{AZProfile: "client-a", Version: "v0.1.0"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := len(result.Statuses), 2; got != want {
		t.Fatalf("len(Statuses) = %d, want %d", got, want)
	}
	if result.Statuses[1].Provider != "azblob" {
		t.Fatalf("providers = %#v, want local + azblob", result.Statuses)
	}
}

func TestFormatTokenValidity(t *testing.T) {
	now := time.Date(2026, 3, 22, 15, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		expiresAt time.Time
		want      string
	}{
		{name: "zero", expiresAt: time.Time{}, want: ""},
		{name: "minutes", expiresAt: now.Add(45 * time.Minute), want: "valid for 45m"},
		{name: "hours and minutes", expiresAt: now.Add(72 * time.Minute), want: "valid for 1h12m"},
		{name: "exact hour", expiresAt: now.Add(2 * time.Hour), want: "valid for 2h"},
		{name: "expired", expiresAt: now.Add(-1 * time.Minute), want: "token expired"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatTokenValidity(tt.expiresAt, now); got != tt.want {
				t.Fatalf("formatTokenValidity() = %q, want %q", got, tt.want)
			}
		})
	}
}
