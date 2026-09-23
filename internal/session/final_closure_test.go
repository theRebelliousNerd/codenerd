package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// External audit F3 (2026-09-19): gate verdicts were not bound to the final
// revision. The closure refreshed the build and the tests only, so a round
// after the coverage, vet or deleted-test gate could leave a branch no test
// runs, a vet finding or a deleted test behind a verdict measured before it --
// and a round that paid a debt off left the debt on record. The critic ran
// after every one of those gates, so its edits were the only ones none of
// them measured.

const (
	closureGoMod  = "module closureprobe\n\ngo 1.25\n"
	closureDouble = "package main\n\nfunc Double(x int) int {\n\treturn x * 2\n}\n\nfunc main() {}\n"
	// closureDoubleBefore is Double as the workspace had it before a turn
	// rewrote it as closureDouble: the same function, written otherwise.
	closureDoubleBefore = "package main\n\nfunc Double(x int) int {\n\treturn x + x\n}\n\nfunc main() {}\n"
	closureHalf         = "package main\n\nfunc Double(x int) int {\n\treturn x * 2\n}\n\nfunc Half(x int) int {\n\treturn x / 2\n}\n\nfunc main() {}\n"

	closureTestDouble = "package main\n\nimport \"testing\"\n\nfunc TestDouble(t *testing.T) {\n\tif Double(2) != 4 {\n\t\tt.Fatal(\"Double(2) != 4\")\n\t}\n}\n"
	closureTestBoth   = closureTestDouble + "\nfunc TestDoubleZero(t *testing.T) {\n\tif Double(0) != 0 {\n\t\tt.Fatal(\"Double(0) != 0\")\n\t}\n}\n"
	closureTestHalf   = closureTestDouble + "\nfunc TestHalf(t *testing.T) {\n\tif Half(4) != 2 {\n\t\tt.Fatal(\"Half(4) != 2\")\n\t}\n}\n"
)

// closureWorkspace writes a module with the given files into the executor's
// workspace and returns the snapshot the gates measured.
func closureWorkspace(t *testing.T, e *Executor, files map[string]string) string {
	t.Helper()
	if testing.Short() {
		t.Skip("needs the go toolchain")
	}
	ws := e.config.WorkspaceRoot
	files["go.mod"] = closureGoMod
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	before, err := snapshotForTest(ws)
	if err != nil {
		t.Fatal(err)
	}
	return before
}

