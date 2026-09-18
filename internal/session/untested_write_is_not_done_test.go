package session

import (
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// A turn that wrote production Go with no test beside it is not /done, and the
// verdict says why. Until 2026-09-18 the executor computed result.UntestedPaths
// on every write turn (build_verify.go) and the list reached the log and the
// subagent return only: the corpus never saw it, so a green build plus a green
// run of the tests that already existed derived turn_verified. The harness
// forces the test; it does not ask for it.
func TestTurnThatWroteUntestedProductionCodeIsNotDone(t *testing.T) {
	t.Run("untestedWriteIsUnverifiedAndNamed", func(t *testing.T) {
		e := newObligationExec(t)

		result := writeTurnResult() // build green, tests green, measured after the edit
		result.UntestedPaths = []string{"internal/widget/widget.go"}
		e.assertTurnEvidence("/create", result)

		if got := queryCount(t, e, "turn_untested"); got != 1 {
			t.Fatalf("turn_untested = %d, want 1: the executor must hand the corpus the coverage debt it measured", got)
		}
		if got := queryCount(t, e, "turn_verified"); got != 0 {
			t.Errorf("turn_verified = %d on a turn that wrote production code with no test beside it", got)
		}
		e.captureTurnOutcome(result, nil)
		if result.TurnOutcome == types.MangleAtom("/done") {
			t.Errorf("TurnOutcome = /done for a turn that wrote untested production code")
		}
		named := false
		for _, m := range result.MissingEvidence {
			if m == "/tests_not_written" {
				named = true
			}
		}
		if !named {
			t.Errorf("MissingEvidence = %v, want it to name /tests_not_written", result.MissingEvidence)
		}
	})

	t.Run("sameTurnWithItsTestsIsDone", func(t *testing.T) {
		e := newObligationExec(t)

		result := writeTurnResult()
		e.assertTurnEvidence("/create", result)

		if got := queryCount(t, e, "turn_verified"); got != 1 {
			t.Errorf("turn_verified = %d: with no coverage debt the green gates must still verify the turn", got)
		}
		e.captureTurnOutcome(result, nil)
		if result.TurnOutcome != types.MangleAtom("/done") {
			t.Errorf("TurnOutcome = %v, want /done", result.TurnOutcome)
		}
	})

	t.Run("theModelCannotAssertOrClearTheDebt", func(t *testing.T) {
		// Probe the filter itself, with a policy that would otherwise let anything in.
		permissive := core.MangleUpdatePolicy{AllowedPrefixes: []string{""}}
		for _, update := range []string{`turn_untested(/create, "internal/widget/widget.go").`, "turn_has_untested(/create)."} {
			if kept, _ := core.FilterMangleUpdates(nil, []string{update}, permissive); len(kept) != 0 {
				t.Errorf("the model can assert %s; the coverage verdict must be the harness's alone", update)
			}
		}
	})
}
