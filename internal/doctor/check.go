package doctor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fulmenhq/dimlox/internal/provider"
	providerazblob "github.com/fulmenhq/dimlox/internal/provider/azblob"
	providergcs "github.com/fulmenhq/dimlox/internal/provider/gcs"
	"github.com/fulmenhq/dimlox/internal/providers"
	"github.com/fulmenhq/dimlox/internal/uri"
)

var (
	probeAzureAuth        = providerazblob.ProbeAuth
	probeGCSAuth          = providergcs.ProbeAuth
	describeGCSAuthSource = providergcs.DescribeAuthSource
	resolveAzureProfile   = providerazblob.ResolveProfile
	nowFunc               = time.Now
)

type Options struct {
	AZProfile  string
	GCPProfile string
	GCPProject string
	Version    string
}

type Status struct {
	Provider string `json:"provider"`
	OK       bool   `json:"ok"`
	Kind     string `json:"kind,omitempty"`
	Detail   string `json:"detail"`
}

type Result struct {
	Statuses     []Status             `json:"statuses,omitempty"`
	Status       *Status              `json:"status,omitempty"`
	Target       string               `json:"target,omitempty"`
	Normalized   string               `json:"normalized,omitempty"`
	ProbeLatency time.Duration        `json:"probe_latency,omitempty"`
	Meta         *provider.ObjectMeta `json:"meta,omitempty"`
	ProviderName string               `json:"provider_name,omitempty"`
	GoVersion    string               `json:"go_version,omitempty"`
	AppVersion   string               `json:"app_version,omitempty"`
	OSArch       string               `json:"os_arch,omitempty"`
}

func Run(ctx context.Context, target string, opts Options) (*Result, error) {
	result := &Result{GoVersion: runtime.Version(), AppVersion: opts.Version, OSArch: runtime.GOOS + "/" + runtime.GOARCH}
	if target == "" {
		statuses := []Status{probeLocal()}
		if shouldProbeAzure(opts) {
			statuses = append(statuses, probeAzure(ctx, opts.AZProfile))
		}
		if shouldProbeGCS(opts) {
			statuses = append(statuses, probeGCS(ctx, opts.GCPProfile, opts.GCPProject))
		}
		result.Statuses = statuses
		for _, status := range statuses {
			if !status.OK {
				return result, fmt.Errorf("doctor checks failed")
			}
		}
		return result, nil
	}

	parsed, err := uri.Parse(target)
	if err != nil {
		return nil, err
	}
	result.Target = target
	result.Normalized = parsed.Normalized
	result.ProviderName = parsed.Provider.String()

	if parsed.Provider == uri.ProviderAZBlob {
		status := probeAzure(ctx, opts.AZProfile)
		if !status.OK {
			result.Status = &status
			return result, fmt.Errorf("doctor checks failed")
		}
	}

	p, _, err := providers.ForURI(ctx, target, providers.Options{AZProfile: opts.AZProfile, GCPProject: opts.GCPProject, GCPProfile: opts.GCPProfile})
	if err != nil {
		return result, err
	}
	result.ProviderName = p.Name()

	start := time.Now()
	if shouldUseListProbe(target, parsed) {
		seq := p.List(ctx, target, provider.ListOptions{Limit: 1})
		for meta, err := range seq {
			if err != nil {
				return result, err
			}
			result.Meta = meta
			break
		}
		result.ProbeLatency = time.Since(start)
		return result, nil
	}

	meta, err := p.Stat(ctx, target)
	result.ProbeLatency = time.Since(start)
	if err != nil {
		return result, err
	}
	result.Meta = meta
	return result, nil
}

func shouldUseListProbe(target string, parsed *uri.ParsedURI) bool {
	if parsed == nil || parsed.Provider == uri.ProviderLocal {
		return false
	}
	if strings.HasSuffix(target, "/") {
		return true
	}
	switch parsed.Provider {
	case uri.ProviderAZBlob:
		return parsed.AZBlobPath == ""
	case uri.ProviderGCS:
		return parsed.GCSObject == ""
	default:
		return false
	}
}

func probeLocal() Status {
	return Status{Provider: "local", OK: true, Detail: "local filesystem available"}
}

