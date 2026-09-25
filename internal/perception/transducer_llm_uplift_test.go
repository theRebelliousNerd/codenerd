package perception

import (
	"context"
	"math"
	"testing"

	"codenerd/internal/core"
)

// cannedJSONClient returns a fixed classification envelope for Understand.
type cannedJSONClient struct {
	baseMockLLMClient
	response string
}

func (c *cannedJSONClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return c.response, nil
}

const upliftClassificationEnvelope = `{
  "understanding": {
    "primary_intent": "document",
    "semantic_type": "causation",
    "action_type": "document",
    "domain": "general",
    "scope": {"level": "file", "target": "auth.go", "file": "auth.go", "symbol": ""},
    "user_constraints": [],
    "implicit_assumptions": [],
    "confidence": 0.9,
    "signals": {"is_question": false, "is_hypothetical": false, "is_multi_step": false,
      "is_negated": false, "requires_confirmation": false, "urgency": "normal"},
    "suggested_approach": {"mode": "normal", "primary_shard": "coder",
      "supporting_shards": [], "tools_needed": ["custom_tool"], "context_needed": ["custom_ctx"]}
  },
  "surface_response": "Documenting auth.go."
}`

// TestUnderstand_DerivesRoutingFromMangle pins the executive half of the
// LLM-first contract: with a kernel wired, the harness mode, primary shard,
// and context/tool priorities come from the Mangle affinity tables — not
// from the LLM's suggestions. The canned envelope deliberately disagrees
// with the tables (suggests mode=normal + shard=coder) so a verbatim
// pass-through cannot pass: causation must derive mode=debug, and the
// document action must derive the researcher shard.
func TestUnderstand_DerivesRoutingFromMangle(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	tr := NewLLMTransducer(&cannedJSONClient{response: upliftClassificationEnvelope},
		NewRealKernelRouter(kernel), "system")
	u, err := tr.Understand(context.Background(), "write docs for auth.go", nil, nil, nil, "")
	if err != nil {
		t.Fatalf("Understand: %v", err)
	}
	if u.Routing == nil {
		t.Fatal("Routing is nil with kernel wired")
	}
	if u.Routing.Mode != "debug" {
		t.Errorf("Routing.Mode=%q, want %q (mode_from_semantic causation)", u.Routing.Mode, "debug")
	}
	if u.Routing.PrimaryShard != "researcher" {
		t.Errorf("Routing.PrimaryShard=%q, want %q (shard_affinity_action document)",
			u.Routing.PrimaryShard, "researcher")
	}
	if got := u.Routing.ContextPriorities["target_source"]; got != 90 {
		t.Errorf("ContextPriorities[target_source]=%d, want 90 (document affinity)", got)
	}
	if got := u.Routing.ContextPriorities["error_logs"]; got != 100 {
		t.Errorf("ContextPriorities[error_logs]=%d, want 100 (causation affinity)", got)
	}
	if got := u.Routing.ToolPriorities["write_file"]; got != 90 {
		t.Errorf("ToolPriorities[write_file]=%d, want 90 (document affinity)", got)
	}
	// LLM suggestions merge in at moderate priority without overriding tables.
	if got := u.Routing.ToolPriorities["custom_tool"]; got != 70 {
		t.Errorf("ToolPriorities[custom_tool]=%d, want 70 (LLM suggestion default)", got)
	}
	if got := u.Routing.ContextPriorities["custom_ctx"]; got != 70 {
		t.Errorf("ContextPriorities[custom_ctx]=%d, want 70 (LLM suggestion default)", got)
	}
}

// TestUnderstand_AssertsAndReplacesRoutingFacts pins the kernel write path:
// the derived routing lands in the EDB for downstream rules, and a second
// turn replaces (not accumulates) the per-turn predicates.
func TestUnderstand_AssertsAndReplacesRoutingFacts(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	tr := NewLLMTransducer(&cannedJSONClient{response: upliftClassificationEnvelope},
		NewRealKernelRouter(kernel), "system")
	ctx := context.Background()
	if _, err := tr.Understand(ctx, "first", nil, nil, nil, ""); err != nil {
		t.Fatalf("first Understand: %v", err)
	}
	assertFactCount := func(pred string, want int) []core.Fact {
		t.Helper()
		facts, err := kernel.Query(pred)
		if err != nil {
			t.Fatalf("query %s: %v", pred, err)
		}
		if len(facts) != want {
			t.Fatalf("%s: %d facts, want %d", pred, len(facts), want)
		}
		return facts
	}
	assertFactCount("current_understanding", 1)
	assertFactCount("derived_mode", 1)
	assertFactCount("derived_primary_shard", 1)
	if facts := assertFactCount("derived_context_priority", 7); len(facts) != 7 {
		t.Fatalf("derived_context_priority: %d facts, want 7 (4 causation + 2 document + 1 custom)", len(facts))
	}

	second := `{"understanding": {"primary_intent": "fix", "semantic_type": "temporal",
	  "action_type": "implement", "domain": "git", "scope": {"level": "file", "target": "x.go"},
	  "confidence": 0.8, "signals": {}, "suggested_approach": {"mode": "normal", "primary_shard": "coder"}},
	  "surface_response": "Fixing."}`
	tr2 := NewLLMTransducer(&cannedJSONClient{response: second},
		NewRealKernelRouter(kernel), "system")
	if _, err := tr2.Understand(ctx, "second", nil, nil, nil, ""); err != nil {
		t.Fatalf("second Understand: %v", err)
	}
	// Replacement, not accumulation: still exactly one fact per predicate.
	assertFactCount("current_understanding", 1)
	assertFactCount("derived_mode", 1)
	assertFactCount("derived_primary_shard", 1)
	facts, err := kernel.Query("current_understanding")
	if err != nil {
		t.Fatalf("query current_understanding: %v", err)
	}
	if len(facts[0].Args) != 4 {
		t.Fatalf("current_understanding arity=%d, want 4", len(facts[0].Args))
	}
}

