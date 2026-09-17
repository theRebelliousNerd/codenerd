package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	toolscore "codenerd/internal/tools/core"
	"codenerd/internal/types"
)

// C1 behavioral tests: bounded evidence-driven repair through the production
// caller chain (ProcessWithIntent -> runToolLoop(Pass) -> verifyTerminal ->
// verifyCompletedToolTurn -> verifyAndRepairBuild/Tests). The model is fake
// (content-routed scripts); tools and go build/test verification are real,
// on a scratch module per test.

// repairHarness boots an Executor with a scripted model and a scratch Go
// module workspace. The mock routes by history content: repair prompts (which
// carry failing output) get fix responses, post-batch turns conclude.
type repairHarness struct {
	executor  *Executor
	ws        string
	calls     int
	sysCalls  int
	toolsCalls int
	onRepair  func(call int, history []types.Message) *types.LLMToolResponse
	initial   func() *types.LLMToolResponse
}

func newRepairHarness(t *testing.T, configure func(*Executor)) *repairHarness {
	t.Helper()
	h := &repairHarness{}
	// Isolated tool registry with the real file tools: production boot
	// registers them in VirtualStore setup, which tests never run.
	reg := tools.NewRegistry()
	if err := toolscore.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tools.SwapGlobal(reg))
	ws := t.TempDir()
	h.ws = ws
	t.Setenv("CODENERD_WORKSPACE_ROOT", ws)
	if err := os.WriteFile(filepath.Join(ws, "go.mod"), []byte("module repairprobe\n\ngo 1.25\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mockLLM := &MockToolResultsLLM{
		MockLLMClient: &MockLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			h.sysCalls++
			return "", nil
		},
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			h.toolsCalls++
			// Initial generation lands here (no working world in tests);
			// follow-ups and repair go through CompleteWithToolResults.
			if strings.Contains(user, "FAIL") || strings.Contains(user, "repair") {
				if h.onRepair != nil {
					return h.onRepair(h.toolsCalls, []types.Message{{Role: "user", Text: user}}), nil
				}
			}
			if h.initial != nil {
				return h.initial(), nil
			}
			return &types.LLMToolResponse{Text: "done"}, nil
		},
		},
		CompleteWithToolResultsFunc: func(ctx context.Context, sys string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			h.calls++
			var sb strings.Builder
			sawResults := false
			for _, m := range history {
				sb.WriteString(m.Text)
				sb.WriteString("\n")
				if len(m.ToolResults) > 0 {
					sawResults = true
				}
			}
			joined := sb.String()
			if strings.Contains(joined, "FAIL") || strings.Contains(joined, "repair") || strings.Contains(joined, "Error") {
				if h.onRepair != nil {
					return h.onRepair(h.calls, history), nil
				}
			}
			if sawResults {
				return &types.LLMToolResponse{Text: "done"}, nil
			}
			if h.initial != nil {
				return h.initial(), nil
			}
			return &types.LLMToolResponse{Text: "done"}, nil
		},
	}
	mockTransducer := &MockTransducer{
		ParseIntentWithContextFunc: func(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
			return perception.Intent{Verb: "/fix", Category: "/mutation"}, nil
		},
	}
	mockCfgFactory := &MockConfigFactory{
		GenerateFunc: func(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*jitconfig.EffectiveAgentRuntimeConfig, error) {
			return &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: []string{"write_file", "read_file", "edit_file"}}, nil
		},
	}
	h.executor = NewExecutor(&MockKernel{}, &testExecutiveStore{}, mockLLM, &MockJITCompiler{}, mockCfgFactory, mockTransducer)
	h.executor.config.WorkspaceRoot = ws
	h.executor.config.EnableSafetyGate = false
	h.executor.EffectiveAgentRuntimeConfig = &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: []string{"write_file", "read_file", "edit_file"}}
	if configure != nil {
		configure(h.executor)
	}
	return h
}

func (h *repairHarness) writeCall(id, name, path, content string) types.ToolCall {
	return types.ToolCall{ID: id, Name: name, Input: map[string]any{"path": path, "content": content}}
}

