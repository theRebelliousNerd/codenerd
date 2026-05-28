package core

import (
	"strings"
	"testing"
)

func TestRuleCourt_CoverageExtra(t *testing.T) {
	// 1. Nil kernel parameter
	err := RatifyRule(nil, `allowed("action").`)
	if err == nil || err.Error() != "no kernel available for ratification" {
		t.Errorf("expected 'no kernel available' error, got: %v", err)
	}

	// 2. Empty or whitespace rule
	k := setupMockKernel(t)
	court := NewRuleCourt(k)
	err = court.RatifyRule("   \n   \t   ")
	if err == nil || !strings.Contains(err.Error(), "empty rule") {
		t.Errorf("expected empty rule error, got: %v", err)
	}

	// 3. Null bytes / invalid UTF-8
	err = court.RatifyRule("allowed(\x00).")
	if err == nil {
		t.Error("expected syntax error for null byte, got nil")
	}

	// 4. Undeclared schemas / types error propagation
	// Propose a rule using undeclared predicate/types
	err = court.RatifyRule(`undeclared_pred(123) :- another_undeclared(X).`)
	if err == nil || !strings.Contains(err.Error(), "rule rejected by sandbox compiler") {
		t.Errorf("expected compiler error for undeclared predicates, got: %v", err)
	}

	// 5. Emergency hatch "ask_user" check
	err = court.RatifyRule(`my_rule("ask_user_id").`)
	if err == nil || !strings.Contains(err.Error(), "cannot forbid emergency hatch 'ask_user'") {
		t.Errorf("expected veto for 'ask_user' substring, got: %v", err)
	}

	// 6. VETO: rule causes total system deadlock (no permitted actions)
	// Base kernel has permitted action
	kDeadlock := setupMockKernel(t)
	// Assert a pending action that should be permitted
	kDeadlock.Assert(Fact{
		Predicate: "pending_action",
		Args:      []any{"action1", "/read_file", "target", "payload", float64(123)},
	})
	if err := kDeadlock.Evaluate(); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Confirm base has permitted action
	basePerm, err := kDeadlock.Query("permitted")
	if err != nil || len(basePerm) == 0 {
		t.Fatalf("setup failed, expected permitted facts, got: %v (err: %v)", basePerm, err)
	}

	// Propose a rule that blocks all actions, causing total deadlock (no permitted actions)
	deadlockRule := `dangerous_content(Action, Payload) :- pending_action(_, Action, _, Payload, _).`
	err = RatifyRule(kDeadlock, deadlockRule)
	if err == nil || !strings.Contains(err.Error(), "rule causes total system deadlock") {
		t.Errorf("expected veto for total system deadlock, got: %v", err)
	}
}
