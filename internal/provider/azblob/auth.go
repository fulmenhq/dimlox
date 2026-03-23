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
	if base := os.Getenv("AZURE_PROFILES_DIR"); base != "" {
		return filepath.Join(base, profile), nil
	}
	home, err := effectiveHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir for az-profile: %w", err)
	}
	preferred := filepath.Join(home, ".azure-profiles", profile)
	if _, err := os.Stat(preferred); err == nil {
		return preferred, nil
	}
	return filepath.Join(home, ".azure", "profiles", profile), nil
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

func ProbeAuth(ctx context.Context, profile string) (*AuthDetails, error) {
	token, err := getAzureAccessToken(ctx, profile)
	if err != nil {
		return nil, err
	}
	return &AuthDetails{TokenExpiry: token.ExpiresOn}, nil
}
