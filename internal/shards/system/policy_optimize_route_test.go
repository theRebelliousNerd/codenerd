package system

import (
	"fmt"
	"testing"

	"codenerd/internal/core"
)

// TestPolicyDerivesNextAction_MutationVerbsRouteToCoder is the deterministic
// regression for F-ROUTE-3.
//
// Perception classifies "optimize / speed up", "migrate / port", "format /
// gofmt", and "scaffold / bootstrap" inputs to the /optimize, /migrate,
// /format, and /scaffold verbs — all Category /mutation, ShardType /coder
// (taxonomy.go: /migrate:755, /optimize:760, /scaffold:785, /format:795).
// None had an action_mapping in delegation.mg, so the one-shot `nerd run` path
// (cmd_instruction.go) queried next_action after finding no delegate_task and,
// on an empty result, failed with "no action derived from policy" (exit 1) —
// the identical gap /audit had before F-ROUTE-2. Adding
// `action_mapping(/<verb>, /delegate_coder).` makes next_action(/delegate_coder)
// derive, which nextActionToShardType maps to the coder shard for SpawnTask.
//
// Each case asserts a user_intent against a real kernel (which embeds
// delegation.mg) and proves next_action(/delegate_coder) is derived. Pre-fix
// each query returns no coder handoff (verified red: []); post-fix it does.
func TestPolicyDerivesNextAction_MutationVerbsRouteToCoder(t *testing.T) {
	verbs := []string{"/optimize", "/migrate", "/format", "/scaffold"}

	for _, verb := range verbs {
		t.Run(verb, func(t *testing.T) {
			kernel, err := core.NewRealKernel()
			if err != nil {
				t.Fatalf("NewRealKernel: %v", err)
			}

			// user_intent(IntentID, Category, Verb, Target, Constraint)
			if err := kernel.Assert(core.Fact{
				Predicate: "user_intent",
				Args:      []any{"/current_intent", "/mutation", verb, "internal/session/executor.go", ""},
			}); err != nil {
				t.Fatalf("Assert user_intent(%s): %v", verb, err)
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
				t.Errorf("verb %s: expected next_action(/delegate_coder); got next_action facts: %v", verb, got)
			}
		})
	}
}
