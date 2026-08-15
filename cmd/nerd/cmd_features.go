package main

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"codenerd/internal/features"

	"github.com/spf13/cobra"
)

var (
	featuresJSON   bool
	featuresSchema bool
)

var featuresCmd = &cobra.Command{
	Use:   "features",
	Short: "Show which feature flags are in effect and why",
	Long: `Lists every codeNERD feature toggle with the value actually in force and
the source that decided it:

  env         the canonical CODENERD_* variable overrode everything else
  legacy-env  a deprecated NERD_* variable decided it (rename it)
  config      the features block in .nerd/config.json decided it
  default     none was set, so the compile-time default applies

Precedence is env → legacy-env → config → default.

Use --schema to print a documented JSON snippet for the features block of
.nerd/config.json.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()

		if featuresSchema {
			fmt.Fprint(out, features.ConfigSchemaJSON())
			return nil
		}

		// Resolve against the same registry the runtime reads, after the
		// config load that root's PersistentPreRun performs — otherwise this
		// command would report defaults while the session uses config values.
		flags := features.Resolved()
		deprecations := features.Deprecations()

		if featuresJSON {
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]any{
				"flags":              flags,
				"fast_scan_workers":  features.FastScanWorkers(),
				"fast_ast_max_bytes": features.FastASTMaxBytes(),
				"deprecations":       deprecations,
			})
		}

		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "FLAG\tVALUE\tSOURCE\tDEFAULT\tENV VAR")
		for _, f := range flags {
			envVar := f.EnvVar
			if f.LegacyEnvVar != "" {
				envVar += " (legacy: " + f.LegacyEnvVar + ")"
			}
			fmt.Fprintf(w, "%s\t%t\t%s\t%t\t%s\n", f.Name, f.Value, f.Source, f.Default, envVar)
		}
		if err := w.Flush(); err != nil {
			return err
		}

		fmt.Fprintln(out)
		fmt.Fprintf(out, "fast_scan_workers   %d (0 = call site default)\n", features.FastScanWorkers())
		fmt.Fprintf(out, "fast_ast_max_bytes  %d (0 = call site default)\n", features.FastASTMaxBytes())

		// Deprecation notices go last so they are the final thing on screen;
		// a legacy variable that is set but shadowed is the case most likely
		// to send an operator debugging the wrong knob.
		if len(deprecations) > 0 {
			fmt.Fprintln(out)
			for _, msg := range deprecations {
				fmt.Fprintf(out, "warning: %s\n", msg)
			}
		}
		return nil
	},
}

func init() {
	featuresCmd.Flags().BoolVar(&featuresJSON, "json", false, "emit resolved flags as JSON")
	featuresCmd.Flags().BoolVar(&featuresSchema, "schema", false,
		"print the JSON schema snippet for the features block of .nerd/config.json")
}
