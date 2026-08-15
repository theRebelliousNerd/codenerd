package world

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/store"
)

// TestIncrementalScan_WhenAFileIsDeleted_ShouldRetractAllOfItsFacts covers two
// defects that combined into one symptom: deleting a file left its facts in the
// kernel forever, with the scanner reporting success.
//
//  1. Root-level files were filed under the wrong owner. groupFactsByPath
//     decided which file a fact belonged to by looking for an argument
//     "containing a slash". "sub/gamma.go" matched; "alpha.go" did not. So every
//     symbol_graph, file_dir and entry_point fact for a file at the repository
//     root went to the global bucket instead of to its file, and deleting the
//     file could not take them with it. Measured before the fix: a nested file
//     persisted 4 rows, a root-level one persisted 1.
//
//  2. The one fact that WAS filed correctly could not be retracted anyway.
//     Retraction facts are rebuilt by reading the persisted rows back, and a
//     retraction only removes a fact matching argument-for-argument — but int64
//     nanosecond timestamps were rounded on the way out of JSON (see
//     internal/store TestFactArgs_WhenInt64ExceedsFloat64Precision). The rebuilt
//     file_topology carried a ModTime a few hundred nanoseconds off from the one
//     that was asserted, and matched nothing.
//
// Either defect alone makes deletion leak. The test asserts both directions:
// every fact is retracted, and each retracted fact is byte-identical to the one
// the first scan emitted.
func TestIncrementalScan_WhenAFileIsDeleted_ShouldRetractAllOfItsFacts(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".nerd", "cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(rel, content string) {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// One at the repository root and one nested: the root-level case is the one
	// the slash heuristic silently excluded, so a fixture with only nested files
	// would have passed throughout.
	write("alpha.go", "package main\n\nfunc Alpha() {}\n")
	write("sub/gamma.go", "package sub\n\nfunc Gamma() {}\n")
	write("keep.go", "package main\n\nfunc Keep() {}\n")

	db, err := store.NewLocalStore(filepath.Join(root, ".nerd", "world.db"))
	if err != nil {
		t.Skipf("local store unavailable: %v", err)
	}
	defer db.Close()

	sc := NewScanner()
	ctx := context.Background()

	first, err := sc.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{})
	if err != nil {
		t.Fatalf("first scan: %v", err)
	}
	if !first.Full {
		t.Fatal("expected the first scan to be full")
	}

	factsFor := func(res *IncrementalResult, sel func(*IncrementalResult) []coreFact, rel string) []coreFact {
		var out []coreFact
		for _, f := range sel(res) {
			for _, a := range f.Args {
				if s, ok := a.(string); ok && s == rel {
					out = append(out, f)
					break
				}
			}
		}
		return out
	}

	for _, rel := range []string{"alpha.go", "sub/gamma.go"} {
		if got := len(factsFor(first, newFactsOf, rel)); got == 0 {
			t.Fatalf("the first scan emitted no facts naming %s; the fixture is not exercising anything", rel)
		}
	}

	// Delete one file from each location.
	for _, rel := range []string{"alpha.go", "sub/gamma.go"} {
		if err := os.Remove(filepath.Join(root, rel)); err != nil {
			t.Fatal(err)
		}
	}

	second, err := sc.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{})
	if err != nil {
		t.Fatalf("second scan: %v", err)
	}
	if second.Full {
		t.Fatal("expected the second scan to be incremental")
	}

	for _, rel := range []string{"alpha.go", "sub/gamma.go"} {
		emitted := factsFor(first, newFactsOf, rel)
		retracted := factsFor(second, retractFactsOf, rel)

		if len(retracted) < len(emitted) {
			t.Errorf("%s: emitted %d facts, retracted only %d — the rest stay in the kernel "+
				"for the life of the workspace\n  emitted:   %v\n  retracted: %v",
				rel, len(emitted), len(retracted), emitted, retracted)
		}

		// Identity, not just count: a retraction that does not match the
		// asserted fact argument-for-argument removes nothing.
		retractedKeys := map[string]bool{}
		for _, f := range retracted {
			retractedKeys[factKey(f)] = true
		}
		for _, f := range emitted {
			if !retractedKeys[factKey(f)] {
				t.Errorf("%s: no retraction matches the emitted fact %s — a near-miss "+
					"retraction (a rounded timestamp, say) silently removes nothing",
					rel, factKey(f))
			}
		}

		// And it must not still be asserting facts for a file that is gone.
		if got := len(factsFor(second, newFactsOf, rel)); got != 0 {
			t.Errorf("%s: the delta scan emitted %d facts for a deleted file", rel, got)
		}
	}

	// The surviving file must be untouched by any of this.
	if got := len(factsFor(second, retractFactsOf, "keep.go")); got != 0 {
		t.Errorf("keep.go was not modified or deleted, but %d of its facts were retracted", got)
	}
}

// Small aliases so the assertions above read as prose rather than as
// core.Fact plumbing.
type coreFact = core.Fact

func newFactsOf(r *IncrementalResult) []coreFact     { return r.NewFacts }
func retractFactsOf(r *IncrementalResult) []coreFact { return r.RetractFacts }

func factKey(f coreFact) string {
	return fmt.Sprintf("%s%v", f.Predicate, f.Args)
}
