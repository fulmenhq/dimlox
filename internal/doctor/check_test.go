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
	origNow := nowFunc
	t.Cleanup(func() {
		probeAzureAuth = origProbe
		nowFunc = origNow
	})

	now := time.Date(2026, 3, 22, 15, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return now }
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
	probeGCSAuth = func(context.Context) (*providergcs.AuthDetails, error) {
		return &providergcs.AuthDetails{TokenExpiry: now.Add(45 * time.Minute)}, nil
	}
	describeGCSAuthSource = func() (string, error) {
		return "ADC via local ADC file (~/.config/gcloud/application_default_credentials.json), quota-project=<none>", nil
	}

	status := probeGCS(context.Background())
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
	probeGCSAuth = func(context.Context) (*providergcs.AuthDetails, error) {
		return &providergcs.AuthDetails{TokenExpiry: now.Add(2 * time.Hour)}, nil
	}
	describeGCSAuthSource = func() (string, error) {
		return "", errors.New("unavailable")
	}

	status := probeGCS(context.Background())
	if status.Detail != "ADC token acquired (valid for 2h)" {
		t.Fatalf("detail = %q, want fallback detail with token validity", status.Detail)
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
