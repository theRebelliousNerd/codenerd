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

// outcomeRow returns the classification_outcome row published for the turn, as
// "disposition/stage/class", or "" when none is.
func outcomeRow(t *testing.T, kernel *core.RealKernel) string {
	t.Helper()
	facts, err := kernel.Query("classification_outcome")
	if err != nil {
		t.Fatalf("Query(classification_outcome) error = %v", err)
	}
	if len(facts) == 0 {
		return ""
	}
	if len(facts) != 1 || len(facts[0].Args) != 5 {
		t.Fatalf("classification_outcome = %v, want one 5-ary row", facts)
	}
	a := facts[0].Args
	return fmt.Sprintf("%v/%v/%v", a[1], strings.TrimPrefix(fmt.Sprint(a[2]), "/"), strings.TrimPrefix(fmt.Sprint(a[3]), "/"))
}

// stopFor returns the kernel's classification_stop action and notice.
func stopFor(t *testing.T, kernel *core.RealKernel) (string, string) {
	t.Helper()
	facts, err := kernel.Query("classification_stop")
	if err != nil {
		t.Fatalf("Query(classification_stop) error = %v", err)
	}
	if len(facts) == 0 {
		return "", ""
	}
	if len(facts) != 1 || len(facts[0].Args) != 3 {
		t.Fatalf("classification_stop = %v, want one row", facts)
	}
	return fmt.Sprintf("%v", facts[0].Args[1]), fmt.Sprintf("%v", facts[0].Args[2])
}

func perceiveWith(t *testing.T, client stubLLMClient, input string) (*core.RealKernel, perception.Intent) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}
	shard := NewPerceptionFirewallShard()
	shard.SetParentKernel(kernel)
	shard.SetLLMClient(client)
	intent, err := shard.Perceive(ctx, input, nil)
	if err != nil {
		t.Fatalf("Perceive error = %v", err)
	}
	return kernel, intent
}

// TestPerception_TransientLLMFailure_PublishesTheOutage proves the outage case
// end to end at the firewall: when classification fails with the
// ErrLLMUnavailable sentinel (the 503 case), the firewall publishes a
// retryable provider_failed outcome and the kernel derives the honest notice,
// never the /heuristic_low (user-ambiguity) reason the live bug produced. The
// transducer swallows the error into a degraded /explain intent with a
// nil error, so the typed Outcome is the only carrier of the cause.
func TestPerception_TransientLLMFailure_PublishesTheOutage(t *testing.T) {
	// Mirror the production wrapping chain: retries wrap the sentinel; the
	// classifier keeps its class through every %w layer.
	transientErr := fmt.Errorf("max retries exceeded: %w", perception.ErrLLMUnavailable)
	kernel, intent := perceiveWith(t, stubLLMClient{err: transientErr}, "what is the jit system?")

	if intent.Verb != "/explain" {
		t.Fatalf("intent.Verb = %s, want /explain", intent.Verb)
	}
	if got := outcomeRow(t, kernel); got != "/provider_failed/transport/transient" {
		t.Fatalf("classification_outcome = %q, want /provider_failed/transport/transient", got)
	}
	action, notice := stopFor(t, kernel)
	if action != "/retry" || !strings.Contains(notice, "couldn't reach the language model") {
		t.Fatalf("classification_stop = %q %q, want /retry with the outage notice", action, notice)
	}
	// Distinguishability: it must NOT have been laundered into user-ambiguity.
	if reasonAsserted(t, kernel, "/heuristic_low") {
		t.Fatalf("transient outage wrongly reported as /heuristic_low (user ambiguity)")
	}
	if clarificationDerived(t, kernel, "I'm not confident I understood") {
		t.Fatalf("transient outage produced a user-ambiguity clarification message")
	}
	if facts, _ := kernel.Query("ambiguity_flag"); len(facts) != 0 {
		t.Fatalf("a failed classification raised ambiguity_flag: %v", facts)
	}
}

// TestPerception_NonTransientFailures_AreTypedAndDistinct pins each cause the
// compatibility intent used to flatten to /heuristic_low: an authentication
// failure and a rejected request are blockers, an unreadable answer is a
// retry, and a canceled call is its own outcome with no semantic question.
func TestPerception_NonTransientFailures_AreTypedAndDistinct(t *testing.T) {
	cases := []struct {
		name       string
		client     stubLLMClient
		wantRow    string
		wantAction string
		wantNotice string
	}{
		{"authentication", stubLLMClient{err: fmt.Errorf("API request failed with status 401: bad key")},
			"/provider_failed/transport/auth", "/blocker", "rejected the configured credentials"},
		{"rejected request", stubLLMClient{err: fmt.Errorf("API request failed with status 422: nope")},
			"/provider_failed/transport/invalid_request", "/blocker", "rejected the request"},
		{"rate limited", stubLLMClient{err: fmt.Errorf("rate limit exceeded (429)")},
			"/provider_failed/transport/rate_limited", "/retry", "rate limiting"},
		{"unreadable answer", stubLLMClient{response: "I could not decide."},
			"/invalid_response/output/none", "/retry", "could not be understood"},
		{"envelope with a mistyped field", stubLLMClient{response: `{"understanding":{"semantic_type":"location","action_type":["investigate"],"confidence":0.9}}`},
			"/invalid_response/decode/none", "/retry", "could not be understood"},
		{"canceled", stubLLMClient{err: context.Canceled},
			"/canceled/transport/canceled", "/canceled", "canceled before it finished"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kernel, _ := perceiveWith(t, tc.client, "investigate why the parser ignores this file")
			if got := outcomeRow(t, kernel); got != tc.wantRow {
				t.Fatalf("classification_outcome = %q, want %q", got, tc.wantRow)
			}
			action, notice := stopFor(t, kernel)
			if action != tc.wantAction || !strings.Contains(notice, tc.wantNotice) {
				t.Fatalf("classification_stop = %q %q, want %s containing %q", action, notice, tc.wantAction, tc.wantNotice)
			}
			if reasonAsserted(t, kernel, "/heuristic_low") {
				t.Fatal("a failed classification was reported as user ambiguity")
			}
		})
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
	if got := outcomeRow(t, kernel); got != "/admitted/none/none" {
		t.Fatalf("classification_outcome = %q, want an admitted outcome for a read but uncertain request", got)
	}
	if action, _ := stopFor(t, kernel); action != "" {
		t.Fatalf("an uncertain reading was stopped as a classification failure: %q", action)
	}
}
