package session

import (
	"context"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/jit/config"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// testRunScript is a model that, sent back to run the tests, starts one
// (probe_policy_tests) and then answers.
type testRunScript struct {
	*MockLLMClient
	calls   int
	prompts []string
}

func (p *testRunScript) CompleteWithToolResults(_ context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	p.calls++
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" && history[i].Text != "" {
			p.prompts = append(p.prompts, history[i].Text)
			break
		}
	}
	if last := history[len(history)-1]; len(last.ToolResults) > 0 {
		return &types.LLMToolResponse{Text: "Ran the policy tests."}, nil
	}
	return &types.LLMToolResponse{ToolCalls: []types.ToolCall{{ID: "run", Name: "probe_policy_tests", Input: map[string]any{}}}}, nil
}

// testRunRoundExec is an executor over the shipped corpus, so the policy
// decides what a write owes, with the test tool the model is scripted to use:
// a test process that exits with exit.
func testRunRoundExec(t *testing.T, client types.LLMClient, exit int) (*Executor, context.Context, *config.EffectiveAgentRuntimeConfig) {
	t.Helper()
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectExecute, Name: "probe_policy_tests", Category: tools.CategoryTest,
		Execute: func(ctx context.Context, _ map[string]any) (string, error) {
			tools.RecordTestRun(ctx, tools.TestRun{Argv: []string{"go", "test", "./internal/core/..."}, ExitCode: exit})
			return "ran", nil
		},
	})
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("the shipped corpus must load: %v", err)
	}
	e := newWorkingLoopExecutor(t, client)
	e.kernel = kernel
	e.virtualStore = &testExecutiveStore{} // the executive gate a test run passes
	ctx, closeLoop, err := e.beginWorkingLoop(context.Background(), "fix the policy", &prompt.CompilationContext{ShardID: "probe", IntentTarget: "x.mg"})
	if err != nil {
		t.Fatalf("beginWorkingLoop: %v", err)
	}
	t.Cleanup(closeLoop)
	return e, ctx, &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{"probe_policy_tests"}}
}

// wroteHistory is the turn's transcript when its gates run: the model wrote
// path with a tool call, and the result came back.
func wroteHistory(path string) []types.Message {
	return []types.Message{
		{Role: "assistant", ToolCalls: []types.ToolCall{{ID: "write", Name: "write_file", Input: map[string]any{"path": path}}}},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "write", Content: "written"}}},
	}
}

// External audit N01, its follow-on: a write the Go gates do not cover owes a
// test run the model started after its last write. A turn that owed one and
// never ran it ended /unverified with the run named, and nothing sent it back
// to run it. It is sent back now -- the policy says what the write owes -- and
// the run it starts closes the turn /done.
func TestVerifyCompletedToolTurn_APolicyEditNoTestRanIsSentBackToRunOne(t *testing.T) {
	client := &testRunScript{MockLLMClient: &MockLLMClient{}}
	e, ctx, cfg := testRunRoundExec(t, client, 0)
	result := mutationResult()
	result.WrittenPaths = []string{"internal/core/defaults/policy/x.mg"}

	if _, _, err := e.verifyCompletedToolTurn(ctx, client, "system", wroteHistory(result.WrittenPaths[0]), &types.LLMToolResponse{Text: "Edited the policy."}, e.buildToolDefinitions(cfg), cfg, result); err != nil {
		t.Fatalf("verifyCompletedToolTurn: %v", err)
	}
	if client.calls == 0 {
		t.Fatal("the turn owed a test run and was not sent back to run one")
	}
	if !strings.Contains(client.prompts[0], "no test process ran after this turn's last write") || !strings.Contains(client.prompts[0], "x.mg") {
		t.Errorf("the round's prompt does not name the file and the missing run:\n%s", client.prompts[0])
	}
	if got := result.testRunVerdict(); got != VerifyPassed {
		t.Fatalf("test run gate = %v after the round, want passed", got)
	}

	turn := result.turnAtom()
	e.assertTurnEvidence(turn, "/fix", result)
	e.captureTurnOutcome(turn, result, nil)
	if result.TurnOutcome != types.MangleAtom("/done") {
		t.Fatalf("TurnOutcome = %v (missing %v), want /done: the run the round asked for passed", result.TurnOutcome, result.MissingEvidence)
	}
}

// A run that keeps failing leaves the gate red to the verdict: the round does
// not fail the turn, and the verdict names the run.
func TestVerifyCompletedToolTurn_ATestRunThatKeepsFailingIsTheVerdicts(t *testing.T) {
	client := &testRunScript{MockLLMClient: &MockLLMClient{}}
	e, ctx, cfg := testRunRoundExec(t, client, 1)
	result := mutationResult()
	result.WrittenPaths = []string{"scripts/check.py"}

	if _, _, err := e.verifyCompletedToolTurn(ctx, client, "system", wroteHistory(result.WrittenPaths[0]), &types.LLMToolResponse{Text: "Edited the script."}, e.buildToolDefinitions(cfg), cfg, result); err != nil {
		t.Fatalf("verifyCompletedToolTurn: %v -- a round that does not converge is the verdict's, not an error", err)
	}
	if len(client.prompts) < 2 || !strings.Contains(client.prompts[len(client.prompts)-1], "exited 1") {
		t.Errorf("a later attempt was not told the run failed: %q", client.prompts)
	}
	turn := result.turnAtom()
	e.assertTurnEvidence(turn, "/fix", result)
	e.captureTurnOutcome(turn, result, nil)
	if result.TurnOutcome == types.MangleAtom("/done") || !slices.Contains(result.MissingEvidence, "/test_run_not_green") {
		t.Fatalf("TurnOutcome = %v, MissingEvidence = %v; want the failing run named", result.TurnOutcome, result.MissingEvidence)
	}
}

// A document owes nothing: no round.
func TestVerifyCompletedToolTurn_ADocumentEditOwesNoTestRun(t *testing.T) {
	client := &testRunScript{MockLLMClient: &MockLLMClient{}}
	e, ctx, cfg := testRunRoundExec(t, client, 0)
	result := mutationResult()
	result.WrittenPaths = []string{"Docs/guide.md"}

	if _, _, err := e.verifyCompletedToolTurn(ctx, client, "system", wroteHistory(result.WrittenPaths[0]), &types.LLMToolResponse{Text: "Edited the guide."}, e.buildToolDefinitions(cfg), cfg, result); err != nil {
		t.Fatalf("verifyCompletedToolTurn: %v", err)
	}
	if client.calls != 0 {
		t.Fatalf("a document edit was sent back %d time(s); it owes no test run", client.calls)
	}
}
