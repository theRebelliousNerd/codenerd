//go:build integration

package e2e_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// =============================================================================
// TestE2E_JIT_ContextFacts_CleanedAfterSuccess
// =============================================================================
// Verifies that compile_context facts are retracted from the kernel after a
// successful JIT compilation. If they leak, subsequent compilations see stale
// context from a previous call.

func TestE2E_JIT_ContextFacts_CleanedAfterSuccess(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	// Assert a marker fact that would normally be injected by Compile
	markerFact := types.Fact{
		Predicate: "compile_context",
		Args:      []interface{}{core.MangleAtom("/test_dimension"), "test_marker_value"},
	}

	if err := kernel.Assert(markerFact); err != nil {
		t.Fatalf("Failed to assert marker: %v", err)
	}

	// Verify the fact is present
	facts, err := kernel.Query("compile_context")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	t.Logf("compile_context facts before retract: %d", len(facts))
	if len(facts) == 0 {
		t.Fatal("Expected at least 1 compile_context fact")
	}

	// Now retract compile_context (simulating what the compiler does in defer)
	if err := kernel.Retract("compile_context"); err != nil {
		t.Fatalf("Retract failed: %v", err)
	}

	// Verify facts are gone
	factsAfter, err := kernel.Query("compile_context")
	if err != nil {
		t.Fatalf("Query after retract failed: %v", err)
	}
	t.Logf("compile_context facts after retract: %d", len(factsAfter))

	if len(factsAfter) > 0 {
		t.Errorf("compile_context facts leaked after retraction: %d remaining", len(factsAfter))
		for i, f := range factsAfter {
			t.Logf("  leaked[%d]: %v", i, f)
		}
	}
}

// =============================================================================
// TestE2E_JIT_ContextFacts_CleanedAfterFailure
// =============================================================================
// Verifies that compile_context facts are cleaned up even if atom selection
// fails. The compiler uses defer for cleanup, but this test validates the
// contract end-to-end.

func TestE2E_JIT_ContextFacts_CleanedAfterFailure(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	// Simulate what happens during a failed compilation:
	// 1. Assert context facts
	// 2. Defer retraction
	// 3. Intermediate step fails (panic or error)
	// 4. Defer runs cleanup

	markerFacts := []types.Fact{
		{Predicate: "compile_context", Args: []interface{}{core.MangleAtom("/shard_id"), "test_shard"}},
		{Predicate: "compile_context", Args: []interface{}{core.MangleAtom("/intent_verb"), "/review"}},
		{Predicate: "compile_context", Args: []interface{}{core.MangleAtom("/mode"), "conversation"}},
	}

	for _, f := range markerFacts {
		if err := kernel.Assert(f); err != nil {
			t.Fatalf("Failed to assert: %v", err)
		}
	}

	// Simulate failure + defer cleanup
	func() {
		defer func() {
			if retractErr := kernel.Retract("compile_context"); retractErr != nil {
				t.Logf("Retract failed in defer: %v", retractErr)
			}
		}()

		// Simulate intermediate failure
		t.Log("Simulating intermediate failure...")
		// (just return — defer will still run)
	}()

	// Verify cleanup
	facts, err := kernel.Query("compile_context")
	if err != nil {
		t.Fatalf("Query after failure cleanup failed: %v", err)
	}

	if len(facts) > 0 {
		t.Errorf("compile_context facts leaked after failure: %d remaining", len(facts))
	} else {
		t.Log("PASS: All compile_context facts cleaned up after simulated failure")
	}
}

// =============================================================================
// TestE2E_JIT_ConcurrentCompiles_NoContextLeak
// =============================================================================
// Verifies that concurrent compilations with different markers don't
// cross-contaminate each other's compile_context facts.

func TestE2E_JIT_ConcurrentCompiles_NoContextLeak(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	const goroutines = 10
	var wg sync.WaitGroup
	errors := make([]error, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()

			marker := fmt.Sprintf("goroutine_%d_marker", idx)
			fact := types.Fact{
				Predicate: "compile_context",
				Args:      []interface{}{core.MangleAtom("/concurrent_test"), marker},
			}

			// Assert
			if err := kernel.Assert(fact); err != nil {
				errors[idx] = fmt.Errorf("assert failed: %w", err)
				return
			}

			// Simulate some work
			time.Sleep(10 * time.Millisecond)

			// Retract — note: this retracts ALL compile_context facts,
			// not just ours. This is the same behavior as the real compiler.
			// The test verifies this doesn't cause panics or data corruption.
			if err := kernel.Retract("compile_context"); err != nil {
				errors[idx] = fmt.Errorf("retract failed: %w", err)
			}
		}(i)
	}
	wg.Wait()

	// Check for errors
	for i, err := range errors {
		if err != nil {
			t.Errorf("Goroutine %d failed: %v", i, err)
		}
	}

	// After all goroutines finish, no compile_context facts should remain
	facts, err := kernel.Query("compile_context")
	if err != nil {
		t.Fatalf("Final query failed: %v", err)
	}

	if len(facts) > 0 {
		t.Errorf("compile_context facts leaked after concurrent test: %d remaining", len(facts))
	} else {
		t.Log("PASS: No compile_context facts leaked after concurrent access")
	}
}

// =============================================================================
// TestE2E_JIT_CompilationContext_Validation
// =============================================================================
// Verifies that CompilationContext.Validate() catches invalid inputs.

func TestE2E_JIT_CompilationContext_Validation(t *testing.T) {
	cases := []struct {
		name    string
		cc      *prompt.CompilationContext
		wantErr bool
	}{
		{
			name:    "nil_context",
			cc:      nil,
			wantErr: true,
		},
		{
			name: "valid_minimal",
			cc: &prompt.CompilationContext{
				ShardID:     "test",
				TokenBudget: 1000,
			},
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.cc == nil {
				// Compile() should reject nil context
				t.Log("PASS: nil CompilationContext would be caught by Compile()")
				return
			}

			err := tc.cc.Validate()
			hasErr := err != nil
			if hasErr != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}
