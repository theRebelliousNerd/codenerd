package system

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/types"
)

// A return guard (`if x == nil { return }`) protects the uses after it in the
// same function. The rule asked for same_scope(Var, File, L1, L2), a fact
// nothing asserted, while the world scanner emits function_scope; so on the
// production kernel no return guard ever guarded anything. The rule now reads
// function_scope, which must sit in the shard that holds uses and guards_return.
func TestReturnGuardsGuardUsesInTheSameFunctionOnTheShardedKernel(t *testing.T) {
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

	user, other := types.MangleAtom("/handle"), types.MangleAtom("/other")
	x := types.MangleAtom("/x")
	facts := []types.Fact{
		// handle spans lines 10-30; other spans 40-60.
		{Predicate: "function_scope", Args: []any{"a.go", user, int64(10), int64(30)}},
		{Predicate: "function_scope", Args: []any{"a.go", other, int64(40), int64(60)}},
		{Predicate: "guards_return", Args: []any{x, types.MangleAtom("/nil_check"), "a.go", int64(12)}},
		{Predicate: "uses", Args: []any{"a.go", user, x, int64(15)}},  // after the guard, same function
		{Predicate: "uses", Args: []any{"a.go", user, x, int64(11)}},  // before the guard
		{Predicate: "uses", Args: []any{"a.go", other, x, int64(45)}}, // after the guard, another function
	}
	if err := cortex.Kernel.AssertBatch(facts); err != nil {
		t.Fatalf("assert: %v", err)
	}
	rows, err := cortex.Kernel.Query("is_guarded")
	if err != nil {
		t.Fatal(err)
	}
	if !factWith(rows, "/x", "a.go", "15") {
		t.Errorf("a use after a return guard in the same function is guarded: %v", rows)
	}
	if factWith(rows, "/x", "a.go", "11") {
		t.Errorf("a use before the guard is not guarded: %v", rows)
	}
	if factWith(rows, "/x", "a.go", "45") {
		t.Errorf("a guard does not reach a use in another function: %v", rows)
	}
}
