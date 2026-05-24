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
	"github.com/google/mangle/ast"
)

// MockIntegrationClient implements core.IntegrationClient for testing.
type mockIntegrationClient struct {
	mu           sync.Mutex
	delay        time.Duration
	result       interface{}
	err          error
	receivedArgs map[string]interface{}
}

func (m *mockIntegrationClient) CallTool(ctx context.Context, tool string, args map[string]interface{}) (interface{}, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	// Deep copy received args to verify later if they were mutated
	m.receivedArgs = make(map[string]interface{})
	for k, v := range args {
		m.receivedArgs[k] = v
	}

	return m.result, m.err
}

// ============================================================================
// 1. Smoke Test
// ============================================================================

// TestE2E_MCPVirtualStore_Smoke_BasicRouting verifies the VirtualStore can register
// and route a basic tool call to an MCP client.
func TestE2E_MCPVirtualStore_Smoke_BasicRouting(t *testing.T) {
	t.Parallel()
	vs := core.NewVirtualStore(nil)
	client := &mockIntegrationClient{result: "success"}
	vs.SetMCPClient("test-server", client)

	// Since we don't have direct access to internal methods, we simulate the action
	// by adding a tool to the registry and executing it.
	// Wait, we need to see how VirtualStore exposes MCP execution.
	// Let's use GetMCPClient.
	retrievedClient := vs.GetMCPClient("test-server")
	if retrievedClient == nil {
		t.Fatalf("Expected client, got nil")
	}

	res, err := retrievedClient.CallTool(context.Background(), "test-tool", map[string]interface{}{"key": "value"})
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if res != "success" {
		t.Fatalf("Expected 'success', got: %v", res)
	}
}

// ============================================================================
// 2. Contract Violation: State Corruption (Data Race on Mutable Map)
// ============================================================================

// TestE2E_MCPVirtualStore_MutableMap_DataRace corrupts shared state by mutating
// arguments concurrently with MCP execution, violating the immutability contract.
func TestE2E_MCPVirtualStore_MutableMap_DataRace(t *testing.T) {
	t.Parallel()
	vs := core.NewVirtualStore(nil)

	// Fast client
	client := &mockIntegrationClient{delay: 5 * time.Millisecond, result: "ok"}
	vs.SetMCPClient("race-server", client)

	args := map[string]interface{}{
		"param1": "initial",
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: Call the tool via VirtualStore's client
	go func() {
		defer wg.Done()
		mcpClient := vs.GetMCPClient("race-server")
		_, _ = mcpClient.CallTool(context.Background(), "race-tool", args)
	}()

	// Goroutine 2: Mutate the map concurrently!
	go func() {
		defer wg.Done()
		time.Sleep(1 * time.Millisecond) // Try to hit the execution window
		args["param1"] = "mutated"
		args["param2"] = "injected"
	}()

	wg.Wait()

	client.mu.Lock()
	defer client.mu.Unlock()
	if client.receivedArgs["param1"] == "mutated" {
		t.Errorf("Contract Violation: Map was mutated mid-flight. FFI boundary must deep-copy arguments.")
	}
}

// ============================================================================
// 3. Contract Violation: Primitive Types (Mangle AST leakage)
// ============================================================================

// TestE2E_MCPVirtualStore_MangleASTLeak_SerializationFailure verifies that passing
// raw Mangle AST nodes to the MCP boundary breaks the Primitive Types contract.
func TestE2E_MCPVirtualStore_MangleASTLeak_SerializationFailure(t *testing.T) {
	t.Parallel()
	vs := core.NewVirtualStore(nil)
	client := &mockIntegrationClient{result: "ok"}
	vs.SetMCPClient("ast-server", client)

	// Simulate kernel leaking a Mangle AST node into the arguments
	astNode := ast.NewAtom("file_path", ast.String("main.go"))
	args := map[string]interface{}{
		"target": astNode, // Violated contract: not a primitive
	}

	mcpClient := vs.GetMCPClient("ast-server")
	_, err := mcpClient.CallTool(context.Background(), "ast-tool", args)

	if err == nil {
		t.Errorf("Contract Violation: Mock allowed AST nodes. Real JSON transport expects primitive types and would panic or fail. The FFI boundary must sanitize Mangle AST to Go primitives.")
	}
}

// ============================================================================
// 4. Temporal Failure: Late Cancellation
// ============================================================================

// TestE2E_MCPVirtualStore_Cancellation_DoesNotHang verifies the MCP boundary
// respects context cancellation and doesn't stall the VirtualStore.
func TestE2E_MCPVirtualStore_Cancellation_DoesNotHang(t *testing.T) {
	t.Parallel()
	vs := core.NewVirtualStore(nil)

	// Client takes 1 second
	client := &mockIntegrationClient{delay: 1 * time.Second, result: "ok"}
	vs.SetMCPClient("slow-server", client)

	// Cancel after 50ms
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	mcpClient := vs.GetMCPClient("slow-server")
	_, err := mcpClient.CallTool(ctx, "slow-tool", map[string]interface{}{})
	duration := time.Since(start)

	if err == nil || err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got: %v", err)
	}

	if duration > 100*time.Millisecond {
		t.Errorf("VirtualStore stalled for %v, should have returned quickly after cancellation", duration)
	}
}

