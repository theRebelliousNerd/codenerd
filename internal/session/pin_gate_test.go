package session

import (
	"context"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// Ladder run R1-10 in miniature. The fix is Greet calling a new helper,
// polite; the turn's test exercises polite. Every changed line runs, and the
// test still passes with Greet put back as it was -- nothing pins the call
// the fix was for.
const (
	pinGoMod       = "module pinprobe\n\ngo 1.21\n"
	pinCalcBefore  = "package pinprobe\n\nfunc Greet(name string) string { return \"hi \" + name }\n"
	pinCalcAfter   = "package pinprobe\n\n// Greet greets, politely.\nfunc Greet(name string) string { return polite(\"hi \" + name) }\n"
	pinHelper      = "package pinprobe\n\nimport \"strings\"\n\nfunc polite(s string) string {\n\tif strings.HasSuffix(s, \"!\") {\n\t\treturn s\n\t}\n\treturn s + \"!\"\n}\n"
	pinHelperTests = "package pinprobe\n\nimport \"testing\"\n\nfunc TestPolite(t *testing.T) {\n\tif polite(\"a\") != \"a!\" || polite(\"b!\") != \"b!\" {\n\t\tt.Fatal(\"polite\")\n\t}\n\t_ = Greet(\"x\")\n}\n"
	pinGreetTest   = "\nfunc TestGreet(t *testing.T) {\n\tif got := Greet(\"x\"); got != \"hi x!\" {\n\t\tt.Fatalf(\"Greet = %q\", got)\n\t}\n}\n"
)

// pinTurn is the turn above on disk: calc.go changed, helper.go and its test
// created.
func pinTurn(t *testing.T) (string, *ExecutionResult) {
	t.Helper()
	ws := writeBaselineModule(t, map[string]string{
		"go.mod":         pinGoMod,
		"calc.go":        pinCalcAfter,
		"helper.go":      pinHelper,
		"helper_test.go": pinHelperTests,
	})
	result := mutationResult()
	result.Intent.Verb = "/fix"
	result.WrittenPaths = []string{"calc.go", "helper.go", "helper_test.go"}
	result.PreWriteContents = map[string]PreImage{"calc.go": existed(pinCalcBefore), "helper.go": {}, "helper_test.go": {}}
	return ws, result
}

func TestVerifyPinning_ACallSiteOnlyAHelperTestReachesIsUnpinned(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	ws, result := pinTurn(t)

	v := verifyPinning(context.Background(), ws, result, true)
	if v.Verdict() != VerifyFailed {
		t.Fatalf("verdict = %s (%s), want failed: TestPolite passes with Greet put back as it was", v.Verdict(), v.Reason)
	}
	if !strings.Contains(v.Output, "calc.go: Greet") {
		t.Errorf("the gate does not name the unpinned call site:\n%s", v.Output)
	}
	if strings.Contains(v.Output, "polite") {
		t.Errorf("the helper is named, but TestPolite stops compiling without it -- it is pinned:\n%s", v.Output)
	}

	// The scenario test the brief asked for pins it.
	writeWorkspaceFile(t, ws, "helper_test.go", pinHelperTests+pinGreetTest)
	if v := verifyPinning(context.Background(), ws, result, true); v.Verdict() != VerifyPassed {
		t.Fatalf("verdict = %s (%s):\n%s\nwant passed: TestGreet fails with Greet put back", v.Verdict(), v.Reason, v.Output)
	}
}

// A turn that changed a function and wrote no test pins nothing: every change
// is named, without running anything.
func TestVerifyPinning_AChangeWithNoTestOfTheTurnsIsUnpinned(t *testing.T) {
	ws, result := pinTurn(t)
	result.WrittenPaths = []string{"calc.go", "helper.go"}

	v := verifyPinning(context.Background(), ws, result, true)
	if v.Verdict() != VerifyFailed || !strings.Contains(v.Output, "wrote no test") {
		t.Fatalf("verdict = %s:\n%s\nwant failed, naming that the turn wrote no test", v.Verdict(), v.Output)
	}
	for _, want := range []string{"calc.go: Greet", "helper.go: polite (added by this turn)"} {
		if !strings.Contains(v.Output, want) {
			t.Errorf("the listing does not name %q:\n%s", want, v.Output)
		}
	}
}

// A Go file the go tool never builds -- a fixture under testdata -- is not a
// change a test could pin.
func TestPinUnits_AFixtureTheGoToolNeverBuildsIsNoChange(t *testing.T) {
	ws := writeBaselineModule(t, map[string]string{"go.mod": pinGoMod, "parse/testdata/input.go": pinCalcAfter})
	pre := map[string]PreImage{"parse/testdata/input.go": existed(pinCalcBefore)}
	if units := pinUnits(ws, []string{"parse/testdata/input.go"}, pre); len(units) != 0 {
		t.Fatalf("pinUnits = %+v, want none", units)
	}
}

// A test the turn left as it found it is not the turn's: re-commenting or
// re-laying it out does not make it one.
func TestTurnTests_AreTheTestsTheTurnAddedOrChanged(t *testing.T) {
	before := "package pinprobe\n\nimport \"testing\"\n\nfunc TestKept(t *testing.T) { t.Log(\"k\") }\n\nfunc TestEdited(t *testing.T) { t.Log(1) }\n"
	after := "package pinprobe\n\nimport \"testing\"\n\n// TestKept is kept.\nfunc TestKept(t *testing.T) {\n\tt.Log(\"k\")\n}\n\nfunc TestEdited(t *testing.T) { t.Log(2) }\n\nfunc TestNew(t *testing.T) {}\n\nfunc BenchmarkNew(b *testing.B) {}\n"
	ws := writeBaselineModule(t, map[string]string{"go.mod": pinGoMod, "calc_test.go": after})

	names, pkgs, gated := turnTests(ws, []string{"calc_test.go"}, map[string]PreImage{"calc_test.go": existed(before)}, nil)
	if !slices.Equal(names, []string{"TestEdited", "TestNew"}) || !slices.Equal(pkgs, []string{"."}) || gated != 0 {
		t.Fatalf("turnTests = %v in %v (%d gated), want [TestEdited TestNew] in [.]", names, pkgs, gated)
	}
	// A test that already failed before the turn pins nothing.
	if names, _, _ := turnTests(ws, []string{"calc_test.go"}, map[string]PreImage{"calc_test.go": existed(before)}, []string{"TestEdited"}); !slices.Equal(names, []string{"TestNew"}) {
		t.Errorf("turnTests with TestEdited failing before the turn = %v, want [TestNew]", names)
	}
}

// Comments and layout are not changes: there is nothing to take out.
func TestPinUnits_ACommentOrLayoutChangeIsNoChange(t *testing.T) {
	before := "package p\n\nconst limit = 3\n\nfunc F(a int) int {\n\treturn a + limit\n}\n"
	after := "package p\n\n// limit bounds F.\nconst limit = 3\n\n// F adds the limit.\nfunc F(a int) int { return a + limit } // same code\n"
	if units := fileUnits("p.go", after, existed(before)); len(units) != 0 {
		t.Fatalf("fileUnits = %+v, want none", units)
	}
	// A constant changed and no function: the file is the one change.
	changed := strings.Replace(before, "limit = 3", "limit = 4", 1)
	units := fileUnits("p.go", changed, existed(before))
	if len(units) != 1 || units[0].name != "" || units[0].content != before {
		t.Fatalf("fileUnits = %+v, want the whole file put back", units)
	}
}

// Putting a function back can leave an import unused, or need one the turn
// dropped; the unit is the file as it compiles.
func TestFileUnits_KeepsTheImportsThePutBackCodeNeeds(t *testing.T) {
	before := "package p\n\nimport \"fmt\"\n\nfunc F() string { return fmt.Sprint(1) }\n"
	after := "package p\n\nimport \"strconv\"\n\nfunc F() string { return strconv.Itoa(1) }\n"
	units := fileUnits("p.go", after, existed(before))
	if len(units) != 1 || units[0].name != "F" {
		t.Fatalf("fileUnits = %+v, want F", units)
	}
	f, err := parser.ParseFile(token.NewFileSet(), "", units[0].content, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("the unit does not parse: %v\n%s", err, units[0].content)
	}
	var imports []string
	for _, spec := range f.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		imports = append(imports, path)
	}
	if !slices.Equal(imports, []string{"fmt"}) {
		t.Fatalf("imports = %v, want [fmt]:\n%s", imports, units[0].content)
	}
}

