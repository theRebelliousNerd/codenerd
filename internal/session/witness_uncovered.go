package session

import (
	"go/build"
	"os"
	"path/filepath"
	"strings"
)

// W6 narrows the file-level debt to what the element-level witness cannot
// judge. turn_uncovered used to name every file holding uncovered changed
// lines; most of those blocks sit inside changed elements that witness_owed
// now obligates and turn_unwitnessed now names. Asserting both for one block
// would be two verdict paths over the same evidence, so the file-level
// producer keeps only blocks outside every turn_changed_element span of
// their file: init bodies (a changed init asserts no ref -- turn_elements.go),
// regions outside any function (const, var, type), and files whose changed
// elements are unknown (no readable preimage or file).

// changedElementSpans is the line spans of the turn_changed_element refs
// assertTurnElements asserted for one written path, read the same way: the
// same skips, the same preimage, the same current file. A path it cannot
// read yields no spans, and every block there is then outside them --
// uncertain keeps the file-level debt rather than dropping evidence the
// witness never saw.
func changedElementSpans(workspace, path string, pre PreImage) []LineRange {
	if !strings.HasSuffix(strings.ToLower(path), ".go") || isTestPath(path) || ignoredByGoTool(path) {
		return nil
	}
	if !pre.Known() {
		return nil
	}
	disk := turnFilePath(workspace, path)
	if included, err := build.Default.MatchFile(filepath.Dir(disk), filepath.Base(disk)); err != nil || !included {
		return nil
	}
	data, err := os.ReadFile(disk)
	if err != nil {
		return nil
	}
	cur := string(data)
	spans := elementSpans(cur)
	var out []LineRange
	for _, ref := range changedElementRefs(cur, pre, path) {
		span, known := spans[ref]
		if !known {
			continue
		}
		out = append(out, span)
	}
	return out
}

// uncoveredOutsideChangedElements reports whether block lies outside every
// changed-element span of its file. Overlap is the same span test the
// element mapping uses (statementBlockInSpan): a block the run executed is
// never asked, because narrowToChangedLines dropped it before this.
func uncoveredOutsideChangedElements(block UncoveredBlock, spans []LineRange) bool {
	for _, span := range spans {
		if block.StartLine <= span.End && block.EndLine >= span.Start {
			return false
		}
	}
	return true
}
