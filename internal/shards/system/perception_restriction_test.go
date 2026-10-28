package system

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/perception"
)

const hypotheticalModifyEnvelope = `{
  "understanding": {
    "primary_intent": "analyze", "semantic_type": "hypothetical", "action_type": "modify", "domain": "general",
    "scope": {"level": "function", "target": "retryLoop", "file": "", "symbol": "retryLoop"},
    "user_constraints": ["no_changes", "confirm_first"], "implicit_assumptions": [],
    "confidence": 0.9,
    "signals": {"is_question": true, "is_hypothetical": true, "is_multi_step": false, "is_negated": false, "requires_confirmation": true, "urgency": "normal"},
    "suggested_approach": {"mode": "normal", "primary_shard": "coder", "supporting_shards": [], "tools_needed": [], "context_needed": []}
  },
  "surface_response": "Looking at what that would do."
}`

// The preferred chat ingress is the firewall. Its transducer must be attached
// to the kernel the firewall asserts into: without it routing takes its
// no-kernel path, no routing facts are published, and the recognised limit
// reaches the Intent but the kernel never derives a mode from it.
func TestPerceptionFirewall_PublishesRoutingAndCarriesTheRestriction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	shard := NewPerceptionFirewallShard()
	shard.SetParentKernel(kernel)
	shard.SetLLMClient(stubLLMClient{response: hypotheticalModifyEnvelope})

	intent, err := shard.Perceive(ctx, "what would happen if we modified retryLoop? don't change anything, check with me first", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []perception.RestrictionKind{perception.RestrictHypothetical, perception.RestrictNoChanges, perception.RestrictConfirmFirst} {
		if !intent.Restriction.Has(k) {
			t.Errorf("the firewall's intent lost %q: %v", k, intent.Restriction.Kinds())
		}
	}
	for _, pred := range []string{"current_understanding", "derived_mode"} {
		rows, err := kernel.Query(pred)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			t.Errorf("the firewall's transducer published no %s: it was never attached to the kernel", pred)
		}
	}
	// A recognised what-if derives the dream mode, not the model's suggested normal.
	rows, _ := kernel.Query("derived_mode")
	if len(rows) != 1 || len(rows[0].Args) != 1 || rows[0].Args[0] != "/dream" {
		t.Errorf("derived_mode = %v, want /dream for a hypothetical", rows)
	}
}
