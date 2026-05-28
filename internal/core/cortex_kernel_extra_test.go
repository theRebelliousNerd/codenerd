package core

import (
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/types"
)

func TestCortexKernel_GetPrimaryRealKernel(t *testing.T) {
	cortex := NewCortexKernel("main")

	// 1. Initial state: no shards registered, should return nil
	if k := cortex.GetPrimaryRealKernel(); k != nil {
		t.Errorf("expected nil GetPrimaryRealKernel, got %v", k)
	}

	// 2. Register non-cortex shard, should fallback to first shard
	shard1 := setupTestShard(t, "other", []string{"other_pred"})
	if err := cortex.RegisterShard(shard1); err != nil {
		t.Fatalf("RegisterShard failed: %v", err)
	}

	if k := cortex.GetPrimaryRealKernel(); k == nil {
		t.Error("expected non-nil fallback kernel from GetPrimaryRealKernel")
	} else if k != shard1.kernel {
		t.Error("expected fallback to shard1's kernel")
	}

	// 3. Register cortex shard, should return cortex shard's kernel
	mainShard := setupTestShard(t, "main", []string{"main_pred"})
	if err := cortex.RegisterShard(mainShard); err != nil {
		t.Fatalf("RegisterShard failed: %v", err)
	}

	if k := cortex.GetPrimaryRealKernel(); k != mainShard.kernel {
		t.Errorf("expected cortex domain kernel, got %v", k)
	}
}

func TestCortexKernel_GetEventBus(t *testing.T) {
	cortex := NewCortexKernel("main")
	bus := cortex.GetEventBus()
	if bus == nil {
		t.Fatal("expected non-nil event bus")
	}
}

func TestCortexKernel_RetractFact(t *testing.T) {
	cortex := NewCortexKernel("main")
	shard := setupTestShard(t, "main", []string{"pred"})
	cortex.RegisterShard(shard)

	fact := types.Fact{Predicate: "pred", Args: []any{"val"}}
	if err := cortex.Assert(fact); err != nil {
		t.Fatalf("Assert failed: %v", err)
	}

	// Verify asserted
	res, err := cortex.Query("pred")
	if err != nil || len(res) != 1 {
		t.Fatalf("Query failed or returned wrong length: %v, %v", err, res)
	}

	// Retract fact
	if err := cortex.RetractFact(fact); err != nil {
		t.Fatalf("RetractFact failed: %v", err)
	}

	// Verify retracted
	res, err = cortex.Query("pred")
	if err != nil || len(res) != 0 {
		t.Errorf("expected 0 results, got %d, err: %v", len(res), err)
	}
}

func TestCortexKernel_RetractExactFactsBatch(t *testing.T) {
	cortex := NewCortexKernel("main")
	mainShard := setupTestShard(t, "main", []string{"main_pred"})
	otherShard := setupTestShard(t, "other", []string{"other_pred"})
	cortex.RegisterShard(mainShard)
	cortex.RegisterShard(otherShard)

	f1 := types.Fact{Predicate: "main_pred", Args: []any{"a"}}
	f2 := types.Fact{Predicate: "other_pred", Args: []any{"b"}}

	if err := cortex.Assert(f1); err != nil {
		t.Fatalf("Assert f1 failed: %v", err)
	}
	if err := cortex.Assert(f2); err != nil {
		t.Fatalf("Assert f2 failed: %v", err)
	}

	// Retract batch
	err := cortex.RetractExactFactsBatch([]types.Fact{f1, f2})
	if err != nil {
		t.Fatalf("RetractExactFactsBatch failed: %v", err)
	}

	// Verify retraction
	res1, _ := cortex.Query("main_pred")
	if len(res1) != 0 {
		t.Errorf("expected main_pred to be empty, got %d", len(res1))
	}
	res2, _ := cortex.Query("other_pred")
	if len(res2) != 0 {
		t.Errorf("expected other_pred to be empty, got %d", len(res2))
	}
}

func TestCortexKernel_RemoveFactsByPredicateSet(t *testing.T) {
	cortex := NewCortexKernel("main")
	mainShard := setupTestShard(t, "main", []string{"p1", "p2"})
	cortex.RegisterShard(mainShard)

	cortex.Assert(types.Fact{Predicate: "p1", Args: []any{"a"}})
	cortex.Assert(types.Fact{Predicate: "p2", Args: []any{"b"}})

	predSet := map[string]struct{}{
		"p1": {},
		"p2": {},
	}

	if err := cortex.RemoveFactsByPredicateSet(predSet); err != nil {
		t.Fatalf("RemoveFactsByPredicateSet failed: %v", err)
	}

	res1, _ := cortex.Query("p1")
	res2, _ := cortex.Query("p2")
	if len(res1) != 0 || len(res2) != 0 {
		t.Errorf("expected p1 and p2 to be empty, got %d and %d", len(res1), len(res2))
	}
}

func TestCortexKernel_QueryAll(t *testing.T) {
	cortex := NewCortexKernel("main")
	mainShard := setupTestShard(t, "main", []string{"main_pred"})
	otherShard := setupTestShard(t, "other", []string{"other_pred"})
	cortex.RegisterShard(mainShard)
	cortex.RegisterShard(otherShard)

	cortex.Assert(types.Fact{Predicate: "main_pred", Args: []any{"1"}})
	cortex.Assert(types.Fact{Predicate: "other_pred", Args: []any{"2"}})

	all, err := cortex.QueryAll()
	if err != nil {
		t.Fatalf("QueryAll failed: %v", err)
	}

	if len(all["main_pred"]) != 1 || len(all["other_pred"]) != 1 {
		t.Errorf("QueryAll returned wrong facts: %v", all)
	}
}

