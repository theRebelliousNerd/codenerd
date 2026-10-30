package autopoiesis

import (
	"context"
	"testing"

	"codenerd/internal/types"
)

// The refiner parses its own JSON object (improved_code, changes). Its model
// call was not marked as parsed by Go, so a client that sniffs the prompt for
// "control_packet" -- present whenever the quoted original reply was an
// envelope -- attached the envelope response format (Meta's plain JSON mode,
// a strict envelope schema elsewhere) and the refiner got an envelope back.
// The generator's own calls were already marked; the refiner's now is.
func TestRefine_TheModelCallIsNotSentTheEnvelopeFormat(t *testing.T) {
	var marked, calls int
	client := &MockLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, _, _ string) (string, error) {
			calls++
			if types.IsStructuredOutputOnlyCtx(ctx) {
				marked++
			}
			return `{"improved_code": "package tools\n\nfunc tool() {}", "changes": ["none"], "expected_gain": 0.1}`, nil
		},
	}
	refiner := NewToolRefiner(client, NewToolGenerator(client, t.TempDir()))
	refiner.SetPromptAssembler(stagePromptAssembler{})
	if _, err := refiner.Refine(context.Background(), RefinementRequest{
		ToolName:     "go_vet",
		OriginalCode: `{"control_packet": {}, "surface_response": "not code"}`,
	}); err != nil {
		t.Fatal(err)
	}
	if calls == 0 || marked != calls {
		t.Fatalf("%d of %d refiner model call(s) marked as parsed by Go", marked, calls)
	}
}
