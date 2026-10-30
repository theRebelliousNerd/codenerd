package session

import (
	"codenerd/internal/evidence"
	"codenerd/internal/types"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The executor's ledger names what the turn wrote, so a final answer that
// denies it contradicts the response itself.
func TestCloseChangeEvidence_ResponseNamesWrittenFiles(t *testing.T) {
	e, result := verifyGateExecutor(t)
	result.WriteToolEffects = 1
	result.WrittenPaths = []string{"x.go"}
	result.Response = "did stuff"

	e.closeAcceptanceEvidence(context.Background(), result)
	e.appendEvidenceSummary(result)

	if !strings.Contains(result.Response, "Wrote 1 file(s): x.go") {
		t.Fatalf("response lacks written file ledger: %q", result.Response)
	}
	if !strings.Contains(result.Response, "Evidence: ") {
		t.Fatalf("response lost the Evidence sentence: %q", result.Response)
	}
}

func TestCloseChangeEvidence_TagGatedPackageIsNotAFailure(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("needs go toolchain")
	}
	e, result := verifyGateExecutor(t)
	ws := e.config.WorkspaceRoot
	before, err := snapshotForTest(ws)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(ws, "e2e"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "go.mod"), []byte("module example.com/fixture\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "ok.go"), []byte("package fixture\n\nfunc OK() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "e2e", "gated_test.go"), []byte("//go:build integration\n\npackage e2e\n\nimport \"testing\"\n\nfunc TestGated(t *testing.T) { t.Log(\"gated\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result.WriteToolEffects = 1
	result.WrittenPaths = []string{"e2e/gated_test.go"}
	result.Response = "did stuff"

	// Must stay on the same helper the post-edit gate uses, or a tag-gated
	// package fails the turn twice over.
	if err := e.closeChangeEvidence(context.Background(), result, before); err != nil {
		t.Fatalf("closeChangeEvidence failed for tag-gated package: %v", err)
	}
	if result.TestCheck.Verdict() == VerifyFailed {
		t.Fatalf("tag-gated package must not be VerifyFailed: %+v", result.TestCheck)
	}
}

const (
	probeCalc = "package verifyprobe\n\nfunc Add(a, b int) int { return a + b }\n"
	probeTest = "package verifyprobe\n\nimport \"testing\"\n\n" +
		"func TestAlwaysFails(t *testing.T) { t.Fatal(\"always fails\") }\n" +
		"func TestOK(t *testing.T) { if Add(2, 3) != 5 { t.Fatal(\"bad\") } }\n"
)

// closeProbe runs the closure over a throwaway module whose tests already
// fail on TestAlwaysFails. calc is the file the turn left; the preimage is
// probeCalc, so attribution can tell a new failure from that one.
func closeProbe(t *testing.T, withKernel bool, calc string) (*Executor, *ExecutionResult, error) {
	t.Helper()
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	var e *Executor
	if withKernel {
		e = newObligationExec(t)
	} else {
		e, _ = verifyGateExecutor(t)
	}
	ws := e.config.WorkspaceRoot
	writeWorkspaceFile(t, ws, "go.mod", "module verifyprobe\n\ngo 1.21\n")
	writeWorkspaceFile(t, ws, "calc.go", calc)
	writeWorkspaceFile(t, ws, "calc_test.go", probeTest)
	result := &ExecutionResult{
		WriteToolEffects: 1,
		WrittenPaths:     []string{"calc.go"},
		PreWriteContents: map[string]PreImage{"calc.go": existed(probeCalc)},
		roundsRan:        goWriteRounds(),
	}
	// before is empty: the closure cannot rule the workspace unchanged, so
	// it remeasures. The gates then describe this tree.
	err := e.closeChangeEvidence(context.Background(), result, "")
	return e, result, err
}

// A turn whose only failing tests already failed before it is not a test
// failure. The process exit stays failed, the text still names them, and
// an empty answer does not turn that suite into an unrecovered tool error.
func TestCloseChangeEvidence_PreExistingFailuresDoNotFailTheTurn(t *testing.T) {
	t.Parallel()
	calc := probeCalc + "\nfunc Unrelated() int { return 42 }\n"
	e, result, err := closeProbe(t, true, calc)
	if err != nil {
		t.Fatalf("closeChangeEvidence = %v, want nil: TestAlwaysFails also failed before the turn", err)
	}
	if result.TestCheck.Verdict() != VerifyFailed {
		t.Fatalf("TestCheck = %s, want the process exit kept", result.TestCheck.Verdict())
	}
	if len(result.TestCheck.PreExistingFailures) != 1 || result.TestCheck.PreExistingFailures[0] != "verifyprobe.TestAlwaysFails" {
		t.Fatalf("PreExistingFailures = %v, want [verifyprobe.TestAlwaysFails]", result.TestCheck.PreExistingFailures)
	}
	if !strings.Contains(result.TestCheck.Output, "TestAlwaysFails") {
		t.Fatalf("the text does not name the pre-existing failure:\n%s", result.TestCheck.Output)
	}
	if result.ChangeStage != "checks_passed" {
		t.Fatalf("ChangeStage = %q, want checks_passed", result.ChangeStage)
	}
	if got := derivedVerify(t, e, result.turnAtom(), "/test"); got != VerifyPassed {
		t.Fatalf("test gate = %v, want passing", got)
	}
	result.Response = ""
	// This probe's executor holds the kernel. The read asks that executor.
	e.surfaceToolErrors(result, []string{"run_build: exit status 1"})
	if result.Error != nil {
		t.Fatalf("a suite red only on pre-existing failures is recovered, got %v", result.Error)
	}
}

// One failure the baseline did not have fails the turn, and the error names it.
func TestCloseChangeEvidence_ANewFailureFailsTheTurn(t *testing.T) {
	t.Parallel()
	calc := "package verifyprobe\n\nfunc Add(a, b int) int { return a + b + 1 }\n"
	e, result, err := closeProbe(t, true, calc)
	if !errors.Is(err, ErrVerificationFailed) || !strings.Contains(err.Error(), "TestOK") {
		t.Fatalf("closeChangeEvidence = %v, want ErrVerificationFailed naming TestOK", err)
	}
	if result.ChangeStage == "checks_passed" {
		t.Fatal("checks_passed was granted with a new test failure")
	}
	if got := derivedVerify(t, e, result.turnAtom(), "/test"); got != VerifyFailed {
		t.Fatalf("test gate = %v, want failing: TestOK did not fail before the turn", got)
	}
	// This probe's executor holds the kernel. The read asks that executor.
	if e.turnRecoveredFromToolErrors(result) {
		t.Fatal("a new test failure recovered the turn's tool errors")
	}
}

// With no kernel there is nothing to ask. The raw suite exit fails the turn
// and no verdict is recorded.
func TestCloseChangeEvidence_NoKernelKeepsTheRawTestExit(t *testing.T) {
	t.Parallel()
	calc := probeCalc + "\nfunc Unrelated() int { return 42 }\n"
	e, result, err := closeProbe(t, false, calc)
	if e.kernel != nil {
		t.Fatal("this fixture must have no kernel")
	}
	if !errors.Is(err, ErrVerificationFailed) || !strings.Contains(err.Error(), "TestAlwaysFails") {
		t.Fatalf("closeChangeEvidence = %v, want ErrVerificationFailed naming TestAlwaysFails", err)
	}
	if result.TestCheck.Verdict() != VerifyFailed {
		t.Fatalf("TestCheck = %s, want the raw exit", result.TestCheck.Verdict())
	}
	if result.ChangeStage == "checks_passed" {
		t.Fatal("checks_passed was granted from the raw exit with no kernel")
	}
	// No executor: a nil receiver has no kernel, so this is the raw suite
	// exit and records no verdict. The same empty answer the kernel path
	// recovers stays an error here.
	result.Response = ""
	var raw *Executor
	raw.surfaceToolErrors(result, []string{"run_build: exit status 1"})
	if result.Error == nil {
		t.Fatal("with no kernel the raw failing exit is not recovered")
	}
}

// Pinning is remeasured when the derived gate passed and the suite is still
// red on a test the turn did not cause. turnTests drops that name, so the
// pin run is the test the turn wrote, and a stale pin check is replaced.
func TestCloseChangeEvidence_PinningRerunsWhenPreExistingFailuresPassTheGate(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	e := newObligationExec(t)
	ws := e.config.WorkspaceRoot
	writeWorkspaceFile(t, ws, "go.mod", pinGoMod)
	writeWorkspaceFile(t, ws, "calc.go", pinCalcAfter)
	writeWorkspaceFile(t, ws, "helper.go", pinHelper)
	writeWorkspaceFile(t, ws, "helper_test.go", pinHelperTests)
	writeWorkspaceFile(t, ws, "legacy_test.go", "package pinprobe\n\nimport \"testing\"\n\nfunc TestAlwaysFails(t *testing.T) { t.Fatal(\"always fails\") }\n")
	rounds := goWriteRounds()
	rounds["/pinned"] = true
	result := &ExecutionResult{
		WriteToolEffects: 1,
		WrittenPaths:     []string{"calc.go", "helper.go", "helper_test.go"},
		PreWriteContents: map[string]PreImage{
			"calc.go":        existed(pinCalcBefore),
			"helper.go":      {},
			"helper_test.go": {},
		},
		roundsRan: rounds,
		PinCheck:  BuildVerification{Ran: true, OK: true, Outcome: VerifyPassed, Reason: "stale pin check"},
	}
	err := e.closeChangeEvidence(context.Background(), result, "")
	if err != nil {
		t.Fatalf("closeChangeEvidence = %v, want nil: TestAlwaysFails also failed before the turn", err)
	}
	if result.TestCheck.Verdict() != VerifyFailed {
		t.Fatalf("TestCheck = %s, want the process exit kept", result.TestCheck.Verdict())
	}
	if !strings.Contains(result.TestCheck.Output, "TestAlwaysFails") {
		t.Fatalf("the text does not name the pre-existing failure:\n%s", result.TestCheck.Output)
	}
	if got := derivedVerify(t, e, result.turnAtom(), "/test"); got != VerifyPassed {
		t.Fatalf("test gate = %v, want passing", got)
	}
	if result.PinCheck.Reason == "stale pin check" {
		t.Fatalf("pinning was not remeasured:\nTestCheck %s\n%s", result.TestCheck.Verdict(), result.TestCheck.Output)
	}
	if result.PinCheck.Verdict() != VerifyFailed || !strings.Contains(result.PinCheck.Output, "calc.go: Greet") {
		t.Fatalf("PinCheck = %s (%s):\n%s\nwant failed naming calc.go: Greet", result.PinCheck.Verdict(), result.PinCheck.Reason, result.PinCheck.Output)
	}
	joined := strings.Join(result.PinCheck.Command, " ")
	if strings.Contains(joined, "TestAlwaysFails") || !strings.Contains(joined, "TestPolite") {
		t.Fatalf("pin command = %s, want TestPolite and not the pre-existing failure", joined)
	}
}

// pinRoundFixture is the pin module plus a test that already failed before
// the turn. breakPolite makes TestPolite a failure the baseline did not have.
// TestCheck is the real gate run, so the raw exit is failed in both cases.
func pinRoundFixture(t *testing.T, breakPolite bool) (*Executor, *ExecutionResult) {
	t.Helper()
	e := newObligationExec(t)
	ws := e.workspaceForVerification()
	helper := pinHelper
	if breakPolite {
		helper = strings.ReplaceAll(pinHelper, `return s + "!"`, `return s`)
	}
	writeWorkspaceFile(t, ws, "go.mod", pinGoMod)
	writeWorkspaceFile(t, ws, "calc.go", pinCalcAfter)
	writeWorkspaceFile(t, ws, "helper.go", helper)
	writeWorkspaceFile(t, ws, "helper_test.go", pinHelperTests)
	writeWorkspaceFile(t, ws, "legacy_test.go", "package pinprobe\n\nimport \"testing\"\n\nfunc TestAlwaysFails(t *testing.T) { t.Fatal(\"always fails\") }\n")
	result := &ExecutionResult{
		WriteToolEffects: 1,
		WrittenPaths:     []string{"calc.go", "helper.go", "helper_test.go"},
		PreWriteContents: map[string]PreImage{
			"calc.go":        existed(pinCalcBefore),
			"helper.go":      {},
			"helper_test.go": {},
		},
	}
	fresh, _ := gateTests(context.Background(), ws, result, false)
	result.TestCheck = fresh
	return e, result
}

// The /pinned round used to return before measuring when the raw suite exit
// was failed. A turn whose only failures already failed still passes the
// derived gate, and the round measures. A new failure does not.
func TestVerifyAndRepairPinning_PreExistingFailuresMeasureAndANewFailureDoesNot(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	const sentinel = "not measured"

	t.Run("pre-existing", func(t *testing.T) {
		e, result := pinRoundFixture(t, false)
		if result.TestCheck.Verdict() != VerifyFailed {
			t.Fatalf("TestCheck = %s, want the raw exit failed", result.TestCheck.Verdict())
		}
		if !result.TestCheck.BaselineRan || len(result.TestCheck.PreExistingFailures) != 1 || result.TestCheck.PreExistingFailures[0] != "pinprobe.TestAlwaysFails" {
			t.Fatalf("attribution = ran %v pre-existing %v\n%s", result.TestCheck.BaselineRan, result.TestCheck.PreExistingFailures, result.TestCheck.Output)
		}
		result.PinCheck = BuildVerification{Reason: sentinel}
		if _, _, err := e.verifyAndRepairPinning(context.Background(), nil, "", nil, nil, nil, result); err != nil {
			t.Fatalf("verifyAndRepairPinning: %v", err)
		}
		if got := derivedVerify(t, e, result.turnAtom(), "/test"); got != VerifyPassed {
			t.Fatalf("test gate = %v, want passing", got)
		}
		if result.PinCheck.Reason == sentinel || !result.PinCheck.Ran {
			t.Fatalf("the round did not measure /pinned: %s (%s)\n%s", result.PinCheck.Verdict(), result.PinCheck.Reason, result.PinCheck.Output)
		}
		if result.PinCheck.Verdict() != VerifyFailed || !strings.Contains(result.PinCheck.Output, "calc.go: Greet") {
			t.Fatalf("PinCheck = %s (%s):\n%s\nwant failed naming calc.go: Greet", result.PinCheck.Verdict(), result.PinCheck.Reason, result.PinCheck.Output)
		}
		joined := strings.Join(result.PinCheck.Command, " ")
		if strings.Contains(joined, "TestAlwaysFails") || !strings.Contains(joined, "TestPolite") {
			t.Fatalf("pin command = %s, want TestPolite and not the pre-existing failure", joined)
		}
	})

	t.Run("new failure", func(t *testing.T) {
		e, result := pinRoundFixture(t, true)
		if result.TestCheck.Verdict() != VerifyFailed {
			t.Fatalf("TestCheck = %s, want the raw exit failed", result.TestCheck.Verdict())
		}
		var hasPolite bool
		for _, name := range failedTestNames(result.TestCheck.Result) {
			if name == "TestPolite" {
				hasPolite = true
			}
		}
		if !hasPolite {
			t.Fatalf("head failures = %v, want TestPolite\n%s", failedTestNames(result.TestCheck.Result), result.TestCheck.Output)
		}
		result.PinCheck = BuildVerification{Reason: sentinel}
		if _, _, err := e.verifyAndRepairPinning(context.Background(), nil, "", nil, nil, nil, result); err != nil {
			t.Fatalf("verifyAndRepairPinning: %v", err)
		}
		if got := derivedVerify(t, e, result.turnAtom(), "/test"); got != VerifyFailed {
			t.Fatalf("test gate = %v, want failing", got)
		}
		if result.PinCheck.Reason != sentinel || result.PinCheck.Ran {
			t.Fatalf("a new failure measured /pinned: %s (%s)\n%s", result.PinCheck.Verdict(), result.PinCheck.Reason, result.PinCheck.Output)
		}
	})
}

// A fix checks its own package before the final suite, and never repeats a
// finished suite on the same covered tree, even when that suite failed.
func TestCloseChangeEvidence_FixSuiteIsFinalAndRevisionBound(t *testing.T) {
	e, result := verifyGateExecutor(t)
	e.kernel = realKernel(t)
	ws := e.config.WorkspaceRoot
	files := map[string]string{
		"go.mod":      "module finalprobe\n\ngo 1.22\n",
		"a/a.go":      "package a\nfunc Value() int { return 1 }\n",
		"a/a_test.go": "package a\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value() != 1 { t.Fatal(\"wrong value\") } }\n",
		"b/b.go":      "package b\nfunc Value() int { return 0 }\n",
		"b/b_test.go": "package b\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value() != 1 { t.Fatal(\"wrong value\") } }\n",
	}
	for path, text := range files {
		abs := filepath.Join(ws, path)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	result.Intent.Verb = "/fix"
	result.WrittenPaths = []string{"a/a.go"}
	result.roundsRan = map[string]bool{"/build": true, "/test": true}
	real := verifyTestRunner
	var commands []string
	verifyTestRunner = func(ctx context.Context, dir string, env []string, name string, args []string) ([]byte, error) {
		commands = append(commands, strings.Join(args, " "))
		return real(ctx, dir, env, name, args)
	}
	t.Cleanup(func() { verifyTestRunner = real })
	ctx := context.Background()
	err := e.closeChangeEvidence(ctx, result, "")
	if !errors.Is(err, ErrVerificationFailed) {
		t.Fatalf("unrelated failing test escaped final suite: %v; commands %v", err, commands)
	}
	if result.TestCheck.Verdict() != VerifyPassed || result.FinalTestCheck == nil || result.FinalTestCheck.Verdict() != VerifyFailed {
		t.Fatalf("narrow and final measurements lost their scopes: own %+v, final %+v", result.TestCheck, result.FinalTestCheck)
	}
	if suiteExit(result) != VerifyFailed || !strings.Contains(failedChecksSummary(result), "final suite fails") {
		t.Fatalf("final failure missing from verdict or diagnostic: %s", failedChecksSummary(result))
	}
	if stream := suiteFailureResult(result, suiteFailureText(result)); stream == nil || stream != result.FinalTestCheck.Result {
		t.Fatal("final failure output is not paired with its runner evidence")
	}
	fullRuns := func() int {
		n := 0
		for _, cmd := range commands {
			if strings.HasSuffix(cmd, " ./...") {
				n++
			}
		}
		return n
	}
	if fullRuns() != 1 || len(commands) < 2 || !strings.HasSuffix(commands[0], " ./a") {
		t.Fatalf("want narrow then one full suite, got %v", commands)
	}
	if err := e.closeChangeEvidence(ctx, result, ""); !errors.Is(err, ErrVerificationFailed) || fullRuns() != 1 {
		t.Fatalf("unchanged failing suite repeated or lost: %v; %v", err, commands)
	}
	// This package is outside WrittenPaths. Whole-suite evidence must still expire.
	if err := os.WriteFile(filepath.Join(ws, "b/b.go"), []byte("package b\nfunc Value() int { return 1 }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := e.closeChangeEvidence(ctx, result, ""); err != nil {
		t.Fatalf("repaired suite: %v; %v", err, commands)
	}
	if fullRuns() != 2 || result.ChangeStage != "checks_passed" {
		t.Fatalf("changed covered input did not refresh final evidence: stage %s; %v", result.ChangeStage, commands)
	}
	narrowRuns := 0
	for _, command := range commands {
		if strings.HasSuffix(command, " ./a") {
			narrowRuns++
		}
	}
	if narrowRuns != 2 {
		t.Fatalf("covered input changed outside WrittenPaths without refreshing narrow evidence: %v", commands)
	}
	if err := e.closeChangeEvidence(ctx, result, ""); err != nil || fullRuns() != 2 {
		t.Fatalf("unchanged green suite repeated: %v; %v", err, commands)
	}
}

func TestCloseChangeEvidence_FixDoesNotRunFullSuiteBeforeNarrowPass(t *testing.T) {
	e, result := verifyGateExecutor(t)
	e.kernel = realKernel(t)
	ws := e.config.WorkspaceRoot
	for path, text := range map[string]string{
		"go.mod":       "module narrowprobe\n\ngo 1.22\n",
		"main.go":      "package narrowprobe\nfunc Value() int { return 0 }\n",
		"main_test.go": "package narrowprobe\nimport \"testing\"\nfunc TestValue(t *testing.T) { if Value()!=1 { t.Fatal(\"wrong value\") } }\n",
	} {
		if err := os.WriteFile(filepath.Join(ws, path), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	result.Intent.Verb = "/fix"
	result.roundsRan = map[string]bool{"/build": true, "/test": true}
	real := verifyTestRunner
	verifyTestRunner = func(ctx context.Context, dir string, env []string, name string, args []string) ([]byte, error) {
		for _, arg := range args {
			if arg == "./..." {
				t.Error("full suite ran before narrow tests passed")
			}
		}
		return real(ctx, dir, env, name, args)
	}
	t.Cleanup(func() { verifyTestRunner = real })
	if err := e.closeChangeEvidence(context.Background(), result, ""); !errors.Is(err, ErrVerificationFailed) {
		t.Fatalf("narrow failure was lost: %v", err)
	}
}

func TestCloseChangeEvidence_FinalScopeErrorsCannotCertifyTheFix(t *testing.T) {
	queryErr := errors.New("final scope query failed")
	cases := []struct {
		name          string
		rows          []types.Fact
		queryErr      error
		cancelAtQuery bool
		want          string
		cause         error
	}{
		{name: "query failure", queryErr: queryErr, want: "query fix verification scope", cause: queryErr},
		{name: "unexpected scope", rows: []types.Fact{scopeRow("/final", "/narrow")}, want: "unexpected final fix verification scope /narrow"},
		{name: "final verifier snapshot failure", rows: []types.Fact{scopeRow("/final", "/full")}, cancelAtQuery: true, want: "snapshot final test inputs", cause: context.Canceled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, result := verifyGateExecutor(t)
			ws := e.config.WorkspaceRoot
			if err := os.WriteFile(filepath.Join(ws, "main.go"), []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			result.Intent.Verb = "/fix"
			result.roundsRan = map[string]bool{"/build": true, "/test": true}
			result.BuildCheck = BuildVerification{Ran: true, Outcome: VerifyPassed}
			result.TestCheck = TestVerification{Ran: true, Outcome: VerifyPassed}
			// The closure must discard final evidence from an earlier invocation even
			// when the narrow gates still describe exactly these bytes.
			result.FinalTestCheck = &TestVerification{Ran: true, Outcome: VerifyPassed}
			before, err := evidence.Snapshot(context.Background(), ws)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			queries := 0
			kernel := &scopeQueryKernel{scopeRows: tc.rows, scopeErr: tc.queryErr, onScopeQuery: func() {
				queries++
				if tc.cancelAtQuery {
					cancel()
				}
			}}
			e.kernel = kernel
			launches := countTestLaunches(t)
			err = e.closeChangeEvidence(ctx, result, before)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("closure accepted unavailable final evidence: %v", err)
			}
			if tc.cause != nil && !errors.Is(err, tc.cause) {
				t.Fatalf("closure lost the underlying cause: %v", err)
			}
			if queries != 1 || *launches != 0 {
				t.Fatalf("wrong policy/verification ordering: queries %d, test launches %d", queries, *launches)
			}
			if result.ChangeStage != "artifact_changed" || result.ChecksSnapshot != "" || result.FinalTestCheck != nil {
				t.Fatalf("failed final verification certified the fix: stage %s, snapshot %q, final %+v", result.ChangeStage, result.ChecksSnapshot, result.FinalTestCheck)
			}
			if result.TestCheck.Verdict() != VerifyPassed || result.BuildCheck.Verdict() != VerifyPassed {
				t.Fatal("final-scope failure corrupted the independent narrow measurements")
			}
		})
	}
}

func TestSuiteFailureResult_NilResultHasNoRunnerEvidence(t *testing.T) {
	for _, seed := range []string{"", "tests failed", "compiler diagnostic"} {
		if stream := suiteFailureResult(nil, seed); stream != nil {
			t.Fatalf("nil execution supplied runner evidence for %q: %+v", seed, stream)
		}
	}
}
