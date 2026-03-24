package main

import (
	"errors"
	"fmt"

	providergcs "github.com/fulmenhq/dimlox/internal/provider/gcs"
	"github.com/fulmenhq/dimlox/internal/transfer"
	"github.com/fulmenhq/dimlox/internal/uri"
	"github.com/fulmenhq/gofulmen/foundry"
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
		gcpProfileSrc   string
		gcpProfileDst   string
		gcpCredsFileSrc string
		gcpCredsFileDst string
		gcpProjectSrc   string
		gcpProjectDst   string
	)
	cmd := &cobra.Command{
		Use:   "cp [flags] <src-uri>... <dst-uri>",
		Short: "Copy between providers via the landing area",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if fromFile != "" && len(args) != 0 {
				return withExitCode(foundry.ExitInvalidArgument, "--from-file cannot be combined with positional source or destination arguments")
			}
			if fromFile == "" && len(args) < 2 {
				return withExitCode(foundry.ExitInvalidArgument, "cp requires at least one source and one destination")
			}
			if parallel < 1 {
				return withExitCode(foundry.ExitInvalidArgument, "--parallel must be >= 1")
			}
			if parallel > 1 {
				return withExitCode(foundry.ExitInvalidArgument, "--parallel > 1 is not yet supported; see future release notes")
			}
			if maxSources < 1 {
				return withExitCode(foundry.ExitInvalidArgument, "--max-sources must be >= 1")
			}

			azProfile, _ := cmd.Flags().GetString("az-profile")
			gcpProfile, _ := cmd.Flags().GetString("gcp-profile")
			gcpProject := selectedGCPProject(cmd)
			landingDir, _ := cmd.Flags().GetString("landing")
			srcProviderOptions := transfer.ProviderOptions{AZProfile: azProfile, GCPProject: gcpProject, GCPProfile: gcpProfile}
			dstProviderOptions := transfer.ProviderOptions{AZProfile: azProfile, GCPProject: gcpProject, GCPProfile: gcpProfile}
			if gcpProjectSrc != "" {
				srcProviderOptions.GCPProject = gcpProjectSrc
			}
			if gcpProjectDst != "" {
				dstProviderOptions.GCPProject = gcpProjectDst
			}
			if gcpProfileSrc != "" {
				srcProviderOptions.GCPProfile = gcpProfileSrc
			}
			if gcpProfileDst != "" {
				dstProviderOptions.GCPProfile = gcpProfileDst
			}
			srcProviderOptions.GCPCredsFile = gcpCredsFileSrc
			dstProviderOptions.GCPCredsFile = gcpCredsFileDst
			plan, err := transfer.BuildCopyPlan(cmd.Context(), args, transfer.CopyPlanOptions{
				ProviderOptions:       transfer.ProviderOptions{AZProfile: azProfile, GCPProject: gcpProject, GCPProfile: gcpProfile},
				SourceProviderOptions: srcProviderOptions,
				FromFile:              fromFile,
				MaxSources:            maxSources,
			})
			if err != nil {
				return withExitCode(foundry.ExitInvalidArgument, "%v", err)
			}
			if err := validateGCSLegSelections(plan, srcProviderOptions, dstProviderOptions); err != nil {
				return withExitCode(foundry.ExitInvalidArgument, "%v", err)
			}
			if dryRun {
				if err := transfer.WriteCopyPlan(cmd.OutOrStdout(), plan); err != nil {
					return withExitCode(foundry.ExitFailure, "%v", err)
				}
				return nil
			}
			result, err := transfer.ExecuteCopyPlan(cmd.Context(), plan, transfer.ExecuteCopyPlanOptions{
				CopyOptions: transfer.CopyOptions{
					ProviderOptions:            transfer.ProviderOptions{AZProfile: azProfile, GCPProject: gcpProject, GCPProfile: gcpProfile},
					SourceProviderOptions:      srcProviderOptions,
					DestinationProviderOptions: dstProviderOptions,
					LandingDir:                 landingDir,
					BlockSize:                  mbToBytes(blockMB),
					Concurrency:                concurrency,
					Compress:                   compress,
					KeepLanding:                keepLanding,
					Verify:                     verify,
				},
				ContinueOnError: continueOnError,
				SummaryWriter:   cmd.ErrOrStderr(),
			})
			if err != nil {
				if continueOnError && result != nil && len(result.Errors) > 0 {
					return withExitCode(worstBatchExitCode(result.Errors), "%v", err)
				}
				if errors.Is(err, transfer.ErrChecksumMismatch) {
					return withExitCode(foundry.ExitDataCorrupt, "%v", err)
				}
				var unsupported *uri.ErrUnsupportedScheme
				if errors.Is(err, uri.ErrEmptyURI) || errors.As(err, &unsupported) {
					return withExitCode(foundry.ExitInvalidArgument, "%v", err)
				}
				return withExitCode(foundry.ExitFailure, "%v", err)
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
	cmd.Flags().StringVar(&gcpProfileSrc, "gcp-profile-src", "", "gcloud named configuration for GCS source legs")
	cmd.Flags().StringVar(&gcpProfileDst, "gcp-profile-dst", "", "gcloud named configuration for GCS destination legs")
	cmd.Flags().StringVar(&gcpCredsFileSrc, "gcp-creds-file-src", "", "credential file for GCS source legs")
	cmd.Flags().StringVar(&gcpCredsFileDst, "gcp-creds-file-dst", "", "credential file for GCS destination legs")
	cmd.Flags().StringVar(&gcpProjectSrc, "gcp-project-src", "", "GCP project for GCS source legs")
	cmd.Flags().StringVar(&gcpProjectDst, "gcp-project-dst", "", "GCP project for GCS destination legs")
	return cmd
}

func worstBatchExitCode(errs []error) foundry.ExitCode {
	worst := foundry.ExitFailure
	worstRank := batchExitRank(worst)
	for _, err := range errs {
		code := exitCodeFor(err)
		rank := batchExitRank(code)
		if rank > worstRank {
			worst = code
			worstRank = rank
		}
	}
	return worst
}

func batchExitRank(code foundry.ExitCode) int {
	switch code {
	case foundry.ExitAuthenticationFailed:
		return 4
	case foundry.ExitDataCorrupt:
		return 3
	case foundry.ExitResourceExhausted:
		return 2
	case foundry.ExitFailure:
		return 1
	default:
		return 0
	}
}

func validateGCSLegSelections(plan *transfer.CopyPlan, srcOpts, dstOpts transfer.ProviderOptions) error {
	if plan == nil {
		return nil
	}
	hasGCSSource := false
	hasGCSDestination := false
	for _, item := range plan.Items {
		srcParsed, err := uri.Parse(item.Source)
		if err != nil {
			return err
		}
		if srcParsed.Provider == uri.ProviderGCS {
			hasGCSSource = true
		}
		dstParsed, err := uri.Parse(item.Destination)
		if err != nil {
			return err
		}
		if dstParsed.Provider == uri.ProviderGCS {
			hasGCSDestination = true
		}
	}
	if hasGCSSource {
		if _, err := providergcs.ResolveOptions(providergcs.Options{Project: srcOpts.GCPProject, Profile: srcOpts.GCPProfile, CredsFile: srcOpts.GCPCredsFile}); err != nil {
			return fmt.Errorf("source GCS auth preflight: %w", err)
		}
	}
	if hasGCSDestination {
		if _, err := providergcs.ResolveOptions(providergcs.Options{Project: dstOpts.GCPProject, Profile: dstOpts.GCPProfile, CredsFile: dstOpts.GCPCredsFile}); err != nil {
			return fmt.Errorf("destination GCS auth preflight: %w", err)
		}
	}
	return nil
}
