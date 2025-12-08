// MANDATE FOR AI CODING TOOLS:
// See internal/mangle/differential.go for PRD header.
// This is a test file for DifferentialEngine.

package mangle

import (
	"testing"
)

// TestDifferentialEngineBasic validates basic creation and fact addition.
func TestDifferentialEngineBasic(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoEval = true

	baseEngine, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("Failed to create base engine: %v", err)
	}

	// Load minimal schema
	schema := "Decl test_fact(Name)."
	if err := baseEngine.LoadSchemaString(schema); err != nil {
		t.Fatalf("Failed to load schema: %v", err)
	}

	diffEngine, err := NewDifferentialEngine(baseEngine)
	if err != nil {
		t.Fatalf("Failed to create differential engine: %v", err)
	}

	err = diffEngine.AddFactIncremental(Fact{Predicate: "test_fact", Args: []interface{}{"/foo"}})
	if err != nil {
		t.Fatalf("AddFactIncremental failed: %v", err)
	}

	// Verify fact is in the store (stratum 0)
	// Access via baseEngine for now since we haven't exposed a generic Query on DiffEngine
	// Wait, DiffEngine puts facts in its OWN strataStores, not baseEngine (except for evaluation?).
	// In my implementation:
	// ApplyDelta adds to baseGraph.store.
	// baseGraph.store is a SimpleInMemoryStore.
	// We need to verify it's there.

	// Since DiffEngine wraps everything, we might need a GetFact on it.
	// But baseEngine query won't see DiffEngine's store unless we updated baseEngine.store?
	// In implementation: `mengine.EvalProgramWithStats(de.programInfo, baseGraph.store)`
	// So facts are in `baseGraph.store`.
	// We didn't sync to `baseEngine.store`.
	// Correct: Differential Engine maintains its own state.

	// How to verify?
	// Access store directly for test
	if diffEngine.strataStores[0].store.EstimateFactCount() != 1 {
		t.Errorf("Expected 1 fact, got %d", diffEngine.strataStores[0].store.EstimateFactCount())
	}
}

// TestSnapshotIsolation validates COW Snapshot.
func TestSnapshotIsolation(t *testing.T) {
	cfg := DefaultConfig()
	baseEngine, _ := NewEngine(cfg, nil)
	baseEngine.LoadSchemaString("Decl item(Name).")

	diffEngine, _ := NewDifferentialEngine(baseEngine)
	diffEngine.AddFactIncremental(Fact{Predicate: "item", Args: []interface{}{"A"}})

	snapshot := diffEngine.Snapshot()
	snapshot.AddFactIncremental(Fact{Predicate: "item", Args: []interface{}{"B"}})

	// Verify Main Engine has 1 fact (A)
	if diffEngine.strataStores[0].store.EstimateFactCount() != 1 {
		t.Errorf("Main engine impacted by snapshot! Count: %d", diffEngine.strataStores[0].store.EstimateFactCount())
	}

	// Verify Snapshot has 2 facts (A, B)
	if snapshot.strataStores[0].store.EstimateFactCount() != 2 {
		t.Errorf("Snapshot missing fact! Count: %d", snapshot.strataStores[0].store.EstimateFactCount())
	}
}