func (h *repairHarness) drive(t *testing.T, task string) (*ExecutionResult, error) {
	t.Helper()
	preset := presetIntentForTask("/fix", task, "")
	if preset == nil {
		t.Fatal("no preset for /fix")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	return h.executor.ProcessWithIntent(ctx, task, preset)
}

const repairMainGo = "package main\n\nfunc main() {}\n"

const repairTestBroken = "package main\n\nimport \"testing\"\n\nfunc TestProbe(t *testing.T) {\n\tt.Fatal(\"broken\")\n}\n"

const repairTestFixed = "package main\n\nimport \"testing\"\n\nfunc TestProbe(t *testing.T) {\n\tt.Log(\"fixed\")\n}\n"

const repairMainTypeBroken = "package main\n\nvar x int = \"nope\"\n\nfunc main() {}\n"

const repairMainTypeFixed = "package main\n\nvar x int = 3\n\nfunc main() {}\n"

// A broken change converges: the repair episode fixes the failing test and
// the turn verifies TestsPass with attempts>=1 and a costed record.
func TestRepairLoop_FixesFailingTests(t *testing.T) {
	h := newRepairHarness(t, nil)
	h.initial = func() *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{
			h.writeCall("c1", "write_file", filepath.Join(h.ws, "main.go"), repairMainGo),
			h.writeCall("c2", "write_file", filepath.Join(h.ws, "main_test.go"), repairTestBroken),
		}}
	}
	h.onRepair = func(call int, history []types.Message) *types.LLMToolResponse {
		return &types.LLMToolResponse{
			Text:  "fixed",
			Usage: types.UsageMetadata{InputTokens: 100, OutputTokens: 50},
			ToolCalls: []types.ToolCall{
				h.writeCall("r1", "write_file", filepath.Join(h.ws, "main_test.go"), repairTestFixed),
			},
		}
	}
	result, err := h.drive(t, "fix make the failing test pass")
	t.Logf("mock calls=%d sys=%d tools=%d", h.calls, h.sysCalls, h.toolsCalls)
	// The repair must land on disk, not just in the transcript: the fixed
	// content is what the recheck verified.
	if disk, rerr := os.ReadFile(filepath.Join(h.ws, "main_test.go")); rerr != nil {
		t.Fatalf("reading repaired file: %v", rerr)
	} else if string(disk) != repairTestFixed {
		t.Fatalf("disk main_test.go=%q, want the fixed content", string(disk))
	}
	if err != nil {
		t.Fatalf("ProcessWithIntent: %v", err)
	}
	if !result.TestCheck.OK || result.TestCheck.Verdict() != VerifyPassed {
		t.Fatalf("TestCheck = %+v, want passed", result.TestCheck)
	}
	rec := result.TestCheck.Repair
	if rec == nil {
		t.Fatal("no repair record attached to TestCheck")
	}
	if !rec.Passed {
		t.Fatal("repair record Passed=false after convergence")
	}
	if rec.Cost.Attempts < 1 {
		t.Fatalf("attempts=%d, want >=1", rec.Cost.Attempts)
	}
	if rec.Cost.LLMCalls < 1 || rec.Cost.ToolCalls < 1 {
		t.Fatalf("cost=%s, want llm and tool calls recorded", rec.Cost.String())
	}
	if rec.Cost.TokensIn != 100 || rec.Cost.TokensOut != 50 {
		t.Fatalf("tokens=%d/%d, want 100/50", rec.Cost.TokensIn, rec.Cost.TokensOut)
	}
	if len(rec.Attempts) != rec.Cost.Attempts {
		t.Fatalf("%d attempt records for %d cost attempts", len(rec.Attempts), rec.Cost.Attempts)
	}
	for i, a := range rec.Attempts {
		if a.Index != i+1 {
			t.Fatalf("attempt[%d].Index=%d, want %d (Index is the ordering key)", i, a.Index, i+1)
		}
		if a.Started.IsZero() {
			t.Fatalf("attempt %d has zero observed_started", a.Index)
		}
	}
	if len(rec.Attempts[len(rec.Attempts)-1].ToolRuns) == 0 {
		t.Fatal("final attempt records no tool runs")
	}
	for _, r := range rec.Attempts[len(rec.Attempts)-1].ToolRuns {
		if !r.OK {
			t.Fatalf("tool run %+v not OK after convergence", r)
		}
	}
	found := false
	for _, f := range rec.EditedFiles {
		if strings.Contains(f, "main_test.go") {
			found = true
		}
	}
	if !found {
		t.Fatalf("EditedFiles=%v, want main_test.go", rec.EditedFiles)
	}
	if len(rec.Followups) != 0 {
		t.Fatalf("Followups=%v on success, want none", rec.Followups)
	}
}

