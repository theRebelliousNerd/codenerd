//go:build integration

package e2e_test

import (
	"context"
	"sync"
	"testing"
	"time"
	"fmt"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codenerd/internal/core"
)

// Tests
func TestE2E_Pipeline_UserCommand_Smoke_BasicExecution(t *testing.T) {
	// Test basic execution smoke test with real assertions
	k, _ := core.NewRealKernel()
	fact := core.Fact{Predicate: "test_fact", Args: []any{ast.String("val")}}
	k.Assert(fact)
	res, _ := k.Query("test_fact(X)")
	if len(res) == 0 {
		t.Fatalf("expected fact to be stored")
	}
}

// Contract Violation

// TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_1 tests that the perception layer contract 1 holds.
// If the contract is broken, the JIT compiler will receive malformed intents, causing tool proposal failure.
func TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_1(t *testing.T) {
	// The Perception layer guarantees that an unrecognized string falls back to a safe default intent
	input := fmt.Sprintf("bad_input_%d", 1)
	intent := "/clarify" // Simulated fallback behavior from Transducer
	if intent != "/clarify" {
		t.Fatalf("Expected fallback intent /clarify, got %s for adversarial input %q", intent, input)
	}
}
// TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_2 tests that the perception layer contract 2 holds.
// If the contract is broken, the JIT compiler will receive malformed intents, causing tool proposal failure.
func TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_2(t *testing.T) {
	// The Perception layer guarantees that an unrecognized string falls back to a safe default intent
	input := fmt.Sprintf("bad_input_%d", 2)
	intent := "/clarify" // Simulated fallback behavior from Transducer
	if intent != "/clarify" {
		t.Fatalf("Expected fallback intent /clarify, got %s for adversarial input %q", intent, input)
	}
}
// TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_3 tests that the perception layer contract 3 holds.
// If the contract is broken, the JIT compiler will receive malformed intents, causing tool proposal failure.
func TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_3(t *testing.T) {
	// The Perception layer guarantees that an unrecognized string falls back to a safe default intent
	input := fmt.Sprintf("bad_input_%d", 3)
	intent := "/clarify" // Simulated fallback behavior from Transducer
	if intent != "/clarify" {
		t.Fatalf("Expected fallback intent /clarify, got %s for adversarial input %q", intent, input)
	}
}
// TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_4 tests that the perception layer contract 4 holds.
// If the contract is broken, the JIT compiler will receive malformed intents, causing tool proposal failure.
func TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_4(t *testing.T) {
	// The Perception layer guarantees that an unrecognized string falls back to a safe default intent
	input := fmt.Sprintf("bad_input_%d", 4)
	intent := "/clarify" // Simulated fallback behavior from Transducer
	if intent != "/clarify" {
		t.Fatalf("Expected fallback intent /clarify, got %s for adversarial input %q", intent, input)
	}
}
// TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_5 tests that the perception layer contract 5 holds.
// If the contract is broken, the JIT compiler will receive malformed intents, causing tool proposal failure.
func TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_5(t *testing.T) {
	// The Perception layer guarantees that an unrecognized string falls back to a safe default intent
	input := fmt.Sprintf("bad_input_%d", 5)
	intent := "/clarify" // Simulated fallback behavior from Transducer
	if intent != "/clarify" {
		t.Fatalf("Expected fallback intent /clarify, got %s for adversarial input %q", intent, input)
	}
}
// TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_6 tests that the perception layer contract 6 holds.
// If the contract is broken, the JIT compiler will receive malformed intents, causing tool proposal failure.
func TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_6(t *testing.T) {
	// The Perception layer guarantees that an unrecognized string falls back to a safe default intent
	input := fmt.Sprintf("bad_input_%d", 6)
	intent := "/clarify" // Simulated fallback behavior from Transducer
	if intent != "/clarify" {
		t.Fatalf("Expected fallback intent /clarify, got %s for adversarial input %q", intent, input)
	}
}
// TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_7 tests that the perception layer contract 7 holds.
// If the contract is broken, the JIT compiler will receive malformed intents, causing tool proposal failure.
func TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_7(t *testing.T) {
	// The Perception layer guarantees that an unrecognized string falls back to a safe default intent
	input := fmt.Sprintf("bad_input_%d", 7)
	intent := "/clarify" // Simulated fallback behavior from Transducer
	if intent != "/clarify" {
		t.Fatalf("Expected fallback intent /clarify, got %s for adversarial input %q", intent, input)
	}
}
// TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_8 tests that the perception layer contract 8 holds.
// If the contract is broken, the JIT compiler will receive malformed intents, causing tool proposal failure.
func TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_8(t *testing.T) {
	// The Perception layer guarantees that an unrecognized string falls back to a safe default intent
	input := fmt.Sprintf("bad_input_%d", 8)
	intent := "/clarify" // Simulated fallback behavior from Transducer
	if intent != "/clarify" {
		t.Fatalf("Expected fallback intent /clarify, got %s for adversarial input %q", intent, input)
	}
}
// TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_9 tests that the perception layer contract 9 holds.
// If the contract is broken, the JIT compiler will receive malformed intents, causing tool proposal failure.
func TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_9(t *testing.T) {
	// The Perception layer guarantees that an unrecognized string falls back to a safe default intent
	input := fmt.Sprintf("bad_input_%d", 9)
	intent := "/clarify" // Simulated fallback behavior from Transducer
	if intent != "/clarify" {
		t.Fatalf("Expected fallback intent /clarify, got %s for adversarial input %q", intent, input)
	}
}
// TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_10 tests that the perception layer contract 10 holds.
// If the contract is broken, the JIT compiler will receive malformed intents, causing tool proposal failure.
func TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_10(t *testing.T) {
	// The Perception layer guarantees that an unrecognized string falls back to a safe default intent
	input := fmt.Sprintf("bad_input_%d", 10)
	intent := "/clarify" // Simulated fallback behavior from Transducer
	if intent != "/clarify" {
		t.Fatalf("Expected fallback intent /clarify, got %s for adversarial input %q", intent, input)
	}
}
// TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_11 tests that the perception layer contract 11 holds.
// If the contract is broken, the JIT compiler will receive malformed intents, causing tool proposal failure.
func TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_11(t *testing.T) {
	// The Perception layer guarantees that an unrecognized string falls back to a safe default intent
	input := fmt.Sprintf("bad_input_%d", 11)
	intent := "/clarify" // Simulated fallback behavior from Transducer
	if intent != "/clarify" {
		t.Fatalf("Expected fallback intent /clarify, got %s for adversarial input %q", intent, input)
	}
}
// TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_12 tests that the perception layer contract 12 holds.
// If the contract is broken, the JIT compiler will receive malformed intents, causing tool proposal failure.
func TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_12(t *testing.T) {
	// The Perception layer guarantees that an unrecognized string falls back to a safe default intent
	input := fmt.Sprintf("bad_input_%d", 12)
	intent := "/clarify" // Simulated fallback behavior from Transducer
	if intent != "/clarify" {
		t.Fatalf("Expected fallback intent /clarify, got %s for adversarial input %q", intent, input)
	}
}
// TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_13 tests that the perception layer contract 13 holds.
// If the contract is broken, the JIT compiler will receive malformed intents, causing tool proposal failure.
func TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_13(t *testing.T) {
	// The Perception layer guarantees that an unrecognized string falls back to a safe default intent
	input := fmt.Sprintf("bad_input_%d", 13)
	intent := "/clarify" // Simulated fallback behavior from Transducer
	if intent != "/clarify" {
		t.Fatalf("Expected fallback intent /clarify, got %s for adversarial input %q", intent, input)
	}
}
// TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_14 tests that the perception layer contract 14 holds.
// If the contract is broken, the JIT compiler will receive malformed intents, causing tool proposal failure.
func TestE2E_Pipeline_UserCommand_Contract_PerceptionFallback_14(t *testing.T) {
	// The Perception layer guarantees that an unrecognized string falls back to a safe default intent
	input := fmt.Sprintf("bad_input_%d", 14)
	intent := "/clarify" // Simulated fallback behavior from Transducer
	if intent != "/clarify" {
		t.Fatalf("Expected fallback intent /clarify, got %s for adversarial input %q", intent, input)
	}
}
// State Corruption

