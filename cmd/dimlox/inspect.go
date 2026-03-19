package main

import (
	"encoding/json"
	"fmt"

	"github.com/fulmenhq/dimlox/internal/inspect"
	"github.com/spf13/cobra"
)

func inspectCmd() *cobra.Command {
	var (
		wcFlag      bool
		headN       int
		midN        int
		tailN       int
		detectFlag  bool
		sampleBytes int64
		format      string
	)
	cmd := &cobra.Command{
		Use:   "inspect <uri>",
		Short: "Stream metadata, counts, and samples from large files",
		Long:  "Inspect streams large files without full-file loads. On .gz sources, --mid and --tail fall back to a forward stream because gzip does not support efficient backward seeking.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			azProfile, _ := cmd.Flags().GetString("az-profile")
			gcpProject, _ := cmd.Flags().GetString("gcp-project")
			if format != "text" && format != "json" {
				return withExitCode(exitOperational, "invalid --format %q (want text|json)", format)
			}
			switch {
			case wcFlag:
				res, err := inspect.WC(cmd.Context(), args[0], inspect.ProviderOptions{AZProfile: azProfile, GCPProject: gcpProject})
				if err != nil {
					return withExitCode(exitOperational, "%v", err)
				}
				return printWC(cmd, res, format)
			case headN > 0:
				res, err := inspect.Head(cmd.Context(), args[0], headN, inspect.ProviderOptions{AZProfile: azProfile, GCPProject: gcpProject})
				if err != nil {
					return withExitCode(exitOperational, "%v", err)
				}
				return printSample(cmd, res, format)
			case midN > 0:
				res, err := inspect.Mid(cmd.Context(), args[0], midN, inspect.ProviderOptions{AZProfile: azProfile, GCPProject: gcpProject})
				if err != nil {
					return withExitCode(exitOperational, "%v", err)
				}
				return printSample(cmd, res, format)
			case tailN > 0:
				res, err := inspect.Tail(cmd.Context(), args[0], tailN, inspect.ProviderOptions{AZProfile: azProfile, GCPProject: gcpProject})
				if err != nil {
					return withExitCode(exitOperational, "%v", err)
				}
				return printSample(cmd, res, format)
			case detectFlag:
				_ = sampleBytes
				return withExitCode(exitOperational, "%v", inspect.UnsupportedInspectError("--detect"))
			default:
				return withExitCode(exitOperational, "inspect requires one of --wc, --head, --mid, --tail, or --detect")
			}
		},
	}
	cmd.Flags().BoolVar(&wcFlag, "wc", false, "stream line counts and report compressed byte size")
	cmd.Flags().IntVar(&headN, "head", 0, "print the first N lines")
	cmd.Flags().IntVar(&midN, "mid", 0, "print N lines near the midpoint")
	cmd.Flags().IntVar(&tailN, "tail", 0, "print the last N lines")
	cmd.Flags().BoolVar(&detectFlag, "detect", false, "detect encoding and delimiter (follow-up slice)")
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
