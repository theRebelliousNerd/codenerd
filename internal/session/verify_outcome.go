package session

import (
	"context"
	"errors"
	"os/exec"
	"time"

	"codenerd/internal/processutil"
)

// VerifyOutcome is the authoritative verdict of one verification run.
//
// Ran/OK alone cannot tell the stories that matter here: a build that timed
// out after four minutes is not "not run", and it is not "failed" either —
// the compiler never rendered a verdict. Collapsing that case into Ran=false
// once let a timed-out repair recheck overwrite a real compiler failure and
// report success. Every gate branches on this outcome; Ran and OK stay as
// derived compatibility (Ran = the command executed, OK = passed).
type VerifyOutcome string

const (
	// VerifyPassed: the command ran to completion and reported success.
	VerifyPassed VerifyOutcome = "passed"
	// VerifyFailed: the command ran to completion and reported failure.
	VerifyFailed VerifyOutcome = "failed"
	// VerifySkipped: the command never ran because there was nothing to
	// verify (no workspace, no toolchain, no packages, no Go writes).
	VerifySkipped VerifyOutcome = "skipped"
	// VerifyIndeterminate: the command ran but produced no verdict — it
	// exhausted the verification budget. Not proof of broken code, and not
	// proof of recovery either.
	VerifyIndeterminate VerifyOutcome = "indeterminate"
	// VerifyCanceled: explicit operator cancellation aborted the run.
	VerifyCanceled VerifyOutcome = "canceled"
)

// Verification budgets. Zero: the harness puts no wall clock on building or
// testing the workspace. Variables (not constants) so tests can bound them;
// see the seam note below.
//
// They were four minutes each, which is shorter than the suite they gate --
// internal/session's own tests take 259 s, and the test gate runs the packages
// that import what a turn wrote. Ladder run R1-18 (2026-09-19) hit it on six
// importer packages and reported "tests ok" over a check that never finished.
// Steve, on the first of these clocks: "there should not be timeouts like
// that... some agentic runs are like hours long." A run's only wall clock is
// the one the user asks for with --timeout; an operator cancel still kills a
// verification at any point.
var (
	buildVerifyTimeout time.Duration
	testVerifyTimeout  time.Duration
)

// verifyCommandRunner executes one verification subprocess. The ctx carries
// the verification budget and doubles MUST honor it: block on ctx.Done()
// rather than sleeping the budget out, or timeout tests take minutes.
//
// Production runners kill the subprocess when ctx ends (exec.CommandContext
// semantics per platform). Test doubles substitute instant or scripted
// behavior. These are package-level seams rather than Executor fields because
// verifyBuild/verifyTests are free functions with eight production call
// sites; tests that swap them must restore via t.Cleanup and must not run in
// parallel with any other verification test.
type verifyCommandRunner func(ctx context.Context, dir string, env []string, name string, args []string) ([]byte, error)

var verifyBuildRunner verifyCommandRunner = func(ctx context.Context, dir string, env []string, name string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	return cmd.CombinedOutput()
}

var verifyTestRunner verifyCommandRunner = func(ctx context.Context, dir string, env []string, name string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	return processutil.NonInteractive(cmd).CombinedOutput()
}

// verifyLookPath resolves the Go toolchain. A seam so missing-toolchain
// tests do not depend on PATH surgery.
var verifyLookPath = exec.LookPath

type verifyRunResult struct {
	out []byte
	err error
}

// runVerificationCommand executes one verification subprocess under an
// independent budget while keeping operator cancellation effective.
//
// The budget context is rooted at Background, not at the caller's context: a
// model turn running out of time must not manufacture verification failures.
// Explicit operator cancellation still aborts the run — and kills the
// subprocess — through the supervisor below. A parent soft deadline (parent
// Done with DeadlineExceeded) is ignored: verification keeps its own budget.
// Only Canceled aborts.
//
// Returns the captured output (partial output survives timeouts and cancels),
// the outcome, and a human reason for non-pass outcomes.
func runVerificationCommand(parent context.Context, dir string, env []string, budget time.Duration, name string, args []string, runner verifyCommandRunner) (out []byte, outcome VerifyOutcome, reason string) {
	if err := parent.Err(); err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, VerifyCanceled, "verification canceled before it started"
		}
		// A pre-expired parent deadline does not apply: verification keeps
		// its own budget.
	}
	// A non-positive budget is unbounded, not expired: context.WithTimeout
	// with zero is already done, so "no budget" would otherwise mean "no time
	// at all" and every verification would come back indeterminate before its
	// command started.
	// The command runs in the spelling of the workspace every gate hands it
	// paths in (go_paths.go). On Windows a process keeps the spelling it was
	// started in, 8.3 short names included, so a short-named directory and a
	// long-named overlay key would be two files to go.
	dir = goWorkspace(dir)
	var budgetCtx context.Context
	var cancel context.CancelFunc
	if budget > 0 {
		budgetCtx, cancel = context.WithTimeout(context.Background(), budget)
	} else {
		budgetCtx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()
	done := make(chan verifyRunResult, 1)
	go func() {
		out, err := runner(budgetCtx, dir, env, name, args)
		done <- verifyRunResult{out: out, err: err}
	}()
	parentDone := parent.Done()
	var cancelWatch <-chan time.Time
	for {
		select {
		case r := <-done:
			if r.err == nil {
				return r.out, VerifyPassed, ""
			}
			if budgetCtx.Err() != nil {
				return r.out, VerifyIndeterminate, "verification exceeded its budget"
			}
			return r.out, VerifyFailed, r.err.Error()
		case <-parentDone:
			if errors.Is(parent.Err(), context.Canceled) {
				cancel() // kills the subprocess via budgetCtx
				r := <-done
				return r.out, VerifyCanceled, "operator canceled verification"
			}
			// Parent soft deadline: keep the independent budget, but arm a
			// slow re-check so an explicit cancel landing AFTER the deadline
			// still aborts the run. (parent.Done stays closed, so it cannot
			// simply be re-armed.)
			parentDone = nil
			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()
			cancelWatch = ticker.C
		case <-cancelWatch:
			if errors.Is(parent.Err(), context.Canceled) {
				cancel() // kills the subprocess via budgetCtx
				r := <-done
				return r.out, VerifyCanceled, "operator canceled verification"
			}
		}
	}
}
