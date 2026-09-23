package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/evidence"
	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// B1: a timed-out repair recheck must not become verified success.
//
// verifyBuild/verifyTests used to report a budget exhaustion as Ran=false,
// and the repair gates only rejected a recheck when recheck.Ran &&
// !recheck.OK — so failure -> repair -> timeout overwrote the original
// compiler failure with "not run" and fell through to "repaired
// successfully" plus a nil error. These tests pin the explicit outcome
// contract: the original failure is retained until an affirmative pass on
// the final workspace clears it, and unverified completion is reported
// separately from verified success.
//
// The tests run against scripted command doubles through the runner seams,
// so no test sleeps out a four-minute budget.

// stubVerifySeams installs hermetic verification doubles: scripted runners, a
// fake toolchain path, and shrunk budgets. Everything is restored on cleanup.
// These tests must never run in parallel with each other.
func stubVerifySeams(t *testing.T, buildBudget, testBudget time.Duration, buildRunner, testRunner verifyCommandRunner) {
	t.Helper()
	oldBuildBudget, oldTestBudget := buildVerifyTimeout, testVerifyTimeout
	oldBuildRunner, oldTestRunner := verifyBuildRunner, verifyTestRunner
	oldLookPath := verifyLookPath
	buildVerifyTimeout, testVerifyTimeout = buildBudget, testBudget
	verifyBuildRunner, verifyTestRunner = buildRunner, testRunner
	verifyLookPath = func(string) (string, error) { return "/fake/go", nil }
	t.Cleanup(func() {
		buildVerifyTimeout, testVerifyTimeout = oldBuildBudget, oldTestBudget
		verifyBuildRunner, verifyTestRunner = oldBuildRunner, oldTestRunner
		verifyLookPath = oldLookPath
	})
}

// scriptVerifyRunner replays one scripted result per invocation and records
// every argv it was handed, so tests can assert call counts and provenance.
type scriptVerifyRunner struct {
	mu      sync.Mutex
	calls   int
	invoked [][]string
	script  []func(ctx context.Context) ([]byte, error)
}

func (s *scriptVerifyRunner) runWithCtx(ctx context.Context, _ string, _ []string, name string, args []string) ([]byte, error) {
	s.mu.Lock()
	s.calls++
	argv := append([]string{name}, args...)
	s.invoked = append(s.invoked, argv)
	idx := s.calls - 1
	if idx >= len(s.script) {
		idx = len(s.script) - 1
	}
	entry := s.script[idx]
	s.mu.Unlock()
	return entry(ctx)
}

func (s *scriptVerifyRunner) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// verifyPass scripts a clean command exit.
func verifyPass() func(context.Context) ([]byte, error) {
	return func(context.Context) ([]byte, error) { return nil, nil }
}

// verifyFail scripts a command that ran and failed with the given output.
func verifyFail(output string) func(context.Context) ([]byte, error) {
	return func(context.Context) ([]byte, error) { return []byte(output), errors.New("exit status 1") }
}

// verifyHang scripts a command that never finishes on its own: it blocks
// until its context ends, then reports what it saw. Production doubles MUST
// honor ctx or timeout tests hang for the whole budget.
func verifyHang(partial string) func(context.Context) ([]byte, error) {
	return func(ctx context.Context) ([]byte, error) {
		<-ctx.Done()
		return []byte(partial), ctx.Err()
	}
}