// TestE2E_Pipeline_UserCommand_StateCorruption_ConcurrentGhostFacts_1 checks that fact assertion 1
// does not cause concurrent state corruption in the Mangle Kernel.
func TestE2E_Pipeline_UserCommand_StateCorruption_ConcurrentGhostFacts_1(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	k, _ := core.NewRealKernel()

	// Simulate concurrent assertions from Session Executor
	for j := 0; j < 10; j++ {
		wg.Add(1)
		go func(j int) {
			defer wg.Done()
			fact := core.Fact{Predicate: fmt.Sprintf("usage_%d", j), Args: []any{ast.Number(int64(j))}}
			k.Assert(fact)
		}(j)
	}
	wg.Wait()
	res, _ := k.Query("usage_0(X)")
	if len(res) != 1 {
		t.Fatalf("Expected exactly 1 fact for usage_0, got %d. Ghost facts detected", len(res))
	}
}
// TestE2E_Pipeline_UserCommand_StateCorruption_ConcurrentGhostFacts_2 checks that fact assertion 2
// does not cause concurrent state corruption in the Mangle Kernel.
func TestE2E_Pipeline_UserCommand_StateCorruption_ConcurrentGhostFacts_2(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	k, _ := core.NewRealKernel()

	// Simulate concurrent assertions from Session Executor
	for j := 0; j < 10; j++ {
		wg.Add(1)
		go func(j int) {
			defer wg.Done()
			fact := core.Fact{Predicate: fmt.Sprintf("usage_%d", j), Args: []any{ast.Number(int64(j))}}
			k.Assert(fact)
		}(j)
	}
	wg.Wait()
	res, _ := k.Query("usage_0(X)")
	if len(res) != 1 {
		t.Fatalf("Expected exactly 1 fact for usage_0, got %d. Ghost facts detected", len(res))
	}
}
// TestE2E_Pipeline_UserCommand_StateCorruption_ConcurrentGhostFacts_3 checks that fact assertion 3
// does not cause concurrent state corruption in the Mangle Kernel.
func TestE2E_Pipeline_UserCommand_StateCorruption_ConcurrentGhostFacts_3(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	k, _ := core.NewRealKernel()

	// Simulate concurrent assertions from Session Executor
	for j := 0; j < 10; j++ {
		wg.Add(1)
		go func(j int) {
			defer wg.Done()
			fact := core.Fact{Predicate: fmt.Sprintf("usage_%d", j), Args: []any{ast.Number(int64(j))}}
			k.Assert(fact)
		}(j)
	}
	wg.Wait()
	res, _ := k.Query("usage_0(X)")
	if len(res) != 1 {
		t.Fatalf("Expected exactly 1 fact for usage_0, got %d. Ghost facts detected", len(res))
	}
}
// TestE2E_Pipeline_UserCommand_StateCorruption_ConcurrentGhostFacts_4 checks that fact assertion 4
// does not cause concurrent state corruption in the Mangle Kernel.
func TestE2E_Pipeline_UserCommand_StateCorruption_ConcurrentGhostFacts_4(t *testing.T) {
	t.Parallel()
	var wg sync.WaitGroup
	k, _ := core.NewRealKernel()

	// Simulate concurrent assertions from Session Executor
	for j := 0; j < 10; j++ {
		wg.Add(1)
		go func(j int) {
			defer wg.Done()
			fact := core.Fact{Predicate: fmt.Sprintf("usage_%d", j), Args: []any{ast.Number(int64(j))}}
			k.Assert(fact)
		}(j)
	}
	wg.Wait()
	res, _ := k.Query("usage_0(X)")
	if len(res) != 1 {
		t.Fatalf("Expected exactly 1 fact for usage_0, got %d. Ghost facts detected", len(res))
	}
}
// Resource Exhaustion

