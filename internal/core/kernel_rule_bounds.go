package core

import (
	"fmt"
	"strings"

	"codenerd/internal/logging"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/parse"
)

// =============================================================================
// STATIC BOUNDS FOR GENERATED RULES
// =============================================================================
//
// Both RealKernel and internal/mangle.Engine analyze with AnalyzeOneUnit,
// whose upstream default is NoBoundsChecking: a Decl's bounds are parsed and
// never checked against the rules that use the predicate. Switching the whole
// boot corpus to strict checking is not an option today -- the analyzer stops
// at the first mismatch, and the corpus carries Decl/rule disagreements of its
// own (activation/2 is declared over strings and derived over tool names).
//
// What IS owned end to end is a generated rule: a candidate the Legislator,
// autopoiesis or the feedback loop proposes. So the candidate alone is
// analyzed with ErrorForBoundsMismatch, against the analyzed program's Decls
// as the global declarations. A rule that joins a /name column to a /string
// column, or derives a number into a name slot, compiles fine and derives
// nothing; this gate rejects it with the mismatch the proposer can repair.
//
// The candidate's composition with the rest of the program is still validated
// by the sandbox compile; this check runs after it. If the candidate alone is
// not analyzable (it reads a predicate only other rules define, say), the
// static check cannot decide and stays out of the way.

// checkCandidateBounds statically type-checks a candidate rule. decls are the
// current program's desugared declarations.
func checkCandidateBounds(rule string, decls map[ast.PredicateSym]*ast.Decl) error {
	if len(decls) == 0 || strings.TrimSpace(rule) == "" {
		return nil
	}
	unit, err := parseUnit(strings.NewReader(rule))
	if err != nil {
		return nil // the sandbox compile reports parse errors
	}
	extra := candidateBoundsDecls(decls, unit)
	units := []parse.SourceUnit{unit}
	if _, err := analysis.AnalyzeAndCheckBounds(units, extra, analysis.ErrorForBoundsMismatch); err != nil {
		if _, baseErr := analysis.AnalyzeAndCheckBounds(units, extra, analysis.NoBoundsChecking); baseErr != nil {
			logging.KernelDebug("checkCandidateBounds: candidate not analyzable in isolation, static bounds undecided: %v", baseErr)
			return nil
		}
		return fmt.Errorf("rule violates declared type bounds: %w", err)
	}
	return nil
}

// candidateBoundsDecls turns the program's Decls into the analyzer's global
// declarations: every one except those the candidate declares itself. A
// nullary Decl without `bound []` desugars to no bound tuple at all, which the
// upstream bounds analyzer refuses ("no bound decls"); it is given the one
// empty tuple it means.
func candidateBoundsDecls(decls map[ast.PredicateSym]*ast.Decl, unit parse.SourceUnit) map[ast.PredicateSym]ast.Decl {
	own := make(map[ast.PredicateSym]struct{}, len(unit.Decls))
	for _, d := range unit.Decls {
		own[d.DeclaredAtom.Predicate] = struct{}{}
	}
	extra := make(map[ast.PredicateSym]ast.Decl, len(decls))
	for sym, d := range decls {
		if d == nil {
			continue
		}
		if _, mine := own[sym]; mine {
			continue
		}
		decl := *d
		if sym.Arity == 0 && len(decl.Bounds) == 0 {
			decl.Bounds = []ast.BoundDecl{ast.NewBoundDecl()}
		}
		extra[sym] = decl
	}
	return extra
}

// checkCandidateBoundsAgainstProgram runs checkCandidateBounds against this
// kernel's analyzed program. Before the first analysis there are no Decls to
// check against, and the sandbox compile has already judged the rule.
func (k *RealKernel) checkCandidateBoundsAgainstProgram(rule string) error {
	info := k.GetProgramInfo()
	if info == nil {
		return nil
	}
	return checkCandidateBounds(rule, info.Decls)
}
