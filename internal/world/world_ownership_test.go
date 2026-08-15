package world

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"codenerd/internal/store"
	"codenerd/internal/types"
)

// recordingKernel captures what ApplyIncrementalResult does to the kernel.
type recordingKernel struct {
	types.Kernel
	removedSets    []map[string]struct{}
	retracted      []string
	loaded         []types.Fact
	retractedExact []types.Fact
}

func (k *recordingKernel) RemoveFactsByPredicateSet(set map[string]struct{}) error {
	k.removedSets = append(k.removedSets, set)
	return nil
}
func (k *recordingKernel) Retract(pred string) error {
	k.retracted = append(k.retracted, pred)
	return nil
}
func (k *recordingKernel) RetractExactFactsBatch(facts []types.Fact) error {
	k.retractedExact = append(k.retractedExact, facts...)
	return nil
}
func (k *recordingKernel) LoadFacts(facts []types.Fact) error {
	k.loaded = append(k.loaded, facts...)
	return nil
}

// TestApplyIncremental_WhenFullScan_ShouldNotWipeFactsItCannotRebuild is the
// ownership matrix as an executable rule. The full replace-set used to be the
// entire WorldPredicates list, so applying a fast scan deleted deep
// (code_defines/code_calls/data flow) and LSP-projected facts that a fast scan
// never re-emits: one rescan erased the whole deep call graph with nothing to
// restore it.
func TestApplyIncremental_WhenFullScan_ShouldNotWipeFactsItCannotRebuild(t *testing.T) {
	k := &recordingKernel{}
	if err := ApplyIncrementalResult(k, &IncrementalResult{Full: true, NewFacts: []Fact{
		{Predicate: "file_topology", Args: []any{"a.go", "h", "/go", int64(1), "/false"}},
	}}); err != nil {
		t.Fatal(err)
	}
	if len(k.removedSets) != 1 {
		t.Fatalf("expected exactly one predicate-set removal, got %d", len(k.removedSets))
	}
	removed := k.removedSets[0]

	for _, p := range ScannerPredicates {
		if _, ok := removed[p]; !ok {
			t.Errorf("scanner-owned predicate %q was not replaced; stale rows for deleted files survive", p)
		}
	}
	for _, group := range [][]string{DeepPredicates, LSPPredicates, SessionScopePredicates, GitPredicates} {
		for _, p := range group {
			if _, ok := removed[p]; ok {
				t.Errorf("predicate %q is not scanner-owned but a fast scan deletes it; nothing in this pass re-asserts it", p)
			}
		}
	}
}

// TestApplyIncremental_WhenDelta_ShouldRetractSnapshotGlobals — snapshot-global
// predicates are recomputed in full by each delta, so the previous generation
// has to go or project_language ends up holding two languages at once.
func TestApplyIncremental_WhenDelta_ShouldRetractSnapshotGlobals(t *testing.T) {
	k := &recordingKernel{}
	if err := ApplyIncrementalResult(k, &IncrementalResult{NewFacts: []Fact{
		{Predicate: "project_language", Args: []any{"/go"}},
	}}); err != nil {
		t.Fatal(err)
	}
	for _, p := range SnapshotGlobalPredicates {
		if !slices.Contains(k.retracted, p) {
			t.Errorf("delta scan did not retract snapshot-global predicate %q before reasserting it", p)
		}
	}
	if len(k.removedSets) != 0 {
		t.Error("a delta scan must not wholesale-replace scanner predicates; it only knows about the files it touched")
	}
	// entry_point is attributed per file and retracted with its file. Wiping the
	// whole relation on a delta would drop every AST-proven entry point in a file
	// this delta did not re-parse.
	if slices.Contains(k.retracted, "entry_point") {
		t.Error("delta scan wholesale-retracted entry_point; it is per-file, not single-valued")
	}
}

// TestWorldPredicates_WhenListed_ShouldCoverEveryOwnerGroup guards the matrix
// itself: a predicate added to an owner group must show up in the union used to
// recognise world facts, and no predicate may claim two owners.
func TestWorldPredicates_WhenListed_ShouldCoverEveryOwnerGroup(t *testing.T) {
	union := WorldPredicateSet()
	owners := map[string]string{}
	groups := map[string][]string{
		"scanner": ScannerPredicates,
		"deep":    DeepPredicates,
		"lsp":     LSPPredicates,
		"scope":   SessionScopePredicates,
		"git":     GitPredicates,
	}
	for name, preds := range groups {
		for _, p := range preds {
			if _, ok := union[p]; !ok {
				t.Errorf("%s predicate %q missing from WorldPredicates", p, name)
			}
			if prev, dup := owners[p]; dup {
				t.Errorf("predicate %q is claimed by both %q and %q; ownership must be unique", p, prev, name)
			}
			owners[p] = name
		}
	}
	if len(union) != len(owners) {
		t.Errorf("WorldPredicates has %d entries but the owner groups define %d", len(union), len(owners))
	}
}

