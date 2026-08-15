// Package regression provides a lightweight regression battery harness.
//
// Batteries are YAML-defined shell task suites used to continuously evaluate
// agent behavior. The one production consumer is `nerd regression run`
// (cmd/nerd/cmd_regression.go). No campaign or gauntlet stage calls this
// package today — the previous package comment claimed batteries "can be run
// as part of Nemesis gauntlets" when nothing referenced the package at all.
//
// Determinism note: battery shells are started with the interactive profile
// disabled (bash --noprofile --norc, powershell -NoProfile). A regression suite
// whose result depends on the operator's dotfiles is not a regression suite.
// Set RunOptions.LoginShell to opt back into a profile-loading shell.
//
// # Runtime dependency
//
// This package shells out. It has exactly one runtime dependency beyond the Go
// standard library, and it is not vendorable:
//
//	Unix / macOS   bash, on PATH. Invoked as `bash --noprofile --norc`, or
//	               `bash -l` when RunOptions.LoginShell is set. sh is NOT a
//	               substitute — the seeded battery and most real suites use
//	               bashisms, and --noprofile/--norc are bash spellings.
//	Windows        powershell (Windows PowerShell 5.x, present on every
//	               supported Windows). Invoked as
//	               `powershell -NoProfile -NonInteractive -Command -`, with the
//	               command piped on stdin. pwsh (PowerShell 7) is not looked up.
//
// Task commands themselves may depend on anything else — `go`, `git`, a test
// runner — but that is the battery author's contract, not this package's.
// RunBatteryWithOptions preflights the interpreter with RequiredShell and
// CheckShell and fails the run with an actionable error rather than reporting
// every task as failed, which is what a bare exec.LookPath failure looked like:
// N identical "executable file not found in $PATH" task errors and no statement
// of the actual cause.
package regression

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"codenerd/internal/logging"

	"gopkg.in/yaml.v3"
)

// SupportedVersion is the battery schema version this package understands.
const SupportedVersion = 1

// defaultTaskTimeout bounds a task that declares no timeout_sec.
const defaultTaskTimeout = 5 * time.Minute

// killGraceDelay bounds how long Wait may block on output pipes still held by
// grandchildren after the shell has been killed.
const killGraceDelay = 2 * time.Second

// Battery is a collection of regression tasks.
type Battery struct {
	Version int    `yaml:"version" json:"version"`
	Tasks   []Task `yaml:"tasks" json:"tasks"`
}

// Task is a single regression task.
// Currently supported: type=shell.
type Task struct {
	ID         string `yaml:"id" json:"id"`
	Type       string `yaml:"type" json:"type"` // "shell"
	Command    string `yaml:"command" json:"command"`
	TimeoutSec int    `yaml:"timeout_sec,omitempty" json:"timeout_sec,omitempty"`

	// ExpectExit asserts the process exit code. Nil means "zero is success",
	// which is the historical behavior. Use 0 explicitly for clarity, or a
	// non-zero value for a task that is supposed to fail.
	ExpectExit *int `yaml:"expect_exit,omitempty" json:"expect_exit,omitempty"`

	// ExpectContains asserts every listed substring appears in combined output.
	// An exit code alone is a weak assertion: plenty of tools exit 0 while
	// printing the failure you care about.
	ExpectContains []string `yaml:"expect_contains,omitempty" json:"expect_contains,omitempty"`

	// ExpectNotContains asserts none of the listed substrings appear.
	ExpectNotContains []string `yaml:"expect_not_contains,omitempty" json:"expect_not_contains,omitempty"`
}

