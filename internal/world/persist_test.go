package world

import (
	"os"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/store"
)

func TestPersistFastSnapshotToDB_PreservesGlobalFacts(t *testing.T) {
	dbPath := t.TempDir() + "/world.db"
	db, err := store.NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("NewLocalStore failed: %v", err)
	}
	defer db.Close()

	filePath := t.TempDir() + "/main.go"
	if err := os.WriteFile(filePath, []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	facts := []core.Fact{
		{
			Predicate: "file_topology",
			Args: []interface{}{
				filePath,
				"hash123",
				core.MangleAtom("/go"),
				int64(10),
				core.MangleAtom("/file"),
			},
		},
		{
			Predicate: "project_language",
			Args:      []interface{}{core.MangleAtom("/go")},
		},
		{
			Predicate: "entry_point",
			Args:      []interface{}{"main.main"},
		},
	}

	if err := PersistFastSnapshotToDB(db, facts); err != nil {
		t.Fatalf("PersistFastSnapshotToDB failed: %v", err)
	}

	loaded, err := db.LoadAllWorldFacts("fast")
	if err != nil {
		t.Fatalf("LoadAllWorldFacts failed: %v", err)
	}

	seen := map[string]bool{}
	for _, fact := range loaded {
		seen[fact.Predicate] = true
	}

	if !seen["file_topology"] {
		t.Fatal("expected file_topology fact in cached snapshot")
	}
	if !seen["project_language"] {
		t.Fatal("expected project_language fact in cached snapshot")
	}
	if !seen["entry_point"] {
		t.Fatal("expected entry_point fact in cached snapshot")
	}
}
