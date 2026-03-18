package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fulmenhq/dimlox/internal/doctor"
	"github.com/fulmenhq/dimlox/internal/uri"
	"github.com/spf13/cobra"
)

func doctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor [uri]",
		Short: "Check auth, connectivity, and metadata probes for configured providers",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			azProfile, _ := cmd.Flags().GetString("az-profile")
			gcpProject, _ := cmd.Flags().GetString("gcp-project")

			target := ""
			if len(args) == 1 {
				target = args[0]
			}

			result, err := doctor.Run(cmd.Context(), target, doctor.Options{
				AZProfile:  azProfile,
				GCPProject: gcpProject,
				Version:    formatVersion(),
			})
			if err != nil {
				if target != "" {
					var unsupported *uri.ErrUnsupportedScheme
					if strings.TrimSpace(target) == "" || errors.Is(err, uri.ErrEmptyURI) || errors.As(err, &unsupported) {
						return withExitCode(exitBadURI, "%v", err)
					}
				}
				printDoctorResult(cmd, result)
				return withExitCode(exitOperational, "%v", err)
			}

			printDoctorResult(cmd, result)
			return nil
		},
	}
	return cmd
}

func printDoctorResult(cmd *cobra.Command, result *doctor.Result) {
	if result == nil {
		return
	}
	out := cmd.OutOrStdout()
	if result.Target == "" {
		_, _ = fmt.Fprintf(out, "dimlox %s\n", result.AppVersion)
		_, _ = fmt.Fprintf(out, "go: %s\n", result.GoVersion)
		_, _ = fmt.Fprintf(out, "platform: %s\n", result.OSArch)
		for _, status := range result.Statuses {
			state := "ok"
			if !status.OK {
				state = status.Kind + " failure"
			}
			_, _ = fmt.Fprintf(out, "%s: %s - %s\n", status.Provider, state, status.Detail)
		}
		return
	}

	_, _ = fmt.Fprintf(out, "provider: %s\n", result.ProviderName)
	_, _ = fmt.Fprintf(out, "normalized: %s\n", result.Normalized)
	if result.Meta != nil {
		_, _ = fmt.Fprintf(out, "name: %s\n", result.Meta.Name)
		_, _ = fmt.Fprintf(out, "size: %d\n", result.Meta.Size)
		_, _ = fmt.Fprintf(out, "content-type: %s\n", result.Meta.ContentType)
		_, _ = fmt.Fprintf(out, "etag: %s\n", result.Meta.ETag)
		if !result.Meta.LastModified.IsZero() {
			_, _ = fmt.Fprintf(out, "last-modified: %s\n", result.Meta.LastModified.Format("2006-01-02T15:04:05Z07:00"))
		}
	}
	_, _ = fmt.Fprintf(out, "latency: %s\n", result.ProbeLatency.Round(10*time.Millisecond))
}