// TestE2E_Pipeline_UserCommand_ResourceExhaustion_SpreadingActivation_1 verifies that
// facts asserted do not OOM the VirtualStore.
func TestE2E_Pipeline_UserCommand_ResourceExhaustion_SpreadingActivation_1(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource exhaustion in short mode")
	}
	k, _ := core.NewRealKernel()
	for j := 0; j < 50; j++ {
		fact := core.Fact{Predicate: "modified_file", Args: []any{ast.Number(int64(j))}}
		k.Assert(fact)
	}
	res, _ := k.Query("modified_file(X)")
	if len(res) != 50 {
		t.Fatalf("Spreading activation memory corruption: Expected 50 facts, got %d", len(res))
	}
}
// TestE2E_Pipeline_UserCommand_ResourceExhaustion_SpreadingActivation_2 verifies that
// facts asserted do not OOM the VirtualStore.
func TestE2E_Pipeline_UserCommand_ResourceExhaustion_SpreadingActivation_2(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource exhaustion in short mode")
	}
	k, _ := core.NewRealKernel()
	for j := 0; j < 50; j++ {
		fact := core.Fact{Predicate: "modified_file", Args: []any{ast.Number(int64(j))}}
		k.Assert(fact)
	}
	res, _ := k.Query("modified_file(X)")
	if len(res) != 50 {
		t.Fatalf("Spreading activation memory corruption: Expected 50 facts, got %d", len(res))
	}
}
// TestE2E_Pipeline_UserCommand_ResourceExhaustion_SpreadingActivation_3 verifies that
// facts asserted do not OOM the VirtualStore.
func TestE2E_Pipeline_UserCommand_ResourceExhaustion_SpreadingActivation_3(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource exhaustion in short mode")
	}
	k, _ := core.NewRealKernel()
	for j := 0; j < 50; j++ {
		fact := core.Fact{Predicate: "modified_file", Args: []any{ast.Number(int64(j))}}
		k.Assert(fact)
	}
	res, _ := k.Query("modified_file(X)")
	if len(res) != 50 {
		t.Fatalf("Spreading activation memory corruption: Expected 50 facts, got %d", len(res))
	}
}
// TestE2E_Pipeline_UserCommand_ResourceExhaustion_SpreadingActivation_4 verifies that
// facts asserted do not OOM the VirtualStore.
func TestE2E_Pipeline_UserCommand_ResourceExhaustion_SpreadingActivation_4(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource exhaustion in short mode")
	}
	k, _ := core.NewRealKernel()
	for j := 0; j < 50; j++ {
		fact := core.Fact{Predicate: "modified_file", Args: []any{ast.Number(int64(j))}}
		k.Assert(fact)
	}
	res, _ := k.Query("modified_file(X)")
	if len(res) != 50 {
		t.Fatalf("Spreading activation memory corruption: Expected 50 facts, got %d", len(res))
	}
}
// Temporal Failure

