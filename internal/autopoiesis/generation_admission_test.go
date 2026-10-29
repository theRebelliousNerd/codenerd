package autopoiesis

import (
	"context"
	"errors"
	"testing"

	"codenerd/internal/perception"
)

// A request that restricts changes does not generate tools: detection (which
// publishes missing_tool_for) and generation are both refused before any
// model call or effect, by the kernel's turn_bars_effects. Before, the
// chat's residual path reached GenerateTool on a read-only turn (audit
// residual-tool-generation-restriction-admission). An unrestricted request
// and a restriction that bars no workspace effect (confirm_first) pass
// admission.
func TestGenerationIsRefusedForARequestThatRestrictsChanges(t *testing.T) {
	orch, _, llm := createTestOrchestrator(t)
	orch.SetKernel(newRecordingRealKernel(t))
	calls := 0
	llm.CompleteWithSystemFunc = func(ctx context.Context, _, _ string) (string, error) {
		calls++
		return `{"needs_new_tool": true, "tool_name": "json_validator", "purpose": "Validate JSON", "input_type": "string", "output_type": "bool", "priority": 0.9, "confidence": 0.9, "reasoning": "test"}`, nil
	}
	restricted := perception.WithRestriction(context.Background(), perception.RestrictionOf("read only", perception.RestrictReadOnly))

	if _, err := orch.DetectToolNeed(restricted, "I need a tool to validate JSON"); !errors.Is(err, ErrGenerationRestricted) {
		t.Fatalf("DetectToolNeed on a read-only request = %v, want ErrGenerationRestricted", err)
	}
	if _, err := orch.GenerateTool(restricted, &ToolNeed{Name: "json_validator", Purpose: "Validate JSON", Confidence: 0.9}); !errors.Is(err, ErrGenerationRestricted) {
		t.Fatalf("GenerateTool on a read-only request = %v, want ErrGenerationRestricted", err)
	}
	if calls != 0 {
		t.Fatalf("the model was called %d time(s) for a refused generation", calls)
	}

	if err := orch.admitGeneration(context.Background()); err != nil {
		t.Fatalf("an unrestricted request was refused: %v", err)
	}
	confirm := perception.WithRestriction(context.Background(), perception.RestrictionOf("ask first", perception.RestrictConfirmFirst))
	if err := orch.admitGeneration(confirm); err != nil {
		t.Fatalf("confirm_first bars no effect here, yet generation was refused: %v", err)
	}
}
