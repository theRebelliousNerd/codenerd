package world

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
	// The import is projected in factors: a.go imports the unit, and b/b.go is
	// a member of it (schemas_world.mg).
	if _, ok := edges()["imports example.com/ws/b"]; !ok {
		t.Fatalf("precondition: the import a.go -> example.com/ws/b was not projected: %v", edges())
	}
	if links, err := db.QueryLinks("b/b.go", "outgoing"); err != nil {
		t.Fatal(err)
	} else {
		member := false
		for _, l := range links {
			member = member || (l.Relation == "member_of" && l.EntityB == "example.com/ws/b")
		}
		if !member {
			t.Fatalf("precondition: b/b.go is not projected as a member of its unit: %v", links)
		}
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
	if _, stale := got["imports example.com/ws/b"]; stale {
		t.Fatalf("a.go no longer imports b, but its graph edge survived the refresh: %v", got)
	}
	for edge, src := range got {
		if src == "memory_operation" {
			continue
		}
		for _, relation := range []string{"depends_on ", "imports "} {
			if strings.HasPrefix(edge, relation) {
				t.Fatalf("an import edge outlived the import: %s (from %s) in %v", edge, src, got)
			}
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
