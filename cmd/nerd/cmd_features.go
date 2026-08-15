package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"codenerd/internal/features"

	"github.com/spf13/cobra"
)

var featuresJSON bool

var featuresCmd = &cobra.Command{
	Use:   "features",
	Short: "Show which feature flags are in effect and why",
	Long: `Lists every codeNERD feature toggle with the value actually in force and
the source that decided it:

  env      an environment variable overrode everything else
  config   the features block in .nerd/config.json decided it
  default  neither was set, so the compile-time default applies

Precedence is env → config → default.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve against the same registry the runtime reads, after the
		// config load that root's PersistentPreRun performs — otherwise this
		// command would report defaults while the session uses config values.
		flags := features.Resolved()

		if featuresJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]any{
				"flags":              flags,
				"fast_scan_workers":  features.FastScanWorkers(),
				"fast_ast_max_bytes": features.FastASTMaxBytes(),
			})
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "FLAG\tVALUE\tSOURCE\tDEFAULT\tENV VAR")
		for _, f := range flags {
			fmt.Fprintf(w, "%s\t%t\t%s\t%t\t%s\n", f.Name, f.Value, f.Source, f.Default, f.EnvVar)
		}
		if err := w.Flush(); err != nil {
			return err
		}

		fmt.Println()
		fmt.Printf("fast_scan_workers   %d (0 = call site default)\n", features.FastScanWorkers())
		fmt.Printf("fast_ast_max_bytes  %d (0 = call site default)\n", features.FastASTMaxBytes())
		return nil
	},
}

func init() {
	featuresCmd.Flags().BoolVar(&featuresJSON, "json", false, "emit resolved flags as JSON")
}
