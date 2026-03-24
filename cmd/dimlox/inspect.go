package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fulmenhq/dimlox/internal/inspect"
	"github.com/fulmenhq/dimlox/internal/uri"
	"github.com/fulmenhq/gofulmen/foundry"
	"github.com/spf13/cobra"
)

func inspectCmd() *cobra.Command {
	var (
		wcFlag      bool
		headN       int
		midN        int
		tailN       int
		detectFlag  bool
		forceStream bool
		sampleBytes int64
		format      string
	)
	cmd := &cobra.Command{
		Use:   "inspect <uri>",
		Short: "Stream metadata, counts, and samples from large files",
		Long:  "Inspect streams large files without full-file loads. On compressed cloud sources, --tail and --mid are refused by default because gzip fallback requires re-streaming the file over the network; use --force-stream to override.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			azProfile, _ := cmd.Flags().GetString("az-profile")
			gcpProfile, _ := cmd.Flags().GetString("gcp-profile")
			gcpProject := selectedGCPProject(cmd)
			if format != "text" && format != "json" {
				return withExitCode(foundry.ExitInvalidArgument, "invalid --format %q (want text|json)", format)
			}
			handleErr := func(err error) error {
				var unsupported *uri.ErrUnsupportedScheme
				if errors.Is(err, uri.ErrEmptyURI) || errors.As(err, &unsupported) {
					return withExitCode(foundry.ExitInvalidArgument, "%v", err)
				}
				return withExitCode(foundry.ExitFailure, "%v", err)
			}
			switch {
			case wcFlag:
				res, err := inspect.WC(cmd.Context(), args[0], inspect.ProviderOptions{AZProfile: azProfile, GCPProject: gcpProject, GCPProfile: gcpProfile})
				if err != nil {
					return handleErr(err)
				}
				return printWC(cmd, res, format)
			case headN > 0:
				res, err := inspect.Head(cmd.Context(), args[0], headN, inspect.ProviderOptions{AZProfile: azProfile, GCPProject: gcpProject, GCPProfile: gcpProfile})
				if err != nil {
					return handleErr(err)
				}
				return printSample(cmd, res, format)
			case midN > 0:
				if !forceStream {
					if err := inspect.RefuseCompressedCloudSample(cmd.Context(), args[0], inspect.SampleMid, midN, inspect.ProviderOptions{AZProfile: azProfile, GCPProject: gcpProject, GCPProfile: gcpProfile}); err != nil {
						return handleErr(err)
					}
				}
				res, err := inspect.Mid(cmd.Context(), args[0], midN, inspect.ProviderOptions{AZProfile: azProfile, GCPProject: gcpProject, GCPProfile: gcpProfile})
				if err != nil {
					return handleErr(err)
				}
				return printSample(cmd, res, format)
			case tailN > 0:
				if !forceStream {
					if err := inspect.RefuseCompressedCloudSample(cmd.Context(), args[0], inspect.SampleTail, tailN, inspect.ProviderOptions{AZProfile: azProfile, GCPProject: gcpProject, GCPProfile: gcpProfile}); err != nil {
						return handleErr(err)
					}
				}
				res, err := inspect.Tail(cmd.Context(), args[0], tailN, inspect.ProviderOptions{AZProfile: azProfile, GCPProject: gcpProject, GCPProfile: gcpProfile})
				if err != nil {
					return handleErr(err)
				}
				return printSample(cmd, res, format)
			case detectFlag:
				res, err := inspect.Detect(cmd.Context(), args[0], sampleBytes, inspect.ProviderOptions{AZProfile: azProfile, GCPProject: gcpProject, GCPProfile: gcpProfile})
				if err != nil {
					return handleErr(err)
				}
				return printDetect(cmd, res, format)
			default:
				return withExitCode(foundry.ExitInvalidArgument, "inspect requires one of --wc, --head, --mid, --tail, or --detect")
			}
		},
	}
	cmd.Flags().BoolVar(&wcFlag, "wc", false, "stream line counts and report compressed byte size")
	cmd.Flags().IntVar(&headN, "head", 0, "print the first N lines")
	cmd.Flags().IntVar(&midN, "mid", 0, "print N lines near the midpoint (compressed cloud sources require --force-stream)")
	cmd.Flags().IntVar(&tailN, "tail", 0, "print the last N lines (compressed cloud sources require --force-stream)")
	cmd.Flags().BoolVar(&detectFlag, "detect", false, "detect encoding and delimiter from a bounded sample")
	cmd.Flags().BoolVar(&forceStream, "force-stream", false, "allow expensive forward-stream fallback on compressed cloud tail/mid operations")
	cmd.Flags().Int64Var(&sampleBytes, "sample-bytes", 65536, "bytes to sample for detection")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return cmd
}

func printWC(cmd *cobra.Command, res *inspect.WCResult, format string) error {
	if format == "json" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "uri: %s\n", res.URI)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "compressed-bytes: %d\n", res.CompressedBytes)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "lines: %d\n", res.Lines)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "compressed: %t\n", res.Compressed)
	return nil
}

func printSample(cmd *cobra.Command, res *inspect.SampleResult, format string) error {
	if format == "json" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "uri: %s\n", res.URI)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "mode: %s\n", res.Mode)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "compressed: %t\n", res.Compressed)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "strategy: %s\n", res.Strategy)
	for _, line := range res.Lines {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
	}
	return nil
}

func printDetect(cmd *cobra.Command, res *inspect.DetectResult, format string) error {
	if format == "json" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "uri: %s\n", res.URI)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "compressed: %t\n", res.Compressed)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "encoding: %s\n", res.Encoding)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "bom: %t\n", res.BOM)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "line-ending: %s\n", res.LineEnding)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "delimiter: %s\n", res.Delimiter)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "delimiter-confidence: %.3f\n", res.DelimiterConfidence)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "fields-per-row: %d\n", res.FieldsPerRow)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "sample-rows: %d\n", res.SampleRows)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "sample-bytes: %d\n", res.SampleBytes)
	return nil
}
