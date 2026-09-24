package session

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// Through the production loop, every request carries every round so far and
// is a byte-for-byte prefix of the next: the ledger only grows at its end
// between compactions, which is what a provider prefix cache needs. A Go cut
// in the loop once kept the last three messages, and a sliding window after
// it moved the first carried round every round; measured 2026-09-21, the
// prefix changed on 767 of 854 follow-up calls.
func TestToolLoop_RequestsCarryTheWholeLedgerAppendOnly(t *testing.T) {
	const toolName = "transcript_window_probe"
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectRead, Name: toolName, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) { return "observed", nil },
	})
	const rounds = 9
	client := &roundScriptProvider{MockLLMClient: &MockLLMClient{}, toolName: toolName, rounds: rounds}
	e := newWorkingLoopExecutor(t, client)

	result := &ExecutionResult{Intent: perception.Intent{Verb: "/explain"}}
	if _, _, err := e.runToolLoop(context.Background(), "system", "probe it",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}},
		&prompt.CompilationContext{ShardID: "probe"}, result); err != nil {
		t.Fatalf("runToolLoop: %v", err)
	}
	if len(client.histories) != rounds+1 {
		t.Fatalf("%d requests, want %d", len(client.histories), rounds+1)
	}
	for n, history := range client.histories {
		if got := carriedRounds(history); got != n {
			t.Errorf("request %d (after %d rounds) carried %d rounds, want all %d", n+1, n, got, n)
		}
		if n == 0 {
			continue
		}
		previous := client.histories[n-1]
		if len(previous) > len(history) {
			t.Fatalf("request %d is shorter than request %d", n+1, n)
		}
		for i := range previous {
			if fmt.Sprintf("%+v", previous[i]) != fmt.Sprintf("%+v", history[i]) {
				t.Fatalf("request %d changed turn %d of request %d: the ledger must only grow at its end\nbefore: %s\nafter:  %s",
					n+1, i, n, truncateForFailure(fmt.Sprintf("%+v", previous[i])), truncateForFailure(fmt.Sprintf("%+v", history[i])))
			}
		}
	}
}

// Inside a working loop nothing is lost to the transcript's byte bound: every
// result a request carries is whole, or archived behind a recall_context
// pointer. The bound used to blank payloads the policy's window still showed
// and tell the model to re-run a tool whose output was archived.
func TestToolLoop_AWorkingLoopLosesNoResultToTheByteBound(t *testing.T) {
	const toolName = "transcript_bytes_probe"
	payload := strings.Repeat("x", 120*1024)
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectRead, Name: toolName, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) { return payload, nil },
	})
	const rounds = 5 // 600 KiB of results against the 256 KiB bound
	client := &roundScriptProvider{MockLLMClient: &MockLLMClient{}, toolName: toolName, rounds: rounds}
	e := newWorkingLoopExecutor(t, client)

	result := &ExecutionResult{Intent: perception.Intent{Verb: "/explain"}}
	if _, _, err := e.runToolLoop(context.Background(), "system", "probe it",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}},
		&prompt.CompilationContext{ShardID: "probe"}, result); err != nil {
		t.Fatalf("runToolLoop: %v", err)
	}
	for n, history := range client.histories {
		for _, m := range history {
			for _, r := range m.ToolResults {
				switch {
				case r.Content == payload, strings.HasPrefix(r.Content, archivedResultPrefix):
				case r.Content == evictedToolResultNotice:
					t.Fatalf("request %d carried an evicted result telling the model to re-run the tool", n+1)
				default:
					t.Fatalf("request %d carried a cut result (%d of %d bytes)", n+1, len(r.Content), len(payload))
				}
			}
		}
	}
}

func carriedRounds(history []types.Message) int {
	n := 0
	for _, m := range history {
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			n++
		}
	}
	return n
}