// TestUnderstand_NoKernelUsesSuggestions pins the degraded fallback: without
// a kernel the harness cannot derive, so it passes the LLM's suggestions
// through verbatim instead of failing the turn.
func TestUnderstand_NoKernelUsesSuggestions(t *testing.T) {
	tr := NewLLMTransducer(&cannedJSONClient{response: upliftClassificationEnvelope}, nil, "system")
	u, err := tr.Understand(context.Background(), "write docs", nil, nil, nil, "")
	if err != nil {
		t.Fatalf("Understand: %v", err)
	}
	if u.Routing == nil {
		t.Fatal("Routing is nil without kernel")
	}
	if u.Routing.Mode != "normal" || u.Routing.PrimaryShard != "coder" {
		t.Errorf("fallback routing=%+v, want verbatim suggestions (normal/coder)", u.Routing)
	}
}

// TestParseResponse_EnvelopeWithoutPrimaryIntent pins key-presence detection:
// an envelope whose model omitted the one-word summary is still an envelope.
// Falling through to the direct parse would zero every field.
func TestParseResponse_EnvelopeWithoutPrimaryIntent(t *testing.T) {
	tr := &LLMTransducer{}
	u, err := tr.parseResponse(`{"understanding": {"semantic_type": "causation",
	  "action_type": "investigate", "domain": "testing", "confidence": 0.7},
	  "surface_response": "Looking into it."}`)
	if err != nil {
		t.Fatalf("parseResponse: %v", err)
	}
	if u.ActionType != "investigate" || u.SemanticType != "causation" || u.Domain != "testing" {
		t.Errorf("envelope fields lost: %+v", u)
	}
	if u.SurfaceResponse != "Looking into it." {
		t.Errorf("SurfaceResponse=%q, want envelope value", u.SurfaceResponse)
	}
}

// TestParseResponse_NullUnderstandingFallsThrough pins that a null
// understanding value is not an envelope: it falls to the direct parse.
func TestParseResponse_NullUnderstandingFallsThrough(t *testing.T) {
	tr := &LLMTransducer{}
	u, err := tr.parseResponse(`{"understanding": null, "surface_response": "hi"}`)
	if err != nil {
		t.Fatalf("parseResponse: %v", err)
	}
	if u.SurfaceResponse != "hi" {
		t.Errorf("SurfaceResponse=%q, want %q", u.SurfaceResponse, "hi")
	}
}

// TestNormalizeLLMFields_TrimsAndClamps pins model-output hygiene: padded
// values trim (so routing lookups hit) and out-of-range confidence clamps
// (downstream gates compare against 0..1 thresholds).
func TestNormalizeLLMFields_TrimsAndClamps(t *testing.T) {
	u := &Understanding{
		SemanticType: "  Causation ", ActionType: " IMPLEMENT ", Domain: " Testing ",
		Scope: Scope{Level: " File "}, SuggestedApproach: SuggestedApproach{Mode: " TDD "},
		Confidence: 0.9,
	}
	normalizeLLMFields(u)
	if u.SemanticType != "causation" || u.ActionType != "implement" || u.Domain != "testing" {
		t.Errorf("fields not trimmed+lowered: %+v", u)
	}
	if u.Scope.Level != "file" || u.SuggestedApproach.Mode != "tdd" {
		t.Errorf("nested fields not normalized: %+v", u)
	}
	for _, c := range []struct {
		in   float64
		want float64
	}{
		{2.5, 1}, {85.0, 1}, {math.Inf(1), 1},
		{-0.5, 0}, {math.Inf(-1), 0}, {math.NaN(), 0},
		{0.7, 0.7}, {0, 0}, {1, 1},
	} {
		v := &Understanding{Confidence: c.in}
		normalizeLLMFields(v)
		if v.Confidence != c.want {
			t.Errorf("clamp(%v)=%v, want %v", c.in, v.Confidence, c.want)
		}
	}
}

// TestCleanRoutingAtom pins atom normalization for kernel asserts.
func TestCleanRoutingAtom(t *testing.T) {
	cases := map[string]string{
		"implement":  "/implement",
		"/implement": "/implement",
		"  spaced  ": "/spaced",
		"":           "",
		"/":          "",
		"two words":  "",
		"tab\there":  "",
		"error_logs": "/error_logs",
	}
	for in, want := range cases {
		if got := cleanRoutingAtom(in); got != want {
			t.Errorf("cleanRoutingAtom(%q)=%q, want %q", in, got, want)
		}
	}
}

// TestIsActionRequest_CoversMutations pins the single-source-of-truth
// unification: every mutation action plus verify is an action request;
// inspection, conversation, and memory turns are not.
func TestIsActionRequest_CoversMutations(t *testing.T) {
	doActions := []string{"implement", "modify", "refactor", "attack", "revert",
		"configure", "migrate", "optimize", "document", "scaffold", "format",
		"deploy", "verify", "  MODIFY "}
	for _, a := range doActions {
		u := &Understanding{ActionType: a}
		if !u.IsActionRequest() {
			t.Errorf("IsActionRequest(%q)=false, want true", a)
		}
	}
	notActions := []string{"investigate", "explain", "research", "review", "audit",
		"lint", "benchmark", "profile", "chat", "remember", "forget", ""}
	for _, a := range notActions {
		u := &Understanding{ActionType: a}
		if u.IsActionRequest() {
			t.Errorf("IsActionRequest(%q)=true, want false", a)
		}
	}
}
