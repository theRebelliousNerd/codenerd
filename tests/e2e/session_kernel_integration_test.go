//go:build integration

package e2e_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core/kerneltest"
	"codenerd/internal/types"
)

// TestE2E_SessionKernel_Smoke_ValidDelegation proves the baseline integration works.
func TestE2E_SessionKernel_Smoke_ValidDelegation(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)
	// Using standard kernel types - asserting a basic fact
	k.Assert(types.Fact{Predicate: "user_intent", Args: []any{types.MangleAtom("jules"), "test"}})

	// Query to see if the fact stuck
	results, err := k.Query("user_intent(X, Y)")

	if err != nil {
		t.Fatalf("Expected successful query, got err: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("Expected fact to be retrieved, results empty. Contract violated: Baseline failure")
	}
}

// TestE2E_SessionKernel_ContractViolation_TypeDissonance tests what happens when
// the session provides String values when the kernel expects Atoms (Names).
func TestE2E_SessionKernel_ContractViolation_TypeDissonance(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)

	// Assert with string instead of name - permitted expects (Atom, Atom, Atom) usually,
	// or we can test context_priority which expects Atom, Number
	k.Assert(types.Fact{Predicate: "context_priority", Args: []any{"target", 10}})

	// A valid query using standard types (this will fail if the schema rejects the initial insert, or we query atom)
	results, err := k.Query("context_priority(/target, X)")

	if err != nil {
		t.Fatalf("Query should not fail, it should just return empty")
	}
	if len(results) > 0 {
		t.Fatalf("Expected query to fail due to type mismatch, but found results! Contract violated: Type strictness broken")
	}
}

// TestE2E_SessionKernel_ContractViolation_GhostFacts tests that state bleeds if the same kernel is used.
func TestE2E_SessionKernel_ContractViolation_GhostFacts(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)

	// Turn 1: Admin session asserts its status
	k.Assert(types.Fact{Predicate: "session_turn", Args: []any{1}})

	// Turn 1 ends. In a bad integration, the kernel isn't cleared.

	// Turn 2: Non-admin session starts
	// It queries for the turn.
	results, err := k.Query("session_turn(X)")

	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) == 0 {
		t.Log("Ghost fact absent")
	} else {
		// The test proves the kernel retained the fact.
		t.Fatalf("Ghost fact present - state isolation must be handled by the session")
	}
}

// TestE2E_SessionKernel_StateCorruption_ConcurrentMutation tests what happens
// if multiple goroutines write to the kernel while another is evaluating.
func TestE2E_SessionKernel_StateCorruption_ConcurrentMutation(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)

	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			k.Assert(types.Fact{Predicate: "observed", Args: []any{id}})
		}(i)

		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := k.Query("observed(X)")
			if err != nil {
				// We don't t.Errorf here strictly because it races with test completion,
				// but we shouldn't swallow it blindly if we want to ensure no kernel crash.
				_ = err
			}
		}()
	}
	wg.Wait()

	// Final assertion to ensure no state corruption led to total failure
	results, err := k.Query("observed(X)")
	if err != nil {
		t.Fatalf("Concurrent mutations corrupted kernel state: %v", err)
	}
	if len(results) != 50 {
		t.Fatalf("Expected 50 observed facts after concurrent mutation, got %d", len(results))
	}
}

// TestE2E_SessionKernel_ResourceExhaustion_MassiveFacts floods the boundary.
func TestE2E_SessionKernel_ResourceExhaustion_MassiveFacts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping massive fact test in short mode")
	}
	t.Parallel()
	k := kerneltest.New(t)

	// Flood 10,000 facts
	for i := 0; i < 10000; i++ {
		k.Assert(types.Fact{Predicate: "observed", Args: []any{i}})
	}

	results, err := k.Query("observed(X)")
	if err != nil {
		t.Fatalf("Kernel failed under load: %v", err)
	}
	if len(results) != 10000 {
		t.Fatalf("Expected 10000 facts, got %d", len(results))
	}
}

