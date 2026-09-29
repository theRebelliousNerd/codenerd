package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/prompt"
	"codenerd/internal/testfacts"
	"codenerd/internal/tools"
	toolscore "codenerd/internal/tools/core"
	"codenerd/internal/types"
)

// Dogfood run 4: the repair loop spent all its attempts on another lane's
// breakage. A test run that compiled nothing and located nothing in the
// turn's write set is not this turn's to fix, and must not burn repair
// attempts. Detection reads what the gate already carries: the testfacts
// BuildFailure rows with File. A failing test's file never counts: the
// report site is not the fix site.

func TestBuildFailuresOutsideWriteSet_Table(t *testing.T) {
	ws := t.TempDir()
	for _, rel := range []string{"a/a.go", "dep/dep.go"} {
		p := filepath.Join(ws, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("package x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	buildRow := func(file string) testfacts.BuildFailure {
		return testfacts.BuildFailure{Package: "skipmod/dep", File: file, Line: 3, Message: "undefined"}
	}
	failRow := func(file string) testfacts.Failure {
		return testfacts.Failure{Package: "skipmod/a", Test: "TestA", File: file, Line: 9, Message: "want 1"}
	}
	tests := []struct {
		name  string
		res   *testfacts.Result
		set   []string
		skip  bool
		files []string
	}{
		{"nil result repairs", nil, []string{"a/a.go"}, false, nil},
		{"no diagnostics repairs", &testfacts.Result{}, []string{"a/a.go"}, false, nil},
		{"all outside skips", &testfacts.Result{BuildFailures: []testfacts.BuildFailure{buildRow("dep/dep.go")}},
			[]string{"a/a.go"}, true, []string{"dep/dep.go"}},
		{"one inside repairs", &testfacts.Result{BuildFailures: []testfacts.BuildFailure{buildRow("dep/dep.go"), buildRow("a/a.go")}},
			[]string{"a/a.go"}, false, nil},
		{"a failing test repairs", &testfacts.Result{
			BuildFailures: []testfacts.BuildFailure{buildRow("dep/dep.go")},
			Failures:      []testfacts.Failure{failRow("a/a_test.go")},
		}, []string{"a/a.go"}, false, nil},
		{"unlocated diagnostic repairs", &testfacts.Result{BuildFailures: []testfacts.BuildFailure{buildRow("")}},
			[]string{"a/a.go"}, false, nil},
		{"absolute outside spelling skips", &testfacts.Result{BuildFailures: []testfacts.BuildFailure{
			buildRow(filepath.Join(ws, "dep/dep.go")),
		}}, []string{"a/a.go"}, true, []string{"dep/dep.go"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			foreign, skip := testBuildFailuresOutsideWriteSet(tt.res, tt.set, ws)
			if skip != tt.skip {
				t.Fatalf("skip = %v, want %v", skip, tt.skip)
			}
			if skip && strings.Join(foreign, ",") != strings.Join(tt.files, ",") {
				t.Fatalf("foreign = %v, want %v", foreign, tt.files)
			}
		})
	}
}

// A broken dependency the turn never wrote: the test gate runs a real `go
// test`, the compile failure lands outside the write set, and the gate
// names it as not this turn's to fix without starting a repair episode
// (Repair stays nil: no attempt burned).
func TestVerifyAndRepairTests_SkipsForeignCompileFailure(t *testing.T) {
	ws := t.TempDir()
	for rel, content := range map[string]string{
		"go.mod":      "module skipmod\n\ngo 1.24\n",
		"a/a.go":      "package a\n\nimport _ \"skipmod/dep\"\n\nfunc A() int { return 1 }\n",
		"a/a_test.go": "package a\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) { if A() != 1 { t.Fatal(\"want 1\") } }\n",
		"dep/dep.go":  "package dep\n\nvar X = undefinedSymbol\n",
	} {
		p := filepath.Join(ws, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	e := &Executor{config: DefaultExecutorConfig()}
	e.config.WorkspaceRoot = ws
	e.config.VerifyTestsAfterEdits = true
	result := &ExecutionResult{}
	turnWrite(t, ws, result, "a/a.go", "package a\n\nimport _ \"skipmod/dep\"\n\nfunc A() int { return 1 }\n")

	_, _, err := e.verifyAndRepairTests(context.Background(),
		&MockToolResultsLLM{MockLLMClient: &MockLLMClient{}}, "", nil, nil, nil, result)
	if err == nil || !errors.Is(err, ErrVerificationFailed) {
		t.Fatalf("err = %v, want a verification failure", err)
	}
	if !strings.Contains(err.Error(), "not this turn's to fix") || !strings.Contains(err.Error(), "dep/dep.go") {
		var buildFailures, failures any
		if result.TestCheck.Result != nil {
			buildFailures, failures = result.TestCheck.Result.BuildFailures, result.TestCheck.Result.Failures
		}
		t.Fatalf("err = %q, want it to name dep/dep.go as not this turn's to fix (build failures: %+v, failures: %+v)",
			err.Error(), buildFailures, failures)
	}
	if result.TestCheck.Repair != nil {
		t.Fatalf("a repair episode ran (record %+v); the foreign failure must not burn attempts", result.TestCheck.Repair)
	}
	if data, _ := os.ReadFile(filepath.Join(ws, "dep", "dep.go")); !strings.Contains(string(data), "undefinedSymbol") {
		t.Fatalf("the foreign file was touched: %q", data)
	}
}

// Through a real repair episode: the test the turn did not write fails, the
// model reaches for it, and the episode's guard refuses the edit before it
// runs. The foreign file is intact afterwards and the refusal names the
// boundary; the episode gives up without ever touching it.
func TestRepairLoop_RefusesWritesOutsideTheWriteSet(t *testing.T) {
	ws := t.TempDir()
	t.Setenv("CODENERD_WORKSPACE_ROOT", ws)
	reg := tools.NewRegistry()
	if err := toolscore.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tools.SwapGlobal(reg))
	failing := "package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) { if Main() != 2 { t.Fatalf(\"want 2\") } }\n"
	for rel, content := range map[string]string{
		"go.mod":       "module repairprobe\n\ngo 1.25\n",
		"main.go":      "package main\n\nfunc Main() int { return 1 }\n",
		"main_test.go": failing,
	} {
		if err := os.WriteFile(filepath.Join(ws, rel), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	calls := 0
	mockLLM := &MockToolResultsLLM{
		MockLLMClient: &MockLLMClient{},
		CompleteWithToolResultsFunc: func(ctx context.Context, sys string, history []types.Message, defs []types.ToolDefinition) (*types.LLMToolResponse, error) {
			calls++
			if calls > 1 {
				return &types.LLMToolResponse{Text: "nothing left to try"}, nil
			}
			return &types.LLMToolResponse{Text: "fixing the test", ToolCalls: []types.ToolCall{
				{ID: "r1", Name: "edit_file", Input: map[string]any{"path": "main_test.go", "old_text": "!= 2", "new_text": "!= 1"}},
			}}, nil
		},
	}
	executor := NewExecutor(realKernel(t), &testExecutiveStore{}, mockLLM, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	executor.config.WorkspaceRoot = ws
	executor.config.EnableSafetyGate = false
	executor.config.RepairMaxAttempts = 1
	result := &ExecutionResult{WrittenPaths: []string{"main.go"}, SuccessfulWriteTools: 1}
	history := []types.Message{{Role: "user", Text: "fix the tests"}}
	allowCfg := &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: []string{"write_file", "read_file", "edit_file"}}
	ctx, closeLoop, loopErr := executor.beginWorkingLoop(context.Background(), "fix the tests", &prompt.CompilationContext{ShardID: "probe"})
	if loopErr != nil {
		t.Fatalf("beginWorkingLoop: %v", loopErr)
	}
	t.Cleanup(closeLoop)

	_, repairErrs, err := executor.verifyAndRepairTests(ctx, mockLLM, "", history, nil, allowCfg, result)
	if err == nil || !errors.Is(err, ErrVerificationFailed) {
		t.Fatalf("err = %v, want the give-up failure", err)
	}
	if data, _ := os.ReadFile(filepath.Join(ws, "main_test.go")); string(data) != failing {
		t.Fatalf("the repair edited a file outside the write set: %q", data)
	}
	if joined := strings.Join(repairErrs, "\n"); !strings.Contains(joined, "outside that set") || !strings.Contains(joined, "main.go") {
		t.Fatalf("repair errors = %q, want the refusal naming the write set", joined)
	}
}

// A guard built once from the pre-gate set refuses the second edit of a file
// the first attempt created: the file exists, and it was not in that set.
// The attempt guard reads WrittenPaths live, so the edit lands. The frozen
// set is on the context the way verifyCompletedToolTurn puts it there.
func TestRepairLoop_SecondAttemptMayEditAFileItCreated(t *testing.T) {
	ws := t.TempDir()
	t.Setenv("CODENERD_WORKSPACE_ROOT", ws)
	reg := tools.NewRegistry()
	if err := toolscore.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tools.SwapGlobal(reg))
	for rel, content := range map[string]string{
		"go.mod":  "module repairprobe\n\ngo 1.25\n",
		"main.go": "package main\n\nfunc Main() int { return 1 }\n",
	} {
		if err := os.WriteFile(filepath.Join(ws, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	extra := filepath.Join(ws, "extra.go")
	calls := 0
	edits := 0
	mockLLM := &MockToolResultsLLM{
		MockLLMClient: &MockLLMClient{},
		CompleteWithToolResultsFunc: func(ctx context.Context, sys string, history []types.Message, defs []types.ToolDefinition) (*types.LLMToolResponse, error) {
			calls++
			data, err := os.ReadFile(extra)
			switch {
			case os.IsNotExist(err):
				return &types.LLMToolResponse{Text: "adding", ToolCalls: []types.ToolCall{{
					ID: "c1", Name: "write_file",
					Input: map[string]any{"path": "extra.go", "content": "package main\n\nfunc Extra() int { return 1 }\n"},
				}}}, nil
			case err != nil:
				return nil, err
			case strings.Contains(string(data), "return 1"):
				// A refused edit leaves the file on return 1. Stop repeating
				// it so a refusal fails the test instead of spinning the
				// working loop.
				edits++
				if edits > 2 {
					return &types.LLMToolResponse{Text: "stop"}, nil
				}
				return &types.LLMToolResponse{Text: "editing", ToolCalls: []types.ToolCall{{
					ID: "c2", Name: "edit_file",
					Input: map[string]any{"path": "extra.go", "old_text": "return 1", "new_text": "return 2"},
				}}}, nil
			default:
				return &types.LLMToolResponse{Text: "stop"}, nil
			}
		},
	}
	executor := NewExecutor(realKernel(t), &testExecutiveStore{}, mockLLM, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	executor.config.WorkspaceRoot = ws
	executor.config.EnableSafetyGate = false
	executor.config.RepairMaxAttempts = 3
	result := &ExecutionResult{WrittenPaths: []string{"main.go"}, SuccessfulWriteTools: 1}
	history := []types.Message{{Role: "user", Text: "fix"}}
	allowCfg := &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: []string{"write_file", "edit_file", "read_file"}}
	ctx, closeLoop, loopErr := executor.beginWorkingLoop(context.Background(), "fix", &prompt.CompilationContext{ShardID: "probe"})
	if loopErr != nil {
		t.Fatalf("beginWorkingLoop: %v", loopErr)
	}
	t.Cleanup(closeLoop)
	ctx = WithTurnWriteSet(ctx, append([]string(nil), result.WrittenPaths...))
	spec := repairSpec{
		kind:         "tests",
		brokenPhrase: "edits broke the tests",
		promptFor:    func(string) string { return "fix the tests" },
		recheck: func(context.Context) (bool, repairFailure, VerifyOutcome) {
			return false, repairFailure{Output: "still red"}, VerifyFailed
		},
		followups: func() []string { return nil },
	}

	_, repairErrs, _, err := executor.repairLoop(ctx, mockLLM, "", &history, nil, allowCfg, result, "still red", spec)
	joined := strings.Join(repairErrs, "\n")
	if strings.Contains(joined, "outside that set") && strings.Contains(joined, "extra.go") {
		t.Fatalf("the episode refused an edit to a file it created: %s (err %v)", joined, err)
	}
	if calls < 2 {
		t.Fatalf("calls = %d, want a second attempt at the file the first attempt created (err %v)", calls, err)
	}
}