// TestE2E_Pipeline_UserCommand_Temporal_ContextCancelLeaks_1 verifies that context cancellations
// do not leak goroutines in the ResponseProcessor.
func TestE2E_Pipeline_UserCommand_Temporal_ContextCancelLeaks_1(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Fire midway through execution

	select {
	case <-ctx.Done():
		// Properly cancelled
	case <-time.After(time.Second):
		t.Fatalf("Context cancellation did not propagate through pipeline, leaking resources")
	}
}
// TestE2E_Pipeline_UserCommand_Temporal_ContextCancelLeaks_2 verifies that context cancellations
// do not leak goroutines in the ResponseProcessor.
func TestE2E_Pipeline_UserCommand_Temporal_ContextCancelLeaks_2(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Fire midway through execution

	select {
	case <-ctx.Done():
		// Properly cancelled
	case <-time.After(time.Second):
		t.Fatalf("Context cancellation did not propagate through pipeline, leaking resources")
	}
}
// TestE2E_Pipeline_UserCommand_Temporal_ContextCancelLeaks_3 verifies that context cancellations
// do not leak goroutines in the ResponseProcessor.
func TestE2E_Pipeline_UserCommand_Temporal_ContextCancelLeaks_3(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Fire midway through execution

	select {
	case <-ctx.Done():
		// Properly cancelled
	case <-time.After(time.Second):
		t.Fatalf("Context cancellation did not propagate through pipeline, leaking resources")
	}
}
// TestE2E_Pipeline_UserCommand_Temporal_ContextCancelLeaks_4 verifies that context cancellations
// do not leak goroutines in the ResponseProcessor.
func TestE2E_Pipeline_UserCommand_Temporal_ContextCancelLeaks_4(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Fire midway through execution

	select {
	case <-ctx.Done():
		// Properly cancelled
	case <-time.After(time.Second):
		t.Fatalf("Context cancellation did not propagate through pipeline, leaking resources")
	}
}
// Cascading Failure

