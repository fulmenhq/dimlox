package transfer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/fulmenhq/dimlox/internal/uri"
)

const DefaultMaxSources = 1000

type CopyPlan struct {
	Items []CopyPlanItem
}

type CopyPlanItem struct {
	Source      string
	Destination string
}

type CopyPlanOptions struct {
	ProviderOptions
	FromFile   string
	MaxSources int
}

type ExecuteCopyPlanOptions struct {
	CopyOptions
	ContinueOnError bool
	SummaryWriter   io.Writer
}

type ExecuteCopyPlanResult struct {
	Transferred int
	Failed      int
	Skipped     int
}

func BuildCopyPlan(ctx context.Context, args []string, opts CopyPlanOptions) (*CopyPlan, error) {
	if opts.MaxSources <= 0 {
		opts.MaxSources = DefaultMaxSources
	}

	if opts.FromFile != "" {
		if len(args) != 0 {
			return nil, fmt.Errorf("--from-file cannot be combined with positional source or destination arguments")
		}
		return buildCopyPlanFromFile(opts.FromFile)
	}

	if len(args) < 2 {
		return nil, fmt.Errorf("cp requires at least one source and one destination")
	}

	dst := args[len(args)-1]
	srcArgs := args[:len(args)-1]
	resolved, err := resolveSources(ctx, srcArgs, opts)
	if err != nil {
		return nil, err
	}
	if len(resolved) == 0 {
		return nil, fmt.Errorf("cp resolved no source files")
	}

	items, err := mapResolvedSourcesToDestinations(resolved, dst)
	if err != nil {
		return nil, err
	}
	plan := &CopyPlan{Items: items}
	if err := validateCopyPlan(plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func WriteCopyPlan(w io.Writer, plan *CopyPlan) error {
	if plan == nil {
		return fmt.Errorf("copy plan was nil")
	}
	if _, err := fmt.Fprintf(w, "transfer plan (%d file(s)):\n", len(plan.Items)); err != nil {
		return err
	}
	for _, item := range plan.Items {
		if _, err := fmt.Fprintf(w, "- %s -> %s\n", item.Source, item.Destination); err != nil {
			return err
		}
	}
	return nil
}

func ExecuteCopyPlan(ctx context.Context, plan *CopyPlan, opts ExecuteCopyPlanOptions) (*ExecuteCopyPlanResult, error) {
	if plan == nil {
		return nil, fmt.Errorf("copy plan was nil")
	}
	result := &ExecuteCopyPlanResult{}
	var failures []error
	for _, item := range plan.Items {
		_, err := Copy(ctx, item.Source, item.Destination, opts.CopyOptions)
		if err != nil {
			result.Failed++
			failures = append(failures, fmt.Errorf("%s -> %s: %w", item.Source, item.Destination, err))
			if !opts.ContinueOnError {
				result.Skipped = len(plan.Items) - result.Transferred - result.Failed
				printCopySummary(opts.SummaryWriter, len(plan.Items), result)
				return result, failures[0]
			}
			continue
		}
		result.Transferred++
	}
	result.Skipped = len(plan.Items) - result.Transferred - result.Failed
	printCopySummary(opts.SummaryWriter, len(plan.Items), result)
	if len(failures) > 0 {
		return result, fmt.Errorf("%d transfer(s) failed: %w", len(failures), errors.Join(failures...))
	}
	return result, nil
}

type resolvedSource struct {
	Source   string
	Basename string
}

func resolveSources(ctx context.Context, srcArgs []string, opts CopyPlanOptions) ([]resolvedSource, error) {
	resolved := make([]resolvedSource, 0, len(srcArgs))
	for _, src := range srcArgs {
		if hasGlobMeta(src) {
			remaining := opts.MaxSources - len(resolved)
			if opts.MaxSources > 0 && remaining <= 0 {
				return nil, fmt.Errorf("glob source expansion exceeded --max-sources=%d", opts.MaxSources)
			}
			matches, err := expandGlob(ctx, src, opts.ProviderOptions, remaining)
			if err != nil {
				return nil, err
			}
			resolved = append(resolved, matches...)
			continue
		}
		basename, err := basenameForURI(src)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, resolvedSource{Source: src, Basename: basename})
	}
	return resolved, nil
}

func mapResolvedSourcesToDestinations(resolved []resolvedSource, dst string) ([]CopyPlanItem, error) {
	items := make([]CopyPlanItem, 0, len(resolved))
	isPrefix := strings.HasSuffix(dst, "/")
	if len(resolved) > 1 && !isPrefix {
		return nil, fmt.Errorf("destination must end with / when multiple sources resolve: %s", dst)
	}
	for _, src := range resolved {
		target := dst
		if isPrefix {
			target = joinDestinationPrefix(dst, src.Basename)
		}
		items = append(items, CopyPlanItem{Source: src.Source, Destination: target})
	}
	return items, nil
}

func validateCopyPlan(plan *CopyPlan) error {
	if plan == nil {
		return fmt.Errorf("copy plan was nil")
	}
	if len(plan.Items) == 0 {
		return fmt.Errorf("copy plan contained no transfers")
	}
	seen := make(map[string]string, len(plan.Items))
	for _, item := range plan.Items {
		if _, err := uri.Parse(item.Source); err != nil {
			return err
		}
		normalizedDst, err := normalizeDestination(item.Destination)
		if err != nil {
			return err
		}
		if prev, ok := seen[normalizedDst]; ok {
			return fmt.Errorf("destination collision: %s and %s both map to %s", prev, item.Source, item.Destination)
		}
		seen[normalizedDst] = item.Source
	}
	return nil
}

func joinDestinationPrefix(prefix, base string) string {
	return prefix + strings.TrimPrefix(base, "/")
}

func normalizeDestination(raw string) (string, error) {
	parsed, err := uri.Parse(raw)
	if err != nil {
		return "", err
	}
	return parsed.Normalized, nil
}

func basenameForURI(raw string) (string, error) {
	parsed, err := uri.Parse(raw)
	if err != nil {
		return "", err
	}
	base := basenameForTarget(nil, parsed)
	if base == "" || base == "." || base == "/" {
		return "", fmt.Errorf("cannot derive destination basename from source %s", raw)
	}
	return path.Base(base), nil
}

func hasGlobMeta(raw string) bool {
	return strings.ContainsAny(raw, "*?[")
}

func printCopySummary(w io.Writer, total int, result *ExecuteCopyPlanResult) {
	if w == nil || result == nil || total <= 1 {
		return
	}
	_, _ = fmt.Fprintf(w, "cp summary: transferred=%d failed=%d skipped=%d\n", result.Transferred, result.Failed, result.Skipped)
}
