package chat

import (
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/perception"
)

// delegation_roundtrip_test.go exercises the Go assert->query->retract cycle of
// the Steps 4-5 decision helpers (Model.shouldDelegate / Model.detectMultiStepTask)
// against a REAL kernel loaded with the embedded policy corpus. It proves three
// things the kernel-rule tests cannot:
//
//  1. the query returns the right boolean when facts are asserted (happy path);
//  2. the Go fail-safe fallback fires when the kernel has no candidate fact;
//  3. (THE LOAD-BEARING ONE) the IN-METHOD retract clears the prior turn's facts
//     so a stale signal cannot contaminate the next turn's decision.
//
// For (3) the methods are called TWICE with NO manual retract between turns.
// Each method self-cleans before asserting (detectMultiStepTask: Retract;
// shouldDelegate: RetractFact) — that in-method retract is the production
// defense (process.go's per-turn Retract is belt-and-suspenders). Turn-2 inputs
// are chosen so that if the prior turn's facts lingered, the boolean would flip
// the WRONG way and the test would catch it.

func newRoundtripModel(t *testing.T) Model {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel failed: %v", err)
	}
	m := NewTestModel()
	m.kernel = k
	return m
}

// -----------------------------------------------------------------------------
// shouldDelegate: happy path + fallback + cross-turn contamination
// -----------------------------------------------------------------------------

func TestRoundtrip_ShouldDelegate_HappyPathAndFallback(t *testing.T) {
	m := newRoundtripModel(t)

	// Happy path: real shard, confidence above the 0.5 (==50) gate -> delegate.
	if !m.shouldDelegate("coder", 0.8) {
		t.Error("shouldDelegate(coder, 0.8) = false, want true (above threshold)")
	}

	// Below threshold -> do not delegate.
	if m.shouldDelegate("coder", 0.3) {
		t.Error("shouldDelegate(coder, 0.3) = true, want false (below threshold)")
	}

	// Empty shard -> do not delegate (legacy gate agrees; /none in Mangle).
	if m.shouldDelegate("", 0.9) {
		t.Error("shouldDelegate(\"\", 0.9) = true, want false (no shard)")
	}
}

func TestRoundtrip_ShouldDelegate_NilKernelFallback(t *testing.T) {
	// Kernel nil -> must fall back to the legacy Go boolean, never panic.
	m := NewTestModel() // kernel is nil
	if m.kernel != nil {
		t.Fatal("expected nil kernel from NewTestModel")
	}
	if !m.shouldDelegate("coder", 0.8) {
		t.Error("nil-kernel fallback: shouldDelegate(coder, 0.8) = false, want true (legacy gate)")
	}
	if m.shouldDelegate("coder", 0.3) {
		t.Error("nil-kernel fallback: shouldDelegate(coder, 0.3) = true, want false (legacy gate)")
	}
}

// TestRoundtrip_ShouldDelegate_NoContaminationAcrossTurns is the stale-fact
// guard. Turn 1 asserts a HIGH-confidence candidate for /coder (delegates).
// Turn 2 uses the SAME shard but BELOW threshold. If the turn-1
// delegation_candidate(/current_intent, /coder, 80) lingered, should_delegate
// would still derive on the stale 80 and turn 2 would wrongly delegate. We call
// shouldDelegate twice with NO retract in between, so only the in-method
// RetractFact protects turn 2.
func TestRoundtrip_ShouldDelegate_NoContaminationAcrossTurns(t *testing.T) {
	m := newRoundtripModel(t)

	if !m.shouldDelegate("coder", 0.8) {
		t.Fatal("turn 1: shouldDelegate(coder, 0.8) = false, want true")
	}

	// Turn 2: same shard, below the gate. Must be false. A lingering turn-1
	// fact (conf 80) would make this wrongly true.
	if m.shouldDelegate("coder", 0.3) {
		t.Error("turn 2 CONTAMINATED: shouldDelegate(coder, 0.3) = true, want false " +
			"(stale turn-1 delegation_candidate not cleared by in-method RetractFact)")
	}
}

// -----------------------------------------------------------------------------
// detectMultiStepTask: happy path + fallback + cross-turn contamination
// -----------------------------------------------------------------------------

func TestRoundtrip_DetectMultiStep_HappyPath(t *testing.T) {
	m := newRoundtripModel(t)
	intent := perception.Intent{Verb: "/fix"}

	// Input with an explicit multi-step keyword ("and then") -> multi-step.
	if !m.detectMultiStepTask("fix the bug and then run the tests", intent) {
		t.Error("detectMultiStepTask multi-step input = false, want true")
	}

	// Single-step input with no signals -> not multi-step.
	if m.detectMultiStepTask("fix the typo", intent) {
		t.Error("detectMultiStepTask single-step input = true, want false")
	}
}

func TestRoundtrip_DetectMultiStep_NilKernelFallback(t *testing.T) {
	m := NewTestModel() // kernel nil
	intent := perception.Intent{Verb: "/fix"}
	if !m.detectMultiStepTask("fix the bug and then run the tests", intent) {
		t.Error("nil-kernel fallback: multi-step input = false, want true (legacy gate)")
	}
	if m.detectMultiStepTask("fix the typo", intent) {
		t.Error("nil-kernel fallback: single-step input = true, want false (legacy gate)")
	}
}

// TestRoundtrip_DetectMultiStep_NoContaminationAcrossTurns is the stale-signal
// guard the user flagged as the highest-risk bug. Turn 1 is multi-step (asserts
// multi_step_signal facts). Turn 2 is a clean single-step request with ZERO
// signals. If turn-1's multi_step_signal lingered, is_multi_step would still
// fire and turn 2 would be wrongly classified multi-step. Called twice with NO
// retract between turns: only the in-method Retract("multi_step_signal")
// protects turn 2.
func TestRoundtrip_DetectMultiStep_NoContaminationAcrossTurns(t *testing.T) {
	m := newRoundtripModel(t)
	intent := perception.Intent{Verb: "/fix"}

	if !m.detectMultiStepTask("first fix the bug, then write a test, finally run it", intent) {
		t.Fatal("turn 1: multi-step input = false, want true")
	}

	// Turn 2: a single, signal-free request. Must be false. A lingering turn-1
	// signal would make is_multi_step fire and wrongly return true.
	if m.detectMultiStepTask("rename the variable", intent) {
		t.Error("turn 2 CONTAMINATED: single-step input = true, want false " +
			"(stale turn-1 multi_step_signal not cleared by in-method Retract)")
	}
}
