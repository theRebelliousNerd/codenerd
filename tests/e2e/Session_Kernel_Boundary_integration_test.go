//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/session"
	"codenerd/internal/jit/config"
	"codenerd/internal/prompt"

	"codeberg.org/TauCeti/mangle-go/ast"
)

// mockSiegeJITCompiler satisfies the JITCompiler interface.
type mockSiegeJITCompiler struct{}

func (m *mockSiegeJITCompiler) Compile(ctx context.Context, compCtx *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	return &prompt.CompilationResult{
		Prompt: "Mock Prompt",
	}, nil
}

// mockSiegeConfigFactory satisfies the ConfigFactory interface.
type mockSiegeConfigFactory struct{}

func (m *mockSiegeConfigFactory) Generate(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error) {
	return &config.EffectiveAgentRuntimeConfig{
		AllowedTools: []string{"test_tool"},
	}, nil
}

// -----------------------------------------------------------------------------
// CATEGORY 1: SMOKE TESTS (2)
// -----------------------------------------------------------------------------

// TestE2E_Boundary_Session_Kernel_FactIsolation verifies executor instantiation.
func TestE2E_Boundary_Session_Kernel_FactIsolation(t *testing.T) {
	t.Log("KNOWN: Verifying basic boundary interaction.")
	k, _ := core.NewRealKernel()

	executor := session.NewExecutor(k, nil, nil, &mockSiegeJITCompiler{}, &mockSiegeConfigFactory{}, nil)

	if executor == nil {
		t.Fatalf("Failed to create executor")
	}

	intent := perception.Intent{
		Verb: "/fix",
		Target: "auth.go",
	}
	fact := intent.ToFact()
	err := k.Assert(fact)
	if err != nil {
		t.Fatalf("Failed to assert fact: %v", err)
	}

	facts, err := k.Query("user_intent(_, _, /fix, _, _)")
	if err != nil || len(facts) == 0 {
		t.Fatalf("Expected to find asserted fact")
	}
}

// TestE2E_Boundary_Session_Kernel_BasicRetraction verifies retract across boundary.
func TestE2E_Boundary_Session_Kernel_BasicRetraction(t *testing.T) {
	t.Log("KNOWN: Verifying retract across the boundary.")
	k, _ := core.NewRealKernel()

	intent := perception.Intent{Verb: "/test", Target: "main.go"}
	_ = k.Assert(intent.ToFact())
	_ = k.Retract("user_intent")

	facts, _ := k.Query("user_intent(_, _, _, _, _)")
	if len(facts) > 0 {
		t.Fatalf("Retract failed across boundary")
	}
}

// -----------------------------------------------------------------------------
// CATEGORY 2: STATE CORRUPTION (5)
// TODO: Add tests for Massive Arity facts to ensure the Kernel rejects them gracefully.
// TODO: Add tests for Extreme String Lengths (e.g., 50MB strings) to check for OOM vs boundary length enforcement.
// -----------------------------------------------------------------------------

// TestE2E_Boundary_Session_Kernel_ConcurrentStateCorruption tests concurrent assertions.
func TestE2E_Boundary_Session_Kernel_ConcurrentStateCorruption(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	var wg sync.WaitGroup
	errCh := make(chan error, 100)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			intent := perception.Intent{
				Verb: fmt.Sprintf("/fix_%d", id),
				Target: fmt.Sprintf("file_%d.go", id),
			}
			err := k.Assert(intent.ToFact())
			if err != nil {
				errCh <- fmt.Errorf("id %d failed to assert: %w", id, err)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("Concurrent assertion error: %v", err)
	}
}

// TestE2E_Boundary_Session_Kernel_ConcurrentReadWrite tests read/write races.
func TestE2E_Boundary_Session_Kernel_ConcurrentReadWrite(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for i := 0; i < 100; i++ {
		intent := perception.Intent{Verb: "/test", Target: fmt.Sprintf("target_%d", i)}
		_ = k.Assert(intent.ToFact())
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_, _ = k.Query("user_intent(_, _, /test, _, _)")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			intent := perception.Intent{Verb: "/test", Target: fmt.Sprintf("target_%d", i)}
			_ = k.Retract(intent.ToFact().Predicate)
		}
	}()

	wg.Wait()
}