// verifySleep scripts a command that finishes after d, ignoring ctx until
// then — the shape of a slow-but-healthy toolchain.
func verifySleep(d time.Duration) func(context.Context) ([]byte, error) {
	return func(ctx context.Context) ([]byte, error) {
		select {
		case <-time.After(d):
			return nil, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// verifyProseRepair answers every repair round with prose and no tool calls:
// a repair round that wrote nothing.
type verifyProseRepair struct {
	*MockLLMClient
	calls int
}

func (p *verifyProseRepair) CompleteWithToolResults(_ context.Context, _ string, _ []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	p.calls++
	return &types.LLMToolResponse{Text: "repaired in prose"}, nil
}

func snapshotForTest(ws string) (string, error) {
	return evidence.Snapshot(context.Background(), ws)
}

func writeFileForTest(ws, name, content string) error {
	return os.WriteFile(filepath.Join(ws, name), []byte(content), 0o644)
}

func verifyGateExecutor(t *testing.T) (*Executor, *ExecutionResult) {
	t.Helper()
	cfg := DefaultExecutorConfig()
	cfg.EnableSafetyGate = false
	cfg.VerifyBuildAfterEdits = true
	cfg.VerifyTestsAfterEdits = true
	cfg.WorkspaceRoot = t.TempDir()
	e := &Executor{config: cfg}
	result := &ExecutionResult{SuccessfulWriteTools: 1, WrittenPaths: []string{"main.go"}}
	return e, result
}

// inTurnWorkingLoop is the context a turn's verification runs in: inside the
// turn's working loop, whose policy ends a repair attempt's rounds (a repair
// outside one refuses, as the tool loop does).
func inTurnWorkingLoop(t *testing.T, e *Executor) context.Context {
	t.Helper()
	ctx, closeLoop, err := e.beginWorkingLoop(context.Background(), "verify the change", &prompt.CompilationContext{ShardID: "probe"})
	if err != nil {
		t.Fatalf("beginWorkingLoop: %v", err)
	}
	t.Cleanup(closeLoop)
	return ctx
}

func TestVerifyBuild_TimeoutIsIndeterminateNotSilent(t *testing.T) {
	builds := &scriptVerifyRunner{script: []func(context.Context) ([]byte, error){verifyHang("still compiling...")}}
	stubVerifySeams(t, 40*time.Millisecond, time.Minute, builds.runWithCtx, verifyPassAsRunner())

	v := verifyBuild(context.Background(), t.TempDir(), nil)
	if v.Verdict() != VerifyIndeterminate {
		t.Fatalf("timed-out build verdict = %q, want indeterminate", v.Verdict())
	}
	if !v.Ran {
		t.Error("a build that ran to the budget must report Ran=true; Ran=false means skipped")
	}
	if v.OK {
		t.Error("an indeterminate build must not report OK")
	}
	if !strings.Contains(v.Output, "still compiling") {
		t.Errorf("partial compiler output was dropped on timeout: %q", v.Output)
	}
	if len(v.Command) == 0 || v.Command[0] != "go" {
		t.Errorf("missing command provenance on timeout: %q", v.Command)
	}
	if builds.callCount() != 1 {
		t.Errorf("runner calls = %d, want 1", builds.callCount())
	}
}

func TestVerifyTests_TimeoutIsIndeterminateNotSilent(t *testing.T) {
	tests := &scriptVerifyRunner{script: []func(context.Context) ([]byte, error){verifyHang("still testing...")}}
	stubVerifySeams(t, time.Minute, 40*time.Millisecond, verifyPassAsRunner(), tests.runWithCtx)

	v := verifyTests(context.Background(), t.TempDir(), []string{"."})
	if v.Verdict() != VerifyIndeterminate {
		t.Fatalf("timed-out test verdict = %q, want indeterminate", v.Verdict())
	}
	if !v.Ran {
		t.Error("a test run that hit the budget must report Ran=true; Ran=false means skipped")
	}
	if v.OK {
		t.Error("indeterminate tests must not report OK")
	}
	if !strings.Contains(v.Output, "still testing") {
		t.Errorf("partial test output was dropped on timeout: %q", v.Output)
	}
}

func verifyPassAsRunner() verifyCommandRunner {
	pass := verifyPass()
	return func(ctx context.Context, _ string, _ []string, _ string, _ []string) ([]byte, error) {
		return pass(ctx)
	}
}

// The core B1 reproduction: known compiler failure -> repair round ->
// recheck timeout must retain the original failure and must not report a
// verified repair. On the old code this returned nil with BuildCheck
// overwritten to Ran=false and logged "Build repaired successfully".
func TestVerifyAndRepairBuild_RecheckTimeoutRetainsOriginalFailure(t *testing.T) {
	const compilerErr = "main.go:3: undefined: neverWritten"
	builds := &scriptVerifyRunner{script: []func(context.Context) ([]byte, error){
		verifyFail(compilerErr),
		verifyHang("recheck still compiling..."),
	}}
	stubVerifySeams(t, 40*time.Millisecond, time.Minute, builds.runWithCtx, verifyPassAsRunner())

	e, result := verifyGateExecutor(t)
	trp := &verifyProseRepair{}
	jitCfg := &jitconfig.EffectiveAgentRuntimeConfig{}

	_, _, err := e.verifyAndRepairBuild(inTurnWorkingLoop(t, e), trp, "system", nil, nil, nil, jitCfg, result)
	if err != nil {
		t.Fatalf("unverified completion must not fail the turn (timeout is not proof of broken code), got: %v", err)
	}
	if builds.callCount() != 2 {
		t.Fatalf("runner calls = %d, want 2 (initial failure + timed-out recheck)", builds.callCount())
	}
	if got := result.BuildCheck.Verdict(); got != VerifyFailed {
		t.Fatalf("retained build verdict = %q, want the original failure", got)
	}
	if !strings.Contains(result.BuildCheck.Output, "neverWritten") {
		t.Errorf("original compiler diagnostics were lost: %q", result.BuildCheck.Output)
	}
}

// Mirror of the build reproduction for the test gate. The gate also runs a
// build recheck between repair rounds, so the build double must pass while
// the test double fails once and then hangs.
func TestVerifyAndRepairTests_RecheckTimeoutRetainsOriginalFailure(t *testing.T) {
	const testErr = "--- FAIL: TestAdd (0.00s)\n    calc_test.go:5: intentional failure"
	tests := &scriptVerifyRunner{script: []func(context.Context) ([]byte, error){
		verifyFail(testErr),
		verifyHang("recheck still testing..."),
	}}
	builds := &scriptVerifyRunner{script: []func(context.Context) ([]byte, error){verifyPass()}}
	stubVerifySeams(t, time.Minute, 40*time.Millisecond, builds.runWithCtx, tests.runWithCtx)

	e, result := verifyGateExecutor(t)
	trp := &verifyProseRepair{}
	jitCfg := &jitconfig.EffectiveAgentRuntimeConfig{}

	_, _, err := e.verifyAndRepairTests(inTurnWorkingLoop(t, e), trp, "system", nil, nil, jitCfg, result)
	if err != nil {
		t.Fatalf("unverified completion must not fail the turn, got: %v", err)
	}
	if tests.callCount() != 2 {
		t.Fatalf("test runner calls = %d, want 2 (initial failure + timed-out recheck)", tests.callCount())
	}
	if got := result.TestCheck.Verdict(); got != VerifyFailed {
		t.Fatalf("retained test verdict = %q, want the original failure", got)
	}
	if !strings.Contains(result.TestCheck.Output, "intentional failure") {
		t.Errorf("original test diagnostics were lost: %q", result.TestCheck.Output)
	}
}

func TestVerifyAndRepairTests_PreExistingFailuresNeedNoRepair(t *testing.T) {
	const failing = "--- FAIL: TestAlwaysFails (0.00s)\n    x_test.go:5: always fails\nFAIL"
	tests := &scriptVerifyRunner{script: []func(context.Context) ([]byte, error){verifyFail(failing), verifyFail(failing)}}
	builds := &scriptVerifyRunner{script: []func(context.Context) ([]byte, error){verifyPass()}}
	stubVerifySeams(t, time.Minute, time.Minute, builds.runWithCtx, tests.runWithCtx)
	e, result := verifyGateExecutor(t)
	result.PreWriteContents = map[string]PreImage{"main.go": existed("package main\n")}
	trp := &verifyProseRepair{}
	_, _, err := e.verifyAndRepairTests(context.Background(), trp, "system", nil, nil, &jitconfig.EffectiveAgentRuntimeConfig{}, result)
	if err != nil {
		t.Fatalf("verifyAndRepairTests returned error: %v", err)
	}
	if trp.calls != 0 {
		t.Errorf("expected no repair rounds, got %d", trp.calls)
	}
	if got := tests.callCount(); got != 2 {
		t.Errorf("tests.callCount() = %d; want 2 (head run + baseline overlay run)", got)
	}
	if got := result.TestCheck.Verdict(); got != VerifyPassed {
		t.Errorf("Verdict() = %v; want VerifyPassed", got)
	}
	if len(result.TestCheck.PreExistingFailures) != 1 || result.TestCheck.PreExistingFailures[0] != "TestAlwaysFails" {
		t.Errorf("PreExistingFailures = %v; want [TestAlwaysFails]", result.TestCheck.PreExistingFailures)
	}
}

// Anti-overcorrection: a repair the recheck affirmatively passes still
// clears the failure and completes.
func TestVerifyAndRepairBuild_TrueRepairClearsFailure(t *testing.T) {
	builds := &scriptVerifyRunner{script: []func(context.Context) ([]byte, error){
		verifyFail("main.go:3: undefined: neverWritten"),
		verifyPass(),
	}}
	stubVerifySeams(t, time.Minute, time.Minute, builds.runWithCtx, verifyPassAsRunner())
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectWrite, Name: "create_file", Category: tools.CategoryCode,
		Execute: func(context.Context, map[string]any) (string, error) { return "written", nil },
	})

	e, result := verifyGateExecutor(t)
	trp := &verifyWriteRepair{}
	jitCfg := &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: []string{"create_file"}}
	toolDefs := e.buildToolDefinitions(jitCfg)

	_, _, err := e.verifyAndRepairBuild(inTurnWorkingLoop(t, e), trp, "system", nil, nil, toolDefs, jitCfg, result)
	if err != nil {
		t.Fatalf("affirmatively repaired build must complete, got: %v", err)
	}
	if got := result.BuildCheck.Verdict(); got != VerifyPassed {
		t.Fatalf("build verdict after true repair = %q, want passed", got)
	}
	if result.BuildCheck.Output != "" {
		t.Errorf("passing recheck should have empty output, got %q", result.BuildCheck.Output)
	}
}

