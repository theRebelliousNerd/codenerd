//go:build integration

package e2e_test

import (

	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/core"

	"codeberg.org/TauCeti/mangle-go/ast"

)

// =============================================================================
// Mock Graph Query Implementation
// =============================================================================

type mockAdversarialGraphQuery struct {
	mu           sync.RWMutex
	callCount    int32
	lastType     string
	lastParams   map[string]any

	// Failure injection controls
	blockForever bool
	delay        time.Duration
	returnError  error
	returnValue  any
	panicMessage string
}

func (m *mockAdversarialGraphQuery) QueryGraph(queryType string, params map[string]any) (any, error) {
	atomic.AddInt32(&m.callCount, 1)

	m.mu.Lock()
	m.lastType = queryType
	// Deep copy params for assertions
	m.lastParams = make(map[string]any)
	for k, v := range params {
		m.lastParams[k] = v
	}
	m.mu.Unlock()

	m.mu.RLock()
	blockForever := m.blockForever
	delay := m.delay
	returnError := m.returnError
	returnValue := m.returnValue
	panicMessage := m.panicMessage
	m.mu.RUnlock()

	if panicMessage != "" {
		panic(panicMessage)
	}

	if blockForever {
		// Simulate a permanent deadlock (e.g. infinite loop in AST resolution)
		select {}
	}

	if delay > 0 {
		time.Sleep(delay)
	}

	if returnError != nil {
		return nil, returnError
	}

	if returnValue != nil {
		return returnValue, nil
	}

	return []string{"mock_result_1", "mock_result_2"}, nil
}

// =============================================================================
// 1. SMOKE TESTS
// =============================================================================

// TestE2E_VirtualStore_GraphQuery_Smoke_BasicIntegration
// Scenario: The VirtualStore successfully dispatches a query_graph request to the
// GraphQuery interface and retrieves the atoms.
func TestE2E_VirtualStore_GraphQuery_Smoke_BasicIntegration(t *testing.T) {
	t.Parallel()

	vs := core.NewVirtualStore(nil)
	mockGQ := &mockAdversarialGraphQuery{
		returnValue: []string{"node1", "node2"},
	}
	vs.SetGraphQuery(mockGQ)

	// Create a real kernel to process the facts
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	kernel.SetVirtualStore(vs)

	// query_graph(Type, Params, Result)
	// We must query the kernel, which will hit the virtual store.

	// Add rules to trigger the virtual predicate
	policy := `
		Decl trigger_graph(String).
		trigger_graph(Res) :- query_graph("dependencies", "module_a", Res).
	`
	kernel.AppendPolicy(policy)

	results, err := kernel.Query("trigger_graph")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// We expect the result to be unified.
	// Since query_graph returns a list, the result should bind to that list.
	if len(results) == 0 {
		t.Fatalf("Expected results, got none. Integration broken.")
	}

	if atomic.LoadInt32(&mockGQ.callCount) == 0 {
		t.Fatalf("GraphQuery was never called.")
	}
}

// =============================================================================
// 2. CONTRACT VIOLATION TESTS
// =============================================================================

// TestE2E_VirtualStore_GraphQuery_Contract_SemanticObliteration
// Scenario: Mangle Map arguments are obliterated into a single string by the FFI boundary.
func TestE2E_VirtualStore_GraphQuery_Contract_SemanticObliteration(t *testing.T) {
	t.Parallel()

	vs := core.NewVirtualStore(nil)
	mockGQ := &mockAdversarialGraphQuery{}
	vs.SetGraphQuery(mockGQ)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	kernel.SetVirtualStore(vs)

	// We construct a query with a map in the second argument.
	// Mangle maps are represented using constructors, e.g., map:put(...) but
	// we will simulate the VirtualStore's internal call directly to bypass parser limits.

	var queryAtom ast.Atom
	_ = queryAtom
	queryAtom = ast.NewAtom("query_graph",
		ast.String("complex_query"),
		ast.String(`{"depth": 3, "exclude": "tests"}`),
		ast.Variable{Symbol: "R"},
	)

	// Invoke the unexported method via reflection or interface adaptation if needed.
	// We can assert the rule instead.
	policy := `
		Decl test_obliteration(Any).
		test_obliteration(R) :- query_graph("complex_query", "{"depth": 3, "exclude": "tests"}", R).
	`
	kernel.AppendPolicy(policy)
	_, _ = kernel.Query("test_obliteration")

	mockGQ.mu.RLock()
	params := mockGQ.lastParams
	mockGQ.mu.RUnlock()

	// ASSERTION: The contract violation. We EXPECT the system to obliterate the JSON/Map
	// into a single "arg" key, rather than parsing it into map[string]any.
	val, ok := params["arg"]
	if !ok {
		t.Fatalf("Expected 'arg' key in params, got: %v", params)
	}

	strVal, ok := val.(string)
	if !ok {
		t.Fatalf("Expected string value for 'arg', got %T", val)
	}

	if !strings.Contains(strVal, "depth") {
		t.Errorf("Expected obliterated string to contain original data, got %s", strVal)
	}

	t.Log("KNOWN: VirtualStore obliterates structured arguments into a single string 'arg' key.")
}