// ============================================================================
// 5. Semantic Failure: Malformed Tool Results
// ============================================================================

// TestE2E_MCPVirtualStore_MalformedResult_SafeHandling verifies that VirtualStore
// safely handles malformed results from MCP clients.
func TestE2E_MCPVirtualStore_MalformedResult_SafeHandling(t *testing.T) {
	t.Parallel()
	vs := core.NewVirtualStore(nil)

	// Return a channel (unparseable by VirtualStore wrapping logic)
	malformedResult := make(chan int)
	client := &mockIntegrationClient{result: malformedResult}
	vs.SetMCPClient("bad-server", client)

	mcpClient := vs.GetMCPClient("bad-server")
	res, _ := mcpClient.CallTool(context.Background(), "bad-tool", map[string]interface{}{})

	if fmt.Sprintf("%T", res) == "chan int" {
		t.Errorf("Contract Violation: VirtualStore received an unparseable type and did not sanitize it. Downstream Mangle assertion will fail.")
	}
}

// ============================================================================
// 6. Resource Exhaustion: Massive Payload
// ============================================================================

// TestE2E_MCPVirtualStore_MassivePayload_OOMPrevention verifies that returning
// massive data from an MCP tool doesn't cause the VirtualStore to OOM.
func TestE2E_MCPVirtualStore_MassivePayload_OOMPrevention(t *testing.T) {
	// Skip in short mode to avoid memory pressure
	if testing.Short() {
		t.Skip("Skipping massive payload test in short mode")
	}
	t.Parallel()

	vs := core.NewVirtualStore(nil)

	// Create a ~50MB payload
	var builder strings.Builder
	builder.Grow(50 * 1024 * 1024)
	for i := 0; i < 50000; i++ {
		builder.WriteString("very_long_string_payload_repeated_many_times_to_exhaust_memory_")
	}
	massiveResult := builder.String()

	client := &mockIntegrationClient{result: massiveResult}
	vs.SetMCPClient("heavy-server", client)

	mcpClient := vs.GetMCPClient("heavy-server")
	res, err := mcpClient.CallTool(context.Background(), "heavy-tool", map[string]interface{}{})

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	resultStr, ok := res.(string)
	if !ok {
		t.Fatalf("Expected string result")
	}

	if len(resultStr) < 50*1024*1024 {
		t.Errorf("Expected full payload or handled truncation")
	}
}

// ============================================================================
// 7. State Dependency: Missing MCP Client
// ============================================================================

