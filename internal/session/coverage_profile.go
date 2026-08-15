package session

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"codenerd/internal/build"
	"codenerd/internal/logging"
)

// Post-edit coverage verification.
//
// The build gate (build_verify.go) proves the edits compile. The test gate
// (test_verify.go) proves the tests pass. Passing is still a coarse signal.
// A turn can add a function, add a test file that does not call it, see
// `go test` go green, and still ship unexercised code — the same false
// success the test gate exists to prevent, one level deeper.
//
// This file supplies the finer signal: run `go test -coverprofile` on the
// packages the turn touched and report the uncovered blocks that belong to
// files the turn wrote. The signal is intentionally narrow — only blocks whose
// count is zero and whose file suffix matches a written file are surfaced, so
// uncovered code in untouched packages does not flag a turn that never edited
// it.
//
// Absence of a profile is "unknown", never "covered" — the same discipline as
// the Ran/OK split in the other two gates. Every not-run path here returns
// (nil, nil), which the caller must read as "no signal", not "nothing
// uncovered".
//
// Scope note: parseCoverProfile is the pure predicate — "is this block
// uncovered and does it belong to a file this turn wrote". uncoveredWrittenCode
// is the impure runner that produces the profile. Both are tested.
//
// Written by codeNERD on itself (2026-08-08). Reviewed by hand, and the review
// was not cosmetic: its original header claimed a split named
// "uncoveredWithoutCoverage" that does not exist, asserted uncoveredWrittenCode
// was "exercised through the integration path" when it had no test at all, and
// pre-signed itself as hand-reviewed before any review had happened. The code
// below was good; the claims about it were invented. That failure mode — a
// confident provenance note with nothing behind it — is the one this whole
// gate stack exists to make impossible, so it is recorded here rather than
// quietly deleted.

// coverVerifyTimeout bounds the `go test -coverprofile` run. Cold tests on this
// repo are slower than a build; the ceiling is generous because a verify that
// times out reports a false alarm, which is worse than a slow one.
const coverVerifyTimeout = 4 * time.Minute

// UncoveredBlock is a single uncovered block from a Go coverage profile that
// belongs to a file the turn wrote.
type UncoveredBlock struct {
	// File is the import-qualified path as it appears in the profile, e.g.
	// "codenerd/internal/session/foo.go".
	File string

	// StartLine is the first line of the block (1-indexed).
	StartLine int

	// EndLine is the last line of the block (1-indexed).
	EndLine int

	// NumStmts is the number of statements in the block.
	NumStmts int
}

