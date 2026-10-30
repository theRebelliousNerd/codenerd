package policy

import (
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// Full production corpus, including the schema inventory. Missing schema
// registration, wrong float comparisons, or a missing budget premise must be
// observable at the same query boundary as the adaptation component kernel.
func TestPromptEvolutionKernelCohortAndScreenAdmission(t *testing.T) {
	k, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := k.AssertBatch([]core.Fact{
		{Predicate: "config_param", Args: []any{types.MangleAtom("/evolution_cohort_size"), 2}},
		{Predicate: "config_param", Args: []any{types.MangleAtom("/evolution_screen_calls"), 1}},
		{Predicate: "evolution_evidence_rank", Args: []any{"g", "new", "v", 0}},
		{Predicate: "evolution_evidence_rank", Args: []any{"g", "old", "v", 1}},
		{Predicate: "evolution_evidence_rank", Args: []any{"g", "middle", "v", 2}},
		{Predicate: "evolution_screen_used", Args: []any{"available", 0}},
		{Predicate: "evolution_screen_used", Args: []any{"exhausted", 1}},
		{Predicate: "evolution_screen_observation", Args: []any{"positive", types.MangleAtom("/answered")}},
		{Predicate: "evolution_screen_probability", Args: []any{"positive", .9}},
		{Predicate: "evolution_screen_threshold", Args: []any{"positive", .8}},
		{Predicate: "evolution_screen_observation", Args: []any{"unsettled", types.MangleAtom("/answered")}},
		{Predicate: "evolution_screen_probability", Args: []any{"unsettled", .5}},
		{Predicate: "evolution_screen_threshold", Args: []any{"unsettled", .8}},
		{Predicate: "evolution_screen_observation", Args: []any{"unavailable", types.MangleAtom("/unavailable")}},
	}); err != nil {
		t.Fatal(err)
	}
	for pred, want := range map[string]int{"evolution_cohort_member": 2, "evolution_screen_allowed": 1, "evolution_generation_allowed": 2} {
		rows, err := k.Query(pred)
		if err != nil || len(rows) != want {
			t.Fatalf("%s=%v err=%v, want %d", pred, rows, err, want)
		}
	}
}