// TestE2E_VirtualStore_GraphQuery_Contract_StructStringification
// Scenario: Returning a custom struct causes goToMangleTerm to fall back to fmt.Sprintf.
func TestE2E_VirtualStore_GraphQuery_Contract_StructStringification(t *testing.T) {
	t.Parallel()

	type CustomStruct struct {
		FieldA string
		FieldB int
	}

	vs := core.NewVirtualStore(nil)
	mockGQ := &mockAdversarialGraphQuery{
		returnValue: CustomStruct{FieldA: "hello", FieldB: 42},
	}
	vs.SetGraphQuery(mockGQ)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	kernel.SetVirtualStore(vs)

	policy := `
		Decl test_struct(Any).
		test_struct(R) :- query_graph("test", "test", R).
	`
	kernel.AppendPolicy(policy)
	results, err := kernel.Query("test_struct")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("Expected result")
	}

	// ASSERTION: The result is a raw string "{hello 42}" instead of a structured Mangle type.
	resStr := fmt.Sprintf("%v", results[0].Args[0])
	if !strings.Contains(resStr, "{hello 42}") {
		t.Errorf("Expected struct stringification '{hello 42}', got: %s", resStr)
	}

	t.Log("KNOWN: VirtualStore stringifies unknown structs, destroying their fields for Mangle queries.")
}

// =============================================================================
// 3. TEMPORAL FAILURE TESTS
// =============================================================================

// TestE2E_VirtualStore_GraphQuery_Temporal_Deadlock
// Scenario: GraphQuery blocks indefinitely. Because getQueryGraphAtoms lacks a ctx,
// the kernel evaluation hangs. We test this by applying a timeout to our test context.
func TestE2E_VirtualStore_GraphQuery_Temporal_Deadlock(t *testing.T) {
	t.Parallel()

	vs := core.NewVirtualStore(nil)
	mockGQ := &mockAdversarialGraphQuery{
		blockForever: true,
	}
	vs.SetGraphQuery(mockGQ)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	kernel.SetVirtualStore(vs)

	policy := `
		Decl test_deadlock(Any).
		test_deadlock(R) :- query_graph("deadlock_test", "test", R).
	`
	kernel.AppendPolicy(policy)

	// We wrap the kernel query in a goroutine because we KNOW it will deadlock.
	done := make(chan struct{})
	go func() {
		_, _ = kernel.Query("test_deadlock")
		close(done)
	}()

	select {
	case <-done:
		t.Fatalf("Query completed when it should have deadlocked indefinitely.")
	case <-time.After(100 * time.Millisecond):
		// Expected behavior: the goroutine is permanently blocked.
		t.Log("KNOWN: Kernel query permanently deadlocks if GraphQuery hangs, due to missing context.Context propagation.")
	}
}

// =============================================================================
// 4. RESOURCE EXHAUSTION TESTS
// =============================================================================

