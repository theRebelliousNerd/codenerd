package core

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"codenerd/internal/types"
)

func TestCortexKernel_InterfaceCast(t *testing.T) {
	cortex := NewCortexKernel("main")
	mainShard := setupTestShard(t, "main", []string{"test_pred"})
	if err := cortex.RegisterShard(mainShard); err != nil {
		t.Fatalf("RegisterShard failed: %v", err)
	}

	// Verify CortexKernel implements KernelTransactor
	var transactor any = cortex
	kt, ok := transactor.(types.KernelTransactor)
	if !ok {
		t.Fatal("CortexKernel does not implement types.KernelTransactor")
	}
	t.Logf("Successfully casted to transactor: %T", kt)

	// Verify NewKernelTx doesn't panic and returns a valid transaction wrapper
	var txWrapper *types.KernelTx
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("NewKernelTx panicked: %v", r)
			}
		}()
		txWrapper = types.NewKernelTx(cortex)
	}()

	if txWrapper == nil {
		t.Fatal("NewKernelTx returned nil wrapper")
	}

	// Verify transaction roundtrip on the wrapper
	txWrapper.Assert(types.Fact{Predicate: "test_pred", Args: []any{"assert_from_wrapper"}})
	if err := txWrapper.Commit(); err != nil {
		t.Fatalf("Commit failed on tx wrapper: %v", err)
	}

	facts, err := mainShard.Query("test_pred")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(facts) != 1 || facts[0].Args[0].(string) != "assert_from_wrapper" {
		t.Errorf("Expected fact asserted from wrapper, got %v", facts)
	}
}

func TestCortexTransaction_RetractExactFact(t *testing.T) {
	cortex := NewCortexKernel("main")
	shardA := setupTestShard(t, "shardA", []string{"my_exact_pred"})
	shardB := setupTestShard(t, "shardB", []string{"other_exact_pred"})

	if err := cortex.RegisterShard(shardA); err != nil {
		t.Fatalf("RegisterShard A failed: %v", err)
	}
	if err := cortex.RegisterShard(shardB); err != nil {
		t.Fatalf("RegisterShard B failed: %v", err)
	}

	// 1. Assert initial facts to Shard A
	cortex.Assert(types.Fact{Predicate: "my_exact_pred", Args: []any{"exact_1"}})
	cortex.Assert(types.Fact{Predicate: "my_exact_pred", Args: []any{"exact_2"}})
	cortex.Assert(types.Fact{Predicate: "my_exact_pred", Args: []any{"exact_3"}})

	// 2. Retract exact fact within a transaction
	tx := cortex.Transaction()
	tx.RetractExactFact(types.Fact{Predicate: "my_exact_pred", Args: []any{"exact_2"}})
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// 3. Verify exact retraction leaving other arguments unharmed
	results, err := shardA.Query("my_exact_pred")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Expected 2 facts remaining, got %d", len(results))
	}

	found1 := false
	found3 := false
	for _, res := range results {
		arg0 := res.Args[0].(string)
		if arg0 == "exact_1" {
			found1 = true
		} else if arg0 == "exact_3" {
			found3 = true
		} else if arg0 == "exact_2" {
			t.Errorf("Fact 'exact_2' was not retracted")
		}
	}
	if !found1 || !found3 {
		t.Errorf("Expected 'exact_1' and 'exact_3' remaining, got results: %v", results)
	}
}

func TestCortexTransaction_RetractPredicateSet(t *testing.T) {
	cortex := NewCortexKernel("main")
	shardA := setupTestShard(t, "shardA", []string{"pred_a1", "pred_a2"})
	shardB := setupTestShard(t, "shardB", []string{"pred_b1"})

	cortex.RegisterShard(shardA)
	cortex.RegisterShard(shardB)

	// Assert facts to both shards
	cortex.Assert(types.Fact{Predicate: "pred_a1", Args: []any{"val_a1"}})
	cortex.Assert(types.Fact{Predicate: "pred_a2", Args: []any{"val_a2"}})
	cortex.Assert(types.Fact{Predicate: "pred_b1", Args: []any{"val_b1"}})

	// Retract a predicate set containing predicates from both shards
	tx := cortex.Transaction()
	predSet := map[string]struct{}{
		"pred_a1": {},
		"pred_b1": {},
	}
	tx.RetractPredicateSet(predSet)
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify only targeted predicates were retracted from their respective shards
	factsA1, _ := shardA.Query("pred_a1")
	if len(factsA1) != 0 {
		t.Errorf("Expected pred_a1 to be retracted, got %v", factsA1)
	}

	factsA2, _ := shardA.Query("pred_a2")
	if len(factsA2) != 1 {
		t.Errorf("Expected pred_a2 to remain unchanged, got %v", factsA2)
	}

	factsB1, _ := shardB.Query("pred_b1")
	if len(factsB1) != 0 {
		t.Errorf("Expected pred_b1 to be retracted, got %v", factsB1)
	}
}

func TestCortexTransaction_ConcurrentCommit(t *testing.T) {
	cortex := NewCortexKernel("main")
	shardA := setupTestShard(t, "main", []string{"concurrent_tx_pred"})
	cortex.RegisterShard(shardA)

	var wg sync.WaitGroup
	const goroutines = 20
	const opsPerGoroutine = 30

	errs := make(chan error, goroutines)

	for i := range goroutines {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			for j := range opsPerGoroutine {
				tx := cortex.Transaction()
				tx.Assert(types.Fact{
					Predicate: "concurrent_tx_pred",
					Args:      []any{fmt.Sprintf("g_%d_op_%d", gID, j)},
				})
				if err := tx.Commit(); err != nil {
					errs <- err
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("Concurrent transaction commit failed: %v", err)
	}

	// Verify total fact count matches the expected sum
	results, err := shardA.Query("concurrent_tx_pred")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	expected := goroutines * opsPerGoroutine
	if len(results) != expected {
		t.Errorf("Expected %d facts, got %d", expected, len(results))
	}
}

func TestCortexTransaction_ShardCommitFailure(t *testing.T) {
	cortex := NewCortexKernel("main")

	// Create a dummy shard with a broken kernel
	fakeShard, err := NewKernelShard(KernelShardConfig{
		Domain:          "broken",
		OwnedPredicates: []string{"bad_pred"},
	})
	if err != nil {
		t.Fatalf("Failed to create fake shard: %v", err)
	}

	expectedErr := errors.New("simulated database rollback / write failure")
	fakeShard.kernel.simulateCommitErr = expectedErr

	if err := cortex.RegisterShard(fakeShard); err != nil {
		t.Fatalf("RegisterShard failed: %v", err)
	}

	tx := cortex.Transaction()
	tx.Assert(types.Fact{Predicate: "bad_pred", Args: []any{"fail"}})

	err = tx.Commit()
	if err == nil {
		t.Fatal("Expected transaction commit to fail, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error wrapping '%v', got '%v'", expectedErr, err)
	}
}
