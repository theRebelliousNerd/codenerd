// Package main implements the codeNERD CLI commands.
//
// This file carries `nerd docs check`, the tracked form of the R6
// documentation grade. Campaigns that rewrite Docs/architecture/<pkg> were
// judged by scripts/r6_structcheck.py, which is gitignored and prints prose
// a remediation task cannot be pointed at; internal/docscheck ports every
// one of its checks to Go with identical judgements and structured
// problems, and this command is its operator surface.
package main

import (
	"fmt"
	"io"
	"os"

	"codenerd/internal/docscheck"

	"github.com/spf13/cobra"
)

var docsCheckJSON bool

// docsCmd groups documentation checks. Register docsCmd on the root
// command; subcommands attach here.
var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Architecture documentation checks",
	Long:  "Checks over Docs/architecture: the deterministic R6 structural grade.",
	RunE:  parentGroupRunE,
}

// docsCheckCmd grades architecture-doc packages the way the R6 acceptance
// does: front-matter, required slots, the gap table, ADR witnesses, and the
// plan/shipped layer rule — exiting 1 when any problem exists.
var docsCheckCmd = &cobra.Command{
	Use:   "check [pkg ...]",
	Short: "Grade Docs/architecture packages against the R6 structural bar",
	Long: `Grades one or more Docs/architecture packages against the R6 structural
bar (no args means every package directory): every .md file's front-matter,
the required slots, the 03-GAP-ANALYSIS.md gap table, every adr/*.md
witness, and the rule that a plan layer exists and the package reaches
shipped.

The human report matches scripts/r6_structcheck.py line for line, so a
campaign can swap its --accept witness to this command without re-baselining.
With --json each problem prints as one JSON object per line
(package/file/code/message, the file workspace-relative) followed by a final
summary object carrying packages/files/problems counts — the summary is the
one object without a "code" key.

Exits 1 when any problem exists, 0 when the graded packages are clean.`,
	Example: `  nerd docs check
  nerd docs check features
  nerd docs check --json features campaign`,
	RunE: runDocsCheck,
}

func init() {
	docsCheckCmd.Flags().BoolVar(&docsCheckJSON, "json", false,
		"print one JSON object per problem per line plus a final summary object")
	docsCmd.AddCommand(docsCheckCmd)
}

func runDocsCheck(cmd *cobra.Command, args []string) error {
	ws := workspace
	if ws == "" {
		ws, _ = os.Getwd()
	}
	total, err := docsCheckInto(cmd.OutOrStdout(), ws, args, docsCheckJSON)
	if err != nil {
		return err
	}
	if total > 0 {
		os.Exit(1)
	}
	return nil
}

// docsCheckInto grades pkgs under workspaceRoot and writes the report to w,
// human-shaped by default or JSON with asJSON. It reports the total problem
// count so RunE can exit 1 on any finding, as the script does. Failures to
// grade (unknown package, unreadable tree) are errors, not empty reports:
// a grade that cannot read its input must not print a clean bill.
func docsCheckInto(w io.Writer, workspaceRoot string, pkgs []string, asJSON bool) (int, error) {
	reports, err := docscheck.NewChecker(workspaceRoot).Check(pkgs)
	if err != nil {
		return 0, fmt.Errorf("docs check: %w", err)
	}
	if asJSON {
		if err := docscheck.WriteJSON(w, reports); err != nil {
			return 0, fmt.Errorf("docs check: %w", err)
		}
	} else if _, err := io.WriteString(w, docscheck.Text(reports)); err != nil {
		return 0, fmt.Errorf("docs check: %w", err)
	}
	return docscheck.Summarize(reports).Problems, nil
}
