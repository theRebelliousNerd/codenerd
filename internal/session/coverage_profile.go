package session

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"codenerd/internal/logging"

	"github.com/sergi/go-diff/diffmatchpatch"
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
// uncovered and does it belong to a file this turn wrote".
// verifyTestsWithCoverage is the impure runner that produces the profile in
// the same invocation that reports pass/fail. Both are tested.
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

		// Only uncovered blocks are surfaced, and only ones with a statement a
		// test could execute: an empty body (`func main() {}`) is a block of
		// zero statements, and reporting it asked for a test of nothing.
		if count != 0 || numStmts == 0 {
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

// LineRange is a 1-based inclusive span of lines.
type LineRange struct {
	Start, End int
}

func changedLines(before, after string) []LineRange {
	dmp := diffmatchpatch.New()
	a, b, lineArray := dmp.DiffLinesToChars(before, after)
	diffs := dmp.DiffMain(a, b, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)

	countLines := func(s string) int {
		if s == "" {
			return 0
		}
		n := strings.Count(s, "\n")
		if !strings.HasSuffix(s, "\n") {
			n++
		}
		return n
	}

	var out []LineRange
	line := 1
	for _, d := range diffs {
		switch d.Type {
		case diffmatchpatch.DiffDelete:
			// Deletes exist only in the before text; the after-text line counter does not advance.
		case diffmatchpatch.DiffEqual:
			line += countLines(d.Text)
		case diffmatchpatch.DiffInsert:
			n := countLines(d.Text)
			if n == 0 {
				continue
			}
			start := line
			end := line + n - 1
			line += n
			if len(out) > 0 && start == out[len(out)-1].End+1 {
				out[len(out)-1].End = end
			} else {
				out = append(out, LineRange{Start: start, End: end})
			}
		}
	}
	return out
}

func blocksInChangedLines(blocks []UncoveredBlock, changed map[string][]LineRange) []UncoveredBlock {
	var out []UncoveredBlock
	for _, b := range blocks {
		fileSlash := NormalizeCoverPath(b.File)
		matched := false
		overlap := false
		for k, ranges := range changed {
			nk := NormalizeCoverPath(k)
			if nk == "" {
				continue
			}
			if !strings.HasSuffix(fileSlash, nk) {
				continue
			}
			matched = true
			for _, r := range ranges {
				if r.Start <= b.EndLine && r.End >= b.StartLine {
					overlap = true
					break
				}
			}
			if overlap {
				break
			}
		}
		if !matched || overlap {
			out = append(out, b)
		}
	}
	return out
}
