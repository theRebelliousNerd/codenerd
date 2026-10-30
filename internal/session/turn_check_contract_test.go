package session

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"codenerd/internal/core"
	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/sessionlog"
	"codenerd/internal/tactile"
	"codenerd/internal/tools"
	toolscore "codenerd/internal/tools/core"
	toolshell "codenerd/internal/tools/shell"
	"codenerd/internal/types"
)

type contractCheckExecutor struct {
	exit     int
	err      error
	commands []tactile.Command
}

func (e *contractCheckExecutor) Execute(_ context.Context, command tactile.Command) (*tactile.ExecutionResult, error) {
	e.commands = append(e.commands, command)
	return &tactile.ExecutionResult{ExitCode: e.exit, Stdout: "fixture observation"}, e.err
}
func (*contractCheckExecutor) Validate(tactile.Command) error { return nil }
func (*contractCheckExecutor) Capabilities() tactile.ExecutorCapabilities {
	return tactile.ExecutorCapabilities{}
}

// Uses the real run_check tool through session dispatch, raw receipt ingestion,
// shipped Mangle and delegated progress. No model judgment supplies the verdict.
func TestCheckContract_ProductionDispatchPreservesDeclaredPassSemantics(t *testing.T) {
	reg := tools.NewRegistry()
	if err := toolscore.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(toolshell.RunCheckTool()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tools.SwapGlobal(reg))
	cfg := &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: []string{"write_file", "run_check"}}
	for _, tc := range []struct {
		name           string
		codes          []int
		exit           int
		err            error
		laterWrite     bool
		want           VerifyOutcome
		progressFailed bool
	}{
		{"declared nonzero pass", []int{0, 3}, 3, nil, false, VerifyPassed, false},
		{"real failure", []int{0, 3}, 1, nil, false, VerifyFailed, true},
		{"omitted rule remains zero only", nil, 3, nil, false, VerifyFailed, true},
		{"default zero", nil, 0, nil, false, VerifyPassed, false},
		{"infrastructure error cannot pass", []int{0, 3}, 0, errors.New("executor failed"), false, VerifyFailed, true},
		{"later write invalidates a nonzero pass", []int{0, 3}, 3, nil, true, VerifySkipped, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			t.Setenv("CODENERD_WORKSPACE_ROOT", ws)
			kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			e := NewExecutor(kernel, &testExecutiveStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
			e.config.WorkspaceRoot = ws
			e.config.EnableSafetyGate = false
			defer e.cleanupTurnFacts()
			exec := &contractCheckExecutor{exit: tc.exit, err: tc.err}
			toolshell.SetCheckExecutor(exec)
			defer toolshell.ClearCheckExecutor(exec)
			ctx := tools.WithCampaignCheck(context.Background(), tools.CampaignCheck{
				CampaignID: "/campaign_contract", TaskID: "/task_contract", Argv: []string{"checker"},
				OKExitCodes: tc.codes, Workspace: ws, Environment: []string{"CHECK_PROFILE=strict"},
			})
			var events []sessionlog.LiveEvent
			ctx = sessionlog.WithAgent(sessionlog.WithLiveProgress(ctx, func(_ context.Context, event sessionlog.LiveEvent) { events = append(events, event) }), "coder", "contract")
			result := &ExecutionResult{}
			write := func(id, body string) {
				_, errs := e.executeToolBatch(ctx, []types.ToolCall{{ID: id, Name: "write_file", Input: map[string]any{"path": filepath.Join(ws, "notes.md"), "content": body}}}, cfg, result)
				if len(errs) != 0 {
					t.Fatalf("write: %v", errs)
				}
			}
			write("w1", "first\n")
			if result.WriteToolEffects == 0 {
				t.Fatal("fixture made no write")
			}
			e.executeToolBatch(ctx, []types.ToolCall{{ID: "check", Name: "run_check", Input: map[string]any{}}}, cfg, result)
			if tc.laterWrite {
				write("w2", "second\n")
			}
			if got := derivedVerify(t, e, result.turnAtom(), "/check"); got != tc.want {
				t.Fatalf("check = %v, want %v", got, tc.want)
			}
			if len(exec.commands) != 1 || !slices.Contains(exec.commands[0].Environment, "CHECK_PROFILE=strict") {
				t.Fatal("dispatch lost the declared overlay or did not execute exactly once")
			}
			rows, err := kernel.Query("turn_check_run")
			if err != nil || len(rows) != 1 {
				t.Fatalf("raw receipts = %v, %v", rows, err)
			}
			if code, ok := types.ExtractInt64(rows[0].Args[2]); !ok || code != int64(tc.exit) {
				t.Fatalf("raw exit rewritten: %v", rows)
			}
			var seen bool
			for _, event := range events {
				if event.Kind != sessionlog.EventToolResult || event.Tool != "run_check" {
					continue
				}
				seen = true
				want := "PASS"
				if tc.progressFailed {
					want = "FAIL"
				}
				if event.Status != want || event.Failed != tc.progressFailed {
					t.Fatalf("progress = %+v, want %s", event, want)
				}
			}
			if !seen {
				t.Fatal("no live check progress event")
			}
		})
	}
}

func TestCheckContract_ReceiptRulesAreScopedAndProtected(t *testing.T) {
	e := newObligationExec(t)
	defer e.cleanupTurnFacts()
	first, second := &ExecutionResult{}, &ExecutionResult{}
	if !e.noteCheckRun(first, tools.AcceptanceRun{ExitCode: 3, OKExitCodes: []int{0, 3}}) {
		t.Fatal("declared pass rejected")
	}
	if e.noteCheckRun(second, tools.AcceptanceRun{ExitCode: 3}) {
		t.Fatal("another turn inherited the first pass set")
	}
	if e.noteCheckRun(first, tools.AcceptanceRun{ExitCode: 3}) {
		t.Fatal("later invocation inherited the earlier pass set")
	}
	for _, update := range []string{
		"turn_check_ok_exit(/turn_x, 1, 3).", "turn_check_has_exit_rule(/turn_x, 1).",
		"turn_check_error(/turn_x, 1).", "turn_check_passed(/turn_x, 1).",
	} {
		kept, blocked := core.FilterMangleUpdates(e.kernel, []string{update}, core.MangleUpdatePolicy{AllowedPrefixes: []string{""}})
		if len(kept) != 0 || len(blocked) != 1 {
			t.Fatalf("model may assert %s: kept=%v blocked=%v", update, kept, blocked)
		}
	}
}
