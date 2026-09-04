package session

import (
	"context"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// historyRecordingClient is a fake LLM double that implements
// types.ToolResultsProvider and records the history it receives, so tests can
// prove prior turns reach the generating model as native messages.
type historyRecordingClient struct {
	mu sync.Mutex

	toolsCalls        int
	toolResultsCalls  int
	systemCalls       int
	lastToolsUser     string
	lastToolsSystem   string
	lastSystemUser    string
	lastHistory       []types.Message
	histories         [][]types.Message
	responseText      string
	toolsResponseText string
}

func newHistoryRecordingClient(responseText string) *historyRecordingClient {
	return &historyRecordingClient{responseText: responseText}
}

func (c *historyRecordingClient) Complete(_ context.Context, _ string) (string, error) {
	return c.responseText, nil
}

func (c *historyRecordingClient) CompleteWithSystem(_ context.Context, _, user string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.systemCalls++
	c.lastSystemUser = user
	if c.responseText != "" {
		return c.responseText, nil
	}
	return "ok", nil
}

func (c *historyRecordingClient) CompleteWithStreaming(_ context.Context, _, _ string, _ bool) (<-chan string, <-chan error) {
	out := make(chan string, 1)
	errs := make(chan error, 1)
	out <- c.responseText
	close(out)
	close(errs)
	return out, errs
}

func (c *historyRecordingClient) CompleteWithTools(_ context.Context, sys, user string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.toolsCalls++
	c.lastToolsUser = user
	c.lastToolsSystem = sys
	text := c.toolsResponseText
	if text == "" {
		text = c.responseText
	}
	if text == "" {
		text = "final answer"
	}
	return &types.LLMToolResponse{Text: text, StopReason: "end_turn"}, nil
}

func (c *historyRecordingClient) CompleteWithToolResults(_ context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.toolResultsCalls++
	cp := make([]types.Message, len(history))
	copy(cp, history)
	c.lastHistory = cp
	c.histories = append(c.histories, cp)
	text := c.responseText
	if text == "" {
		text = "final answer"
	}
	return &types.LLMToolResponse{Text: text, StopReason: "end_turn"}, nil
}

func (c *historyRecordingClient) snapshot() (toolsCalls, toolResultsCalls int, lastHistory []types.Message, lastToolsUser string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.toolsCalls, c.toolResultsCalls, c.lastHistory, c.lastToolsUser
}

func registerHistoryProbeTool(t *testing.T, name string) {
	t.Helper()
	tools.Global().Register(&tools.Tool{
		Name:        name,
		Description: "history test probe",
		Category:    tools.CategoryGeneral,
		Schema: tools.ToolSchema{
			Properties: map[string]tools.Property{
				"path": {Type: "string"},
			},
		},
		Execute: func(_ context.Context, _ map[string]any) (string, error) {
			return "probe output", nil
		},
	})
}

func newHistoryTestExecutor(client types.LLMClient, toolName string) *Executor {
	mockTransducer := &MockTransducer{
		ParseIntentWithContextFunc: func(_ context.Context, _ string, _ []perception.ConversationTurn) (perception.Intent, error) {
			return perception.Intent{Verb: "/greet", Category: "/chat"}, nil
		},
	}
	mockConfig := &MockConfigFactory{
		GenerateFunc: func(_ context.Context, _ *prompt.CompilationResult, _ ...string) (*config.EffectiveAgentRuntimeConfig, error) {
			return &config.EffectiveAgentRuntimeConfig{
				AllowedTools: []string{toolName},
			}, nil
		},
	}
	return NewExecutor(
		&MockKernel{},
		&MockVirtualStore{},
		client,
		&MockJITCompiler{},
		mockConfig,
		mockTransducer,
	)
}

func twoExchangeHistory() []perception.ConversationTurn {
	return []perception.ConversationTurn{
		{Role: "user", Content: "first question"},
		{Role: "assistant", Content: "first answer"},
		{Role: "user", Content: "second question"},
		{Role: "assistant", Content: "second answer"},
	}
}

func TestExecutor_HistoryPriorTurnsReachModelAsNativeMessages(t *testing.T) {
	const toolName = "historyProbeNative"
	registerHistoryProbeTool(t, toolName)
	client := newHistoryRecordingClient("third answer")
	executor := newHistoryTestExecutor(client, toolName)
	executor.SetHistory(twoExchangeHistory())

	result, err := executor.Process(context.Background(), "third question")
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if result.Response != "third answer" {
		t.Fatalf("expected third answer, got %q", result.Response)
	}

	toolsCalls, toolResultsCalls, lastHistory, _ := client.snapshot()
	if toolResultsCalls != 1 {
		t.Fatalf("expected initial call via CompleteWithToolResults (1 call), got %d (CompleteWithTools calls=%d)", toolResultsCalls, toolsCalls)
	}
	if toolsCalls != 0 {
		t.Fatalf("expected no CompleteWithTools call with history present, got %d", toolsCalls)
	}
	if len(lastHistory) != 5 {
		t.Fatalf("expected 5 messages (4 prior + current), got %d: %+v", len(lastHistory), lastHistory)
	}
	want := []struct {
		role string
		text string
	}{
		{"user", "first question"},
		{"assistant", "first answer"},
		{"user", "second question"},
		{"assistant", "second answer"},
		{"user", "third question"},
	}
	for i, w := range want {
		if lastHistory[i].Role != w.role || lastHistory[i].Text != w.text {
			t.Fatalf("history[%d] = %+v, want role=%q text=%q", i, lastHistory[i], w.role, w.text)
		}
	}
}

func TestExecutor_HistoryTurnWindowBoundsPriorMessages(t *testing.T) {
	const toolName = "historyProbeWindow"
	registerHistoryProbeTool(t, toolName)
	client := newHistoryRecordingClient("windowed answer")
	executor := newHistoryTestExecutor(client, toolName)
	cfg := DefaultExecutorConfig()
	cfg.HistoryTurnWindow = 2
	executor.SetConfig(cfg)
	executor.SetHistory(twoExchangeHistory())

	if _, err := executor.Process(context.Background(), "third question"); err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	_, toolResultsCalls, lastHistory, _ := client.snapshot()
	if toolResultsCalls != 1 {
		t.Fatalf("expected 1 CompleteWithToolResults call, got %d", toolResultsCalls)
	}
	if len(lastHistory) != 3 {
		t.Fatalf("expected 3 messages (latest exchange + current), got %d: %+v", len(lastHistory), lastHistory)
	}
	if lastHistory[0].Role != "user" || lastHistory[0].Text != "second question" {
		t.Fatalf("history[0] = %+v, want latest exchange user turn", lastHistory[0])
	}
	if lastHistory[1].Role != "assistant" || lastHistory[1].Text != "second answer" {
		t.Fatalf("history[1] = %+v, want latest exchange assistant turn", lastHistory[1])
	}
	if lastHistory[2].Role != "user" || lastHistory[2].Text != "third question" {
		t.Fatalf("history[2] = %+v, want current user turn", lastHistory[2])
	}
}

func TestExecutor_HistoryFallbackRendersTranscriptWithoutToolResultsProvider(t *testing.T) {
	const toolName = "historyProbeFallback"
	registerHistoryProbeTool(t, toolName)
	var gotUser string
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(_ context.Context, _, user string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
			gotUser = user
			return &types.LLMToolResponse{Text: "fallback answer", StopReason: "end_turn"}, nil
		},
		CompleteWithSystemFunc: func(_ context.Context, _, user string) (string, error) {
			gotUser = user
			return "fallback answer", nil
		},
	}
	executor := newHistoryTestExecutor(mockLLM, toolName)
	executor.SetHistory(twoExchangeHistory())

	result, err := executor.Process(context.Background(), "third question")
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if result.Response != "fallback answer" {
		t.Fatalf("expected fallback answer, got %q", result.Response)
	}
	if !strings.Contains(gotUser, "Conversation so far:") {
		t.Fatalf("expected rendered transcript prefix, got %q", gotUser)
	}
	if !strings.Contains(gotUser, "second answer") {
		t.Fatalf("expected prior assistant text in transcript, got %q", gotUser)
	}
	if !strings.Contains(gotUser, "third question") {
		t.Fatalf("expected current input in transcript, got %q", gotUser)
	}
}

