package main

import (
	"errors"

	"github.com/fulmenhq/dimlox/internal/transfer"
	"github.com/fulmenhq/dimlox/internal/uri"
	"github.com/spf13/cobra"
)

func getCmd() *cobra.Command {
	var (
		blockMB     int64
		concurrency int
		compress    bool
		overwrite   bool
		verify      bool
	)
	cmd := &cobra.Command{
		Use:   "get <src-uri> [dst-path]",
		Short: "Download a cloud or local object to a local file",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			azProfile, _ := cmd.Flags().GetString("az-profile")
			gcpProject, _ := cmd.Flags().GetString("gcp-project")
			landingDir, _ := cmd.Flags().GetString("landing")
			dst := ""
			if len(args) == 2 {
				dst = args[1]
			}
			_, err := transfer.Download(cmd.Context(), args[0], transfer.DownloadOptions{
				ProviderOptions: transfer.ProviderOptions{AZProfile: azProfile, GCPProject: gcpProject},
				DestinationPath: dst,
				LandingDir:      landingDir,
				BlockSize:       mbToBytes(blockMB),
				Concurrency:     concurrency,
				Compress:        compress,
				Overwrite:       overwrite,
				Verify:          verify,
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
	cmd.Flags().Int64Var(&blockMB, "block-mb", 32, "chunk size for parallel download in MiB")
	cmd.Flags().IntVar(&concurrency, "concurrency", 8, "parallel chunks")
	cmd.Flags().BoolVar(&compress, "compress", false, "gzip-compress uncompressed text on write")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "replace an existing local destination")
	cmd.Flags().BoolVar(&verify, "verify", false, "verify checksum after download when metadata is available")
	return cmd
}
