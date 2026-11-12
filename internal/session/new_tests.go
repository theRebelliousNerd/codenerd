package session

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// A failing test the turn itself wrote is named as the turn's own.
//
// Live 2026-11-12: a self-repair fix turn wrote
// TestBatchEquivalencePreservesDerivedFacts, whose Mangle program did not
// even analyze ("missing arg atoms for arguments"). Every repair round told
// the model "your edits compile but the tests fail ... fix the code", the turn
// ended "edits broke the tests", and three rounds edited the production code
// under a test that could never pass. A new test that fails is evidence about
// the test as much as about the code: the round says which tests are new, and
// that a failure in the test's own setup is the test's to fix. The test stays
// either way -- a test the turn added is part of its deliverable.

// addedTests is every test function in the turn's written _test.go files
// that was not there before the turn, keyed by file directory and name. A
// file the turn created contributes all of its tests.
func addedTests(workspace string, written []string, pre map[string]PreImage) map[string]bool {
	out := map[string]bool{}
	for _, p := range written {
		if !strings.HasSuffix(p, "_test.go") {
			continue
		}
		img, ok := preImageFor(workspace, p, pre)
		if !ok || !img.Known() {
			continue
		}
		data, err := os.ReadFile(diskPath(workspace, p))
		if err != nil {
			continue
		}
		after, ok := parseTestNames(string(data))
		if !ok {
			continue
		}
		before := map[string]bool{}
		if img.Existed {
			if names, ok := parseTestNames(img.Content); ok {
				before = names
			}
		}
		dir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(p)))
		for name := range after {
			if !before[name] {
				out[dir+"|"+name] = true
			}
		}
	}
	return out
}

// newFailingTests splits failing tests (keys, the ones the turn has to clear)
// into the ones it wrote and the rest, by top-level name. A test is the turn's when a
// _test.go file the turn wrote, in a directory whose import path ends the
// failing package's, added it.
func newFailingTests(workspace string, keys []TestKey, written []string, pre map[string]PreImage) (added, other []string) {
	if len(keys) == 0 {
		return nil, nil
	}
	tests := addedTests(workspace, written, pre)
	seen := map[string]bool{}
	for _, k := range keys {
		top, _, _ := strings.Cut(k.Name, "/")
		label := k.Pkg + "." + top
		if seen[label] {
			continue
		}
		seen[label] = true
		if addedIn(tests, k.Pkg, top) {
			added = append(added, top)
		} else {
			other = append(other, label)
		}
	}
	sort.Strings(added)
	sort.Strings(other)
	return added, other
}

func addedIn(tests map[string]bool, pkg, name string) bool {
	for key := range tests {
		dir, test, _ := strings.Cut(key, "|")
		if test != name {
			continue
		}
		if dir == "." || pkg == dir || strings.HasSuffix(pkg, "/"+dir) {
			return true
		}
	}
	return false
}

// newTestRepairNote opens a test repair round whose failing tests include
// ones the turn wrote; "" when none are.
func newTestRepairNote(added []string) string {
	if len(added) == 0 {
		return ""
	}
	return "These failing tests are new: this turn wrote them (" + strings.Join(added, ", ") + "). " +
		"A test you just wrote can be wrong itself. Read its failure first: when it fails in the test's own setup -- " +
		"the program, fixture or input it builds is invalid, it panics or errors before it asserts anything -- the test is what is wrong, so fix the test. " +
		"When it fails on what it asserts about the code, the code is what is wrong. Either way keep the test, asserting the behaviour the task needs.\n\n"
}

// testsBrokenPhrase is the test gate's give-up phrase: "edits broke the
// tests", unless every failing test the turn has to clear is one it wrote.
func testsBrokenPhrase(workspace string, result *ExecutionResult) string {
	const broke = "edits broke the tests"
	if result == nil {
		return broke
	}
	run := lastFailingRun(result)
	if run == nil {
		return broke
	}
	added, other := newFailingTests(workspace, turnFailures(*run), result.WrittenPaths, result.PreWriteContents)
	if len(added) == 0 || len(other) > 0 {
		return broke
	}
	return "the tests this turn wrote fail (" + strings.Join(added, ", ") + "): a new test that cannot pass, or code that does not meet it"
}