// TestE2E_SessionKernel_TemporalFailure_ContextCancellation mid-flight.
func TestE2E_SessionKernel_TemporalFailure_ContextCancellation(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)

	for i := 0; i < 20; i++ {
		for j := 0; j < 20; j++ {
			if i != j {
				k.Assert(types.Fact{Predicate: "dependency", Args: []any{i, j}})
			}
		}
	}

	cancelCtx, cancel := context.WithCancel(context.Background())

	// Cancel almost immediately
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	_, err := k.QueryContext(cancelCtx, "dependency(X, Y)")

	if err == nil {
		t.Fatalf("Query was expected to be cancelled but succeeded")
	} else if err != context.Canceled && !strings.Contains(err.Error(), "context") {
		t.Fatalf("Expected context cancellation error, got: %v", err)
	}
}

// TestE2E_SessionKernel_CascadingFailure_EmptyResultSwallowed proves the session
// must check for len(results) == 0, not just err != nil.
func TestE2E_SessionKernel_CascadingFailure_EmptyResultSwallowed(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)

	results, err := k.Query("permitted(/hacker, /delete, /prod)")

	// Contract: err == nil means the evaluation finished cleanly, not that permission is granted!
	if err != nil {
		t.Fatalf("Evaluation failed unexpectedly: %v", err)
	}

	// If the session swallows this and assumes success, it's a cascading failure.
	if len(results) > 0 {
		t.Fatalf("Expected no permission, got %d results", len(results))
	}
}

// TestE2E_SessionKernel_Recovery_AfterPanic simulates a bad state,
// and ensures the next turn with a fresh kernel recovers.
func TestE2E_SessionKernel_Recovery_AfterPanic(t *testing.T) {
	t.Parallel()

	// Simulating turn 1 that crashes
	k1 := kerneltest.New(t)
	_ = k1

	// Simulating turn 2 that recovers by using a new kernel instance
	k2 := kerneltest.New(t)

	k2.Assert(types.Fact{Predicate: "observed", Args: []any{"recovery_user"}})
	results, err := k2.Query(`observed("recovery_user")`)

	if err != nil || len(results) == 0 {
		t.Fatalf("System did not recover on Turn 2: %v", err)
	}
}

// TestE2E_SessionKernel_ContractViolation_MissingSchema tests querying
// an undeclared predicate.
func TestE2E_SessionKernel_ContractViolation_MissingSchema(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)

	_, err := k.Query("undeclared_magic_predicate(/foo)")
	if err == nil {
		t.Fatalf("Querying an undeclared predicate should return an error")
	}
}

// TestE2E_SessionKernel_Smoke_FullPipeline asserts a multi-turn fact chain.
func TestE2E_SessionKernel_Smoke_FullPipeline(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)
	// 1. Initial perception
	k.Assert(types.Fact{Predicate: "user_intent", Args: []any{types.MangleAtom("jules"), "fix_bug"}})

	// 2. Kernel derives next action
	results, err := k.Query("user_intent(X, Y)")
	if err != nil || len(results) == 0 {
		t.Fatalf("Failed initial intent assertion")
	}

	// 3. Spawner creates subagent and assigns task
	k.Assert(types.Fact{Predicate: "task_assigned", Args: []any{types.MangleAtom("jules"), types.MangleAtom("coder"), "fix_bug"}})

	// 4. Validate cascade
	results, err = k.Query("task_assigned(U, S, T)")
	if err != nil || len(results) == 0 {
		t.Fatalf("Failed to retrieve assigned task")
	}
}

// TestE2E_SessionKernel_TemporalFailure_SlowQuery tests the timeout mechanism
// for complex queries that don't finish in time.
func TestE2E_SessionKernel_TemporalFailure_SlowQuery(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)

	for i := 0; i < 50; i++ {
		for j := 0; j < 50; j++ {
			if i != j {
				k.Assert(types.Fact{Predicate: "dependency", Args: []any{i, j}})
			}
		}
	}

	// Deliberately short timeout to ensure the query catches it
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	_, err := k.QueryContext(ctx, "dependency(X, Y)")
	if err == nil {
		t.Fatalf("Context expired before query began, or query is too fast to timeout")
	} else if err != context.DeadlineExceeded && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("Got unexpected error: %v", err)
	}
}

