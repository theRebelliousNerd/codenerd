package store

import (
	"path/filepath"
	"testing"
)

// ReplaceWorldLinks retires only rows the world projection wrote: those it
// marks now, and those its source labels identify from before the marker.
// A memory-operation row on the very same triple stays.
func TestReplaceWorldLinks_RetiresOnlyTheProjectionsRows(t *testing.T) {
	db, err := NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// A legacy projection row (no marker) and a memory row.
	if err := db.StoreLink("a.go", "depends_on", "old.go", 1.0, map[string]any{"source": "scan"}); err != nil {
		t.Fatal(err)
	}
	if err := db.StoreLink("a.go", "depends_on", "kept.go", 1.0, map[string]any{"source": "memory_operation"}); err != nil {
		t.Fatal(err)
	}
	// A marked projection row.
	if err := db.ReplaceWorldLinks(nil, []KnowledgeLink{{EntityA: "a.go", Relation: "depends_on", EntityB: "marked.go", Weight: 1, Metadata: map[string]any{"source": "scan-deep"}}}); err != nil {
		t.Fatal(err)
	}

	retire := []KnowledgeLink{
		{EntityA: "a.go", Relation: "depends_on", EntityB: "old.go"},
		{EntityA: "a.go", Relation: "depends_on", EntityB: "kept.go"},
		{EntityA: "a.go", Relation: "depends_on", EntityB: "marked.go"},
	}
	publish := []KnowledgeLink{{EntityA: "a.go", Relation: "depends_on", EntityB: "new.go", Weight: 1, Metadata: map[string]any{"source": "scan"}}}
	if err := db.ReplaceWorldLinks(retire, publish); err != nil {
		t.Fatal(err)
	}

	links, err := db.QueryLinks("a.go", "outgoing")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]map[string]any{}
	for _, l := range links {
		got[l.EntityB] = l.Metadata
	}
	if _, ok := got["old.go"]; ok {
		t.Error("the legacy projection row was not retired")
	}
	if _, ok := got["marked.go"]; ok {
		t.Error("the marked projection row was not retired")
	}
	if got["kept.go"]["source"] != "memory_operation" {
		t.Errorf("the memory row was retired or rewritten: %v", got["kept.go"])
	}
	if got["new.go"]["projection"] != WorldLinkProjection {
		t.Errorf("a published row lacks the projection marker: %v", got["new.go"])
	}

	// A bad edge fails the whole replacement: nothing is half-published.
	if err := db.ReplaceWorldLinks([]KnowledgeLink{{EntityA: "a.go", Relation: "depends_on", EntityB: "new.go"}},
		[]KnowledgeLink{{EntityA: "", Relation: "depends_on", EntityB: "x.go", Weight: 1}}); err == nil {
		t.Fatal("an empty endpoint was accepted")
	}
	if links, _ := db.QueryLinks("a.go", "outgoing"); len(links) != 2 {
		t.Fatalf("a refused replacement changed the graph: %v", links)
	}
}
