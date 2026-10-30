package system

import (
	"context"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

// This boots the factory, including production predicate homes. A monolithic
// kernel test alone cannot detect a negation reading an empty sibling shard.
func TestDeliberationRulesFireAfterCortexBoot(t *testing.T) {
	if testing.Short() {
		t.Skip("full cortex boot")
	}
	ws := t.TempDir()
	cortex, err := BootCortex(context.Background(), ws, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer requireWorkspaceReleased(t, cortex, ws)
	if err := config.EnsureParams(cortex.Kernel, config.DefaultRoutingConfig().Params()); err != nil {
		t.Fatal(err)
	}
	facts := []types.Fact{
		{Predicate: "deliberation_tick", Args: []any{"boot-thread", int64(1), int64(0)}},
		{Predicate: "deliberation_mode", Args: []any{"boot-thread", types.MangleAtom("/yolo")}},
		{Predicate: "deliberation_candidate", Args: []any{"boot-thread", int64(1), "blocked", int64(0)}},
		{Predicate: "deliberation_candidate", Args: []any{"boot-thread", int64(1), "survivor", int64(1)}},
		{Predicate: "deliberation_success", Args: []any{"boot-thread", int64(1), "blocked"}},
		{Predicate: "deliberation_success", Args: []any{"boot-thread", int64(1), "survivor"}},
		{Predicate: "deliberation_item", Args: []any{"boot-thread", int64(1), "blocked", "critical", types.MangleAtom("/critical"), types.MangleAtom("/design")}},
		{Predicate: "deliberation_recommended", Args: []any{"boot-thread", int64(1), "blocked"}},
		{Predicate: "deliberation_brief", Args: []any{"boot-thread", int64(1), "survivor", "a passing reading", "the success test"}},
		{Predicate: "deliberation_citation", Args: []any{"boot-thread", int64(1), "survivor", "the original request"}},
		{Predicate: "deliberation_round", Args: []any{"boot-thread", int64(1)}},
	}
	if err := cortex.Kernel.AssertBatch(facts); err != nil {
		t.Fatal(err)
	}
	rows, err := cortex.Kernel.Query("deliberation_disposition")
	if err != nil {
		t.Fatal(err)
	}
	if !factWith(rows, "boot-thread", "1", "/proceed", "survivor") {
		t.Fatalf("blocked top candidate survived, or the joined rules did not fire: %v", rows)
	}
}
