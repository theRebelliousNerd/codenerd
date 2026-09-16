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

	// Fixture verb /remember is known to the corpus but carries no
	// action_mapping in a fresh kernel, which is the reachable unmapped
	// premise: the transducer only emits corpus verbs (unknown action types
	// fall back to /explain), so a corpus-unknown verb can never arrive
	// through Perceive. (The old fixture, /deploy, grew a mapping and the
	// premise silently stopped holding.)
	intentJSON := `{"understanding":{"primary_intent":"remember preference","semantic_type":"instruction","action_type":"remember","domain":"general","scope":{"level":"codebase","target":"","file":"","symbol":""},"user_constraints":[],"implicit_assumptions":[],"confidence":0.9,"signals":{"is_question":false,"is_hypothetical":false,"is_multi_step":false,"is_negated":false,"requires_confirmation":false,"urgency":"normal"},"suggested_approach":{"mode":"normal","primary_shard":"coder","supporting_shards":[],"tools_needed":[],"context_needed":[]}},"surface_response":"ok"}`
	shard := NewPerceptionFirewallShard()
	shard.SetParentKernel(kernel)
	shard.SetLLMClient(stubLLMClient{response: intentJSON})

	intent, err := shard.Perceive(ctx, "remember that I prefer tabs", nil)
	if err != nil {
		t.Fatalf("Perceive error = %v", err)
	}
	if intent.Verb != "/remember" {
		t.Fatalf("intent.Verb = %s, want /remember", intent.Verb)
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
		if verb == "/remember" && reason == "/no_action_mapping" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("intent_unmapped missing /remember /no_action_mapping (facts=%v)", facts)
	}
}

func (s stubLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, forceJSON bool) (<-chan string, <-chan error) {
	contentChan := make(chan string, 1)
	errorChan := make(chan error, 1)
	go func() {
		defer close(contentChan)
		defer close(errorChan)
		res, err := s.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err != nil {
			errorChan <- err
			return
		}
		contentChan <- res
	}()
	return contentChan, errorChan
}
