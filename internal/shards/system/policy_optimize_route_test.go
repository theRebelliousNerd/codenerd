package system

import (
	"fmt"
	"testing"

	"codenerd/internal/core"
)

// TestPolicyDerivesNextAction_OptimizeRoutesToCoder is the deterministic
// regression for F-ROUTE-3.
//
// Perception emits the /optimize verb (taxonomy.go:760, Category /mutation,
// ShardType /coder) for "optimize / speed up / make faster" inputs, but
// delegation.mg had no action_mapping for it. The one-shot `nerd run` path
// (cmd_instruction.go) queries next_action after finding no delegate_task and,
// on an empty result, fails with "no action derived from policy" (exit 1) —
// the identical gap /audit had before F-ROUTE-2. Adding
// `action_mapping(/optimize, /delegate_coder).` makes next_action(/delegate_coder)
// derive, which nextActionToShardType maps to the coder shard for SpawnTask.
//
// This asserts a user_intent for /optimize against a real kernel (which embeds
// delegation.mg) and proves next_action(/delegate_coder) is derived. Pre-fix
// this query returns no coder handoff; post-fix it does.
func TestPolicyDerivesNextAction_OptimizeRoutesToCoder(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}

	// user_intent(IntentID, Category, Verb, Target, Constraint)
	if err := kernel.Assert(core.Fact{
		Predicate: "user_intent",
		Args:      []any{"/current_intent", "/mutation", "/optimize", "internal/session/executor.go", ""},
	}); err != nil {
		t.Fatalf("Assert user_intent: %v", err)
	}

	facts, err := kernel.Query("next_action")
	if err != nil {
		t.Fatalf("Query next_action: %v", err)
	}

	found := false
	var got []string
	for _, f := range facts {
		if len(f.Args) == 0 {
			continue
		}
		action := fmt.Sprintf("%v", f.Args[0])
		got = append(got, action)
		if action == "/delegate_coder" {
			found = true
		}
	}

	if !found {
		t.Errorf("expected next_action(/delegate_coder) derived from /optimize intent; got next_action facts: %v", got)
	}
}