// TestScannerPredicates_WhenScanning_ShouldAllBeEmitted keeps the replace-set
// honest from the other side: a predicate may only be declared scanner-owned
// (and therefore wiped on every full scan) if a scan actually produces it.
func TestScannerPredicates_WhenScanning_ShouldAllBeEmitted(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, root, "go.mod", "module example.com/app\n\ngo 1.26\n")
	writeWorkspaceFile(t, root, "internal/core/core.go", "package core\n\nfunc Do() {}\n")
	writeWorkspaceFile(t, root, "internal/core/core_test.go", "package core\n\nimport \"testing\"\n\nfunc TestDo(t *testing.T) {}\n")
	writeWorkspaceFile(t, root, "cmd/app/main.go", "package main\n\nimport \"example.com/app/internal/core\"\n\nfunc main() { core.Do() }\n")

	facts, err := NewScanner().ScanWorkspaceCtx(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, f := range facts {
		seen[f.Predicate] = true
	}
	for _, p := range ScannerPredicates {
		if !seen[p] {
			t.Errorf("predicate %q is in the scanner replace-set but a full scan never emits it: every scan would retract it and re-add nothing", p)
		}
	}
}

// TestIncrementalScan_WhenMajorityLanguageShifts_ShouldRefreshProjectLanguage —
// project_language and entry_point were only ever derived on the first (full
// fallback) scan, so a workspace that grew past its original majority language
// kept reporting the old one until the cache was deleted by hand.
func TestIncrementalScan_WhenMajorityLanguageShifts_ShouldRefreshProjectLanguage(t *testing.T) {
	root := t.TempDir()
	for i := range 3 {
		writeWorkspaceFile(t, root, filepathJoin("py", i, ".py"), "def f():\n    return 1\n")
	}
	scanner := NewScanner()
	ctx := context.Background()

	first, err := scanner.ScanWorkspaceIncremental(ctx, root, nil, IncrementalOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if first.ProjectLanguage != "python" {
		t.Fatalf("initial project language = %q, want python", first.ProjectLanguage)
	}

	// The workspace becomes majority Go, and gains an entry point.
	for i := range 6 {
		writeWorkspaceFile(t, root, filepathJoin("go", i, ".go"), "package p\n\nfunc F() {}\n")
	}
	writeWorkspaceFile(t, root, "cmd/app/main.go", "package main\n\nfunc main() {}\n")

	delta, err := scanner.ScanWorkspaceIncremental(ctx, root, nil, IncrementalOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if delta.Full {
		t.Fatal("expected a delta scan")
	}
	if delta.ProjectLanguage != "go" {
		t.Errorf("delta project language = %q, want go (majority shifted)", delta.ProjectLanguage)
	}
	var sawLanguageFact, sawEntryPoint bool
	for _, f := range delta.NewFacts {
		if f.Predicate == "project_language" {
			sawLanguageFact = true
		}
		if f.Predicate == "entry_point" {
			if p, _ := f.Args[0].(string); p == "cmd/app/main.go" {
				sawEntryPoint = true
			}
		}
	}
	if !sawLanguageFact {
		t.Error("delta scan emitted no project_language fact, so the kernel keeps the stale one")
	}
	if !sawEntryPoint {
		t.Error("delta scan did not report the new entry point cmd/app/main.go")
	}
}

func filepathJoin(dir string, i int, ext string) string {
	return filepath.ToSlash(filepath.Join(dir, "f"+string(rune('a'+i))+ext))
}

// TestIncrementalScan_WhenFileChanges_ShouldRetractItsPreviousFacts proves the
// store keys line up: the retraction lookup is by canonical path, and it used
// to be by absolute path while the rows were written canonically, so no scan
// ever retracted anything and superseded facts accumulated forever.
func TestIncrementalScan_WhenFileChanges_ShouldRetractItsPreviousFacts(t *testing.T) {
	db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	defer db.Close()
	root := t.TempDir()
	target := writeWorkspaceFile(t, root, "pkg/thing.go", "package pkg\n\nfunc Old() {}\n")

	scanner := NewScanner()
	ctx := context.Background()
	if _, err := scanner.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("package pkg\n\nfunc New() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	delta, scanErr := scanner.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{})
	if scanErr != nil {
		t.Fatal(scanErr)
	}
	if len(delta.RetractFacts) == 0 {
		t.Fatal("changed file produced no retraction set; its superseded facts stay in the kernel forever")
	}
	var sawOldTopology bool
	for _, f := range delta.RetractFacts {
		if f.Predicate == "file_topology" {
			if p, _ := f.Args[0].(string); p == "pkg/thing.go" {
				sawOldTopology = true
			}
		}
	}
	if !sawOldTopology {
		t.Errorf("retraction set does not contain the previous file_topology for pkg/thing.go: %v", delta.RetractFacts)
	}
}
