package core

import (
	"fmt"
	"testing"
)

// TestEvaluate_DemotesDifferentialPathWhenSlowerThanFull pins the adaptive
// switch: once a differential evaluation exceeds diffDemoteThreshold the
// kernel drops the engine and evaluates by full fixpoint from then on, and
// results stay correct across the switch. Measured 2026-09-05 on the world
// shard (48K facts): a one-fact delta took 91 s on the differential path
// where the full fixpoint takes a fraction of that.
func TestEvaluate_DemotesDifferentialPathWhenSlowerThanFull(t *testing.T) {
	t.Setenv("CODENERD_DIFF_EVAL", "1")
	prev := diffDemoteThreshold
	diffDemoteThreshold = 0 // any measurable delta application counts as slow
	t.Cleanup(func() { diffDemoteThreshold = prev })

	k := setupMockKernel(t)
	if k.diffPathDemoted {
		t.Fatal("a fresh kernel must start on the differential path")
	}
	// First evaluate after boot seeds the diff engine; the second applies a
	// delta and, with the threshold at zero, demotes.
	mustAssert(t, k, "test_state", MangleAtom("/failing"))
	if err := k.Evaluate(); err != nil {
		t.Fatal(err)
	}
	mustAssert(t, k, "test_state", MangleAtom("/passing"))
	if err := k.Evaluate(); err != nil {
		t.Fatal(err)
	}
	k.mu.RLock()
	demoted, engine := k.diffPathDemoted, k.diffEngine
	k.mu.RUnlock()
	if !demoted {
		t.Fatal("a slow differential evaluation must demote the kernel to the full path")
	}
	if engine != nil {
		t.Fatal("the differential engine must be dropped on demotion")
	}

	// Correctness across the switch: the retract path and a fresh assert
	// both go through the full fixpoint now.
	if err := k.Retract("test_state"); err != nil {
		t.Fatal(err)
	}
	mustAssert(t, k, "test_state", MangleAtom("/unknown"))
	facts, err := k.Query("test_state")
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 1 || fmt.Sprint(facts[0].Args[0]) != "/unknown" {
		t.Fatalf("test_state after demotion = %+v, want one /unknown fact", facts)
	}
	k.mu.RLock()
	stillDemoted := k.diffPathDemoted
	k.mu.RUnlock()
	if !stillDemoted {
		t.Fatal("demotion must persist for the kernel's lifetime")
	}
}
