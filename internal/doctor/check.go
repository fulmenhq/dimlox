package doctor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strings"
	"time"

	"github.com/fulmenhq/dimlox/internal/provider"
	providerazblob "github.com/fulmenhq/dimlox/internal/provider/azblob"
	providergcs "github.com/fulmenhq/dimlox/internal/provider/gcs"
	"github.com/fulmenhq/dimlox/internal/providers"
	"github.com/fulmenhq/dimlox/internal/uri"
)

type Options struct {
	AZProfile  string
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
		statuses := []Status{
			probeLocal(),
			probeAzure(ctx, opts.AZProfile),
			probeGCS(ctx),
		}
		result.Statuses = statuses
		for _, status := range statuses {
			if !status.OK {
				return result, fmt.Errorf("doctor checks failed")
			}
		}
		return result, nil
	}

	p, parsed, err := providers.ForURI(ctx, target, providers.Options{AZProfile: opts.AZProfile, GCPProject: opts.GCPProject})
	if err != nil {
		return nil, err
	}
	result.Target = target
	result.Normalized = parsed.Normalized
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
	if err := providerazblob.ProbeAuth(ctx, profile); err != nil {
		kind := classify(err)
		return Status{Provider: "azblob", OK: false, Kind: kind, Detail: err.Error()}
	}
	detail := "DefaultAzureCredential token acquired"
	if profile != "" {
		detail += fmt.Sprintf(" (az-profile=%s)", profile)
	}
	return Status{Provider: "azblob", OK: true, Detail: detail}
}

func probeGCS(ctx context.Context) Status {
	if err := providergcs.ProbeAuth(ctx); err != nil {
		kind := classify(err)
		return Status{Provider: "gcs", OK: false, Kind: kind, Detail: err.Error()}
	}
	detail, err := providergcs.DescribeAuthSource()
	if err != nil {
		return Status{Provider: "gcs", OK: true, Detail: "ADC token acquired"}
	}
	return Status{Provider: "gcs", OK: true, Detail: detail}
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
