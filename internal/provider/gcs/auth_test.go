package gcs

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDetectAuthSourceMissingCredentials(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	t.Setenv("APPDATA", "")

	_, err := detectAuthSource(nil)
	if !errors.Is(err, ErrADCMissing) {
		t.Fatalf("detectAuthSource() error = %v, want ErrADCMissing", err)
	}
}

func TestDetectAuthSourceExplicitCredentialsFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "adc.json")
	if err := os.WriteFile(file, []byte(`{"type":"authorized_user"}`), 0o644); err != nil {
		t.Fatalf("write adc file: %v", err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", file)

	source, err := detectAuthSource(nil)
	if err != nil {
		t.Fatalf("detectAuthSource() error = %v, want nil", err)
	}
	if source != AuthSourceEnvVar {
		t.Fatalf("detectAuthSource() source = %q, want %q", source, AuthSourceEnvVar)
	}
}

func TestDetectAuthSourceExplicitCredentialsFileDoesNotLeakRawEnvValueOnError(t *testing.T) {
	raw := `{"type":"service_account","private_key":"secret-value"}`
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", raw)

	_, err := detectAuthSource(nil)
	if err == nil {
		t.Fatal("detectAuthSource() error = nil, want error")
	}
	if strings.Contains(err.Error(), raw) {
		t.Fatalf("detectAuthSource() error leaked raw env value: %q", err.Error())
	}
}

func TestDetectAuthSourceDefaultCredentialsFile(t *testing.T) {
	home := t.TempDir()
	adcPath, appData := testADCPath(home)
	if err := os.MkdirAll(filepath.Dir(adcPath), 0o755); err != nil {
		t.Fatalf("mkdir adc dir: %v", err)
	}
	if err := os.WriteFile(adcPath, []byte(`{"type":"authorized_user"}`), 0o644); err != nil {
		t.Fatalf("write adc file: %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	t.Setenv("APPDATA", appData)

	source, err := detectAuthSource(nil)
	if err != nil {
		t.Fatalf("detectAuthSource() error = %v, want nil", err)
	}
	if source != AuthSourceLocalADC {
		t.Fatalf("detectAuthSource() source = %q, want %q", source, AuthSourceLocalADC)
	}
}

func TestDescribeAuthSourceExplicitCredentialsFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "service.json")
	if err := os.WriteFile(file, []byte(`{"type":"service_account","quota_project_id":"svc-quota"}`), 0o644); err != nil {
		t.Fatalf("write service file: %v", err)
	}
	t.Chdir(root)
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", file)

	detail, err := DescribeAuthSource()
	if err != nil {
		t.Fatalf("DescribeAuthSource() error = %v", err)
	}
	want := "ADC via GOOGLE_APPLICATION_CREDENTIALS=" + file + ", quota-project=svc-quota"
	if detail != want {
		t.Fatalf("DescribeAuthSource() = %q, want %q", detail, want)
	}
}

func TestDescribeAuthSourceExplicitCredentialsFileResolvesRelativePathAndBlindsHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("APPDATA", "")
	serviceDir := filepath.Join(home, "creds")
	if err := os.MkdirAll(serviceDir, 0o755); err != nil {
		t.Fatalf("mkdir creds dir: %v", err)
	}
	file := filepath.Join(serviceDir, "service.json")
	if err := os.WriteFile(file, []byte(`{"type":"service_account"}`), 0o644); err != nil {
		t.Fatalf("write service file: %v", err)
	}
	t.Chdir(home)
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "./creds/service.json")

	detail, err := DescribeAuthSource()
	if err != nil {
		t.Fatalf("DescribeAuthSource() error = %v", err)
	}
	want := "ADC via GOOGLE_APPLICATION_CREDENTIALS=~/creds/service.json, quota-project=<none>"
	if detail != want {
		t.Fatalf("DescribeAuthSource() = %q, want %q", detail, want)
	}
}

func TestDescribeAuthSourceLocalADCIncludesQuotaProject(t *testing.T) {
	home := t.TempDir()
	adcPath, appData := testADCPath(home)
	if err := os.MkdirAll(filepath.Dir(adcPath), 0o755); err != nil {
		t.Fatalf("mkdir adc dir: %v", err)
	}
	if err := os.WriteFile(adcPath, []byte(`{"type":"authorized_user","quota_project_id":"local-quota"}`), 0o644); err != nil {
		t.Fatalf("write adc file: %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	t.Setenv("APPDATA", appData)

	detail, err := DescribeAuthSource()
	if err != nil {
		t.Fatalf("DescribeAuthSource() error = %v", err)
	}
	want := "ADC via local ADC file (" + testADCDisplayPath() + "), quota-project=local-quota"
	if detail != want {
		t.Fatalf("DescribeAuthSource() = %q, want %q", detail, want)
	}
}

func TestProbeAuthIncludesTokenExpiry(t *testing.T) {
	origProbe := probeGCSTokenExpiry
	t.Cleanup(func() { probeGCSTokenExpiry = origProbe })

	wantExpiry := time.Date(2026, 3, 22, 16, 45, 0, 0, time.UTC)
	probeGCSTokenExpiry = func(context.Context) (time.Time, error) {
		return wantExpiry, nil
	}

	root := t.TempDir()
	file := filepath.Join(root, "service.json")
	if err := os.WriteFile(file, []byte(`{"type":"service_account","quota_project_id":"svc-quota"}`), 0o644); err != nil {
		t.Fatalf("write service file: %v", err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", file)

	details, err := ProbeAuth(context.Background())
	if err != nil {
		t.Fatalf("ProbeAuth() error = %v", err)
	}
	if details == nil || details.Source != AuthSourceEnvVar {
		t.Fatalf("ProbeAuth() source = %v, want %v", details.Source, AuthSourceEnvVar)
	}
	if !details.TokenExpiry.Equal(wantExpiry) {
		t.Fatalf("ProbeAuth() expiry = %v, want %v", details.TokenExpiry, wantExpiry)
	}
}

func TestMetadataServerAvailableRequiresGoogleResponse(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("instance-id")),
		}, nil
	})}

	if metadataServerAvailable(client) {
		t.Fatal("metadataServerAvailable() = true, want false for non-Google response")
	}
}

func TestMetadataServerAvailableAcceptsGoogleResponse(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		headers := make(http.Header)
		headers.Set("Metadata-Flavor", metadataFlavorGoogle)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader("1234567890")),
		}, nil
	})}

	if !metadataServerAvailable(client) {
		t.Fatal("metadataServerAvailable() = false, want true for Google metadata response")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testADCPath(home string) (path string, appData string) {
	if runtime.GOOS == "windows" {
		appData = filepath.Join(home, "AppData", "Roaming")
		return filepath.Join(appData, "gcloud", "application_default_credentials.json"), appData
	}
	return filepath.Join(home, ".config", "gcloud", "application_default_credentials.json"), ""
}

func testADCDisplayPath() string {
	if runtime.GOOS == "windows" {
		return "~/AppData/Roaming/gcloud/application_default_credentials.json"
	}
	return "~/.config/gcloud/application_default_credentials.json"
}
