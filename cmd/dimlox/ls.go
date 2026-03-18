package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fulmenhq/dimlox/internal/provider"
	"github.com/fulmenhq/dimlox/internal/providers"
	"github.com/fulmenhq/dimlox/internal/uri"
	"github.com/spf13/cobra"
)

func lsCmd() *cobra.Command {
	var (
		longOutput bool
		showHash   bool
		recursive  bool
		limit      int
		format     string
	)

	cmd := &cobra.Command{
		Use:   "ls <uri>",
		Short: "List objects under an Azure, GCS, or local prefix",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			azProfile, _ := cmd.Flags().GetString("az-profile")
			gcpProject, _ := cmd.Flags().GetString("gcp-project")

			if format != "text" && format != "json" {
				return withExitCode(exitBadURI, "invalid --format %q (want text|json)", format)
			}

			p, _, err := providers.ForURI(cmd.Context(), args[0], providers.Options{AZProfile: azProfile, GCPProject: gcpProject})
			if err != nil {
				var unsupported *uri.ErrUnsupportedScheme
				if errors.Is(err, uri.ErrEmptyURI) || errors.As(err, &unsupported) {
					return withExitCode(exitBadURI, "%v", err)
				}
				return withExitCode(exitOperational, "%v", err)
			}

			objects := p.List(cmd.Context(), args[0], provider.ListOptions{Recursive: recursive, Limit: limit})
			if format == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				for meta, err := range objects {
					if err != nil {
						return withExitCode(exitOperational, "%v", err)
					}
					if err := enc.Encode(meta); err != nil {
						return withExitCode(exitOperational, "encode ls result: %v", err)
					}
				}
				return nil
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			if longOutput || showHash {
				_, _ = fmt.Fprintln(tw, "NAME\tSIZE\tLAST MODIFIED\tCONTENT-TYPE\tETAG\tHASH")
			}
			for meta, err := range objects {
				if err != nil {
					return withExitCode(exitOperational, "%v", err)
				}
				writeLSRow(tw, meta, longOutput, showHash)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().BoolVarP(&longOutput, "long", "l", false, "show size, content-type, last-modified, and ETag")
	cmd.Flags().BoolVar(&showHash, "hash", false, "also show MD5 or CRC32C from metadata")
	cmd.Flags().BoolVar(&recursive, "recursive", false, "list all objects under prefix")
	cmd.Flags().IntVar(&limit, "limit", 0, "stop after N results (default: unlimited)")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return cmd
}

func writeLSRow(w *tabwriter.Writer, meta *provider.ObjectMeta, longOutput, showHash bool) {
	name := meta.Name
	if meta.IsPrefix && !strings.HasSuffix(name, "/") {
		name += "/"
	}
	if !longOutput && !showHash {
		_, _ = fmt.Fprintln(w, name)
		return
	}
	modified := ""
	if !meta.LastModified.IsZero() {
		modified = meta.LastModified.Format(time.RFC3339)
	}
	hash := ""
	if showHash {
		switch {
		case len(meta.MD5) > 0:
			hash = hex.EncodeToString(meta.MD5)
		case meta.CRC32C != 0:
			hash = fmt.Sprintf("%08x", meta.CRC32C)
		}
	}
	_, _ = fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\t%s\n", name, meta.Size, modified, meta.ContentType, meta.ETag, hash)
}
