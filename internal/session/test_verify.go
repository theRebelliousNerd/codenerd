package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"codenerd/internal/build"
	"codenerd/internal/logging"
	"codenerd/internal/testfacts"
)

// Post-edit test verification.
//
// The build gate (build_verify.go) closes one half of the verification gap: it
// proves the edits compile. Compiling is a low bar. A turn can write code that
// builds cleanly, was never executed once, and still be reported as complete —
// which is the same false success the build gate exists to prevent, one level
// up.
//
// This file supplies the other half: run the tests for exactly the packages the
// turn touched, and notice when a turn wrote production Go with no test
// alongside it. Both signals are structured the same way as BuildVerification
// so the turn can act on them rather than merely logging them.
//
// The Ran/OK split is load-bearing and matches the build gate: Ran=false means
// "unknown", never "pass". A verification that did not run must not be allowed
// to look like one that did.
//
// Scope note: untestedGoFiles is the pure same-turn predicate — "did this turn
// write a test next to the code it wrote". On its own it is far too eager to
// gate on, because editing a long-tested file without touching its test file
// looks identical to shipping untested code. untestedWithoutCoverageOnDisk
// narrows it to files with no test anywhere, and that is what the executor
// uses. Neither answers "is this new function covered" — that needs a coverage
// profile, not a filename comparison.
//
// Written by codeNERD on itself (2026-08-08), reviewed and corrected by hand.

// TestVerification is the outcome of running `go test` on the packages touched
// by a turn. Outcome is the authoritative verdict; Ran and OK stay as derived
// compatibility (Ran = the command executed, OK = passed). Gates must branch
// on Verdict, never on OK alone.
type TestVerification struct {
	// Ran is true when the verification command executed, even if it
	// produced no verdict (timeout, cancellation). It is false only when
	// nothing ran: skipped for lack of workspace, toolchain, or packages.
	Ran bool

	// OK is true only when the tests actually ran and passed.
	OK bool

	// Output is the run's Summary: failing tests with first failure
	// location and message, repeats as one line with a count, then a
	// tally. Never the raw stream and never a truncation of it; the
	// full per-test output stays recallable through Result. Empty on
	// success and on runs that produced no text (skips, pre-start
	// cancels).
	Output string

	// Result is the parsed `go test -json` stream: per-package and
	// per-test status, every test's full output, failures, build
	// failures, and repeats. Gates read their verdicts from it.
	// assertTurnTestFacts asserts Facts() keyed by the turn, and when a
	// working loop is active annotateFailingTestRecall names a
	// recall_context handle for each failing test's full output on the
	// summary line.
	Result *testfacts.Result

	// Duration is how long the test run took.
	Duration time.Duration

	// Outcome is the explicit verdict: passed, failed, skipped,
	// indeterminate (budget exhausted), or canceled.
	Outcome VerifyOutcome

	// Command is the argv executed, for provenance. Nil when nothing ran.
	Command []string

	// Reason explains a non-pass outcome without overloading Output.
	Reason string

	// Repair is the episode record when a repair loop ran for this gate.
	// Nil when the gate passed (or was skipped/canceled) without repair.
	Repair *RepairRecord

	// PreExistingFailures lists sanitized test names, subtests included
	// ("TestX/case_one"), that also fail without this turn's edits.
	// Set by attributeTestFailures. The gate does not read this list as a
	// verdict; the model-facing output names the same tests.
	PreExistingFailures []string

	// BaselineRan is true when the pre-turn run finished. BaselineFailures
	// keys every test that run failed, not only the ones the head also
	// failed. Nil keys and BaselineRan false means the baseline did not run,
	// so nothing is known to predate the turn.
	BaselineRan      bool
	BaselineFailures []TestKey

	// Selection is set by the importer gate: which importer tests it ran
	// and why, or why it ran them all. Nil on every other gate.
	Selection *TestSelection
}

// Verdict returns the authoritative outcome, deriving one for hand-built
// structs that predate the Outcome field.
func (v TestVerification) Verdict() VerifyOutcome {
	if v.Outcome != "" {
		return v.Outcome
	}
	switch {
	case v.Ran && v.OK:
		return VerifyPassed
	case v.Ran:
		return VerifyFailed
	default:
		return VerifySkipped
	}
}

