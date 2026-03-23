package azblob

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

const storageScope = "https://storage.azure.com/.default"

type AuthDetails struct {
	TokenExpiry time.Time
}

type ProfileResolution struct {
	Candidates []string
	Resolved   string
	Exists     bool
}

var getAzureAccessToken = func(ctx context.Context, profile string) (azcore.AccessToken, error) {
	cred, err := newCredential(profile)
	if err != nil {
		return azcore.AccessToken{}, err
	}
	return cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{storageScope}})
}

func applyAZProfile(profile string) error {
	if profile == "" {
		return nil
	}
	// AZURE_CONFIG_DIR is process-global. This is acceptable for the current
	// single-profile command model, but Phase 2 multi-endpoint Azure workflows
	// must avoid constructing providers with different profiles concurrently.
	configDir, err := azureConfigDir(profile)
	if err != nil {
		return err
	}
	if err := os.Setenv("AZURE_CONFIG_DIR", configDir); err != nil {
		return fmt.Errorf("set AZURE_CONFIG_DIR: %w", err)
	}
	return nil
}

func azureConfigDir(profile string) (string, error) {
	resolution, err := ResolveProfile(profile)
	if err != nil {
		return "", err
	}
	return resolution.Resolved, nil
}

func ResolveProfile(profile string) (*ProfileResolution, error) {
	if profile == "" {
		return &ProfileResolution{}, nil
	}
	if base := os.Getenv("AZURE_PROFILES_DIR"); base != "" {
		resolved := filepath.Join(base, profile)
		exists, err := pathExists(resolved)
		if err != nil {
			return nil, err
		}
		return &ProfileResolution{
			Candidates: []string{resolved},
			Resolved:   resolved,
			Exists:     exists,
		}, nil
	}
	home, err := effectiveHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir for az-profile: %w", err)
	}
	preferred := filepath.Join(home, ".azure-profiles", profile)
	legacy := filepath.Join(home, ".azure", "profiles", profile)

	exists, err := pathExists(preferred)
	if err != nil {
		return nil, err
	}
	if exists {
		return &ProfileResolution{
			Candidates: []string{preferred, legacy},
			Resolved:   preferred,
			Exists:     true,
		}, nil
	}
	exists, err = pathExists(legacy)
	if err != nil {
		return nil, err
	}
	return &ProfileResolution{
		Candidates: []string{preferred, legacy},
		Resolved:   legacy,
		Exists:     exists,
	}, nil
}

func effectiveHomeDir() (string, error) {
	if home := os.Getenv("HOME"); home != "" {
		return home, nil
	}
	return os.UserHomeDir()
}

func newCredential(profile string) (azcore.TokenCredential, error) {
	if err := applyAZProfile(profile); err != nil {
		return nil, err
	}
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}
	return cred, nil
}

func pathExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return info.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func ProbeAuth(ctx context.Context, profile string) (*AuthDetails, error) {
	token, err := getAzureAccessToken(ctx, profile)
	if err != nil {
		return nil, err
	}
	return &AuthDetails{TokenExpiry: token.ExpiresOn}, nil
}