// parseCoverProfile parses a Go coverage profile from r and returns only the
// blocks whose count is 0 and whose file path has a suffix matching one of
// writtenFiles.
//
// The profile format is the one `go test -coverprofile` writes:
//
//	mode: set
//	importpath/file.go:startLine.startCol,endLine.endCol numStmts count
//
// The first line must be a mode line ("mode: ..."). Each subsequent line is
// split into three fields: the file-range, the statement count, and the
// execution count. An error is returned on any malformed line, including a
// missing or malformed mode line.
//
// Suffix matching is deliberate: the profile records import-qualified paths
// (e.g. "codenerd/internal/session/foo.go") while writtenFiles are
// workspace-relative (e.g. "internal/session/foo.go"). A block is kept when
// its File ends with one of the writtenFiles entries after slash-normalisation.
func parseCoverProfile(r io.Reader, writtenFiles []string) ([]UncoveredBlock, error) {
	scanner := bufio.NewScanner(r)

	// The first line is the mode line.
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("reading coverage profile: %w", err)
		}
		return nil, fmt.Errorf("empty coverage profile: missing mode line")
	}
	modeLine := strings.TrimSpace(scanner.Text())
	if !strings.HasPrefix(modeLine, "mode:") {
		return nil, fmt.Errorf("malformed coverage profile: first line must be 'mode: <mode>', got %q", modeLine)
	}

	// Normalise writtenFiles for suffix comparison: trim space, slash-normalise,
	// strip leading "./".
	normalizedWritten := make([]string, 0, len(writtenFiles))
	for _, wf := range writtenFiles {
		wf = strings.TrimSpace(wf)
		if wf == "" {
			continue
		}
		wf = NormalizeCoverPath(wf)
		if wf == "" {
			continue
		}
		normalizedWritten = append(normalizedWritten, wf)
	}

	var out []UncoveredBlock
	lineNum := 1 // already consumed mode line
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, fmt.Errorf("malformed coverage line %d: expected 3 fields, got %d: %q", lineNum, len(fields), line)
		}
		fileAndRange := fields[0]
		numStmtsStr := fields[1]
		countStr := fields[2]

		colonIdx := strings.LastIndex(fileAndRange, ":")
		if colonIdx < 0 {
			return nil, fmt.Errorf("malformed coverage line %d: missing ':' in %q", lineNum, line)
		}
		file := fileAndRange[:colonIdx]
		rangePart := fileAndRange[colonIdx+1:]

		commaIdx := strings.Index(rangePart, ",")
		if commaIdx < 0 {
			return nil, fmt.Errorf("malformed coverage line %d: missing ',' in range %q", lineNum, rangePart)
		}
		startPart := rangePart[:commaIdx]
		endPart := rangePart[commaIdx+1:]

		startDot := strings.Index(startPart, ".")
		endDot := strings.Index(endPart, ".")
		if startDot < 0 || endDot < 0 {
			return nil, fmt.Errorf("malformed coverage line %d: missing '.' in range %q", lineNum, rangePart)
		}
		startLineStr := startPart[:startDot]
		endLineStr := endPart[:endDot]

		startLine, err := strconv.Atoi(startLineStr)
		if err != nil {
			return nil, fmt.Errorf("malformed coverage line %d: invalid start line %q: %w", lineNum, startLineStr, err)
		}
		endLine, err := strconv.Atoi(endLineStr)
		if err != nil {
			return nil, fmt.Errorf("malformed coverage line %d: invalid end line %q: %w", lineNum, endLineStr, err)
		}
		numStmts, err := strconv.Atoi(numStmtsStr)
		if err != nil {
			return nil, fmt.Errorf("malformed coverage line %d: invalid numStmts %q: %w", lineNum, numStmtsStr, err)
		}
		count, err := strconv.Atoi(countStr)
		if err != nil {
			return nil, fmt.Errorf("malformed coverage line %d: invalid count %q: %w", lineNum, countStr, err)
		}

		// Only uncovered blocks are surfaced.
		if count != 0 {
			continue
		}

		// Only blocks in files the turn wrote are surfaced.
		fileSlash := NormalizeCoverPath(file)
		matched := false
		for _, wf := range normalizedWritten {
			if strings.HasSuffix(fileSlash, wf) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		out = append(out, UncoveredBlock{
			File:      file,
			StartLine: startLine,
			EndLine:   endLine,
			NumStmts:  numStmts,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading coverage profile: %w", err)
	}
	return out, nil
}

// verifyTestsWithCoverage runs the packages' tests ONCE and returns both
// signals: whether they passed, and which blocks in the turn's own files were
// never executed.
//
// Two separate invocations would be the obvious composition — verifyTests for
// pass/fail, then uncoveredWrittenCode for coverage — and it would double the
// test time of every green write turn. Measured on internal/session that is
// ~9s becoming ~18s, paid on every edit. `go test -coverprofile` already
// reports pass/fail, so the second run buys nothing.
//
// Coverage is a secondary signal and must never turn a passing turn into a
// failing one on its own: if the profile cannot be produced or parsed, the test
// verdict still stands and the coverage list is simply empty.
func verifyTestsWithCoverage(
	ctx context.Context,
	workspace string,
	packages []string,
	writtenPaths []string,
) (TestVerification, []UncoveredBlock) {
	tmp, err := os.CreateTemp("", "coverprofile-*.out")
	if err != nil {
		// No profile is possible, but the tests still matter.
		logging.SessionDebug("could not create coverage profile (%v); running tests without it", err)
		return verifyTests(ctx, workspace, packages), nil
	}
	path := tmp.Name()
	tmp.Close()
	defer os.Remove(path)

	verification := verifyTests(ctx, workspace, packages,
		"-covermode=set", "-coverprofile="+path)
	if !verification.Ran {
		return verification, nil
	}

	f, err := os.Open(path)
	if err != nil {
		logging.SessionDebug("coverage profile unavailable (%v); test verdict stands", err)
		return verification, nil
	}
	defer f.Close()

	blocks, perr := parseCoverProfile(f, writtenPaths)
	if perr != nil {
		logging.Get(logging.CategorySession).Warn(
			"coverage profile could not be parsed (%v); test verdict stands", perr)
		return verification, nil
	}
	return verification, blocks
}

// uncoveredWrittenCode runs `go test -covermode=set -coverprofile=<temp file>`
// on the given packages and returns the uncovered blocks that belong to files
// in writtenPaths.
//
// Uses build.GetBuildEnv so the run inherits the same CGO_CFLAGS the project
// needs. A verification that fails for want of the build environment would send
// the agent chasing phantom failures.
//
// The temp profile file is removed before returning. An empty workspace or an
// empty package list returns (nil, nil) — there is nothing to verify and that
// is not an error, only "unknown" in the Ran/OK sense. A missing Go toolchain
// is also treated as not-run rather than as a hard failure, matching
// verifyTests.
func uncoveredWrittenCode(ctx context.Context, workspace string, packages []string, writtenPaths []string) ([]UncoveredBlock, error) {
	if strings.TrimSpace(workspace) == "" {
		return nil, nil
	}

	// Filter packages: drop empty/whitespace entries.
	filtered := make([]string, 0, len(packages))
	for _, p := range packages {
		if strings.TrimSpace(p) != "" {
			filtered = append(filtered, strings.TrimSpace(p))
		}
	}
	if len(filtered) == 0 {
		return nil, nil
	}

	if _, err := exec.LookPath("go"); err != nil {
		logging.Get(logging.CategorySession).Warn(
			"coverage verification skipped: no Go toolchain on PATH (%v)", err)
		return nil, nil
	}

	// Create a temp file for the cover profile. The file is created empty and
	// closed so `go test` can write to it on all platforms.
	tmpFile, err := os.CreateTemp("", "coverprofile-*.out")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp coverprofile: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	buildCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), coverVerifyTimeout)
	defer cancel()

	args := append([]string{"test", "-covermode=set", "-coverprofile=" + tmpPath}, filtered...)
	cmd := exec.CommandContext(buildCtx, "go", args...)
	cmd.Dir = workspace
	cmd.Env = build.GetBuildEnv(nil, workspace)

	out, err := cmd.CombinedOutput()
	if buildCtx.Err() != nil {
		logging.Get(logging.CategorySession).Warn(
			"coverage verification timed out after %s; treating as not run", coverVerifyTimeout)
		return nil, nil
	}
	if err != nil {
		// If the profile was not produced, surface the test output as the error.
		// When the profile exists despite a test failure we still parse it, so
		// check for the file before failing hard.
		if _, statErr := os.Stat(tmpPath); statErr != nil {
			text := strings.TrimSpace(string(out))
			if text == "" {
				text = err.Error()
			}
			return nil, fmt.Errorf("go test -coverprofile failed: %w: %s", err, text)
		}
		// Profile exists — fall through to parsing; the uncovered signal is still
		// useful even when tests fail.
	}

	f, err := os.Open(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open coverprofile %q: %w", tmpPath, err)
	}
	defer f.Close()

	blocks, err := parseCoverProfile(f, writtenPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to parse coverprofile: %w", err)
	}
	return blocks, nil
}

// summarizeUncovered renders uncovered blocks as a short "file:start-end" list
// for a log line, capped so a turn that rewrites a large file does not produce
// an unreadable warning.
func summarizeUncovered(blocks []UncoveredBlock) string {
	const maxListed = 8
	parts := make([]string, 0, maxListed+1)
	for i, b := range blocks {
		if i == maxListed {
			parts = append(parts, fmt.Sprintf("... and %d more", len(blocks)-maxListed))
			break
		}
		parts = append(parts, fmt.Sprintf("%s:%d-%d", filepath.Base(b.File), b.StartLine, b.EndLine))
	}
	return strings.Join(parts, ", ")
}

// NormalizeCoverPath puts a path in the form the profile comparison needs:
// forward slashes, no leading "./".
//
// Coverage profiles always use forward slashes regardless of platform, while
// written paths arrive however the tool that produced them spelled them. On
// Windows that difference alone would make every suffix comparison fail.
func NormalizeCoverPath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimPrefix(p, "./")
	return p
}
