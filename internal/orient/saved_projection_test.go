package orient

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"codenerd/internal/config"
)

// A stale refresh reads the pre-refresh projection back from the file the last
// Save wrote instead of re-deriving it. The rows must be the ones the engine
// derives, compared the way the refresh diff compares them.
func TestSavedProjectionFactsMatchTheEngineProjection(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "--quiet")
	for rel, body := range map[string]string{
		"README.md":       "# Fixture\n\nThe project vision lives here.\n",
		"docs/DESIGN.md":  "# Design\n\nSee [the readme](../README.md).\n",
		"main.go":         "package main\n\nfunc main() {}\n",
		"AGENTS.md":       "# Agents\n\nRun the tests.\n",
		"docs/ROADMAP.md": "# Roadmap\n\nNext steps.\n",
	} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "--quiet", "-m", "fixture")

	ctx := context.Background()
	snap, err := Measure(ctx, root, config.OrientConfig{}.WithDefaults(), nil)
	if err != nil {
		t.Fatalf("Measure: %v", err)
	}
	eng, err := snap.Engine(ctx)
	if err != nil {
		t.Fatalf("Engine: %v", err)
	}
	defer eng.Close()
	if err := snap.Save(root, eng); err != nil {
		t.Fatalf("Save: %v", err)
	}
	derived, err := projectionFacts(eng)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := savedProjectionFacts(root)
	if err != nil {
		t.Fatalf("savedProjectionFacts: %v", err)
	}
	key := func(rows []string) []string { sort.Strings(rows); return rows }
	var d, s []string
	for _, r := range derived {
		d = append(d, r.String())
	}
	for _, r := range saved {
		s = append(s, r.String())
	}
	d, s = key(d), key(s)
	if len(d) == 0 {
		t.Fatal("fixture derived no projection rows")
	}
	if len(d) != len(s) {
		t.Fatalf("saved projection has %d rows, the engine derives %d", len(s), len(d))
	}
	for i := range d {
		if d[i] != s[i] {
			t.Fatalf("row %d differs: saved %q, derived %q", i, s[i], d[i])
		}
	}
}
