package session

import (
	"context"
	"testing"

	"codenerd/internal/perception"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// The session a turn's model calls are filed under is the caller's when it
// names one, and the executor's own otherwise (a bare nerd run).
func TestExecutor_Process_CallerSessionWinsOverTheExecutors(t *testing.T) {
	var seen []string
	record := func(ctx context.Context) { seen = append(seen, usage.SessionIDFromContext(ctx)) }
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			record(ctx)
			return &types.LLMToolResponse{Text: "ok"}, nil
		},
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			record(ctx)
			return "ok", nil
		},
	}
	executor := NewExecutor(&MockKernel{}, &MockVirtualStore{}, mockLLM, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{
		ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
			return perception.Intent{Verb: "/greet", Category: "/chat"}, nil
		},
	})
	executor.SetSessionID("session-boot")

	for _, tc := range []struct {
		name string
		ctx  context.Context
		want string
	}{
		{"the caller names the chat session", usage.WithSessionID(context.Background(), "session-chat"), "session-chat"},
		{"a bare turn uses the executor's session", context.Background(), "session-boot"},
	} {
		seen = nil
		if _, err := executor.Process(tc.ctx, "Hello"); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if len(seen) == 0 {
			t.Fatalf("%s: no model call was made", tc.name)
		}
		for _, got := range seen {
			if got != tc.want {
				t.Fatalf("%s: a call was filed under %q, want %q", tc.name, got, tc.want)
			}
		}
	}
}
