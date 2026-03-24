package main

import (
	"errors"

	"github.com/fulmenhq/dimlox/internal/transfer"
	"github.com/fulmenhq/dimlox/internal/uri"
	"github.com/spf13/cobra"
)

func cpCmd() *cobra.Command {
	var (
		blockMB         int64
		concurrency     int
		compress        bool
		keepLanding     bool
		verify          bool
		fromFile        string
		parallel        int
		maxSources      int
		continueOnError bool
		dryRun          bool
	)
	cmd := &cobra.Command{
		Use:   "cp [flags] <src-uri>... <dst-uri>",
		Short: "Copy between providers via the landing area",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if fromFile != "" && len(args) != 0 {
				return withExitCode(exitBadURI, "--from-file cannot be combined with positional source or destination arguments")
			}
			if fromFile == "" && len(args) < 2 {
				return withExitCode(exitBadURI, "cp requires at least one source and one destination")
			}
			if parallel < 1 {
				return withExitCode(exitBadURI, "--parallel must be >= 1")
			}
			if parallel > 1 {
				return withExitCode(exitBadURI, "--parallel > 1 is not yet supported; see future release notes")
			}
			if maxSources < 1 {
				return withExitCode(exitBadURI, "--max-sources must be >= 1")
			}

			azProfile, _ := cmd.Flags().GetString("az-profile")
			gcpProject, _ := cmd.Flags().GetString("gcp-project")
			landingDir, _ := cmd.Flags().GetString("landing")
			plan, err := transfer.BuildCopyPlan(cmd.Context(), args, transfer.CopyPlanOptions{
				ProviderOptions: transfer.ProviderOptions{AZProfile: azProfile, GCPProject: gcpProject},
				FromFile:        fromFile,
				MaxSources:      maxSources,
			})
			if err != nil {
				return withExitCode(exitBadURI, "%v", err)
			}
			if dryRun {
				if err := transfer.WriteCopyPlan(cmd.OutOrStdout(), plan); err != nil {
					return withExitCode(exitOperational, "%v", err)
				}
				return nil
			}
			_, err = transfer.ExecuteCopyPlan(cmd.Context(), plan, transfer.ExecuteCopyPlanOptions{
				CopyOptions: transfer.CopyOptions{
					ProviderOptions: transfer.ProviderOptions{AZProfile: azProfile, GCPProject: gcpProject},
					LandingDir:      landingDir,
					BlockSize:       mbToBytes(blockMB),
					Concurrency:     concurrency,
					Compress:        compress,
					KeepLanding:     keepLanding,
					Verify:          verify,
				},
				ContinueOnError: continueOnError,
				SummaryWriter:   cmd.ErrOrStderr(),
			})
			if err != nil {
				if errors.Is(err, transfer.ErrChecksumMismatch) {
					return withExitCode(exitChecksumMismatch, "%v", err)
				}
				var unsupported *uri.ErrUnsupportedScheme
				if errors.Is(err, uri.ErrEmptyURI) || errors.As(err, &unsupported) {
					return withExitCode(exitBadURI, "%v", err)
				}
				return withExitCode(exitOperational, "%v", err)
			}
			return nil
		},
	}
	cmd.Flags().Int64Var(&blockMB, "block-mb", 32, "chunk size for download and upload legs in MiB")
	cmd.Flags().IntVar(&concurrency, "concurrency", 8, "parallel chunk workers for the download leg")
	cmd.Flags().BoolVar(&compress, "compress", false, "gzip-compress uncompressed text in landing before upload")
	cmd.Flags().BoolVar(&keepLanding, "keep-landing", false, "keep the intermediate landing file after upload")
	cmd.Flags().BoolVar(&verify, "verify", false, "verify the download leg checksum when metadata is available")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "JSONL file with src/dst transfer pairs")
	cmd.Flags().IntVar(&parallel, "parallel", 1, "reserved for future concurrent file transfers (currently only 1 is supported)")
	cmd.Flags().IntVar(&maxSources, "max-sources", transfer.DefaultMaxSources, "maximum number of sources resolved by glob expansion before preflight fails")
	cmd.Flags().BoolVar(&continueOnError, "continue-on-error", false, "attempt all transfers and report failures at the end")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the transfer plan without executing it")
	return cmd
}