// DeduplicatePreservingOrder removes duplicate strings while preserving the
// order of first occurrence. A nil input returns nil and the input slice is
// not mutated.
func DeduplicatePreservingOrder(in []string) []string {
	if in == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// packagesForPaths maps written .go file paths to their Go package directories,
// deduplicated. Non-Go files are skipped. The returned entries are the
// directory packages suitable for `go test` (e.g. "./internal/session").
//
// Paths are treated as workspace-relative; leading/trailing whitespace is
// ignored and the .go suffix check is case-insensitive. Paths like "internal/session/foo.go" become
// "./internal/session"; a file at the module root becomes ".".
func packagesForPaths(paths []string) []string {
	var out []string
	for _, p := range paths {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(trimmed), ".go") {
			continue
		}
		slash := filepath.ToSlash(trimmed)
		dir := filepath.ToSlash(filepath.Dir(slash))
		// Normalise dir: clean dot segments and duplicate slashes.
		dir = strings.TrimSpace(dir)
		if dir == "." || dir == "" {
			out = append(out, ".")
			continue
		}
		// Strip any leading "./" then re-add it so the result is an explicit
		// local pattern that `go test` accepts.
		dir = strings.TrimPrefix(dir, "./")
		dir = strings.TrimPrefix(dir, "/")
		// Clean interior: collapse empty and "." segments.
		parts := strings.Split(dir, "/")
		cleaned := make([]string, 0, len(parts))
		for _, part := range parts {
			if part == "" || part == "." {
				continue
			}
			cleaned = append(cleaned, part)
		}
		if len(cleaned) == 0 {
			out = append(out, ".")
			continue
		}
		pkg := "./" + strings.Join(cleaned, "/")
		out = append(out, pkg)
	}
	out = DeduplicatePreservingOrder(out)
	sort.Strings(out)
	return out
}

// untestedWithoutCoverageOnDisk narrows untestedGoFiles to the files that have
// no test anywhere — not merely no test written in this turn.
//
// untestedGoFiles alone is too eager to be an enforcement signal. It flagged
// internal/session/test_verify.go on two consecutive live turns (2026-08-08
// 11:08 and 11:13) because neither turn happened to rewrite
// test_verify_test.go — a file sitting right next to it with 40 passing
// subtests. A gate that cries wolf about tested code is one that gets ignored,
// and then switched off.
//
// A file counts as covered when either its own <base>_test.go exists on disk or
// its package contains any _test.go at all. The second clause is deliberate:
// Go's convention does not require one test file per source file, and demanding
// it would flag most of this repo.
func untestedWithoutCoverageOnDisk(workspace string, paths []string) []string {
	candidates := untestedGoFiles(paths)
	if len(candidates) == 0 || strings.TrimSpace(workspace) == "" {
		return candidates
	}

	var out []string
	for _, rel := range candidates {
		abs := rel
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(workspace, filepath.FromSlash(strings.TrimPrefix(filepath.ToSlash(rel), "./")))
		}

		sibling := strings.TrimSuffix(abs, filepath.Ext(abs)) + "_test.go"
		if _, err := os.Stat(sibling); err == nil {
			continue
		}
		if packageHasTestFile(filepath.Dir(abs)) {
			continue
		}
		out = append(out, rel)
	}
	return out
}

// packageHasTestFile reports whether dir contains any _test.go file.
func packageHasTestFile(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Unreadable directory is not evidence of missing tests. Fail toward
		// silence: a false "untested" claim is worse than a missed one.
		return true
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), "_test.go") {
			return true
		}
	}
	return false
}

// verifyTests runs `go test` on exactly the given packages and reports whether
// they pass.
//
// Uses build.GetBuildEnv so the verification inherits the same CGO_CFLAGS the
// project needs. A verification that fails for want of the build environment
// would send the agent chasing phantom failures.
// extraArgs are passed to `go test` before the package list, so a caller can
// ask for a coverage profile from the same invocation instead of paying for a
// second full test run.
func verifyTests(ctx context.Context, workspace string, packages []string, extraArgs ...string) TestVerification {
	return verifyTestRuns(ctx, workspace, "", testRun{Packages: packages, Args: extraArgs})
}

// testRun is one `go test` invocation: the packages it names and the flags
// that go before them. A gate that selects tests per package needs several,
// because -run applies to every package of an invocation.
type testRun struct {
	Packages []string
	Args     []string
}