// A fix that needs two rounds converges on the second: the loop iterates
// instead of dying after one shot.
func TestRepairLoop_ConvergesOnSecondAttempt(t *testing.T) {
	h := newRepairHarness(t, nil)
	h.initial = func() *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{
			h.writeCall("c1", "write_file", filepath.Join(h.ws, "main.go"), repairMainGo),
			h.writeCall("c2", "write_file", filepath.Join(h.ws, "main_test.go"), repairTestBroken),
		}}
	}
	repairs := 0
	h.onRepair = func(call int, history []types.Message) *types.LLMToolResponse {
		repairs++
		content := repairTestBroken // first repair still broken
		if repairs > 1 {
			content = repairTestFixed
		}
		return &types.LLMToolResponse{Text: "attempt", ToolCalls: []types.ToolCall{
			h.writeCall("r1", "write_file", filepath.Join(h.ws, "main_test.go"), content),
		}}
	}
	result, err := h.drive(t, "fix make the failing test pass")
	if err != nil {
		t.Fatalf("ProcessWithIntent: %v", err)
	}
	rec := result.TestCheck.Repair
	if rec == nil || !rec.Passed {
		t.Fatalf("repair record = %+v, want passed", rec)
	}
	if rec.Cost.Attempts != 2 {
		t.Fatalf("attempts=%d, want 2", rec.Cost.Attempts)
	}
	if rec.Cost.Backtracks != 1 {
		t.Fatalf("backtracks=%d, want 1", rec.Cost.Backtracks)
	}
}

// The attempt budget is hard: with one round allowed, a two-round fix gives
// up with outcome false and actionable follow-ups.
func TestRepairLoop_RespectsAttemptBudget(t *testing.T) {
	h := newRepairHarness(t, func(e *Executor) { e.config.RepairMaxAttempts = 1 })
	h.initial = func() *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{
			h.writeCall("c1", "write_file", filepath.Join(h.ws, "main.go"), repairMainGo),
			h.writeCall("c2", "write_file", filepath.Join(h.ws, "main_test.go"), repairTestBroken),
		}}
	}
	repairs := 0
	h.onRepair = func(call int, history []types.Message) *types.LLMToolResponse {
		repairs++
		content := repairTestBroken
		if repairs > 1 {
			content = repairTestFixed
		}
		return &types.LLMToolResponse{Text: "attempt", ToolCalls: []types.ToolCall{
			h.writeCall("r1", "write_file", filepath.Join(h.ws, "main_test.go"), content),
		}}
	}
	_, err := h.drive(t, "fix make the failing test pass")
	if err == nil {
		t.Fatal("expected give-up error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "attempts=1") && !strings.Contains(msg, "after 1 attempt") {
		t.Fatalf("error does not report the exhausted budget: %v", err)
	}
	if !strings.Contains(msg, "go test") {
		t.Fatalf("error lacks actionable follow-up: %v", err)
	}
}

// A type error (parses, does not compile) exercises the build gate: the
// syntax gate lets it through, the repair loop fixes it.
func TestRepairLoop_FixesBuildBreak(t *testing.T) {
	h := newRepairHarness(t, nil)
	h.initial = func() *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{
			h.writeCall("c1", "write_file", filepath.Join(h.ws, "main.go"), repairMainTypeBroken),
		}}
	}
	h.onRepair = func(call int, history []types.Message) *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "fixed", ToolCalls: []types.ToolCall{
			h.writeCall("r1", "write_file", filepath.Join(h.ws, "main.go"), repairMainTypeFixed),
		}}
	}
	result, err := h.drive(t, "fix make the package build")
	if err != nil {
		t.Fatalf("ProcessWithIntent: %v", err)
	}
	if !result.BuildCheck.OK || result.BuildCheck.Verdict() != VerifyPassed {
		t.Fatalf("BuildCheck = %+v, want passed", result.BuildCheck)
	}
	if result.BuildCheck.Repair == nil || !result.BuildCheck.Repair.Passed {
		t.Fatalf("build repair record = %+v, want passed", result.BuildCheck.Repair)
	}
}

