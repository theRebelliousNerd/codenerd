package world

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/store"
)

func TestRetireNonCanonicalRows(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.go"), []byte("package main\n\nfunc A() {}\n"), 0o644)
	os.MkdirAll(filepath.Join(root, "sub"), 0o755)
	os.WriteFile(filepath.Join(root, "sub", "b.go"), []byte("package sub\n\nfunc B() {}\n"), 0o644)

	db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	absKey := `C:\fakeroot\thing.go`
	db.UpsertWorldFile(store.WorldFileMeta{Path: absKey, Lang: "go", Fingerprint: "stale"})
	db.ReplaceWorldFactsForFile(absKey, "fast", "stale", []store.WorldFactInput{
		{Predicate: "file_topology", Args: []any{absKey, "h", "/go", int64(1), "/false"}},
	})

	sc := NewScanner()
	_, err = sc.ScanWorkspaceIncremental(context.Background(), root, db, IncrementalOptions{})
	if err != nil {
		t.Fatal(err)
	}

	paths, err := db.ListWorldFilePaths()
	if err != nil {
		t.Fatal(err)
	}
	assertRetired(t, paths, absKey)
}

func assertRetired(t *testing.T, paths []string, absKey string) {
	t.Helper()
	seen := map[string]bool{}
	for _, p := range paths {
		seen[p] = true
	}
	if seen[absKey] {
		t.Fatalf("absolute row %q survived", absKey)
	}
	if !seen["a.go"] {
		t.Fatalf("canonical a.go missing: %q", paths)
	}
	if !seen["sub/b.go"] {
		t.Fatalf("canonical sub/b.go missing: %q", paths)
	}
}
