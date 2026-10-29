package session

import (
	"context"
	"errors"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// External audit N07 (2026-09-19): a tool call counted as a test execution by
// its name, so a run_impacted_tests dry run -- a success that ran nothing --
// backed a turn's test-runner output as if tests had run. A call counts when
// the tool layer recorded a test run, and a run whose tests failed is still a
// run.
func TestTestRunCalls_AreReceiptsNotNames(t *testing.T) {
	reg := tools.NewRegistry()
	t.Cleanup(tools.SwapGlobal(reg))
	for _, tool := range []*tools.Tool{
		{
			Effect: tools.EffectRead, Name: "run_impacted_tests", Category: tools.CategoryTest,
			Execute: func(context.Context, map[string]any) (string, error) {
				return "Found 1 impacted tests:\n\n(dry run - tests not executed)\n", nil
			},
		},
		{
			Effect: tools.EffectRead, Name: "receipt_probe_failing_run", Category: tools.CategoryTest,
			Execute: func(ctx context.Context, _ map[string]any) (string, error) {
				tools.RecordTestRun(ctx, tools.TestRun{Argv: []string{"go", "test", "./..."}, ExitCode: 1})
				return "--- FAIL: TestX", errors.New("exit status 1")
			},
		},
	} {
		if err := reg.Register(tool); err != nil {
			t.Fatalf("register %s: %v", tool.Name, err)
		}
	}

	e := &Executor{config: ExecutorConfig{}, virtualStore: &testExecutiveStore{}}
	cfg := &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{"run_impacted_tests", "receipt_probe_failing_run"}}
	result := &ExecutionResult{}

	e.executeToolBatch(context.Background(), []types.ToolCall{{ID: "a", Name: "run_impacted_tests", Input: map[string]any{"dry_run": true}}}, cfg, result)
	if result.TestRunCalls != 0 {
		t.Fatalf("TestRunCalls = %d after a dry run, want 0: the name is not a run", result.TestRunCalls)
	}

	e.executeToolBatch(context.Background(), []types.ToolCall{{ID: "b", Name: "receipt_probe_failing_run"}}, cfg, result)
	if result.TestRunCalls != 1 {
		t.Fatalf("TestRunCalls = %d after a recorded run whose tests failed, want 1", result.TestRunCalls)
	}
}