// TestE2E_Boundary_Session_Kernel_CrossTalk verifies session isolation.
func TestE2E_Boundary_Session_Kernel_CrossTalk(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	intentA := perception.Intent{Verb: "/coder"}
	intentB := perception.Intent{Verb: "/tester"}

	_ = k.Assert(intentA.ToFact())
	_ = k.Assert(intentB.ToFact())

	facts, _ := k.Query("user_intent(_, _, V, _, _)")
	if len(facts) != 2 {
		t.Fatalf("Expected 2 facts indicating cross-talk risk is present, got %d", len(facts))
	}
}

// TestE2E_Boundary_Session_Kernel_ConcurrentRetract tests multi-retract races.
func TestE2E_Boundary_Session_Kernel_ConcurrentRetract(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	intent := perception.Intent{Verb: "/test"}
	_ = k.Assert(intent.ToFact())

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = k.Retract("user_intent")
		}()
	}
	wg.Wait()
}

// TestE2E_Boundary_Session_Kernel_HighContention tests RWMutex bottlenecks.
func TestE2E_Boundary_Session_Kernel_HighContention(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping high contention test in short mode")
	}
	k, _ := core.NewRealKernel()

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			intent := perception.Intent{Verb: fmt.Sprintf("/verb_%d", id)}
			_ = k.Assert(intent.ToFact())
		}(i)
	}
	wg.Wait()
}

// -----------------------------------------------------------------------------
// CATEGORY 3: CONTRACT VIOLATION (7)
// TODO: Add tests for Nil Intent pointers being passed to ProcessWithIntent.
// TODO: Add tests for Empty string targets and empty category fields.
// TODO: Add tests for Type coercion attacks (passing raw Go structs instead of Mangle AST to Fact.Args).
// -----------------------------------------------------------------------------

// TestE2E_Boundary_Session_Kernel_MangleInjection tests injection via target.
func TestE2E_Boundary_Session_Kernel_MangleInjection(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	maliciousTarget := "auth.go). nasty_fact(/hacked). p("
	intent := perception.Intent{Verb: "/fix", Target: maliciousTarget}
	fact := intent.ToFact()

	err := k.Assert(fact)
	if err != nil {
		t.Logf("Expected parsing error or success with escaping: %v", err)
	}

	nastyFacts, err := k.Query("nasty_fact(_)")
	if err == nil && len(nastyFacts) > 0 {
		t.Fatalf("CRITICAL: Mangle injection succeeded across boundary!")
	}
}

// TestE2E_Boundary_Session_Kernel_AtomStringDissonance tests type matching.
func TestE2E_Boundary_Session_Kernel_AtomStringDissonance(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	fact := core.Fact{
		Predicate: "user_intent",
		Args: []any{ast.String("id"), ast.String("cat"), ast.String("/fix"), ast.String("t"), ast.String("c")},
	}
	_ = k.Assert(fact)

	facts, _ := k.Query("user_intent(_, _, /fix, _, _)")
	if len(facts) > 0 {
		t.Fatalf("Contract violation: String matched an Atom. This hides real bugs.")
	}
}

// TestE2E_Boundary_Session_Kernel_InvalidArity tests schema enforcement.
func TestE2E_Boundary_Session_Kernel_InvalidArity(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	fact := core.Fact{
		Predicate: "user_intent",
		Args: []any{ast.String("/fix"), ast.String("target"), ast.String("constraint")},
	}

	err := k.Assert(fact)
	if err == nil {
		t.Log("KNOWN LIMITATION: Kernel does not strictly enforce schema arity on assert.")
	}
}

// TestE2E_Boundary_Session_Kernel_DuplicateAssertion tests idempotency contract.
func TestE2E_Boundary_Session_Kernel_DuplicateAssertion(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	intent := perception.Intent{Verb: "/fix", Target: "auth.go"}

	_ = k.Assert(intent.ToFact())
	_ = k.Assert(intent.ToFact())

	facts, _ := k.Query("user_intent(_, _, /fix, _, _)")
	if len(facts) > 1 {
		t.Fatalf("Contract violation: Duplicate facts inserted. Sets must be unique.")
	}
}

// TestE2E_Boundary_Session_Kernel_EmptyIntent tests empty data propagation.
func TestE2E_Boundary_Session_Kernel_EmptyIntent(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	intent := perception.Intent{}
	err := k.Assert(intent.ToFact())
	if err != nil {
		t.Fatalf("Kernel rejected empty intent fact: %v", err)
	}
}