// verifyWriteRepair answers the repair round with one write-tool call.
type verifyWriteRepair struct {
	*MockLLMClient
}

func (p *verifyWriteRepair) CompleteWithToolResults(_ context.Context, _ string, _ []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{ToolCalls: []types.ToolCall{
		{ID: "repair-write", Name: "create_file", Input: map[string]any{"path": "main.go", "content": "fixed"}},
	}}, nil
}

// When the final workspace cannot be verified, checks_passed is withheld and
// the turn is labeled unverified — without failing on unknown evidence.
func TestCloseChangeEvidence_TimeoutWithholdsChecksPassed(t *testing.T) {
	builds := &scriptVerifyRunner{script: []func(context.Context) ([]byte, error){verifyHang("build pending...")}}
	tests := &scriptVerifyRunner{script: []func(context.Context) ([]byte, error){verifyHang("tests pending...")}}
	stubVerifySeams(t, 40*time.Millisecond, 40*time.Millisecond, builds.runWithCtx, tests.runWithCtx)

	e, result := verifyGateExecutor(t)
	result.Response = "did stuff"
	ws := e.config.WorkspaceRoot
	before, err := snapshotForTest(ws)
	if err != nil {
		t.Fatalf("before snapshot: %v", err)
	}
	if err := writeFileForTest(ws, "main.go", "package main\n"); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	if err := e.closeChangeEvidence(context.Background(), result, before); err != nil {
		t.Fatalf("unverifiable final workspace must not fail the turn, got: %v", err)
	}
	if result.ChangeStage == "checks_passed" {
		t.Fatal("checks_passed was granted without an affirmative pass")
	}
	if result.BuildCheck.Verdict() != VerifyIndeterminate {
		t.Errorf("final build verdict = %q, want indeterminate", result.BuildCheck.Verdict())
	}
	if result.TestCheck.Verdict() != VerifyIndeterminate {
		t.Errorf("final test verdict = %q, want indeterminate", result.TestCheck.Verdict())
	}
}