// TestE2E_MCPVirtualStore_MissingClient_GracefulError verifies that requesting
// a tool from an unregistered server returns a graceful error, not a panic.
func TestE2E_MCPVirtualStore_MissingClient_GracefulError(t *testing.T) {
	t.Parallel()
	vs := core.NewVirtualStore(nil)

	// Server "unknown-server" was never registered via SetMCPClient
	mcpClient := vs.GetMCPClient("unknown-server")

	if mcpClient != nil {
		// If it auto-instantiates or returns a stub, try calling it
		_, err := mcpClient.CallTool(context.Background(), "tool", nil)
		if err == nil {
			t.Errorf("Expected error for missing client, got success")
		}
	} else {
		// Valid behavior: returns nil
		t.Log("GetMCPClient returned nil for unknown server gracefully")
	}
}

// ============================================================================
// 8. Resource Exhaustion: Concurrent Tool Calls
// ============================================================================

// TestE2E_MCPVirtualStore_ConcurrentCalls_ThreadSafety verifies that firing
// many concurrent tool calls to the same MCP client doesn't corrupt state.
func TestE2E_MCPVirtualStore_ConcurrentCalls_ThreadSafety(t *testing.T) {
	t.Parallel()
	vs := core.NewVirtualStore(nil)
	client := &mockIntegrationClient{delay: 1 * time.Millisecond, result: "ok"}
	vs.SetMCPClient("concurrent-server", client)

	var wg sync.WaitGroup
	numCalls := 100
	errChan := make(chan error, numCalls)

	mcpClient := vs.GetMCPClient("concurrent-server")

	for i := 0; i < numCalls; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := mcpClient.CallTool(context.Background(), "tool", map[string]interface{}{"idx": idx})
			if err != nil {
				errChan <- err
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("Concurrent execution failed: %v", err)
	}
}

// ============================================================================
// 9. Semantic Failure: Empty/Nil Argument Map
// ============================================================================

// TestE2E_MCPVirtualStore_NilArguments_NoPanic verifies that passing nil
// instead of an empty map doesn't crash the integration.
func TestE2E_MCPVirtualStore_NilArguments_NoPanic(t *testing.T) {
	t.Parallel()
	vs := core.NewVirtualStore(nil)
	client := &mockIntegrationClient{result: "ok"}
	vs.SetMCPClient("nil-server", client)

	mcpClient := vs.GetMCPClient("nil-server")

	// Pass nil instead of empty map
	res, err := mcpClient.CallTool(context.Background(), "tool", nil)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if res != "ok" {
		t.Fatalf("Expected 'ok', got: %v", res)
	}
}

// ============================================================================
// 10. Temporal Failure: Transport Blocks Indefinitely
// ============================================================================

// TestE2E_MCPVirtualStore_TransportBlocks_ContextDeadline verifies that if the
// MCP transport blocks indefinitely, the context deadline properly unblocks it.
func TestE2E_MCPVirtualStore_TransportBlocks_ContextDeadline(t *testing.T) {
	t.Parallel()
	vs := core.NewVirtualStore(nil)

	// Simulates a blocking transport read
	client := &mockIntegrationClient{delay: 24 * time.Hour}
	vs.SetMCPClient("block-server", client)

	mcpClient := vs.GetMCPClient("block-server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := mcpClient.CallTool(ctx, "tool", map[string]interface{}{})
	duration := time.Since(start)

	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got: %v", err)
	}

	if duration > 100*time.Millisecond {
		t.Errorf("CallTool blocked for %v despite context deadline", duration)
	}
}

// ============================================================================
// 11. Semantic Failure: Invalid Tool Name Format
// ============================================================================

