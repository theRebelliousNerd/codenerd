package session

import (
	"slices"
	"testing"

	"codenerd/internal/types"
)

// External audit F1 (2026-09-19): CloneForTask hands every delegated task the
// same kernel, and the turn verdict's facts were keyed by verb -- so two /fix
// turns on it were one turn to the corpus -- and the end-of-turn cleanup of a
// turn that had asserted nothing swept whole predicate populations. These run
// the audit's regression on the shipped corpus: hold turn A between its
// assertion and its consumption, close turn B beside it, and check both.

// greenFix is a /fix turn that wrote, with both of its gates measured green.
func greenFix() *ExecutionResult {
	res := mutationResult()
	res.Intent.Verb = "/fix"
	res.BuildCheck = BuildVerification{Ran: true, OK: true, Outcome: VerifyPassed}
	res.TestCheck = TestVerification{Ran: true, OK: true, Outcome: VerifyPassed}
	return res
}

func TestTurnVerdict_AConcurrentTurnWithTheSameVerbIsNotItsEvidence(t *testing.T) {
	a := newObligationExec(t)
	b := a.CloneForTask()

	turnA := newTurnAtom()
	a.assertTurnEvidence(turnA, "/fix", greenFix())
	defer a.cleanupTurnFacts()

	// B wrote too, but its test gate never ran.
	resB := greenFix()
	resB.TestCheck = TestVerification{}
	if err := b.checkHollowSuccess(resB); err != nil {
		t.Fatalf("B is not hollow, only unverified: %v", err)
	}
	if resB.TurnOutcome != types.MangleAtom("/unverified") || !slices.Contains(resB.MissingEvidence, "/tests_not_green") {
		t.Fatalf("B = %s missing %v, want /unverified missing /tests_not_green: A's test gate is not B's", resB.TurnOutcome, resB.MissingEvidence)
	}

	// B's cleanup retracted B's facts -- including a green build gate equal to
	// A's in every argument but the turn -- and A's verdict is intact.
	if v := a.consumeTurnDoneSignal(turnA, "/fix"); !v.Done {
		t.Fatalf("A's verdict after B closed = %+v, want done: B's cleanup took A's evidence", v)
	}
}

func TestTurnVerdict_AnEarlyReturnTurnClearsNothingItDidNotAssert(t *testing.T) {
	a := newObligationExec(t)
	b := a.CloneForTask()
	world := assertWorldTestPairing(t, a, "pkg/foo_test.go", "pkg/foo.go")

	turnA := newTurnAtom()
	a.assertTurnEvidence(turnA, "/fix", greenFix())
	defer a.cleanupTurnFacts()

	// A turn with no verb asserts nothing and returns early; its cleanup still
	// runs.
	if err := b.checkHollowSuccess(&ExecutionResult{}); err != nil {
		t.Fatalf("an empty turn: %v", err)
	}

	pairs, err := a.kernel.Query("test_file_for")
	if err != nil || !slices.ContainsFunc(pairs, func(f types.Fact) bool { return f.String() == world.String() }) {
		t.Fatalf("the world's test_file_for pairing was swept by a turn that asserted nothing: %v (%v)", pairs, err)
	}
	if rows, err := a.turnRows("turn_evidence", turnA); err != nil || len(rows) != 1 {
		t.Fatalf("A's turn_evidence after B's empty cleanup = %v (%v), want it intact", rows, err)
	}
}
