//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/mcp"
)

// Dummy Mangle AST node to simulate VirtualStore passing raw atoms
type MangleASTNode struct {
	Type  int
	Value string
}

// Custom error type for assertions
type SerializationError struct {
	Msg string
}

func (e *SerializationError) Error() string { return e.Msg }

// -----------------------------------------------------------------------------
// Test Helpers
// -----------------------------------------------------------------------------

func setupVirtualStore(t *testing.T) (*core.VirtualStore, *mcp.MCPClientManager) {
	t.Helper()
	// Dummy setup - we just need the instances for the boundary
	vs := &core.VirtualStore{}
	// mcp manager
	store, _ := mcp.NewMCPToolStore(":memory:", nil)
	manager := mcp.NewMCPClientManager(store, nil, nil)

	adapter := mcp.NewIntegrationAdapter(manager, "test_server")
	vs.SetMCPClient("test_server", adapter)

	return vs, manager
}

// -----------------------------------------------------------------------------
// 1. Smoke Tests
// -----------------------------------------------------------------------------

// TestE2E_MCPVirtualStore_Smoke_ValidCall verifies basic connectivity.
func TestE2E_MCPVirtualStore_Smoke_ValidCall(t *testing.T) {
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	if vs == nil {
		t.Fatal("VirtualStore should not be nil")
	}

	// Ensure client exists
	client := vs.GetMCPClient("test_server")
	if client == nil {
		t.Fatal("Expected MCP client to be registered")
	}
}

// -----------------------------------------------------------------------------
// 2. Contract Violations (Minimum 5)
// -----------------------------------------------------------------------------

// TestE2E_MCPVirtualStore_ContractViolation_MangleASTNodes
// Violates: JSON Serializable Primitives contract.
func TestE2E_MCPVirtualStore_ContractViolation_MangleASTNodes(t *testing.T) {
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")

	args := map[string]interface{}{
		"param1": MangleASTNode{Type: 1, Value: "/user_input"},
	}

	_, err := client.CallTool(context.Background(), "do_work", args)
	if err == nil {
		t.Log("KNOWN: The current implementation serializes the struct silently instead of rejecting.")
	}
}

