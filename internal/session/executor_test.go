package session

import (
	"codenerd/internal/types"
	"fmt"
	"testing"
)

func TestExecutor_CheckSafety_ConstitutionalGate(t *testing.T) {
	// 1. Setup
	mockKernel := &MockKernel{}
	executor := &Executor{
		kernel: mockKernel,
		config: DefaultExecutorConfig(),
	}

	toolCall := ToolCall{
		ID:   "call_1",
		Name: "readFile",
		Args: map[string]interface{}{
			"path": "secret.txt",
		},
	}
	target := "secret.txt"
	payload := `{"path":"secret.txt"}`

	// 2. Case: Denied Action (No permitted fact)
	allowed := executor.checkSafety(toolCall)
	if allowed {
		t.Error("Expected action to be denied when no permitted fact exists")
	}

	// Verify pending_action was asserted
	foundPending := false
	for _, f := range mockKernel.asserts {
		if f.Predicate == "pending_action" {
			// Check args: ActionID, ActionType, Target, Payload, Timestamp
			if len(f.Args) == 5 && f.Args[0] == "call_1" {
				foundPending = true
				break
			}
		}
	}
	if !foundPending {
		t.Error("pending_action fact was not asserted")
	}

	// 3. Case: Permitted Action
	// Add permitted fact: permitted(Action, Target, Payload)
	// Action must be MangleAtom "/readFile"
	// We use string "/readFile" which matches fmt.Sprintf("%v", arg) check
	mockKernel.facts = append(mockKernel.facts, types.Fact{
		Predicate: "permitted",
		Args: []interface{}{
			"/readFile",
			target,
			payload,
		},
	})

	allowed = executor.checkSafety(toolCall)
	if !allowed {
		t.Error("Expected action to be allowed when permitted fact exists")
	}

	// 4. Case: Mismatch Target
	mockKernel.facts = []types.Fact{{
		Predicate: "permitted",
		Args: []interface{}{
			"/readFile",
			"other.txt", // Different target
			payload,
		},
	}}

	allowed = executor.checkSafety(toolCall)
	if allowed {
		t.Error("Expected action to be denied when target mismatches")
	}

	// 5. Case: No Kernel (Fail Closed when Safety Gate is enabled)
	executorNoKernel := &Executor{
		kernel: nil,
		config: DefaultExecutorConfig(),
	}
	allowed = executorNoKernel.checkSafety(toolCall)
	if allowed {
		t.Error("Expected action to be denied (fail closed) when kernel is nil and EnableSafetyGate=true")
	}
}

// TestExecutor_EmptyToolCallName tests behavior when ToolCall.Name is empty
func TestExecutor_EmptyToolCallName(t *testing.T) {
	mockKernel := &MockKernel{}
	executor := &Executor{
		kernel: mockKernel,
		config: DefaultExecutorConfig(),
	}

	toolCall := ToolCall{
		ID:   "call_1",
		Name: "", // Empty name
		Args: map[string]interface{}{"path": "test.txt"},
	}

	// Empty name should be treated as "/" atom and denied by default
	allowed := executor.checkSafety(toolCall)
	// Should not panic and should handle gracefully
	if allowed {
		t.Log("Empty tool name was allowed - may need explicit handling")
	}
}

// TestExecutor_NilArgsInToolCall tests behavior when ToolCall.Args is nil
func TestExecutor_NilArgsInToolCall(t *testing.T) {
	mockKernel := &MockKernel{}
	executor := &Executor{
		kernel: mockKernel,
		config: DefaultExecutorConfig(),
	}

	toolCall := ToolCall{
		ID:   "call_1",
		Name: "readFile",
		Args: nil, // nil args
	}

	// Should not panic with nil args
	allowed := executor.checkSafety(toolCall)
	if allowed {
		t.Log("Nil args tool call was allowed - may need explicit handling")
	}
}

// TestExecutor_ExtractTargetMultipleKeys tests extractTarget with multiple candidate keys
func TestExecutor_ExtractTargetMultipleKeys(t *testing.T) {
	executor := &Executor{
		config: DefaultExecutorConfig(),
	}

	// Test with multiple target keys - should return first match
	args := map[string]interface{}{
		"query": "SELECT * FROM users", // First in candidate order that exists
		"path":  "/home/user/file.txt", // "path" comes before "query" in candidates
	}

	target := executor.extractTarget(args)
	// "path" should be returned since it comes first in candidates list
	if target != "/home/user/file.txt" {
		t.Errorf("Expected path '/home/user/file.txt', got '%s'", target)
	}
}