// A missing toolchain is a skip with a reason, not a silent hole and not a
// failure. It must also stay fast: no budget is ever started.
func TestVerifyBuild_MissingToolchainIsSkipped(t *testing.T) {
	builds := &scriptVerifyRunner{script: []func(context.Context) ([]byte, error){verifyPass()}}
	stubVerifySeams(t, time.Minute, time.Minute, builds.runWithCtx, verifyPassAsRunner())
	verifyLookPath = func(string) (string, error) { return "", errors.New("exec: \"go\": not found in PATH") }

	v := verifyBuild(context.Background(), t.TempDir(), nil)
	if v.Verdict() != VerifySkipped {
		t.Fatalf("missing-toolchain verdict = %q, want skipped", v.Verdict())
	}
	if v.Ran || v.OK {
		t.Errorf("a skipped verification must report Ran=false OK=false, got Ran=%v OK=%v", v.Ran, v.OK)
	}
	if v.Output != "" {
		t.Errorf("a skipped verification must keep Output empty, got %q", v.Output)
	}
	if v.Reason == "" {
		t.Error("a skipped verification must say why")
	}
	if builds.callCount() != 0 {
		t.Errorf("runner calls = %d, want 0 (nothing to run without a toolchain)", builds.callCount())
	}
}

// The verification budget is independent of the turn's remaining time: a
// parent soft deadline must not manufacture a verification failure.
func TestVerifyBuild_ParentDeadlineDoesNotAbort(t *testing.T) {
	builds := &scriptVerifyRunner{script: []func(context.Context) ([]byte, error){verifySleep(100 * time.Millisecond)}}
	stubVerifySeams(t, 10*time.Second, time.Minute, builds.runWithCtx, verifyPassAsRunner())

	parent, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	v := verifyBuild(parent, t.TempDir(), nil)
	if v.Verdict() != VerifyPassed {
		t.Fatalf("verdict under an expired parent deadline = %q, want passed (independent budget)", v.Verdict())
	}
}

