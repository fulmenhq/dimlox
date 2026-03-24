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
	"sort"
	"strings"
	"time"

	storageapi "cloud.google.com/go/storage"
	"github.com/fulmenhq/dimlox/internal/appctx"
	"go.uber.org/zap"
	"golang.org/x/oauth2/google"
)

var ErrADCMissing = errors.New("application default credentials not found")
var ErrProfileNotFound = errors.New("gcloud profile not found")

const metadataFlavorGoogle = "Google"

type Options struct {
	Project   string
	Profile   string
	CredsFile string
}

type Profile struct {
	Name                   string
	Account                string
	Project                string
	CredentialFileOverride string
	ConfigDir              string
}

type ProfileList struct {
	ConfigDir string
	Profiles  []Profile
}

type ResolvedOptions struct {
	Project          string
	CredsFile        string
	Profile          *Profile
	IdentityOverride bool
}

type AuthDetails struct {
	Source           AuthSource
	Path             string
	QuotaProject     string
	TokenExpiry      time.Time
	ResolvedProject  string
	Profile          *Profile
	IdentityOverride bool
}

type AuthSource string

const (
	AuthSourceExplicitCreds AuthSource = "credential file"
	AuthSourceEnvVar        AuthSource = "GOOGLE_APPLICATION_CREDENTIALS"
	AuthSourceLocalADC      AuthSource = "local ADC file"
	AuthSourceMetadata      AuthSource = "metadata server"
)

var probeGCSTokenExpiry = func(ctx context.Context, opts Options) (time.Time, error) {
	return tokenExpiryForOptions(ctx, opts)
}

func ProbeAuth(ctx context.Context, opts Options) (*AuthDetails, error) {
	details, err := detectAuthDetails(http.DefaultClient, opts)
	if err != nil {
		return nil, err
	}
	expiry, err := probeGCSTokenExpiry(ctx, opts)
	if err != nil {
		return nil, err
	}
	details.TokenExpiry = expiry
	return details, nil
}

func DescribeAuthSource(opts Options) (string, error) {
	details, err := detectAuthDetails(http.DefaultClient, opts)
	if err != nil {
		return "", err
	}
	if details.Profile != nil {
		project := details.ResolvedProject
		if project == "" {
			project = "<none>"
		}
		identity := describeProfileIdentity(details)
		return fmt.Sprintf("ADC token acquired (profile: %s, identity: %s, project: %s)", details.Profile.Name, identity, project), nil
	}
	detail := "ADC token acquired"
	switch details.Source {
	case AuthSourceExplicitCreds, AuthSourceEnvVar:
		detail = fmt.Sprintf("ADC via %s=%s", details.Source, describeCredentialPath(details.Path))
	case AuthSourceLocalADC:
		detail = fmt.Sprintf("ADC via %s (%s)", details.Source, describeCredentialPath(details.Path))
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

func ResolveOptions(opts Options) (*ResolvedOptions, error) {
	resolved := &ResolvedOptions{}
	if opts.Profile != "" {
		profile, err := ResolveProfile(opts.Profile)
		if err != nil {
			return nil, err
		}
		resolved.Profile = profile
		if profile.CredentialFileOverride != "" {
			resolved.CredsFile = profile.CredentialFileOverride
			resolved.IdentityOverride = true
		}
		if profile.Project != "" {
			resolved.Project = profile.Project
		}
	}
	if opts.CredsFile != "" {
		resolvedPath, err := validatedCredentialFile(opts.CredsFile)
		if err != nil {
			return nil, err
		}
		resolved.CredsFile = resolvedPath
		resolved.IdentityOverride = true
	} else if resolved.CredsFile != "" {
		resolvedPath, err := validatedCredentialFile(resolved.CredsFile)
		if err != nil {
			return nil, err
		}
		resolved.CredsFile = resolvedPath
	}
	if opts.Project != "" {
		resolved.Project = opts.Project
	} else if resolved.Project == "" {
		resolved.Project = defaultProjectFromEnv()
	}
	return resolved, nil
}

func ResolveProfile(name string) (*Profile, error) {
	configDir, err := gcloudConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(configDir, "configurations", "config_"+name)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w %q in %s", ErrProfileNotFound, name, filepath.ToSlash(path))
		}
		return nil, err
	}
	profile, err := parseProfileFile(path, name, configDir)
	if err != nil {
		return nil, err
	}
	return profile, nil
}

func ListProfiles() (*ProfileList, error) {
	configDir, err := gcloudConfigDir()
	if err != nil {
		return nil, err
	}
	configurationsDir := filepath.Join(configDir, "configurations")
	entries, err := os.ReadDir(configurationsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &ProfileList{ConfigDir: configDir}, nil
		}
		return nil, err
	}
	profiles := make([]Profile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "config_") {
			continue
		}
		name := strings.TrimPrefix(entry.Name(), "config_")
		profile, err := parseProfileFile(filepath.Join(configurationsDir, entry.Name()), name, configDir)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, *profile)
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})
	return &ProfileList{ConfigDir: configDir, Profiles: profiles}, nil
}

func LogProfileContext(ctx context.Context, opts Options) error {
	resolved, err := ResolveOptions(opts)
	if err != nil {
		return err
	}
	if resolved.Profile == nil || resolved.IdentityOverride {
		return nil
	}
	log := appctx.Logger(ctx)
	if log == nil {
		return nil
	}
	log.Info(fmt.Sprintf("gcp profile %q: no credential_file_override; ADC identity unchanged", resolved.Profile.Name),
		zap.String("profile", resolved.Profile.Name),
		zap.String("project", resolved.Project),
	)
	return nil
}

