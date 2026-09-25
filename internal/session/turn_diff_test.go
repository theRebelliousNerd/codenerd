package session

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

// The round is shown the turn's own edits: R1-12 and R1-15 each spent three
// attempts asking why an element was "not extracted" while the rename that
// did it was one line of their own diff.
func TestTurnDiffSection_ShowsWhatTheTurnChanged(t *testing.T) {
	before := "package p\n\nfunc Name() string {\n\treturn \"widget\"\n}\n"
	after := "package p\n\nfunc Name() string {\n\treturn \"a.widget\"\n}\n"
	ws := writeBaselineModule(t, map[string]string{"go.mod": "module p\n\ngo 1.21\n", "a.go": after, "new.go": "package p\n\nfunc Added() {}\n"})

	section := turnDiffSection(ws, []string{"a.go", "new.go"}, map[string]PreImage{"a.go": existed(before), "new.go": {}}, testDiffBudget)
	if !strings.Contains(section, "What this turn changed") {
		t.Fatalf("no diff section:\n%s", section)
	}
	for _, want := range []string{"a.go", "-\treturn \"widget\"", "+\treturn \"a.widget\"", "new.go (created by this turn)", "+func Added() {}"} {
		if !strings.Contains(section, want) {
			t.Errorf("the diff does not carry %q:\n%s", want, section)
		}
	}
	// A file the turn left as it found it has no diff to show.
	if got := turnDiffSection(ws, []string{"a.go"}, map[string]PreImage{"a.go": existed(after)}, testDiffBudget); got != "" {
		t.Errorf("an unchanged file produced a diff:\n%s", got)
	}
	// A path with no recorded preimage is not guessed at.
	if got := turnDiffSection(ws, []string{"a.go"}, map[string]PreImage{"a.go": {Unknown: "denied"}}, testDiffBudget); got != "" {
		t.Errorf("an unknown preimage produced a diff:\n%s", got)
	}
}

// testDiffBudget is the repair diff budget an executor built from the session
// section's defaults uses.
var testDiffBudget = ExecutorConfig{}.repairDiffBudget()

// The budget is the session section's, reaching the executor through
// ExecutorConfigFrom: a knob config.json can set, not a Go constant.
func TestRepairDiffBudget_IsTheSessionSections(t *testing.T) {
	policy, err := config.SessionConfig{RepairDiffFileBytes: 2048, RepairDiffTurnBytes: 4096}.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	got := ExecutorConfigFrom(policy, config.DefaultWorkingConfig()).repairDiffBudget()
	if got.perFile != 2048 || got.perTurn != 4096 {
		t.Errorf("repairDiffBudget() = %+v, want the section's 2048/4096", got)
	}
	def := config.DefaultSessionConfig()
	if testDiffBudget.perFile != def.RepairDiffFileBytes || testDiffBudget.perTurn != def.RepairDiffTurnBytes {
		t.Errorf("a zero ExecutorConfig budget = %+v, want the section's defaults %d/%d",
			testDiffBudget, def.RepairDiffFileBytes, def.RepairDiffTurnBytes)
	}
}

// A binary edit is shown as one, not dropped: the old renderer returned "" for
// the zero hunks a binary diff has, and the round was told the turn never
// touched the file (GAP-DIFF-02).
func TestRenderFileDiff_BinaryMarker(t *testing.T) {
	before := "PNG\x00\x01old"
	after := "PNG\x00\x02new-and-longer"
	ws := writeBaselineModule(t, map[string]string{"go.mod": "module p\n\ngo 1.21\n", "logo.png": after})

	s, _ := renderFileDiff(fileChange{path: "logo.png", before: before, after: after})
	out := s.String()
	if !strings.Contains(out, "binary") || !strings.Contains(out, "logo.png") {
		t.Fatalf("a binary edit rendered without a marker: %q", out)
	}
	section := turnDiffSection(ws, []string{"logo.png"}, map[string]PreImage{"logo.png": existed(before)}, testDiffBudget)
	if !strings.Contains(section, "logo.png (binary:") {
		t.Fatalf("turnDiffSection dropped the binary edit:\n%s", section)
	}
}