func laterWrite(t *testing.T, e *Executor, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(e.config.WorkspaceRoot, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCloseChangeEvidence_ALaterBranchNoTestRunsIsNamed(t *testing.T) {
	e, result := verifyGateExecutor(t)
	before := closureWorkspace(t, e, map[string]string{"main.go": closureDouble, "main_test.go": closureTestDouble})
	result.PreWriteContents = map[string]PreImage{"main.go": {}}

	laterWrite(t, e, "main.go", closureHalf)
	if err := e.closeChangeEvidence(context.Background(), result, before); err != nil {
		t.Fatalf("closeChangeEvidence: %v", err)
	}
	if result.TestCheck.Verdict() != VerifyPassed {
		t.Fatalf("TestCheck = %+v, want passed", result.TestCheck)
	}
	if got := uncoveredPaths(result); !slices.Equal(got, []string{"main.go"}) {
		t.Fatalf("uncovered paths = %v (blocks %+v), want [main.go]: Half was added after the coverage gate and no test runs it",
			got, result.UncoveredBlocks)
	}
}

func TestCloseChangeEvidence_ALaterRoundThatPaysTheDebtClearsIt(t *testing.T) {
	e, result := verifyGateExecutor(t)
	before := closureWorkspace(t, e, map[string]string{"main.go": closureHalf, "main_test.go": closureTestDouble})
	result.WrittenPaths = []string{"main.go", "main_test.go"}
	result.PreWriteContents = map[string]PreImage{"main.go": {}, "main_test.go": {}}
	// What the coverage gate measured before the later round.
	result.UncoveredBlocks = []UncoveredBlock{{File: "closureprobe/main.go", StartLine: 7, EndLine: 9, NumStmts: 1}}

	laterWrite(t, e, "main_test.go", closureTestHalf)
	if err := e.closeChangeEvidence(context.Background(), result, before); err != nil {
		t.Fatalf("closeChangeEvidence: %v", err)
	}
	if len(result.UncoveredBlocks) != 0 {
		t.Fatalf("UncoveredBlocks = %+v, want none: TestHalf runs Half now, and the debt was paid", result.UncoveredBlocks)
	}
}

func TestCloseChangeEvidence_ALaterVetFindingIsNamed(t *testing.T) {
	e, result := verifyGateExecutor(t)
	before := closureWorkspace(t, e, map[string]string{"main.go": closureDouble, "main_test.go": closureTestDouble})
	result.PreWriteContents = map[string]PreImage{"main.go": {}}
	// The vet gate found the turn's files clean.
	result.VetCheck = BuildVerification{Ran: true, OK: true, Outcome: VerifyPassed}

	// Unreachable code: go vet reports it, and the vet subset go test runs
	// does not, so the tests still pass and only the vet gate can see it.
	laterWrite(t, e, "main.go", "package main\n\nfunc Double(x int) int {\n\treturn x * 2\n\tprintln(\"never\")\n\treturn 0\n}\n\nfunc main() {}\n")
	if err := e.closeChangeEvidence(context.Background(), result, before); err != nil {
		t.Fatalf("closeChangeEvidence: %v", err)
	}
	if result.VetCheck.Verdict() != VerifyFailed || !strings.Contains(result.VetCheck.Output, "main.go") {
		t.Fatalf("VetCheck = %+v, want failed on main.go: the unreachable line was added after the vet gate", result.VetCheck)
	}
}

// The closure remeasures the rounds the schedule ran, not every gate the
// config switches allow: a turn whose schedule owed no /vet round keeps the
// vet check it has, though a later write would now fail vet. Until 2026-09-23
// the switches decided here -- a second answer to turn_round_owed.
func TestCloseChangeEvidence_OnlyTheRoundsThatRanAreRemeasured(t *testing.T) {
	e, result := verifyGateExecutor(t)
	before := closureWorkspace(t, e, map[string]string{"main.go": closureDouble, "main_test.go": closureTestDouble})
	result.PreWriteContents = map[string]PreImage{"main.go": {}}
	delete(result.roundsRan, "/vet")
	result.VetCheck = BuildVerification{Ran: true, OK: true, Outcome: VerifyPassed}

	laterWrite(t, e, "main.go", "package main\n\nfunc Double(x int) int {\n\treturn x * 2\n\tprintln(\"never\")\n\treturn 0\n}\n\nfunc main() {}\n")
	if err := e.closeChangeEvidence(context.Background(), result, before); err != nil {
		t.Fatalf("closeChangeEvidence: %v", err)
	}
	if result.VetCheck.Verdict() != VerifyPassed {
		t.Fatalf("VetCheck = %+v, want the round's own verdict: the schedule ran no /vet round", result.VetCheck)
	}
	if result.TestCheck.Verdict() != VerifyPassed {
		t.Fatalf("TestCheck = %+v, want passed: the /test round ran and is remeasured", result.TestCheck)
	}
}

func TestCloseChangeEvidence_ALaterDeletedTestFailsTheTurn(t *testing.T) {
	e, result := verifyGateExecutor(t)
	before := closureWorkspace(t, e, map[string]string{"main.go": closureDouble, "main_test.go": closureTestBoth})
	result.WrittenPaths = []string{"main.go", "main_test.go"}
	result.PreWriteContents = map[string]PreImage{"main.go": existed(closureDouble), "main_test.go": existed(closureTestBoth)}

	laterWrite(t, e, "main_test.go", closureTestDouble)
	err := e.closeChangeEvidence(context.Background(), result, before)
	if !errors.Is(err, ErrVerificationFailed) || !strings.Contains(err.Error(), "main_test.go:TestDoubleZero") {
		t.Fatalf("closeChangeEvidence = %v, want ErrVerificationFailed naming main_test.go:TestDoubleZero", err)
	}
}

// criticTurn drives a /fix turn whose model writes `initial`, whose critic
// reports one high-severity finding, and whose answer to every later prompt
// is `answer(prompt)`: files to write, or nil to write nothing.
func criticTurn(t *testing.T, seed, initial map[string]string, answer func(prompt string) map[string]string) (*repairHarness, *ExecutionResult, error) {
	t.Helper()
	if testing.Short() {
		t.Skip("needs the go toolchain")
	}
	h := newRepairHarness(t, nil)
	for name, content := range seed {
		if err := os.WriteFile(filepath.Join(h.ws, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	round := 0
	writes := func(files map[string]string) *types.LLMToolResponse {
		round++
		names := make([]string, 0, len(files))
		for name := range files {
			names = append(names, name)
		}
		slices.Sort(names)
		resp := &types.LLMToolResponse{Text: "writing"}
		for i, name := range names {
			resp.ToolCalls = append(resp.ToolCalls,
				h.writeCall(fmt.Sprintf("w%d-%d", round, i), "write_file", filepath.Join(h.ws, name), files[name]))
		}
		return resp
	}
	h.executor.llmClient = &MockToolResultsLLM{
		MockLLMClient: &MockLLMClient{
			CompleteWithSystemFunc: func(_ context.Context, sys, _ string) (string, error) {
				if sys == criticSystemPrompt {
					return "FINDING main.go:3 high: Double needs a companion the callers ask for", nil
				}
				return "", nil
			},
			CompleteWithToolsFunc: func(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
				return writes(initial), nil
			},
		},
		CompleteWithToolResultsFunc: func(_ context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
			worked := slices.ContainsFunc(history, func(m types.Message) bool { return len(m.ToolResults) > 0 })
			if !worked {
				return writes(initial), nil
			}
			if last := history[len(history)-1]; last.Text != "" && len(last.ToolResults) == 0 {
				if files := answer(last.Text); files != nil {
					return writes(files), nil
				}
			}
			return &types.LLMToolResponse{Text: "done"}, nil
		},
	}
	result, err := h.drive(t, "fix add the Double helper")
	return h, result, err
}

const upliftAsk = "An adversarial review of the code you just wrote"

// The critic's uplift adds code no test runs. It comes before the coverage
// round now, so that round hands the model the new lines like any others.
func TestCriticUplift_ItsNewCodeGetsTheCoverageRound(t *testing.T) {
	coveragePrompt := ""
	_, result, err := criticTurn(t, nil,
		map[string]string{"main.go": closureDouble, "main_test.go": closureTestDouble},
		func(prompt string) map[string]string {
			switch {
			case strings.Contains(prompt, upliftAsk):
				return map[string]string{"main.go": closureHalf}
			case strings.Contains(prompt, "no test executes these lines"):
				coveragePrompt = prompt
				return map[string]string{"main_test.go": closureTestHalf}
			}
			return nil
		})
	if err != nil {
		t.Fatalf("ProcessWithIntent: %v", err)
	}
	if !strings.Contains(coveragePrompt, "main.go:7-9") {
		t.Fatalf("the coverage round was not handed the uplift's Half (main.go:7-9); prompt:\n%s", coveragePrompt)
	}
	if len(result.UncoveredBlocks) != 0 || result.TestCheck.Verdict() != VerifyPassed {
		t.Fatalf("after the round Half must be run by a test: blocks %+v, TestCheck %+v", result.UncoveredBlocks, result.TestCheck)
	}
}

// The critic's opinion is advisory: an uplift that breaks the tests is
// undone, and the change that was green without it stays green.
func TestCriticUplift_ThatBreaksTheTestsIsUndone(t *testing.T) {
	h, result, err := criticTurn(t, nil,
		map[string]string{"main.go": closureDouble, "main_test.go": closureTestDouble},
		func(prompt string) map[string]string {
			if strings.Contains(prompt, upliftAsk) {
				return map[string]string{"main.go": strings.Replace(closureDouble, "x * 2", "x * 3", 1)}
			}
			return nil
		})
	if err != nil {
		t.Fatalf("an uplift that broke the tests must be undone, not fail the change: %v", err)
	}
	if data, readErr := os.ReadFile(filepath.Join(h.ws, "main.go")); readErr != nil || string(data) != closureDouble {
		t.Fatalf("main.go must be as the review found it: %q (%v)", data, readErr)
	}
	if result.TestCheck.Verdict() != VerifyPassed {
		t.Fatalf("TestCheck = %+v, want passed", result.TestCheck)
	}
	if len(result.CriticFindings) != 1 {
		t.Fatalf("the finding must stay on the result: %+v", result.CriticFindings)
	}
}

// The critic's uplift deletes a test the workspace had before the turn. The
// deleted-test round comes after the critic now, and hands the test back.
func TestCriticUplift_ADeletedTestIsHandedBack(t *testing.T) {
	restorePrompt := ""
	// main.go exists before the turn, as the package its tests test does;
	// the turn edits it. A turn that created it would owe it a test of its
	// own first (turn_missing_test), which is not what this test is about.
	h, _, err := criticTurn(t,
		map[string]string{"main_test.go": closureTestBoth, "main.go": closureDoubleBefore},
		map[string]string{"main.go": closureDouble},
		func(prompt string) map[string]string {
			switch {
			case strings.Contains(prompt, upliftAsk):
				return map[string]string{"main_test.go": closureTestDouble}
			case strings.Contains(prompt, "This turn deleted tests"):
				restorePrompt = prompt
				return map[string]string{"main_test.go": closureTestBoth}
			}
			return nil
		})
	if err != nil {
		t.Fatalf("ProcessWithIntent: %v", err)
	}
	if !strings.Contains(restorePrompt, "TestDoubleZero") {
		t.Fatalf("the deleted-test round was not handed TestDoubleZero; prompt:\n%s", restorePrompt)
	}
	if data, _ := os.ReadFile(filepath.Join(h.ws, "main_test.go")); !strings.Contains(string(data), "TestDoubleZero") {
		t.Fatalf("TestDoubleZero must be back on disk:\n%s", data)
	}
}