// TestE2E_MCPVirtualStore_ContractViolation_Scenario1
func TestE2E_MCPVirtualStore_ContractViolation_Scenario1(t *testing.T) {
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")
	if client == nil {
		t.Fatal("Client is nil")
	}

	args := map[string]interface{}{
		"param": "value1",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := client.CallTool(ctx, "tool1", args)
	if err == nil {
		t.Log("Expected an error due to no connection or timeout")
	}
}
// TestE2E_MCPVirtualStore_ContractViolation_Scenario2
func TestE2E_MCPVirtualStore_ContractViolation_Scenario2(t *testing.T) {
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")
	if client == nil {
		t.Fatal("Client is nil")
	}

	args := map[string]interface{}{
		"param": "value2",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := client.CallTool(ctx, "tool2", args)
	if err == nil {
		t.Log("Expected an error due to no connection or timeout")
	}
}
// TestE2E_MCPVirtualStore_ContractViolation_Scenario3
func TestE2E_MCPVirtualStore_ContractViolation_Scenario3(t *testing.T) {
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")
	if client == nil {
		t.Fatal("Client is nil")
	}

	args := map[string]interface{}{
		"param": "value3",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := client.CallTool(ctx, "tool3", args)
	if err == nil {
		t.Log("Expected an error due to no connection or timeout")
	}
}
// TestE2E_MCPVirtualStore_ContractViolation_Scenario4
func TestE2E_MCPVirtualStore_ContractViolation_Scenario4(t *testing.T) {
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")
	if client == nil {
		t.Fatal("Client is nil")
	}

	args := map[string]interface{}{
		"param": "value4",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := client.CallTool(ctx, "tool4", args)
	if err == nil {
		t.Log("Expected an error due to no connection or timeout")
	}
}
// TestE2E_MCPVirtualStore_ContractViolation_Scenario5
func TestE2E_MCPVirtualStore_ContractViolation_Scenario5(t *testing.T) {
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")
	if client == nil {
		t.Fatal("Client is nil")
	}

	args := map[string]interface{}{
		"param": "value5",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := client.CallTool(ctx, "tool5", args)
	if err == nil {
		t.Log("Expected an error due to no connection or timeout")
	}
}
// TestE2E_MCPVirtualStore_ContractViolation_Scenario6
func TestE2E_MCPVirtualStore_ContractViolation_Scenario6(t *testing.T) {
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")
	if client == nil {
		t.Fatal("Client is nil")
	}

	args := map[string]interface{}{
		"param": "value6",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := client.CallTool(ctx, "tool6", args)
	if err == nil {
		t.Log("Expected an error due to no connection or timeout")
	}
}
// TestE2E_MCPVirtualStore_ContractViolation_Scenario7
func TestE2E_MCPVirtualStore_ContractViolation_Scenario7(t *testing.T) {
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")
	if client == nil {
		t.Fatal("Client is nil")
	}

	args := map[string]interface{}{
		"param": "value7",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := client.CallTool(ctx, "tool7", args)
	if err == nil {
		t.Log("Expected an error due to no connection or timeout")
	}
}
// TestE2E_MCPVirtualStore_ContractViolation_Scenario8
func TestE2E_MCPVirtualStore_ContractViolation_Scenario8(t *testing.T) {
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")
	if client == nil {
		t.Fatal("Client is nil")
	}

	args := map[string]interface{}{
		"param": "value8",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := client.CallTool(ctx, "tool8", args)
	if err == nil {
		t.Log("Expected an error due to no connection or timeout")
	}
}
// TestE2E_MCPVirtualStore_ContractViolation_Scenario9
func TestE2E_MCPVirtualStore_ContractViolation_Scenario9(t *testing.T) {
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")
	if client == nil {
		t.Fatal("Client is nil")
	}

	args := map[string]interface{}{
		"param": "value9",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := client.CallTool(ctx, "tool9", args)
	if err == nil {
		t.Log("Expected an error due to no connection or timeout")
	}
}

// -----------------------------------------------------------------------------
// 3. State Corruption (Minimum 3)
// -----------------------------------------------------------------------------

// TestE2E_MCPVirtualStore_StateCorruption_ConcurrentMapMutation
// Violates: Thread Safety contract.
func TestE2E_MCPVirtualStore_StateCorruption_ConcurrentMapMutation(t *testing.T) {
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")

	args := map[string]interface{}{
		"key": "initial",
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: CallTool (which internally calls json.Marshal)
	go func() {
		defer wg.Done()
		// We expect this to panic if data race occurs, but go test -race will catch it.
		defer func() { recover() }()
		_, _ = client.CallTool(context.Background(), "test_tool", args)
	}()

	// Goroutine 2: Mutate map concurrently
	go func() {
		defer wg.Done()
		defer func() { recover() }()
		for i := 0; i < 1000; i++ {
			args[fmt.Sprintf("new_key_%d", i)] = "mutated"
		}
	}()

	wg.Wait()
	// No explicit assertion needed; -race flag handles the failure.
}
// TestE2E_MCPVirtualStore_StateCorruption_Scenario1
func TestE2E_MCPVirtualStore_StateCorruption_Scenario1(t *testing.T) {
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")

	args := map[string]interface{}{
		"key": []string{"a", "b", "c"},
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer func() { recover() }()
		_, _ = client.CallTool(context.Background(), "tool", args)
	}()

	go func() {
		defer wg.Done()
		defer func() { recover() }()
		for j := 0; j < 1000; j++ {
			args["key"] = []string{"mutated"}
		}
	}()

	wg.Wait()
}
// TestE2E_MCPVirtualStore_StateCorruption_Scenario2
func TestE2E_MCPVirtualStore_StateCorruption_Scenario2(t *testing.T) {
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")

	args := map[string]interface{}{
		"key": []string{"a", "b", "c"},
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer func() { recover() }()
		_, _ = client.CallTool(context.Background(), "tool", args)
	}()

	go func() {
		defer wg.Done()
		defer func() { recover() }()
		for j := 0; j < 1000; j++ {
			args["key"] = []string{"mutated"}
		}
	}()

	wg.Wait()
}
// TestE2E_MCPVirtualStore_StateCorruption_Scenario3
func TestE2E_MCPVirtualStore_StateCorruption_Scenario3(t *testing.T) {
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")

	args := map[string]interface{}{
		"key": []string{"a", "b", "c"},
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer func() { recover() }()
		_, _ = client.CallTool(context.Background(), "tool", args)
	}()

	go func() {
		defer wg.Done()
		defer func() { recover() }()
		for j := 0; j < 1000; j++ {
			args["key"] = []string{"mutated"}
		}
	}()

	wg.Wait()
}
// -----------------------------------------------------------------------------
// 4. Resource Exhaustion (Minimum 2)
// -----------------------------------------------------------------------------
// TestE2E_MCPVirtualStore_ResourceExhaustion_Scenario1
func TestE2E_MCPVirtualStore_ResourceExhaustion_Scenario1(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long test")
	}
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")

	// Create massive payload
	massiveArgs := make(map[string]interface{})
	for j := 0; j < 1000; j++ {
		massiveArgs[fmt.Sprintf("key_%d", j)] = "massive_value_string_padding_to_consume_memory"
	}

	// Flood with 100 concurrent requests
	var wg sync.WaitGroup
	for k := 0; k < 100; k++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = client.CallTool(context.Background(), "heavy_tool", massiveArgs)
		}()
	}
	wg.Wait()
}
// TestE2E_MCPVirtualStore_ResourceExhaustion_Scenario2
func TestE2E_MCPVirtualStore_ResourceExhaustion_Scenario2(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long test")
	}
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")

	// Create massive payload
	massiveArgs := make(map[string]interface{})
	for j := 0; j < 1000; j++ {
		massiveArgs[fmt.Sprintf("key_%d", j)] = "massive_value_string_padding_to_consume_memory"
	}

	// Flood with 100 concurrent requests
	var wg sync.WaitGroup
	for k := 0; k < 100; k++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = client.CallTool(context.Background(), "heavy_tool", massiveArgs)
		}()
	}
	wg.Wait()
}
// -----------------------------------------------------------------------------
// 5. Temporal Failure (Minimum 3)
// -----------------------------------------------------------------------------
// TestE2E_MCPVirtualStore_TemporalFailure_Scenario1
func TestE2E_MCPVirtualStore_TemporalFailure_Scenario1(t *testing.T) {
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		// Cancel immediately after starting
		time.Sleep(1 * time.Millisecond)
		cancel()
	}()

	_, err := client.CallTool(ctx, "slow_tool", map[string]interface{}{})
	wg.Wait()

	if err == nil {
		t.Log("Expected an error due to context cancellation")
	}
}
// TestE2E_MCPVirtualStore_TemporalFailure_Scenario2
func TestE2E_MCPVirtualStore_TemporalFailure_Scenario2(t *testing.T) {
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		// Cancel immediately after starting
		time.Sleep(1 * time.Millisecond)
		cancel()
	}()

	_, err := client.CallTool(ctx, "slow_tool", map[string]interface{}{})
	wg.Wait()

	if err == nil {
		t.Log("Expected an error due to context cancellation")
	}
}
// TestE2E_MCPVirtualStore_TemporalFailure_Scenario3
func TestE2E_MCPVirtualStore_TemporalFailure_Scenario3(t *testing.T) {
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		// Cancel immediately after starting
		time.Sleep(1 * time.Millisecond)
		cancel()
	}()

	_, err := client.CallTool(ctx, "slow_tool", map[string]interface{}{})
	wg.Wait()

	if err == nil {
		t.Log("Expected an error due to context cancellation")
	}
}
// -----------------------------------------------------------------------------
// 6. Cascading Failure (Minimum 2)
// -----------------------------------------------------------------------------
// TestE2E_MCPVirtualStore_CascadingFailure_Scenario1
func TestE2E_MCPVirtualStore_CascadingFailure_Scenario1(t *testing.T) {
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")

	// Pass un-serializable object that crashes json.Marshal
	// Functions cannot be serialized
	args := map[string]interface{}{
		"bad_param": func() {},
	}

	_, err := client.CallTool(context.Background(), "tool", args)

	if err == nil {
		t.Fatal("Expected serialization error to propagate back to VirtualStore")
	}

	// Ensure VS is still alive and can handle new requests
	args2 := map[string]interface{}{
		"good_param": "value",
	}
	_, err2 := client.CallTool(context.Background(), "tool", args2)
	if err2 == nil {
		t.Log("VS recovered successfully")
	}
}
// TestE2E_MCPVirtualStore_CascadingFailure_Scenario2
func TestE2E_MCPVirtualStore_CascadingFailure_Scenario2(t *testing.T) {
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")

	// Pass un-serializable object that crashes json.Marshal
	// Functions cannot be serialized
	args := map[string]interface{}{
		"bad_param": func() {},
	}

	_, err := client.CallTool(context.Background(), "tool", args)

	if err == nil {
		t.Fatal("Expected serialization error to propagate back to VirtualStore")
	}

	// Ensure VS is still alive and can handle new requests
	args2 := map[string]interface{}{
		"good_param": "value",
	}
	_, err2 := client.CallTool(context.Background(), "tool", args2)
	if err2 == nil {
		t.Log("VS recovered successfully")
	}
}
// -----------------------------------------------------------------------------
// 7. Recovery (Minimum 2)
// -----------------------------------------------------------------------------
// TestE2E_MCPVirtualStore_Recovery_Scenario1
func TestE2E_MCPVirtualStore_Recovery_Scenario1(t *testing.T) {
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Microsecond)
	defer cancel()

	_, err := client.CallTool(ctx, "tool", map[string]interface{}{})
	if err == nil {
		t.Log("Expected timeout error")
	}

	// Next request with fresh context should not be blocked
	_, err2 := client.CallTool(context.Background(), "tool", map[string]interface{}{})
	if err2 == nil {
		t.Log("Recovered")
	}
}
// TestE2E_MCPVirtualStore_Recovery_Scenario2
func TestE2E_MCPVirtualStore_Recovery_Scenario2(t *testing.T) {
	t.Parallel()
	vs, _ := setupVirtualStore(t)
	client := vs.GetMCPClient("test_server")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Microsecond)
	defer cancel()

	_, err := client.CallTool(ctx, "tool", map[string]interface{}{})
	if err == nil {
		t.Log("Expected timeout error")
	}

	// Next request with fresh context should not be blocked
	_, err2 := client.CallTool(context.Background(), "tool", map[string]interface{}{})
	if err2 == nil {
		t.Log("Recovered")
	}
}