// A delete is shown with its note and its removed lines. The old section
// skipped any path it could not read back from disk, which is every deleted
// file (GAP-DIFF-03).
func TestRenderFileDiff_DeleteNote(t *testing.T) {
	gone := "package p\n\nfunc Gone() {}\n"
	ws := writeBaselineModule(t, map[string]string{"go.mod": "module p\n\ngo 1.21\n", "keep.go": "package p\n"})

	section := turnDiffSection(ws, []string{"gone.go"}, map[string]PreImage{"gone.go": existed(gone)}, testDiffBudget)
	for _, want := range []string{"gone.go (deleted by this turn)", "-func Gone() {}"} {
		if !strings.Contains(section, want) {
			t.Errorf("the deleted file's diff does not carry %q:\n%s", want, section)
		}
	}

	// A file the turn emptied is not a deleted file, whatever diff.FileDiff's
	// IsDelete says about an empty new side.
	emptied := writeBaselineModule(t, map[string]string{"go.mod": "module p\n\ngo 1.21\n", "empty.go": ""})
	out := turnDiffSection(emptied, []string{"empty.go"}, map[string]PreImage{"empty.go": existed(gone)}, testDiffBudget)
	if strings.Contains(out, "deleted by this turn") || !strings.Contains(out, "-func Gone() {}") {
		t.Errorf("an emptied file was reported as deleted, or its removed lines were lost:\n%s", out)
	}
}

// The repair prompt's diff is bounded: one generated file rewritten whole used
// to put the entire file in the round's prompt (GAP-DIFF-04).
func TestTurnDiffSection_Budget(t *testing.T) {
	var before, after strings.Builder
	for i := range 4000 {
		fmt.Fprintf(&before, "line %d before\n", i)
		fmt.Fprintf(&after, "line %d after\n", i)
	}
	files := map[string]string{"go.mod": "module p\n\ngo 1.21\n"}
	pre := map[string]PreImage{}
	var written []string
	for i := range 6 {
		name := fmt.Sprintf("gen%d.go", i)
		files[name] = after.String()
		pre[name] = existed(before.String())
		written = append(written, name)
	}
	ws := writeBaselineModule(t, files)

	single := turnDiffSection(ws, written[:1], pre, testDiffBudget)
	if len(single) > testDiffBudget.perTurn {
		t.Fatalf("one file's section is %d bytes, over the %d turn budget", len(single), testDiffBudget.perTurn)
	}
	if body := strings.TrimPrefix(single, turnDiffHeader); len(body) > testDiffBudget.perFile {
		t.Errorf("one file's diff is %d bytes, over the %d file budget", len(body), testDiffBudget.perFile)
	}
	if !strings.Contains(single, diffTruncatedMarker) || !strings.Contains(single, "read_file") {
		t.Errorf("a cut file diff does not say it was cut, or where the rest is:\n%s", tail(single, 400))
	}
	if !strings.HasSuffix(single, "```\n") {
		t.Errorf("a cut diff left its fence open:\n%s", tail(single, 400))
	}

	all := turnDiffSection(ws, written, pre, testDiffBudget)
	if len(all) > testDiffBudget.perTurn {
		t.Fatalf("the turn's section is %d bytes, over its %d budget", len(all), testDiffBudget.perTurn)
	}
	// Every file the turn changed is mentioned: shown, cut, or named as left out.
	for _, name := range written {
		if !strings.Contains(all, name) {
			t.Errorf("%s is not mentioned in the bounded section", name)
		}
	}
	if !strings.Contains(all, "more changed file(s) not shown") {
		t.Errorf("files past the turn budget are not named:\n%s", tail(all, 600))
	}

	// The saved attempt patch is the whole attempt, not the prompt's view.
	patch := turnDiffPatch(ws, written, pre)
	if strings.Contains(patch, diffTruncatedMarker) || !strings.Contains(patch, "+line 3999 after") {
		t.Error("the saved attempt patch was truncated; it is the record a person restores from")
	}
}

func tail(s string, n int) string { return s[max(len(s)-n, 0):] }

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
