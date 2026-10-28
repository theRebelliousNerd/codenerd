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

var (
	docsCheckJSON    bool
	docsEvidenceJSON bool
)

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

// docsEvidenceCmd is the evidence view over the same packages. It leaves the
// structural grade above untouched and reports, per citation, what the
// document refers to and how far that resolved.
var docsEvidenceCmd = &cobra.Command{
	Use:   "evidence [pkg ...]",
	Short: "Report what Docs/architecture citations establish, by evidence kind",
	Long: `Reports every ADR witness and every backticked internal/, cmd/ or pkg/ path
citation in the selected Docs/architecture packages (no args means all), each
as a reference with a role (witness, planned_witness, planned_seam, negative,
historical, cited) and a resolution grade (declared, test_declared,
file_present, ambiguous, occurrence_only, unresolved, unobserved,
unsupported).

Declarations are read from the Go parse tree and Mangle Decl statements, not
from text: a name that occurs only in a comment or string is occurrence_only,
a name declared in several packages is ambiguous and every candidate is
listed, and a test witness must be a _test.go function with a signature
go test runs. A resolved reference is never an implementation claim, and no
test is executed: the manifest lists the evidence families this view does not
produce.

The report ends with a scan manifest: scope (workspace or subset), the
requested and observed packages, a generation digest of every input read,
deficits, and a terminal state. An incomplete or empty scan is never clean.
With --json each reference and its resolution print as consecutive JSON
objects and the manifest is last.

Exits 1 when the scan is incomplete or empty, or when any reference that
must resolve (every role except planned_witness, negative and historical)
did not, 0 otherwise.`,
	Example: `  nerd docs evidence
  nerd docs evidence --json mcp orient`,
	RunE: runDocsEvidence,
}

func init() {
	docsEvidenceCmd.Flags().BoolVar(&docsEvidenceJSON, "json", false,
		"print each reference and resolution as JSON objects, then the manifest")
	docsCmd.AddCommand(docsEvidenceCmd)
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

func runDocsEvidence(cmd *cobra.Command, args []string) error {
	ws := workspace
	if ws == "" {
		ws, _ = os.Getwd()
	}
	clean, err := docsEvidenceInto(cmd.OutOrStdout(), ws, args, docsEvidenceJSON)
	if err != nil {
		return err
	}
	if !clean {
		os.Exit(1)
	}
	return nil
}

// docsEvidenceInto writes the evidence view of pkgs under workspaceRoot to w
// and reports whether the scan was clean: complete, with nothing failing.
func docsEvidenceInto(w io.Writer, workspaceRoot string, pkgs []string, asJSON bool) (bool, error) {
	ev, err := docscheck.NewChecker(workspaceRoot).Evidence(pkgs)
	if err != nil {
		return false, fmt.Errorf("docs evidence: %w", err)
	}
	if asJSON {
		if err := docscheck.WriteEvidenceJSON(w, ev); err != nil {
			return false, fmt.Errorf("docs evidence: %w", err)
		}
	} else if _, err := io.WriteString(w, docscheck.EvidenceText(ev)); err != nil {
		return false, fmt.Errorf("docs evidence: %w", err)
	}
	return ev.Clean(), nil
}
