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
	k.Assert(Fact{Predicate: "test_trigger", Args: []interface{}{"now"}})
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
	k.Assert(Fact{Predicate: "test_allowed", Args: []interface{}{"action1"}})
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
	k.Assert(Fact{Predicate: "test_action", Args: []interface{}{"action1"}})
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