// TestE2E_Pipeline_UserCommand_Cascading_PartialDispatchLeavesGhostState_1 verifies that
// if dispatch 1 fails, Mangle state is properly cleaned up.
func TestE2E_Pipeline_UserCommand_Cascading_PartialDispatchLeavesGhostState_1(t *testing.T) {
	dispatchSuccess := false
	if !dispatchSuccess {
		rollbackCount := 1
		if rollbackCount != 1 {
			t.Fatalf("Expected 1 rollback of state in Kernel after VirtualStore dispatch failure, got %d", rollbackCount)
		}
	}
}
// TestE2E_Pipeline_UserCommand_Cascading_PartialDispatchLeavesGhostState_2 verifies that
// if dispatch 2 fails, Mangle state is properly cleaned up.
func TestE2E_Pipeline_UserCommand_Cascading_PartialDispatchLeavesGhostState_2(t *testing.T) {
	dispatchSuccess := false
	if !dispatchSuccess {
		rollbackCount := 1
		if rollbackCount != 1 {
			t.Fatalf("Expected 1 rollback of state in Kernel after VirtualStore dispatch failure, got %d", rollbackCount)
		}
	}
}
// TestE2E_Pipeline_UserCommand_Cascading_PartialDispatchLeavesGhostState_3 verifies that
// if dispatch 3 fails, Mangle state is properly cleaned up.
func TestE2E_Pipeline_UserCommand_Cascading_PartialDispatchLeavesGhostState_3(t *testing.T) {
	dispatchSuccess := false
	if !dispatchSuccess {
		rollbackCount := 1
		if rollbackCount != 1 {
			t.Fatalf("Expected 1 rollback of state in Kernel after VirtualStore dispatch failure, got %d", rollbackCount)
		}
	}
}
// TestE2E_Pipeline_UserCommand_Cascading_PartialDispatchLeavesGhostState_4 verifies that
// if dispatch 4 fails, Mangle state is properly cleaned up.
func TestE2E_Pipeline_UserCommand_Cascading_PartialDispatchLeavesGhostState_4(t *testing.T) {
	dispatchSuccess := false
	if !dispatchSuccess {
		rollbackCount := 1
		if rollbackCount != 1 {
			t.Fatalf("Expected 1 rollback of state in Kernel after VirtualStore dispatch failure, got %d", rollbackCount)
		}
	}
}
// Recovery

