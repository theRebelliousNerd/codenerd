package diff

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// displayOnlyImporters are the production packages allowed to import this
// one. Both render diffs for a reader -- the TUI's approval view and the repair
// round's prompt (internal/session/turn_diff.go) -- and neither applies a hunk
// to a file.
//
// That is what makes the cache's trusted keys an accepted risk rather than a
// defect (GAP-DIFF-01, Docs/architecture/diff/TODO.md): both production
// engines run with Options.VerifyCacheContent off, so a dual-FNV-1a key
// collision would show one file's hunks for another's. Shown wrongly is a
// display fault; applied wrongly is a corrupted file. A new importer that
// applies diffs must turn VerifyCacheContent on for its engine (its collision
// path is proven by TestCache_WhenVerifyEnabledAndKeyCollides_ShouldRecomputeRatherThanServeWrongDiff),
// and then be added here.
var displayOnlyImporters = map[string]bool{
	"cmd/nerd/ui":      true,
	"internal/session": true,
}

func TestDiffImporters_AreDisplayOnly(t *testing.T) {
	root := repoRoot(t)
	found := map[string]bool{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".nerd", ".claude", "node_modules", "vendor", "third_party", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			return nil
		}
		for _, imp := range f.Imports {
			if p, _ := strconv.Unquote(imp.Path.Value); p == "codenerd/internal/diff" {
				rel, _ := filepath.Rel(root, filepath.Dir(path))
				found[filepath.ToSlash(rel)] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 {
		t.Fatal("found no importer of internal/diff; the walk is asserting nothing")
	}
	var unexpected []string
	for pkg := range found {
		if !displayOnlyImporters[pkg] {
			unexpected = append(unexpected, pkg)
		}
	}
	sort.Strings(unexpected)
	for _, pkg := range unexpected {
		t.Errorf("%s imports internal/diff and is not a known display-only consumer: if it applies diffs, "+
			"build its engine with Options{VerifyCacheContent: true} (cache keys are trusted, not proven, by default), "+
			"then add it to displayOnlyImporters", pkg)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the test directory")
		}
		dir = parent
	}
}