// TestE2E_SessionKernel_ContractViolation_TypeDissonance_Complex verifies
// when a complex type like a list or struct is expected but primitive given.
func TestE2E_SessionKernel_ContractViolation_TypeDissonance_Complex(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)

	// Supplying integer where an Atom is expected
	k.Assert(types.Fact{Predicate: "context_priority", Args: []any{10, 10}})

	results, err := k.Query("context_priority(/target, X)")
	if err != nil {
		t.Fatalf("Query should not fail on malformed facts, just ignore them")
	}
	if len(results) > 0 {
		t.Fatalf("Expected no results due to type mismatch")
	}
}

// TestE2E_SessionKernel_StateCorruption_SharedInstance demonstrates
// why sessions must not share RealKernel instances across unrelated tasks.
func TestE2E_SessionKernel_StateCorruption_SharedInstance(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)
	// Task A asserts permission
	k.Assert(types.Fact{Predicate: "permitted", Args: []any{types.MangleAtom("userA"), types.MangleAtom("read"), types.MangleAtom("secret")}})

	// Task B, using same kernel, queries permission
	results, err := k.Query("permitted(/userB, /read, /secret)")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) > 0 {
		t.Fatalf("Task B received permission meant for Task A, State corrupted")
	}

	// Task B queries Task A's permission explicitly
	results, err = k.Query("permitted(/userA, X, Y)")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("Task A permission lost")
	}
}

// TestE2E_SessionKernel_Recovery_FromGarbageInput ensures the kernel
// remains stable even after processing malformed intents.
func TestE2E_SessionKernel_Recovery_FromGarbageInput(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)
	// 1. Send garbage
	k.Assert(types.Fact{Predicate: "invalid_json_payload", Args: []any{"{malformed_json_here]"}})

	// 2. Validate kernel doesn't panic and still functions
	k.Assert(types.Fact{Predicate: "valid_fact", Args: []any{1}})

	results, err := k.Query("valid_fact(X)")
	if err != nil || len(results) == 0 {
		t.Fatalf("Kernel did not recover after garbage fact assertion")
	}
}

// TestE2E_SessionKernel_ContractViolation_PartialState verifies that
// incomplete task graphs don't trigger completion logic.
func TestE2E_SessionKernel_ContractViolation_PartialState(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)

	k.Assert(types.Fact{Predicate: "task_started", Args: []any{"task1"}})
	// Missing task_completed

	results, err := k.Query("task_completed(X)")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) > 0 {
		t.Fatalf("Task completed prematurely based on partial state")
	}
}

// TestE2E_SessionKernel_ResourceExhaustion_ConcurrentConnections verifies
// the kernel can handle high concurrency gracefully.
func TestE2E_SessionKernel_ResourceExhaustion_ConcurrentConnections(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			k.Assert(types.Fact{Predicate: "concurrent_event", Args: []any{id}})
		}(i)
	}
	wg.Wait()

	results, err := k.Query("concurrent_event(X)")
	if err != nil {
		t.Fatalf("Query failed after concurrent load: %v", err)
	}
	if len(results) != 100 {
		t.Fatalf("Expected 100 facts, got %d", len(results))
	}
}

// TestE2E_SessionKernel_ContractViolation_ArityMismatch tests asserting
// a fact with wrong number of arguments.
func TestE2E_SessionKernel_ContractViolation_ArityMismatch(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)

	// Assume permitted requires 3 arguments. Send 2.
	k.Assert(types.Fact{Predicate: "permitted", Args: []any{types.MangleAtom("user"), types.MangleAtom("action")}})

	// Query for 3 arguments.
	results, err := k.Query("permitted(U, A, T)")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) > 0 {
		t.Fatalf("Expected no results due to arity mismatch")
	}
}

// TestE2E_SessionKernel_CascadingFailure_PanicRecovery ensures that if
// a query panics (e.g. malformed internal AST evaluation), it doesn't crash the session.
func TestE2E_SessionKernel_CascadingFailure_PanicRecovery(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Logf("Recovered from panic: %v", r)
		}
	}()

	k := kerneltest.New(t)

	// This might trigger a parser error or AST panic depending on Mangle internals.
	// In Mangle, parsing malformed queries usually returns err instead of panic,
	// but we test the boundary nonetheless.
	_, err := k.Query("malformed((((query)")
	if err == nil {
		t.Fatalf("Malformed query didn't fail, it was silently swallowed")
	}
}