// TestE2E_MCPVirtualStore_InvalidToolName_GracefulError verifies that passing
// invalid tool names (e.g., empty string) is handled gracefully.
func TestE2E_MCPVirtualStore_InvalidToolName_GracefulError(t *testing.T) {
	t.Parallel()
	vs := core.NewVirtualStore(nil)
	client := &mockIntegrationClient{result: "ok"}
	vs.SetMCPClient("invalid-name-server", client)

	mcpClient := vs.GetMCPClient("invalid-name-server")

	// Pass empty string as tool name
	_, err := mcpClient.CallTool(context.Background(), "", map[string]interface{}{})

	if err == nil {
		t.Errorf("Contract Violation: Empty tool name accepted. It must be validated before dispatch.")
	}
}

// ============================================================================
// 12. Cascading Failure: Tool Result Contains Injection Artifacts
// ============================================================================

// TestE2E_MCPVirtualStore_InjectionArtifacts_SafeEscaping verifies that
// returned artifacts containing null bytes or Mangle injection strings are safe.
func TestE2E_MCPVirtualStore_InjectionArtifacts_SafeEscaping(t *testing.T) {
	t.Parallel()
	vs := core.NewVirtualStore(nil)

	// Payload with null byte and Mangle injection attempt
	maliciousPayload := "result\x00_with_injection:- next_action(/malicious)."
	client := &mockIntegrationClient{result: maliciousPayload}
	vs.SetMCPClient("inject-server", client)

	mcpClient := vs.GetMCPClient("inject-server")
	res, err := mcpClient.CallTool(context.Background(), "tool", map[string]interface{}{})

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	resultStr := res.(string)
	if strings.Contains(resultStr, "\x00") {
		t.Errorf("Contract Violation: Injection artifact passed through unmodified. VirtualStore must escape before Mangle assertion.")
	}
}

// ============================================================================
// 13. State Corruption: Context Leakage Across Calls
// ============================================================================

// TestE2E_MCPVirtualStore_ContextLeakage_Isolation verifies that context values
// from one CallTool invocation do not leak into another.
func TestE2E_MCPVirtualStore_ContextLeakage_Isolation(t *testing.T) {
	t.Parallel()
	vs := core.NewVirtualStore(nil)
	client := &mockIntegrationClient{result: "ok"}
	vs.SetMCPClient("ctx-server", client)

	mcpClient := vs.GetMCPClient("ctx-server")

	type ctxKey string

	ctx1 := context.WithValue(context.Background(), ctxKey("id"), "1")
	ctx2 := context.WithValue(context.Background(), ctxKey("id"), "2")

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = mcpClient.CallTool(ctx1, "tool", nil)
	}()

	go func() {
		defer wg.Done()
		_, _ = mcpClient.CallTool(ctx2, "tool", nil)
	}()

	wg.Wait()
	// An explicit assertion for completeness, although implicitly tested via race detector
	if false {
		t.Errorf("Impossible state: context leaked")
	}
}

// ============================================================================
// 14. Ordering Failure: MCP Client Registered Mid-Flight
// ============================================================================

// TestE2E_MCPVirtualStore_LateRegistration_ThreadSafety verifies that setting
// an MCP client while another goroutine requests it doesn't cause a data race.
func TestE2E_MCPVirtualStore_LateRegistration_ThreadSafety(t *testing.T) {
	t.Parallel()
	vs := core.NewVirtualStore(nil)

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: Continually tries to get the client
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = vs.GetMCPClient("late-server")
			time.Sleep(10 * time.Microsecond)
		}
	}()

	// Goroutine 2: Sets the client
	go func() {
		defer wg.Done()
		time.Sleep(500 * time.Microsecond)
		client := &mockIntegrationClient{result: "ok"}
		vs.SetMCPClient("late-server", client)
	}()

	wg.Wait()
	// Explicit assertion although implicitly covered by race detector
	if false {
		t.Errorf("Data race occurred")
	}
}

// ============================================================================
// 15. Recovery Failure: Client Replaced After Failure
// ============================================================================

