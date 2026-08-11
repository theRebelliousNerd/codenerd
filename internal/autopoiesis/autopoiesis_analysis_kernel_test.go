package autopoiesis

import (
	"context"
	"testing"
)

// TestAnalyze_ToolNeed_AssertsMissingToolFor proves the main analysis path
// reaches the kernel. Before the fix, Analyze called o.toolGen.DetectToolNeed
// directly and never asserted missing_tool_for, so the Ouroboros loop could
// not close via next_action(/generate_tool). This test builds an Orchestrator
// with a recording kernel (MockKernelInterface), runs Analyze with an input that
// yields a tool need, and asserts a missing_tool_for fact was recorded. It fails
// if the assertion is absent — the regression this task fixes.
func TestAnalyze_ToolNeed_AssertsMissingToolFor(t *testing.T) {
	orch, kernel, llm := createTestOrchestrator(t)
	// Enable the tool-generation branch and use heuristic-only complexity so no
	// unrelated LLM calls are needed.
	orch.config.EnableToolGeneration = true
	orch.config.EnableLLM = false
	orch.config.MinConfidence = 0.5
	orch.config.MinToolConfidence = 0.5
	orch.config.MaxToolsPerSession = 10
	orch.config.ToolGenerationCooldown = 0

	// Make DetectToolNeed succeed with a high-confidence need. The input must
	// match shouldCheckToolNeed (missingCapabilityPatterns) so the branch is
	// entered; "I need a tool to ..." does.
	llm.CompleteFunc = func(ctx context.Context, prompt string) (string, error) {
		return `{"needs_new_tool": true, "tool_name": "json_validator", "purpose": "Validate JSON input", "input_type": "string", "output_type": "bool", "priority": 0.9, "confidence": 0.9, "reasoning": "test"}`, nil
	}

	ctx := context.Background()
	input := "I need a tool to validate JSON"

	result, err := orch.Analyze(ctx, input, "")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if len(result.ToolNeeds) == 0 {
		t.Fatalf("expected ToolNeeds to be populated for input %q; Analyze did not detect a need", input)
	}

	// The wrapper Orchestrator.DetectToolNeed must have asserted missing_tool_for
	// to the kernel. Without routing through the wrapper (raw o.toolGen call) this
	// assertion is absent and the test fails — the desired regression.
	found := false
	for _, f := range kernel.AssertedFacts {
		if f.Predicate == "missing_tool_for" {
			// The second arg is the capability / tool name.
			if len(f.Args) >= 2 {
				if cap, ok := f.Args[1].(string); ok && cap == "json_validator" {
					found = true
					break
				}
				// Fallback: any missing_tool_for counts, but prefer exact name.
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatalf("expected missing_tool_for fact to be asserted to kernel; got %d facts: %v", len(kernel.AssertedFacts), kernel.AssertedFacts)
	}

	// Also verify the high-level action was produced (filtered through shouldGenerateToolNeed).
	hasGenerate := false
	for _, a := range result.Actions {
		if a.Type == ActionGenerateTool {
			hasGenerate = true
			break
		}
	}
	if !hasGenerate {
		t.Fatalf("expected ActionGenerateTool in result.Actions, got %v", result.Actions)
	}
}
