package session

import (
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/types"
)

// The /test_retention gate is derived (coder_safety.mg): the round measures
// the removed tests and reads the verdict instead of failing from the
// listing. Pinned from a real removal on a throwaway module.
func TestRemovedTestsGate_DerivedFromRealRemoval(t *testing.T) {
	const kept = "package main\n\nimport \"testing\"\n\nfunc TestKept(t *testing.T) {}\n"
	const gone = "\nfunc TestGone(t *testing.T) {}\n"
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "x_test.go"), []byte(kept), 0o600); err != nil {
		t.Fatal(err)
	}
	e := NewExecutor(realKernel(t), &testExecutiveStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	e.config.WorkspaceRoot = ws
	result := &ExecutionResult{
		SuccessfulWriteTools: 1,
		WrittenPaths:         []string{"x_test.go"},
		PreWriteContents:     map[string]PreImage{"x_test.go": existed(kept + gone)},
	}
	turn := result.turnAtom()

	removed := removedTestFunctions(ws, result.WrittenPaths, result.PreWriteContents)
	if len(removed) != 1 || removed[0] != "x_test.go:TestGone" {
		t.Fatalf("removed = %v, want [x_test.go:TestGone]", removed)
	}
	if !e.removedTestsGateRed(turn, removed) {
		t.Fatal("a turn that removed a test must read red from the derived gate")
	}
	if got := derivedVerify(t, e, turn, "/test_retention"); got != VerifyFailed {
		t.Fatalf("derived /test_retention = %v, want failed", got)
	}
	if !gateFact(t, e, "turn_red_gate", turn, "/test_retention") {
		t.Fatal("turn_red_gate(Turn, /test_retention) did not derive")
	}
	if !gateFact(t, e, "turn_owes_gate", turn, "/test_retention") {
		t.Fatal("turn_owes_gate(Turn, /test_retention) did not derive once measured")
	}

	// The test is restored: the re-sync retracts the stale row and the gate
	// goes green. Without the retraction the first sync's row would stay red
	// after the restoration.
	if err := os.WriteFile(filepath.Join(ws, "x_test.go"), []byte(kept+gone), 0o600); err != nil {
		t.Fatal(err)
	}
	restored := removedTestFunctions(ws, result.WrittenPaths, result.PreWriteContents)
	if len(restored) != 0 {
		t.Fatalf("restored listing = %v, want none", restored)
	}
	if e.removedTestsGateRed(turn, restored) {
		t.Fatal("a turn that put the test back must read green from the derived gate")
	}
	if got := derivedVerify(t, e, turn, "/test_retention"); got != VerifyPassed {
		t.Fatalf("derived /test_retention = %v, want passed", got)
	}
}

// The gate is owed exactly when the measurement ran: a turn whose removals
// were never measured owes no retention verdict, so fixtures that predate
// the gate keep their verdicts.
func TestRemovedTestsGate_UnmeasuredTurnOwesNothing(t *testing.T) {
	e := NewExecutor(realKernel(t), &testExecutiveStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	turn := types.MangleAtom("/turn_retention_unmeasured")
	if gateFact(t, e, "turn_owes_gate", turn, "/test_retention") {
		t.Fatal("an unmeasured turn must not owe /test_retention")
	}
	if got := derivedVerify(t, e, turn, "/test_retention"); got != VerifySkipped {
		t.Fatalf("derived /test_retention = %v, want skipped (the rule did not fire)", got)
	}
}

// A red retention gate blocks the verdict and names /tests_removed; a green
// one lets an otherwise green turn verify. The closure is a green build, a
// green suite, and the retention measurement.
func TestRemovedTestsGate_BlocksTheVerdict(t *testing.T) {
	e := NewExecutor(realKernel(t), &testExecutiveStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	turn := types.MangleAtom("/turn_retention_verdict")
	closure := []types.Fact{
		{Predicate: "turn_evidence", Args: []any{turn, types.MangleAtom("/fix"), 1, 1, 1, types.MangleAtom("/false"), types.MangleAtom("/false")}},
		{Predicate: "turn_written", Args: []any{turn, types.MangleString("pkg/foo.go"), types.MangleString(".go")}},
		{Predicate: "turn_gate", Args: []any{turn, types.MangleAtom("/build"), types.MangleAtom("/passing")}},
		{Predicate: "turn_test_measured", Args: []any{turn, types.MangleAtom("/passing")}},
	}
	for _, fact := range closure {
		if !e.assertTurnFact(fact) {
			t.Fatalf("assert %s: failed", fact.Predicate)
		}
	}
	e.syncRemovedTestGateFacts(turn, []string{"x_test.go:TestGone"})
	if hasUnaryVerdict(t, e, "turn_verified", turn) {
		t.Fatal("a turn charged with a removed test must not verify")
	}
	if !gateFact(t, e, "turn_missing_evidence", turn, "/tests_removed") {
		t.Fatal("turn_missing_evidence(Turn, /tests_removed) did not derive")
	}
	e.syncRemovedTestGateFacts(turn, nil)
	if !hasUnaryVerdict(t, e, "turn_verified", turn) {
		t.Fatal("an otherwise green turn with its tests kept must verify")
	}
	if !hasUnaryVerdict(t, e, "turn_done", turn) {
		t.Fatal("an otherwise green turn with its tests kept must be done")
	}
}

// hasUnaryVerdict reports whether a unary turn predicate holds for this turn.
// gateFact only matches binary rows.
func hasUnaryVerdict(t *testing.T, e *Executor, pred string, turn types.MangleAtom) bool {
	t.Helper()
	facts, err := e.kernel.Query(pred)
	if err != nil {
		t.Fatalf("query %s: %v", pred, err)
	}
	for _, fact := range facts {
		if len(fact.Args) > 0 && types.ExtractString(fact.Args[0]) == string(turn) {
			return true
		}
	}
	return false
}
