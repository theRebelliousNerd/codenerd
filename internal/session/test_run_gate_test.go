package session

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/core"
	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/tools"
	toolscore "codenerd/internal/tools/core"
	"codenerd/internal/types"
)

// External audit N01 (2026-09-19): a write the Go gates do not cover owes a
// test run that passed after the turn's last write. The measurement is the
// last test process the tool layer recorded since the last successful write:
// a run before a write says nothing about what the write left, and the run
// the model started last is the one its answer rests on.
func TestTestRunGate_IsTheLastRunAfterTheLastWrite(t *testing.T) {
	ws := t.TempDir()
	t.Setenv("CODENERD_WORKSPACE_ROOT", ws)
	reg := tools.NewRegistry()
	if err := toolscore.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(&tools.Tool{
		Effect: tools.EffectRead, Name: "probe_test_run", Category: tools.CategoryTest,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			code, _ := args["exit"].(int)
			tools.RecordTestRun(ctx, tools.TestRun{Argv: []string{"pytest"}, ExitCode: code})
			return "ran", nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tools.SwapGlobal(reg))

	write := func(id string) types.ToolCall {
		return types.ToolCall{ID: id, Name: "write_file", Input: map[string]any{"path": filepath.Join(ws, "app.py"), "content": "x = 1\n"}}
	}
	run := func(id string, exit int) types.ToolCall {
		return types.ToolCall{ID: id, Name: "probe_test_run", Input: map[string]any{"exit": exit}}
	}
	cfg := &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: []string{"write_file", "probe_test_run"}}

	for _, tc := range []struct {
		name  string
		calls []types.ToolCall
		want  VerifyOutcome
	}{
		{"a run before the write is no verdict", []types.ToolCall{run("r", 0), write("w")}, VerifySkipped},
		{"the last run after the write passed", []types.ToolCall{write("w"), run("r1", 1), run("r2", 0)}, VerifyPassed},
		{"the last run after the write failed", []types.ToolCall{write("w"), run("r1", 0), run("r2", 1)}, VerifyFailed},
		{"a later write sets the verdict aside", []types.ToolCall{write("w1"), run("r", 0), write("w2")}, VerifySkipped},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := NewExecutor(&MockKernel{}, &testExecutiveStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
			e.config.WorkspaceRoot = ws
			e.config.EnableSafetyGate = false
			result := &ExecutionResult{}
			for _, call := range tc.calls {
				e.executeToolBatch(context.Background(), []types.ToolCall{call}, cfg, result)
			}
			if result.SuccessfulWriteTools == 0 {
				t.Fatal("no write succeeded; the probe measured nothing")
			}
			if got := result.testRunVerdict(); got != tc.want {
				t.Fatalf("test run gate = %v, want %v", got, tc.want)
			}
		})
	}
}