// Result captures execution outcome for a task.
type Result struct {
	TaskID     string `json:"task_id"`
	Success    bool   `json:"success"`
	Output     string `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
	ExitCode   int    `json:"exit_code"`
	TimedOut   bool   `json:"timed_out,omitempty"`
	Skipped    bool   `json:"skipped,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// RunOptions tunes a battery run. The zero value preserves the historical
// behavior: fail fast, no profile-loading shell.
type RunOptions struct {
	// Workdir is the subprocess working directory when non-empty.
	Workdir string

	// ContinueOnFailure runs every task even after one fails. The default
	// (false) stops at the first failure to keep gauntlet latency bounded.
	ContinueOnFailure bool

	// LoginShell restores profile-loading shells (bash -l). Off by default so
	// results do not depend on operator dotfiles.
	LoginShell bool

	// Env supplies additional environment entries as "KEY=VALUE".
	Env []string
}

// Summary aggregates a run for reporting and persistence.
type Summary struct {
	StartedAt  time.Time `json:"started_at"`
	DurationMs int64     `json:"duration_ms"`
	Total      int       `json:"total"`
	Passed     int       `json:"passed"`
	Failed     int       `json:"failed"`
	Skipped    int       `json:"skipped"`
	Results    []Result  `json:"results"`
}

// OK reports whether the run is a pass: nothing failed and nothing was skipped.
// Skips count against it because a skipped task is an unanswered question, not
// a green one.
func (s Summary) OK() bool { return s.Failed == 0 && s.Skipped == 0 }

// Validate checks a battery is well-formed and runnable.
//
// Empty-suite policy: an empty battery is a configuration error, not a vacuous
// pass. A suite that silently passes because it has no tasks is the worst
// possible regression signal — it reports green precisely when it is checking
// nothing. Programmatic callers of RunBattery still get (nil, nil) for an empty
// battery; the error is raised where a human authored the file.
func (b *Battery) Validate() error {
	if b == nil {
		return fmt.Errorf("battery is nil")
	}
	if b.Version != 0 && b.Version != SupportedVersion {
		return fmt.Errorf("unsupported battery version %d (this build understands %d)", b.Version, SupportedVersion)
	}
	if len(b.Tasks) == 0 {
		return fmt.Errorf("battery declares no tasks; an empty suite would report success while testing nothing")
	}

	seen := make(map[string]int, len(b.Tasks))
	for i, task := range b.Tasks {
		if strings.TrimSpace(task.ID) == "" {
			return fmt.Errorf("task %d has no id", i)
		}
		if prev, dup := seen[task.ID]; dup {
			return fmt.Errorf("task %d duplicates the id %q already used by task %d", i, task.ID, prev)
		}
		seen[task.ID] = i

		typ := normalizeType(task.Type)
		if typ != "shell" {
			return fmt.Errorf("task %q has unsupported type %q (only \"shell\" is supported)", task.ID, task.Type)
		}
		if strings.TrimSpace(task.Command) == "" {
			return fmt.Errorf("task %q has an empty command", task.ID)
		}
		if task.TimeoutSec < 0 {
			return fmt.Errorf("task %q has a negative timeout_sec (%d)", task.ID, task.TimeoutSec)
		}
	}
	return nil
}

func normalizeType(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	if t == "" {
		return "shell"
	}
	return t
}

// LoadBattery reads and validates a YAML battery file from disk.
func LoadBattery(path string) (*Battery, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var b Battery
	if err := yaml.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("failed to parse battery YAML: %w", err)
	}
	if err := b.Validate(); err != nil {
		return nil, fmt.Errorf("invalid battery %s: %w", path, err)
	}
	return &b, nil
}

// RunBattery executes all tasks in order using the local shell.
// workdir is used as the subprocess working directory when non-empty.
func RunBattery(ctx context.Context, b *Battery, workdir string) ([]Result, error) {
	summary, err := RunBatteryWithOptions(ctx, b, RunOptions{Workdir: workdir})
	if err != nil {
		return nil, err
	}
	if summary.Total == 0 {
		return nil, nil
	}
	return summary.Results, nil
}

// RequiredShell names the interpreter every shell task is executed with on
// this platform. Exported so a host can state the dependency in a preflight or
// a doctor command instead of discovering it one failed task at a time.
func RequiredShell() string {
	if runtime.GOOS == "windows" {
		return "powershell"
	}
	return "bash"
}

// CheckShell reports whether the interpreter this platform needs is on PATH.
// A missing shell is an environment failure, not a suite failure: without it
// not one task can run, so the distinction matters to whoever reads the result.
func CheckShell() error {
	shell := RequiredShell()
	if _, err := exec.LookPath(shell); err != nil {
		return fmt.Errorf("regression batteries require %q on PATH (see package docs for the runtime dependency): %w", shell, err)
	}
	return nil
}