func TestExecutor_HistoryEmptyLeavesInitialCallUnchanged(t *testing.T) {
	const toolName = "historyProbeEmpty"
	registerHistoryProbeTool(t, toolName)
	client := newHistoryRecordingClient("bare answer")
	executor := newHistoryTestExecutor(client, toolName)

	// No SetHistory: empty conversation.
	if _, err := executor.Process(context.Background(), "bare input"); err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	toolsCalls, toolResultsCalls, _, lastToolsUser := client.snapshot()
	if toolResultsCalls != 0 {
		t.Fatalf("expected no CompleteWithToolResults call with empty history, got %d", toolResultsCalls)
	}
	if toolsCalls != 1 {
		t.Fatalf("expected exactly 1 CompleteWithTools call, got %d", toolsCalls)
	}
	if lastToolsUser != "bare input" {
		t.Fatalf("expected byte-identical bare input, got %q", lastToolsUser)
	}
}

func TestPriorTurnMessages_SkipsEmptyAndMapsRoles(t *testing.T) {
	executor := NewExecutor(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	executor.SetHistory([]perception.ConversationTurn{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: ""},
		{Role: "system", Content: "sys note"},
		{Role: "", Content: "blank role"},
	})
	msgs := executor.priorTurnMessages()
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages after skipping empty, got %d: %+v", len(msgs), msgs)
	}
	if msgs[0].Role != "user" || msgs[0].Text != "hello" {
		t.Fatalf("msgs[0] = %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Text != "sys note" {
		t.Fatalf("non-user roles must map to assistant, got %+v", msgs[1])
	}
	if msgs[2].Role != "assistant" || msgs[2].Text != "blank role" {
		t.Fatalf("msgs[2] = %+v", msgs[2])
	}
}

func TestPriorTurnMessages_CharBudgetNeverSplitsPair(t *testing.T) {
	e2 := NewExecutor(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	cfg := DefaultExecutorConfig()
	cfg.HistoryTurnWindow = 6
	cfg.HistoryCharBudget = 12
	e2.SetConfig(cfg)
	e2.SetHistory([]perception.ConversationTurn{
		{Role: "user", Content: "aaaa"},
		{Role: "assistant", Content: "bbbb"},
		{Role: "user", Content: "cccc"},
		{Role: "assistant", Content: "dddd"},
	})
	msgs := e2.priorTurnMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected oldest pair dropped leaving 2 messages, got %d: %+v", len(msgs), msgs)
	}
	if msgs[0].Text != "cccc" || msgs[1].Text != "dddd" {
		t.Fatalf("expected latest pair, got %+v", msgs)
	}
}
