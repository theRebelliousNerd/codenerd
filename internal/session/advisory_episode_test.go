package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// advisoryLoopCtx installs a working loop, the policy an advisory episode
// needs to end it.
func advisoryLoopCtx(t *testing.T, e *Executor) context.Context {
	t.Helper()
	ctx, closeLoop, err := e.beginWorkingLoop(context.Background(), nil, "advise", &prompt.CompilationContext{ShardID: "probe"})
	if err != nil {
		t.Fatalf("beginWorkingLoop: %v", err)
	}
	t.Cleanup(closeLoop)
	if activeWorkingLoop(ctx) == nil {
		t.Fatal("fixture: no working loop (the executor has no workspace root)")
	}
	return ctx
}

// writingTurnHistory is the history an advisory round follows: the turn's
// task and the tool round that wrote. A working request carries the messages
// from the first tool round on (ledgerStart), so the round's demand rides
// after it.
func writingTurnHistory() []types.Message {
	return []types.Message{
		{Role: "user", Text: "change a.go"},
		{Role: "assistant", ToolCalls: []types.ToolCall{{ID: "turn-write", Name: "write_file", Input: map[string]any{"path": "a.go"}}}},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "turn-write", Content: "wrote a.go"}}},
	}
}

// readThenJudge reads on its first call and answers in words on its second,
// keeping every request's history.
type readThenJudge struct {
	*MockLLMClient
	readTool  string
	histories [][]types.Message
}

func (p *readThenJudge) CompleteWithToolResults(_ context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	p.histories = append(p.histories, append([]types.Message(nil), history...))
	if len(p.histories) == 1 {
		return &types.LLMToolResponse{ToolCalls: []types.ToolCall{{ID: "advise-read", Name: p.readTool, Input: map[string]any{"path": "a.go"}}}}, nil
	}
	return &types.LLMToolResponse{Text: "The guard's other branch changes nothing observable; no test is owed."}, nil
}

// An advisory round was one model response: a model that read the cited code
// before judging the advice never got to judge it, and the read alone bought
// a second build and test run of a turn that had written Go. The episode now
// hands the read's result to the next response, and an episode that changed
// nothing is not rechecked.
func TestAdvisoryEpisode_TheReadReachesTheJudgmentAndBuysNoRecheck(t *testing.T) {
	e := newWorkingLoopExecutor(t, &MockLLMClient{})
	ws := e.config.WorkspaceRoot
	for name, content := range map[string]string{
		"go.mod": "module adviseprobe\n\ngo 1.21\n",
		"a.go":   "package adviseprobe\n\nfunc Add(a, b int) int {\n\tif a == 0 {\n\t\treturn b\n\t}\n\treturn a + b\n}\n",
	} {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const readTool = "advise_read_probe"
	const marker = "ADVISE_READ_MARKER func Add"
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectRead, Name: readTool, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) { return marker, nil },
	})
	ctx := advisoryLoopCtx(t, e)
	cfg := &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{readTool}}
	provider := &readThenJudge{MockLLMClient: &MockLLMClient{}, readTool: readTool}
	result := &ExecutionResult{
		WriteToolEffects: 1, WrittenPaths: []string{"a.go"},
		PinCheck:    BuildVerification{Ran: true, OK: true, Outcome: VerifyPassed},
		PinAdvisory: []string{"a.go line 4, a == 0 forced false"},
	}

	if _, err := e.adviseOnSurvivors(ctx, provider, "sys", writingTurnHistory(), e.buildToolDefinitions(cfg), cfg, result); err != nil {
		t.Fatalf("adviseOnSurvivors: %v", err)
	}
	if len(provider.histories) != 2 {
		t.Fatalf("the episode made %d model call(s), want the read and then the judgment", len(provider.histories))
	}
	if !anyContains(toolResultContents(provider.histories[1:]), marker) {
		t.Fatal("the read's result did not reach the response that judges the advice")
	}
	if result.BuildCheck.Ran {
		t.Fatal("a read-only episode re-ran the build and tests")
	}
}
