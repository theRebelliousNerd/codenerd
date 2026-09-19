package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

// A finding is what it says about a file, not where it sits: an edit above it
// moves its line. The turn's findings are the ones vet did not report before
// the turn, counted, so a second copy of an old finding is still new. On
// Windows vet prints backslashed, workspace-relative paths under a "# pkg"
// header, and lines without a position are not findings.
func TestNewVetFindings_AFindingIsItsFileAndMessageNotItsLine(t *testing.T) {
	ws := t.TempDir()
	resolve := func(p string) string { return workspaceFile(ws, p) }
	before := vetDiagnostics(strings.Join([]string{
		"# codenerd/cmd/nerd",
		`cmd\nerd\cmd_mangle_check.go:286:2: unreachable code`,
		"vet: some diagnostic without a position",
	}, "\n"), resolve)
	now := vetDiagnostics(strings.Join([]string{
		"# codenerd/cmd/nerd",
		`cmd\nerd\cmd_mangle_check.go:301:2: unreachable code`,
		`cmd\nerd\cmd_mangle_check.go:340:2: unreachable code`,
		"# codenerd/internal/widget",
		"internal/widget/use.go:9:1: Use passes lock by value: widget.State contains sync.Mutex",
	}, "\n"), resolve)

	got := newVetFindings(now, before)

	want := []string{
		`cmd\nerd\cmd_mangle_check.go:340:2: unreachable code`,
		"internal/widget/use.go:9:1: Use passes lock by value: widget.State contains sync.Mutex",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("newVetFindings =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// vetWorkspace writes a module under a fresh workspace (slash paths) and
// returns the workspace.
func vetWorkspace(t *testing.T, files map[string]string) string {
	t.Helper()
	if testing.Short() {
		t.Skip("needs the go toolchain")
	}
	return guardWorkspace(t, files)
}

const (
	vetStateBefore = "package p\n\ntype State struct {\n\tN int\n}\n"
	vetStateMutex  = "package p\n\nimport \"sync\"\n\ntype State struct {\n\tMu sync.Mutex\n\tN  int\n}\n"
	vetUse         = "package p\n\nfunc Consume(s State) int { return s.N }\n"
	vetUnreachable = "\nfunc Old() int {\n\treturn 1\n\tprintln(\"never\")\n\treturn 2\n}\n"
)

// External audit F5 (2026-09-19): the turn adds a lock to State in state.go;
// vet reports it in use.go, which the turn did not touch. The gate kept only
// findings in written files and called this a pass.
func TestVerifyVet_ACauseInAWrittenFileReportedInAnother(t *testing.T) {
	ws := vetWorkspace(t, map[string]string{"go.mod": "module vetprobe\n\ngo 1.25\n", "p/state.go": vetStateMutex, "p/use.go": vetUse})

	v := verifyVet(context.Background(), ws, []string{"p/state.go"}, map[string]PreImage{"p/state.go": existed(vetStateBefore)})

	if v.Verdict() != VerifyFailed || !strings.Contains(v.Output, "use.go") || !strings.Contains(v.Output, "passes lock by value") {
		t.Fatalf("verifyVet = %+v, want failed naming use.go's copied lock", v)
	}
}

// A finding that was there before the turn is the workspace's, in a file the
// turn wrote -- where the turn's edit moved its line and vet prints the
// baseline's copy under the overlay's temporary path -- or in one it did not.
func TestVerifyVet_AFindingThatPredatesTheTurnIsNotCharged(t *testing.T) {
	stateBefore := vetStateBefore + vetUnreachable
	stateNow := "package p\n\n// State is the probe's state.\ntype State struct {\n\tN int\n\tM int\n}\n" + vetUnreachable
	ws := vetWorkspace(t, map[string]string{"go.mod": "module vetprobe\n\ngo 1.25\n", "p/state.go": stateNow, "p/use.go": vetUse + strings.Replace(vetUnreachable, "Old", "Older", 1)})

	v := verifyVet(context.Background(), ws, []string{"p/state.go"}, map[string]PreImage{"p/state.go": existed(stateBefore)})

	if v.Verdict() != VerifyPassed {
		t.Fatalf("verifyVet = %+v, want passed: both findings were there before the turn", v)
	}
}

// Without a baseline the gate cannot tell old from new, and attribution it
// cannot make is not a pass: every finding in the packages is charged.
func TestVerifyVet_WithoutABaselineEveryFindingIsCharged(t *testing.T) {
	ws := vetWorkspace(t, map[string]string{"go.mod": "module vetprobe\n\ngo 1.25\n", "p/state.go": vetStateBefore, "p/use.go": vetUse + vetUnreachable})

	v := verifyVet(context.Background(), ws, []string{"p/state.go"}, map[string]PreImage{"p/state.go": {Unknown: "permission denied"}})

	if v.Verdict() != VerifyFailed || !strings.Contains(v.Output, "use.go") || !strings.Contains(v.Reason, "no pre-turn vet") {
		t.Fatalf("verifyVet = %+v, want failed naming use.go, with the missing baseline as the reason", v)
	}
}

// A vet run that fails without naming a finding is not evidence of anything,
// and in particular not a pass.
func TestVerifyVet_AFailureThatNamesNoFindingIsNotAPass(t *testing.T) {
	stubVerifySeams(t, time.Minute, time.Minute, func(context.Context, string, []string, string, []string) ([]byte, error) {
		return []byte("go: error obtaining buildID for go tool vet: exit status 1\n"), errors.New("exit status 1")
	}, verifyPassAsRunner())
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "x.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	v := verifyVet(context.Background(), ws, []string{"x.go"}, map[string]PreImage{"x.go": {}})

	if v.Verdict() == VerifyPassed || v.Verdict() == VerifyFailed {
		t.Fatalf("verifyVet = %+v, want neither passed nor failed: vet named no finding", v)
	}
}

// The repair prompt names every block. A summary capped at eight blocks
// ("... and 19 more") is a log line; a repair built from it is a repair of
// the first eight.
func TestUncoveredList_NamesEveryBlock(t *testing.T) {
	var blocks []UncoveredBlock
	for i := 0; i < 27; i++ {
		blocks = append(blocks, UncoveredBlock{File: "codenerd/cmd/nerd/cmd_mangle_check.go", StartLine: 100 + 3*i, EndLine: 101 + 3*i, NumStmts: 1})
	}
	list := uncoveredList(blocks, []string{"cmd/nerd/cmd_mangle_check.go"})
	for i := 0; i < 27; i++ {
		want := fmt.Sprintf("cmd/nerd/cmd_mangle_check.go:%d-%d", 100+3*i, 101+3*i)
		if !strings.Contains(list, want) {
			t.Fatalf("block %s missing from the repair list:\n%s", want, list)
		}
	}
	if strings.Contains(list, "more") {
		t.Fatalf("the repair list must not elide blocks:\n%s", list)
	}
}

// A forcing round that runs out of attempts leaves its debt to the verdict,
// which names it; it is not a failed turn. A cancel is still a cancel.
func TestSettleForcingRepair(t *testing.T) {
	if err := settleForcingRepair(nil); err != nil {
		t.Fatalf("nil stays nil, got %v", err)
	}
	gaveUp := fmt.Errorf("%w: changed code is executed by no test and the repair loop did not converge", ErrVerificationFailed)
	if err := settleForcingRepair(gaveUp); err != nil {
		t.Fatalf("a round that gave up must leave its debt to the verdict, got %v", err)
	}
	canceled := fmt.Errorf("repair canceled: %w", context.Canceled)
	if err := settleForcingRepair(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancel must propagate, got %v", err)
	}
}

// The forcing round end to end, through the production caller chain
// (ProcessWithIntent -> the tool loop -> verifyCompletedToolTurn) with real
// go build/test/coverage on a scratch module: a turn that writes a function
// no test executes is handed the lines and writes the test; a turn that will
// not write it ends with the debt on the result for the verdict to name --
// unverified, not failed.
func TestCoverageRound_TheModelWritesTheTestOrTheDebtStays(t *testing.T) {
	const code = `package main

func Double(x int) int {
	return x * 2
}

func main() {}
`
	const emptyTest = `package main

import "testing"

func TestProbe(t *testing.T) {}
`
	const doubleTest = `package main

import "testing"

func TestProbe(t *testing.T) {}

func TestDouble(t *testing.T) {
	if Double(2) != 4 {
		t.Fatal("Double(2) != 4")
	}
}
`
	// A test that runs the lines and asserts the wrong thing: the round's own
	// work turns a green suite red.
	const wrongTest = `package main

import "testing"

func TestProbe(t *testing.T) {}

func TestDouble(t *testing.T) {
	if Double(2) != 5 {
		t.Fatal("Double(2) != 5")
	}
}
`

	// answer: "test" writes doubleTest, "wrongTest" writes wrongTest, anything
	// else declines.
	drive := func(t *testing.T, answer string) (*ExecutionResult, string, error) {
		t.Helper()
		h := newRepairHarness(t, nil)
		initial := func() *types.LLMToolResponse {
			return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{
				h.writeCall("c1", "write_file", filepath.Join(h.ws, "main.go"), code),
				h.writeCall("c2", "write_file", filepath.Join(h.ws, "main_test.go"), emptyTest),
			}}
		}
		coveragePrompt := ""
		h.executor.llmClient = &MockToolResultsLLM{
			MockLLMClient: &MockLLMClient{
				CompleteWithToolsFunc: func(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
					return initial(), nil
				},
			},
			CompleteWithToolResultsFunc: func(_ context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
				if last := history[len(history)-1]; strings.Contains(last.Text, "no test executes these lines") {
					coveragePrompt = last.Text
					content := ""
					switch answer {
					case "test":
						content = doubleTest
					case "wrongTest":
						content = wrongTest
					default:
						return &types.LLMToolResponse{Text: "done"}, nil
					}
					return &types.LLMToolResponse{Text: "testing Double", ToolCalls: []types.ToolCall{
						h.writeCall(fmt.Sprintf("t%d", len(history)), "write_file", filepath.Join(h.ws, "main_test.go"), content),
					}}, nil
				}
				for _, m := range history {
					if len(m.ToolResults) > 0 {
						return &types.LLMToolResponse{Text: "done"}, nil
					}
				}
				return initial(), nil
			},
		}
		result, err := h.drive(t, "fix add a Double function")
		data, readErr := os.ReadFile(filepath.Join(h.ws, "main_test.go"))
		if readErr == nil && answer == "test" && !strings.Contains(string(data), "TestDouble") {
			t.Fatalf("the model's test did not land on disk:\n%s", data)
		}
		if readErr == nil && answer == "wrongTest" && string(data) != emptyTest {
			t.Fatalf("a round that gave up on a red suite must leave the test file as it found it:\n%s", data)
		}
		return result, coveragePrompt, err
	}

	t.Run("theModelWritesTheTest", func(t *testing.T) {
		result, prompt, err := drive(t, "test")
		if err != nil {
			t.Fatalf("ProcessWithIntent: %v", err)
		}
		if !strings.Contains(prompt, "main.go:3-5") {
			t.Fatalf("the round must hand the model the unexecuted lines by the path it wrote; prompt:\n%s", prompt)
		}
		if len(result.UncoveredBlocks) != 0 {
			t.Fatalf("after the model's test the changed code must be executed, still unexecuted: %+v", result.UncoveredBlocks)
		}
		if result.TestCheck.Verdict() != VerifyPassed {
			t.Fatalf("TestCheck = %+v, want passed", result.TestCheck)
		}
	})

	// Ladder run R1-4b (2026-09-19): the change passed build and tests, the
	// coverage round's three attempts wrote a test that failed, the round gave
	// up and left it in place, and the final re-verification failed the turn --
	// the round took down the change it was only meant to cover. A round that
	// gives up having turned a green suite red is undone; the debt it leaves is
	// the uncovered code, as when the model declines.
	t.Run("theModelsTestFailsAndTheRoundIsUndone", func(t *testing.T) {
		result, prompt, err := drive(t, "wrongTest")
		if err != nil {
			t.Fatalf("a coverage round must not fail the change it was covering: %v", err)
		}
		if prompt == "" {
			t.Fatal("the model was never handed the unexecuted lines")
		}
		if result.TestCheck.Verdict() != VerifyPassed {
			t.Fatalf("the suite must be as green as the round found it: TestCheck = %+v", result.TestCheck)
		}
		if len(result.UncoveredBlocks) == 0 {
			t.Fatal("the debt must stay on the result for the verdict to name")
		}
	})

	t.Run("theModelDoesNotAndTheDebtStays", func(t *testing.T) {
		result, prompt, err := drive(t, "decline")
		if err != nil {
			t.Fatalf("a coverage round that does not converge must not fail the turn: %v", err)
		}
		if prompt == "" {
			t.Fatal("the model was never handed the unexecuted lines")
		}
		if len(result.UncoveredBlocks) == 0 {
			t.Fatal("the debt must stay on the result for the verdict to name")
		}
	})
}

// The final re-verification names what failed. R1-4b's turn ended with "final
// workspace failed mechanical checks" and nothing more; the failing test was in
// the session log only.
func TestFailedChecksSummary_NamesWhatFailed(t *testing.T) {
	result := &ExecutionResult{
		BuildCheck: BuildVerification{Outcome: VerifyPassed},
		TestCheck: TestVerification{Outcome: VerifyFailed, Output: "--- FAIL: TestA (0.10s)\n" +
			"    a_test.go:3: boom\n--- FAIL: TestB (0.00s)\n    --- FAIL: TestB/sub (0.00s)\nFAIL\nFAIL\tpkg\t0.2s\n"},
	}
	if got, want := failedChecksSummary(result), "tests fail: TestA, TestB"; got != want {
		t.Fatalf("failedChecksSummary = %q, want %q", got, want)
	}
	result = &ExecutionResult{
		BuildCheck: BuildVerification{Outcome: VerifyFailed, Output: "# pkg\npkg/a.go:3:2: undefined: x\npkg/b.go:9:1: missing return\n"},
		TestCheck:  TestVerification{Outcome: VerifySkipped},
	}
	got := failedChecksSummary(result)
	for _, want := range []string{"build fails", "pkg/a.go:3:2: undefined: x", "pkg/b.go:9:1: missing return"} {
		if !strings.Contains(got, want) {
			t.Fatalf("failedChecksSummary = %q, want it to contain %q", got, want)
		}
	}
	if strings.Contains(got, "tests") {
		t.Fatalf("a skipped test run is not a failure: %q", got)
	}
}

// The vet round keeps the suite green too. A vet repair that breaks the tests
// has repaired nothing: the next attempt is told the tests fail, and a round
// that gives up that way is undone -- the finding stays for the verdict to
// name and the suite is as green as the round found it.
func TestVetRound_ARepairThatBreaksTheTestsIsUndone(t *testing.T) {
	// Unreachable code: a vet finding outside the subset go test runs itself,
	// so the tests still pass and the vet round is what sees it.
	const code = "package main\n\nfunc Double(x int) int {\n\treturn x * 2\n}\n\n" +
		"func Describe(x int) string {\n\treturn \"x\"\n\treturn \"y\"\n}\n\nfunc main() {}\n"
	const tests = "package main\n\nimport \"testing\"\n\n" +
		"func TestDouble(t *testing.T) {\n\tif Double(2) != 4 {\n\t\tt.Fatal(\"Double(2) != 4\")\n\t}\n}\n\n" +
		"func TestDescribe(t *testing.T) {\n\tif Describe(1) == \"\" {\n\t\tt.Fatal(\"empty\")\n\t}\n}\n"
	// Vet-clean, and Double no longer doubles.
	const vetCleanButBroken = "package main\n\nfunc Double(x int) int {\n\treturn x * 3\n}\n\n" +
		"func Describe(x int) string {\n\treturn \"x\"\n}\n\nfunc main() {}\n"

	h := newRepairHarness(t, nil)
	mainPath := filepath.Join(h.ws, "main.go")
	initial := func() *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{
			h.writeCall("c1", "write_file", mainPath, code),
			h.writeCall("c2", "write_file", filepath.Join(h.ws, "main_test.go"), tests),
		}}
	}
	brokeTestsPrompt := ""
	h.executor.llmClient = &MockToolResultsLLM{
		MockLLMClient: &MockLLMClient{
			CompleteWithToolsFunc: func(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
				return initial(), nil
			},
		},
		CompleteWithToolResultsFunc: func(_ context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
			last := history[len(history)-1].Text
			if strings.Contains(last, "go vet is clean now, but the tests fail") && brokeTestsPrompt == "" {
				brokeTestsPrompt = last
			}
			if strings.Contains(last, "go vet reports these problems") || strings.Contains(last, "go vet is clean now") {
				return &types.LLMToolResponse{Text: "fixing the format verb", ToolCalls: []types.ToolCall{
					h.writeCall(fmt.Sprintf("v%d", len(history)), "write_file", mainPath, vetCleanButBroken),
				}}, nil
			}
			for _, m := range history {
				if len(m.ToolResults) > 0 {
					return &types.LLMToolResponse{Text: "done"}, nil
				}
			}
			return initial(), nil
		},
	}
	result, err := h.drive(t, "fix add Double and Describe")
	if err != nil {
		t.Fatalf("a vet round must not fail the change it was repairing: %v", err)
	}
	if brokeTestsPrompt == "" {
		t.Fatal("the attempt after a vet repair that broke the tests must be told so")
	}
	if result.TestCheck.Verdict() != VerifyPassed {
		t.Fatalf("the suite must be as green as the round found it: TestCheck = %+v", result.TestCheck)
	}
	if result.VetCheck.Verdict() != VerifyFailed {
		t.Fatalf("the finding the round did not fix stays for the verdict: VetCheck = %+v", result.VetCheck)
	}
	if data, readErr := os.ReadFile(mainPath); readErr != nil || string(data) != code {
		t.Fatalf("main.go must be as the round found it, got %q (%v)", data, readErr)
	}
}