// RunBatteryWithOptions executes a battery and returns an aggregated Summary.
func RunBatteryWithOptions(ctx context.Context, b *Battery, opts RunOptions) (Summary, error) {
	summary := Summary{StartedAt: time.Now().UTC()}
	if b == nil || len(b.Tasks) == 0 {
		return summary, nil
	}

	// Preflight before spending a single task: a missing interpreter would
	// otherwise surface as N identical exec failures with the real cause buried.
	if err := CheckShell(); err != nil {
		return summary, err
	}

	log := logging.Get(logging.CategoryRegression)
	log.Info("regression: running battery with %d task(s) in %q", len(b.Tasks), opts.Workdir)

	runStart := time.Now()
	failed := false

	for _, task := range b.Tasks {
		if failed && !opts.ContinueOnFailure {
			summary.Results = append(summary.Results, Result{
				TaskID:  task.ID,
				Skipped: true,
				Error:   "skipped after an earlier failure",
			})
			continue
		}
		// A cancelled parent context must stop the run rather than record a
		// cascade of spurious failures.
		if err := ctx.Err(); err != nil {
			summary.Results = append(summary.Results, Result{
				TaskID:  task.ID,
				Skipped: true,
				Error:   fmt.Sprintf("skipped: %v", err),
			})
			continue
		}

		res := runTask(ctx, task, opts)
		summary.Results = append(summary.Results, res)

		if res.Success {
			log.Debug("regression: task %q passed in %dms", res.TaskID, res.DurationMs)
		} else {
			failed = true
			log.Warn("regression: task %q failed (exit=%d timed_out=%v): %s",
				res.TaskID, res.ExitCode, res.TimedOut, res.Error)
		}
	}

	summary.DurationMs = time.Since(runStart).Milliseconds()
	for _, r := range summary.Results {
		switch {
		case r.Skipped:
			summary.Skipped++
		case r.Success:
			summary.Passed++
		default:
			summary.Failed++
		}
	}
	summary.Total = len(summary.Results)

	log.Info("regression: battery finished — %d passed, %d failed, %d skipped in %dms",
		summary.Passed, summary.Failed, summary.Skipped, summary.DurationMs)

	return summary, nil
}

// runTask executes one task and applies its expectations.
func runTask(ctx context.Context, task Task, opts RunOptions) Result {
	start := time.Now()
	res := Result{TaskID: task.ID}

	if normalizeType(task.Type) != "shell" {
		res.Error = fmt.Sprintf("unsupported task type: %s", task.Type)
		res.DurationMs = time.Since(start).Milliseconds()
		return res
	}

	timeout := time.Duration(task.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = defaultTaskTimeout
	}
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	out, exitCode, err := runShell(tctx, task.Command, opts)
	res.Output = out
	res.ExitCode = exitCode
	res.TimedOut = tctx.Err() != nil && ctx.Err() == nil

	switch {
	case res.TimedOut:
		res.Error = fmt.Sprintf("timed out after %s", timeout)
	case err != nil && exitCode < 0:
		// The process could not be started at all (missing shell, bad workdir).
		res.Error = err.Error()
	default:
		res.Error = evaluateExpectations(task, out, exitCode)
	}
	res.Success = res.Error == ""
	res.DurationMs = time.Since(start).Milliseconds()
	return res
}

// evaluateExpectations returns "" when the task met every declared assertion,
// or a description of the first violation.
func evaluateExpectations(task Task, output string, exitCode int) string {
	wantExit := 0
	if task.ExpectExit != nil {
		wantExit = *task.ExpectExit
	}
	if exitCode != wantExit {
		return fmt.Sprintf("expected exit code %d, got %d", wantExit, exitCode)
	}
	for _, want := range task.ExpectContains {
		if !strings.Contains(output, want) {
			return fmt.Sprintf("expected output to contain %q", want)
		}
	}
	for _, unwanted := range task.ExpectNotContains {
		if strings.Contains(output, unwanted) {
			return fmt.Sprintf("expected output not to contain %q", unwanted)
		}
	}
	return ""
}

