package core

import (
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/mangle"

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
	"activation":                     "Decl /string; rules derive tool names (/name) and refs",
	"beats":                          ":lt over unbounded terms",
	"campaign_learning":              "pattern/fact slots",
	"coder_blocked_reason":           "Decl /string; has_edit_block yields /any",
	"compilation_error":              "Decl /string; category is /name",
	"context_atom":                   "Decl /string; activation is polymorphic",
	"delegate_task":                  "Decl /string description; rules pass /name",
	"escalation_needed":              "Decl /string subject; rules pass /name",
	"intelligence_relevant_strategy": "campaign id inferred /any",
	"is_error_checked":               "Decl /string var; data-flow columns are /name",
	"is_guarded":                     "Decl /string var; data-flow columns are /name",
	"learning_signal":                "Decl /string rule id; rules pass /name",
	"missing_hypothesis":             "Decl /string; symptom is /name",
	"path_of_length":                 "fn:plus over unbounded length",
	"potential_score":                "fn:plus over unbounded priority",
	"priority_diagnostic":            "coder_target/coder_impacted file slots",
	"promote_to_long_term":           "Decl /string value; rules pass /name or /any",
	"raw_finding":                    "Decl /string file/message; sources inferred /any",
	"shard_context_atom":             "Decl /string atom; recall yields /any",
	"system_invariant_violated":      "Decl /number timestamp; invariant_value yields /any",
	"tool_lifecycle":                 "Decl /string tool; missing_tool_for yields /name",
	"unchecked_error":                "Decl /string func; data-flow columns are /name",
	"unsafe_deref":                   "Decl /string var; data-flow columns are /name",
}

// bootCorpusBoundsMismatches analyzes every clause of the production program
// alone, against all of its Decls, with ErrorForBoundsMismatch, and returns the
// head predicates that fail. A clause the analyzer cannot decide alone is
// skipped, as checkCandidateBounds does.
//
// Each clause's analysis is independent of every other's, so the clauses are
// shared out over workers. Every worker parses its own copy of the program:
// the analyzer is handed the unit's Decls and is not promised to leave them
// untouched, so no two analyses share one.
func bootCorpusBoundsMismatches(t *testing.T) map[string]string {
	t.Helper()
	k := setupMockKernel(t)
	var program strings.Builder
	k.mu.Lock()
	k.writeProgramLocked(&program)
	k.mu.Unlock()
	text := program.String()
	parseOwn := func() (parse.SourceUnit, error) {
		return mangle.ParseUnit(strings.NewReader(text))
	}
	unit, err := parseOwn()
	if err != nil {
		t.Fatalf("parse boot program: %v", err)
	}

	// verdicts[i] is clause i's mismatch, or "" when it has none.
	type verdict struct{ pred, why string }
	verdicts := make([]verdict, len(unit.Clauses))
	next := make(chan int)
	var wg sync.WaitGroup
	for w := range min(runtime.GOMAXPROCS(0), 16) {
		own := unit
		if w > 0 {
			if own, err = parseOwn(); err != nil {
				t.Fatalf("parse boot program: %v", err)
			}
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				clause := own.Clauses[i]
				single := own
				single.Clauses = []ast.Clause{clause}
				units := []parse.SourceUnit{single}
				_, err := analysis.AnalyzeAndCheckBounds(units, nil, analysis.ErrorForBoundsMismatch)
				if err == nil {
					continue
				}
				if _, baseErr := analysis.AnalyzeAndCheckBounds(units, nil, analysis.NoBoundsChecking); baseErr != nil {
					continue
				}
				verdicts[i] = verdict{clause.Head.Predicate.Symbol, strings.ReplaceAll(err.Error(), "\n", " ")}
			}
		}()
	}
	for i := range unit.Clauses {
		next <- i
	}
	close(next)
	wg.Wait()

	// In clause order, as the serial loop ran: a predicate with several
	// failing clauses reports its last one.
	found := map[string]string{}
	for _, v := range verdicts {
		if v.pred != "" {
			found[v.pred] = v.why
		}
	}
	return found
}

// TestBootCorpusBoundsDisagreementsOnlyShrink is the shadow analysis the
// feature-adoption audit asked for before strict bounds can be enforced on the
// boot corpus: a new rule that contradicts a Decl fails here, and so does an
// inventory entry that has been repaired.
func TestBootCorpusBoundsDisagreementsOnlyShrink(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("analyzes every clause of the boot program alone")
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
