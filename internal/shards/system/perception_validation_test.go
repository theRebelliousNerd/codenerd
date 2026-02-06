package system

import (
	"context"
	"fmt"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

type stubLLMClient struct {
	response string
	err      error
}

func (s stubLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return s.response, s.err
}

func (s stubLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return s.response, s.err
}

func (s stubLLMClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &types.LLMToolResponse{Text: s.response, StopReason: "end_turn"}, nil
}

func TestPerceptionUnknownVerbEmitsIntentUnmapped(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	intentJSON := fmt.Sprintf(`{"understanding":{"primary_intent":"deploy service","semantic_type":"instruction","action_type":"deploy","domain":"general","scope":{"level":"codebase","target":"","file":"","symbol":""},"user_constraints":[],"implicit_assumptions":[],"confidence":0.9,"signals":{"is_question":false,"is_hypothetical":false,"is_multi_step":false,"is_negated":false,"requires_confirmation":false,"urgency":"normal"},"suggested_approach":{"mode":"normal","primary_shard":"coder","supporting_shards":[],"tools_needed":[],"context_needed":[]}},"surface_response":"ok"}`)
	shard := NewPerceptionFirewallShard()
	shard.SetParentKernel(kernel)
	shard.SetLLMClient(stubLLMClient{response: intentJSON})

	intent, err := shard.Perceive(ctx, "deploy the service", nil)
	if err != nil {
		t.Fatalf("Perceive error = %v", err)
	}
	if intent.Verb != "/deploy" {
		t.Fatalf("intent.Verb = %s, want /deploy", intent.Verb)
	}
	if intent.Confidence > 0.4 {
		t.Fatalf("intent.Confidence = %.2f, want <= 0.4 after unmapped verb", intent.Confidence)
	}

	facts, err := kernel.Query("intent_unmapped")
	if err != nil {
		t.Fatalf("Query(intent_unmapped) error = %v", err)
	}
	if len(facts) == 0 {
		t.Fatalf("intent_unmapped not asserted")
	}
	found := false
	for _, f := range facts {
		if len(f.Args) < 2 {
			continue
		}
		verb := fmt.Sprintf("%v", f.Args[0])
		reason := fmt.Sprintf("%v", f.Args[1])
		if verb == "/deploy" && reason == "/unknown_verb" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("intent_unmapped missing /deploy /unknown_verb (facts=%v)", facts)
	}
}
