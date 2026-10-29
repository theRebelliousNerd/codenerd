package store

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// A scan's whole projection is stored in one transaction: every edge lands,
// an invalid edge refuses the batch, and the result reads back exactly as
// one-at-a-time stores did.
func TestStoreLinksStoresABatchAsOne(t *testing.T) {
	db, err := NewLocalStore(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var links []KnowledgeLink
	for i := 0; i < 500; i++ {
		links = append(links, KnowledgeLink{EntityA: fmt.Sprintf("f%d.go", i), Relation: "depends_on", EntityB: "core.go", Weight: 1, Metadata: map[string]any{"source": "scan"}})
	}
	start := time.Now()
	if err := db.StoreLinks(links); err != nil {
		t.Fatal(err)
	}
	batch := time.Since(start)
	in, err := db.QueryLinks("core.go", "incoming")
	if err != nil || len(in) != 500 {
		t.Fatalf("incoming links %d, %v; want 500", len(in), err)
	}

	bad := append([]KnowledgeLink{{EntityA: "x", Relation: "r", EntityB: "y", Weight: 1}}, KnowledgeLink{EntityA: "", Relation: "r", EntityB: "z", Weight: 1})
	if err := db.StoreLinks(bad); err == nil {
		t.Fatal("a batch with an invalid edge was accepted")
	}
	if out, _ := db.QueryLinks("x", "outgoing"); len(out) != 0 {
		t.Fatalf("a refused batch stored %d edges", len(out))
	}

	start = time.Now()
	for i := 0; i < 500; i++ {
		if err := db.StoreLink(fmt.Sprintf("g%d.go", i), "depends_on", "util.go", 1, nil); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("500 edges: one batch %v, one at a time %v", batch, time.Since(start))
}

func TestStoreFactsStoresABatchAsOne(t *testing.T) {
	db, err := NewLocalStore(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var facts []ColdFact
	for i := 0; i < 300; i++ {
		facts = append(facts, ColdFact{Predicate: "batch_fact", Args: []any{fmt.Sprintf("v%d", i), int64(i)}})
	}
	if err := db.StoreFacts(facts, "fact", 5); err != nil {
		t.Fatal(err)
	}
	if err := db.StoreFacts(facts[:10], "fact", 7); err != nil { // upsert, not duplicate
		t.Fatal(err)
	}
	got, err := db.LoadFacts("batch_fact")
	if err != nil || len(got) != 300 {
		t.Fatalf("loaded %d facts, %v; want 300", len(got), err)
	}
	raised := 0
	for _, f := range got {
		if f.Priority == 7 {
			raised++
		}
	}
	if raised != 10 {
		t.Fatalf("%d facts carry the upserted priority, want 10", raised)
	}
}