// Explicit operator cancellation aborts the run promptly and the kill
// reaches the subprocess context.
func TestVerifyBuild_ParentCancelAbortsAndKills(t *testing.T) {
	sawDone := make(chan struct{})
	builds := &scriptVerifyRunner{script: []func(context.Context) ([]byte, error){
		func(ctx context.Context) ([]byte, error) {
			<-ctx.Done()
			close(sawDone)
			return []byte("partial..."), ctx.Err()
		},
	}}
	stubVerifySeams(t, 5*time.Second, time.Minute, builds.runWithCtx, verifyPassAsRunner())

	parent, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	v := verifyBuild(parent, t.TempDir(), nil)
	elapsed := time.Since(start)
	if v.Verdict() != VerifyCanceled {
		t.Fatalf("verdict after operator cancel = %q, want canceled", v.Verdict())
	}
	if elapsed > 2*time.Second {
		t.Fatalf("canceled verification took %v; the kill must land promptly, not at budget expiry", elapsed)
	}
	select {
	case <-sawDone:
	default:
		t.Fatal("the subprocess context was never canceled; the kill did not propagate")
	}
}

// A parent that is already canceled skips the run entirely: no subprocess.
func TestVerifyBuild_PreCanceledParentSkipsWithoutRunning(t *testing.T) {
	builds := &scriptVerifyRunner{script: []func(context.Context) ([]byte, error){verifyPass()}}
	stubVerifySeams(t, time.Minute, time.Minute, builds.runWithCtx, verifyPassAsRunner())

	parent, cancel := context.WithCancel(context.Background())
	cancel()
	v := verifyBuild(parent, t.TempDir(), nil)
	if v.Verdict() != VerifyCanceled {
		t.Fatalf("verdict with pre-canceled parent = %q, want canceled", v.Verdict())
	}
	if builds.callCount() != 0 {
		t.Errorf("runner calls = %d, want 0 (canceled before start)", builds.callCount())
	}
}

// Verdict derives the honest outcome for structs built before Outcome
// existed, so old call sites and fixtures keep their meaning.
func TestVerifyOutcome_VerdictDerivation(t *testing.T) {
	buildCases := []struct {
		name string
		v    BuildVerification
		want VerifyOutcome
	}{
		{"explicit wins over fields", BuildVerification{Ran: false, OK: false, Outcome: VerifyIndeterminate}, VerifyIndeterminate},
		{"ran and ok is passed", BuildVerification{Ran: true, OK: true}, VerifyPassed},
		{"ran and not ok is failed", BuildVerification{Ran: true, OK: false}, VerifyFailed},
		{"zero value is skipped", BuildVerification{}, VerifySkipped},
		{"not ran is never a pass", BuildVerification{Ran: false, OK: true}, VerifySkipped},
	}
	for _, tc := range buildCases {
		t.Run("build/"+tc.name, func(t *testing.T) {
			if got := tc.v.Verdict(); got != tc.want {
				t.Errorf("Verdict() = %q, want %q", got, tc.want)
			}
		})
	}
	testCases := []struct {
		name string
		v    TestVerification
		want VerifyOutcome
	}{
		{"explicit wins over fields", TestVerification{Ran: true, OK: true, Outcome: VerifyCanceled}, VerifyCanceled},
		{"ran and ok is passed", TestVerification{Ran: true, OK: true}, VerifyPassed},
		{"ran and not ok is failed", TestVerification{Ran: true, OK: false}, VerifyFailed},
		{"zero value is skipped", TestVerification{}, VerifySkipped},
		{"not ran is never a pass", TestVerification{Ran: false, OK: true}, VerifySkipped},
	}
	for _, tc := range testCases {
		t.Run("tests/"+tc.name, func(t *testing.T) {
			if got := tc.v.Verdict(); got != tc.want {
				t.Errorf("Verdict() = %q, want %q", got, tc.want)
			}
		})
	}
}
