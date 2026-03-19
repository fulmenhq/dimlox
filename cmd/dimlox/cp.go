package main

import (
	"errors"

	"github.com/fulmenhq/dimlox/internal/transfer"
	"github.com/spf13/cobra"
)

func cpCmd() *cobra.Command {
	var (
		blockMB     int64
		concurrency int
		compress    bool
		keepLanding bool
		verify      bool
	)
	cmd := &cobra.Command{
		Use:   "cp <src-uri> <dst-uri>",
		Short: "Copy between providers via the landing area",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			azProfile, _ := cmd.Flags().GetString("az-profile")
			gcpProject, _ := cmd.Flags().GetString("gcp-project")
			landingDir, _ := cmd.Flags().GetString("landing")
			_, err := transfer.Copy(cmd.Context(), args[0], args[1], transfer.CopyOptions{
				ProviderOptions: transfer.ProviderOptions{AZProfile: azProfile, GCPProject: gcpProject},
				LandingDir:      landingDir,
				BlockSize:       mbToBytes(blockMB),
				Concurrency:     concurrency,
				Compress:        compress,
				KeepLanding:     keepLanding,
				Verify:          verify,
			})
			if err != nil {
				if errors.Is(err, transfer.ErrChecksumMismatch) {
					return withExitCode(exitChecksumMismatch, "%v", err)
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
	return cmd
}