func probeAzure(ctx context.Context, profile string) Status {
	status, resolution, err := azureProfileSetupStatus(profile)
	if err != nil {
		return Status{Provider: "azblob", OK: false, Kind: classify(err), Detail: err.Error()}
	}
	if status != nil {
		return *status
	}
	details, err := probeAzureAuth(ctx, profile)
	if err != nil {
		if status := azureLoginSetupStatus(profile, resolution, err); status != nil {
			return *status
		}
		kind := classify(err)
		return Status{Provider: "azblob", OK: false, Kind: kind, Detail: err.Error()}
	}
	detail := "DefaultAzureCredential token acquired"
	if profile != "" {
		detail += fmt.Sprintf(" (az-profile=%s)", profile)
	}
	if suffix := formatTokenValidity(details.TokenExpiry, nowFunc()); suffix != "" {
		detail += " (" + suffix + ")"
	}
	return Status{Provider: "azblob", OK: true, Detail: detail}
}

func shouldProbeAzure(opts Options) bool {
	return opts.AZProfile != "" || (opts.GCPProject == "" && opts.GCPProfile == "")
}

func shouldProbeGCS(opts Options) bool {
	return opts.GCPProfile != "" || opts.GCPProject != "" || opts.AZProfile == ""
}

func azureProfileSetupStatus(profile string) (*Status, *providerazblob.ProfileResolution, error) {
	if profile == "" {
		return nil, nil, nil
	}
	resolution, err := resolveAzureProfile(profile)
	if err != nil {
		return nil, nil, err
	}
	if resolution.Exists {
		return nil, resolution, nil
	}
	return &Status{
		Provider: "azblob",
		OK:       false,
		Kind:     "setup",
		Detail:   formatAzureProfileGuidance(profile, resolution),
	}, resolution, nil
}

func azureLoginSetupStatus(profile string, resolution *providerazblob.ProfileResolution, err error) *Status {
	if !strings.Contains(strings.ToLower(err.Error()), "please run 'az login'") {
		return nil
	}
	return &Status{
		Provider: "azblob",
		OK:       false,
		Kind:     "setup",
		Detail:   formatAzureLoginGuidance(profile, resolution),
	}
}

func formatAzureProfileGuidance(profile string, resolution *providerazblob.ProfileResolution) string {
	var b strings.Builder
	fmt.Fprintf(&b, "az-profile %q not found\n\n", profile)
	b.WriteString("  No profile directory exists at:\n")
	for _, candidate := range resolution.Candidates {
		fmt.Fprintf(&b, "    %s\n", describePath(candidate))
	}
	b.WriteString("\n")
	b.WriteString("  To create this profile:\n\n")
	if runtime.GOOS == "windows" {
		fmt.Fprintf(&b, "    # Windows (PowerShell)\n")
		fmt.Fprintf(&b, "    $env:AZURE_CONFIG_DIR = \"%s\"\n", windowsCommandPath(profile, resolution))
		b.WriteString("    New-Item -ItemType Directory -Force -Path $env:AZURE_CONFIG_DIR\n")
		b.WriteString("    az login\n\n")
		fmt.Fprintf(&b, "    # Linux / macOS\n")
		fmt.Fprintf(&b, "    export AZURE_CONFIG_DIR=\"%s\"\n", posixCommandPath(profile, resolution))
		b.WriteString("    mkdir -p \"$AZURE_CONFIG_DIR\"\n")
		b.WriteString("    az login\n\n")
	} else {
		fmt.Fprintf(&b, "    # Linux / macOS\n")
		fmt.Fprintf(&b, "    export AZURE_CONFIG_DIR=\"%s\"\n", posixCommandPath(profile, resolution))
		b.WriteString("    mkdir -p \"$AZURE_CONFIG_DIR\"\n")
		b.WriteString("    az login\n\n")
		fmt.Fprintf(&b, "    # Windows (PowerShell)\n")
		fmt.Fprintf(&b, "    $env:AZURE_CONFIG_DIR = \"%s\"\n", windowsCommandPath(profile, resolution))
		b.WriteString("    New-Item -ItemType Directory -Force -Path $env:AZURE_CONFIG_DIR\n")
		b.WriteString("    az login\n\n")
	}
	fmt.Fprintf(&b, "  Then retry: dimlox doctor --az-profile %s", profile)
	return b.String()
}

