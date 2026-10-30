package shards

import (
	"codenerd/internal/core"
	"codenerd/internal/types"
	"testing"
)

// The component kernel and the sharded production corpus must agree. These
// facts join a globally shared config_param row with prompt-owned measures.
func TestPromptEvolutionPolicyFiresOnTheShardedKernel(t *testing.T) {
	k := bootGateCortex(t)
	facts := []core.Fact{
		{Predicate: "config_param", Args: []any{types.MangleAtom("/evolution_cohort_size"), 1}},
		{Predicate: "config_param", Args: []any{types.MangleAtom("/evolution_screen_calls"), 1}},
		{Predicate: "evolution_evidence_rank", Args: []any{"g", "selected", "v", 0}},
		{Predicate: "evolution_evidence_rank", Args: []any{"g", "deferred", "v", 1}},
		{Predicate: "evolution_screen_used", Args: []any{"c", 0}},
		{Predicate: "evolution_screen_used", Args: []any{"exhausted", 1}},
		{Predicate: "evolution_screen_observation", Args: []any{"c", types.MangleAtom("/answered")}},
		{Predicate: "evolution_screen_probability", Args: []any{"c", .9}},
		{Predicate: "evolution_screen_threshold", Args: []any{"c", .8}},
	}
	for _, f := range facts {
		if err := k.Assert(f); err != nil {
			t.Fatal(err)
		}
	}
	if err := k.Evaluate(); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"evolution_cohort_member", "evolution_screen_allowed", "evolution_generation_allowed"} {
		rows, err := k.Query(p)
		if err != nil || len(rows) != 1 {
			t.Fatalf("%s=%v err=%v", p, rows, err)
		}
	}
}
