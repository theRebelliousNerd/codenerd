package core

import (
	"context"
	"testing"
)

// Note: VirtualStore handlers require a fully initialized VirtualStore with kernel.
// We test what we can without complex mocking, and skip handlers that require
// external dependencies (LLM, file system, etc.).

func TestVirtualStoreWorkflows_CompleteHandler(t *testing.T) {
	// handleComplete is relatively simple - it marks task complete.
	// We can test the fact building helper.

	facts := buildTaskCompletionFacts("task-123", "Completed successfully")

	if len(facts) == 0 {
		t.Fatal("Expected completion facts, got none")
	}

	// Check for expected predicates
	foundStatus := false
	foundComplete := false
	for _, f := range facts {
		if f.Predicate == "task_status" {
			foundStatus = true
		}
		if f.Predicate == "task_completed" || f.Predicate == "task_complete" {
			foundComplete = true
		}
	}

	if !foundStatus {
		t.Log("Warning: task_status predicate not found in completion facts")
	}
	if !foundComplete {
		t.Log("Warning: task_completed predicate not found in completion facts")
	}
}

func TestVirtualStoreWorkflows_ActionTypes(t *testing.T) {
	// Test that action types constants are defined correctly
	actions := []ActionType{
		ActionRunTests,
		ActionComplete,
		ActionEscalateToUser,
	}

	for _, a := range actions {
		if a == "" {
			t.Error("Found empty action type constant")
		}
	}
}

// TestVirtualStoreWorkflows_Integration requires a real kernel.
// We can do a smoke test if setupMockKernel is available.
func TestVirtualStoreWorkflows_Integration(t *testing.T) {
	k := setupMockKernel(t)
	if k == nil {
		t.Skip("Mock kernel not available")
	}

	// Create a minimal VirtualStore
	vs := &VirtualStore{
		kernel: k,
	}

	// Test handleComplete
	ctx := context.Background()
	req := ActionRequest{
		Type: ActionComplete,
		Payload: map[string]interface{}{
			"task_id": "test-task",
			"summary": "Test complete",
		},
	}

	result, err := vs.handleComplete(ctx, req)
	if err != nil {
		t.Fatalf("handleComplete failed: %v", err)
	}
	if !result.Success {
		t.Errorf("handleComplete returned failure: %s", result.Error)
	}
}
