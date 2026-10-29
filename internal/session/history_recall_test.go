package session

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/tools"
	toolscore "codenerd/internal/tools/core"
	"codenerd/internal/types"
)

var historyHandleInNotice = regexp.MustCompile(`recall_context id="(obs:hist:[0-9a-f]+)"`)

// A conversation longer than the history window loses its oldest turns from
// what the model is shown, and the window says so. Until 2026-09-25 it also
// said "the session still holds them" while nothing could fetch them: the
// evicted turns sat in a field only tests read. Inside a working loop the
// notice now names a handle, and recall_context -- the one recall verb every
// persona is given -- returns the turns behind it.
//
// Driven through the tool loop, with the real recall_context tool: the model
// reads the handle out of the notice it was sent and asks for it.
func TestHistoryEviction_RecallContextReturnsTheEvictedTurns(t *testing.T) {
	reg := tools.NewRegistry()
	if err := toolscore.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tools.SwapGlobal(reg))

	var recalled string
	var sawNotice bool
	provider := &scriptedProvider{
		completeWithToolResults: func(_ context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
			for _, m := range history {
				for _, r := range m.ToolResults {
					recalled = r.Content
				}
			}
			if recalled != "" {
				return &types.LLMToolResponse{Text: "the first thing you said was TURN00"}, nil
			}
			match := historyHandleInNotice.FindStringSubmatch(history[0].Text)
			if match == nil {
				return &types.LLMToolResponse{Text: "no handle to recall"}, nil
			}
			sawNotice = true
			return &types.LLMToolResponse{ToolCalls: []types.ToolCall{{
				ID: "recall-1", Name: "recall_context", Input: map[string]any{"id": match[1]},
			}}}, nil
		},
	}
	executor := &Executor{
		kernel:       &MockKernel{},
		virtualStore: &MockVirtualStore{},
		llmClient:    provider,
		config:       DefaultExecutorConfig(),
	}
	executor.config.EnableSafetyGate = false
	executor.config.WorkspaceRoot = t.TempDir()
	executor.config.HistoryCharBudget = 4000
	for i := range 10 {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		executor.appendToHistory(perception.ConversationTurn{
			Role: role, Content: fmt.Sprintf("TURN%02d ", i) + strings.Repeat("x", 1000),
		})
	}

	result := &ExecutionResult{Intent: perception.Intent{Verb: "/review"}}
	resp, toolErrs, err := executor.runToolLoop(context.Background(), "system", "what did I say first?",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{"recall_context"}}, nil, result)
	if err != nil {
		t.Fatalf("runToolLoop: %v (%v)", err, toolErrs)
	}
	if !sawNotice {
		t.Fatalf("the window's eviction notice names no recall handle; the model cannot get the turns back (response %q)", resp.Text)
	}
	if len(toolErrs) != 0 {
		t.Fatalf("recalling the handle failed: %v", toolErrs)
	}
	if !strings.Contains(recalled, "TURN00") || !strings.Contains(recalled, `"evicted_messages"`) {
		t.Fatalf("recall_context did not return the evicted turns:\n%s", truncateForFailure(recalled))
	}
}

// A handle from an earlier window is refused with the current one named,
// rather than answered with a different set of turns than it was minted for;
// every other id still reaches the working set.
func TestHistoryRecall_NamesTheCurrentHandleForAStaleOne(t *testing.T) {
	evicted := []types.Message{{Role: "user", Text: "old"}}
	recall := historyRecall{handle: historyEvictionHandle(evicted), evicted: evicted}

	_, err := recall.Recall(context.Background(), historyHandlePrefix+"0000000000000000", 0, 0)
	if err == nil || !strings.Contains(err.Error(), recall.handle) {
		t.Fatalf("a stale handle got %v, want a refusal naming %s", err, recall.handle)
	}
	page, err := recall.Recall(context.Background(), recall.handle, 0, 4)
	if err != nil || !strings.Contains(page, `"conversation":"user"`) || !strings.Contains(page, `"next_offset":4`) {
		t.Fatalf("a page of the current handle = %s, %v", page, err)
	}
	if _, err := recall.Recall(context.Background(), "some-observation-id", 0, 0); err == nil ||
		!strings.Contains(err.Error(), "working context recall unavailable") {
		t.Fatalf("an observation id did not go to the working set: %v", err)
	}
}
