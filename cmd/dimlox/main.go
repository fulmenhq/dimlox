package main

import (
	"context"
	"fmt"
	"os"

	"github.com/fulmenhq/dimlox/internal/appctx"
	"github.com/fulmenhq/gofulmen/foundry"
	"github.com/fulmenhq/gofulmen/logging"
	"github.com/fulmenhq/gofulmen/signals"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// Injected at build time via -ldflags.
var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	log, err := logging.NewCLI("dimlox")
	if err != nil {
		fmt.Fprintf(os.Stderr, "dimlox: failed to initialize logger: %v\n", err)
		os.Exit(foundry.ExitFailure)
	}
	defer log.Sync() //nolint:errcheck

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel the root context when a shutdown signal arrives so every command's
	// context unblocks and in-flight streaming operations stop cleanly.
	signals.OnShutdown(func(_ context.Context) error {
		cancel()
		return nil
	})
	if err := signals.EnableDoubleTap(signals.DoubleTapConfig{}); err != nil {
		log.Warn("double-tap Ctrl+C unavailable on this platform", zap.Error(err))
	}
	go signals.Listen(ctx) //nolint:errcheck

	root := rootCmd()
	root.SetContext(appctx.WithLogger(ctx, log))
	if err := root.Execute(); err != nil {
		exitWithError(err)
	}
}

func rootCmd() *cobra.Command {
	var (
		azProfile  string
		gcpProfile string
		gcpProject string
		landingDir string
		logLevel   string
	)

	root := &cobra.Command{
		Use:   "dimlox",
		Short: "Data in motion - large-file transfer, inspection, and splitting across clouds",
		Long: `dimlox moves, inspects, and splits large files across Azure Blob Storage,
Google Cloud Storage, and local filesystems without loading them into memory.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       formatVersion(),

		// PersistentPreRunE runs before every subcommand. It applies --log-level
		// when provided, and falls back to a default logger when none is in the
		// context (test callers that construct rootCmd directly).
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			log := appctx.Logger(cmd.Context())
			if log == nil {
				var err error
				log, err = logging.NewCLI("dimlox")
				if err != nil {
					return fmt.Errorf("initialize logger: %w", err)
				}
				cmd.SetContext(appctx.WithLogger(cmd.Context(), log))
			}
			if logLevel != "" {
				log.SetLevel(logging.ParseSeverity(logLevel))
			}
			return nil
		},
	}
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")

	root.PersistentFlags().StringVar(&azProfile, "az-profile", "",
		"Azure CLI profile name (sets AZURE_CONFIG_DIR via AZURE_PROFILES_DIR or ~/.azure-profiles/<name>)")
	root.PersistentFlags().StringVar(&gcpProfile, "gcp-profile", "",
		"gcloud named configuration for GCS endpoints (respects CLOUDSDK_CONFIG when set)")
	root.PersistentFlags().StringVar(&gcpProject, "gcp-project", defaultGCPProject(),
		"GCP project ID for requester-pays buckets (also: GCLOUD_PROJECT env)")
	root.PersistentFlags().StringVar(&landingDir, "landing", os.Getenv("DIMLOX_LANDING_DIR"),
		"landing area for large files (also: DIMLOX_LANDING_DIR env)")
	root.PersistentFlags().StringVar(&logLevel, "log-level", "",
		"log verbosity: trace, debug, info, warn, error (default: info)")

	root.AddCommand(versionCmd())
	root.AddCommand(doctorCmd())
	root.AddCommand(lsCmd())
	root.AddCommand(getCmd())
	root.AddCommand(putCmd())
	root.AddCommand(cpCmd())
	root.AddCommand(inspectCmd())
	root.AddCommand(splitCmd())

	// Phase 2:
	// Phase 3:
	// Phase 4:

	return root
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "dimlox %s\n", formatVersion())
		},
	}
}

func formatVersion() string {
	return fmt.Sprintf("%s (commit %s, built %s)", version, gitCommit, buildTime)
}

func defaultGCPProject() string {
	if project := os.Getenv("GCLOUD_PROJECT"); project != "" {
		return project
	}
	return os.Getenv("GOOGLE_CLOUD_PROJECT")
}

func mbToBytes(mb int64) int64 {
	if mb <= 0 {
		return 0
	}
	return mb * 1024 * 1024
}

func selectedGCPProject(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	flag := cmd.Flags().Lookup("gcp-project")
	if flag == nil || !flag.Changed {
		return ""
	}
	value, _ := cmd.Flags().GetString("gcp-project")
	return value
}
