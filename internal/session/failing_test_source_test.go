package session

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/types"
)

const failingProbeTest = "package probe\n\nimport \"testing\"\n\n" +
	"// TestExtents is the contract.\n" +
	"func TestExtents(t *testing.T) {\n\tif Extract(\"nerdmd.go\") != \"ForbidsPath\" {\n\t\tt.Fatal(\"ForbidsPath was not extracted from nerdmd.go\")\n\t}\n}\n\n" +
	"func TestOther(t *testing.T) {}\n"

// The round's prompt carries the failing test as it is on disk, found from
// the package the runner named, so a round with the read tools closed does
// not have to open it.
func TestFailingTestSection_RendersTheFailingTestFromThePackageTheRunnerNamed(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	ws := writeBaselineModule(t, map[string]string{
		"go.mod":                       "module probe\n\ngo 1.21\n",
		"internal/probe/extract.go":    "package probe\n\nfunc Extract(string) string { return \"\" }\n",
		"internal/probe/probe_test.go": failingProbeTest,
	})
	res := parseJSONTest(t, ws, runJSONTest(t, ws, "./internal/probe/"))

	section := failingTestSection(ws, res, nil)
	if !strings.Contains(section, "internal/probe/probe_test.go: TestExtents") {
		t.Fatalf("the section does not name the test's file:\n%s", section)
	}
	for _, want := range []string{"// TestExtents is the contract.", "func TestExtents(t *testing.T) {", "ForbidsPath was not extracted"} {
		if !strings.Contains(section, want) {
			t.Errorf("the section does not carry %q:\n%s", want, section)
		}
	}
	if strings.Contains(section, "func TestOther") {
		t.Errorf("the section carries a test that did not fail:\n%s", section)
	}
	pass := parseJSONTest(t, ws, jsonStream(t,
		jsonEvent{Action: "pass", Package: "probe/internal/probe", Test: "TestOther"},
		jsonEvent{Action: "pass", Package: "probe/internal/probe"},
	))
	if got := failingTestSection(ws, pass, nil); got != "" {
		t.Errorf("a passing run produced a section:\n%s", got)
	}
	if got := failingTestSection(ws, nil, nil); got != "" {
		t.Errorf("a run with no Result produced a section:\n%s", got)
	}
}

// A test in a directory the turn wrote is found even when the Summary's
// package names no directory on disk.
func TestFailingTestSection_FindsTheTestBesideTheTurnsOwnWrite(t *testing.T) {
	ws := writeBaselineModule(t, map[string]string{
		"go.mod":                       "module probe\n\ngo 1.21\n",
		"internal/probe/probe_test.go": failingProbeTest,
	})
	// The package the runner named is not a directory in this module. The
	// events are the fail protocol; a real `go test` of this tree would
	// name probe/internal/probe, which would find the file without the
	// written path.
	res := parseJSONTest(t, ws, jsonStream(t,
		jsonEvent{Action: "output", Package: "probe/elsewhere", Test: "TestExtents", Output: "    x_test.go:1: boom\n"},
		jsonEvent{Action: "fail", Package: "probe/elsewhere", Test: "TestExtents"},
		jsonEvent{Action: "fail", Package: "probe/elsewhere"},
	))
	section := failingTestSection(ws, res, []string{"internal/probe/extract.go"})
	if !strings.Contains(section, "func TestExtents") {
		t.Fatalf("the section does not carry the test beside the written file:\n%s", section)
	}
}

// With the tests in hand the prompt stops telling the model to read them: the
// round has closed the read tools, and R1-12 spent three attempts trying to
// obey that sentence with recall_context.
func TestTestRepairPrompt_StopsAskingForAReadItCannotDo(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	dir := jsonTestModule(t, map[string]string{
		"calc.go": "package verifyprobe\n\nfunc Add(a, b int) int { return a + b }\n",
		"calc_test.go": "package verifyprobe\n\nimport \"testing\"\n\n" +
			"func TestAdd(t *testing.T) { if Add(2, 3) != 999 { t.Fatal(\"intentional failure\") } }\n",
	})
	out := verificationOutput(parseJSONTest(t, dir, runJSONTest(t, dir, ".")))
	withSource := testRepairPrompt(out, "\nThe failing tests, as they are on disk:\nx_test.go: TestAdd\n")
	if strings.Contains(withSource, "Read the failing test and the code under test") {
		t.Errorf("the prompt still asks for a read it has already answered:\n%s", withSource)
	}
	for _, want := range []string{"read them there, not with a tool", "x_test.go: TestAdd"} {
		if !strings.Contains(withSource, want) {
			t.Errorf("the prompt does not contain %q:\n%s", want, withSource)
		}
	}
	if without := testRepairPrompt(out, ""); !strings.Contains(without, "Read the failing test and the code under test") {
		t.Errorf("without the source the prompt must still send the model to read it:\n%s", without)
	}
}

// End to end: a turn whose edit breaks a test is handed the test.
func TestVerifyAndRepairTests_ThePromptCarriesTheFailingTest(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	const code = "package main\n\nfunc Double(x int) int {\n\treturn x * 3\n}\n\nfunc main() {}\n"
	const test = "package main\n\nimport \"testing\"\n\n// TestDouble pins doubling.\nfunc TestDouble(t *testing.T) {\n\tif Double(2) != 4 {\n\t\tt.Fatal(\"Double(2) != 4\")\n\t}\n}\n"
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
	result, err := h.drive(t, "fix make Double double")
	if err != nil {
		t.Fatalf("ProcessWithIntent: %v", err)
	}
	if result.TestCheck.Verdict() != VerifyPassed {
		t.Fatalf("TestCheck = %+v, want passed after the repair", result.TestCheck)
	}
	if len(prompts) == 0 {
		t.Fatal("no repair prompt was sent")
	}
	for _, want := range []string{"The failing tests, as they are on disk", "// TestDouble pins doubling.", "func TestDouble(t *testing.T) {"} {
		if !strings.Contains(prompts[0], want) {
			t.Errorf("the repair prompt does not carry %q:\n%s", want, prompts[0])
		}
	}
}
