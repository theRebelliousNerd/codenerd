package autopoiesis

import (
	"context"
	"testing"
	"time"
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

func TestRecordCodeEditOutcome_NoKernel(t *testing.T) {
	cfg := Config{
		ToolsDir:         t.TempDir(),
		AgentsDir:        t.TempDir(),
		WorkspaceRoot:    t.TempDir(),
		MinConfidence:    0.6,
		MaxLearningFacts: 10,
	}
	orch := NewOrchestrator(&MockLLMClient{}, cfg)

	// Should be a no-op when kernel is not attached.
	orch.RecordCodeEditOutcome("func:NoKernel", "fix", true)
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

func TestRecordExecution_RefreshesLearningsContext(t *testing.T) {
	orch, kernel, _ := createTestOrchestrator(t)
	mockToolSynth := replaceOuroborosWithMock(orch)

	var ouroborosContext string
	mockToolSynth.SetLearningsContextFunc = func(ctx string) {
		ouroborosContext = ctx
	}

	feedback := &ExecutionFeedback{
		ToolName:   "test_tool",
		Timestamp:  time.Now(),
		Input:      `{"query":"status"}`,
		Output:     "ok",
		OutputSize: 2,
		Success:    true,
		Quality: &QualityAssessment{
			Score: 0.95,
		},
	}

	orch.RecordExecution(context.Background(), feedback)

	if orch.toolGen.learningsContext == "" {
		t.Fatal("expected tool generator learnings context to be refreshed")
	}
	if ouroborosContext == "" {
		t.Fatal("expected ouroboros learnings context to be refreshed")
	}

	foundLearningFact := false
	for _, fact := range kernel.AssertedFacts {
		if fact.Predicate == "tool_learning" {
			foundLearningFact = true
			if got, ok := fact.Args[3].(float64); !ok || got != 95 {
				t.Fatalf("expected avg quality to be normalized to 95, got %#v", fact.Args[3])
			}
			break
		}
	}
	if !foundLearningFact {
		t.Fatal("expected tool_learning fact to be asserted to kernel")
	}
}

func TestExecuteOuroborosLoop_RefreshesLearningsAfterGeneration(t *testing.T) {
	orch, kernel, _ := createTestOrchestrator(t)
	mockToolSynth := replaceOuroborosWithMock(orch)

	var ouroborosContext string
	mockToolSynth.SetLearningsContextFunc = func(ctx string) {
		if ctx != "" {
			ouroborosContext = ctx
		}
	}
	mockToolSynth.ExecuteFunc = func(ctx context.Context, need *ToolNeed) *LoopResult {
		return &LoopResult{
			Success:  false,
			ToolName: need.Name,
			Stage:    StageSafetyCheck,
			Error:    "blocked by safety policy",
			Duration: 25 * time.Millisecond,
		}
	}

	result := orch.ExecuteOuroborosLoop(context.Background(), &ToolNeed{
		Name:    "generated_tool",
		Purpose: "Generate a test helper",
	})
	if result == nil {
		t.Fatal("expected loop result")
	}

	if orch.toolGen.learningsContext == "" {
		t.Fatal("expected tool generator learnings context to be updated after generation")
	}
	if ouroborosContext == "" {
		t.Fatal("expected ouroboros learnings context to be updated after generation")
	}

	foundLearningFact := false
	for _, fact := range kernel.AssertedFacts {
		if fact.Predicate == "tool_learning" {
			foundLearningFact = true
			break
		}
	}
	if !foundLearningFact {
		t.Fatal("expected generation learning to assert tool_learning fact")
	}
}
