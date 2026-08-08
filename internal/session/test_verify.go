package session

import (
	"context"
	"os/exec"
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
// Scope note: untestedGoFiles compares against files written in the SAME turn.
// It answers "did this turn ship code without a test", not "does this package
// have coverage" — a file edited today whose tests were written last week is
// not flagged, by design. Package-level coverage is a different measurement and
// belongs in a different function.
//
// Written by codeNERD on itself (2026-08-08), reviewed and corrected by hand.

// testVerifyTimeout bounds the verification `go test`. Cold tests on this repo
// are slower than a build; the ceiling is generous because a verify that times
// out reports a false alarm, which is worse than a slow one.
const testVerifyTimeout = 4 * time.Minute

// testVerifyMaxOutput caps how much test output is retained. A failing package
// can be verbose; a runaway dump would blow the context budget any repair
// logic needs.
const testVerifyMaxOutput = 6000

// TestVerification is the outcome of running `go test` on the packages touched
// by a turn.
type TestVerification struct {
	// Ran is false when verification was skipped (no Go packages touched, empty
	// workspace, no Go toolchain, verification disabled). A skipped verification
	// is NOT a pass and must never be reported as one.
	Ran bool

	// OK is true only when the tests actually ran and passed.
	OK bool

	// Output is the test command's combined stderr/stdout, truncated. Empty on
	// success.
	Output string

	// Duration is how long the test run took.
	Duration time.Duration
}

// packagesForPaths maps written .go file paths to their Go package directories,
// deduplicated. Non-Go files are skipped. The returned entries are the
// directory packages suitable for `go test` (e.g. "./internal/session").
//
// Paths are treated as workspace-relative; leading/trailing whitespace is
// ignored and the .go suffix check is case-insensitive to mirror
// touchedGoFiles. Paths like "internal/session/foo.go" become
// "./internal/session"; a file at the module root becomes ".".
func packagesForPaths(paths []string) []string {
	seen := make(map[string]struct{})
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
			if _, ok := seen["."]; !ok {
				seen["."] = struct{}{}
				out = append(out, ".")
			}
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
			if _, ok := seen["."]; !ok {
				seen["."] = struct{}{}
				out = append(out, ".")
			}
			continue
		}
		pkg := "./" + strings.Join(cleaned, "/")
		if _, ok := seen[pkg]; ok {
			continue
		}
		seen[pkg] = struct{}{}
		out = append(out, pkg)
	}
	sort.Strings(out)
	return out
}

// verifyTests runs `go test` on exactly the given packages and reports whether
// they pass.
//
// Uses build.GetBuildEnv so the verification inherits the same CGO_CFLAGS the
// project needs. A verification that fails for want of the build environment
// would send the agent chasing phantom failures.
func verifyTests(ctx context.Context, workspace string, packages []string) TestVerification {
	start := time.Now()

	if strings.TrimSpace(workspace) == "" {
		return TestVerification{Ran: false}
	}
	if len(packages) == 0 {
		return TestVerification{Ran: false}
	}
	filtered := make([]string, 0, len(packages))
	for _, p := range packages {
		if strings.TrimSpace(p) != "" {
			filtered = append(filtered, strings.TrimSpace(p))
		}
	}
	if len(filtered) == 0 {
		return TestVerification{Ran: false}
	}
	if _, err := exec.LookPath("go"); err != nil {
		logging.Get(logging.CategorySession).Warn(
			"test verification skipped: no Go toolchain on PATH (%v)", err)
		return TestVerification{Ran: false}
	}

	buildCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), testVerifyTimeout)
	defer cancel()

	args := append([]string{"test"}, filtered...)
	cmd := exec.CommandContext(buildCtx, "go", args...)
	cmd.Dir = workspace
	cmd.Env = build.GetBuildEnv(nil, workspace)

	out, err := cmd.CombinedOutput()
	elapsed := time.Since(start)

	if err == nil {
		logging.SessionDebug("test verification passed in %s", elapsed.Round(time.Millisecond))
		return TestVerification{Ran: true, OK: true, Duration: elapsed}
	}

	if buildCtx.Err() != nil {
		logging.Get(logging.CategorySession).Warn(
			"test verification timed out after %s; treating as not run", testVerifyTimeout)
		return TestVerification{Ran: false, Duration: elapsed}
	}

	text := strings.TrimSpace(string(out))
	if text == "" {
		text = err.Error()
	}
	if len(text) > testVerifyMaxOutput {
		text = text[:testVerifyMaxOutput] + "\n... (test output truncated)"
	}

	logging.Get(logging.CategorySession).Warn(
		"test verification FAILED in %s:\n%s", elapsed.Round(time.Millisecond), text)

	return TestVerification{Ran: true, OK: false, Output: text, Duration: elapsed}
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
