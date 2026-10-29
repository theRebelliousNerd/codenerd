package main

import (
	"fmt"

	"codenerd/internal/world"

	"github.com/spf13/cobra"
)

// worldCmd groups operator-facing world-model commands.
//
// The runbook text is owned by internal/world (world.ScanRunbook) so the CLI
// help, the chat help and the docs cannot describe three different scanners.
// Register worldCmd on the root command; `nerd scan --help` should also print
// world.ScanRunbook after its own Long text.
var worldCmd = &cobra.Command{
	Use:   "world",
	Short: "Inspect and operate the world model",
	Long:  "Commands for the world model: what a scan owns, and how to diagnose a stale index.",
}

var worldRunbookCmd = &cobra.Command{
	Use:   "runbook",
	Short: "Print the world scan operator runbook",
	Long:  "Prints what fast and deep scans do, which predicates each writer owns, and how to diagnose a stale or broken index.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), world.ScanRunbook)
		return err
	},
}

var worldPredicatesCmd = &cobra.Command{
	Use:   "predicates",
	Short: "List world EDB predicates by owning writer",
	Long: `Lists every world-model predicate grouped by the writer that owns it.

Ownership decides what a scan may delete: a writer may replace only the
predicates the same pass re-asserts.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		for _, g := range world.OwnerGroups {
			if _, err := fmt.Fprintf(out, "%s — %s\n", g.Name, g.Note); err != nil {
				return err
			}
			for _, p := range g.Predicates {
				if _, err := fmt.Fprintf(out, "  %s\n", p); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(out); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	worldCmd.AddCommand(worldRunbookCmd)
	worldCmd.AddCommand(worldPredicatesCmd)
}
