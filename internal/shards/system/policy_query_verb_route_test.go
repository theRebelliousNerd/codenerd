package system

import (
	"fmt"
	"testing"

	"codenerd/internal/core"
)

// queryVerb assembles a /query-category user_intent for a taxonomy verb and
// returns the derived next_action atoms.
func queryVerbNextActions(t *testing.T, verb string) []string {
	t.Helper()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	// user_intent(IntentID, Category, Verb, Target, Constraint)
	if err := kernel.Assert(core.Fact{
		Predicate: "user_intent",
		Args:      []any{"/current_intent", "/query", verb, "internal/core", ""},
	}); err != nil {
		t.Fatalf("Assert user_intent(%s): %v", verb, err)
	}
	facts, err := kernel.Query("next_action")
	if err != nil {
		t.Fatalf("Query next_action: %v", err)
	}
	var got []string
	for _, f := range facts {
		if len(f.Args) > 0 {
			got = append(got, fmt.Sprintf("%v", f.Args[0]))
		}
	}
	return got
}

// TestPolicyDerivesNextAction_QueryVerbsRoute is the deterministic regression
// for the F-ROUTE-3 residual (task_dc40000b): /lint, /benchmark, /profile
// classified correctly (taxonomy.go: /benchmark:770, /profile:775, /lint:790)
// but had no action_mapping, so `nerd run "<verb> ..."` derived no next_action
// and died "no action derived from policy" (exit 1). /lint routes to the
// reviewer (analysis, prose verdict OK); /benchmark and /profile route to the
// tester (which executes measurement). Proven red ([] without the mapping) and
// green.
func TestPolicyDerivesNextAction_QueryVerbsRoute(t *testing.T) {
	cases := []struct {
		verb   string
		action string
	}{
		{"/lint", "/delegate_reviewer"},
		{"/benchmark", "/delegate_tester"},
		{"/profile", "/delegate_tester"},
	}

	for _, tc := range cases {
		t.Run(tc.verb, func(t *testing.T) {
			got := queryVerbNextActions(t, tc.verb)
			found := false
			for _, a := range got {
				if a == tc.action {
					found = true
				}
			}
			if !found {
				t.Errorf("verb %s: expected next_action(%s); got %v", tc.verb, tc.action, got)
			}
		})
	}
}

// TestPolicyIntentRequiresToolCall_TesterVsReviewer verifies the side_effecting
// design decision: /benchmark and /profile route to the tester which EXECUTES,
// so intent_requires_tool_call is true (a prose-only turn is not acceptable);
// /lint routes to the reviewer which ANALYZES, so it is false (a prose verdict
// is a valid terminal response).
func TestPolicyIntentRequiresToolCall_TesterVsReviewer(t *testing.T) {
	requiresToolCall := func(verb string) bool {
		kernel, err := core.NewRealKernel()
		if err != nil {
			t.Fatalf("NewRealKernel: %v", err)
		}
		if err := kernel.Assert(core.Fact{
			Predicate: "user_intent",
			Args:      []any{"/current_intent", "/query", verb, "internal/core", ""},
		}); err != nil {
			t.Fatalf("Assert user_intent(%s): %v", verb, err)
		}
		facts, err := kernel.Query("intent_requires_tool_call")
		if err != nil {
			t.Fatalf("Query intent_requires_tool_call: %v", err)
		}
		for _, f := range facts {
			if len(f.Args) > 0 && fmt.Sprintf("%v", f.Args[0]) == verb {
				return true
			}
		}
		return false
	}

	if !requiresToolCall("/benchmark") {
		t.Error("/benchmark should require a tool_call (tester executes measurement)")
	}
	if !requiresToolCall("/profile") {
		t.Error("/profile should require a tool_call (tester executes measurement)")
	}
	if requiresToolCall("/lint") {
		t.Error("/lint should NOT require a tool_call (reviewer prose verdict is valid)")
	}
}