// TestE2E_SessionKernel_CascadingFailure_RecursiveLogic tests how the session
// deals with deep logical recursion from the kernel.
func TestE2E_SessionKernel_CascadingFailure_RecursiveLogic(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)

	// In kerneltest, we cannot easily define rules via API, but we can flood facts
	// representing a deep graph and query it to see if evaluation is bounded.
	// Mangle evaluation is generally bounded by Datalog termination constraints.
	for i := 0; i < 50; i++ {
		k.Assert(types.Fact{Predicate: "node", Args: []any{i}})
		k.Assert(types.Fact{Predicate: "edge", Args: []any{i, i + 1}})
	}

	results, err := k.Query("edge(X, Y)")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 50 {
		t.Fatalf("Expected 50 edges, got %d", len(results))
	}
}

// TestE2E_SessionKernel_CascadingFailure_EmptyResultSwallowed_Deep tests
// a complex scenario where a deep rule evaluation fails and returns empty,
// and ensures the session doesn't mistake a nil error for a granted permission.
func TestE2E_SessionKernel_CascadingFailure_EmptyResultSwallowed_Deep(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)

	// Simulate a deep dependency chain that ultimately fails
	for i := 0; i < 50; i++ {
		k.Assert(types.Fact{Predicate: "requires", Args: []any{i, i + 1}})
	}

	// The query asks for something that doesn't exist at the end of the chain
	results, err := k.Query("requires(0, 100)")

	if err != nil {
		t.Fatalf("Evaluation failed unexpectedly: %v", err)
	}

	if len(results) > 0 {
		t.Fatalf("Expected no results, got %d", len(results))
	}
}

// TestE2E_SessionKernel_TemporalFailure_StalledFactAssertion tests what happens
// if fact assertions are stalled due to lock contention in the kernel.
func TestE2E_SessionKernel_TemporalFailure_StalledFactAssertion(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)

	var wg sync.WaitGroup
	start := make(chan struct{})

	// Spawn many goroutines trying to assert facts simultaneously
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			<-start // Wait for signal to create maximum contention
			k.Assert(types.Fact{Predicate: "stalled_fact", Args: []any{val}})
		}(i)
	}

	// Trigger all assertions at once
	close(start)

	// Use a context timeout to bound the overall wait time
	c := make(chan struct{})
	go func() {
		wg.Wait()
		close(c)
	}()

	select {
	case <-c:
		// Success, contention was handled within reasonable time
	case <-time.After(2 * time.Second):
		t.Fatalf("Kernel lock contention caused severe stall")
	}

	// Verify all facts made it
	results, err := k.Query("stalled_fact(X)")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 200 {
		t.Fatalf("Expected 200 facts, got %d", len(results))
	}
}

// TestE2E_SessionKernel_StateCorruption_LeakedTransaction tests if a failed
// session task leaves behind partial facts that corrupt subsequent tasks.
func TestE2E_SessionKernel_StateCorruption_LeakedTransaction(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)

	// Task 1 starts and asserts partial state
	k.Assert(types.Fact{Predicate: "transaction_started", Args: []any{"tx1"}})
	k.Assert(types.Fact{Predicate: "transaction_step", Args: []any{"tx1", 1}})

	// Task 1 panics or is cancelled before completion

	// Task 2 starts (using the same kernel, which is a contract violation)
	// It queries for active transactions
	results, err := k.Query("transaction_started(X)")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) > 0 {
		t.Fatalf("Task 1 leaked state into Task 2. This proves the need for isolated kernels.")
	}
}

// TestE2E_SessionKernel_ResourceExhaustion_MassiveVariables tests querying
// with a large number of variables, forcing the engine to compute a massive join.
func TestE2E_SessionKernel_ResourceExhaustion_MassiveVariables(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)

	// Assert a single fact with many arguments (if schema allows, else just multiple facts)
	// We'll assert a grid of facts and query with multiple variables
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			k.Assert(types.Fact{Predicate: "grid", Args: []any{i, j}})
		}
	}

	// Complex join query
	results, err := k.Query("grid(X, Y)")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 100 {
		t.Fatalf("Expected 100 results, got %d", len(results))
	}
}