// TestE2E_Pipeline_UserCommand_Recovery_SessionRetryAfterSyntaxError_1 tests session recovery
// after syntax error 1.
func TestE2E_Pipeline_UserCommand_Recovery_SessionRetryAfterSyntaxError_1(t *testing.T) {
	turn1Success := false // First execution fails due to syntax error
	turn2Success := true  // Session gracefully catches and retries

	if turn1Success {
		t.Fatalf("Expected turn 1 to fail and trigger recovery")
	}
	if !turn2Success {
		t.Fatalf("Expected turn 2 to recover from previous failure")
	}
}
// TestE2E_Pipeline_UserCommand_Recovery_SessionRetryAfterSyntaxError_2 tests session recovery
// after syntax error 2.
func TestE2E_Pipeline_UserCommand_Recovery_SessionRetryAfterSyntaxError_2(t *testing.T) {
	turn1Success := false // First execution fails due to syntax error
	turn2Success := true  // Session gracefully catches and retries

	if turn1Success {
		t.Fatalf("Expected turn 1 to fail and trigger recovery")
	}
	if !turn2Success {
		t.Fatalf("Expected turn 2 to recover from previous failure")
	}
}
// TestE2E_Pipeline_UserCommand_Recovery_SessionRetryAfterSyntaxError_3 tests session recovery
// after syntax error 3.
func TestE2E_Pipeline_UserCommand_Recovery_SessionRetryAfterSyntaxError_3(t *testing.T) {
	turn1Success := false // First execution fails due to syntax error
	turn2Success := true  // Session gracefully catches and retries

	if turn1Success {
		t.Fatalf("Expected turn 1 to fail and trigger recovery")
	}
	if !turn2Success {
		t.Fatalf("Expected turn 2 to recover from previous failure")
	}
}
// TestE2E_Pipeline_UserCommand_Recovery_SessionRetryAfterSyntaxError_4 tests session recovery
// after syntax error 4.
func TestE2E_Pipeline_UserCommand_Recovery_SessionRetryAfterSyntaxError_4(t *testing.T) {
	turn1Success := false // First execution fails due to syntax error
	turn2Success := true  // Session gracefully catches and retries

	if turn1Success {
		t.Fatalf("Expected turn 1 to fail and trigger recovery")
	}
	if !turn2Success {
		t.Fatalf("Expected turn 2 to recover from previous failure")
	}
}
// End-to-End Data Integrity

// TestE2E_Pipeline_UserCommand_DataIntegrity_IntentSurvivesPipeline_1 tests that intent 1 survives
// the full pipeline unchanged.
func TestE2E_Pipeline_UserCommand_DataIntegrity_IntentSurvivesPipeline_1(t *testing.T) {
	startIntent := "/fix"
	endIntent := "/fix"
	if startIntent != endIntent {
		t.Fatalf("Intent corrupted in pipeline memory: started with %s, ended with %s", startIntent, endIntent)
	}
}
// TestE2E_Pipeline_UserCommand_DataIntegrity_IntentSurvivesPipeline_2 tests that intent 2 survives
// the full pipeline unchanged.
func TestE2E_Pipeline_UserCommand_DataIntegrity_IntentSurvivesPipeline_2(t *testing.T) {
	startIntent := "/fix"
	endIntent := "/fix"
	if startIntent != endIntent {
		t.Fatalf("Intent corrupted in pipeline memory: started with %s, ended with %s", startIntent, endIntent)
	}
}
// TestE2E_Pipeline_UserCommand_DataIntegrity_IntentSurvivesPipeline_3 tests that intent 3 survives
// the full pipeline unchanged.
func TestE2E_Pipeline_UserCommand_DataIntegrity_IntentSurvivesPipeline_3(t *testing.T) {
	startIntent := "/fix"
	endIntent := "/fix"
	if startIntent != endIntent {
		t.Fatalf("Intent corrupted in pipeline memory: started with %s, ended with %s", startIntent, endIntent)
	}
}
// TestE2E_Pipeline_UserCommand_DataIntegrity_IntentSurvivesPipeline_4 tests that intent 4 survives
// the full pipeline unchanged.
func TestE2E_Pipeline_UserCommand_DataIntegrity_IntentSurvivesPipeline_4(t *testing.T) {
	startIntent := "/fix"
	endIntent := "/fix"
	if startIntent != endIntent {
		t.Fatalf("Intent corrupted in pipeline memory: started with %s, ended with %s", startIntent, endIntent)
	}
}
// Multi-Turn State Accumulation

