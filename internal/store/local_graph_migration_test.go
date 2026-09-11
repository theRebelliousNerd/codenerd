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
	// Absolute-path file identities in scanner edges never join anything
	// keyed by the canonical path; the live store held thousands of them.
	for _, e := range [][3]string{
		{"pred:panic_state/2", "defined_in", `C:\ws\internal\core\debug_program_ERROR.mg`},
		{"func:Run", "defined_in", "C:/ws/internal/a/a.go"},
		{"func:Do", "defined_in", "/home/steve/ws/internal/b/b.go"},
		{`C:\ws\internal\a\a.go`, "depends_on", "pkg:fmt"},
		// A non-scanner relation with an absolute path is not ours to drop.
		{"C:/ws/docs", "/has_file", "docs/readme.md"},
	} {
		if err := db.StoreLink(e[0], e[1], e[2], 1.0, nil); err != nil {
			t.Fatalf("StoreLink(%v): %v", e, err)
		}
	}
	db.Close()

	db, err = NewLocalStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db.Close()

	var edges []string
	if _, err := db.HydrateKnowledgeGraph(func(_ string, args []any) error {
		edges = append(edges, args[0].(string)+" "+args[1].(string)+" "+args[2].(string))
		return nil
	}); err != nil {
		t.Fatalf("HydrateKnowledgeGraph: %v", err)
	}
	want := map[string]bool{
		"internal/a/a.go /depends_on internal/b/b.go": false,
		"internal/a/a.go /defined_in internal/b/b.go": false,
		"C:/ws/docs /has_file docs/readme.md":         false,
	}
	for _, e := range edges {
		if _, ok := want[e]; !ok {
			t.Errorf("edge %q survived the reopen; it can never join a canonical file identity", e)
			continue
		}
		want[e] = true
	}
	for e, seen := range want {
		if !seen {
			t.Errorf("edge %q was dropped but should have been kept", e)
		}
	}
}
