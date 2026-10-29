package session

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// A turn that must act, offered no tool, fails naming that cause -- not as a
// hollow success. On 2026-09-18 a full kernel rejected the turn's own facts, the
// runtime compiled zero tools, the no-tool nudge fired, the model answered in
// prose again (it had nothing to call), and six `nerd fix` runs were recorded
// /hollow with nothing pointing at the missing tools.
func TestTurnThatMustActWithZeroToolsOfferedFailsByName(t *testing.T) {
	llmCalls := 0
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			llmCalls++
			return &types.LLMToolResponse{Text: "I have applied the fix and the tests are green."}, nil
		},
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			llmCalls++
			return "I have applied the fix and the tests are green.", nil
		},
	}
	mockTransducer := &MockTransducer{
		ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
			return perception.Intent{Verb: "/fix", Category: "/mutation"}, nil
		},
	}
	mockJIT := &MockJITCompiler{
		CompileFunc: func(ctx context.Context, cctx *prompt.CompilationContext) (*prompt.CompilationResult, error) {
			return &prompt.CompilationResult{Prompt: "system"}, nil
		},
	}
	mockCfg := &MockConfigFactory{
		GenerateFunc: func(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error) {
			// The observed state: the config compiled with no tool in it.
			return &config.EffectiveAgentRuntimeConfig{}, nil
		},
	}

	executor := NewExecutor(
		&requiresToolKernel{&MockKernel{}},
		&MockVirtualStore{},
		mockLLM,
		mockJIT,
		mockCfg,
		mockTransducer,
	)

	preset := &perception.Intent{Verb: "/fix", Category: "/mutation", Confidence: 1.0}
	_, err := executor.ProcessWithIntent(context.Background(), "fix the nil deref in parse.go", preset)
	if err == nil {
		t.Fatal("a /fix turn offered zero tools reported success")
	}
	if !strings.Contains(err.Error(), "no tools were offered") {
		t.Fatalf("the failure does not name the missing tools: %v", err)
	}
	if strings.Contains(err.Error(), "hollow success blocked") {
		t.Fatalf("the turn was blamed on the model (hollow) when the runtime offered it nothing to call: %v", err)
	}
	if llmCalls > 1 {
		t.Errorf("the model was re-prompted %d time(s) to call a tool that was never offered", llmCalls-1)
	}
}
