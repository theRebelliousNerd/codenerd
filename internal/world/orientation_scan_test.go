package world

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/orient"
	"codenerd/internal/types"
)

func orientationFixture(t *testing.T, initializeSnapshot bool) string {
	t.Helper()
	root := t.TempDir()
	runOrientationGit(t, root, "init", "--quiet")
	runOrientationGit(t, root, "config", "user.name", "World Orientation Test")
	runOrientationGit(t, root, "config", "user.email", "world-orientation@example.invalid")
	writeStructFile(t, root, "README.md", "# Fixture\n\nInitial documentation.\n")
	writeStructFile(t, root, "main.go", "package main\n\nfunc main() {}\n")
	runOrientationGit(t, root, "add", "README.md", "main.go")
	runOrientationGit(t, root, "commit", "--quiet", "-m", "initial")
	if initializeSnapshot {
		ctx := context.Background()
		snapshot, err := orient.Measure(ctx, root, config.OrientConfig{}.WithDefaults(), nil)
		if err != nil {
			t.Fatalf("measure orientation fixture: %v", err)
		}
		engine, err := snapshot.Engine(ctx)
		if err != nil {
			t.Fatalf("build orientation fixture engine: %v", err)
		}
		if err := snapshot.Save(root, engine); err != nil {
			_ = engine.Close()
			t.Fatalf("save orientation fixture: %v", err)
		}
		if err := engine.Close(); err != nil {
			t.Fatalf("close orientation fixture engine: %v", err)
		}
	}
	return root
}

func runOrientationGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"-C", root}, args...)
	cmd := exec.Command("git", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

type orientationTransactionRecorder struct {
	added   []types.Fact
	removed []types.Fact
	commits int
}

func (tx *orientationTransactionRecorder) Retract(string)         {}
func (tx *orientationTransactionRecorder) RetractFact(types.Fact) {}
func (tx *orientationTransactionRecorder) RetractExactFact(fact types.Fact) {
	tx.removed = append(tx.removed, fact)
}
func (tx *orientationTransactionRecorder) RetractPredicateSet(map[string]struct{}) {}
func (tx *orientationTransactionRecorder) Assert(fact types.Fact) {
	tx.added = append(tx.added, fact)
}
func (tx *orientationTransactionRecorder) Commit() error {
	tx.commits++
	return nil
}

type orientationApplyKernel struct {
	recordingKernel
	tx *orientationTransactionRecorder
}

func (k *orientationApplyKernel) Transaction() types.KernelTransaction {
	if k.tx == nil {
		k.tx = &orientationTransactionRecorder{}
	}
	return k.tx
}

func TestApplyIncrementalRequiresTransactionBeforeWorldChanges(t *testing.T) {
	kernel := &recordingKernel{}
	result := &IncrementalResult{
		NewFacts: []Fact{{Predicate: "file_topology", Args: []any{"main.go"}}},
		Orientation: &orient.RefreshResult{
			Refreshed: true,
			Added:     []types.Fact{{Predicate: "oriented_head", Args: []any{"new-head"}}},
		},
	}
	if err := ApplyIncrementalResult(kernel, result); err == nil || !strings.Contains(err.Error(), "does not support transactions") {
		t.Fatalf("ApplyIncrementalResult error = %v; want a transactor requirement", err)
	}
	if len(kernel.removedSets) != 0 || len(kernel.loaded) != 0 || len(kernel.retracted) != 0 {
		t.Fatalf("world facts changed before rejecting non-transactional orientation apply: removed=%d loaded=%d retracted=%v", len(kernel.removedSets), len(kernel.loaded), kernel.retracted)
	}
}

func factStrings(facts []types.Fact) []string {
	rows := make([]string, 0, len(facts))
	for _, fact := range facts {
		rows = append(rows, fact.String())
	}
	return rows
}

func TestIncrementalScanRefreshesOrientationForUnchangedWorldAndTrackedDocumentChanges(t *testing.T) {
	root := orientationFixture(t, true)
	ctx := context.Background()
	scanner := NewScanner()
	kernel := &orientationApplyKernel{}

	initial, err := scanner.ScanWorkspaceIncremental(ctx, root, nil, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		t.Fatalf("initial scan: %v", err)
	}
	if !initial.Full || initial.Orientation == nil || initial.OrientationNotInitialized {
		t.Fatalf("initial result: full=%v orientation=%+v notInitialized=%v; want full scan with an initialized refresh result", initial.Full, initial.Orientation, initial.OrientationNotInitialized)
	}
	if err := ApplyIncrementalResult(kernel, initial); err != nil {
		t.Fatalf("apply initial result: %v", err)
	}

	quiet, err := scanner.ScanWorkspaceIncremental(ctx, root, nil, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil || quiet == nil || !quiet.Unchanged || quiet.Orientation == nil || quiet.Orientation.Refreshed {
		t.Fatalf("stable scan: result=%+v err=%v; want unchanged world and stable orientation", quiet, err)
	}
	if err := ApplyIncrementalResult(kernel, quiet); err != nil {
		t.Fatalf("apply stable result: %v", err)
	}
	if len(kernel.retracted) != 0 {
		t.Fatalf("unchanged result retracted world globals: %v", kernel.retracted)
	}

	oldHead := runOrientationGit(t, root, "rev-parse", "HEAD")
	runOrientationGit(t, root, "commit", "--quiet", "--allow-empty", "-m", "advance orientation without source changes")
	newHead := runOrientationGit(t, root, "rev-parse", "HEAD")
	if oldHead == newHead {
		t.Fatal("fixture commit did not advance HEAD")
	}

	started := time.Now()
	refreshed, err := scanner.ScanWorkspaceIncremental(ctx, root, nil, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		t.Fatalf("head-only scan: %v", err)
	}
	if refreshed == nil || !refreshed.Unchanged {
		t.Fatalf("head-only scan changed world census: %+v; want world-Unchanged", refreshed)
	}
	if refreshed.Orientation == nil || !refreshed.Orientation.Refreshed {
		t.Fatalf("head-only scan did not refresh orientation: %+v", refreshed.Orientation)
	}
	if err := ApplyIncrementalResult(kernel, refreshed); err != nil {
		t.Fatalf("apply head-only refresh: %v", err)
	}
	if got, want := factStrings(kernel.tx.added), factStrings(refreshed.Orientation.Added); !reflect.DeepEqual(got, want) {
		t.Fatalf("transaction additions = %v, RefreshResult.Added = %v", got, want)
	}
	if got, want := factStrings(kernel.tx.removed), factStrings(refreshed.Orientation.Removed); !reflect.DeepEqual(got, want) {
		t.Fatalf("transaction removals = %v, RefreshResult.Removed = %v", got, want)
	}
	if kernel.tx.commits != 1 {
		t.Fatalf("orientation applied in %d transactions; want one", kernel.tx.commits)
	}
	if len(kernel.retracted) != 0 {
		t.Fatalf("world-Unchanged orientation refresh retracted global world facts: %v", kernel.retracted)
	}
	var sawOld, sawNew bool
	for _, row := range refreshed.Orientation.Removed {
		if row.Predicate == "oriented_head" && len(row.Args) == 1 && types.ExtractString(row.Args[0]) == oldHead {
			sawOld = true
		}
	}
	for _, row := range refreshed.Orientation.Added {
		if row.Predicate == "oriented_head" && len(row.Args) == 1 && types.ExtractString(row.Args[0]) == newHead {
			sawNew = true
		}
	}
	if !sawOld || !sawNew {
		t.Fatalf("orientation head delta removed old=%v and added new=%v; rows removed=%v added=%v", sawOld, sawNew, refreshed.Orientation.Removed, refreshed.Orientation.Added)
	}
	t.Logf("world-Unchanged head refresh took %s; orientation added=%d removed=%d", time.Since(started), len(refreshed.Orientation.Added), len(refreshed.Orientation.Removed))

	oldDocumentHead := newHead
	writeStructFile(t, root, "README.md", "# Fixture\n\nUpdated documentation.\n")
	runOrientationGit(t, root, "add", "README.md")
	runOrientationGit(t, root, "commit", "--quiet", "-m", "update tracked documentation")
	committedPaths := runOrientationGit(t, root, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD")
	if committedPaths != "README.md" {
		t.Fatalf("orientation fixture commit changed %q; want only README.md", committedPaths)
	}
	newDocumentHead := runOrientationGit(t, root, "rev-parse", "HEAD")
	if oldDocumentHead == newDocumentHead {
		t.Fatal("tracked-document commit did not advance HEAD")
	}
	documentStarted := time.Now()
	docChanged, err := scanner.ScanWorkspaceIncremental(ctx, root, nil, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		t.Fatalf("tracked-document-only scan: %v", err)
	}
	if docChanged == nil || docChanged.Unchanged || !reflect.DeepEqual(docChanged.ChangedFiles, []string{filepath.Join(root, "README.md")}) {
		t.Fatalf("tracked-document-only scan = %+v; want only README.md in the world delta", docChanged)
	}
	if docChanged.Orientation == nil || !docChanged.Orientation.Refreshed {
		t.Fatalf("tracked-document-only scan did not carry orientation refresh: %+v", docChanged.Orientation)
	}
	addedStart, removedStart := len(kernel.tx.added), len(kernel.tx.removed)
	if err := ApplyIncrementalResult(kernel, docChanged); err != nil {
		t.Fatalf("apply tracked-document-only refresh: %v", err)
	}
	if got, want := factStrings(kernel.tx.added[addedStart:]), factStrings(docChanged.Orientation.Added); !reflect.DeepEqual(got, want) {
		t.Fatalf("document transaction additions = %v, RefreshResult.Added = %v", got, want)
	}
	if got, want := factStrings(kernel.tx.removed[removedStart:]), factStrings(docChanged.Orientation.Removed); !reflect.DeepEqual(got, want) {
		t.Fatalf("document transaction removals = %v, RefreshResult.Removed = %v", got, want)
	}
	if kernel.tx.commits != 2 {
		t.Fatalf("orientation applied in %d transactions after document commit; want one transaction per refresh", kernel.tx.commits)
	}
	var oldDocumentDigest, newDocumentDigest string
	var removedDocumentHead, addedDocumentHead bool
	for _, row := range docChanged.Orientation.Removed {
		if row.Predicate == "oriented_document" && len(row.Args) == 2 && types.ExtractString(row.Args[0]) == "README.md" {
			oldDocumentDigest = types.ExtractString(row.Args[1])
		}
		if row.Predicate == "oriented_head" && len(row.Args) == 1 && types.ExtractString(row.Args[0]) == oldDocumentHead {
			removedDocumentHead = true
		}
	}
	for _, row := range docChanged.Orientation.Added {
		if row.Predicate == "oriented_document" && len(row.Args) == 2 && types.ExtractString(row.Args[0]) == "README.md" {
			newDocumentDigest = types.ExtractString(row.Args[1])
		}
		if row.Predicate == "oriented_head" && len(row.Args) == 1 && types.ExtractString(row.Args[0]) == newDocumentHead {
			addedDocumentHead = true
		}
	}
	if !removedDocumentHead || !addedDocumentHead || oldDocumentDigest == "" || newDocumentDigest == "" || oldDocumentDigest == newDocumentDigest {
		t.Fatalf("tracked README orientation delta old/new head=%v/%v document digest=%q/%q; rows removed=%v added=%v", removedDocumentHead, addedDocumentHead, oldDocumentDigest, newDocumentDigest, docChanged.Orientation.Removed, docChanged.Orientation.Added)
	}
	t.Logf("tracked-document-only scan took %s; orientation added=%d removed=%d", time.Since(documentStarted), len(docChanged.Orientation.Added), len(docChanged.Orientation.Removed))

	retractCountBeforeRepeat := len(kernel.retracted)
	repeat, err := scanner.ScanWorkspaceIncremental(ctx, root, nil, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil || repeat == nil || !repeat.Unchanged || repeat.Orientation == nil || repeat.Orientation.Refreshed {
		t.Fatalf("repeat scan: result=%+v err=%v; want stable unchanged result", repeat, err)
	}
	if err := ApplyIncrementalResult(kernel, repeat); err != nil {
		t.Fatalf("apply repeat result: %v", err)
	}
	if kernel.tx.commits != 2 || len(kernel.retracted) != retractCountBeforeRepeat {
		t.Fatalf("repeat changed kernel state: orientation commits=%d globalRetracts=%v", kernel.tx.commits, kernel.retracted)
	}
}

func TestIncrementalScanAllowsMissingOrientationSnapshot(t *testing.T) {
	root := orientationFixture(t, false)
	result, err := NewScanner().ScanWorkspaceIncremental(context.Background(), root, nil, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		t.Fatalf("initial scan without orientation snapshot: %v", err)
	}
	if result == nil || !result.Full || result.Orientation != nil || !result.OrientationNotInitialized {
		t.Fatalf("result without orientation snapshot: %+v; want full scan and explicit not-initialized state", result)
	}
}

func TestIncrementalScanSurfacesOrientationFailureAndCancellation(t *testing.T) {
	root := orientationFixture(t, false)
	snapshotPath := filepath.Join(root, ".nerd", "orientation", "snapshot.json")
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(snapshotPath, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	scanner := NewScanner()
	if _, err := scanner.ScanWorkspaceIncremental(context.Background(), root, nil, IncrementalOptions{SkipWhenUnchanged: true}); err == nil || !strings.Contains(err.Error(), "incremental orientation refresh") {
		t.Fatalf("malformed orientation snapshot error = %v; want surfaced refresh failure", err)
	}

	// The failed refresh must not save the world census cache. Repair the
	// snapshot and the next attempt should still perform its initial full scan.
	snapshot, err := orient.Measure(context.Background(), root, config.OrientConfig{}.WithDefaults(), nil)
	if err != nil {
		t.Fatalf("measure repaired orientation: %v", err)
	}
	engine, err := snapshot.Engine(context.Background())
	if err != nil {
		t.Fatalf("build repaired orientation engine: %v", err)
	}
	if err := snapshot.Save(root, engine); err != nil {
		_ = engine.Close()
		t.Fatalf("save repaired orientation: %v", err)
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	retried, err := scanner.ScanWorkspaceIncremental(context.Background(), root, nil, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil || retried == nil || !retried.Full {
		t.Fatalf("retry after orientation repair: result=%+v err=%v; want uncached full scan", retried, err)
	}
	if err := os.Remove(filepath.Join(root, "README.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := scanner.ScanWorkspaceIncremental(context.Background(), root, nil, IncrementalOptions{SkipWhenUnchanged: true}); err == nil || !strings.Contains(err.Error(), "incremental orientation refresh") {
		t.Fatalf("missing tracked-document error = %v; want surfaced refresh failure", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := scanner.ScanWorkspaceIncremental(ctx, root, nil, IncrementalOptions{SkipWhenUnchanged: true}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled refresh error = %v; want context.Canceled", err)
	}
}
