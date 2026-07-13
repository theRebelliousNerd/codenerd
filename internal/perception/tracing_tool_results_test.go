package perception

import (
	"context"
	"testing"

	"codenerd/internal/types"
)

// trpUnderlying implements LLMClient + ToolResultsProvider for tracing tests.
type trpUnderlying struct {
	calls int
	hist  int
}

func (u *trpUnderlying) Complete(ctx context.Context, prompt string) (string, error) {
	return "ok", nil
}
func (u *trpUnderlying) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return "ok", nil
}
func (u *trpUnderlying) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []ToolDefinition) (*LLMToolResponse, error) {
	return &LLMToolResponse{Text: "tools"}, nil
}
func (u *trpUnderlying) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	ch := make(chan string)
	close(ch)
	errc := make(chan error)
	close(errc)
	return ch, errc
}
func (u *trpUnderlying) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	u.calls++
	u.hist = len(history)
	return &types.LLMToolResponse{Text: "traced-tool-results", StopReason: "end_turn"}, nil
}

func TestTracingLLMClient_CompleteWithToolResults_Forwards(t *testing.T) {
	u := &trpUnderlying{}
	tc := NewTracingLLMClient(u, nil)
	tc.SetShardContext("shard1", "coder", "ephemeral", "sess", "task")

	// Compile-time / runtime: TracingLLMClient must be ToolResultsProvider.
	var _ types.ToolResultsProvider = tc

	history := []types.Message{
		{Role: "user", Text: "create app"},
		{Role: "assistant", ToolCalls: []types.ToolCall{{ID: "c1", Name: "write_file"}}},
	}
	resp, err := tc.CompleteWithToolResults(context.Background(), "sys", history, []types.ToolDefinition{{Name: "write_file"}})
	if err != nil {
		t.Fatalf("CompleteWithToolResults: %v", err)
	}
	if resp == nil || resp.Text != "traced-tool-results" {
		t.Fatalf("resp=%+v", resp)
	}
	if u.calls != 1 {
		t.Fatalf("underlying calls=%d want 1", u.calls)
	}
	if u.hist != 2 {
		t.Fatalf("hist=%d want 2", u.hist)
	}
}

func TestTracingLLMClient_CompleteWithToolResults_NoUnderlying(t *testing.T) {
	// underlying without TRP
	base := &staticClient{}
	tc := NewTracingLLMClient(base, nil)
	_, err := tc.CompleteWithToolResults(context.Background(), "sys", nil, nil)
	if err == nil {
		t.Fatal("expected error when underlying lacks ToolResultsProvider")
	}
}

// staticClient is a minimal LLMClient without ToolResultsProvider.
type staticClient struct{}

func (s *staticClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "x", nil
}
func (s *staticClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return "x", nil
}
func (s *staticClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []ToolDefinition) (*LLMToolResponse, error) {
	return &LLMToolResponse{Text: "x"}, nil
}
func (s *staticClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	ch := make(chan string)
	close(ch)
	errc := make(chan error)
	close(errc)
	return ch, errc
}
