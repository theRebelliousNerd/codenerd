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

// A turn that rewrites a test file and drops tests that existed before it is
// handed each deleted test's source as it was and asked to put it back; the
// restored tests are run again. Driven through the production chain
// (ProcessWithIntent -> the tool loop -> verifyCompletedToolTurn) with real go
// test on a scratch module. Ladder run R1-4 (2026-09-19): a whole-file write
// of cmd_mangle_check_test.go dropped three tests that still passed against the
// new code, and the guard failed the turn on the spot -- fourteen minutes of
// work that had passed build, tests, coverage and vet, refused with no round.
func TestRemovedTestsRound_TheModelRestoresThemOrTheTurnFails(t *testing.T) {
	const code = "package main\n\nfunc Double(x int) int {\n\treturn x * 2\n}\n\nfunc main() {}\n"
	const before = `package main

import "testing"

func TestDouble(t *testing.T) {
	if Double(2) != 4 {
		t.Fatal("Double(2) != 4")
	}
}

// TestZero pins the identity at zero.
func TestZero(t *testing.T) {
	if Double(0) != 0 {
		t.Fatal("Double(0) != 0")
	}
}
`
	// The turn's rewrite: a new case, and TestZero silently gone.
	const rewritten = `package main

import "testing"

func TestDouble(t *testing.T) {
	if Double(2) != 4 {
		t.Fatal("Double(2) != 4")
	}
}

func TestNegative(t *testing.T) {
	if Double(-1) != -2 {
		t.Fatal("Double(-1) != -2")
	}
}
`
	const restored = rewritten + `
// TestZero pins the identity at zero.
func TestZero(t *testing.T) {
	if Double(0) != 0 {
		t.Fatal("Double(0) != 0")
	}
}
`
	const restoredBroken = rewritten + `
func TestZero(t *testing.T) {
	if Double(0) != 1 {
		t.Fatal("zero doubled is not one")
	}
}
`

	drive := func(t *testing.T, answer string) (*ExecutionResult, string, error) {
		t.Helper()
		h := newRepairHarness(t, nil)
		for name, content := range map[string]string{"main.go": code, "main_test.go": before} {
			if err := os.WriteFile(filepath.Join(h.ws, name), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		testPath := filepath.Join(h.ws, "main_test.go")
		initial := func() *types.LLMToolResponse {
			return &types.LLMToolResponse{Text: "adding a negative case", ToolCalls: []types.ToolCall{
				h.writeCall("c1", "write_file", testPath, rewritten),
			}}
		}
		restorePrompt := ""
		h.executor.llmClient = &MockToolResultsLLM{
			MockLLMClient: &MockLLMClient{
				CompleteWithToolsFunc: func(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
					return initial(), nil
				},
			},
			CompleteWithToolResultsFunc: func(_ context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
				if last := history[len(history)-1]; strings.Contains(last.Text, "deleted tests that existed before it") {
					if restorePrompt == "" {
						restorePrompt = last.Text
					}
					content := ""
					switch answer {
					case "restore":
						content = restored
					case "restoreBroken":
						content = restoredBroken
					default:
						return &types.LLMToolResponse{Text: "done"}, nil
					}
					return &types.LLMToolResponse{Text: "restoring TestZero", ToolCalls: []types.ToolCall{
						h.writeCall(fmt.Sprintf("r%d", len(history)), "write_file", testPath, content),
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
		result, err := h.drive(t, "fix add a negative case to the Double tests")
		return result, restorePrompt, err
	}

	t.Run("theModelRestoresThem", func(t *testing.T) {
		result, prompt, err := drive(t, "restore")
		if err != nil {
			t.Fatalf("a turn whose deleted test is restored must not fail: %v", err)
		}
		if !strings.Contains(prompt, "// TestZero pins the identity at zero.") || !strings.Contains(prompt, "Double(0) != 0") {
			t.Fatalf("the round must hand the model the deleted test's source as it was, doc comment included; prompt:\n%s", prompt)
		}
		if result.TestCheck.Verdict() != VerifyPassed {
			t.Fatalf("TestCheck = %+v, want passed", result.TestCheck)
		}
	})

	t.Run("theRestoredTestsAreRun", func(t *testing.T) {
		_, prompt, err := drive(t, "restoreBroken")
		if prompt == "" {
			t.Fatal("the model was never asked to restore the deleted test")
		}
		if err == nil || !errors.Is(err, ErrVerificationFailed) {
			t.Fatalf("a restored test that fails must fail the turn, got %v", err)
		}
		if !strings.Contains(err.Error(), "zero doubled is not one") {
			t.Fatalf("the failure must be the restored test's own, from running it: %v", err)
		}
		if strings.Contains(err.Error(), "without replacing them") {
			t.Fatalf("the test is back; the error must not say it was removed: %v", err)
		}
	})

	t.Run("theModelDeclinesAndTheTurnFails", func(t *testing.T) {
		_, prompt, err := drive(t, "decline")
		if prompt == "" {
			t.Fatal("the model was never asked to restore the deleted test")
		}
		if err == nil || !errors.Is(err, ErrVerificationFailed) || !strings.Contains(err.Error(), "TestZero") {
			t.Fatalf("a turn that still deletes a test must fail naming it, got %v", err)
		}
	})
}

// With no model to ask, a deletion fails the turn at once, naming the test --
// what the guard did before the round existed.
func TestVerifyAndRepairRemovedTests_NoModelToAskFailsNamingThem(t *testing.T) {
	h := newRepairHarness(t, nil)
	const kept = "package main\n\nimport \"testing\"\n\nfunc TestKept(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(h.ws, "x_test.go"), []byte(kept), 0o600); err != nil {
		t.Fatal(err)
	}
	result := &ExecutionResult{
		SuccessfulWriteTools: 1,
		WrittenPaths:         []string{"x_test.go"},
		PreWriteContents:     map[string]string{"x_test.go": kept + "\nfunc TestGone(t *testing.T) {}\n"},
	}
	_, _, err := h.executor.verifyAndRepairRemovedTests(context.Background(), nil, "", nil, nil, nil, result)
	if err == nil || !errors.Is(err, ErrVerificationFailed) || !strings.Contains(err.Error(), "x_test.go:TestGone") {
		t.Fatalf("with no model to ask, the deletion must fail the turn naming the test, got %v", err)
	}
}

// The listing names every removed test with its source. A pre-write file that
// no longer parses still names the test, with no source invented for it; a
// method sharing the name is not taken for the test; an entry with no path
// separator names nothing.
func TestRemovedTestListing(t *testing.T) {
	const pre = "package a\n\nimport \"testing\"\n\ntype s struct{}\n\nfunc (s) TestGone() {}\n\n// TestGone is the contract.\nfunc TestGone(t *testing.T) { t.Log(\"gone\") }\n"
	listing := removedTestListing(
		[]string{"a/x_test.go:TestGone", "no-separator", "b/y_test.go:TestBroken"},
		map[string]string{"a/x_test.go": pre, "b/y_test.go": "package b\nfunc TestBroken(t *testing.T) {"})
	for _, want := range []string{
		"a/x_test.go: TestGone\n```go\n// TestGone is the contract.\nfunc TestGone(t *testing.T) { t.Log(\"gone\") }\n```",
		"b/y_test.go: TestBroken",
	} {
		if !strings.Contains(listing, want) {
			t.Fatalf("listing is missing %q:\n%s", want, listing)
		}
	}
	for _, unwanted := range []string{"func (s) TestGone", "no-separator", "TestBroken\n```"} {
		if strings.Contains(listing, unwanted) {
			t.Fatalf("listing must not contain %q:\n%s", unwanted, listing)
		}
	}
}
