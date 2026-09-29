package session

import (
	"path/filepath"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

const witnessTestBothFailing = witnessTestAdd + "\nfunc TestSub(t *testing.T) {\n\tif Sub(5, 3) != 99 {\n\t\tt.Fatal(\"sub\")\n\t}\n}\n"

// Changed Add+Sub, the run executes both but fails: execution under a red
// gate proves the code ran, not that it works, so the red /test gate
// withholds the verdict. The run did execute both elements -- the element
// debt is empty -- which is what makes this the gate's case and not the
// coverage's.
func TestWitnessStep6_FailingRunWithholdsViaTheRedGate(t *testing.T) {
	e, result := witnessVerdict(t, witnessTestBothFailing)

	if result.TurnOutcome != types.MangleAtom("/unverified") {
		t.Fatalf("TurnOutcome = %q, want /unverified: the tests failed", result.TurnOutcome)
	}
	found := false
	for _, m := range result.MissingEvidence {
		if m == "/tests_not_green" {
			found = true
		}
	}
	if !found {
		t.Fatalf("MissingEvidence = %v, want it to name /tests_not_green", result.MissingEvidence)
	}
	if got := queryCount(t, e, "turn_element_uncovered"); got != 0 {
		t.Fatalf("turn_element_uncovered = %d, want 0: the failing run executed both elements", got)
	}
	// The red gate is the whole story: naming every changed element as
	// unwitnessed on top of it would point the model away from the failure.
	if containsString(result.MissingEvidence, "/change_unwitnessed") {
		t.Fatalf("MissingEvidence = %v, must not name /change_unwitnessed while the /test gate is red", result.MissingEvidence)
	}
	if got := queryCount(t, e, "turn_unwitnessed"); got != 0 {
		t.Fatalf("turn_unwitnessed = %d, want 0 under a red /test gate", got)
	}
}

const (
	witnessInitBefore = "package calc\n\nvar flag = false\n"
	witnessInitAfter  = "package calc\n\nvar flag = false\n\nfunc init() {\n\tif flag {\n\t\tprintln(\"never\")\n\t}\n}\n"
	witnessInitTest   = "package calc\n\nimport \"testing\"\n\nfunc TestFlag(t *testing.T) {\n\tif flag {\n\t\tt.Fatal(\"flag\")\n\t}\n}\n"
)

// A changed init body the run never executes stays file-level debt: a changed
// init asserts no turn_changed_element (one file may declare several, and
// fn:<pkg>.init would name them all), so the witness owes nothing for it and
// turn_uncovered still names the file. The witness names nothing.
func TestWitnessStep6_InitBodyStaysFileLevel(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and tests a throwaway package")
	}
	written := filepath.Join("sub", "calc.go")
	ws := writeBaselineModule(t, map[string]string{
		"go.mod":           "module witnessprobe\n\ngo 1.21\n",
		"sub/calc.go":      witnessInitAfter,
		"sub/calc_test.go": witnessInitTest,
	})
	v, blocks := verifyTestsWithCoverage(t.Context(), ws, packagesForPaths([]string{written}), []string{written})
	if !v.Ran || !v.OK {
		t.Fatalf("coverage run Ran=%v OK=%v output=%q", v.Ran, v.OK, v.Output)
	}
	result := writeTurnResult()
	result.WrittenPaths = []string{written}
	result.PreWriteContents = map[string]PreImage{written: existed(witnessInitBefore)}
	result.UncoveredBlocks = narrowToChangedLines(ws, result, blocks)
	result.TestCheck = v
	result.UntestedPaths = untestedWithoutCoverageOnDisk(ws, result.WrittenPaths)
	e := newObligationExec(t)
	e.config.WorkspaceRoot = ws
	e.assertTurnEvidence(testTurn, "/fix", result)
	e.captureTurnOutcome(testTurn, result, nil)
	t.Cleanup(e.cleanupTurnFacts)

	if len(result.UncoveredBlocks) == 0 {
		t.Fatal("the init's untaken branch left no uncovered block; the case is vacuous")
	}
	if got := factStrings(t, e, "turn_uncovered", 1); len(got) != 1 || got[0] != "sub/calc.go" {
		t.Fatalf("turn_uncovered = %v, want [sub/calc.go]", got)
	}
	if got := queryCount(t, e, "turn_unwitnessed"); got != 0 {
		t.Fatalf("turn_unwitnessed = %d, want 0: an init owes no witness", got)
	}
	if result.TurnOutcome != types.MangleAtom("/unverified") {
		t.Fatalf("TurnOutcome = %q, want /unverified", result.TurnOutcome)
	}
	if !containsString(result.MissingEvidence, "/changed_code_unexecuted") {
		t.Fatalf("MissingEvidence = %v, want it to name /changed_code_unexecuted", result.MissingEvidence)
	}
	if containsString(result.MissingEvidence, "/change_unwitnessed") {
		t.Fatalf("MissingEvidence = %v, must not name /change_unwitnessed for an init", result.MissingEvidence)
	}
}

// A write that changed no element owes no witness: docs-only and test-only
// turns verify on their gates with no witness rows at all.
func TestWitnessStep6_NonElementWriteOwesNoWitness(t *testing.T) {
	for _, written := range []string{"Docs/guide.md", "sub/calc_test.go"} {
		e := newObligationExec(t)
		result := writeTurnResult()
		result.WrittenPaths = []string{written}
		e.assertTurnEvidence(testTurn, "/fix", result)
		e.captureTurnOutcome(testTurn, result, nil)
		if got := queryCount(t, e, "witness_owed"); got != 0 {
			t.Errorf("%s: witness_owed = %d, want 0", written, got)
		}
		if got := queryCount(t, e, "turn_unwitnessed"); got != 0 {
			t.Errorf("%s: turn_unwitnessed = %d, want 0", written, got)
		}
		if result.TurnOutcome != types.MangleAtom("/done") {
			t.Errorf("%s: TurnOutcome = %q (missing %v), want /done", written, result.TurnOutcome, result.MissingEvidence)
		}
		e.cleanupTurnFacts()
	}
}

// The verdict's witness is the harness's alone: a model that could assert
// witness_met would suppress turn_unwitnessed through its negation.
func TestWitnessStep6_ModelCannotAssertWitness(t *testing.T) {
	permissive := core.MangleUpdatePolicy{AllowedPrefixes: []string{""}}
	for _, update := range []string{
		`witness_executed(/turn_test, "fn:calc.Sub").`,
		`witness_met(/turn_test, "fn:calc.Sub").`,
		`turn_unwitnessed(/turn_test, "fn:calc.Sub").`,
		`turn_has_unwitnessed(/turn_test).`,
	} {
		if kept, _ := core.FilterMangleUpdates(nil, []string{update}, permissive); len(kept) != 0 {
			t.Errorf("the model can assert %s; the verdict's evidence must be the harness's alone", update)
		}
		if kept, _ := core.FilterMangleUpdates(nil, []string{update}, core.ModelObservationPolicy()); len(kept) != 0 {
			t.Errorf("the model can assert %s through the observation policy", update)
		}
	}
}
