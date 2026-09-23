package session

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// The transcript a request carries is the working policy's window, measured
// through the production loop rather than by handing prepareWorkingRequest a
// synthetic history. A Go cut in the loop used to keep the last three messages,
// so no request ever carried a third round whatever the policy said, and the
// slack that makes the transcript append-only (and so cacheable) never ran.
func TestToolLoop_RequestsCarryThePolicysTranscriptWindow(t *testing.T) {
	const toolName = "transcript_window_probe"
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectRead, Name: toolName, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) { return "observed", nil },
	})
	const rounds = 9
	client := &roundScriptProvider{MockLLMClient: &MockLLMClient{}, toolName: toolName, rounds: rounds}
	e := newWorkingLoopExecutor(t, client)

	// The window's size is the policy's, read the way the loop reads it.
	probeCtx, closeProbe, err := e.beginWorkingLoop(context.Background(), "probe", &prompt.CompilationContext{ShardID: "probe"})
	if err != nil {
		t.Fatal(err)
	}
	loop := activeWorkingLoop(probeCtx)
	keepRounds, err := loop.set.TranscriptRounds(probeCtx)
	if err != nil {
		t.Fatal(err)
	}
	slack, err := loop.set.TranscriptSlack(probeCtx)
	if err != nil {
		t.Fatal(err)
	}
	closeProbe()

	result := &ExecutionResult{Intent: perception.Intent{Verb: "/explain"}}
	if _, _, err := e.runToolLoop(context.Background(), "system", "probe it",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}},
		&prompt.CompilationContext{ShardID: "probe"}, result); err != nil {
		t.Fatalf("runToolLoop: %v", err)
	}
	if len(client.histories) != rounds+1 {
		t.Fatalf("%d requests, want %d", len(client.histories), rounds+1)
	}

	most := 0
	for n, history := range client.histories {
		done := n // request n+1 is sent after n completed rounds
		want := done
		if done > keepRounds {
			want = keepRounds + (done-keepRounds)%(slack+1)
		}
		got := carriedRounds(history)
		if got != want {
			t.Errorf("request %d (after %d rounds) carried %d rounds, want the policy's %d (rounds=%d slack=%d)",
				n+1, done, got, want, keepRounds, slack)
		}
		most = max(most, got)
	}
	if most < 3 {
		t.Errorf("no request carried more than %d rounds; the policy keeps %d", most, keepRounds)
	}

	// Between cuts the transcript only grows at its end, so its first round is
	// the same call from request to request: that is what a prefix cache needs.
	if slack > 0 {
		first := firstCarriedCall(client.histories[keepRounds])
		for n := keepRounds + 1; n <= keepRounds+slack && n < len(client.histories); n++ {
			if got := firstCarriedCall(client.histories[n]); got != first {
				t.Errorf("request %d starts its transcript at %q, request %d at %q: it moved between cuts",
					n+1, got, keepRounds+1, first)
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

func firstCarriedCall(history []types.Message) string {
	for _, m := range history {
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			return m.ToolCalls[0].ID
		}
	}
	return ""
}