// TestExecutor_ExtractTargetNoMatch tests extractTarget with no matching keys
func TestExecutor_ExtractTargetNoMatch(t *testing.T) {
	executor := &Executor{
		config: DefaultExecutorConfig(),
	}

	args := map[string]interface{}{
		"unknown_key": "some_value",
		"other_key":   123,
	}

	target := executor.extractTarget(args)
	if target != "unknown" {
		t.Errorf("Expected 'unknown', got '%s'", target)
	}
}

// TestExecutor_PermittedFactIncorrectArity tests behavior with wrong arity permitted fact
func TestExecutor_PermittedFactIncorrectArity(t *testing.T) {
	mockKernel := &MockKernel{
		facts: []types.Fact{{
			Predicate: "permitted",
			Args:      []interface{}{"/readFile", "test.txt"}, // Only 2 args instead of 3
		}},
	}
	executor := &Executor{
		kernel: mockKernel,
		config: DefaultExecutorConfig(),
	}

	toolCall := ToolCall{
		ID:   "call_1",
		Name: "readFile",
		Args: map[string]interface{}{"path": "test.txt"},
	}

	// Should not match due to incorrect arity
	allowed := executor.checkSafety(toolCall)
	if allowed {
		t.Error("Expected action to be denied with incorrect arity permitted fact")
	}
}

// TestExecutor_ArgsMarshalFailure tests behavior when Args contains types that fail json.Marshal
func TestExecutor_ArgsMarshalFailure(t *testing.T) {
	mockKernel := &MockKernel{}
	executor := &Executor{
		kernel: mockKernel,
		config: DefaultExecutorConfig(),
	}

	// Create a channel, which fails json.Marshal
	ch := make(chan int)
	toolCall := ToolCall{
		ID:   "call_marshal_fail",
		Name: "someTool",
		Args: map[string]interface{}{
			"bad_arg": ch,
		},
	}

	allowed := executor.checkSafety(toolCall)
	if allowed {
		t.Error("Expected checkSafety to return false when json.Marshal fails")
	}
}

// TestExecutor_KernelAssertFailure tests behavior when kernel.Assert fails
func TestExecutor_KernelAssertFailure(t *testing.T) {
	mockKernel := &MockKernel{
		AssertError: fmt.Errorf("assert failed"),
	}
	executor := &Executor{
		kernel: mockKernel,
		config: DefaultExecutorConfig(),
	}

	toolCall := ToolCall{
		ID:   "call_assert_fail",
		Name: "someTool",
		Args: map[string]interface{}{"p": "v"},
	}

	allowed := executor.checkSafety(toolCall)
	if allowed {
		t.Error("Expected checkSafety to return false when kernel.Assert fails")
	}
}

// TestExecutor_KernelQueryFailure tests behavior when kernel.Query fails
func TestExecutor_KernelQueryFailure(t *testing.T) {
	mockKernel := &MockKernel{
		QueryError: fmt.Errorf("query failed"),
	}
	executor := &Executor{
		kernel: mockKernel,
		config: DefaultExecutorConfig(),
	}

	toolCall := ToolCall{
		ID:   "call_query_fail",
		Name: "someTool",
		Args: map[string]interface{}{"p": "v"},
	}

	allowed := executor.checkSafety(toolCall)
	if allowed {
		t.Error("Expected checkSafety to return false when kernel.Query fails")
	}
}

// TestExecutor_RetractFactFailure tests behavior when kernel.RetractFact fails
func TestExecutor_RetractFactFailure(t *testing.T) {
	// Setup a scenario where checkSafety SHOULD succeed, but Retract fails.
	// Since Retract is in a defer, checkSafety should still return true (or whatever the main logic dictates).
	// The requirement is: "Verify that it still returns true (defer block error logging shouldn't block execution)"

	target := "test.txt"
	payload := `{"path":"test.txt"}`

	mockKernel := &MockKernel{
		RetractError: fmt.Errorf("retract failed"),
		facts: []types.Fact{{
			Predicate: "permitted",
			Args: []interface{}{
				"/readFile",
				target,
				payload,
			},
		}},
	}
	executor := &Executor{
		kernel: mockKernel,
		config: DefaultExecutorConfig(),
	}

	toolCall := ToolCall{
		ID:   "call_retract_fail",
		Name: "readFile",
		Args: map[string]interface{}{"path": "test.txt"},
	}

	allowed := executor.checkSafety(toolCall)
	if !allowed {
		t.Error("Expected checkSafety to return true even if RetractFact fails in defer")
	}
}