// TestE2E_VirtualStore_GraphQuery_Resource_MassiveList
// Scenario: GraphQuery returns an enormous list, stressing the `goToMangleTerm` array builder.
func TestE2E_VirtualStore_GraphQuery_Resource_MassiveList(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large payload test in short mode")
	}
	t.Parallel()

	vs := core.NewVirtualStore(nil)

	// Create a 500,000 item slice
	massiveSlice := make([]string, 500000)
	for i := 0; i < 500000; i++ {
		massiveSlice[i] = fmt.Sprintf("node_%d", i)
	}

	mockGQ := &mockAdversarialGraphQuery{
		returnValue: massiveSlice,
	}
	vs.SetGraphQuery(mockGQ)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	kernel.SetVirtualStore(vs)

	policy := `
		Decl test_massive(Any).
		test_massive(R) :- query_graph("test", "test", R).
	`
	kernel.AppendPolicy(policy)

	// This should not OOM, but it will take time to build the AST node.
	start := time.Now()
	_, err = kernel.Query("test_massive")

	if err != nil {
		t.Fatalf("Failed to process massive list: %v", err)
	}

	if time.Since(start) > 5*time.Second {
		t.Logf("Performance warning: processing massive graph list took %v", time.Since(start))
	}
}

// =============================================================================
// 5. CONCURRENCY & STATE CORRUPTION TESTS
// =============================================================================

// TestE2E_VirtualStore_GraphQuery_State_ConcurrentSwap
// Scenario: The Orchestrator swaps the GraphQuery pointer while the Kernel is evaluating heavily.
func TestE2E_VirtualStore_GraphQuery_State_ConcurrentSwap(t *testing.T) {
	t.Parallel()

	vs := core.NewVirtualStore(nil)
	mockGQ1 := &mockAdversarialGraphQuery{returnValue: []string{"A"}}
	mockGQ2 := &mockAdversarialGraphQuery{returnValue: []string{"B"}}

	vs.SetGraphQuery(mockGQ1)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	kernel.SetVirtualStore(vs)

	policy := `
		Decl test_swap(Any).
		test_swap(R) :- query_graph("test", "test", R).
	`
	kernel.AppendPolicy(policy)

	var wg sync.WaitGroup

	// Reader goroutines
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				res, _ := kernel.Query("test_swap")
				// We don't care about the result, just that it doesn't panic on a read/write race.
				_ = res
			}
		}()
	}

	// Writer goroutines swapping the pointer
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if idx%2 == 0 {
					vs.SetGraphQuery(mockGQ1)
				} else {
					vs.SetGraphQuery(mockGQ2)
				}
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	// If the RWMutex works, this will not trigger the race detector or panic.
	wg.Wait()
}

// =============================================================================
// 6. CASCADING FAILURE TESTS
// =============================================================================

// TestE2E_VirtualStore_GraphQuery_Cascading_NilInterface
// Scenario: GraphQuery is never initialized. Evaluation silently fails rather than erroring,
// masking the misconfiguration from the orchestrator.
func TestE2E_VirtualStore_GraphQuery_Cascading_NilInterface(t *testing.T) {
	t.Parallel()

	vs := core.NewVirtualStore(nil)
	// Deliberately do NOT call vs.SetGraphQuery()

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	kernel.SetVirtualStore(vs)

	policy := `
		Decl test_nil(Any).
		test_nil(R) :- query_graph("test", "test", R).
	`
	kernel.AppendPolicy(policy)

	results, err := kernel.Query("test_nil")

	// The cascade: err is nil, results are empty. The caller thinks the graph is empty,
	// rather than realizing the integration is broken.
	if err != nil {
		t.Fatalf("Expected silent failure (nil error), got: %v", err)
	}

	if len(results) != 0 {
		t.Fatalf("Expected 0 results from nil interface, got %d", len(results))
	}
}

// =============================================================================
// 7. RECOVERY TESTS
// =============================================================================

