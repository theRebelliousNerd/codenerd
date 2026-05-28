package core

import (
	"strings"
	"testing"
)

func TestRuleCourt_RatifySafe(t *testing.T) {
	k := setupMockKernel(t)
	// Use unique predicate names to avoid schema conflicts
	k.AppendPolicy(`
	Decl test_perm(Name).
	Decl test_trigger(Name).
	test_perm("base_action") :- test_trigger("now").
	`)
	k.Assert(Fact{Predicate: "test_trigger", Args: []any{"now"}})
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	res, _ := k.Query("test_perm")
	if len(res) == 0 {
		t.Skipf("Setup failed: no derived test_perm (schema conflict?)")
	}

	court := NewRuleCourt(k)
	newRule := `test_perm("new_action") :- test_trigger("later").`

	if err := court.RatifyRule(newRule); err != nil {
		t.Errorf("RatifyRule failed for safe rule: %v", err)
	}
}

func TestRuleCourt_RatifyDeadlock(t *testing.T) {
	k := setupMockKernel(t)

	// Use unique predicates and EDB facts
	k.AppendPolicy(`
	Decl test_allowed(Name).
	`)
	k.Assert(Fact{Predicate: "test_allowed", Args: []any{"action1"}})
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	res, _ := k.Query("test_allowed")
	if len(res) == 0 {
		t.Skipf("Setup failed: no test_allowed facts")
	}

	court := NewRuleCourt(k)

	// Test empty rule rejection
	emptyRule := ""
	err := court.RatifyRule(emptyRule)
	if err == nil {
		t.Error("Expected error for empty rule, got nil")
	}

	// Test syntax error rule
	badRule := `test_allowed(123 :- .`
	err = court.RatifyRule(badRule)
	if err == nil {
		t.Error("Expected error for syntactically invalid rule, got nil")
	} else {
		t.Logf("Got expected error for bad syntax: %v", err)
	}
}

func TestRuleCourt_RatifyAskUserVeto(t *testing.T) {
	k := setupMockKernel(t)
	k.AppendPolicy(`Decl test_action(Name).`)
	k.Assert(Fact{Predicate: "test_action", Args: []any{"action1"}})
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	court := NewRuleCourt(k)

	// Rule that mentions ask_user (should be vetoed by RatifyRule safety check)
	newRule := `blocked("ask_user").`

	err := court.RatifyRule(newRule)
	if err == nil {
		t.Error("Expected VETO for rule mentioning ask_user")
	} else if !strings.Contains(err.Error(), "ask_user") {
		t.Logf("Got error: %v", err)
	} else {
		t.Logf("Got expected ask_user veto: %v", err)
	}
}

// TestRuleCourt_NilKernel verifies that RatifyRule rejects a nil kernel
// argument with a clean error instead of panicking with a nil dereference.
func TestRuleCourt_NilKernel(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RatifyRule(nil, ...) panicked: %v", r)
		}
	}()
	err := RatifyRule(nil, `allowed("action").`)
	if err == nil {
		t.Fatal("Expected error for nil kernel, got nil")
	}
	if !strings.Contains(err.Error(), "no kernel available") {
		t.Errorf("Expected 'no kernel available' error, got: %v", err)
	}
}

// TestRuleCourt_WhitespaceOnly verifies that rules consisting entirely of
// exotic whitespace (zero-width space, non-breaking space, tabs, newlines)
// are either trimmed to empty (and rejected) or rejected by the compiler.
// Either outcome is acceptable; the contract is that no such rule is
// silently ratified.
func TestRuleCourt_WhitespaceOnly(t *testing.T) {
	k := setupMockKernel(t)
	court := NewRuleCourt(k)

	cases := []string{
		" \t \n \r ", // standard whitespace — trimmed to empty
		"​​",         // zero-width spaces only
		"  ",         // non-breaking spaces only
		"​   \t",     // mixed exotic + standard
	}

	for _, rule := range cases {
		err := court.RatifyRule(rule)
		if err == nil {
			t.Errorf("Expected rejection for whitespace-only rule %q, got nil", rule)
		}
	}
}

// TestRuleCourt_NullBytes verifies that rules containing null bytes or
// otherwise UTF-8-malformed sequences are rejected as syntax errors by the
// sandbox compiler rather than crashing the Mangle parser or the Go
// runtime.
func TestRuleCourt_NullBytes(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RatifyRule with null bytes panicked: %v", r)
		}
	}()
	k := setupMockKernel(t)
	court := NewRuleCourt(k)

	cases := []string{
		"allowed(\"\x00\").",     // null byte inside a string literal
		"allow\x00ed(\"x\").",    // null byte in identifier
		"allowed(\"\xff\xfe\").", // invalid UTF-8 sequence
	}

	for _, rule := range cases {
		err := court.RatifyRule(rule)
		if err == nil {
			t.Errorf("Expected rejection for malformed rule %q, got nil", rule)
		}
	}
}

// TODO: TEST_GAP: [Null/Undefined/Empty] Verify RatifyRule handles a nil kernel parameter gracefully without panicking.
// TODO: TEST_GAP: [Null/Undefined/Empty] Verify RatifyRule rejects rules that consist entirely of zero-width spaces or non-breaking spaces, not just standard whitespace.
// TODO: TEST_GAP: [Type Coercion] Verify RatifyRule handles rules with null bytes (\x00) or invalid UTF-8 sequences as syntax errors rather than crashing the parser.
// TODO: TEST_GAP: [Type Coercion] Verify RatifyRule rejects rules that reference undeclared schemas or types, propagating the error from sandbox.Evaluate().
// TODO: TEST_GAP: [User Request Extremes] Verify RatifyRule correctly handles an enormous rule string (e.g., 10MB) without OOMing or hanging indefinitely.
// TODO: TEST_GAP: [User Request Extremes] Verify sandbox.Evaluate() enforces a strict context timeout to prevent infinite derivation loops in recursive logic.
// TODO: TEST_GAP: [State Conflicts] Verify RatifyRule correctly identifies and vetoes rules even under heavy concurrency during GetFactsSnapshot.
// TODO: TEST_GAP: [State Conflicts] Verify the "ask_user" safety hatch isn't triggered as a false positive for safe logic (e.g., a rule containing the substring "ask_user_id").
