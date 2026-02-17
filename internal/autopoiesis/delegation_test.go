package autopoiesis

import (
	"context"
	"testing"

	"codenerd/internal/types"
)

func TestProcessKernelDelegations_Success(t *testing.T) {
	orch, kernel, _ := createTestOrchestrator(t)
	mockToolSynth := replaceOuroborosWithMock(orch)

	// Setup: Kernel returns pending delegation facts
	kernel.QueryPredicateFunc = func(predicate string) ([]types.KernelFact, error) {
		if predicate == "delegate_task" {
			return []types.KernelFact{
				{Predicate: "delegate_task", Args: []interface{}{"/tool_generator", "my_new_tool", "/pending"}},
			}, nil
		}
		return nil, nil
	}

	// Setup: Mock tool generation success
	mockToolSynth.ExecuteFunc = func(ctx context.Context, need *ToolNeed) *LoopResult {
		if need.Name != "my_new_tool" {
			t.Errorf("Expected tool name 'my_new_tool', got '%s'", need.Name)
		}
		return &LoopResult{
			Success: true,
			ToolHandle: &RuntimeTool{
				Name: "my_new_tool",
			},
		}
	}

	// Setup: Mock tool checking (not exists)
	mockToolSynth.ListToolsFunc = func() []types.ToolInfo {
		return []types.ToolInfo{} // Empty list
	}

	// Mock SetLearningsContext
	mockToolSynth.SetLearningsContextFunc = func(ctx string) {}

	// Execute
	count, err := orch.ProcessKernelDelegations(context.Background())

	// Verify
	if err != nil {
		t.Fatalf("ProcessKernelDelegations failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 tool generated, got %d", count)
	}

	// Verify assertions to kernel (success and delegation complete)
	// We expect:
	// 1. tool_registered facts (via assertToolRegistered)
	// 2. tool_delegation_complete (via generateToolFromDelegation)
	foundComplete := false
	for _, f := range kernel.AssertedFacts {
		if f.Predicate == "tool_delegation_complete" {
			foundComplete = true
			if len(f.Args) < 2 || f.Args[0] != "my_new_tool" {
				t.Errorf("tool_delegation_complete args mismatch: %v", f.Args)
			}
		}
	}
	if !foundComplete {
		t.Error("Expected tool_delegation_complete fact")
	}
}

func TestProcessKernelDelegations_GenerationFailure(t *testing.T) {
	orch, kernel, _ := createTestOrchestrator(t)
	mockToolSynth := replaceOuroborosWithMock(orch)

	kernel.QueryPredicateFunc = func(predicate string) ([]types.KernelFact, error) {
		if predicate == "delegate_task" {
			return []types.KernelFact{
				{Predicate: "delegate_task", Args: []interface{}{"/tool_generator", "fail_tool", "/pending"}},
			}, nil
		}
		return nil, nil
	}

	mockToolSynth.ExecuteFunc = func(ctx context.Context, need *ToolNeed) *LoopResult {
		return &LoopResult{
			Success: false,
			Error:   "generation failed intentionally",
		}
	}

	mockToolSynth.SetLearningsContextFunc = func(ctx string) {}

	// ProcessKernelDelegations swallows errors for individual tools but logs them
	// It returns error only if query fails.
	count, err := orch.ProcessKernelDelegations(context.Background())

	if err != nil {
		t.Fatalf("ProcessKernelDelegations returned error: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 tools generated, got %d", count)
	}

	// Verify failure fact asserted
	foundFailure := false
	for _, f := range kernel.AssertedFacts {
		if f.Predicate == "tool_generation_failed" {
			if len(f.Args) > 0 && f.Args[0] == "fail_tool" {
				foundFailure = true
			}
		}
	}
	if !foundFailure {
		t.Error("Expected tool_generation_failed fact")
	}
}
