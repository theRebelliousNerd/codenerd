package world

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/store"
)

// A refresh that removes an import retires the depends_on edge the import
// projected into knowledge_graph. The world cache and the kernel already
// retracted the old dependency_link; the graph mirror only ever upserted, so
// the edge kept answering graph queries and session dependency context as
// current (audit scanner-graph-source-retirement). This runs the real
// incremental scanner and the projection the chat scan uses.
func TestIncrementalScan_RemovedImportRetiresItsGraphEdge(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/ws\n\ngo 1.22\n")
	write("b/b.go", "package b\n\nfunc B() {}\n")
	write("a.go", "package main\n\nimport \"example.com/ws/b\"\n\nfunc main() { b.B() }\n")
	if err := os.MkdirAll(filepath.Join(root, ".nerd", "cache"), 0o755); err != nil {
		t.Fatal(err)
	}

	db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	vs := core.NewVirtualStore(nil)
	vs.SetLocalDB(db)
	scanner := NewScanner()

	scan := func() {
		t.Helper()
		res, err := scanner.ScanWorkspaceIncremental(ctx, root, db, IncrementalOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if err := vs.ReplaceLinkFacts(res.RetractFacts, res.NewFacts, "scan"); err != nil {
			t.Fatal(err)
		}
	}
	edges := func() map[string]string {
		t.Helper()
		links, err := db.QueryLinks("a.go", "outgoing")
		if err != nil {
			t.Fatal(err)
		}
		out := make(map[string]string, len(links))
		for _, l := range links {
			src, _ := l.Metadata["source"].(string)
			out[l.Relation+" "+l.EntityB] = src
		}
		return out
	}

	scan()
	if _, ok := edges()["depends_on b/b.go"]; !ok {
		t.Fatalf("precondition: the import a.go -> b/b.go was not projected: %v", edges())
	}
	// Knowledge the session recorded about the same file is not the scanner's.
	if err := db.StoreLink("a.go", "depends_on", "notes/design.md", 1.0, map[string]any{"source": "memory_operation"}); err != nil {
		t.Fatal(err)
	}

	// The import goes; a.go stays and still parses.
	time.Sleep(10 * time.Millisecond)
	write("a.go", "package main\n\nfunc main() {}\n")
	scan()

	got := edges()
	if _, stale := got["depends_on b/b.go"]; stale {
		t.Fatalf("a.go no longer imports b, but its graph edge survived the refresh: %v", got)
	}
	for edge, src := range got {
		if src != "memory_operation" && len(edge) > len("depends_on ") && edge[:len("depends_on ")] == "depends_on " {
			t.Fatalf("an import edge outlived the import: %s (from %s) in %v", edge, src, got)
		}
	}
	if got["depends_on notes/design.md"] != "memory_operation" {
		t.Fatalf("the memory edge was retired with the scanner's: %v", got)
	}
	defined, err := db.QueryLinks("a.go", "incoming")
	if err != nil {
		t.Fatal(err)
	}
	var mainDefined bool
	for _, l := range defined {
		if l.Relation == "defined_in" {
			mainDefined = true
		}
	}
	if !mainDefined {
		t.Fatalf("the still-current definition edge into a.go was retired: %v", defined)
	}
}