// runShell executes command and returns combined output and the exit code.
// An exit code of -1 means the process could not be started or was killed by
// a signal, which callers must distinguish from an ordinary non-zero exit.
func runShell(ctx context.Context, command string, opts RunOptions) (string, int, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", -1, fmt.Errorf("empty command")
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// -NoProfile keeps a battery from picking up the operator's profile.
		cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", "-")
	} else if opts.LoginShell {
		cmd = exec.CommandContext(ctx, "bash", "-l")
	} else {
		cmd = exec.CommandContext(ctx, "bash", "--noprofile", "--norc")
	}

	cmd.Stdin = strings.NewReader(command)
	if opts.Workdir != "" {
		cmd.Dir = opts.Workdir
	}
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}

	// Killing the shell on context cancellation is not enough to unblock
	// CombinedOutput: any grandchild the command spawned (`sleep 30`, a test
	// runner, a server) inherits the output pipes and keeps them open, so Wait
	// hangs until the grandchild exits and the timeout is silently ignored.
	// WaitDelay bounds that, force-closing the pipes shortly after the kill.
	cmd.WaitDelay = killGraceDelay

	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0, nil
	}

	// An ordinary non-zero exit is data, not a harness failure: a task may
	// legitimately expect one via expect_exit.
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode(), nil
	}
	return string(out), -1, fmt.Errorf("command failed (%s): %w", command, err)
}

// DefaultBatteryPath returns the canonical battery path for a workspace.
func DefaultBatteryPath(workspace string) string {
	return filepath.Join(workspace, ".nerd", "regression", "battery.yaml")
}

// RunsDir returns the directory where run records are persisted.
func RunsDir(workspace string) string {
	return filepath.Join(workspace, ".nerd", "regression", "runs")
}

// SaveRun persists a summary under .nerd/regression/runs/ and returns its path.
// Failure to persist is reported but never invalidates the run itself.
func SaveRun(workspace string, summary Summary) (string, error) {
	dir := RunsDir(workspace)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create runs dir: %w", err)
	}

	name := fmt.Sprintf("%s.json", summary.StartedAt.Format("20060102-150405"))
	path := filepath.Join(dir, name)

	payload, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal summary: %w", err)
	}
	if err := os.WriteFile(path, payload, 0644); err != nil {
		return "", fmt.Errorf("write run record: %w", err)
	}
	return path, nil
}

// FormatSummary renders a run as an aligned operator-facing table.
func FormatSummary(summary Summary) string {
	if summary.Total == 0 {
		return "No tasks ran.\n"
	}

	widest := len("TASK")
	for _, r := range summary.Results {
		if len(r.TaskID) > widest {
			widest = len(r.TaskID)
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%-*s  %-7s  %10s  %s\n", widest, "TASK", "STATUS", "DURATION", "DETAIL")
	for _, r := range summary.Results {
		status := "PASS"
		switch {
		case r.Skipped:
			status = "SKIP"
		case !r.Success:
			status = "FAIL"
		}
		detail := r.Error
		if detail == "" && r.Success {
			detail = "-"
		}
		fmt.Fprintf(&sb, "%-*s  %-7s  %9dms  %s\n", widest, r.TaskID, status, r.DurationMs, firstLine(detail))
	}

	fmt.Fprintf(&sb, "\n%d passed, %d failed, %d skipped in %dms\n",
		summary.Passed, summary.Failed, summary.Skipped, summary.DurationMs)
	return sb.String()
}

// firstLine keeps a multi-line error from breaking table alignment.
func firstLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return s[:i] + " …"
	}
	return s
}

// ListRuns returns persisted run record paths, newest first.
func ListRuns(workspace string) ([]string, error) {
	dir := RunsDir(workspace)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		paths = append(paths, filepath.Join(dir, e.Name()))
	}
	// Filenames are timestamp-prefixed, so reverse lexical order is newest first.
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	return paths, nil
}
