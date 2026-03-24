package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fulmenhq/dimlox/internal/split"
	"github.com/fulmenhq/dimlox/internal/uri"
	"github.com/fulmenhq/gofulmen/foundry"
	"github.com/spf13/cobra"
)

func splitCmd() *cobra.Command {
	var (
		mode        string
		rows        int64
		bytesLimit  int64
		outDir      string
		outFmt      string
		header      bool
		delimiter   string
		encoding    string
		manifest    bool
		dryRun      bool
		sampleBytes int64
	)
	cmd := &cobra.Command{
		Use:   "split <uri>",
		Short: "Split large files into shard files",
		RunE: func(cmd *cobra.Command, args []string) error {
			azProfile, _ := cmd.Flags().GetString("az-profile")
			gcpProfile, _ := cmd.Flags().GetString("gcp-profile")
			gcpProject := selectedGCPProject(cmd)
			landingDir, _ := cmd.Flags().GetString("landing")
			handleErr := func(err error) error {
				var unsupported *uri.ErrUnsupportedScheme
				if errors.Is(err, uri.ErrEmptyURI) || errors.As(err, &unsupported) || isBadInputError(err) {
					return withExitCode(foundry.ExitInvalidArgument, "%v", err)
				}
				return withExitCode(foundry.ExitFailure, "%v", err)
			}
			res, err := split.Split(cmd.Context(), args[0], split.Options{
				ProviderOptions: split.ProviderOptions{AZProfile: azProfile, GCPProject: gcpProject, GCPProfile: gcpProfile},
				Mode:            split.Mode(mode),
				Rows:            rows,
				Bytes:           mbToBytes(bytesLimit),
				OutDir:          chooseSplitOutDir(outDir, landingDir),
				OutFmt:          outFmt,
				Header:          header,
				Delimiter:       delimiter,
				Encoding:        encoding,
				Manifest:        manifest,
				DryRun:          dryRun,
				SampleBytes:     sampleBytes,
			})
			if err != nil {
				return handleErr(err)
			}
			return printSplit(cmd, res)
		},
		Args: cobra.ExactArgs(1),
	}
	cmd.Flags().StringVar(&mode, "mode", string(split.ModeAuto), "split mode: auto, stream, range, or binary")
	cmd.Flags().Int64Var(&rows, "rows", 0, "max data rows per shard for text modes")
	cmd.Flags().Int64Var(&bytesLimit, "bytes", 0, "max MiB per shard")
	cmd.Flags().StringVar(&outDir, "out-dir", "", "output directory for shard files")
	cmd.Flags().StringVar(&outFmt, "out-fmt", "match", "output format: match, text, or gz")
	cmd.Flags().BoolVar(&header, "header", false, "copy the first line to every text shard")
	cmd.Flags().StringVar(&delimiter, "delimiter", "", "override delimiter detection for text split modes")
	cmd.Flags().StringVar(&encoding, "encoding", "", "override encoding detection for text split modes")
	cmd.Flags().BoolVar(&manifest, "manifest", true, "write a JSON Lines manifest alongside shards")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the shard plan without writing files")
	cmd.Flags().Int64Var(&sampleBytes, "sample-bytes", 65536, "bytes to sample for delimiter and encoding detection")
	return cmd
}

func chooseSplitOutDir(outDir, landingDir string) string {
	if outDir != "" {
		return outDir
	}
	return landingDir
}

func printSplit(cmd *cobra.Command, res *split.Result) error {
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "source: %s\n", res.SourceURI)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "mode: %s\n", res.Mode)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "out-dir: %s\n", res.OutDir)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "dry-run: %t\n", res.DryRun)
	if res.Delimiter != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "delimiter: %s\n", res.Delimiter)
	}
	if res.Encoding != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "encoding: %s\n", res.Encoding)
	}
	if res.ManifestPath != "" {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "manifest: %s\n", res.ManifestPath)
	}
	for _, note := range res.Notes {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "note: %s\n", note)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "shards: %d\n", len(res.Shards))
	for _, shard := range res.Shards {
		payload, err := json.Marshal(shard)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(payload))
	}
	return nil
}
