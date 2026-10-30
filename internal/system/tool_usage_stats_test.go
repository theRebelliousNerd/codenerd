package system

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/types"
)

// tool_usage_stats had a Decl and two readers (the success and recency boosts of
// tool_routing.mg) and no producer, so every tool scored 0 for both on the
// production kernel. It is now derived from the tool_execution facts the tool
// fact sink asserts. This runs on the real boot path: the sharded kernel, where
// tool_execution, tool_registered and the routing rules must share a shard.
func TestToolUsageStatsAreDerivedFromExecutionsOnTheShardedKernel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	ws := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cortex, err := BootCortex(ctx, ws, "", nil)
	if err != nil {
		t.Fatalf("BootCortex: %v", err)
	}
	defer requireWorkspaceReleased(t, cortex, ws)

	ok, bad := types.MangleAtom("/true"), types.MangleAtom("/false")
	facts := []types.Fact{
		{Predicate: "tool_registered", Args: []any{"go_build", int64(1)}},
		{Predicate: "tool_registered", Args: []any{"go_vet", int64(1)}},
		{Predicate: "tool_registered", Args: []any{"never_run", int64(1)}},
		{Predicate: "tool_execution", Args: []any{"go_build", ok, int64(100)}},
		{Predicate: "tool_execution", Args: []any{"go_build", bad, int64(200)}},
		{Predicate: "tool_execution", Args: []any{"go_vet", bad, int64(300)}},
		{Predicate: "current_time", Args: []any{int64(400)}},
	}
	if err := cortex.Kernel.AssertBatch(facts); err != nil {
		t.Fatalf("assert: %v", err)
	}

	stats, err := cortex.Kernel.Query("tool_usage_stats")
	if err != nil {
		t.Fatal(err)
	}
	if !factWith(stats, "go_build", "2", "1", "200") {
		t.Errorf("go_build ran twice, once successfully, last at 200: %v", stats)
	}
	if !factWith(stats, "go_vet", "1", "0", "300") {
		t.Errorf("go_vet ran once and never succeeded, last at 300: %v", stats)
	}
	for _, f := range stats {
		if len(f.Args) > 0 && types.ExtractString(f.Args[0]) == "never_run" {
			t.Errorf("a tool that never ran has usage stats: %v", f)
		}
	}

	success, err := cortex.Kernel.Query("tool_success_relevance")
	if err != nil {
		t.Fatal(err)
	}
	if !factWith(success, "go_build", "20") {
		t.Errorf("a tool with a success earns the success boost: %v", success)
	}
	if factWith(success, "go_vet", "20") {
		t.Errorf("a tool with no success must not earn the success boost: %v", success)
	}
	if !factWith(success, "never_run", "0") {
		t.Errorf("a tool with no history keeps the zero default: %v", success)
	}
}
