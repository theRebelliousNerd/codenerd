package regression

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell assertions target the bash path")
	}
}

func intPtr(n int) *int { return &n }

// =============================================================================
// Validation / empty-suite policy
// =============================================================================

func TestValidate_ShouldRejectEmptySuite(t *testing.T) {
	err := (&Battery{Version: 1}).Validate()
	if err == nil {
		t.Fatal("an empty battery must be a configuration error, not a vacuous pass")
	}
	if !strings.Contains(err.Error(), "no tasks") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_ShouldRejectMalformedTasks(t *testing.T) {
	tests := map[string]Battery{
		"missing id": {Tasks: []Task{{Type: "shell", Command: "echo hi"}}},
		"empty command": {Tasks: []Task{
			{ID: "a", Type: "shell", Command: "   "},
		}},
		"unsupported type": {Tasks: []Task{
			{ID: "a", Type: "python", Command: "print(1)"},
		}},
		"duplicate id": {Tasks: []Task{
			{ID: "a", Type: "shell", Command: "echo 1"},
			{ID: "a", Type: "shell", Command: "echo 2"},
		}},
		"negative timeout": {Tasks: []Task{
			{ID: "a", Type: "shell", Command: "echo 1", TimeoutSec: -5},
		}},
		"bad version": {Version: 99, Tasks: []Task{
			{ID: "a", Type: "shell", Command: "echo 1"},
		}},
	}
	for name, b := range tests {
		t.Run(name, func(t *testing.T) {
			battery := b
			if err := battery.Validate(); err == nil {
				t.Errorf("expected %s to be rejected", name)
			}
		})
	}
}

func TestValidate_ShouldAcceptOmittedTypeAndVersion(t *testing.T) {
	b := &Battery{Tasks: []Task{{ID: "a", Command: "echo hi"}}}
	if err := b.Validate(); err != nil {
		t.Errorf("type and version should default: %v", err)
	}
}

func TestLoadBattery_ShouldRejectMissingFile(t *testing.T) {
	if _, err := LoadBattery(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("expected an error for a missing battery file")
	}
}

func TestLoadBattery_ShouldRejectBadYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "battery.yaml")
	if err := os.WriteFile(path, []byte("version: 1\ntasks: [oops"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadBattery(path); err == nil {
		t.Fatal("expected a parse error")
	}
}

func TestLoadBattery_ShouldRejectEmptyTaskList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "battery.yaml")
	if err := os.WriteFile(path, []byte("version: 1\ntasks: []\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadBattery(path); err == nil {
		t.Fatal("an empty task list must be rejected at load time")
	}
}

// =============================================================================
// Expectations
// =============================================================================

func TestExpectContains_ShouldFailWhenAbsent(t *testing.T) {
	skipOnWindows(t)
	b := &Battery{Tasks: []Task{
		{ID: "t", Command: "echo hello", ExpectContains: []string{"goodbye"}, TimeoutSec: 20},
	}}
	s, err := RunBatteryWithOptions(context.Background(), b, RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if s.Results[0].Success {
		t.Fatal("expected failure when expect_contains is not satisfied")
	}
	if !strings.Contains(s.Results[0].Error, "goodbye") {
		t.Errorf("error should name the missing substring: %s", s.Results[0].Error)
	}
}

func TestExpectContains_ShouldPassWhenAllPresent(t *testing.T) {
	skipOnWindows(t)
	b := &Battery{Tasks: []Task{
		{ID: "t", Command: "echo alpha; echo beta", ExpectContains: []string{"alpha", "beta"}, TimeoutSec: 20},
	}}
	s, _ := RunBatteryWithOptions(context.Background(), b, RunOptions{})
	if !s.Results[0].Success {
		t.Fatalf("expected pass, got %s", s.Results[0].Error)
	}
}

func TestExpectNotContains_ShouldFailWhenPresent(t *testing.T) {
	skipOnWindows(t)
	b := &Battery{Tasks: []Task{
		{ID: "t", Command: "echo FAILURE", ExpectNotContains: []string{"FAILURE"}, TimeoutSec: 20},
	}}
	s, _ := RunBatteryWithOptions(context.Background(), b, RunOptions{})
	if s.Results[0].Success {
		t.Fatal("expected failure when a forbidden substring appears")
	}
}

// TestExpectExit_ShouldAllowIntentionalNonZero is the case an exit-code-only
// harness cannot express: a task that is supposed to fail.
func TestExpectExit_ShouldAllowIntentionalNonZero(t *testing.T) {
	skipOnWindows(t)
	b := &Battery{Tasks: []Task{
		{ID: "t", Command: "exit 3", ExpectExit: intPtr(3), TimeoutSec: 20},
	}}
	s, _ := RunBatteryWithOptions(context.Background(), b, RunOptions{})
	if !s.Results[0].Success {
		t.Fatalf("expected pass for an expected non-zero exit, got %s", s.Results[0].Error)
	}
	if s.Results[0].ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", s.Results[0].ExitCode)
	}
}

