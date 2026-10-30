package session

import (
	"context"
	"slices"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// The no-tool retry describes the first attempt's callable projection,
// narrowed by the final allowlist, never the raw grant list. It used to
// replace AvailableTools with cfg.AllowedTools, so a generated tool the
// native channel cannot carry came back as a callable bullet while the same
// prompt's UnavailableTools said the request could not call it.
func TestNoToolRetryKeepsTheCallableProjection(t *testing.T) {
	var retried *prompt.CompilationContext
	e := &Executor{jitCompiler: &MockJITCompiler{CompileFunc: func(_ context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
		retried = cc
		return &prompt.CompilationResult{Prompt: "retry prompt"}, nil
	}}}
	first := prompt.NewCompilationContext()
	first.AvailableTools = []string{"write_file", "read_file"}
	first.UnavailableTools = []string{"generated_example: the native tool channel cannot carry a generated tool's schema"}
	cfg := &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{"write_file", "generated_example"}}
	client := &MockLLMClient{CompleteWithToolsFunc: func(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
		return &types.LLMToolResponse{Text: "ok"}, nil
	}}

	if _, err := e.retryWithNoToolNudge(context.Background(), client, "do it", cfg, first); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if retried == nil || !retried.PreviousAttemptNoToolCall {
		t.Fatal("the retry did not recompile with the no-tool nudge")
	}
	// read_file was narrowed away by the final allowlist; generated_example
	// was never callable on this channel.
	if !slices.Equal(retried.AvailableTools, []string{"write_file"}) {
		t.Fatalf("retry callable tools = %v, want [write_file]", retried.AvailableTools)
	}
	for _, name := range retried.AvailableTools {
		if name == "generated_example" {
			t.Fatal("an unavailable tool became a callable bullet on the retry")
		}
	}
	if !slices.Equal(retried.UnavailableTools, first.UnavailableTools) {
		t.Fatalf("retry unavailable dispositions = %v", retried.UnavailableTools)
	}
}