// verifyTestRuns runs each testRun (concurrently when there are several) and
// reads one verdict from their combined `-json` streams, as if they were one
// invocation: a failure in any run is the gate's failure, a run that did not
// finish leaves the verdict unfinished unless another run failed, and the
// Result holds every run's packages. note is shown in the live progress
// beside the commands, for a gate that has something to say about what it
// chose to run.
func verifyTestRuns(ctx context.Context, workspace, note string, runs ...testRun) TestVerification {
	v := verifyTestRunsOnce(ctx, workspace, note, runs...)
	if v.Verdict() != VerifyFailed {
		return v
	}
	cause, ok := environmentBuildFailure(workspace, v.Result)
	if !ok {
		return v
	}
	// The toolchain or the build cache failed under the run, not the code:
	// a cache entry another build trimmed mid-run made a shared 247 GB cache
	// report crypto/ecdsa "not in std" in 22 packages, and a live campaign
	// reverted a correct edit for it (2026-11-07). Such a failure is usually
	// gone on the next run; run once more before any verdict.
	logging.Get(logging.CategorySession).Warn("test verification hit a Go toolchain or build cache failure (%s); running it once more", cause)
	again := verifyTestRunsOnce(ctx, workspace, note, runs...)
	if again.Verdict() != VerifyFailed {
		return again
	}
	if cause, ok := environmentBuildFailure(workspace, again.Result); ok {
		again.Outcome, again.OK = VerifyIndeterminate, false
		again.Reason = "the Go toolchain or build cache failed twice, not the code under test: " + cause
		logging.Get(logging.CategorySession).Warn("test verification indeterminate: %s", again.Reason)
	}
	return again
}

// environmentLoadErrors are the go command's messages for a toolchain or
// build cache it could not read, as opposed to code that does not compile.
var environmentLoadErrors = []string{
	"no such file or directory",
	"is not in std",
	"cannot find package",
	"could not import",
	"unexpected EOF",
}

// environmentBuildFailure reports whether a failed run failed only in the
// environment: no test failed, and every compiler diagnostic is located
// outside the workspace (the Go root, the module or build cache) with a
// loader or I/O message. A change in the workspace cannot cause that; a
// dependency bump that breaks a module's code reports type errors there and
// stays the change's failure. It returns the first diagnostic as the cause.
func environmentBuildFailure(workspace string, res *testfacts.Result) (string, bool) {
	if res == nil || len(res.Failures) != 0 || len(res.BuildFailures) == 0 {
		return "", false
	}
	workspace = goWorkspace(workspace)
	for _, bf := range res.BuildFailures {
		file := strings.TrimSpace(bf.File)
		if file == "" {
			return "", false
		}
		rel := workspaceFile(workspace, file)
		if !filepath.IsAbs(rel) && !strings.HasPrefix(rel, "../") {
			return "", false
		}
		loader := false
		for _, msg := range environmentLoadErrors {
			if strings.Contains(bf.Message, msg) {
				loader = true
				break
			}
		}
		if !loader {
			return "", false
		}
	}
	first := res.BuildFailures[0]
	return fmt.Sprintf("%s:%d: %s", first.File, first.Line, first.Message), true
}

