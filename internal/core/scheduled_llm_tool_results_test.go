package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"codenerd/internal/types"
)

// trpMock implements types.LLMClient + types.ToolResultsProvider.
// Used to prove ScheduledLLMCall forwards CompleteWithToolResults (the bug that
// blocked multi-turn write_file after the first tool batch).
type trpMock struct {
	mockLLMClient
	toolResultsCalls int
	lastHistoryLen   int
	lastTools        int
	resp             *types.LLMToolResponse
	err              error
}

func (m *trpMock) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	m.toolResultsCalls++
	m.lastHistoryLen = len(history)
	m.lastTools = len(tools)
	if m.err != nil {
		return nil, m.err
	}
	if m.resp != nil {
		return m.resp, nil
	}
	return &types.LLMToolResponse{Text: "ok-after-tools", StopReason: "end_turn"}, nil
}

func TestScheduledLLMCall_CompleteWithToolResults_Forwards(t *testing.T) {
	sched := NewAPIScheduler(APISchedulerConfig{
		MaxConcurrentAPICalls: 2,
		SlotAcquireTimeout:    5 * time.Second,
		MinCallSpacing:        0,
	})
	sched.RegisterShard("coder", "coder")

	mock := &trpMock{}
	wrapper := &ScheduledLLMCall{
		Scheduler: sched,
		ShardID:   "coder",
		Client:    mock,
	}

	// Compile-time: wrapper must satisfy ToolResultsProvider so session
	// executor type-assert does not fail on *ScheduledLLMCall.
	var _ types.ToolResultsProvider = wrapper

	history := []types.Message{
		{Role: "user", Text: "write files"},
		{Role: "assistant", ToolCalls: []types.ToolCall{{ID: "1", Name: "write_file", Input: map[string]any{"path": "a.go"}}}},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "1", Content: "ok"}}},
	}
	tools := []types.ToolDefinition{{Name: "write_file", Description: "write a file", InputSchema: map[string]any{"type": "object"}}}

	resp, err := wrapper.CompleteWithToolResults(context.Background(), "sys", history, tools)
	if err != nil {
		t.Fatalf("CompleteWithToolResults: %v", err)
	}
	if resp == nil || resp.Text != "ok-after-tools" {
		t.Fatalf("unexpected resp: %+v", resp)
	}
	if mock.toolResultsCalls != 1 {
		t.Fatalf("underlying CompleteWithToolResults calls=%d want 1", mock.toolResultsCalls)
	}
	if mock.lastHistoryLen != 3 {
		t.Fatalf("history len=%d want 3", mock.lastHistoryLen)
	}
	if mock.lastTools != 1 {
		t.Fatalf("tools=%d want 1", mock.lastTools)
	}
	// Slot must be released after call.
	if m := sched.GetMetrics(); m.ActiveSlots != 0 {
		t.Fatalf("slot leak ActiveSlots=%d", m.ActiveSlots)
	}
}

func TestScheduledLLMCall_CompleteWithToolResults_NoUnderlyingTRP(t *testing.T) {
	sched := NewAPIScheduler(APISchedulerConfig{MaxConcurrentAPICalls: 1, SlotAcquireTimeout: time.Second})
	sched.RegisterShard("s", "s")
	// mockLLMClient has no CompleteWithToolResults
	wrapper := &ScheduledLLMCall{Scheduler: sched, ShardID: "s", Client: &mockLLMClient{}}
	_, err := wrapper.CompleteWithToolResults(context.Background(), "sys", nil, nil)
	if err == nil {
		t.Fatal("expected error when underlying lacks ToolResultsProvider")
	}
	if !errors.Is(err, err) && err.Error() == "" {
		t.Fatal("empty error")
	}
}