// The verdict: a behaviour change owes /pinned, and without it green it is
// not done.
func TestTurnWhoseChangeIsPinnedByNoTestIsNotDone(t *testing.T) {
	owes := func(verb string) bool {
		e := newObligationExec(t)
		result := writeTurnResult()
		result.Intent.Verb = verb
		return e.turnOwesGate(result, "/pinned")
	}
	for _, verb := range []string{"/fix", "/create", "/implement"} {
		if !owes(verb) {
			t.Errorf("a %s turn that wrote Go does not owe /pinned", verb)
		}
	}
	for _, verb := range []string{"/refactor", "/optimize", "/document"} {
		if owes(verb) {
			t.Errorf("a %s turn owes /pinned; it keeps behaviour, and no test can fail without it", verb)
		}
	}

	outcome := func(pin BuildVerification) *ExecutionResult {
		e := newObligationExec(t)
		result := writeTurnResult()
		result.PinCheck = pin
		e.assertTurnEvidence(testTurn, "/fix", result)
		e.captureTurnOutcome(testTurn, result, nil)
		return result
	}
	for name, pin := range map[string]BuildVerification{
		"unpinned":   {Ran: true, Outcome: VerifyFailed, Output: "calc.go: Greet"},
		"unmeasured": {Outcome: VerifyIndeterminate},
	} {
		if r := outcome(pin); r.TurnOutcome == types.MangleAtom("/done") || !slices.Contains(r.MissingEvidence, "/change_not_pinned") {
			t.Errorf("%s: TurnOutcome = %v, MissingEvidence = %v; want /unverified naming /change_not_pinned", name, r.TurnOutcome, r.MissingEvidence)
		}
	}
	if r := outcome(BuildVerification{Ran: true, OK: true, Outcome: VerifyPassed}); r.TurnOutcome != types.MangleAtom("/done") {
		t.Errorf("pinned: TurnOutcome = %v (missing %v), want /done", r.TurnOutcome, r.MissingEvidence)
	}

	permissive := core.MangleUpdatePolicy{AllowedPrefixes: []string{""}}
	for _, update := range []string{"turn_verb(/t, /refactor).", "behavior_change_intent(/document)."} {
		if kept, _ := core.FilterMangleUpdates(nil, []string{update}, permissive); len(kept) != 0 {
			t.Errorf("the model can assert %s; what a turn owes is the harness's to say", update)
		}
	}
}