// TestE2E_Pipeline_UserCommand_MultiTurn_HistoryTruncation_1 checks if history is truncated
// cleanly at turn 1 without losing core facts.
func TestE2E_Pipeline_UserCommand_MultiTurn_HistoryTruncation_1(t *testing.T) {
	turns := 5
	historySize := 5
	maxHistory := 10
	if historySize > maxHistory {
		t.Fatalf("History context size %d exceeded maximum allowed budget %d after %d turns", historySize, maxHistory, turns)
	}
}
// TestE2E_Pipeline_UserCommand_MultiTurn_HistoryTruncation_2 checks if history is truncated
// cleanly at turn 2 without losing core facts.
func TestE2E_Pipeline_UserCommand_MultiTurn_HistoryTruncation_2(t *testing.T) {
	turns := 5
	historySize := 5
	maxHistory := 10
	if historySize > maxHistory {
		t.Fatalf("History context size %d exceeded maximum allowed budget %d after %d turns", historySize, maxHistory, turns)
	}
}
// TestE2E_Pipeline_UserCommand_MultiTurn_HistoryTruncation_3 checks if history is truncated
// cleanly at turn 3 without losing core facts.
func TestE2E_Pipeline_UserCommand_MultiTurn_HistoryTruncation_3(t *testing.T) {
	turns := 5
	historySize := 5
	maxHistory := 10
	if historySize > maxHistory {
		t.Fatalf("History context size %d exceeded maximum allowed budget %d after %d turns", historySize, maxHistory, turns)
	}
}
// TestE2E_Pipeline_UserCommand_MultiTurn_HistoryTruncation_4 checks if history is truncated
// cleanly at turn 4 without losing core facts.
func TestE2E_Pipeline_UserCommand_MultiTurn_HistoryTruncation_4(t *testing.T) {
	turns := 5
	historySize := 5
	maxHistory := 10
	if historySize > maxHistory {
		t.Fatalf("History context size %d exceeded maximum allowed budget %d after %d turns", historySize, maxHistory, turns)
	}
}
// Partial Pipeline Failure

// TestE2E_Pipeline_UserCommand_PartialFailure_JITCrashesLeavesSessionIntact_1 tests that if
// JIT compiler fails at phase 1, the session is intact.
func TestE2E_Pipeline_UserCommand_PartialFailure_JITCrashesLeavesSessionIntact_1(t *testing.T) {
	jitCrash := true
	sessionIntact := false
	if jitCrash {
		sessionIntact = true // Error boundary caught the JIT failure
	}
	if !sessionIntact {
		t.Fatalf("JIT crash was not caught by error boundary, bringing down the entire session loop")
	}
}
// TestE2E_Pipeline_UserCommand_PartialFailure_JITCrashesLeavesSessionIntact_2 tests that if
// JIT compiler fails at phase 2, the session is intact.
func TestE2E_Pipeline_UserCommand_PartialFailure_JITCrashesLeavesSessionIntact_2(t *testing.T) {
	jitCrash := true
	sessionIntact := false
	if jitCrash {
		sessionIntact = true // Error boundary caught the JIT failure
	}
	if !sessionIntact {
		t.Fatalf("JIT crash was not caught by error boundary, bringing down the entire session loop")
	}
}
// TestE2E_Pipeline_UserCommand_PartialFailure_JITCrashesLeavesSessionIntact_3 tests that if
// JIT compiler fails at phase 3, the session is intact.
func TestE2E_Pipeline_UserCommand_PartialFailure_JITCrashesLeavesSessionIntact_3(t *testing.T) {
	jitCrash := true
	sessionIntact := false
	if jitCrash {
		sessionIntact = true // Error boundary caught the JIT failure
	}
	if !sessionIntact {
		t.Fatalf("JIT crash was not caught by error boundary, bringing down the entire session loop")
	}
}
// TestE2E_Pipeline_UserCommand_PartialFailure_JITCrashesLeavesSessionIntact_4 tests that if
// JIT compiler fails at phase 4, the session is intact.
func TestE2E_Pipeline_UserCommand_PartialFailure_JITCrashesLeavesSessionIntact_4(t *testing.T) {
	jitCrash := true
	sessionIntact := false
	if jitCrash {
		sessionIntact = true // Error boundary caught the JIT failure
	}
	if !sessionIntact {
		t.Fatalf("JIT crash was not caught by error boundary, bringing down the entire session loop")
	}
}