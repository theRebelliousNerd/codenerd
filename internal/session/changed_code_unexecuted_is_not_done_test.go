package session

import (
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// The harness measured both of these on every Go write turn and acted on
// neither. Observed 2026-09-19 (ladder run R1-2, nerd fix on cmd/nerd): the
// turn wrote 27 blocks of Go that no test executes -- the session log said so
// -- and left an unreachable statement `go vet` reports; the coverage list went
// to the log and the vet finding to an advisory critic that timed out, and the
// turn was recorded /done beside "Verified by evidence". Both are evidence
// about the change the turn made, so both reach the corpus, and the verdict
// names whichever is missing.
func TestTurnWhoseChangedCodeIsUnexecutedOrVetRedIsNotDone(t *testing.T) {
	missing := func(result *ExecutionResult, atom string) bool {
		for _, m := range result.MissingEvidence {
			if m == atom {
				return true
			}
		}
		return false
	}

	t.Run("changedCodeNoTestExecutesIsUnverifiedAndNamed", func(t *testing.T) {
		e := newObligationExec(t)

		result := writeTurnResult() // build green, tests green, measured after the edit
		result.UncoveredBlocks = []UncoveredBlock{{File: "codenerd/pkg/foo.go", StartLine: 10, EndLine: 12, NumStmts: 2}}
		e.assertTurnEvidence(testTurn, "/fix", result)

		if got := queryCount(t, e, "turn_uncovered"); got != 1 {
			t.Fatalf("turn_uncovered = %d, want 1: the executor must hand the corpus the unexecuted code it measured", got)
		}
		if got := queryCount(t, e, "turn_verified"); got != 0 {
			t.Errorf("turn_verified = %d on a turn whose changed code no test executes", got)
		}
		e.captureTurnOutcome(testTurn, result, nil)
		if result.TurnOutcome == types.MangleAtom("/done") {
			t.Errorf("TurnOutcome = /done for a turn whose changed code no test executes")
		}
		if !missing(result, "/changed_code_unexecuted") {
			t.Errorf("MissingEvidence = %v, want it to name /changed_code_unexecuted", result.MissingEvidence)
		}
	})

	t.Run("vetRedIsUnverifiedAndNamed", func(t *testing.T) {
		e := newObligationExec(t)

		result := writeTurnResult()
		result.VetCheck = BuildVerification{Ran: true, OK: false, Outcome: VerifyFailed, Output: "pkg/foo.go:286:2: unreachable code"}
		e.assertTurnEvidence(testTurn, "/fix", result)

		if got := queryCount(t, e, "turn_verified"); got != 0 {
			t.Errorf("turn_verified = %d on a turn go vet rejects", got)
		}
		e.captureTurnOutcome(testTurn, result, nil)
		if result.TurnOutcome == types.MangleAtom("/done") {
			t.Errorf("TurnOutcome = /done for a turn go vet rejects")
		}
		if !missing(result, "/vet_not_clean") {
			t.Errorf("MissingEvidence = %v, want it to name /vet_not_clean", result.MissingEvidence)
		}
	})

	t.Run("executedAndVetCleanIsDone", func(t *testing.T) {
		e := newObligationExec(t)

		result := writeTurnResult()
		result.VetCheck = BuildVerification{Ran: true, OK: true, Outcome: VerifyPassed}
		e.assertTurnEvidence(testTurn, "/fix", result)

		if got := queryCount(t, e, "turn_verified"); got != 1 {
			t.Errorf("turn_verified = %d: with every changed block executed and vet clean the green gates must verify the turn", got)
		}
		e.captureTurnOutcome(testTurn, result, nil)
		if result.TurnOutcome != types.MangleAtom("/done") {
			t.Errorf("TurnOutcome = %v, want /done", result.TurnOutcome)
		}
	})

	t.Run("theModelCannotAssertOrClearEither", func(t *testing.T) {
		permissive := core.MangleUpdatePolicy{AllowedPrefixes: []string{""}}
		for _, update := range []string{
			`turn_uncovered(/fix, "pkg/foo.go").`,
			"turn_has_uncovered(/fix).",
			"turn_vet_green(/fix).",
			"turn_vet_red(/fix).",
		} {
			if kept, _ := core.FilterMangleUpdates(nil, []string{update}, permissive); len(kept) != 0 {
				t.Errorf("the model can assert %s; the verdict's evidence must be the harness's alone", update)
			}
		}
	})
}
