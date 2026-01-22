package session

import (
	"codenerd/internal/types"
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

	// 5. Case: No Kernel (Fail Open)
	executorNoKernel := &Executor{
		kernel: nil,
		config: DefaultExecutorConfig(),
	}
	allowed = executorNoKernel.checkSafety(toolCall)
	if !allowed {
		t.Error("Expected action to be allowed (fail open) when kernel is nil")
	}
}

// TODO: TEST_GAP: Missing test for empty 'Name' in ToolCall (should default to "/" atom or be handled).

// TODO: TEST_GAP: Missing test for nil 'Args' in ToolCall (Verify json.Marshal handles it or fails safely).

// TODO: TEST_GAP: Verify behavior when 'Args' contains types that fail json.Marshal (e.g., functions, channels).

// TODO: TEST_GAP: Missing test for 'permitted' fact with incorrect arity (not 3 arguments).

// TODO: TEST_GAP: Verify behavior when kernel.Assert fails for 'pending_action'.

// TODO: TEST_GAP: Verify behavior when kernel.Query fails for 'permitted'.

// TODO: TEST_GAP: Missing test for ambiguity in 'extractTarget' when multiple target keys are present.

// TODO: TEST_GAP: Verify behavior when kernel.RetractFact fails (defer block error handling).
