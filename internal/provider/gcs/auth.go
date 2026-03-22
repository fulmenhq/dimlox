package gcs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	storageapi "cloud.google.com/go/storage"
	"golang.org/x/oauth2/google"
)

var ErrADCMissing = errors.New("application default credentials not found")

const metadataFlavorGoogle = "Google"

type AuthDetails struct {
	Source       AuthSource
	Path         string
	QuotaProject string
	TokenExpiry  time.Time
}

type AuthSource string

const (
	AuthSourceEnvVar   AuthSource = "GOOGLE_APPLICATION_CREDENTIALS"
	AuthSourceLocalADC AuthSource = "local ADC file"
	AuthSourceMetadata AuthSource = "metadata server"
)

var probeGCSTokenExpiry = func(ctx context.Context) (time.Time, error) {
	creds, err := google.FindDefaultCredentials(ctx, storageapi.ScopeReadOnly)
	if err != nil {
		return time.Time{}, err
	}
	token, err := creds.TokenSource.Token()
	if err != nil {
		return time.Time{}, err
	}
	return token.Expiry, nil
}

func ProbeAuth(ctx context.Context) (*AuthDetails, error) {
	details, err := detectAuthDetails(http.DefaultClient)
	if err != nil {
		return nil, err
	}
	expiry, err := probeGCSTokenExpiry(ctx)
	if err != nil {
		return nil, err
	}
	details.TokenExpiry = expiry
	return details, nil
}

func DescribeAuthSource() (string, error) {
	details, err := detectAuthDetails(http.DefaultClient)
	if err != nil {
		return "", err
	}
	detail := "ADC token acquired"
	switch details.Source {
	case AuthSourceEnvVar:
		detail = fmt.Sprintf("ADC via %s=%s", AuthSourceEnvVar, describeCredentialPath(details.Path))
	case AuthSourceLocalADC:
		detail = fmt.Sprintf("ADC via %s (%s)", AuthSourceLocalADC, describeCredentialPath(details.Path))
	case AuthSourceMetadata:
		detail = "ADC via metadata server"
	}
	if details.QuotaProject != "" {
		detail += fmt.Sprintf(", quota-project=%s", details.QuotaProject)
	} else {
		detail += ", quota-project=<none>"
	}
	return detail, nil
}

func detectAuthSource(client *http.Client) (AuthSource, error) {
	details, err := detectAuthDetails(client)
	if err != nil {
		return "", err
	}
	return details.Source, nil
}

func detectAuthDetails(client *http.Client) (*AuthDetails, error) {
	if explicit := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return nil, fmt.Errorf("%w: GOOGLE_APPLICATION_CREDENTIALS is set but not usable as a readable credentials file", ErrADCMissing)
		}
		quotaProject, err := quotaProjectForFile(explicit)
		if err != nil {
			return nil, err
		}
		return &AuthDetails{Source: AuthSourceEnvVar, Path: resolvedCredentialPath(explicit), QuotaProject: quotaProject}, nil
	}

	if adcPath, ok := defaultADCCredentialsPath(); ok {
		if _, err := os.Stat(adcPath); err == nil {
			quotaProject, err := quotaProjectForFile(adcPath)
			if err != nil {
				return nil, err
			}
			return &AuthDetails{Source: AuthSourceLocalADC, Path: adcPath, QuotaProject: quotaProject}, nil
		}
	}

	if metadataServerAvailable(client) {
		return &AuthDetails{Source: AuthSourceMetadata}, nil
	}

	return nil, fmt.Errorf("%w: set GOOGLE_APPLICATION_CREDENTIALS or run `gcloud auth application-default login`", ErrADCMissing)
}

func describeCredentialPath(path string) string {
	if path == "" {
		return "<redacted>"
	}
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		prefix := home + string(os.PathSeparator)
		if path == home {
			return "~"
		}
		if strings.HasPrefix(path, prefix) {
			return "~" + string(os.PathSeparator) + strings.TrimPrefix(path, prefix)
		}
	}
	return path
}

func resolvedCredentialPath(path string) string {
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func quotaProjectForFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read ADC file %q: %w", path, err)
	}
	var raw struct {
		QuotaProject string `json:"quota_project_id"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("parse ADC file %q: %w", path, err)
	}
	return raw.QuotaProject, nil
}

func defaultADCCredentialsPath() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", false
	}
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", false
		}
		return filepath.Join(appData, "gcloud", "application_default_credentials.json"), true
	}
	return filepath.Join(home, ".config", "gcloud", "application_default_credentials.json"), true
}

func metadataServerAvailable(client *http.Client) bool {
	if client == nil {
		client = http.DefaultClient
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://169.254.169.254/computeMetadata/v1/instance/id", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Metadata-Flavor", metadataFlavorGoogle)

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}
	if resp.Header.Get("Metadata-Flavor") != metadataFlavorGoogle {
		return false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(body)) != ""
}