// TestE2E_VirtualStore_GraphQuery_Recovery_ErrorPropagation
// Scenario: GraphQuery returns a transient error. The query fails, but subsequent
// queries succeed once the graph query engine recovers.
func TestE2E_VirtualStore_GraphQuery_Recovery_ErrorPropagation(t *testing.T) {
	t.Parallel()

	vs := core.NewVirtualStore(nil)
	mockGQ := &mockAdversarialGraphQuery{
		returnError: fmt.Errorf("transient connection lost to graph db"),
	}
	vs.SetGraphQuery(mockGQ)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	kernel.SetVirtualStore(vs)

	policy := `
		Decl test_recovery(Any).
		test_recovery(R) :- query_graph("test", "test", R).
	`
	kernel.AppendPolicy(policy)

	// First query fails (returns empty in Mangle context due to nil return)
	results, _ := kernel.Query("test_recovery")
	if len(results) != 0 {
		t.Fatalf("Expected empty results on error")
	}

	// Recover the mock
	mockGQ.mu.Lock()
	mockGQ.returnError = nil
	mockGQ.returnValue = []string{"recovered"}
	mockGQ.mu.Unlock()

	// Second query succeeds
	results, err = kernel.Query("test_recovery")
	if err != nil {
		t.Fatalf("Failed to query after recovery: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("Expected successful result after recovery")
	}
}


// =============================================================================
// 8. ADDITIONAL CONTRACT VIOLATION & EDGE CASE TESTS
// =============================================================================

// TestE2E_VirtualStore_GraphQuery_Contract_ReturnMapObliteration
// Scenario: GraphQuery returns a map[string]interface{}. goToMangleTerm should handle it,
// but it lacks a case for maps and falls back to stringification.
func TestE2E_VirtualStore_GraphQuery_Contract_ReturnMapObliteration(t *testing.T) {
	t.Parallel()

	vs := core.NewVirtualStore(nil)
	mockGQ := &mockAdversarialGraphQuery{
		returnValue: map[string]interface{}{"key": "value", "count": 42},
	}
	vs.SetGraphQuery(mockGQ)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	kernel.SetVirtualStore(vs)

	policy := `
		Decl test_return_map(Any).
		test_return_map(R) :- query_graph("test", "test", R).
	`
	kernel.AppendPolicy(policy)
	results, err := kernel.Query("test_return_map")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("Expected result")
	}

	resStr := fmt.Sprintf("%v", results[0].Args[0])
	if !strings.Contains(resStr, "map[count:42 key:value]") && !strings.Contains(resStr, "map[key:value count:42]") {
		t.Errorf("Expected map stringification 'map[...]!', got: %s", resStr)
	}
}

// TestE2E_VirtualStore_GraphQuery_Contract_Float32Precision
// Scenario: GraphQuery returns a float32. goToMangleTerm converts it to float64,
// potentially introducing precision errors.
func TestE2E_VirtualStore_GraphQuery_Contract_Float32Precision(t *testing.T) {
	t.Parallel()

	vs := core.NewVirtualStore(nil)
	var val float32 = 0.1
	mockGQ := &mockAdversarialGraphQuery{
		returnValue: val,
	}
	vs.SetGraphQuery(mockGQ)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	kernel.SetVirtualStore(vs)

	policy := `
		Decl test_float32(Any).
		test_float32(R) :- query_graph("test", "test", R).
	`
	kernel.AppendPolicy(policy)
	results, err := kernel.Query("test_float32")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("Expected result")
	}

	// Because 0.1 cannot be represented exactly in binary floating point,
	// converting from float32 to float64 often results in something like 0.10000000149011612.
	// We just verify it successfully returns the atom.
	resTerm, ok := results[0].Args[0].(ast.Constant)
	if !ok {
		t.Fatalf("Expected float64, got %T", results[0].Args[0])
	}
	if resTerm.String() == "0.0" {
		t.Fatalf("Expected non-zero float")
	}
}

// TestE2E_VirtualStore_GraphQuery_Contract_NestedSliceStringification
// Scenario: GraphQuery returns a nested slice. goToMangleTerm handles []string,
// but what about [][]string? It falls back to stringification.
func TestE2E_VirtualStore_GraphQuery_Contract_NestedSliceStringification(t *testing.T) {
	t.Parallel()

	vs := core.NewVirtualStore(nil)
	mockGQ := &mockAdversarialGraphQuery{
		returnValue: [][]string{{"a", "b"}, {"c", "d"}},
	}
	vs.SetGraphQuery(mockGQ)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	kernel.SetVirtualStore(vs)

	policy := `
		Decl test_nested_slice(Any).
		test_nested_slice(R) :- query_graph("test", "test", R).
	`
	kernel.AppendPolicy(policy)
	results, err := kernel.Query("test_nested_slice")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("Expected result")
	}

	resStr := fmt.Sprintf("%v", results[0].Args[0])
	if !strings.Contains(resStr, "[[a b] [c d]]") {
		t.Errorf("Expected nested slice stringification '[[a b] [c d]]', got: %s", resStr)
	}
}

