package main

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/fulmenhq/dimlox/internal/doctor"
	providergcs "github.com/fulmenhq/dimlox/internal/provider/gcs"
	"github.com/fulmenhq/dimlox/internal/uri"
	"github.com/fulmenhq/gofulmen/foundry"
	"github.com/spf13/cobra"
)

var listGCPProfilesFunc = providergcs.ListProfiles
var doctorRunFunc = doctor.Run

func doctorCmd() *cobra.Command {
	var listGCPProfiles bool
	cmd := &cobra.Command{
		Use:   "doctor [uri]",
		Short: "Check auth, connectivity, and metadata probes for configured providers",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if listGCPProfiles {
				if len(args) != 0 {
					return withExitCode(foundry.ExitInvalidArgument, "--list-gcp-profiles does not accept a target URI")
				}
				profiles, err := listGCPProfilesFunc()
				if err != nil {
					return withExitCode(foundry.ExitFailure, "%v", err)
				}
				printGCPProfiles(cmd, profiles)
				return nil
			}
			azProfile, _ := cmd.Flags().GetString("az-profile")
			gcpProfile, _ := cmd.Flags().GetString("gcp-profile")
			gcpProject := selectedGCPProject(cmd)

			target := ""
			if len(args) == 1 {
				target = args[0]
			}

			result, err := doctorRunFunc(cmd.Context(), target, doctor.Options{
				AZProfile:  azProfile,
				GCPProfile: gcpProfile,
				GCPProject: gcpProject,
				Version:    formatVersion(),
			})
			if err != nil {
				if target != "" {
					var unsupported *uri.ErrUnsupportedScheme
					if strings.TrimSpace(target) == "" || errors.Is(err, uri.ErrEmptyURI) || errors.As(err, &unsupported) {
						return withExitCode(foundry.ExitInvalidArgument, "%v", err)
					}
				}
				printDoctorResult(cmd, result)
				if doctorHasAuthFailure(result) {
					return withExitCode(foundry.ExitAuthenticationFailed, "%v", err)
				}
				return withExitCode(foundry.ExitFailure, "%v", err)
			}

			printDoctorResult(cmd, result)
			return nil
		},
	}
	cmd.Flags().BoolVar(&listGCPProfiles, "list-gcp-profiles", false, "list available gcloud named configurations without making network calls")
	return cmd
}

func doctorHasAuthFailure(result *doctor.Result) bool {
	if result == nil {
		return false
	}
	if result.Status != nil && strings.EqualFold(result.Status.Kind, "auth") {
		return true
	}
	for _, status := range result.Statuses {
		if strings.EqualFold(status.Kind, "auth") {
			return true
		}
	}
	return false
}

func printGCPProfiles(cmd *cobra.Command, report *providergcs.ProfileList) {
	if report == nil {
		return
	}
	configDir := report.ConfigDir
	if configDir == "" {
		configDir = "<unknown>"
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "gcp profiles (from %s/configurations/):\n", configDir)
	if len(report.Profiles) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  <none>")
		return
	}
	for _, profile := range report.Profiles {
		line := fmt.Sprintf("  %-14s account=%s  project=%s", profile.Name, emptyProfileField(profile.Account), emptyProfileField(profile.Project))
		if profile.CredentialFileOverride != "" {
			line += fmt.Sprintf("  credential_file_override=%s", profile.CredentialFileOverride)
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
	}
}

func emptyProfileField(value string) string {
	if value == "" {
		return "<none>"
	}
	return value
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
			printDoctorStatus(out, status)
		}
		return
	}

	if result.Status != nil {
		_, _ = fmt.Fprintf(out, "provider: %s\n", result.ProviderName)
		_, _ = fmt.Fprintf(out, "normalized: %s\n", result.Normalized)
		printDoctorStatus(out, *result.Status)
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

func printDoctorStatus(out io.Writer, status doctor.Status) {
	state := "ok"
	if !status.OK {
		state = status.Kind + " failure"
	}
	if status.Kind == "setup" {
		detail := strings.TrimRight(status.Detail, "\n")
		lines := strings.Split(detail, "\n")
		if len(lines) > 0 {
			_, _ = fmt.Fprintf(out, "%s: %s - %s\n", status.Provider, state, lines[0])
			for _, line := range lines[1:] {
				_, _ = fmt.Fprintf(out, "%s\n", line)
			}
			return
		}
	}
	_, _ = fmt.Fprintf(out, "%s: %s - %s\n", status.Provider, state, status.Detail)
}