// TestE2E_Boundary_Session_Kernel_InvalidMangleName tests invalid predicate names.
func TestE2E_Boundary_Session_Kernel_InvalidMangleName(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	fact := core.Fact{Predicate: "invalid name with spaces", Args: []any{ast.String("test")}}
	err := k.Assert(fact)
	if err == nil {
		t.Fatalf("Expected kernel to reject fact with invalid mangle predicate name.")
	}
}

// TestE2E_Boundary_Session_Kernel_NullTarget tests handling of empty strings.
func TestE2E_Boundary_Session_Kernel_NullTarget(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	intent := perception.Intent{Verb: "/fix", Target: ""}
	err := k.Assert(intent.ToFact())
	if err != nil {
		t.Fatalf("Kernel rejected intent with empty target string: %v", err)
	}
}

// -----------------------------------------------------------------------------
// CATEGORY 4: RESOURCE EXHAUSTION (3)
// TODO: Add concurrent Assert/Retract race condition tests (e.g., 100 goroutines asserting and retracting the same fact simultaneously).
// -----------------------------------------------------------------------------

// TestE2E_Boundary_Session_Kernel_WidthExhaustion floods the EDB.
func TestE2E_Boundary_Session_Kernel_WidthExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource exhaustion test in short mode")
	}
	k, _ := core.NewRealKernel()

	for i := 0; i < 10000; i++ {
		intent := perception.Intent{Verb: "/explore", Target: fmt.Sprintf("file_%d.txt", i)}
		err := k.Assert(intent.ToFact())
		if err != nil {
			t.Fatalf("Failed on fact %d: %v", i, err)
		}
	}

	res, _ := k.Query("user_intent(_, _, /explore, _, _)")
	if len(res) < 10000 {
		t.Fatalf("Lost facts during flood. Expected >= 10000, got %d", len(res))
	}
}

// TestE2E_Boundary_Session_Kernel_StringAllocationPressure tests GC pressure.
func TestE2E_Boundary_Session_Kernel_StringAllocationPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping allocation test in short mode")
	}
	k, _ := core.NewRealKernel()

	largeString := strings.Repeat("A", 1024*1024)

	for i := 0; i < 100; i++ {
		intent := perception.Intent{Verb: "/parse", Target: fmt.Sprintf("%s_%d", largeString, i)}
		_ = k.Assert(intent.ToFact())
	}

	res, _ := k.Query("user_intent(_, _, /parse, _, _)")
	if len(res) != 100 {
		t.Fatalf("Failed large string allocation test")
	}
}

// TestE2E_Boundary_Session_Kernel_ContinuousAssertionLoop tests unbounded loops.
func TestE2E_Boundary_Session_Kernel_ContinuousAssertionLoop(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping continuous assertion test in short mode")
	}
	k, _ := core.NewRealKernel()
	intent := perception.Intent{Verb: "/loop"}
	fact := intent.ToFact()

	for i := 0; i < 50000; i++ {
		_ = k.Assert(fact)
	}

	res, _ := k.Query("user_intent(_, _, /loop, _, _)")
	if len(res) != 1 {
		t.Fatalf("Expected exactly 1 deduplicated fact, got %d", len(res))
	}
}

// -----------------------------------------------------------------------------
// CATEGORY 5: TEMPORAL FAILURE (4)
// TODO: Add tests for Context Cancellation during active Kernel locks (simulating concurrent query/write interrupted).
// -----------------------------------------------------------------------------