func TestExpectExit_ShouldFailOnUnexpectedCode(t *testing.T) {
	skipOnWindows(t)
	b := &Battery{Tasks: []Task{
		{ID: "t", Command: "exit 1", TimeoutSec: 20},
	}}
	s, _ := RunBatteryWithOptions(context.Background(), b, RunOptions{})
	if s.Results[0].Success {
		t.Fatal("a non-zero exit with no expect_exit must fail")
	}
	if s.Results[0].ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", s.Results[0].ExitCode)
	}
}

// TestExitCodeZero_WithFailingOutput demonstrates why expect_contains exists:
// plenty of tools exit 0 while printing the failure.
func TestExitCodeZero_WithFailingOutput(t *testing.T) {
	skipOnWindows(t)
	b := &Battery{Tasks: []Task{
		{ID: "t", Command: "echo 'FAIL: 3 tests failed'; exit 0",
			ExpectNotContains: []string{"FAIL:"}, TimeoutSec: 20},
	}}
	s, _ := RunBatteryWithOptions(context.Background(), b, RunOptions{})
	if s.Results[0].Success {
		t.Fatal("expected the output assertion to catch a lying exit code")
	}
}

// =============================================================================
// Timeout, workdir, env
// =============================================================================

func TestRunTask_ShouldTimeOut(t *testing.T) {
	skipOnWindows(t)
	b := &Battery{Tasks: []Task{{ID: "slow", Command: "sleep 30", TimeoutSec: 1}}}

	start := time.Now()
	s, _ := RunBatteryWithOptions(context.Background(), b, RunOptions{})
	elapsed := time.Since(start)

	if s.Results[0].Success {
		t.Fatal("expected the task to fail on timeout")
	}
	if !s.Results[0].TimedOut {
		t.Errorf("TimedOut should be set, got %+v", s.Results[0])
	}
	if elapsed > 20*time.Second {
		t.Errorf("timeout was not enforced: took %s", elapsed)
	}
}

func TestRunTask_ShouldHonorWorkdir(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "marker.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	b := &Battery{Tasks: []Task{
		{ID: "ls", Command: "ls", ExpectContains: []string{"marker.txt"}, TimeoutSec: 20},
	}}
	s, _ := RunBatteryWithOptions(context.Background(), b, RunOptions{Workdir: dir})
	if !s.Results[0].Success {
		t.Fatalf("workdir not applied: %s (output %q)", s.Results[0].Error, s.Results[0].Output)
	}
}

func TestRunTask_ShouldInjectEnv(t *testing.T) {
	skipOnWindows(t)
	b := &Battery{Tasks: []Task{
		{ID: "env", Command: "echo $NERD_TEST_VAR", ExpectContains: []string{"sentinel"}, TimeoutSec: 20},
	}}
	s, _ := RunBatteryWithOptions(context.Background(), b, RunOptions{Env: []string{"NERD_TEST_VAR=sentinel"}})
	if !s.Results[0].Success {
		t.Fatalf("env not injected: %s", s.Results[0].Error)
	}
}

// =============================================================================
// Fail-fast vs continue
// =============================================================================

func TestContinueOnFailure_ShouldRunEveryTask(t *testing.T) {
	skipOnWindows(t)
	b := &Battery{Tasks: []Task{
		{ID: "a", Command: "exit 1", TimeoutSec: 20},
		{ID: "b", Command: "echo ok", TimeoutSec: 20},
		{ID: "c", Command: "echo ok", TimeoutSec: 20},
	}}

	s, _ := RunBatteryWithOptions(context.Background(), b, RunOptions{ContinueOnFailure: true})
	if s.Total != 3 {
		t.Fatalf("Total = %d, want 3", s.Total)
	}
	if s.Failed != 1 || s.Passed != 2 || s.Skipped != 0 {
		t.Errorf("summary = %+v; want 1 failed, 2 passed, 0 skipped", s)
	}
	if s.OK() {
		t.Error("OK() must be false when a task failed")
	}
}

func TestFailFast_ShouldSkipRemainder(t *testing.T) {
	skipOnWindows(t)
	b := &Battery{Tasks: []Task{
		{ID: "a", Command: "exit 1", TimeoutSec: 20},
		{ID: "b", Command: "echo ok", TimeoutSec: 20},
	}}
	s, _ := RunBatteryWithOptions(context.Background(), b, RunOptions{})
	if s.Failed != 1 || s.Skipped != 1 {
		t.Errorf("summary = %+v; want 1 failed, 1 skipped", s)
	}
}

func TestSummary_OK_ShouldRejectSkips(t *testing.T) {
	s := Summary{Total: 2, Passed: 1, Skipped: 1}
	if s.OK() {
		t.Error("a skipped task is an unanswered question, not a pass")
	}
}

