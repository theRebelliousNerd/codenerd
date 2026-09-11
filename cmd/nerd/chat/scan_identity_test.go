package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/store"
	"codenerd/internal/types"
)

// The chat scan commands (/scan-path, /scan-dir) must produce facts under the
// same identity the full and incremental scanners use: workspace-relative,
// forward-slash. They keyed everything by the absolute path they opened, so a
// file scanned this way acquired a second identity that joined nothing.

func newScanModel(t *testing.T) (Model, string) {
	t.Helper()
	ws := t.TempDir()
	write := func(rel, content string) {
		full := filepath.Join(ws, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/ws\n\ngo 1.22\n")
	write("internal/a/a.go", "package a\n\nimport \"example.com/ws/internal/b\"\n\nfunc Run() { b.Do() }\n")
	write("internal/b/b.go", "package b\n\nfunc Do() {}\n")

	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(ws, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := store.NewLocalStore(filepath.Join(ws, ".nerd", "knowledge.db"))
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	vs := core.NewVirtualStore(nil)
	vs.SetLocalDB(db)

	m := NewTestModel()
	m.kernel = k
	m.workspace = ws
	m.localDB = db
	m.virtualStore = vs
	return m, ws
}

func factPathSet(t *testing.T, m Model, predicate string, idx int) map[string]int {
	t.Helper()
	facts, err := m.kernel.Query(predicate)
	if err != nil {
		t.Fatalf("Query(%s): %v", predicate, err)
	}
	out := make(map[string]int)
	for _, f := range facts {
		if len(f.Args) > idx {
			out[types.ExtractString(f.Args[idx])]++
		}
	}
	return out
}

func assertCanonicalScan(t *testing.T, m Model, ws string) {
	t.Helper()
	want := []string{"internal/a/a.go", "internal/b/b.go"}

	topo := factPathSet(t, m, "file_topology", 0)
	if len(topo) != len(want) {
		t.Fatalf("file_topology identities = %v, want exactly %v", topo, want)
	}
	for _, w := range want {
		if topo[w] != 1 {
			t.Errorf("file_topology(%q) appears %d times, want 1 (got %v)", w, topo[w], topo)
		}
	}

	for _, sym := range []struct {
		pred string
		idx  int
	}{{"symbol_graph", 3}, {"dependency_link", 0}} {
		for p := range factPathSet(t, m, sym.pred, sym.idx) {
			if strings.Contains(p, ws) || strings.Contains(p, `\`) || filepath.IsAbs(p) {
				t.Errorf("%s carries non-canonical path %q", sym.pred, p)
			}
			if _, known := topo[p]; !known {
				t.Errorf("%s carries %q, which no file_topology row identifies", sym.pred, p)
			}
		}
	}

	deps, err := m.kernel.Query("dependency_link")
	if err != nil {
		t.Fatal(err)
	}
	resolved := false
	for _, f := range deps {
		if types.ExtractString(f.Args[0]) == "internal/a/a.go" && types.ExtractString(f.Args[1]) == "internal/b/b.go" {
			resolved = true
		}
	}
	if !resolved {
		t.Errorf("import of internal/b was not resolved into a file->file dependency_link; edges: %v", deps)
	}

	rows, _, err := m.localDB.LoadWorldFactsForFile("internal/a/a.go", "fast")
	if err != nil || len(rows) == 0 {
		t.Errorf("world cache has no rows under the canonical path (err=%v): the next incremental scan could not retract this file", err)
	}

	edges := map[string]bool{}
	if _, err := m.localDB.HydrateKnowledgeGraph(func(_ string, args []any) error {
		edges[types.ExtractString(args[0])+" "+types.ExtractString(args[1])+" "+types.ExtractString(args[2])] = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !edges["internal/a/a.go /depends_on internal/b/b.go"] {
		t.Errorf("knowledge graph lacks a joinable /depends_on edge; edges: %v", edges)
	}
	if !edges["func:Run /defined_in internal/a/a.go"] {
		t.Errorf("knowledge graph lacks the /defined_in edge for Run; edges: %v", edges)
	}
}

func TestScanPath_ShouldKeyFactsByCanonicalPath(t *testing.T) {
	m, ws := newScanModel(t)

	// One relative, one absolute: both must land under the canonical identity.
	msg := m.runPartialScan([]string{"internal/a/a.go", filepath.Join(ws, "internal", "b", "b.go")})()
	res, ok := msg.(scanCompleteMsg)
	if !ok || res.err != nil {
		t.Fatalf("runPartialScan returned %#v", msg)
	}
	if res.fileCount != 2 || res.factCount == 0 {
		t.Fatalf("scan reported %d files / %d facts", res.fileCount, res.factCount)
	}
	assertCanonicalScan(t, m, ws)

	// Rescanning a file replaces its generation instead of adding a second one.
	if msg := m.runPartialScan([]string{"internal/a/a.go"})(); msg.(scanCompleteMsg).err != nil {
		t.Fatal(msg)
	}
	assertCanonicalScan(t, m, ws)
}

func TestScanDir_ShouldKeyFactsByCanonicalPath(t *testing.T) {
	m, ws := newScanModel(t)

	msg := m.runDirScan("internal")()
	res, ok := msg.(scanCompleteMsg)
	if !ok || res.err != nil {
		t.Fatalf("runDirScan returned %#v", msg)
	}
	if res.fileCount != 2 || res.directoryCount < 3 || res.factCount == 0 {
		t.Fatalf("scan reported %d files / %d dirs / %d facts", res.fileCount, res.directoryCount, res.factCount)
	}
	assertCanonicalScan(t, m, ws)
}
