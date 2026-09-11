package store

import (
	"path/filepath"
	"testing"
)

// TestNewLocalStore_ShouldRetireMislabelledDependencyEdges — rows written under
// the old "depends_on:<import>" label hydrated as string relations no policy
// rule could join, on every boot. Opening the store drops them; edges under
// the joinable label survive.
func TestNewLocalStore_ShouldRetireMislabelledDependencyEdges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "knowledge.db")
	db, err := NewLocalStore(path)
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	for _, rel := range []string{"depends_on:fmt", "depends_on:codenerd/internal/core", "depends_on", "defined_in"} {
		if err := db.StoreLink("internal/a/a.go", rel, "internal/b/b.go", 1.0, nil); err != nil {
			t.Fatalf("StoreLink(%s): %v", rel, err)
		}
	}
	db.Close()

	db, err = NewLocalStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db.Close()

	var relations []string
	if _, err := db.HydrateKnowledgeGraph(func(_ string, args []any) error {
		relations = append(relations, args[1].(string))
		return nil
	}); err != nil {
		t.Fatalf("HydrateKnowledgeGraph: %v", err)
	}
	if len(relations) != 2 {
		t.Fatalf("hydrated relations %v, want only /depends_on and /defined_in", relations)
	}
	for _, rel := range relations {
		if rel != "/depends_on" && rel != "/defined_in" {
			t.Errorf("mislabelled relation %q survived the reopen", rel)
		}
	}
}