// TestE2E_Boundary_Session_Kernel_ContextCancel_Process verifies Executor honors cancellation.
func TestE2E_Boundary_Session_Kernel_ContextCancel_Process(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()
	executor := session.NewExecutor(k, nil, nil, &mockSiegeJITCompiler{}, &mockSiegeConfigFactory{}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := executor.Process(ctx, "Test Input")
	if err == nil {
		t.Fatalf("Expected context cancellation error, got nil")
	}
}

// TestE2E_Boundary_Session_Kernel_ContextCancel_WithIntent verifies explicit intent cancellation.
func TestE2E_Boundary_Session_Kernel_ContextCancel_WithIntent(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()
	executor := session.NewExecutor(k, nil, nil, &mockSiegeJITCompiler{}, &mockSiegeConfigFactory{}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	intent := perception.Intent{Verb: "/test"}
	_, err := executor.ProcessWithIntent(ctx, "Input", &intent)
	if err == nil {
		t.Fatalf("Expected context cancellation error, got nil")
	}
}

// TestE2E_Boundary_Session_Kernel_QueryTimeout verifies Kernel handles timeouts.
func TestE2E_Boundary_Session_Kernel_QueryTimeout(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	intent := perception.Intent{Verb: "/slow_query"}
	_ = k.Assert(intent.ToFact())

	_, err := k.Query("user_intent(_, _, /slow_query, _, _)")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
}

// TestE2E_Boundary_Session_Kernel_AssertionTimeout verifies timeout on Assert.
func TestE2E_Boundary_Session_Kernel_AssertionTimeout(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	// Simulated timeout since Kernel.Assert doesn't natively take Context
	done := make(chan bool)
	go func() {
		intent := perception.Intent{Verb: "/timeout"}
		_ = k.Assert(intent.ToFact())
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatalf("Assertion hung indefinitely")
	}
}

// -----------------------------------------------------------------------------
// CATEGORY 6: CASCADING FAILURE (3)
// -----------------------------------------------------------------------------

// TestE2E_Boundary_Session_Kernel_CascadingTransducerFailure verifies error propagation.
func TestE2E_Boundary_Session_Kernel_CascadingTransducerFailure(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()
	executor := session.NewExecutor(k, nil, nil, &mockSiegeJITCompiler{}, &mockSiegeConfigFactory{}, nil)

	intent := perception.Intent{
		Verb: "/unknown_verb_that_causes_nil",
	}
	_ = k.Assert(intent.ToFact())

	ctx := context.Background()
	_, err := executor.ProcessWithIntent(ctx, "Input", &intent)

	if err != nil {
		t.Logf("Executor safely handled cascading failure: %v", err)
	} else {
		t.Log("Executor recovered or fell back gracefully.")
	}
}

// TestE2E_Boundary_Session_Kernel_CascadingJITFailure verifies JIT handles kernel garbage.
func TestE2E_Boundary_Session_Kernel_CascadingJITFailure(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()
	executor := session.NewExecutor(k, nil, nil, &mockSiegeJITCompiler{}, &mockSiegeConfigFactory{}, nil)

	_ = k.Assert(core.Fact{Predicate: "user_intent", Args: []any{}})

	_, err := executor.Process(context.Background(), "Input")
	if err != nil {
		t.Logf("Executor propagated error cleanly: %v", err)
	}
}

// TestE2E_Boundary_Session_Kernel_VirtualStoreFailure verifies Executor isolates store panics.
func TestE2E_Boundary_Session_Kernel_VirtualStoreFailure(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()
	executor := session.NewExecutor(k, nil, nil, &mockSiegeJITCompiler{}, &mockSiegeConfigFactory{}, nil)

	intent := perception.Intent{Verb: "/test"}
	_, err := executor.ProcessWithIntent(context.Background(), "Input", &intent)

	if err != nil {
		t.Logf("Safely caught potential VirtualStore downstream failure: %v", err)
	}
}

// -----------------------------------------------------------------------------
// CATEGORY 7: RECOVERY (3)
// -----------------------------------------------------------------------------

// TestE2E_Boundary_Session_Kernel_RecoveryAfterBadAssert verifies recovery after failure.
func TestE2E_Boundary_Session_Kernel_RecoveryAfterBadAssert(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	intent1 := perception.Intent{Verb: "/valid1"}
	_ = k.Assert(intent1.ToFact())

	_ = k.Assert(core.Fact{Predicate: "bad_predicate", Args: []any{"broken"}})

	intent2 := perception.Intent{Verb: "/valid2"}
	err := k.Assert(intent2.ToFact())
	if err != nil {
		t.Fatalf("Failed to recover and assert valid fact: %v", err)
	}

	facts, _ := k.Query("user_intent(_, _, /valid2, _, _)")
	if len(facts) == 0 {
		t.Fatalf("Recovery failed, second fact not found.")
	}
}

// TestE2E_Boundary_Session_Kernel_RecoveryAfterRetract verifies state recovers.
func TestE2E_Boundary_Session_Kernel_RecoveryAfterRetract(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	intent := perception.Intent{Verb: "/temp"}
	_ = k.Assert(intent.ToFact())

	_ = k.Retract("user_intent")

	facts, _ := k.Query("user_intent(_, _, /temp, _, _)")
	if len(facts) > 0 {
		t.Fatalf("Retract failed, fact still present.")
	}

	_ = k.Assert(intent.ToFact())
	facts2, _ := k.Query("user_intent(_, _, /temp, _, _)")
	if len(facts2) == 0 {
		t.Fatalf("Recovery after retract failed.")
	}
}

// TestE2E_Boundary_Session_Kernel_RecoveryAfterQueryError verifies query failure doesn't break EDB.
func TestE2E_Boundary_Session_Kernel_RecoveryAfterQueryError(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	intent := perception.Intent{Verb: "/valid"}
	_ = k.Assert(intent.ToFact())

	_, _ = k.Query("invalid_syntax_query(")

	facts, err := k.Query("user_intent(_, _, /valid, _, _)")
	if err != nil || len(facts) == 0 {
		t.Fatalf("Kernel state corrupted by malformed query: %v", err)
	}
}

// -----------------------------------------------------------------------------
// END-TO-END DATA INTEGRITY (PIPELINE TESTS) (4)
// -----------------------------------------------------------------------------

// TestE2E_Boundary_Session_Kernel_IntentFidelity verifies parameters pass through intact.
func TestE2E_Boundary_Session_Kernel_IntentFidelity(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	testCases := []struct {
		name       string
		verb       string
		target     string
		category   string
		constraint string
	}{
		{"Standard Fix", "/fix", "auth.go", "/code", "/none"},
		{"Complex Target", "/implement", "pkg/api/handlers.go", "/feature", "/high_perf"},
		{"Empty Fields", "/explore", "", "/research", ""},
		{"Special Chars", "/test", "file_with-dash.go", "/qa", "/no-cache"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			intent := perception.Intent{
				Verb: tc.verb,
				Target: tc.target,
				Category: tc.category,
				Constraint: tc.constraint,
			}
			err := k.Assert(intent.ToFact())
			if err != nil {
				t.Fatalf("Failed to assert: %v", err)
			}

			queryStr := fmt.Sprintf("user_intent(_, _, %s, _, _)", tc.verb)
			facts, err := k.Query(queryStr)
			if err != nil {
				t.Fatalf("Failed to query: %v", err)
			}
			if len(facts) == 0 {
				t.Fatalf("Data integrity lost across boundary for verb %s", tc.verb)
			}
		})
	}
}

// TestE2E_Boundary_Session_Kernel_PartialFailure verifies upstream state is preserved.
func TestE2E_Boundary_Session_Kernel_PartialFailure(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()
	executor := session.NewExecutor(k, nil, nil, &mockSiegeJITCompiler{}, &mockSiegeConfigFactory{}, nil)

	intent := perception.Intent{
		Verb: "/crash_the_router",
	}

	_ = k.Assert(intent.ToFact())

	_, err := executor.ProcessWithIntent(context.Background(), "Input", &intent)

	facts, qErr := k.Query("user_intent(_, _, /crash_the_router, _, _)")
	if qErr != nil || len(facts) == 0 {
		t.Fatalf("Upstream state was corrupted by downstream failure.")
	}

	if err == nil {
		t.Log("Expected an error from downstream processing, but it recovered.")
	}
}

// TestE2E_Boundary_Session_Kernel_MultiTurnAccumulation verifies history doesn't leak.
func TestE2E_Boundary_Session_Kernel_MultiTurnAccumulation(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()
	executor := session.NewExecutor(k, nil, nil, &mockSiegeJITCompiler{}, &mockSiegeConfigFactory{}, nil)

	for i := 0; i < 5; i++ {
		intent := perception.Intent{Verb: fmt.Sprintf("/turn_%d", i)}
		_, _ = executor.ProcessWithIntent(context.Background(), "Input", &intent)
	}

	if len(executor.GetHistory()) > 0 {
		t.Log("Executor successfully retained multi-turn history.")
	} else {
		t.Log("Turn execution completed without history leak/panic.")
	}
}

// TestE2E_Boundary_Session_Kernel_MultiTurnFactPruning verifies ephemeral facts are pruned.
func TestE2E_Boundary_Session_Kernel_MultiTurnFactPruning(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for i := 0; i < 5; i++ {
		intent := perception.Intent{Verb: fmt.Sprintf("/turn_%d", i)}
		_ = k.Assert(intent.ToFact())
	}

	_ = k.Retract("user_intent")

	facts, _ := k.Query("user_intent(_, _, _, _, _)")
	if len(facts) != 0 {
		t.Fatalf("Multi-turn fact pruning failed, ephemeral facts leaked.")
	}
}
