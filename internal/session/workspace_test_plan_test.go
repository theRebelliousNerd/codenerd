package session

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/gates"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// exitWith builds a program that exits with the code in its first argument.
func exitWith(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain to build the gate")
	}
	dir := t.TempDir()
	src := "package main\n\nimport (\n\t\"os\"\n\t\"strconv\"\n)\n\nfunc main() {\n\tn, _ := strconv.Atoi(os.Args[1])\n\tos.Exit(n)\n}\n"
	for name, body := range map[string]string{"go.mod": "module exitwith\n\ngo 1.21\n", "main.go": src} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bin := filepath.Join(dir, "exitwith")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build gate: %v\n%s", err, out)
	}
	return bin
}

// planTurn is an executor over the shipped corpus whose workspace declares
// gateLines, and a write turn that wrote svc/app.py and is sequenced.
func planTurn(t *testing.T, gateLines string) (*Executor, *ExecutionResult, string) {
	t.Helper()
	ws := workspaceWithGate(t, gateLines)
	e := newObligationExec(t)
	e.config.WorkspaceRoot = ws
	result := mutationResult()
	result.WrittenPaths = []string{"svc/app.py"}
	e.ensureWriteSequenced(result.turnAtom(), result)
	return e, result, ws
}

func nodeGate(id, program string) string {
	return "  - id: " + id + "\n    kind: test\n    run: >-\n      '" + filepath.ToSlash(program) + "' {node}\n    scope: node\n"
}

func testRunGate(t *testing.T, e *Executor, result *ExecutionResult) VerifyOutcome {
	t.Helper()
	return derivedVerify(t, e, result.turnAtom(), "/test_run")
}

func outcomeOf(result *ExecutionResult, id string) obligationOutcome {
	for _, o := range result.testPlan {
		if o.ID == id || o.ID == "nerd.md:"+id {
			return o.Outcome
		}
	}
	return ""
}

// The settled verdict, as the closing sentence and the evidence read it.
func settle(e *Executor, result *ExecutionResult) {
	e.assertTurnEvidence(result.turnAtom(), "/fix", result)
	e.captureTurnOutcome(result.turnAtom(), result, nil)
}

// W1: a test process the model chose is not the workspace's check. An
// unrelated passing process started before the forcing round must not stand in
// for a declared check that is red.
func TestVerifyAndRepairTestRun_AnUnrelatedPassingRunDoesNotReplaceTheWorkspacesCheck(t *testing.T) {
	bin := checkGate(t) // value.txt says "bad": the declared check fails
	client := &testRunScript{MockLLMClient: &MockLLMClient{}}
	e, ctx, cfg := testRunRoundExec(t, client, 0)
	ws := workspaceWithGate(t, nodeGate("check", bin))
	e.config.WorkspaceRoot = ws
	result := mutationResult()
	result.WrittenPaths = []string{"svc/app.py"}
	turn := result.turnAtom()
	e.ensureWriteSequenced(turn, result)
	unrelated := tools.TestRun{Argv: []string{"go", "test", "-run", "TestSomethingElse", "./..."}, ExitCode: 0}
	e.noteTestRun(result, unrelated)
	result.TestRunSinceLastWrite = &unrelated

	if _, _, err := e.verifyCompletedToolTurn(ctx, client, "system", wroteHistory("svc/app.py"), &types.LLMToolResponse{Text: "Edited."}, e.buildToolDefinitions(cfg), cfg, result); err != nil {
		t.Fatalf("verifyCompletedToolTurn: %v", err)
	}
	if got := derivedVerify(t, e, turn, "/test_run"); got != VerifyFailed {
		t.Fatalf("/test_run = %v: the declared check is red and an unrelated passing process was started (plan %+v)", got, result.testPlan)
	}
	if outcomeOf(result, "check@svc") != outcomeFailed {
		t.Fatalf("the declared check was not run: %+v", result.testPlan)
	}
	settle(e, result)
	if result.TurnOutcome == types.MangleAtom("/done") {
		t.Fatal("a red declared check was reported done")
	}
}

