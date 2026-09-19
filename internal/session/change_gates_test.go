package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// go vet is package-wide; the turn owns only what it wrote. A finding in a
// file the turn did not touch is not this turn's evidence, and on Windows vet
// prints backslashed, workspace-relative paths under a "# pkg" header.
func TestVetFindingsInFiles_KeepsOnlyTheTurnsOwnFiles(t *testing.T) {
	output := strings.Join([]string{
		"# codenerd/cmd/nerd",
		`cmd\nerd\cmd_mangle_check.go:286:2: unreachable code`,
		`cmd\nerd\other.go:12:3: fmt.Sprintf format %d has arg s of wrong type string`,
		"# codenerd/internal/widget",
		"internal/widget/widget.go:9:1: result of fmt.Sprintf call not used",
		"vet: some diagnostic without a position",
	}, "\n")
	written := []string{"cmd/nerd/cmd_mangle_check.go", "internal/widget/widget.go"}

	got := vetFindingsInFiles(output, written)

	want := []string{
		`cmd\nerd\cmd_mangle_check.go:286:2: unreachable code`,
		"internal/widget/widget.go:9:1: result of fmt.Sprintf call not used",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("vetFindingsInFiles =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if none := vetFindingsInFiles(output, []string{"internal/elsewhere/x.go"}); len(none) != 0 {
		t.Fatalf("findings in files the turn did not write must not count, got %v", none)
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