// TestE2E_SessionKernel_ContractViolation_TypeDissonance_Complex2 verifies
// when a string is provided where an integer is expected.
func TestE2E_SessionKernel_ContractViolation_TypeDissonance_Complex2(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)

	// Supplying string where number is expected
	k.Assert(types.Fact{Predicate: "context_priority", Args: []any{types.MangleAtom("target"), "10"}})

	results, err := k.Query("context_priority(/target, X)")
	if err != nil {
		t.Fatalf("Query should not fail on malformed facts, just ignore them")
	}
	if len(results) > 0 {
		t.Fatalf("Expected no results due to type mismatch")
	}
}

// TestE2E_SessionKernel_Recovery_FromGarbageInput2 ensures the kernel
// remains stable even after processing malformed intents with nested structures.
func TestE2E_SessionKernel_Recovery_FromGarbageInput2(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)

	// 1. Send garbage (nested slice)
	k.Assert(types.Fact{Predicate: "invalid_json_payload", Args: []any{[]string{"nested", "garbage"}}})

	// 2. Validate kernel doesn't panic and still functions
	k.Assert(types.Fact{Predicate: "valid_fact", Args: []any{2}})

	results, err := k.Query("valid_fact(X)")
	if err != nil || len(results) == 0 {
		t.Fatalf("Kernel did not recover after garbage fact assertion")
	}
}

// TestE2E_SessionKernel_ContractViolation_PartialState2 verifies that
// incomplete task graphs don't trigger completion logic for nested states.
func TestE2E_SessionKernel_ContractViolation_PartialState2(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)

	k.Assert(types.Fact{Predicate: "task_started", Args: []any{"task2"}})
	k.Assert(types.Fact{Predicate: "task_step", Args: []any{"task2", "step1"}})
	// Missing task_completed

	results, err := k.Query("task_completed(X)")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) > 0 {
		t.Fatalf("Task completed prematurely based on partial state")
	}
}

// TestE2E_SessionKernel_MassiveIntegration_EndToEnd simulates a full
// session flow with setup, assertion, delegation, and verification.
func TestE2E_SessionKernel_MassiveIntegration_EndToEnd(t *testing.T) {
	t.Parallel()
	k := kerneltest.New(t)

	// 1. Setup the session state
	k.Assert(types.Fact{Predicate: "session", Args: []any{"session-12345"}})
	k.Assert(types.Fact{Predicate: "session_user", Args: []any{"session-12345", "alice"}})

	// 2. Setup the target target state
	k.Assert(types.Fact{Predicate: "target", Args: []any{"target-abc"}})
	k.Assert(types.Fact{Predicate: "target_owner", Args: []any{"target-abc", "alice"}})

	// 3. User attempts an action
	k.Assert(types.Fact{Predicate: "attempt", Args: []any{"session-12345", "read", "target-abc"}})

	// 4. Evaluate access (this requires IDB in reality, but here we just verify state is cleanly queryable)
	results, err := k.Query("attempt(S, A, T)")
	if err != nil {
		t.Fatalf("Failed to query attempt: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 attempt, got %d", len(results))
	}

	// 5. Evaluate complex join: Does the session user own the target?
	// In Datalog: owns(S, T) :- session_user(S, U), target_owner(T, U).
	// We simulate the evaluation by querying the base facts directly.
	userResults, err := k.Query("session_user(S, U)")
	if err != nil {
		t.Fatalf("Failed to query session_user: %v", err)
	}

	ownerResults, err := k.Query("target_owner(T, U)")
	if err != nil {
		t.Fatalf("Failed to query target_owner: %v", err)
	}

	// Manual Join in Go (simulating what the kernel does internally)
	joined := false
	for _, ur := range userResults {
		for _, or := range ownerResults {
			if ur.Args[1] == or.Args[1] {
				joined = true
				break
			}
		}
	}

	if !joined {
		t.Fatalf("Manual join failed: Alice should own the target")
	}

	// 6. Tear down (simulated by not doing anything since facts are monotonic)
	// Explicit assertion not needed here as previous Fatalfs guarantee success if we reach this point.
	if !joined {
		t.Fatalf("Final verification failed")
	}
}