// The corpus decides what a write owes from its extension, so the executor
// asserts every written path with it.
func TestAssertTurnEvidence_NamesEveryWrittenPathWithItsExtension(t *testing.T) {
	kernel := &MockKernel{}
	e := NewExecutor(kernel, &testExecutiveStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	turn := types.MangleAtom("/turn_written_probe")
	e.assertTurnEvidence(turn, "/fix", &ExecutionResult{
		SuccessfulWriteTools: 2,
		WrittenPaths:         []string{"Docs/Guide.MD", "internal/core/defaults/policy/x.mg"},
	})

	facts, err := kernel.Query("turn_written")
	if err != nil {
		t.Fatalf("query turn_written: %v", err)
	}
	var got []string
	for _, f := range facts {
		if len(f.Args) != 3 || types.ExtractString(f.Args[0]) != string(turn) {
			t.Fatalf("turn_written row %v is not keyed by the turn", f.Args)
		}
		got = append(got, types.ExtractString(f.Args[1])+" "+types.ExtractString(f.Args[2]))
	}
	slices.Sort(got)
	want := []string{"Docs/Guide.MD .md", "internal/core/defaults/policy/x.mg .mg"}
	if !slices.Equal(got, want) {
		t.Fatalf("turn_written = %v, want %v", got, want)
	}
}

// A write under a path the workspace's nerd.md declares as docs is marked
// turn_doc_write, which is how the corpus tells a docs corpus.toml from a
// config file of the same extension; a write outside it is not marked.
func TestAssertTurnEvidence_MarksWritesUnderADeclaredDocsPath(t *testing.T) {
	root := t.TempDir()
	nerdmd := "---\nschema: nerd/v1\nproject: probe\nlanguage: go\ndocs:\n  - Docs\n---\n"
	if err := os.WriteFile(filepath.Join(root, "nerd.md"), []byte(nerdmd), 0o644); err != nil {
		t.Fatal(err)
	}
	kernel := &MockKernel{}
	e := NewExecutor(kernel, &testExecutiveStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	e.SetConfig(ExecutorConfig{WorkspaceRoot: root})
	turn := types.MangleAtom("/turn_doc_probe")
	e.assertTurnEvidence(turn, "/fix", &ExecutionResult{
		SuccessfulWriteTools: 2,
		WrittenPaths: []string{
			filepath.Join(root, "Docs", "architecture", "corpus.toml"),
			filepath.Join(root, "config", "corpus.toml"),
		},
	})

	facts, err := kernel.Query("turn_doc_write")
	if err != nil {
		t.Fatalf("query turn_doc_write: %v", err)
	}
	if len(facts) != 1 || !strings.Contains(filepath.ToSlash(types.ExtractString(facts[0].Args[1])), "Docs/architecture/corpus.toml") {
		t.Fatalf("turn_doc_write = %v, want only the write under Docs", facts)
	}
}

// The executor's verdict for writes the Go gates never run on, end to end
// through the shipped corpus: a document is done on execution, a policy file
// is done on a passing test run after it and names the run it lacks otherwise,
// and the model can neither assert a write nor reclassify its extension.
func TestTurnOutcome_AWriteTheGoGatesDoNotCoverHasACompletionPath(t *testing.T) {
	missing := func(result *ExecutionResult, atom string) bool {
		return slices.Contains(result.MissingEvidence, atom)
	}

	t.Run("aDocumentIsDoneOnExecution", func(t *testing.T) {
		e := newObligationExec(t)
		result := mutationResult()
		result.WrittenPaths = []string{"Docs/guide.md"}
		e.assertTurnEvidence(testTurn, "/fix", result)
		e.captureTurnOutcome(testTurn, result, nil)
		if result.TurnOutcome != types.MangleAtom("/done") {
			t.Fatalf("TurnOutcome = %v (missing %v), want /done: nothing compiles or runs a document", result.TurnOutcome, result.MissingEvidence)
		}
	})

	t.Run("aPolicyFileWithNoRunAfterItNamesTheRun", func(t *testing.T) {
		e := newObligationExec(t)
		result := mutationResult()
		result.WrittenPaths = []string{"internal/core/defaults/policy/x.mg"}
		e.assertTurnEvidence(testTurn, "/fix", result)
		e.captureTurnOutcome(testTurn, result, nil)
		if result.TurnOutcome == types.MangleAtom("/done") {
			t.Fatal("TurnOutcome = /done for a policy edit no test ran after")
		}
		if !missing(result, "/test_run_not_green") || missing(result, "/build_not_green") {
			t.Fatalf("MissingEvidence = %v, want the test run named and no Go gate", result.MissingEvidence)
		}
	})

	t.Run("aPolicyFileWithAPassingRunAfterItIsDone", func(t *testing.T) {
		e := newObligationExec(t)
		result := mutationResult()
		result.WrittenPaths = []string{"internal/core/defaults/policy/x.mg"}
		result.TestRunSinceLastWrite = &tools.TestRun{Argv: []string{"go", "test", "./internal/core/..."}, ExitCode: 0}
		e.assertTurnEvidence(testTurn, "/fix", result)
		e.captureTurnOutcome(testTurn, result, nil)
		if result.TurnOutcome != types.MangleAtom("/done") {
			t.Fatalf("TurnOutcome = %v (missing %v), want /done after a passing test run", result.TurnOutcome, result.MissingEvidence)
		}
	})

	t.Run("theModelCannotAssertAWriteOrReclassifyAnExtension", func(t *testing.T) {
		permissive := core.MangleUpdatePolicy{AllowedPrefixes: []string{""}}
		for _, update := range []string{
			`write_class(".py", /doc).`,
			`turn_written(/turn_x, "a.py", ".md").`,
			"turn_owes_gate(/turn_x, /test_run).",
			"turn_unmet_gate(/turn_x, /test_run).",
			"has_unmet_gate(/turn_x).",
		} {
			if kept, _ := core.FilterMangleUpdates(nil, []string{update}, permissive); len(kept) != 0 {
				t.Errorf("the model can assert %s; what a write owes must be the harness's alone", update)
			}
		}
	})
}
