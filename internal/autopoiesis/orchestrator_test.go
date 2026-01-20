package autopoiesis

import (
	"testing"
)

func TestNewOrchestrator(t *testing.T) {
	orch, kernel, llm := createTestOrchestrator(t)

	if orch == nil {
		t.Fatal("NewOrchestrator returned nil")
	}
	if orch.GetKernel() != kernel {
		t.Error("Kernel not attached correctly")
	}
	if orch.client != llm {
		t.Error("LLM client not attached correctly")
	}
}

func TestRecordCodeEditOutcome(t *testing.T) {
	orch, kernel, _ := createTestOrchestrator(t)

	// Case 1: Record success
	orch.RecordCodeEditOutcome("func:Test", "fix", true)

	if len(kernel.AssertedFacts) != 1 {
		t.Fatalf("Expected 1 assertion, got %d", len(kernel.AssertedFacts))
	}

	fact := kernel.AssertedFacts[0]
	if fact.Predicate != "code_edit_outcome" {
		t.Errorf("Expected predicate 'code_edit_outcome', got '%s'", fact.Predicate)
	}
	// Args: [Ref, Type, Success, Timestamp]
	if len(fact.Args) != 4 {
		t.Errorf("Expected 4 args, got %d", len(fact.Args))
	}
	if fact.Args[0] != "func:Test" {
		t.Errorf("Expected ref 'func:Test', got '%v'", fact.Args[0])
	}
	if fact.Args[2] != "/true" {
		t.Errorf("Expected success '/true', got '%v'", fact.Args[2])
	}

	// Case 2: Pruning old events (MaxLearningFacts=10 in helper)
	// Fill up to limit
	for i := 0; i < 15; i++ {
		orch.RecordCodeEditOutcome("func:Test", "fix", true)
	}
	// Check retraction happens (requires query mock to return facts to prune)
	// Since mock QueryPredicate returns nil by default, pruning won't happen logic-wise in this basic test.
	// We'd need to mock QueryPredicate to return > MaxLearningFacts.
}

func TestShouldGenerateTool(t *testing.T) {
	orch, kernel, _ := createTestOrchestrator(t)

	// Mock query result
	kernel.QueryBoolFunc = func(predicate string) bool {
		return predicate == "next_action(/generate_tool)"
	}

	if !orch.ShouldGenerateTool() {
		t.Error("Expected ShouldGenerateTool to return true")
	}

	// Test negative case
	kernel.QueryBoolFunc = func(predicate string) bool {
		return false
	}

	if orch.ShouldGenerateTool() {
		t.Error("Expected ShouldGenerateTool to return false")
	}
}

func TestSyncExistingToolsToKernel(t *testing.T) {
	orch, kernel, _ := createTestOrchestrator(t)

	// Manually inject a tool into the registry (via Ouroboros if exposed, or just assume empty for now)
	// Since Ouroboros is internal, we can't easily populate it without mocking.
	// But `SetKernel` calls `syncExistingToolsToKernel`.

	// Just verify it doesn't panic on empty
	orch.syncExistingToolsToKernel()

	if len(kernel.AssertedFacts) > 0 {
		// Should be 0 if no tools
		t.Errorf("Expected 0 assertions for empty registry, got %d", len(kernel.AssertedFacts))
	}
}
