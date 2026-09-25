package session

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"codenerd/internal/build"
	"codenerd/internal/logging"
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

	// Output is the test command's combined stderr/stdout, truncated. Empty on
	// success and on runs that produced no text (skips, pre-start cancels).
	Output string

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

	// PreExistingFailures lists top-level test names that also fail without
	// this turn's edits (baseline overlay run). Set by attributeTestFailures.
	PreExistingFailures []string
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
	start := time.Now()

	if strings.TrimSpace(workspace) == "" {
		return TestVerification{Outcome: VerifySkipped, Reason: "empty workspace path", Duration: time.Since(start)}
	}
	if len(packages) == 0 {
		return TestVerification{Outcome: VerifySkipped, Reason: "no packages to verify", Duration: time.Since(start)}
	}
	filtered := make([]string, 0, len(packages))
	for _, p := range packages {
		if strings.TrimSpace(p) != "" {
			filtered = append(filtered, strings.TrimSpace(p))
		}
	}
	if len(filtered) == 0 {
		return TestVerification{Outcome: VerifySkipped, Reason: "no packages to verify", Duration: time.Since(start)}
	}
	if _, err := verifyLookPath("go"); err != nil {
		logging.Get(logging.CategorySession).Warn(
			"test verification skipped: no Go toolchain on PATH (%v)", err)
		return TestVerification{Outcome: VerifySkipped, Reason: "no Go toolchain on PATH", Duration: time.Since(start)}
	}

	args := append([]string{"test"}, extraArgs...)
	args = append(args, filtered...)
	command := append([]string{"go"}, args...)

	out, outcome, reason := runVerificationCommand(ctx, workspace, build.GetBuildEnv(nil, workspace), testVerifyTimeout, command[0], command[1:], verifyTestRunner)
	elapsed := time.Since(start)

	switch outcome {
	case VerifyPassed:
		logging.SessionDebug("test verification passed in %s", elapsed.Round(time.Millisecond))
		return TestVerification{Ran: true, OK: true, Outcome: VerifyPassed, Command: command, Duration: elapsed}
	case VerifyFailed:
		text := strings.TrimSpace(string(out))
		if text == "" {
			text = reason
		}
		// The test output goes back whole; the failure that matters is usually
		// the last thing printed, which a head cut dropped first.
		logging.Get(logging.CategorySession).Warn(
			"test verification FAILED in %s:\n%s", elapsed.Round(time.Millisecond), text)
		return TestVerification{Ran: true, OK: false, Output: text, Outcome: VerifyFailed, Command: command, Reason: reason, Duration: elapsed}
	case VerifyCanceled:
		logging.Get(logging.CategorySession).Warn("test verification canceled: %s", reason)
		return TestVerification{Ran: len(out) > 0, Output: strings.TrimSpace(string(out)), Outcome: VerifyCanceled, Command: command, Reason: reason, Duration: elapsed}
	default: // VerifyIndeterminate
		// A timeout is not evidence the tests are broken — but it is not
		// evidence of recovery either. Report it as indeterminate with
		// whatever the runner had printed, so gates retain what they knew
		// instead of minting a pass from silence.
		logging.Get(logging.CategorySession).Warn(
			"test verification timed out after %s; recovery not verified", testVerifyTimeout)
		return TestVerification{Ran: true, Output: strings.TrimSpace(string(out)), Outcome: VerifyIndeterminate, Command: command, Reason: reason, Duration: elapsed}
	}
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
