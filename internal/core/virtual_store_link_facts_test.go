package core

import (
	"context"
	"path/filepath"
	"testing"

	"codenerd/internal/store"
)

// TestPersistLinkFacts_ShouldProjectEdgesThePolicyCanJoin — the knowledge
// graph edge for an import has to hydrate as knowledge_link(_, /depends_on, _)
// or policy/knowledge.mg's activation spreading never fires. The per-caller
// copies this replaced labelled it "depends_on:<import>", which is not a name.
func TestPersistLinkFacts_ShouldProjectEdgesThePolicyCanJoin(t *testing.T) {
	db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	defer db.Close()

	vs := NewVirtualStore(nil)
	vs.SetLocalDB(db)

	facts := []Fact{
		{Predicate: "dependency_link", Args: []any{"internal/a/a.go", "internal/b/b.go", "example.com/ws/internal/b"}},
		{Predicate: "dependency_link", Args: []any{"internal/a/a.go", "pkg:fmt", "fmt"}},
		{Predicate: "symbol_graph", Args: []any{"internal/a/a.go:Run", MangleAtom("/function"), MangleAtom("/public"), "internal/a/a.go", "func Run()"}},
		{Predicate: "file_topology", Args: []any{"internal/a/a.go", "hash", MangleAtom("/go"), int64(1), MangleAtom("/false")}},
	}
	if err := vs.PersistLinkFacts(facts, "scan-path"); err != nil {
		t.Fatalf("PersistLinkFacts: %v", err)
	}

	type edge struct{ a, rel, b string }
	var got []edge
	count, err := db.HydrateKnowledgeGraph(func(predicate string, args []any) error {
		if predicate != "knowledge_link" {
			t.Fatalf("unexpected predicate %s", predicate)
		}
		got = append(got, edge{args[0].(string), args[1].(string), args[2].(string)})
		return nil
	})
	if err != nil {
		t.Fatalf("HydrateKnowledgeGraph: %v", err)
	}
	if count != 3 {
		t.Fatalf("hydrated %d edges, want 3 (two imports, one definition): %v", count, got)
	}
	want := map[edge]bool{
		{"internal/a/a.go", "/depends_on", "internal/b/b.go"}:     false,
		{"internal/a/a.go", "/depends_on", "pkg:fmt"}:             false,
		{"internal/a/a.go:Run", "/defined_in", "internal/a/a.go"}: false,
	}
	for _, e := range got {
		if _, ok := want[e]; !ok {
			t.Errorf("unexpected edge %+v", e)
			continue
		}
		want[e] = true
	}
	for e, seen := range want {
		if !seen {
			t.Errorf("edge %+v was not hydrated", e)
		}
	}

	// A nil store is a no-op, not an error: the chat model persists links
	// opportunistically and must not fail a scan over a missing knowledge DB.
	bare := NewVirtualStore(nil)
	if err := bare.PersistLinkFacts(facts, "scan"); err != nil {
		t.Fatalf("PersistLinkFacts without a store: %v", err)
	}
	_ = context.Background()
}
