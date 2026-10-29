package campaign

import (
	"context"
	"slices"
	"testing"
)

func TestRecurseConfig_Normalize(t *testing.T) {
	if _, err := (RecurseConfig{MaxWaves: 3, ContextBudget: 1000}).Normalize(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	for name, cfg := range map[string]RecurseConfig{
		"negative passes": {MaxWaves: -1},
		"negative budget": {ContextBudget: -1},
	} {
		if _, err := cfg.Normalize(); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

// Narrowing to a subsystem keeps what it depends on, in order, and nothing
// that depends on it.
func TestRecurseSweepOrder_NarrowsToSubsystemsAndTheirDependencies(t *testing.T) {
	useFixtureDAG(t)
	all, err := RecurseSweepOrder(context.Background(), t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(fixtureDAG()) {
		t.Fatalf("unnarrowed order has %d nodes, want %d", len(all), len(fixtureDAG()))
	}
	narrowed, err := RecurseSweepOrder(context.Background(), t.TempDir(), []string{"session"})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, n := range narrowed {
		ids = append(ids, n.ID)
	}
	if !slices.Contains(ids, "session") || slices.Contains(ids, "cli") || ids[len(ids)-1] != "session" {
		t.Fatalf("narrowed to session: %v", ids)
	}
	if _, err := RecurseSweepOrder(context.Background(), t.TempDir(), []string{"no-such-node"}); err == nil {
		t.Fatal("an unknown subsystem must fail, not sweep nothing")
	}
}
