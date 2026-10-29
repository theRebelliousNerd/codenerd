package session

import (
	"testing"

	"codenerd/internal/types"
)

// S4 made turn_verified read test_state(/passing) as the host's own record of
// what the test runner returned. S1 made recordBuildState the producer that
// writes it, tracked it in perTurnBuildStateFacts and retracted it in
// cleanupTurnFacts — "a red build is evidence about THIS turn".
//
// test_state/1 is not that producer's private predicate. Three other producers
// write it and none of them is per-turn:
//
//   - internal/core/virtual_store_actions.go:460 — handleRunTests, the
//     `run_tests` action the MODEL invokes as a tool, asserts
//     test_state(/passing) or test_state(/failing) from the command it ran.
//     The facts reach the kernel through virtual_store_routing.go:176
//     injectFacts and are never retracted.
//   - internal/core/virtual_store_workflows.go:70 — test_state(/log_read).
//   - internal/core/tdd_loop.go:254 and :958 — the TDD state machine's own
//     vocabulary (/red, /green, /running, /log_read, /cause_found,
//     /patch_applied), one fact per transition.
//
// So the predicate turn_verified treats as "the gate this turn ran" is a
// session-global with a second, incompatible vocabulary in it. The scenario
// below is the one that matters, and it needs nothing exotic: the model runs
// the tests with a tool early in the session, they pass, and the fact stays.
// Several turns later it edits a file; the closing TestCheck is SKIPPED —
// build_verify.go:406 returns exactly that for a package whose tests are all
// tag-gated, and recordBuildState deliberately asserts nothing for a skipped
// gate — and the turn verifies anyway, on a test run that happened before the
// edit existed.
//
// That is stale evidence surviving a source change as current truth, which is
// the thing the whole seam is against.
func TestTurnVerified_DoesNotReuseAnEarlierTurnsTestState(t *testing.T) {
	e := newObligationExec(t)

	// Turn N. The model calls the run_tests tool. handleRunTests injects this
	// exact fact — a Go string "/passing", not a types.MangleAtom, which is
	// how the action builds it.
	runTestsFact := types.Fact{Predicate: "test_state", Args: []any{"/passing"}}
	if err := e.kernel.Assert(runTestsFact); err != nil {
		t.Fatalf("assert the run_tests action's test_state: %v", err)
	}
	if got := queryCount(t, e, "test_state"); got != 1 {
		t.Fatalf("the run_tests action's test_state must land in the kernel, got %d facts "+
			"(if this is 0 the finding is different: the action's facts are silently dropped)", got)
	}
	// Turn N ends. Nothing retracts it: it was not asserted by
	// recordBuildState, so it is not in perTurnBuildStateFacts.
	e.cleanupTurnFacts()
	if got := queryCount(t, e, "test_state"); got != 1 {
		t.Fatalf("precondition: the foreign test_state survives the turn cleanup, got %d facts", got)
	}

	// Turn N+k. A write turn. The build gate ran and passed; the test gate was
	// SKIPPED, so this turn measured nothing at all about the tests.
	result := writeTurnResult()
	result.TestCheck = TestVerification{Outcome: VerifySkipped, Reason: "only tag-gated packages; compile-checked with go vet"}

	e.assertTurnEvidence(testTurn, "/create", result)
	e.captureTurnOutcome(testTurn, result, nil)

	if got := queryCount(t, e, "turn_verified"); got != 0 {
		t.Errorf("turn_verified = %d: this turn ran no test gate, so the only test_state in the "+
			"kernel is an earlier turn's. Verification must rest on evidence measured AFTER the edit.", got)
	}
	if result.TurnOutcome == types.MangleAtom("/done") {
		t.Errorf("TurnOutcome = /done on a turn whose tests were never run: the verdict was bought " +
			"with a test_state left behind by the run_tests tool call in an earlier turn")
	}
}

// The other direction. The review that found the hole above proposed that a
// red test_state from any producer should block verification, symmetric with
// the guard turn_executed put on build_state. The decision taken is narrower
// and follows the same principle as the test above: THIS turn's gate is the
// evidence, and it is the fresher measurement. A red left in the session by
// the TDD loop's state machine or by an earlier run_tests call does not
// outrank a gate that ran green after the edit; what does block is this
// turn's own red. Both are pinned here so the arm cannot drift back to
// reading the global in either direction.
func TestTurnVerified_ReadsThisTurnsGateNotTheSessionsRed(t *testing.T) {
	t.Run("foreignRedDoesNotBlockAGreenGate", func(t *testing.T) {
		e := newObligationExec(t)

		// A failing test_state from somewhere else in the session — the TDD
		// loop sitting in its failing state, or a run_tests call that went red
		// before this turn's edit existed.
		if err := e.kernel.Assert(types.Fact{Predicate: "test_state", Args: []any{"/failing"}}); err != nil {
			t.Fatalf("assert test_state(/failing): %v", err)
		}

		// This turn's own gates: build green, tests green, measured after the edit.
		result := writeTurnResult()
		e.assertTurnEvidence(testTurn, "/create", result)

		if got := queryCount(t, e, "test_state"); got != 2 {
			t.Fatalf("precondition: both test_state values must coexist, got %d facts", got)
		}
		if got := queryCount(t, e, "turn_verified"); got != 1 {
			t.Errorf("turn_verified = %d: this turn's own gate ran green after the edit; a red left "+
				"behind by another producer is older evidence and must not outrank it", got)
		}
		e.captureTurnOutcome(testTurn, result, nil)
		if result.TurnOutcome != types.MangleAtom("/done") {
			t.Errorf("TurnOutcome = %v, want /done: the verdict read the session's stale red instead "+
				"of this turn's gate", result.TurnOutcome)
		}
	})

	t.Run("thisTurnsOwnRedBlocks", func(t *testing.T) {
		e := newObligationExec(t)

		// This turn's own gates: build green, tests RED.
		result := writeTurnResult()
		result.TestCheck = TestVerification{Ran: true, OK: false, Outcome: VerifyFailed}
		e.assertTurnEvidence(testTurn, "/create", result)

		if got := queryCount(t, e, "turn_verified"); got != 0 {
			t.Errorf("turn_verified = %d with this turn's own test gate red", got)
		}
		if got := queryCount(t, e, "turn_missing_evidence"); got == 0 {
			t.Errorf("a turn whose own tests are red must name what is missing")
		}
		e.captureTurnOutcome(testTurn, result, nil)
		if result.TurnOutcome == types.MangleAtom("/done") {
			t.Errorf("TurnOutcome = /done on a turn whose own test gate failed")
		}
	})
}