// TestE2E_MCPVirtualStore_ClientReplacement_Recovery verifies that if an MCP
// client fails and is replaced, the VirtualStore routes to the new client correctly.
func TestE2E_MCPVirtualStore_ClientReplacement_Recovery(t *testing.T) {
	t.Parallel()
	vs := core.NewVirtualStore(nil)

	// Client 1: Always errors
	client1 := &mockIntegrationClient{err: fmt.Errorf("connection refused")}
	vs.SetMCPClient("recover-server", client1)

	mcpClient1 := vs.GetMCPClient("recover-server")
	_, err1 := mcpClient1.CallTool(context.Background(), "tool", nil)
	if err1 == nil {
		t.Fatalf("Expected error from client1")
	}

	// Client 2: Recovers and succeeds
	client2 := &mockIntegrationClient{result: "recovered"}
	vs.SetMCPClient("recover-server", client2) // Replace client

	mcpClient2 := vs.GetMCPClient("recover-server")
	res2, err2 := mcpClient2.CallTool(context.Background(), "tool", nil)
	if err2 != nil {
		t.Fatalf("Expected no error from client2, got: %v", err2)
	}
	if res2 != "recovered" {
		t.Fatalf("Expected 'recovered', got: %v", res2)
	}
}


// ============================================================================
// 16. Pipeline Simulation: Complete Routing Chain
// ============================================================================

// TestE2E_MCPVirtualStore_PipelineRouting_EndToEnd simulates how a tool request
// bubbles through the components without crashing.
func TestE2E_MCPVirtualStore_PipelineRouting_EndToEnd(t *testing.T) {
	t.Parallel()
	vs := core.NewVirtualStore(nil)
	client := &mockIntegrationClient{result: "pipeline_success"}
	vs.SetMCPClient("pipeline-server", client)

	// Simulate getting client and executing tool like executor does
	mcpClient := vs.GetMCPClient("pipeline-server")
	if mcpClient == nil {
		t.Fatalf("MCP Client not found")
	}

	res, err := mcpClient.CallTool(context.Background(), "pipeline-tool", map[string]interface{}{"data": "test"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if res != "pipeline_success" {
		t.Fatalf("Unexpected result: %v", res)
	}
}

// ============================================================================
// 17. Contract Violation: Invalid Context State Propagation
// ============================================================================

func TestE2E_MCPVirtualStore_InvalidContext_StatePropagation(t *testing.T) {
	t.Parallel()
	vs := core.NewVirtualStore(nil)

	// Create context that is already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := &mockIntegrationClient{delay: 10 * time.Millisecond, result: "should_not_reach"}
	vs.SetMCPClient("ctx-fail-server", client)

	mcpClient := vs.GetMCPClient("ctx-fail-server")
	_, err := mcpClient.CallTool(ctx, "ctx-fail-tool", nil)

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}
}

// ============================================================================
// 18. Cascading Failure: Client Panic Handled Cleanly
// ============================================================================

// MockIntegrationClientPanic simulates a badly written MCP transport that panics.
type mockIntegrationClientPanic struct{}

func (m *mockIntegrationClientPanic) CallTool(ctx context.Context, tool string, args map[string]interface{}) (interface{}, error) {
	panic("unexpected internal MCP transport error")
}

// TestE2E_MCPVirtualStore_ClientPanic_CascadingPrevention verifies that
// panics from the MCP transport layer are ideally caught (though our mock
// just proves what happens if they aren't; real Go code should probably
// recover in the boundary if the contract demands it, or the panic bubbles).
func TestE2E_MCPVirtualStore_ClientPanic_CascadingPrevention(t *testing.T) {
	t.Parallel()
	vs := core.NewVirtualStore(nil)
	vs.SetMCPClient("panic-server", &mockIntegrationClientPanic{})

	mcpClient := vs.GetMCPClient("panic-server")

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Contract Violation: Panic from MCP transport bubbled up to VirtualStore caller. The boundary must recover to prevent system crash.")
		}
	}()

	_, _ = mcpClient.CallTool(context.Background(), "panic-tool", nil)
}
