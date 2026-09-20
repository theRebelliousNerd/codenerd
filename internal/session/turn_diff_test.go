package session

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// The round is shown the turn's own edits: R1-12 and R1-15 each spent three
// attempts asking why an element was "not extracted" while the rename that
// did it was one line of their own diff.
func TestTurnDiffSection_ShowsWhatTheTurnChanged(t *testing.T) {
	before := "package p\n\nfunc Name() string {\n\treturn \"widget\"\n}\n"
	after := "package p\n\nfunc Name() string {\n\treturn \"a.widget\"\n}\n"
	ws := writeBaselineModule(t, map[string]string{"go.mod": "module p\n\ngo 1.21\n", "a.go": after, "new.go": "package p\n\nfunc Added() {}\n"})

	section := turnDiffSection(ws, []string{"a.go", "new.go"}, map[string]PreImage{"a.go": existed(before), "new.go": {}})
	if !strings.Contains(section, "What this turn changed") {
		t.Fatalf("no diff section:\n%s", section)
	}
	for _, want := range []string{"a.go", "-\treturn \"widget\"", "+\treturn \"a.widget\"", "new.go (created by this turn)", "+func Added() {}"} {
		if !strings.Contains(section, want) {
			t.Errorf("the diff does not carry %q:\n%s", want, section)
		}
	}
	// A file the turn left as it found it has no diff to show.
	if got := turnDiffSection(ws, []string{"a.go"}, map[string]PreImage{"a.go": existed(after)}); got != "" {
		t.Errorf("an unchanged file produced a diff:\n%s", got)
	}
	// A path with no recorded preimage is not guessed at.
	if got := turnDiffSection(ws, []string{"a.go"}, map[string]PreImage{"a.go": {Unknown: "denied"}}); got != "" {
		t.Errorf("an unknown preimage produced a diff:\n%s", got)
	}
}

// End to end: the round that repairs broken tests carries the diff.
func TestVerifyAndRepairTests_ThePromptCarriesTheTurnsOwnDiff(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	const code = "package main\n\nfunc Double(x int) int {\n\treturn x * 3\n}\n\nfunc main() {}\n"
	const test = "package main\n\nimport \"testing\"\n\nfunc TestDouble(t *testing.T) {\n\tif Double(2) != 4 {\n\t\tt.Fatal(\"Double(2) != 4\")\n\t}\n}\n"
	const fixed = "package main\n\nfunc Double(x int) int {\n\treturn x * 2\n}\n\nfunc main() {}\n"

	h := newRepairHarness(t, nil)
	var prompts []string
	initial := func() *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{
			h.writeCall("c1", "write_file", filepath.Join(h.ws, "main.go"), code),
			h.writeCall("c2", "write_file", filepath.Join(h.ws, "main_test.go"), test),
		}}
	}
	h.executor.llmClient = &MockToolResultsLLM{
		MockLLMClient: &MockLLMClient{
			CompleteWithToolsFunc: func(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
				return initial(), nil
			},
		},
		CompleteWithToolResultsFunc: func(_ context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
			last := history[len(history)-1]
			if strings.Contains(last.Text, "the tests fail") || strings.Contains(last.Text, "Double(2) != 4") {
				prompts = append(prompts, last.Text)
				return &types.LLMToolResponse{Text: "fixing", ToolCalls: []types.ToolCall{
					h.writeCall("r1", "write_file", filepath.Join(h.ws, "main.go"), fixed),
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
	if _, err := h.drive(t, "fix make Double double"); err != nil {
		t.Fatalf("ProcessWithIntent: %v", err)
	}
	if len(prompts) == 0 {
		t.Fatal("no repair prompt was sent")
	}
	for _, want := range []string{"What this turn changed", "main.go (created by this turn)", "+func Double(x int) int {"} {
		if !strings.Contains(prompts[0], want) {
			t.Errorf("the repair prompt does not carry %q:\n%s", want, prompts[0])
		}
	}
}
