package session

import (
	"os"
	"sort"
	"strings"
)

// The coverage profile is one aggregate for the whole `go test` run. It can
// say which statement blocks of a changed element executed. It cannot say
// which test executed them, so nothing here asserts a per-test row.
//
// turn_element_uncovered names a turn_changed_element whose span holds at
// least one statement block and none of those blocks executed. A function
// with one branch executed and another missed is not on the list: the run
// did execute it. A function the profile does not mention — the file was
// not compiled, or the body is empty — is not on it either: no block is not
// evidence that a block was missed.
//
// turn_element_measured is the positive half of the same walk: a changed
// element whose span holds at least one statement block, executed or not.
// witness.mg reads both (witness_executed); the file-level turn_uncovered is
// the verdict's debt only for blocks outside every changed element.

// elementUncoveredRefs is the changed elements of this turn whose statement
// blocks the profile shows and the run never executed. refs match
// changedElementRefs, so the fact joins turn_changed_element on the ref.
//
// It also records ElementMeasured from the same read of each file. The
// caller keeps only the unexecuted blocks after this returns
// (narrowToChangedLines), so a later walk of result.UncoveredBlocks cannot
// see a block the run did execute and would call a measured element
// unmeasured. One read, one span parse.
func elementUncoveredRefs(workspace string, result *ExecutionResult, blocks []UncoveredBlock) []string {
	if result == nil {
		return nil
	}
	if len(blocks) == 0 {
		result.ElementMeasured = nil
		return nil
	}
	seenPath := make(map[string]bool, len(result.WrittenPaths))
	seenRef := make(map[string]bool)
	var uncovered, measured []string
	for _, path := range result.WrittenPaths {
		// The same paths assertTurnElements skips: a test of a test, and a
		// file the go tool never builds, have nothing the profile ran.
		if seenPath[path] || !strings.HasSuffix(strings.ToLower(path), ".go") || isTestPath(path) || ignoredByGoTool(path) {
			continue
		}
		seenPath[path] = true
		pre, ok := preImageFor(workspace, path, result.PreWriteContents)
		if !ok || !pre.Known() {
			continue
		}
		data, err := os.ReadFile(turnFilePath(workspace, path))
		if err != nil {
			continue
		}
		cur := string(data)
		spans := elementSpans(cur)
		fileBlocks := blocksForWritten(blocks, path)
		for _, ref := range changedElementRefs(cur, pre, path) {
			span, known := spans[ref]
			if !known || seenRef[ref] {
				continue
			}
			seenRef[ref] = true
			if elementMeasured(span, fileBlocks) {
				measured = append(measured, ref)
			}
			if elementUncovered(span, fileBlocks) {
				uncovered = append(uncovered, ref)
			}
		}
	}
	sort.Strings(measured)
	sort.Strings(uncovered)
	if len(measured) == 0 {
		result.ElementMeasured = nil
	} else {
		result.ElementMeasured = measured
	}
	return uncovered
}

// blocksForWritten keeps the profile blocks for one workspace-relative path.
// The profile spells the import path with forward slashes; written paths
// arrive however the tool recorded them, including Windows separators.
func blocksForWritten(blocks []UncoveredBlock, written string) []UncoveredBlock {
	w := NormalizeCoverPath(written)
	if w == "" {
		return nil
	}
	var out []UncoveredBlock
	for _, b := range blocks {
		if strings.HasSuffix(NormalizeCoverPath(b.File), w) {
			out = append(out, b)
		}
	}
	return out
}

// statementBlockInSpan reports a profile block that is a statement and
// overlaps the element's lines. A zero-statement block is an empty body;
// it is neither measured nor missed.
func statementBlockInSpan(b UncoveredBlock, span LineRange) bool {
	return b.NumStmts > 0 && b.StartLine <= span.End && b.EndLine >= span.Start
}

// elementMeasured reports whether the profile holds at least one statement
// block in span. The execution count does not matter: a block the run saw
// and did not execute is still a block the run measured.
func elementMeasured(span LineRange, blocks []UncoveredBlock) bool {
	for _, b := range blocks {
		if statementBlockInSpan(b, span) {
			return true
		}
	}
	return false
}

// elementUncovered reports whether span's statement blocks all went unexecuted.
// One executed block discharges the element. Zero statement blocks do not
// count: an empty body was not missed.
func elementUncovered(span LineRange, blocks []UncoveredBlock) bool {
	stmts := 0
	for _, b := range blocks {
		if !statementBlockInSpan(b, span) {
			continue
		}
		if b.Count > 0 {
			return false
		}
		stmts++
	}
	return stmts >= 1
}
