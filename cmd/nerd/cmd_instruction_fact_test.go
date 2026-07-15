package main

import (
	"fmt"
	"testing"

	"codenerd/internal/types"
)

// TestNextActionFact_ThreeArgShape is the deterministic regression for
// F-ROUTE-1. VirtualStore.parseActionFact requires (ActionID, Type, Target);
// the CLI previously built a 2-arg {action, input} fact, so EVERY policy-
// derived non-delegate next_action (e.g. /explain → /analyze_code) failed with
// "invalid action fact: requires at least 3 arguments" instead of executing.
func TestNextActionFact_ThreeArgShape(t *testing.T) {
	fact := nextActionFact("/analyze_code", "explain what the OODA loop does")

	if fact.Predicate != "next_action" {
		t.Fatalf("predicate = %q, want next_action", fact.Predicate)
	}
	if len(fact.Args) != 3 {
		t.Fatalf("len(Args) = %d, want 3 (ActionID, Type, Target)", len(fact.Args))
	}
	if id := fmt.Sprintf("%v", fact.Args[0]); id == "" {
		t.Error("ActionID (Args[0]) must be non-empty")
	}
	if got := types.ExtractString(fact.Args[1]); got != "/analyze_code" {
		t.Errorf("Type (Args[1]) = %q, want /analyze_code", got)
	}
	if got := fmt.Sprintf("%v", fact.Args[2]); got != "explain what the OODA loop does" {
		t.Errorf("Target (Args[2]) = %q, want the user input", got)
	}
}