func formatAzureLoginGuidance(profile string, resolution *providerazblob.ProfileResolution) string {
	var b strings.Builder
	b.WriteString("Azure CLI profile not logged in")
	if profile != "" {
		fmt.Fprintf(&b, " (az-profile=%s)", profile)
	}
	b.WriteString("\n\n")
	if profile == "" {
		b.WriteString("  Run this in your shell:\n\n")
		b.WriteString("    az login\n\n")
		b.WriteString("  If you normally use a non-default Azure CLI config directory, set AZURE_CONFIG_DIR to that directory before running az login.\n\n")
		b.WriteString("  Then retry: dimlox doctor")
		return b.String()
	}
	b.WriteString("  Run this in your shell to select the same profile directory:\n\n")
	if runtime.GOOS == "windows" {
		fmt.Fprintf(&b, "    $env:AZURE_CONFIG_DIR = \"%s\"\n", windowsCommandPath(profile, resolution))
		b.WriteString("    az login\n\n")
	} else {
		fmt.Fprintf(&b, "    export AZURE_CONFIG_DIR=\"%s\"\n", posixCommandPath(profile, resolution))
		b.WriteString("    az login\n\n")
	}
	fmt.Fprintf(&b, "  Then retry: dimlox doctor --az-profile %s", profile)
	return b.String()
}

func windowsCommandPath(profile string, resolution *providerazblob.ProfileResolution) string {
	if resolution != nil && resolution.Resolved != "" {
		return resolution.Resolved
	}
	if home, err := effectiveHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".azure-profiles", profile)
	}
	return profile
}

func posixCommandPath(profile string, resolution *providerazblob.ProfileResolution) string {
	path := ""
	if resolution != nil {
		path = resolution.Resolved
	}
	if path == "" {
		if home, err := effectiveHomeDir(); err == nil && home != "" {
			path = filepath.Join(home, ".azure-profiles", profile)
		}
	}
	if path == "" {
		return profile
	}
	home, err := effectiveHomeDir()
	if err == nil && home != "" {
		rel, relErr := filepath.Rel(home, path)
		if relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return "$HOME/" + filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(path)
}

func describePath(path string) string {
	if path == "" {
		return path
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

func effectiveHomeDir() (string, error) {
	if home := os.Getenv("HOME"); home != "" {
		return home, nil
	}
	return os.UserHomeDir()
}

func probeGCS(ctx context.Context, profile, project string) Status {
	details, err := probeGCSAuth(ctx, providergcs.Options{Profile: profile, Project: project})
	if err != nil {
		kind := classify(err)
		return Status{Provider: "gcs", OK: false, Kind: kind, Detail: err.Error()}
	}
	detail, err := describeGCSAuthSource(providergcs.Options{Profile: profile, Project: project})
	if err != nil {
		detail = "ADC token acquired"
	}
	if suffix := formatTokenValidity(details.TokenExpiry, nowFunc()); suffix != "" {
		detail += " (" + suffix + ")"
	}
	return Status{Provider: "gcs", OK: true, Detail: detail}
}

func formatTokenValidity(expiresAt, now time.Time) string {
	if expiresAt.IsZero() {
		return ""
	}
	remaining := expiresAt.Sub(now)
	if remaining <= 0 {
		return "token expired"
	}
	return "valid for " + formatDurationShort(remaining)
}

func formatDurationShort(d time.Duration) string {
	if d <= 0 {
		return "0m"
	}
	minutes := int64((d + time.Minute - 1) / time.Minute)
	hours := minutes / 60
	mins := minutes % 60
	if hours == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	if mins == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh%dm", hours, mins)
}

func classify(err error) string {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return "network"
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{"dial tcp", "lookup ", "connection refused", "timeout", "temporarily unavailable", "no such host"} {
		if strings.Contains(msg, marker) {
			return "network"
		}
	}
	return "auth"
}