// TestRunBattery_ShouldStopOnCancelledContext keeps a cancelled run from
// recording a cascade of spurious failures.
func TestRunBattery_ShouldStopOnCancelledContext(t *testing.T) {
	skipOnWindows(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	b := &Battery{Tasks: []Task{
		{ID: "a", Command: "echo one", TimeoutSec: 20},
		{ID: "b", Command: "echo two", TimeoutSec: 20},
	}}
	s, err := RunBatteryWithOptions(ctx, b, RunOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if s.Failed != 0 {
		t.Errorf("cancelled run recorded %d failures; want skips only", s.Failed)
	}
	if s.Skipped != 2 {
		t.Errorf("Skipped = %d, want 2", s.Skipped)
	}
}

// =============================================================================
// Persistence and reporting
// =============================================================================

func TestSaveRun_AndListRuns(t *testing.T) {
	ws := t.TempDir()
	summary := Summary{
		StartedAt: time.Date(2026, 8, 15, 10, 30, 0, 0, time.UTC),
		Total:     1, Passed: 1, DurationMs: 42,
		Results: []Result{{TaskID: "smoke", Success: true, DurationMs: 42}},
	}

	path, err := SaveRun(ws, summary)
	if err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var restored Summary
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatalf("run record does not round-trip: %v", err)
	}
	if restored.Passed != 1 || restored.Results[0].TaskID != "smoke" {
		t.Errorf("restored = %+v", restored)
	}

	runs, err := ListRuns(ws)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("ListRuns returned %d entries, want 1", len(runs))
	}
}

func TestListRuns_WhenNoDirectory_ShouldReturnEmpty(t *testing.T) {
	runs, err := ListRuns(t.TempDir())
	if err != nil {
		t.Fatalf("ListRuns should tolerate a missing directory: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("expected no runs, got %d", len(runs))
	}
}

func TestFormatSummary_ShouldReportEveryStatus(t *testing.T) {
	out := FormatSummary(Summary{
		Total: 3, Passed: 1, Failed: 1, Skipped: 1, DurationMs: 100,
		Results: []Result{
			{TaskID: "ok", Success: true, DurationMs: 10},
			{TaskID: "broken", Error: "expected exit code 0, got 1", DurationMs: 20},
			{TaskID: "later", Skipped: true, Error: "skipped after an earlier failure"},
		},
	})

	for _, want := range []string{"PASS", "FAIL", "SKIP", "ok", "broken", "later", "1 passed, 1 failed, 1 skipped"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
}

func TestFormatSummary_ShouldCollapseMultilineErrors(t *testing.T) {
	out := FormatSummary(Summary{
		Total: 1, Failed: 1,
		Results: []Result{{TaskID: "t", Error: "line one\nline two\nline three"}},
	})
	if strings.Contains(out, "line two") {
		t.Errorf("multi-line error broke table alignment:\n%s", out)
	}
}

func TestFormatSummary_WhenEmpty(t *testing.T) {
	if out := FormatSummary(Summary{}); !strings.Contains(out, "No tasks ran") {
		t.Errorf("unexpected output: %q", out)
	}
}

// =============================================================================
// Determinism
// =============================================================================

// TestRunShell_ShouldNotLoadProfileByDefault pins the determinism decision: a
// battery result must not depend on the operator's dotfiles.
func TestRunShell_ShouldNotLoadProfileByDefault(t *testing.T) {
	skipOnWindows(t)
	home := t.TempDir()
	// A profile that would corrupt output if it were sourced.
	for _, name := range []string{".bashrc", ".bash_profile", ".profile"} {
		if err := os.WriteFile(filepath.Join(home, name), []byte("echo PROFILE_LEAKED\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	b := &Battery{Tasks: []Task{
		{ID: "t", Command: "echo clean", ExpectNotContains: []string{"PROFILE_LEAKED"}, TimeoutSec: 20},
	}}
	s, _ := RunBatteryWithOptions(context.Background(), b, RunOptions{Env: []string{"HOME=" + home}})
	if !s.Results[0].Success {
		t.Errorf("shell profile leaked into a battery run: %s (output %q)",
			s.Results[0].Error, s.Results[0].Output)
	}
}

// =============================================================================
// Backward compatibility
// =============================================================================

func TestRunBattery_LegacySignature_ShouldStillWork(t *testing.T) {
	skipOnWindows(t)
	b := &Battery{Version: 1, Tasks: []Task{{ID: "smoke", Type: "shell", Command: "echo ok", TimeoutSec: 20}}}

	results, err := RunBattery(context.Background(), b, "")
	if err != nil {
		t.Fatalf("RunBattery: %v", err)
	}
	if len(results) != 1 || !results[0].Success {
		t.Fatalf("unexpected results: %+v", results)
	}
}
