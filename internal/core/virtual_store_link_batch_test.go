package core

import (
	"path/filepath"
	"testing"

	"codenerd/internal/store"
)

// A projection is stored in one transaction, but an edge with an empty end is
// still refused on its own: the error names it and every valid edge lands.
func TestPersistLinkFacts_RefusesAnEmptyEdgeAndStoresTheRest(t *testing.T) {
	db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	defer db.Close()
	vs := NewVirtualStore(nil)
	vs.SetLocalDB(db)
	facts := []Fact{
		{Predicate: "dependency_link", Args: []any{"a.go", "b.go", "example.com/b"}},
		{Predicate: "dependency_link", Args: []any{"", "c.go", "example.com/c"}},
		{Predicate: "symbol_graph", Args: []any{"a.go:Run", MangleAtom("/function"), MangleAtom("/public"), "a.go", "func Run()"}},
	}
	if err := vs.PersistLinkFacts(facts, "scan"); err == nil {
		t.Fatal("an edge with an empty end was accepted")
	}
	count, err := db.HydrateKnowledgeGraph(func(string, []any) error { return nil })
	if err != nil || count != 2 {
		t.Fatalf("hydrated %d edges (%v), want the 2 valid ones", count, err)
	}
}