// An identical, adequately scoped process the turn already ran is reused: the
// declared check is not run a second time, and the receipt satisfies it.
func TestWorkspaceTestRun_AnIdenticalRunIsReusedNotRepeated(t *testing.T) {
	t.Parallel()
	bin := checkGate(t)
	e, result, ws := planTurn(t, nodeGate("check", bin))
	// value.txt is "bad": running the check would fail. The receipt says the
	// very same command already exited 0 after the write.
	e.noteTestRun(result, plannedIdentity(t, ws, bin))
	if ran, _ := e.workspaceTestRun(context.Background(), result); !ran {
		t.Fatal("the workspace declares a check for the written file")
	}
	if got := testRunGate(t, e, result); got != VerifyPassed {
		t.Fatalf("/test_run = %v, want passed by the identical receipt (plan %+v)", got, result.testPlan)
	}
	if len(result.testPlan) != 1 || !result.testPlan[0].Reused {
		t.Fatalf("the check was run again instead of reusing the receipt: %+v", result.testPlan)
	}
}

// W2: one declared check passes and another cannot run. The passing subset is
// kept, the missing obligation is named, and no later unrelated process
// clears it.
func TestWorkspaceTestRun_AnUnavailableDeclaredCheckIsStillOwed(t *testing.T) {
	t.Parallel()
	bin := checkGate(t)
	if err := os.WriteFile(filepath.Join(t.TempDir(), "x"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	e, result, ws := planTurn(t, nodeGate("a-check", bin)+"  - id: b-missing\n    kind: test\n    run: definitely-not-a-test-runner {node}\n    scope: node\n")
	if err := os.WriteFile(filepath.Join(ws, "svc", "value.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ran, _ := e.workspaceTestRun(context.Background(), result); !ran {
		t.Fatal("the workspace declares checks for the written file")
	}
	if outcomeOf(result, "a-check@svc") != outcomePassed || outcomeOf(result, "b-missing@svc") != outcomeUnavailable {
		t.Fatalf("plan = %+v, want the first passed and the second unavailable", result.testPlan)
	}
	if got := testRunGate(t, e, result); got == VerifyPassed {
		t.Fatal("/test_run passed with a declared check unable to run")
	}
	// A later unrelated passing process cannot stand in for it.
	e.noteTestRun(result, tools.TestRun{Argv: []string{"go", "test", "./..."}, ExitCode: 0})
	if got := testRunGate(t, e, result); got == VerifyPassed {
		t.Fatal("an unrelated passing process cleared a check that could not run")
	}
	settle(e, result)
	if result.TurnOutcome == types.MangleAtom("/done") || !slices.Contains(result.MissingEvidence, "/test_run_unavailable") {
		t.Fatalf("outcome %v missing %v, want unverified naming the unavailable check", result.TurnOutcome, result.MissingEvidence)
	}
}

// W2 second form: an earlier check fails and a later one cannot start. Both
// outcomes reach the kernel; neither the unavailable check nor a later
// passing process erases the failure.
func TestWorkspaceTestRun_AFailureSurvivesALaterCheckThatCannotStart(t *testing.T) {
	t.Parallel()
	bin := checkGate(t)
	e, result, _ := planTurn(t, nodeGate("a-check", bin)+"  - id: b-gone\n    kind: test\n    run: ./scripts/gone.sh {node}\n    scope: node\n")
	if ran, _ := e.workspaceTestRun(context.Background(), result); !ran {
		t.Fatal("the workspace declares checks for the written file")
	}
	if outcomeOf(result, "a-check@svc") != outcomeFailed || outcomeOf(result, "b-gone@svc") != outcomeUnavailable {
		t.Fatalf("plan = %+v, want a failure and an unavailable check, both kept", result.testPlan)
	}
	e.noteTestRun(result, tools.TestRun{Argv: []string{"go", "test", "./..."}, ExitCode: 0})
	if got := testRunGate(t, e, result); got != VerifyFailed {
		t.Fatalf("/test_run = %v, want failed: the early failure is not erased", got)
	}
}

// W2 recheck variant: the workspace's check failed, a repair produced a
// passing model-selected process, and the recheck is stopped before a verdict.
// The newer receipt must not turn the gate green.
func TestWorkspaceTestRun_AStoppedRecheckDoesNotLeaveTheToolReceiptCurrent(t *testing.T) {
	t.Parallel()
	bin := checkGate(t)
	e, result, _ := planTurn(t, nodeGate("check", bin))
	e.workspaceTestRun(context.Background(), result)
	if got := testRunGate(t, e, result); got != VerifyFailed {
		t.Fatalf("setup: /test_run = %v, want failed", got)
	}
	repair := tools.TestRun{Argv: []string{"go", "test", "./..."}, ExitCode: 0}
	e.noteTestRun(result, repair)
	stopped, cancel := context.WithCancel(context.Background())
	cancel()
	e.workspaceTestRun(stopped, result)
	if outcomeOf(result, "check@svc") != outcomeIndeterminate {
		t.Fatalf("plan = %+v, want the stopped check indeterminate", result.testPlan)
	}
	if got := testRunGate(t, e, result); got == VerifyPassed {
		t.Fatal("the repair's unrelated receipt made the stopped workspace check pass")
	}
}

// W3: pytest exit 5 is not a failure and is not coverage. The check passes its
// own exit policy but the obligation stays open until a test process that
// executed tests passes -- the project's tests may live outside the directory
// written -- and a project that declared its own ok exits keeps its meaning.
func TestTestPlan_NoTestsCollectedIsNotCoverage(t *testing.T) {
	t.Parallel()
	bin := exitWith(t)
	pytest := gates.Gate{ID: "python:pytest", Kind: gates.Test, Argv: []string{bin, "5"}, Scope: gates.ScopeAll,
		Language: "python", OKExitCodes: []int{0, 5}, NoTestsExitCodes: []int{5}}
	declared := gates.Gate{ID: "declared", Kind: gates.Test, Argv: []string{bin, "5"}, Scope: gates.ScopeAll, OKExitCodes: []int{0, 5}}

	run := func(g gates.Gate) (*Executor, *ExecutionResult) {
		e, result, ws := planTurn(t, nodeGate("unused", bin))
		plan := buildTestPlan(gates.Set{Gates: []gates.Gate{g}}, map[string]bool{"python": true}, []string{"svc"})
		e.runTestPlan(context.Background(), ws, result, plan)
		return e, result
	}

	e, result := run(pytest)
	if outcomeOf(result, "python:pytest@.") != outcomeNoTests {
		t.Fatalf("plan = %+v, want no_tests", result.testPlan)
	}
	if got := testRunGate(t, e, result); got == VerifyPassed {
		t.Fatal("a check that collected no test passed the gate")
	}
	settle(e, result)
	if !slices.Contains(result.MissingEvidence, "/test_run_no_tests") {
		t.Fatalf("missing %v, want the empty collection named", result.MissingEvidence)
	}
	// The project's suite lives elsewhere: a process that ran tests and passed
	// is what meets the obligation.
	e.noteTestRun(result, tools.TestRun{Argv: []string{"python3", "-m", "pytest", "tests"}, ExitCode: 0})
	if got := testRunGate(t, e, result); got != VerifyPassed {
		t.Fatalf("/test_run = %v, want passed by a process that ran the project's tests", got)
	}

	// An exit the project declared as a pass is a pass, not an empty collection.
	e, result = run(declared)
	if outcomeOf(result, "declared@.") != outcomePassed || testRunGate(t, e, result) != VerifyPassed {
		t.Fatalf("a declared ok exit lost its meaning: %+v", result.testPlan)
	}
}

// Multi-language: a pass for one written language does not cover another that
// has no declared check; a language-free check is declared shared coverage.
func TestTestPlan_AnUncoveredLanguageStaysVisible(t *testing.T) {
	t.Parallel()
	bin := exitWith(t)
	py := gates.Gate{ID: "python:pytest", Kind: gates.Test, Argv: []string{bin, "0"}, Scope: gates.ScopeAll, Language: "python"}
	shared := gates.Gate{ID: "all", Kind: gates.Test, Argv: []string{bin, "0"}, Scope: gates.ScopeAll}
	langs := map[string]bool{"python": true, "js/ts": true}

	plan := buildTestPlan(gates.Set{Gates: []gates.Gate{py}}, langs, []string{"svc"})
	if ids := planIDs(plan); !slices.Equal(ids, []string{"python:pytest@.", "uncovered:js/ts"}) {
		t.Fatalf("plan = %v, want the python check and the uncovered language", ids)
	}
	e, result, ws := planTurn(t, nodeGate("unused", bin))
	e.runTestPlan(context.Background(), ws, result, plan)
	if outcomeOf(result, "python:pytest@.") != outcomePassed || outcomeOf(result, "uncovered:js/ts") != outcomeUncovered {
		t.Fatalf("plan = %+v", result.testPlan)
	}
	if got := testRunGate(t, e, result); got == VerifyPassed {
		t.Fatal("python's pass covered the TypeScript this turn wrote")
	}
	e.noteTestRun(result, tools.TestRun{Argv: []string{"npm", "test"}, ExitCode: 0})
	if got := testRunGate(t, e, result); got != VerifyPassed {
		t.Fatalf("/test_run = %v, want passed once a process covered the other language", got)
	}

	// Control: a language-free declared check is shared coverage.
	plan = buildTestPlan(gates.Set{Gates: []gates.Gate{py, shared}}, langs, []string{"svc"})
	if ids := planIDs(plan); slices.Contains(ids, "uncovered:js/ts") {
		t.Fatalf("plan = %v: a language-free check covers every language", ids)
	}
	// And a workspace with no applicable check leaves the gate to the model.
	if plan = buildTestPlan(gates.Set{}, langs, []string{"svc"}); len(plan.checks) != 0 {
		t.Fatalf("plan = %v, want none", planIDs(plan))
	}
}

func planIDs(p testPlan) []string {
	var ids []string
	for _, c := range p.checks {
		ids = append(ids, c.ID)
	}
	return ids
}

// A repair write puts every earlier outcome behind it: a check that passed
// before the last write no longer speaks for the file.
func TestWorkspaceTestRun_AWriteAfterThePlanRanInvalidatesIt(t *testing.T) {
	t.Parallel()
	bin := checkGate(t)
	e, result, ws := planTurn(t, nodeGate("check", bin))
	if err := os.WriteFile(filepath.Join(ws, "svc", "value.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.workspaceTestRun(context.Background(), result)
	if got := testRunGate(t, e, result); got != VerifyPassed {
		t.Fatalf("setup: /test_run = %v", got)
	}
	e.noteWrite(result)
	if got := testRunGate(t, e, result); got == VerifyPassed {
		t.Fatal("a pass from before the last write still counts")
	}
}

// W4: the closing sentence claims only what the corpus says the turn owed.
func TestDoneSentence_NamesTheGatesActuallyOwed(t *testing.T) {
	for name, tc := range map[string]struct {
		owed []string
		want string
		not  string
	}{
		"go":       {[]string{"/build", "/test"}, "the build and the tests were both measured green", ""},
		"non-Go":   {[]string{"/test_run"}, "the workspace's test run was measured green", "build"},
		"document": {nil, "none was run", "measured green"},
		"pinned":   {[]string{"/build", "/test", "/pinned"}, "the build, the tests and the check that the tests pin the change", ""},
	} {
		t.Run(name, func(t *testing.T) {
			got := verdictSentence(&ExecutionResult{TurnOutcome: types.MangleAtom("/done"), OwedGates: tc.owed})
			if !strings.Contains(got, tc.want) || (tc.not != "" && strings.Contains(got, tc.not)) {
				t.Fatalf("sentence = %q, want %q and no %q", got, tc.want, tc.not)
			}
		})
	}
}

// The same, through the corpus: the turn's owed gates come from the kernel, so
// a document is not told it was built and tested.
func TestTurnOutcome_TheClosingSentenceSpeaksOnlyOfWhatWasOwed(t *testing.T) {
	t.Parallel()
	e := newObligationExec(t)
	doc := mutationResult()
	doc.WrittenPaths = []string{"Docs/guide.md"}
	settle(e, doc)
	if doc.TurnOutcome != types.MangleAtom("/done") || strings.Contains(verdictSentence(doc), "build") {
		t.Fatalf("document: %v %q", doc.TurnOutcome, verdictSentence(doc))
	}
	e = newObligationExec(t)
	script := mutationResult()
	script.WrittenPaths = []string{"scripts/check.py"}
	script.TestRunSinceLastWrite = &tools.TestRun{Argv: []string{"pytest"}, ExitCode: 0}
	settle(e, script)
	if script.TurnOutcome != types.MangleAtom("/done") {
		t.Fatalf("script: %v %v", script.TurnOutcome, script.MissingEvidence)
	}
	if s := verdictSentence(script); !strings.Contains(s, "test run") || strings.Contains(s, "build") {
		t.Fatalf("script sentence = %q, want the test run and no build", s)
	}
}
