package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/learnjudge"
	"codenerd/internal/perception"
	"codenerd/internal/types"
	"codenerd/internal/typesafe"
)

// liveReportJudge is the repository's configured Jev, recording every report
// it reads. The /finish round's behaviour depends on how Jev reads a report,
// so it is tested against Jev itself; a workspace with no typesafe section
// skips.
type recordingAsker struct {
	next    learnjudge.Asker
	reports []string
}

func (r *recordingAsker) Ask(ctx context.Context, state any, questions map[string]typesafe.Question) (*typesafe.Response, error) {
	if m, ok := state.(map[string]any); ok {
		report, _ := m["report"].(string)
		r.reports = append(r.reports, report)
	}
	return r.next.Ask(ctx, state, questions)
}

func liveReportJudge(t *testing.T) (*ReportJudge, *recordingAsker) {
	t.Helper()
	cfg, err := config.LoadUserConfig("../../.nerd/config.json")
	if err != nil || cfg == nil || cfg.TypeSafe == nil {
		t.Skipf("no typesafe section in .nerd/config.json: %v", err)
	}
	client, err := typesafe.FromConfig(cfg.TypeSafe)
	if err != nil || client == nil {
		t.Skipf("typesafe section does not resolve: %v", err)
	}
	rec := &recordingAsker{next: client}
	return &ReportJudge{Asker: rec, Config: cfg.TypeSafe}, rec
}