// Give-up record shape, driven one level down so the record stays observable
// (the production caller returns nil result on turn error; the error text
// carries cost and follow-ups, asserted above).
func TestRepairLoop_GiveUpRecord(t *testing.T) {
	ws := t.TempDir()
	t.Setenv("CODENERD_WORKSPACE_ROOT", ws)
	reg := tools.NewRegistry()
	if err := toolscore.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tools.SwapGlobal(reg))
	if err := os.WriteFile(filepath.Join(ws, "go.mod"), []byte("module repairprobe\n\ngo 1.25\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(ws, "main_test.go")
	if err := os.WriteFile(broken, []byte(repairTestBroken), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "main.go"), []byte(repairMainGo), 0o600); err != nil {
		t.Fatal(err)
	}
	mockLLM := &MockToolResultsLLM{
		MockLLMClient: &MockLLMClient{},
		CompleteWithToolResultsFunc: func(ctx context.Context, sys string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return &types.LLMToolResponse{Text: "still broken", ToolCalls: []types.ToolCall{
				{ID: "r1", Name: "write_file", Input: map[string]any{"path": broken, "content": repairTestBroken}},
			}}, nil
		},
	}
	executor := NewExecutor(&MockKernel{}, &testExecutiveStore{}, mockLLM, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	executor.config.WorkspaceRoot = ws
	executor.config.EnableSafetyGate = false
	executor.config.RepairMaxAttempts = 2
	result := &ExecutionResult{WrittenPaths: []string{"main_test.go", "main.go"}, SuccessfulWriteTools: 1}
	history := []types.Message{{Role: "user", Text: "fix the tests"}}
	allowCfg := &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: []string{"write_file", "read_file", "edit_file"}}
	_, _, err := executor.verifyAndRepairTests(context.Background(), mockLLM, "", history, nil, allowCfg, result)
	if err == nil {
		t.Fatal("expected give-up error, got nil")
	}
	rec := result.TestCheck.Repair
	if rec == nil {
		t.Fatal("no repair record on give-up")
	}
	if rec.Passed {
		t.Fatal("give-up record Passed=true")
	}
	if result.TestCheck.OK || result.TestCheck.Verdict() != VerifyFailed {
		t.Fatalf("TestCheck = %+v, want failed", result.TestCheck)
	}
	if rec.Cost.Attempts != 2 || len(rec.Attempts) != 2 {
		t.Fatalf("attempts=%d records=%d, want 2/2", rec.Cost.Attempts, len(rec.Attempts))
	}
	if rec.Cost.Backtracks != 2 {
		t.Fatalf("backtracks=%d, want 2", rec.Cost.Backtracks)
	}
	if rec.Attempts[0].Index != 1 || rec.Attempts[1].Index != 2 {
		t.Fatal("attempt indexes not ordered 1,2")
	}
	if len(rec.Followups) == 0 || !strings.Contains(strings.Join(rec.Followups, " "), "go test") {
		t.Fatalf("Followups=%v, want actionable go test command", rec.Followups)
	}
	if len(rec.EditedFiles) == 0 {
		t.Fatal("EditedFiles empty on give-up")
	}
	if rec.InitialFailure == "" || !strings.Contains(rec.InitialFailure, "FAIL") {
		t.Fatal("InitialFailure does not retain the failing output")
	}
}

// A canceled parent kills the episode before the first attempt: cancel
// supremacy survives the independent episode budget.
func TestRepairLoop_CancelBeforeStart(t *testing.T) {
	ws := t.TempDir()
	executor := NewExecutor(&MockKernel{}, &testExecutiveStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	executor.config.WorkspaceRoot = ws
	executor.config.EnableSafetyGate = false
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := &ExecutionResult{WrittenPaths: []string{"x.go"}, SuccessfulWriteTools: 1}
	history := []types.Message{{Role: "user", Text: "fix"}}
	spec := repairSpec{
		kind:      "tests",
		promptFor: testRepairPrompt,
		recheck: func(ctx context.Context) (bool, string, VerifyOutcome) {
			t.Fatal("recheck must not run on a canceled episode")
			return false, "", VerifyCanceled
		},
		followups: func() []string { return []string{"go test ./..."} },
	}
	_, _, rec, err := executor.repairLoop(ctx, &MockToolResultsLLM{MockLLMClient: &MockLLMClient{}}, "", &history, nil, nil, result, "FAIL", spec)
	if err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("err=%v, want cancel", err)
	}
	if rec == nil || rec.Cost.Attempts != 0 {
		t.Fatalf("record=%+v, want 0 attempts", rec)
	}
}

// An already-expired parent deadline does not apply: the episode keeps its
// own budget (this is the run-4 lesson — verification at the deadline edge
// still gets its repair).
func TestRepairEpisodeContext_ExpiredDeadlineKeepsBudget(t *testing.T) {
	parent, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	epCtx, stop := repairEpisodeContext(parent, time.Minute)
	defer stop()
	if epCtx.Err() != nil {
		t.Fatalf("episode ctx already done: %v", epCtx.Err())
	}
	if _, ok := epCtx.Deadline(); !ok {
		t.Fatal("episode ctx has no deadline of its own")
	}
}

func TestRepairEpisodeContext_CancelKillsFast(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	epCtx, stop := repairEpisodeContext(parent, time.Minute)
	defer stop()
	if epCtx.Err() != context.Canceled {
		t.Fatalf("episode ctx err=%v, want canceled", epCtx.Err())
	}
}

func TestRepairEpisodeContext_LiveDeadlineRespected(t *testing.T) {
	parent, cancel := context.WithDeadline(context.Background(), time.Now().Add(50*time.Millisecond))
	defer cancel()
	epCtx, stop := repairEpisodeContext(parent, time.Minute)
	defer stop()
	deadline, ok := epCtx.Deadline()
	if !ok {
		t.Fatal("episode ctx has no deadline")
	}
	if time.Until(deadline) > 5*time.Second {
		t.Fatalf("episode deadline %v ignores the live parent deadline", deadline)
	}
}