func detectAuthSource(client *http.Client, opts Options) (AuthSource, error) {
	details, err := detectAuthDetails(client, opts)
	if err != nil {
		return "", err
	}
	return details.Source, nil
}

func detectAuthDetails(client *http.Client, opts Options) (*AuthDetails, error) {
	resolved, err := ResolveOptions(opts)
	if err != nil {
		return nil, err
	}
	if resolved.CredsFile != "" {
		quotaProject, err := quotaProjectForFile(resolved.CredsFile)
		if err != nil {
			return nil, err
		}
		return &AuthDetails{
			Source:           AuthSourceExplicitCreds,
			Path:             resolved.CredsFile,
			QuotaProject:     quotaProject,
			ResolvedProject:  resolved.Project,
			Profile:          resolved.Profile,
			IdentityOverride: true,
		}, nil
	}

	if explicit := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return nil, fmt.Errorf("%w: GOOGLE_APPLICATION_CREDENTIALS is set but not usable as a readable credentials file", ErrADCMissing)
		}
		quotaProject, err := quotaProjectForFile(explicit)
		if err != nil {
			return nil, err
		}
		return &AuthDetails{Source: AuthSourceEnvVar, Path: resolvedCredentialPath(explicit), QuotaProject: quotaProject, ResolvedProject: resolved.Project, Profile: resolved.Profile}, nil
	}

	if adcPath, ok := defaultADCCredentialsPath(); ok {
		if _, err := os.Stat(adcPath); err == nil {
			quotaProject, err := quotaProjectForFile(adcPath)
			if err != nil {
				return nil, err
			}
			return &AuthDetails{Source: AuthSourceLocalADC, Path: adcPath, QuotaProject: quotaProject, ResolvedProject: resolved.Project, Profile: resolved.Profile}, nil
		}
	}

	if metadataServerAvailable(client) {
		return &AuthDetails{Source: AuthSourceMetadata, ResolvedProject: resolved.Project, Profile: resolved.Profile}, nil
	}

	return nil, fmt.Errorf("%w: set GOOGLE_APPLICATION_CREDENTIALS or run `gcloud auth application-default login`", ErrADCMissing)
}

func tokenExpiryForOptions(ctx context.Context, opts Options) (time.Time, error) {
	resolved, err := ResolveOptions(opts)
	if err != nil {
		return time.Time{}, err
	}
	if resolved.CredsFile != "" {
		data, err := os.ReadFile(resolved.CredsFile)
		if err != nil {
			return time.Time{}, fmt.Errorf("read credentials file %q: %w", resolved.CredsFile, err)
		}
		creds, err := google.CredentialsFromJSON(ctx, data, storageapi.ScopeReadOnly)
		if err != nil {
			return time.Time{}, fmt.Errorf("load credentials file %q: %w", resolved.CredsFile, err)
		}
		token, err := creds.TokenSource.Token()
		if err != nil {
			return time.Time{}, err
		}
		return token.Expiry, nil
	}
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

func describeProfileIdentity(details *AuthDetails) string {
	if details == nil {
		return "default ADC"
	}
	if details.IdentityOverride {
		return describeCredentialPath(details.Path)
	}
	switch details.Source {
	case AuthSourceEnvVar:
		return describeCredentialPath(details.Path)
	default:
		return "default ADC"
	}
}

func describeCredentialPath(path string) string {
	if path == "" {
		return "<redacted>"
	}
	home, err := effectiveHomeDir()
	if err == nil && home != "" {
		rel, relErr := filepath.Rel(home, path)
		switch {
		case relErr != nil:
		case rel == ".":
			return "~"
		case rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)):
			return "~/" + filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(path)
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
	configDir, err := gcloudConfigDir()
	if err != nil || configDir == "" {
		return "", false
	}
	return filepath.Join(configDir, "application_default_credentials.json"), true
}

func gcloudConfigDir() (string, error) {
	if dir := os.Getenv("CLOUDSDK_CONFIG"); dir != "" {
		return dir, nil
	}
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, "gcloud"), nil
		}
		home, err := effectiveHomeDir()
		if err != nil || home == "" {
			return "", err
		}
		return filepath.Join(home, "AppData", "Roaming", "gcloud"), nil
	}
	home, err := effectiveHomeDir()
	if err != nil || home == "" {
		return "", err
	}
	return filepath.Join(home, ".config", "gcloud"), nil
}

func effectiveHomeDir() (string, error) {
	if home := os.Getenv("HOME"); home != "" {
		return home, nil
	}
	return os.UserHomeDir()
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

func parseProfileFile(path, name, configDir string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	section := ""
	profile := &Profile{Name: name, ConfigDir: configDir}
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch section + "/" + key {
		case "core/project":
			profile.Project = value
		case "core/account":
			profile.Account = value
		case "auth/credential_file_override":
			if value != "" && !filepath.IsAbs(value) {
				value = filepath.Join(configDir, value)
			}
			profile.CredentialFileOverride = resolvedCredentialPath(value)
		}
	}
	return profile, nil
}

func validatedCredentialFile(path string) (string, error) {
	resolved := resolvedCredentialPath(path)
	if _, err := os.Stat(resolved); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("credential file not found: %s", resolved)
		}
		return "", err
	}
	return resolved, nil
}

func defaultProjectFromEnv() string {
	if project := os.Getenv("GCLOUD_PROJECT"); project != "" {
		return project
	}
	return os.Getenv("GOOGLE_CLOUD_PROJECT")
}
