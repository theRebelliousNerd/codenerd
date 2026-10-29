package system

import (
	"fmt"
	"testing"

	"codenerd/internal/core"
)

// requiresReasoningModel asks the live policy corpus whether a verb's turn
// should be served by the high-reasoning planner LLM slot. The executor calls
// the same predicate with a bound argument (see
// internal/session/executor.go:intentRequiresReasoningModel); here we query the
// whole relation and look for the verb so a wrong-verb derivation is visible.
func requiresReasoningModel(t *testing.T, verb string) bool {
	t.Helper()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	facts, err := kernel.Query("intent_requires_reasoning_model")
	if err != nil {
		t.Fatalf("Query intent_requires_reasoning_model: %v", err)
	}
	for _, f := range facts {
		if len(f.Args) > 0 && fmt.Sprintf("%v", f.Args[0]) == verb {
			return true
		}
	}
	return false
}

// TestPolicyIntentRequiresReasoningModel is the regression for the two-tier
// model split. Pointing the worker slot at a cheap bulk model must not demote
// the turns whose output quality IS the product — otherwise the split spends
// the budget backwards. The reviewer family escapes to the reasoning tier
// because a prose verdict is the deliverable and nothing downstream checks it;
// the coder family stays cheap because tests and the tool loop check its work.
func TestPolicyIntentRequiresReasoningModel(t *testing.T) {
	reasoning := []string{
		// action_mapping -> /delegate_reviewer -> reasoning_intensive_action
		"/review", "/review_enhance", "/security", "/analyze", "/audit", "/lint",
		// reasoning_intensive_verb, no action_mapping needed
		"/campaign", "/assault", "/dream", "/shadow", "/generate_tool",
	}
	for _, verb := range reasoning {
		t.Run("reasoning"+verb, func(t *testing.T) {
			if !requiresReasoningModel(t, verb) {
				t.Errorf("%s should route to the planner LLM slot", verb)
			}
		})
	}

	// /explain is deliberately excluded despite mapping to /analyze_code: it is
	// the highest-volume verb in an interactive session, so routing it to the
	// reasoning tier would spend most of the budget on the cheapest-to-get-right
	// work. The rest are mutation verbs whose correctness is checked by tools.
	bulk := []string{"/explain", "/create", "/fix", "/refactor", "/write", "/read",
		"/test", "/document", "/search", "/benchmark", "/profile"}
	for _, verb := range bulk {
		t.Run("bulk"+verb, func(t *testing.T) {
			if requiresReasoningModel(t, verb) {
				t.Errorf("%s should stay on the cheap worker LLM slot", verb)
			}
		})
	}
}

// TestPolicyReasoningAndSideEffectingAreOrthogonal pins the design decision
// that these two predicates answer different questions, so a verb may be in
// either, both, or neither. /audit is reasoning-intensive but NOT
// side-effecting (a prose audit is a valid terminal answer); /create is
// side-effecting but NOT reasoning-intensive. If a future edit collapses one
// predicate into the other, this fails.
func TestPolicyReasoningAndSideEffectingAreOrthogonal(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	sideEffecting := func(verb string) bool {
		facts, qerr := kernel.Query("intent_requires_tool_call")
		if qerr != nil {
			t.Fatalf("Query intent_requires_tool_call: %v", qerr)
		}
		for _, f := range facts {
			if len(f.Args) > 0 && fmt.Sprintf("%v", f.Args[0]) == verb {
				return true
			}
		}
		return false
	}

	if !requiresReasoningModel(t, "/audit") || sideEffecting("/audit") {
		t.Errorf("/audit should be reasoning-intensive and not side-effecting; got reasoning=%v side_effecting=%v",
			requiresReasoningModel(t, "/audit"), sideEffecting("/audit"))
	}
	if requiresReasoningModel(t, "/create") || !sideEffecting("/create") {
		t.Errorf("/create should be side-effecting and not reasoning-intensive; got reasoning=%v side_effecting=%v",
			requiresReasoningModel(t, "/create"), sideEffecting("/create"))
	}
}
