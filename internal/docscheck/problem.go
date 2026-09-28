// Package docscheck is the tracked Go port of scripts/r6_structcheck.py, the
// deterministic half of the R6 architecture-docs grade: per package directory
// under Docs/architecture it checks front-matter, required slots, the gap
// table, ADR witnesses, and the planned/shipped layer rule.
//
// The script is the specification and this package mirrors its judgements
// exactly — same checks, same human report lines — while returning each
// finding as a structured Problem the harness can point a remediation task
// at. Where the script hardcodes the workspace root, the caller supplies it
// (see Checker); where the script shells out to `git grep` for witnesses,
// the checker scans the worktree in Go.
package docscheck

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"codenerd/internal/types"
)

// Code is the stable short name of one R6 structural check. Codes are part
// of the harness contract — remediation tasks and the later Mangle wiring
// match on them — so they change only when the R6 bar itself changes.
type Code string

// One code per judgement in scripts/r6_structcheck.py.
const (
	// Front-matter (every .md file).
	CodeMissingFrontMatter      Code = "missing_front_matter"
	CodeBadDocClass             Code = "bad_doc_class"
	CodeBadImplementationStatus Code = "bad_implementation_status"
	CodeBadLastVerified         Code = "bad_last_verified"
	CodeMissingVerifiedAgainst  Code = "missing_verified_against"
	// Gap table (03-GAP-ANALYSIS.md).
	CodeMissingGapTable Code = "missing_gap_table"
	CodeEmptyGapTable   Code = "empty_gap_table"
	CodeGapRowWithoutID Code = "gap_row_without_id"
	CodeVagueGapExit    Code = "vague_gap_exit"
	// ADR witnesses (adr/*.md).
	CodeNoWitnessLine     Code = "no_witness_line"
	CodeWitnessUnresolved Code = "witness_unresolved"
	// Package shape.
	CodeMissingSlot           Code = "missing_slot"
	CodeMissingADRDir         Code = "missing_adr_dir"
	CodeMissingCapabilitySpec Code = "missing_capability_spec"
	// Layer rule: a plan layer must exist, and the package must reach
	// shipped — "no shipped layer" in the script means the package never
	// got past planned, hence only_planned.
	CodeNoPlanLayer Code = "no_plan_layer"
	CodeOnlyPlanned Code = "only_planned"
)

// Problem is one R6 structural finding.
type Problem struct {
	// Package is the Docs/architecture directory the finding belongs to.
	Package string `json:"package"`
	// File is the workspace-relative path the finding points at, slash
	// separated on every OS so facts and JSON stay stable. Per-file
	// findings name the file (Docs/architecture/<pkg>/<rel>); a missing
	// slot names the file that should exist; package-scope findings (no
	// capability spec, no plan layer, never shipped) name the package
	// directory itself, which is the scope the remediation must look at.
	File string `json:"file"`
	// Code is the stable check name.
	Code Code `json:"code"`
	// Message is the script-identical human line (package-relative, as the
	// script prints it), so the text report matches r6_structcheck.py
	// line for line.
	Message string `json:"message"`
}

// Fact renders the problem as doc_problem(Package, File, Code, Message) with
// the code as a /name so policy rules can match it. Package, File and
// Message are wrapped in MangleString explicitly: Fact.ToAtom infers a bare
// Go string's constant type from its shape, and a message that happened to
// start with "/" would otherwise land as a name no rule expects.
func (p Problem) Fact() types.Fact {
	return types.Fact{
		Predicate: "doc_problem",
		Args: []any{
			types.MangleString(p.Package),
			types.MangleString(p.File),
			types.MangleAtom("/" + string(p.Code)),
			types.MangleString(p.Message),
		},
	}
}

// Facts converts problems to the facts a later lane asserts into the
// kernel. There is deliberately no .mg Decl here — that wiring lands
// separately.
func Facts(problems []Problem) []types.Fact {
	facts := make([]types.Fact, 0, len(problems))
	for _, p := range problems {
		facts = append(facts, p.Fact())
	}
	return facts
}

// PackageReport is one package's grade: how many .md files were read and
// what was found, in the script's order (per-file findings sorted by path,
// then package-shape findings in SLOTS order, then the layer rule).
type PackageReport struct {
	Package  string
	Files    int
	Problems []Problem
}

// Facts converts the report's problems, in order.
func (r PackageReport) Facts() []types.Fact {
	return Facts(r.Problems)
}

// Count returns the number of problems in the report.
func (r PackageReport) Count() int {
	return len(r.Problems)
}

// Text renders the report exactly as the script prints it: one header line
// ("{pkg:<16}{n:>4} md  {len:>3} problems") and one indented line per
// problem, each newline-terminated.
func (r PackageReport) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-16s%4d md  %3d problems\n", r.Package, r.Files, len(r.Problems))
	for _, p := range r.Problems {
		fmt.Fprintf(&b, "    %s\n", p.Message)
	}
	return b.String()
}

// Text renders every report in order, as the script's main loop does.
func Text(reports []PackageReport) string {
	var b strings.Builder
	for _, r := range reports {
		b.WriteString(r.Text())
	}
	return b.String()
}

// Summary counts a whole run for the JSON trailer.
type Summary struct {
	Packages int `json:"packages"`
	Files    int `json:"files"`
	Problems int `json:"problems"`
}

// Summarize counts packages, .md files read, and problems across reports.
func Summarize(reports []PackageReport) Summary {
	s := Summary{Packages: len(reports)}
	for _, r := range reports {
		s.Files += r.Files
		s.Problems += len(r.Problems)
	}
	return s
}

// WriteJSON writes one JSON object per problem per line, in report order,
// then a final summary object. The summary is the one object without a
// "code" key: problems carry package/file/code/message, the trailer carries
// packages/files/problems counts. HTML escaping is off so a message keeps
// its script spelling byte for byte.
func WriteJSON(w io.Writer, reports []PackageReport) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for _, r := range reports {
		for _, p := range r.Problems {
			if err := enc.Encode(p); err != nil {
				return fmt.Errorf("encode doc problem: %w", err)
			}
		}
	}
	if err := enc.Encode(Summarize(reports)); err != nil {
		return fmt.Errorf("encode doc summary: %w", err)
	}
	return nil
}