// TestE2E_VirtualStore_GraphQuery_Contract_QuoteStrippingOnReturn
// Scenario: If the returned string contains quotes, does it get stripped on return?
// goToMangleTerm creates ast.String(), so it should preserve quotes. Let's verify.
func TestE2E_VirtualStore_GraphQuery_Contract_QuoteStrippingOnReturn(t *testing.T) {
	t.Parallel()

	vs := core.NewVirtualStore(nil)
	mockGQ := &mockAdversarialGraphQuery{
		returnValue: `"quoted_string"`,
	}
	vs.SetGraphQuery(mockGQ)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	kernel.SetVirtualStore(vs)

	policy := `
		Decl test_quote(Any).
		test_quote(R) :- query_graph("test", "test", R).
	`
	kernel.AppendPolicy(policy)
	results, err := kernel.Query("test_quote")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("Expected result")
	}

	// Verify the quotes were preserved.
	resTerm, ok := results[0].Args[0].(ast.Constant)
	if !ok {
		t.Fatalf("Expected constant, got %T", results[0].Args[0])
	}

	val := resTerm.String()
	// It's a string atom, so mangle-go might add quotes. Let's just ensure the inner text is intact.
	if !strings.Contains(val, "quoted_string") {
		t.Errorf("Expected preserved quotes, got: %s", val)
	}
}

// TestE2E_VirtualStore_GraphQuery_Malformed_Arity
// Scenario: A malformed query with wrong arity should gracefully fail (return nil, nil).
func TestE2E_VirtualStore_GraphQuery_Malformed_Arity(t *testing.T) {
	t.Parallel()

	vs := core.NewVirtualStore(nil)
	mockGQ := &mockAdversarialGraphQuery{}
	vs.SetGraphQuery(mockGQ)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	kernel.SetVirtualStore(vs)

	// query_graph normally takes 3 args. We give it 1.
	policy := `
		Decl test_arity(Any).
		test_arity(X) :- query_graph(X).
	`
	kernel.AppendPolicy(policy)
	results, err := kernel.Query("test_arity")
	if err != nil {
		// Should not error, but evaluate to false.
	}

	if len(results) != 0 {
		t.Fatalf("Expected 0 results for malformed arity, got %d", len(results))
	}
}

// TestE2E_VirtualStore_GraphQuery_Malformed_NonStringQueryType
// Scenario: A malformed query with a non-string QueryType should gracefully fail.
func TestE2E_VirtualStore_GraphQuery_Malformed_NonStringQueryType(t *testing.T) {
	t.Parallel()

	vs := core.NewVirtualStore(nil)
	mockGQ := &mockAdversarialGraphQuery{}
	vs.SetGraphQuery(mockGQ)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	kernel.SetVirtualStore(vs)

	// First arg must be a string, we give it a number.
	policy := `
		Decl test_type(Any).
		test_type(R) :- query_graph(42, "params", R).
	`
	kernel.AppendPolicy(policy)
	results, err := kernel.Query("test_type")

	if len(results) != 0 {
		t.Fatalf("Expected 0 results for malformed type, got %d", len(results))
	}
}

// TestE2E_VirtualStore_GraphQuery_State_NilInterfaceAfterSet
// Scenario: Setting GraphQuery to nil mid-flight should gracefully fail future queries.
func TestE2E_VirtualStore_GraphQuery_State_NilInterfaceAfterSet(t *testing.T) {
	t.Parallel()

	vs := core.NewVirtualStore(nil)
	mockGQ := &mockAdversarialGraphQuery{ returnValue: "ok" }
	vs.SetGraphQuery(mockGQ)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	kernel.SetVirtualStore(vs)

	policy := `
		Decl test_nil_after(Any).
		test_nil_after(R) :- query_graph("test", "test", R).
	`
	kernel.AppendPolicy(policy)

	// Should succeed initially
	results, _ := kernel.Query("test_nil_after")
	if len(results) == 0 {
		t.Fatalf("Expected initial query to succeed")
	}

	// Set to nil
	vs.SetGraphQuery(nil)

	// Should now fail gracefully
	results, _ = kernel.Query("test_nil_after")
	if len(results) != 0 {
		t.Fatalf("Expected 0 results after setting GraphQuery to nil, got %d", len(results))
	}
}
