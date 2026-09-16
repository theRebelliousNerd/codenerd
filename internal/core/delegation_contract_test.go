package core

import (
	"testing"

	"codenerd/internal/types"
)

// TestDelegationContract_AdapterVerbsDelegate pins the execution half of the
// perception→policy contract: every /mutation verb the understanding adapter
// emits must derive a delegate_task (executive + `nerd run` consume it) and a
// next_action handoff (the `nerd run` fallback), and must require a tool call
// (the anti-hollow gate). Before the /configure, /deploy, and /assault
// mappings, `nerd run "deploy X"` died with "no action derived from policy".
func TestDelegationContract_AdapterVerbsDelegate(t *testing.T) {
	cases := []struct {
		verb      string
		category  string
		shard     string
		action    string
		toolGated bool
	}{
		{"/deploy", "/mutation", "/coder", "/delegate_coder", true},
		// /configure has a mapping but deliberately no delegate_task rule:
		// conversational_verb(/configure) derives wants_direct_answer, which
		// would veto a guarded rule. It still reaches the coder through the
		// next_action handoff and still requires tool calls.
		{"/configure", "/mutation", "", "/delegate_coder", true},
		{"/assault", "/mutation", "/tester", "/delegate_tester", true},
		// Regression anchors for pre-existing mappings.
		{"/create", "/mutation", "/coder", "/delegate_coder", true},
		{"/test", "/query", "/tester", "/run_tests", true},
		// /review delegates but stays tool-optional: a prose verdict is a
		// valid terminal response (/delegate_reviewer is not side_effecting).
		{"/review", "/query", "/reviewer", "/delegate_reviewer", false},
		{"/migrate", "/mutation", "", "/delegate_coder", true},
	}
	for _, c := range cases {
		k, err := NewRealKernel()
		if err != nil {
			t.Fatalf("NewRealKernel: %v", err)
		}
		intent := Fact{Predicate: "user_intent", Args: []any{
			MangleAtom("/current_intent"), MangleAtom(c.category),
			MangleAtom(c.verb), "contract-target", "",
		}}
		if err := k.Assert(intent); err != nil {
			t.Fatalf("assert user_intent %s: %v", c.verb, err)
		}

		if c.shard != "" {
			delegations, err := k.Query("delegate_task")
			if err != nil {
				t.Fatalf("query delegate_task for %s: %v", c.verb, err)
			}
			found := false
			for _, d := range delegations {
				if len(d.Args) < 3 {
					continue
				}
				if types.ExtractString(d.Args[0]) == c.shard &&
					types.ExtractString(d.Args[2]) == "/pending" {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: no delegate_task(%s, _, /pending) derived", c.verb, c.shard)
			}
		}

		actions, err := k.Query("next_action")
		if err != nil {
			t.Fatalf("query next_action for %s: %v", c.verb, err)
		}
		found := false
		for _, a := range actions {
			if len(a.Args) < 1 {
				continue
			}
			if types.ExtractString(a.Args[0]) == c.action {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: next_action(%s) not derived (got %d next_actions)", c.verb, c.action, len(actions))
		}

		gated, err := k.Query("intent_requires_tool_call(" + c.verb + ")")
		if err != nil {
			t.Fatalf("query intent_requires_tool_call(%s): %v", c.verb, err)
		}
		if c.toolGated && len(gated) == 0 {
			t.Errorf("%s: intent_requires_tool_call is false; prose-only turns would pass", c.verb)
		}
		if !c.toolGated && len(gated) != 0 {
			t.Errorf("%s: intent_requires_tool_call is true; valid prose terminals would be rejected", c.verb)
		}
	}
}

// TestDelegationContract_ProseVerbsStayProse pins the other half: verbs whose
// terminal response is prose or memory must NOT derive shard delegations and
// must NOT require tool calls. /converse answers directly; /remember and
// /forget ride the articulation control packet, not shards.
func TestDelegationContract_ProseVerbsStayProse(t *testing.T) {
	cases := []struct {
		verb     string
		category string
	}{
		{"/converse", "/query"},
		{"/remember", "/instruction"},
		{"/forget", "/instruction"},
		{"/explain", "/query"},
	}
	for _, c := range cases {
		k, err := NewRealKernel()
		if err != nil {
			t.Fatalf("NewRealKernel: %v", err)
		}
		intent := Fact{Predicate: "user_intent", Args: []any{
			MangleAtom("/current_intent"), MangleAtom(c.category),
			MangleAtom(c.verb), "contract-target", "",
		}}
		if err := k.Assert(intent); err != nil {
			t.Fatalf("assert user_intent %s: %v", c.verb, err)
		}
		delegations, err := k.Query("delegate_task")
		if err != nil {
			t.Fatalf("query delegate_task for %s: %v", c.verb, err)
		}
		if len(delegations) != 0 {
			t.Errorf("%s: derived %d delegate_task facts, want none (prose terminal)", c.verb, len(delegations))
		}
		actions, err := k.Query("next_action")
		if err != nil {
			t.Fatalf("query next_action for %s: %v", c.verb, err)
		}
		for _, a := range actions {
			if len(a.Args) < 1 {
				continue
			}
			if got := types.ExtractString(a.Args[0]); len(got) > 10 && got[:10] == "/delegate" {
				t.Errorf("%s: next_action(%s) derived for a prose-terminal verb", c.verb, got)
			}
		}
		gated, err := k.Query("intent_requires_tool_call(" + c.verb + ")")
		if err != nil {
			t.Fatalf("query intent_requires_tool_call(%s): %v", c.verb, err)
		}
		if len(gated) != 0 {
			t.Errorf("%s: intent_requires_tool_call is true; direct answers would be rejected", c.verb)
		}
	}
}

// TestRoutingVocabulary_CoversPromptActions pins that the routing schema's
// validation vocabulary covers every action_type the perception prompt
// teaches the model to emit. A gap here means the harness cannot validate
// (or route affinities for) a classification it explicitly solicited.
func TestRoutingVocabulary_CoversPromptActions(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	facts, err := k.Query("valid_action_type")
	if err != nil {
		t.Fatalf("query valid_action_type: %v", err)
	}
	have := map[string]bool{}
	for _, f := range facts {
		if len(f.Args) < 1 {
			continue
		}
		have[types.ExtractString(f.Args[0])] = true
	}
	// The 23 action types in understanding_adapter.go's system prompt table.
	want := []string{
		"/investigate", "/implement", "/modify", "/refactor", "/verify",
		"/explain", "/research", "/configure", "/attack", "/revert",
		"/review", "/remember", "/forget", "/chat", "/migrate", "/optimize",
		"/document", "/benchmark", "/profile", "/audit", "/scaffold",
		"/lint", "/format",
	}
	if len(want) != 23 {
		t.Fatalf("test lists %d actions, prompt table has 23; fix the test", len(want))
	}
	for _, a := range want {
		if !have[a] {
			t.Errorf("valid_action_type(%s) missing from intent_routing.mg", a)
		}
	}
}

// TestRoutingAffinity_NewActionsHaveAffinities pins that the newly covered
// actions carry shard, tool, and context affinities so routing lookups for
// them hit instead of silently falling back.
func TestRoutingAffinity_NewActionsHaveAffinities(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	spot := []struct {
		predicate string
		action    string
	}{
		{"shard_affinity_action", "/deploy"},
		{"shard_affinity_action", "/audit"},
		{"shard_affinity_action", "/benchmark"},
		{"tool_affinity_action", "/lint"},
		{"tool_affinity_action", "/scaffold"},
		{"context_affinity_action", "/profile"},
		{"context_affinity_action", "/migrate"},
	}
	for _, s := range spot {
		facts, err := k.Query(s.predicate)
		if err != nil {
			t.Fatalf("query %s: %v", s.predicate, err)
		}
		found := false
		for _, f := range facts {
			if len(f.Args) < 1 {
				continue
			}
			if types.ExtractString(f.Args[0]) == s.action {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s has no %s facts", s.action, s.predicate)
		}
	}
}

// TestCodeDOMEdit_MutationCategoryGatesEdits pins the behavioral reason the
// adapter files migrate/optimize/document/scaffold/format/deploy as
// /mutation: next_action(/edit_element) derives only for /mutation intents
// targeting a known in-scope element. A /query filing would derive
// /query_elements instead and the turn could never edit through CodeDOM.
func TestCodeDOMEdit_MutationCategoryGatesEdits(t *testing.T) {
	newKernelWithElement := func(t *testing.T, category string) *RealKernel {
		t.Helper()
		k, err := NewRealKernel()
		if err != nil {
			t.Fatalf("NewRealKernel: %v", err)
		}
		ref := "main.go:FixAuth"
		facts := []Fact{
			{Predicate: "user_intent", Args: []any{
				MangleAtom("/current_intent"), MangleAtom(category),
				MangleAtom("/migrate"), ref, "",
			}},
			{Predicate: "code_element", Args: []any{ref, MangleAtom("/function"), "main.go", int64(1), int64(10)}},
			{Predicate: "in_scope", Args: []any{"main.go"}},
			{Predicate: "active_file", Args: []any{"main.go"}},
		}
		for _, f := range facts {
			if err := k.Assert(f); err != nil {
				t.Fatalf("assert %s: %v", f.Predicate, err)
			}
		}
		return k
	}

	hasAction := func(t *testing.T, k *RealKernel, want string) bool {
		t.Helper()
		actions, err := k.Query("next_action")
		if err != nil {
			t.Fatalf("query next_action: %v", err)
		}
		for _, a := range actions {
			if len(a.Args) > 0 && types.ExtractString(a.Args[0]) == want {
				return true
			}
		}
		return false
	}

	kMut := newKernelWithElement(t, "/mutation")
	if !hasAction(t, kMut, "/edit_element") {
		t.Error("/mutation migrate turn did not derive next_action(/edit_element)")
	}
	kQuery := newKernelWithElement(t, "/query")
	if hasAction(t, kQuery, "/edit_element") {
		t.Error("/query migrate turn derived next_action(/edit_element); category gate is open")
	}
}
