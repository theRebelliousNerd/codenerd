package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// roundScriptProvider answers every model call from one script: `rounds`
// responses that each request a single tool call, then a final text. It keeps
// every history it was handed, so a test can see exactly what the model saw.
//
// Each round asks for a different path on purpose: an identical call every
// round is a period-1 cycle, which the working policy stops for its own reason
// and would mask the one under test.
type roundScriptProvider struct {
	*MockLLMClient
	toolName  string
	rounds    int
	histories [][]types.Message
}

func (p *roundScriptProvider) CompleteWithToolResults(_ context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	p.histories = append(p.histories, append([]types.Message(nil), history...))
	if p.rounds > 0 {
		p.rounds--
		n := len(p.histories)
		return &types.LLMToolResponse{ToolCalls: []types.ToolCall{{
			ID: fmt.Sprintf("call-%d", n), Name: p.toolName,
			Input: map[string]any{"path": fmt.Sprintf("probe-%d.txt", n)},
		}}}, nil
	}
	return &types.LLMToolResponse{Text: "done"}, nil
}

// newWorkingLoopExecutor is the production shape of the loop after boot: a
// working world is set, so every turn runs inside a working loop, and the
// count limits are zero unless core_limits says otherwise.
func newWorkingLoopExecutor(t *testing.T, client types.LLMClient) *Executor {
	t.Helper()
	e := &Executor{kernel: &MockKernel{}, virtualStore: &MockVirtualStore{}, llmClient: client, config: DefaultExecutorConfig()}
	e.config.EnableSafetyGate = false
	e.config.VerifyTestsAfterEdits = false
	e.config.CriticReviewAfterEdits = false
	e.config.WorkspaceRoot = t.TempDir()
	e.config.ProgressDrivenTools = true
	e.config.MaxToolCalls = 0
	e.config.MaxToolIterations = 0
	e.workingWorld = &MockKernel{}
	return e
}

func toolResultContents(histories [][]types.Message) []string {
	var out []string
	for _, history := range histories {
		for _, m := range history {
			for _, r := range m.ToolResults {
				out = append(out, r.Content)
			}
		}
	}
	return out
}

func anyContains(items []string, needle string) bool {
	for _, item := range items {
		if strings.Contains(item, needle) {
			return true
		}
	}
	return false
}

// The nudge is keyed to a count ceiling, not to the progress-driven flag. A
// user who keeps core_limits.max_tool_iterations on a progress-driven loop has
// a hard stop at that round; the model must still be told how much is left.
// Only a genuinely open loop, where policy is the sole ceiling, stays quiet.
func TestRunToolLoop_ProgressDriven_NudgesOnlyWhenACeilingExists(t *testing.T) {
	const toolName = "working_loop_probe"
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectRead, Name: toolName, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) { return "observed", nil },
	})

	run := func(t *testing.T, iterations int) *roundScriptProvider {
		t.Helper()
		client := &roundScriptProvider{MockLLMClient: &MockLLMClient{}, toolName: toolName, rounds: 1}
		e := newWorkingLoopExecutor(t, client)
		e.config.MaxToolIterations = iterations
		result := &ExecutionResult{Intent: perception.Intent{Verb: "/explain"}}
		resp, _, err := e.runToolLoop(context.Background(), "system", "probe it",
			&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}},
			&prompt.CompilationContext{ShardID: "probe"}, result)
		if err != nil {
			t.Fatalf("runToolLoop: %v", err)
		}
		if resp == nil || resp.Text != "done" {
			t.Fatalf("response = %+v, want the scripted final answer", resp)
		}
		if result.ToolCallsExecuted != 1 {
			t.Fatalf("executed = %d, want 1", result.ToolCallsExecuted)
		}
		return client
	}

	t.Run("a ceiling from core_limits keeps the nudge", func(t *testing.T) {
		client := run(t, 1)
		if !anyContains(toolResultContents(client.histories), "[orchestrator] Orchestrator budget:") {
			t.Fatalf("max_tool_iterations=1 on a progress-driven loop is a hard stop the model was never warned about; tool results seen: %q", toolResultContents(client.histories))
		}
	})
	t.Run("an open loop has nothing to warn about", func(t *testing.T) {
		client := run(t, 0)
		if anyContains(toolResultContents(client.histories), "[orchestrator]") {
			t.Fatalf("no count ceiling, yet the model was nudged about one; tool results seen: %q", toolResultContents(client.histories))
		}
	})
}

// With no count ceiling the loop ends only when the working policy says so.
// Three consecutive rounds in which every tool failed derive
// working_stop(/tool_failures); the turn then ends as unresolved, not as a
// success and not by running the model out of script.
func TestRunToolLoop_ProgressDriven_StopsOnRepeatedToolFailures(t *testing.T) {
	const toolName = "working_loop_failing_probe"
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectRead, Name: toolName, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) { return "", errors.New("probe failed") },
	})
	client := &roundScriptProvider{MockLLMClient: &MockLLMClient{}, toolName: toolName, rounds: 50}
	e := newWorkingLoopExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/explain"}}

	_, _, err := e.runToolLoop(context.Background(), "system", "probe it",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{toolName}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if err == nil || !strings.Contains(err.Error(), "stopped by policy") || !strings.Contains(err.Error(), "tool_failures") {
		t.Fatalf("err = %v, want the working policy to stop the turn for repeated tool failures", err)
	}
	if result.ToolCallsExecuted != 3 {
		t.Fatalf("executed = %d, want 3: the third failed round is the stop, not the script running out", result.ToolCallsExecuted)
	}
	if client.rounds == 0 {
		t.Fatal("the script ran out: the loop was bounded by the mock, not by policy")
	}
}
