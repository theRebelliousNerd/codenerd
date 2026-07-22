package core

import (
	"strings"
	"testing"
	"time"
)

// REMEDIATED: TEST_GAP: [Type Coercion] Verify RatifyRule rejects rules that reference undeclared schemas or types, propagating the error from sandbox.Evaluate().
func TestRuleCourt_UndeclaredSchemas(t *testing.T) {
	k := setupMockKernel(t)
	court := NewRuleCourt(k)

	// 'undeclared_pred' does not exist in schema.
	err := court.RatifyRule(`undeclared_pred(X) :- other(X).`)
	if err == nil {
		t.Fatal("Expected error for undeclared schema rule, got nil")
	}
	if !strings.Contains(err.Error(), "rule rejected by sandbox compiler") {
		t.Errorf("Expected sandbox compiler rejection error, got: %v", err)
	}
}

// REMEDIATED: TEST_GAP: [Type Coercion] String vs Atom mismatch (Atom/String Dissonance)
//
// The original version used `Decl expects_atom(Name.Type<Atom>).`, which is not
// valid Mangle — the test "passed" on a parse error, never exercising type
// dissonance. With a valid Decl, the kernel analyzes with NoBoundsChecking
// (kernel_eval.go rebuildProgram), so a String in an Atom-bounded slot is
// ACCEPTED and simply never unifies with /name joins (the classic silent-join
// failure). This test pins that current behavior as a canary: if the kernel is
// ever upgraded to analysis.ErrorForBoundsMismatch (requires retrofitting
// `bound` blocks across the embedded corpus — zero-arity Decls included), this
// test will fail and should then assert the rejection instead.
func TestRuleCourt_AtomStringDissonance(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy(`Decl expects_atom(Name) bound [/name].`)

	court := NewRuleCourt(k)

	err := court.RatifyRule(`expects_atom("bad").`)
	if err != nil {
		t.Fatalf("NoBoundsChecking kernel unexpectedly rejected string-in-atom-slot: %v", err)
	}
}

// REMEDIATED: TEST_GAP: [User Request Extremes] Verify RatifyRule correctly handles an enormous rule string (e.g., 10MB) without OOMing or hanging indefinitely.
func TestRuleCourt_EnormousRuleSize(t *testing.T) {
	k := setupMockKernel(t)
	court := NewRuleCourt(k)

	// Generate a 15MB rule string (5 million 'a's -> length 15M bytes)
	// We want to verify it rejects it gracefully, ideally via length check
	hugeRule := `test_allowed("` + strings.Repeat("a", 15*1024*1024) + `").`

	err := court.RatifyRule(hugeRule)
	if err == nil {
		t.Fatal("Expected error for enormous rule, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum allowed length") {
		t.Errorf("Expected length rejection error, got: %v", err)
	}
}

// REMEDIATED: TEST_GAP: [User Request Extremes] Verify sandbox.Evaluate() enforces a strict context timeout to prevent infinite derivation loops in recursive logic.
func TestRuleCourt_RunawayRecursionTimeout(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy(`
		Decl counter(Num) bound [/number].
		counter(0).
	`)

	court := NewRuleCourt(k)

	// Infinite recursion logic: counter(N+1) :- counter(N).
	recursiveRule := `counter(fn:plus(N, 1)) :- counter(N).`

	// Temporarily shorten the ratifyEvalTimeout to ensure the test finishes quickly
	originalTimeout := ratifyEvalTimeout
	ratifyEvalTimeout = 100 * time.Millisecond
	defer func() { ratifyEvalTimeout = originalTimeout }()

	err := court.RatifyRule(recursiveRule)
	if err == nil {
		t.Fatal("Expected timeout error for infinite recursion, got nil")
	}
	if !strings.Contains(err.Error(), "VETO: sandbox evaluation timed out") {
		t.Errorf("Expected timeout veto, got: %v", err)
	}
}

// REMEDIATED: TEST_GAP: [State Conflicts] Verify the "ask_user" safety hatch isn't triggered as a false positive for safe logic (e.g., a rule containing the substring "ask_user_id").
func TestRuleCourt_AskUserFalsePositive(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy(`Decl user_data(ID).`)

	court := NewRuleCourt(k)

	// This is a perfectly safe rule, but contains 'ask_user' as a substring inside 'ask_user_id'.
	// It should NOT be vetoed.
	safeRule := `user_data("ask_user_id").`

	err := court.RatifyRule(safeRule)
	if err != nil {
		t.Fatalf("Safe rule falsely vetoed due to substring match: %v", err)
	}
}