// verifyTestRunsOnce is one measurement of runs; verifyTestRuns repeats it
// once when the toolchain, not the code, failed.
func verifyTestRunsOnce(ctx context.Context, workspace, note string, runs ...testRun) TestVerification {
	start := time.Now()

	if strings.TrimSpace(workspace) == "" {
		return TestVerification{Outcome: VerifySkipped, Reason: "empty workspace path", Duration: time.Since(start)}
	}
	var commands [][]string
	for _, r := range runs {
		filtered := make([]string, 0, len(r.Packages))
		for _, p := range r.Packages {
			if strings.TrimSpace(p) != "" {
				filtered = append(filtered, strings.TrimSpace(p))
			}
		}
		if len(filtered) == 0 {
			continue
		}
		// -json: the verdicts below are read from the parsed Result, not
		// scanned from human-readable text (testfacts_helpers.go).
		args := append([]string{"test", "-json"}, r.Args...)
		commands = append(commands, append([]string{"go"}, append(args, filtered...)...))
	}
	if len(commands) == 0 {
		return TestVerification{Outcome: VerifySkipped, Reason: "no packages to verify", Duration: time.Since(start)}
	}
	if _, err := verifyLookPath("go"); err != nil {
		logging.Get(logging.CategorySession).Warn(
			"test verification skipped: no Go toolchain on PATH (%v)", err)
		return TestVerification{Outcome: VerifySkipped, Reason: "no Go toolchain on PATH", Duration: time.Since(start)}
	}
	command := commands[0]

	type runOutcome struct {
		out     []byte
		outcome VerifyOutcome
		reason  string
	}
	env := build.GetBuildEnv(nil, workspace)
	outcomes := make([]runOutcome, len(commands))
	var wg sync.WaitGroup
	// One invocation per package of a selected gate is what `go test` does
	// across packages anyway: -p, by default one per core, builds, links and
	// runs that many at once. Fewer slots than that serialise the gate.
	slots := make(chan struct{}, min(len(commands), runtime.GOMAXPROCS(0)))
	for i, c := range commands {
		wg.Add(1)
		go func() {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			done := liveVerification(ctx, c, note)
			out, outcome, reason := runVerificationCommand(ctx, workspace, env, testVerifyTimeout, c[0], c[1:], verifyTestRunner)
			done(outcome)
			outcomes[i] = runOutcome{out, outcome, reason}
		}()
	}
	wg.Wait()
	var out []byte
	outcome, reason := VerifyPassed, ""
	for _, o := range outcomes {
		out = append(out, o.out...)
		outcome = combineVerifyOutcomes(outcome, o.outcome)
		if o.reason != "" && (reason == "" || o.outcome == VerifyFailed) {
			reason = o.reason
		}
	}
	elapsed := time.Since(start)
	res := parseTestJSON(workspace, out)

	var v TestVerification
	switch outcome {
	case VerifyPassed:
		logging.SessionDebug("test verification passed in %s", elapsed.Round(time.Millisecond))
		v = TestVerification{Ran: true, OK: true, Outcome: VerifyPassed, Command: command, Duration: elapsed, Result: res}
	case VerifyFailed:
		text := verificationOutput(res)
		if text == "" {
			text = reason
		}
		v = TestVerification{Ran: true, OK: false, Output: text, Outcome: VerifyFailed, Command: command, Reason: reason, Duration: elapsed, Result: res}
		annotateFailingTestRecall(ctx, &v)
		logging.Get(logging.CategorySession).Warn(
			"test verification FAILED in %s:\n%s", elapsed.Round(time.Millisecond), v.Output)
		return v
	case VerifyCanceled:
		logging.Get(logging.CategorySession).Warn("test verification canceled: %s", reason)
		v = TestVerification{Ran: len(out) > 0, Output: verificationOutput(res), Outcome: VerifyCanceled, Command: command, Reason: reason, Duration: elapsed, Result: res}
	default: // VerifyIndeterminate
		// A timeout is not evidence the tests are broken — but it is not
		// evidence of recovery either. Report it as indeterminate with
		// whatever the runner had produced, so gates retain what they knew
		// instead of minting a pass from silence.
		logging.Get(logging.CategorySession).Warn(
			"test verification timed out after %s; recovery not verified", testVerifyTimeout)
		v = TestVerification{Ran: true, Output: verificationOutput(res), Outcome: VerifyIndeterminate, Command: command, Reason: reason, Duration: elapsed, Result: res}
	}
	annotateFailingTestRecall(ctx, &v)
	return v
}

// combineVerifyOutcomes folds two runs' outcomes into the verdict of both: a
// failure is a measurement and wins, a run that did not finish beats a pass
// (a pass for half the packages is no pass), and a cancel beats a timeout.
func combineVerifyOutcomes(a, b VerifyOutcome) VerifyOutcome {
	rank := func(o VerifyOutcome) int {
		switch o {
		case VerifyFailed:
			return 3
		case VerifyCanceled:
			return 2
		case VerifyPassed:
			return 0
		default:
			return 1
		}
	}
	if rank(b) > rank(a) {
		return b
	}
	return a
}

// untestedGoFiles returns the subset of paths that are non-test .go files
// with no corresponding _test.go file written in the same turn.
//
// A file is considered "tested" in this turn when a file named
// <base>_test.go in the same directory was also written in this turn. For
// example, "internal/foo/bar.go" is considered tested if
// "internal/foo/bar_test.go" appears in paths. Non-Go files and _test.go
// files themselves are never returned.
func untestedGoFiles(paths []string) []string {
	// Build a set of normalized test file paths present in this turn.
	testSet := make(map[string]struct{})
	for _, p := range paths {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if !strings.HasSuffix(lower, ".go") {
			continue
		}
		if !strings.HasSuffix(lower, "_test.go") {
			continue
		}
		norm := strings.ToLower(filepath.ToSlash(strings.TrimSpace(trimmed)))
		norm = strings.TrimPrefix(norm, "./")
		testSet[norm] = struct{}{}
		testSet["./"+norm] = struct{}{}
	}

	seen := make(map[string]struct{})
	var out []string
	for _, p := range paths {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if !strings.HasSuffix(lower, ".go") {
			continue
		}
		if strings.HasSuffix(lower, "_test.go") {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(trimmed))
		base := filepath.Base(trimmed)
		ext := filepath.Ext(base)
		stem := strings.TrimSuffix(base, ext)
		expected := filepath.ToSlash(filepath.Join(dir, stem+"_test.go"))
		expectedNorm := strings.ToLower(strings.TrimPrefix(expected, "./"))
		altNorm := strings.ToLower(expected)
		if _, ok := testSet[expectedNorm]; ok {
			continue
		}
		if _, ok := testSet[altNorm]; ok {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}