func TestCortexKernel_LoadFacts(t *testing.T) {
	cortex := NewCortexKernel("main")
	mainShard := setupTestShard(t, "main", []string{"p1"})
	cortex.RegisterShard(mainShard)

	facts := []types.Fact{
		{Predicate: "p1", Args: []any{"val"}},
	}

	if err := cortex.LoadFacts(facts); err != nil {
		t.Fatalf("LoadFacts failed: %v", err)
	}

	res, _ := cortex.Query("p1")
	if len(res) != 1 {
		t.Errorf("expected p1 to have 1 fact, got %d", len(res))
	}
}

func TestCortexKernel_UpdateSystemFacts(t *testing.T) {
	cortex := NewCortexKernel("main")
	mainShard := setupTestShard(t, "main", []string{"p1"})
	cortex.RegisterShard(mainShard)

	if err := cortex.UpdateSystemFacts(); err != nil {
		t.Errorf("UpdateSystemFacts failed: %v", err)
	}
}

func TestCortexKernel_GetProgramInfo(t *testing.T) {
	cortex := NewCortexKernel("main")

	// No cortex domain shard registered
	if info := cortex.GetProgramInfo(); info != nil {
		t.Errorf("expected nil ProgramInfo, got %v", info)
	}

	// Register cortex domain shard
	mainShard := setupTestShard(t, "main", []string{"main_pred"})
	cortex.RegisterShard(mainShard)

	if info := cortex.GetProgramInfo(); info == nil {
		t.Error("expected non-nil ProgramInfo after registering main shard")
	}
}

func TestCortexKernel_Reset(t *testing.T) {
	cortex := NewCortexKernel("main")
	mainShard := setupTestShard(t, "main", []string{"p1"})
	cortex.RegisterShard(mainShard)

	cortex.Assert(types.Fact{Predicate: "p1", Args: []any{"val"}})
	cortex.Reset()

	res, _ := cortex.Query("p1")
	if len(res) != 0 {
		t.Errorf("expected empty after Reset, got %d", len(res))
	}
}

func TestCortexKernel_AppendPolicy(t *testing.T) {
	cortex := NewCortexKernel("main")
	mainShard := setupTestShard(t, "main", []string{"p1"})
	cortex.RegisterShard(mainShard)

	// Append policy to main shard through cortex
	cortex.AppendPolicy("Decl appended_pred(X).")
	// Verify it evaluates without error
	if err := cortex.Evaluate(); err != nil {
		t.Errorf("Evaluate failed after AppendPolicy: %v", err)
	}
}

func TestCortexKernel_LogMetrics(t *testing.T) {
	cortex := NewCortexKernel("main")
	mainShard := setupTestShard(t, "main", []string{"p1"})
	cortex.RegisterShard(mainShard)

	// Direct execution should not panic
	cortex.LogMetrics()
}

func TestCortexKernel_EvaluateAll(t *testing.T) {
	cortex := NewCortexKernel("main")
	mainShard := setupTestShard(t, "main", []string{"p1"})
	cortex.RegisterShard(mainShard)

	cortex.Assert(types.Fact{Predicate: "p1", Args: []any{"val"}})
	duration, err := cortex.EvaluateAll()
	if err != nil {
		t.Fatalf("EvaluateAll failed: %v", err)
	}
	if duration < 0 {
		t.Errorf("expected non-negative duration, got %v", duration)
	}
}

func TestCortexKernel_LoadFactsFromFile(t *testing.T) {
	cortex := NewCortexKernel("main")
	mainShard := setupTestShard(t, "main", []string{"custom_intent"})
	cortex.RegisterShard(mainShard)

	// Create temp file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_facts.mg")

	// custom_intent must have arity 1 as declared by setupTestShard
	content := `custom_intent(/query).`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	err := cortex.LoadFactsFromFile(filePath)
	t.Logf("LoadFactsFromFile err: %v", err)
	t.Logf("mainShard domain: %s", mainShard.Domain())
	mainFacts, _ := mainShard.kernel.QueryAll()
	t.Logf("mainShard facts: %+v", mainFacts)

	res, err := cortex.Query("custom_intent")
	if err != nil || len(res) != 1 {
		all, _ := cortex.QueryAll()
		t.Fatalf("Query failed or returned wrong length: %v, %v. QueryAll output: %v", err, res, all)
	}
}

func TestCortexKernel_ConsumeBootPrompts(t *testing.T) {
	cortex := NewCortexKernel("main")
	mainShard := setupTestShard(t, "main", []string{"p1"})
	cortex.RegisterShard(mainShard)

	// nil is a valid representation of empty prompts slice, but we can check if it returns
	prompts := cortex.ConsumeBootPrompts()
	if prompts != nil && len(prompts) != 0 {
		t.Errorf("expected empty prompts, got %v", prompts)
	}
}

func TestCortexTransaction_RetractFact(t *testing.T) {
	cortex := NewCortexKernel("main")
	mainShard := setupTestShard(t, "main", []string{"p1"})
	cortex.RegisterShard(mainShard)

	// p1 must have arity 1 as declared by setupTestShard
	f1 := types.Fact{Predicate: "p1", Args: []any{"a"}}
	f2 := types.Fact{Predicate: "p1", Args: []any{"b"}}

	cortex.Assert(f1)
	cortex.Assert(f2)

	tx := cortex.Transaction()
	tx.RetractFact(f1)

	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	res, _ := cortex.Query("p1")
	// Only f2 should remain
	if len(res) != 1 {
		t.Errorf("expected 1 remaining fact, got %d: %v", len(res), res)
	}
}
