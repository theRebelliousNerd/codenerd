package world

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Legacy manifests know the test's bytes but not its symbols. Upgrade through
// the delta path exactly once, even when no file's content changed.
func TestIncrementalScanMigratesLegacyTestSymbolsWithoutFullScan(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "source_test.go")
	if err := os.WriteFile(path, []byte("package p\nfunc TestSurface() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	scanner := NewScanner()
	ctx := context.Background()
	first, err := scanner.ScanWorkspaceIncremental(ctx, root, nil, IncrementalOptions{})
	if err != nil || !first.Full {
		t.Fatalf("initial scan=%+v err=%v", first, err)
	}
	// Emulate the pre-retirement manifest without changing the file.
	cache := NewFileCache(root)
	entry := cache.Entries[path]
	entry.TestSymbols = false
	cache.Entries[path] = entry
	cache.Dirty = true
	if err := cache.Save(); err != nil {
		t.Fatal(err)
	}
	delta, err := scanner.ScanWorkspaceIncremental(ctx, root, nil, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		t.Fatal(err)
	}
	if delta.Full || delta.Unchanged || len(delta.ChangedFiles) != 1 {
		t.Fatalf("migration must be a one-file delta: %+v", delta)
	}
	found := false
	for _, f := range delta.NewFacts {
		if f.Predicate == "symbol_graph" && len(f.Args) == 5 && f.Args[0] == "func:TestSurface" && f.Args[3] == "source_test.go" {
			found = true
		}
	}
	if !found {
		t.Fatal("legacy test symbol missing from delta")
	}
	quiet, err := scanner.ScanWorkspaceIncremental(ctx, root, nil, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil || !quiet.Unchanged {
		t.Fatalf("migration repeated: %+v err=%v", quiet, err)
	}
}
