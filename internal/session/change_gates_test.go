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

	drive := func(t *testing.T, answer bool) (*ExecutionResult, string, error) {
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
					if answer {
						return &types.LLMToolResponse{Text: "testing Double", ToolCalls: []types.ToolCall{
							h.writeCall(fmt.Sprintf("t%d", len(history)), "write_file", filepath.Join(h.ws, "main_test.go"), doubleTest),
						}}, nil
					}
					return &types.LLMToolResponse{Text: "done"}, nil
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
		if data, readErr := os.ReadFile(filepath.Join(h.ws, "main_test.go")); readErr == nil && answer && !strings.Contains(string(data), "TestDouble") {
			t.Fatalf("the model's test did not land on disk:\n%s", data)
		}
		return result, coveragePrompt, err
	}

	t.Run("theModelWritesTheTest", func(t *testing.T) {
		result, prompt, err := drive(t, true)
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

	t.Run("theModelDoesNotAndTheDebtStays", func(t *testing.T) {
		result, prompt, err := drive(t, false)
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
