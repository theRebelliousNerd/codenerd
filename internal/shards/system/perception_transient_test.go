package system

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/perception"
)

// reasonAsserted reports whether intent_unknown(_, reason) is present in the kernel.
func reasonAsserted(t *testing.T, kernel *core.RealKernel, reason string) bool {
	t.Helper()
	facts, err := kernel.Query("intent_unknown")
	if err != nil {
		t.Fatalf("Query(intent_unknown) error = %v", err)
	}
	for _, f := range facts {
		if len(f.Args) < 2 {
			continue
		}
		if fmt.Sprintf("%v", f.Args[1]) == reason {
			return true
		}
	}
	return false
}

// TestPerception_TransientLLMFailure_EmitsLLMUnavailable proves Layer 2 end to
// end at the firewall: when classification fails with the ErrLLMUnavailable
// sentinel (the 503 case), the firewall must assert intent_unknown(_, /llm_unavailable)
// rather than the misleading /heuristic_low (user-ambiguity) reason that the live
// bug produced. The transducer swallows the error into a degraded /explain intent
// with a nil error, so parseFailed is false and the TransientFailure flag is the
// only carrier of the signal.
func TestPerception_TransientLLMFailure_EmitsLLMUnavailable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	// Mirror the production wrapping chain: retries wrap the sentinel; Understand
	// re-wraps with "LLM classification failed: %w"; errors.Is must still hold.
	transientErr := fmt.Errorf("max retries exceeded: %w", perception.ErrLLMUnavailable)

	shard := NewPerceptionFirewallShard()
	shard.SetParentKernel(kernel)
	shard.SetLLMClient(stubLLMClient{err: transientErr})

	intent, err := shard.Perceive(ctx, "what is the jit system?", nil)
	if err != nil {
		t.Fatalf("Perceive error = %v", err)
	}
	// Degraded fallback verb.
	if intent.Verb != "/explain" {
		t.Fatalf("intent.Verb = %s, want /explain", intent.Verb)
	}

	if !reasonAsserted(t, kernel, "/llm_unavailable") {
		t.Fatalf("intent_unknown(_, /llm_unavailable) not asserted on a transient model outage")
	}
	// Distinguishability: it must NOT have been laundered into user-ambiguity.
	if reasonAsserted(t, kernel, "/heuristic_low") {
		t.Fatalf("transient outage wrongly reported as /heuristic_low (user ambiguity)")
	}

	// Close the loop: the honest clarification message must actually be DERIVED in
	// the kernel from the /llm_unavailable reason. This is the text the live chat
	// path surfaces (via shouldClarifyFromKernel querying clarification_question),
	// so asserting it here proves deliverable (b) reaches the user and documents
	// the canonical wording (clarification.mg is the single source of truth).
	const wantSubstr = "couldn't reach the language model"
	if !clarificationDerived(t, kernel, wantSubstr) {
		t.Fatalf("clarification_question with %q not derived for /llm_unavailable", wantSubstr)
	}
	// And it must NOT blame the user's phrasing.
	if clarificationDerived(t, kernel, "I'm not confident I understood") {
		t.Fatalf("transient outage produced a user-ambiguity clarification message")
	}
}

// clarificationDerived reports whether any clarification_question fact contains
// the given substring in its message argument.
func clarificationDerived(t *testing.T, kernel *core.RealKernel, substr string) bool {
	t.Helper()
	facts, err := kernel.Query("clarification_question")
	if err != nil {
		t.Fatalf("Query(clarification_question) error = %v", err)
	}
	for _, f := range facts {
		for _, a := range f.Args {
			if s, ok := a.(string); ok && strings.Contains(s, substr) {
				return true
			}
		}
	}
	return false
}

// TestPerception_LowConfidence_EmitsHeuristicLow is the contrast control: a real,
// successfully-classified-but-low-confidence parse must still produce /heuristic_low
// and must NOT be mislabeled /llm_unavailable. Together with the test above this
// proves the two failure modes are genuinely distinguishable end to end.
func TestPerception_LowConfidence_EmitsHeuristicLow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	// Valid classification JSON but with low confidence (< AmbiguityThreshold 0.7).
	// Uses a mapped verb (/explain) so we land on /heuristic_low, not /no_verb_match
	// or intent_unmapped.
	lowConfJSON := `{"understanding":{"primary_intent":"unclear request","semantic_type":"definition","action_type":"explain","domain":"general","scope":{"level":"codebase","target":"","file":"","symbol":""},"user_constraints":[],"implicit_assumptions":[],"confidence":0.2,"signals":{"is_question":true,"is_hypothetical":false,"is_multi_step":false,"is_negated":false,"requires_confirmation":false,"urgency":"normal"},"suggested_approach":{"mode":"normal","primary_shard":"researcher","supporting_shards":[],"tools_needed":[],"context_needed":[]}},"surface_response":"ok"}`

	shard := NewPerceptionFirewallShard()
	shard.SetParentKernel(kernel)
	shard.SetLLMClient(stubLLMClient{response: lowConfJSON})

	if _, err := shard.Perceive(ctx, "hmm something", nil); err != nil {
		t.Fatalf("Perceive error = %v", err)
	}

	if !reasonAsserted(t, kernel, "/heuristic_low") {
		t.Fatalf("intent_unknown(_, /heuristic_low) not asserted on a low-confidence parse")
	}
	if reasonAsserted(t, kernel, "/llm_unavailable") {
		t.Fatalf("low-confidence parse wrongly reported as /llm_unavailable")
	}
}