// A turn whose report says part of the task is not done is pushed back to the
// model, which goes on with its tools, unless the report names an obstacle
// codeNERD cannot clear, or a push-back makes no progress. Driven through the
// production chain with real go test on a scratch module, and live Jev reading
// the reports.
func TestFinishRound_UnfinishedWorkIsPushedBack(t *testing.T) {
	const code = "package main\n\nfunc Double(x int) int {\n\treturn x * 2\n}\n\nfunc main() {}\n"
	const before = "package main\n\nimport \"testing\"\n\nfunc TestDouble(t *testing.T) {\n\tif Double(2) != 4 {\n\t\tt.Fatal(\"Double(2) != 4\")\n\t}\n}\n"
	const withNegative = before + "\nfunc TestNegative(t *testing.T) {\n\tif Double(-1) != -2 {\n\t\tt.Fatal(\"Double(-1) != -2\")\n\t}\n}\n"
	const withZero = withNegative + "\nfunc TestZero(t *testing.T) {\n\tif Double(0) != 0 {\n\t\tt.Fatal(\"Double(0) != 0\")\n\t}\n}\n"

	drive := func(t *testing.T, report, onPushback string) (*ExecutionResult, *recordingAsker, int, string) {
		t.Helper()
		judge, jev := liveReportJudge(t)
		h := newRepairHarness(t, func(e *Executor) { e.SetReportJudge(judge) })
		for name, content := range map[string]string{"main.go": code, "main_test.go": before} {
			if err := os.WriteFile(filepath.Join(h.ws, name), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		testPath := filepath.Join(h.ws, "main_test.go")
		pushbacks := 0
		h.executor.llmClient = &MockToolResultsLLM{
			MockLLMClient: &MockLLMClient{
				CompleteWithToolsFunc: func(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
					return &types.LLMToolResponse{ToolCalls: []types.ToolCall{h.writeCall("c1", "write_file", testPath, withNegative)}}, nil
				},
			},
			CompleteWithToolResultsFunc: func(_ context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
				last := history[len(history)-1].Text
				switch {
				case strings.Contains(last, "Your report says part of the task is not done"):
					pushbacks++
					if onPushback == "write" {
						return &types.LLMToolResponse{ToolCalls: []types.ToolCall{h.writeCall("c2", "write_file", testPath, withZero)}}, nil
					}
					return &types.LLMToolResponse{Text: "I still did not get to TestZero."}, nil
				case strings.Contains(last, "Keep going"):
					return &types.LLMToolResponse{Text: "Added TestZero too; the task is done."}, nil
				}
				// The turn's first call comes through here too; once its
				// write's result is in, the model reports.
				for _, m := range history {
					if len(m.ToolResults) > 0 {
						return &types.LLMToolResponse{Text: report}, nil
					}
				}
				return &types.LLMToolResponse{ToolCalls: []types.ToolCall{h.writeCall("c1", "write_file", testPath, withNegative)}}, nil
			},
		}
		result, err := h.drive(t, "fix add TestNegative and TestZero for Double")
		if err != nil {
			t.Fatalf("drive: %v", err)
		}
		return result, jev, pushbacks, h.ws
	}

	t.Run("pushedBackAndFinished", func(t *testing.T) {
		result, jev, pushbacks, ws := drive(t, "Added TestNegative. I did not get to TestZero.", "write")
		if pushbacks != 1 {
			t.Fatalf("pushed back %d time(s), want once; reports read: %q", pushbacks, jev.reports)
		}
		got, err := os.ReadFile(filepath.Join(ws, "main_test.go"))
		if err != nil || !strings.Contains(string(got), "func TestZero") {
			t.Fatalf("the finished work is not in the file: %v\n%s", err, got)
		}
		if result.SelfReportedIncomplete {
			t.Fatalf("the finished turn is still read as incomplete; reports read: %q", jev.reports)
		}
		if !strings.Contains(result.Response, "the task is done") {
			t.Fatalf("the turn's answer is %q, want the finished report", result.Response)
		}
	})

	t.Run("aNamedBlockerIsNotPushed", func(t *testing.T) {
		result, _, pushbacks, _ := drive(t, "Added TestNegative. I could not add TestZero: writing main_test.go a second time was refused by a permission denial I cannot clear.", "write")
		if pushbacks != 0 {
			t.Fatalf("pushed back %d time(s) over a named blocker", pushbacks)
		}
		if !result.SelfReportedIncomplete {
			t.Fatal("a turn that names a blocker is unfinished, and the verdict must say so")
		}
	})

	t.Run("aPushbackWithNoProgressStops", func(t *testing.T) {
		result, _, pushbacks, _ := drive(t, "Added TestNegative. I did not get to TestZero.", "talk")
		if pushbacks != 1 {
			t.Fatalf("pushed back %d time(s); an attempt that writes nothing ends the round", pushbacks)
		}
		if !result.SelfReportedIncomplete {
			t.Fatal("the work is still unfinished, and the verdict must say so")
		}
	})
}

// A /finish write re-arms the gate rounds, so what it finished is built and
// tested; the advisory rounds are not repeated.
func TestFinishRound_AWriteRearmsTheGates(t *testing.T) {
	t.Parallel()
	e := roundsExecutor(t)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}, WriteToolEffects: 1, WrittenPaths: []string{"internal/x/x.go"}}
	schedule(t, e, result)
	e.rearmGateRounds(result)
	got := schedule(t, e, result)
	want := []string{"/build", "/test", "/coverage", "/pinned", "/vet", "/removed_tests", "/unwired"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("after a /finish write the schedule ran %v, want %v", got, want)
	}
}

// The kernel's truth table for the push-back (policy/turn_finish.mg).
func TestFinishPolicy_PushbackTruthTable(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                string
		unfinished, blocked int64
		idle, want          bool
	}{
		{"unfinished and no blocker", 79, 16, false, true},
		{"an unsettled blocker still pushes", 99, 34, false, true},
		{"a named blocker stops it", 97, 98, false, false},
		{"done", 6, 31, false, false},
		{"an attempt that wrote nothing stops it", 99, 3, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := roundsExecutor(t)
			result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}
			if err := config.EnsureParams(e.kernel, (&config.TypeSafeConfig{}).Params()); err != nil {
				t.Fatal(err)
			}
			turn := result.turnAtom()
			facts := []types.Fact{{Predicate: learnjudge.ReportReadingPredicate, Args: []any{turn, tc.unfinished, tc.blocked}}}
			if tc.idle {
				facts = append(facts, types.Fact{Predicate: "turn_finish_idle", Args: []any{turn}})
			}
			for _, f := range facts {
				if !e.assertTurnFact(f) {
					t.Fatalf("assert %v", f)
				}
			}
			if got := e.turnHolds("turn_finish_pushback", result); got != tc.want {
				t.Fatalf("turn_finish_pushback = %v, want %v", got, tc.want)
			}
		})
	}
}
