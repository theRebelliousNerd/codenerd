package core

import (
	"sort"
	"strings"
	"testing"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/parse"
)

// knownBoundsDisagreements are the boot-corpus predicates whose rules the
// upstream static bounds analyzer rejects against their Decls. The kernel
// still analyzes with NoBoundsChecking because the analyzer stops at the first
// mismatch of a whole program; this per-clause inventory reports every one, so
// the set can only shrink. Each entry is a Decl/rule disagreement to repair
// (widen the Decl to what the rules derive, or narrow the rule); none is known
// to corrupt a fact. Removing an entry once repaired is required: the test
// fails on a stale one.
var knownBoundsDisagreements = map[string]string{
	"activation":                          "Decl /string; rules derive tool names (/name) and refs",
	"apply_convention":                    "Decl /string; derived from language_convention's /name",
	"beats":                               ":lt over unbounded terms",
	"campaign_learning":                   "pattern/fact slots",
	"coder_blocked_reason":                "Decl /string; has_edit_block yields /any",
	"compilation_error":                   "Decl /string; category is /name",
	"context_atom":                        "Decl /string; activation is polymorphic",
	"delegate_task":                       "Decl /string description; rules pass /name",
	"escalation_needed":                   "Decl /string subject; rules pass /name",
	"intelligence_is_core_infrastructure": "path inferred /any through a count",
	"intelligence_relevant_strategy":      "campaign id inferred /any",
	"is_error_checked":                    "Decl /string var; data-flow columns are /name",
	"is_guarded":                          "Decl /string var; data-flow columns are /name",
	"learning_signal":                     "Decl /string rule id; rules pass /name",
	"missing_hypothesis":                  "Decl /string; symptom is /name",
	"path_of_length":                      "fn:plus over unbounded length",
	"potential_score":                     "fn:plus over unbounded priority",
	"priority_diagnostic":                 "coder_target/coder_impacted file slots",
	"promote_to_long_term":                "Decl /string value; rules pass /name or /any",
	"raw_finding":                         "Decl /string file/message; sources inferred /any",
	"review_finding_with_id":              "review id inferred /any",
	"shard_context_atom":                  "Decl /string atom; recall yields /any",
	"specialist_outperforms":              "reasoning_trace slot types",
	"symbol_reachable_safe":               "path_of_length is unbounded",
	"system_invariant_violated":           "Decl /number timestamp; invariant_value yields /any",
	"tool_generation_hint":                "Decl /string tool; capabilities are /name",
	"tool_lifecycle":                      "Decl /string tool; missing_tool_for yields /name",
	"unchecked_error":                     "Decl /string func; data-flow columns are /name",
	"unsafe_deref":                        "Decl /string var; data-flow columns are /name",
}

// bootCorpusBoundsMismatches analyzes every clause of the production program
// alone, against all of its Decls, with ErrorForBoundsMismatch, and returns the
// head predicates that fail. A clause the analyzer cannot decide alone is
// skipped, as checkCandidateBounds does.
func bootCorpusBoundsMismatches(t *testing.T) map[string]string {
	t.Helper()
	k := setupMockKernel(t)
	var program strings.Builder
	k.mu.Lock()
	k.writeProgramLocked(&program)
	k.mu.Unlock()
	unit, err := parse.Unit(strings.NewReader(program.String()))
	if err != nil {
		t.Fatalf("parse boot program: %v", err)
	}
	found := map[string]string{}
	for _, clause := range unit.Clauses {
		single := unit
		single.Clauses = []ast.Clause{clause}
		units := []parse.SourceUnit{single}
		_, err := analysis.AnalyzeAndCheckBounds(units, nil, analysis.ErrorForBoundsMismatch)
		if err == nil {
			continue
		}
		if _, baseErr := analysis.AnalyzeAndCheckBounds(units, nil, analysis.NoBoundsChecking); baseErr != nil {
			continue
		}
		found[clause.Head.Predicate.Symbol] = strings.ReplaceAll(err.Error(), "\n", " ")
	}
	return found
}

// TestBootCorpusBoundsDisagreementsOnlyShrink is the shadow analysis the
// feature-adoption audit asked for before strict bounds can be enforced on the
// boot corpus: a new rule that contradicts a Decl fails here, and so does an
// inventory entry that has been repaired.
func TestBootCorpusBoundsDisagreementsOnlyShrink(t *testing.T) {
	if testing.Short() {
		t.Skip("analyzes every clause of the boot program alone (~40s)")
	}
	found := bootCorpusBoundsMismatches(t)

	var added, stale []string
	for pred, why := range found {
		if _, known := knownBoundsDisagreements[pred]; !known {
			added = append(added, pred+": "+why)
		}
	}
	for pred := range knownBoundsDisagreements {
		if _, still := found[pred]; !still {
			stale = append(stale, pred)
		}
	}
	sort.Strings(added)
	sort.Strings(stale)
	if len(added) > 0 {
		t.Errorf("rules contradict their Decl bounds (repair the Decl or the rule):\n%s", strings.Join(added, "\n"))
	}
	if len(stale) > 0 {
		t.Errorf("repaired; remove from knownBoundsDisagreements: %s", strings.Join(stale, ", "))
	}
}
