package main

import (
	"github.com/fulmenhq/dimlox/internal/transfer"
	"github.com/spf13/cobra"
)

func putCmd() *cobra.Command {
	var (
		blockMB     int64
		concurrency int
		compress    bool
		contentType string
	)
	cmd := &cobra.Command{
		Use:   "put <src-path> <dst-uri>",
		Short: "Upload a local file to cloud or local storage",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			azProfile, _ := cmd.Flags().GetString("az-profile")
			gcpProject, _ := cmd.Flags().GetString("gcp-project")
			landingDir, _ := cmd.Flags().GetString("landing")
			_, err := transfer.Upload(cmd.Context(), transfer.UploadOptions{
				ProviderOptions: transfer.ProviderOptions{AZProfile: azProfile, GCPProject: gcpProject},
				SourcePath:      args[0],
				Destination:     args[1],
				BlockSize:       mbToBytes(blockMB),
				Concurrency:     concurrency,
				Compress:        compress,
				ContentType:     contentType,
				LandingDir:      landingDir,
			})
			if err != nil {
				return withExitCode(exitOperational, "%v", err)
			}
			return nil
		},
	}
	cmd.Flags().Int64Var(&blockMB, "block-mb", 32, "multipart upload chunk size in MiB")
	cmd.Flags().IntVar(&concurrency, "concurrency", 4, "parallel upload workers")
	cmd.Flags().StringVar(&contentType, "content-type", "", "override destination content type")
	cmd.Flags().BoolVar(&compress, "compress", false, "gzip-compress the local file before upload")
	return cmd
}