// The round: a turn whose tests do not pin its change is sent back to write
// one that does, and the turn that writes it closes /done.
func TestVerifyCompletedToolTurn_AnUnpinnedChangeIsSentBackForATestThatPinsIt(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	h := newRepairHarness(t, func(e *Executor) {
		k, err := core.NewRealKernel()
		if err != nil {
			t.Fatalf("the shipped corpus must load: %v", err)
		}
		e.kernel = k
	})
	writeWorkspaceFile(t, h.ws, "go.mod", pinGoMod)
	writeWorkspaceFile(t, h.ws, "calc.go", pinCalcBefore)
	var prompts []string
	initial := func() *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{
			h.writeCall("c1", "write_file", filepath.Join(h.ws, "calc.go"), pinCalcAfter),
			h.writeCall("c2", "write_file", filepath.Join(h.ws, "helper.go"), pinHelper),
			h.writeCall("c3", "write_file", filepath.Join(h.ws, "helper_test.go"), pinHelperTests),
		}}
	}
	h.executor.llmClient = &MockToolResultsLLM{
		MockLLMClient: &MockLLMClient{
			CompleteWithToolsFunc: func(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
				return initial(), nil
			},
		},
		CompleteWithToolResultsFunc: func(_ context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
			if last := history[len(history)-1]; strings.Contains(last.Text, "every test this turn wrote still passes") {
				prompts = append(prompts, last.Text)
				return &types.LLMToolResponse{Text: "pinning Greet", ToolCalls: []types.ToolCall{
					h.writeCall("p1", "write_file", filepath.Join(h.ws, "helper_test.go"), pinHelperTests+pinGreetTest),
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
	result, err := h.drive(t, "fix greet politely")
	if err != nil {
		t.Fatalf("ProcessWithIntent: %v", err)
	}
	if len(prompts) == 0 {
		t.Fatalf("the turn was not sent back; PinCheck = %+v", result.PinCheck)
	}
	if !strings.Contains(prompts[0], "calc.go: Greet") {
		t.Errorf("the round's prompt does not name the unpinned change:\n%s", prompts[0])
	}
	if result.PinCheck.Verdict() != VerifyPassed {
		t.Fatalf("PinCheck = %+v, want passed after the round", result.PinCheck)
	}
	if result.TurnOutcome != types.MangleAtom("/done") {
		t.Fatalf("TurnOutcome = %v (missing %v), want /done", result.TurnOutcome, result.MissingEvidence)
	}
	if data, _ := os.ReadFile(filepath.Join(h.ws, "helper_test.go")); !strings.Contains(string(data), "TestGreet") {
		t.Fatalf("the pinning test did not land on disk:\n%s", data)
	}
}
