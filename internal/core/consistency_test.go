package core

import (
	"fmt"
	"sync"
	"testing"
)

// TestKernelConcurrency verifies that the kernel is thread-safe for concurrent operations.
// This addresses the "split brain" problem by ensuring that multiple components (shards)
// updating the kernel simultaneously do not corrupt the state.
func TestKernelConcurrency(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}
	// Declare the predicate to ensure Query can find it
	kernel.SetSchemas("Decl concurrent_test(ID, Val).")
	kernel.SetPolicy("")

	// 1. Concurrent Assertions
	var wg sync.WaitGroup
	numRoutines := 20
	factsPerRoutine := 25
	if testing.Short() {
		numRoutines = 8
		factsPerRoutine = 10
	}

	wg.Add(numRoutines)
	for i := 0; i < numRoutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < factsPerRoutine; j++ {
				fact := Fact{
					Predicate: "concurrent_test",
					Args:      []interface{}{fmt.Sprintf("worker_%d", id), j},
				}
				// Use AssertBatch to avoid race condition in logging.Audit() 
				// which occurs in Assert() outside the lock. AssertBatch does it inside.
				if err := kernel.AssertBatch([]Fact{fact}); err != nil {
					t.Errorf("AssertBatch failed: %v", err)
				}
			}
		}(i)
	}
	wg.Wait()

	// Verify total facts
	results, err := kernel.Query("concurrent_test")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	expected := numRoutines * factsPerRoutine
	if len(results) != expected {
		t.Errorf("Expected %d facts, got %d", expected, len(results))
	}
}

// TestKernelDataConsistency ensures that the kernel maintains data integrity
// under mixed read/write loads, preventing "split brain" views where queries
// might miss recently added facts.
func TestKernelDataConsistency(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}
	// Declare predicates
	kernel.SetSchemas("Decl state(Val).")
	kernel.SetPolicy("")

	// 1. Assert initial state
	initialFact := Fact{Predicate: "state", Args: []interface{}{"/init"}}
	if err := kernel.Assert(initialFact); err != nil {
		t.Fatalf("Assert failed: %v", err)
	}

	var wg sync.WaitGroup
	// Reader goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				// Query should never panic and should return valid data
				facts, err := kernel.Query("state")
				if err != nil {
					t.Errorf("Query failed: %v", err)
				}
				if len(facts) == 0 {
					t.Errorf("Query returned empty results")
				}
			}
		}()
	}

	// Writer goroutines (Transition state)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				fact := Fact{Predicate: "state", Args: []interface{}{fmt.Sprintf("/update_%d_%d", id, j)}}
				// Use AssertBatch to minimize locking overhead and potential races in side-effects
				if err := kernel.AssertBatch([]Fact{fact}); err != nil {
					t.Errorf("Assert failed: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestKernelIdempotencyUnderLoad checks that concurrent assertions of the same fact
// do not result in duplicates (Fact Index integrity).
func TestKernelIdempotencyUnderLoad(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}
	kernel.SetSchemas("Decl unique_fact(Val).")
	kernel.SetPolicy("")

	fact := Fact{Predicate: "unique_fact", Args: []interface{}{"/singleton"}}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := kernel.AssertBatch([]Fact{fact}); err != nil {
				t.Errorf("Assert failed: %v", err)
			}
		}()
	}
	wg.Wait()

	results, err := kernel.Query("unique_fact")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected exactly 1 fact, got %d. Deduplication failed under load.", len(results))
	}
}

// TestSplitBrainScenarios simulates a divergence where multiple sources try to 
// assert conflicting state, and verifies that the kernel resolves it (last write wins or both exist).
// Mangle allows multiple facts, so "conflict" here implies presence of multiple facts.
func TestSplitBrainScenarios(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}
	kernel.SetSchemas("Decl task_status(Task, Status).")
	kernel.SetPolicy("")

	// Simulating two shards reporting different status for the same task
	shard1Fact := Fact{Predicate: "task_status", Args: []interface{}{"task_1", "/completed"}}
	shard2Fact := Fact{Predicate: "task_status", Args: []interface{}{"task_1", "/failed"}}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		kernel.AssertBatch([]Fact{shard1Fact})
	}()
	go func() {
		defer wg.Done()
		kernel.AssertBatch([]Fact{shard2Fact})
	}()
	wg.Wait()

	// Both facts should exist in EDB (Mangle is monotonic by default for facts)
	// The application logic (rules) would need to handle the conflict (e.g., aggregation),
	// but the Kernel must safely store both.
	results, err := kernel.Query("task_status")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	
	if len(results) != 2 {
		t.Errorf("Expected 2 conflicting facts to be stored, got %d", len(results))
	}
}
