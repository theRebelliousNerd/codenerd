package articulation

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/prompt"
)

// The tool generator's compiled prompt carried the Piggyback atoms and the
// mandatory suffix, so the model answered with a JSON envelope instead of the
// Go source the generator parses. Its prompt now asks for nothing it does not
// parse; a conversational shard keeps the envelope.
func TestAssembleSystemPrompt_TheToolGeneratorIsNotAskedForTheEnvelope(t *testing.T) {
	jit, err := prompt.NewJITPromptCompiler()
	if err != nil {
		t.Fatal(err)
	}
	pa, err := NewPromptAssemblerWithJIT(newMockKernel(), jit)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := pa.AssembleSystemPrompt(context.Background(), map[string]any{
		"shard_id": "tool_generator_go_vet", "shard_type": "tool_generator",
		"stage": "/specification", "ouroboros_stage": "/specification", "tool_name": "go_vet",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, envelope := range []string{"control_packet", "surface_response", "PIGGYBACK"} {
		if strings.Contains(gen, envelope) {
			t.Errorf("the tool generator's prompt asks for the envelope (%q):\n%s", envelope, gen)
		}
	}

	coder, err := pa.AssembleSystemPrompt(context.Background(), map[string]any{"shard_id": "coder-1", "shard_type": "coder"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(coder, "control_packet") {
		t.Error("a conversational shard lost the envelope it speaks")
	}
}
