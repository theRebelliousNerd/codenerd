package tactile

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Every production construction of a DirectExecutor outside this package is
// an exception to the governed VirtualStore route and must be registered in
// DirectBypassRegistry with its owner, reason, permission proof, audit sink,
// limits and review date. Adding a new one without an entry fails here.
func TestDirectBypassRegistryMatchesProductionConstructors(t *testing.T) {
	root := moduleRoot(t)
	found := productionDirectConstructions(t, root)

	registered := make(map[string]DirectBypass, len(DirectBypassRegistry))
	for _, entry := range DirectBypassRegistry {
		if _, dup := registered[entry.File]; dup {
			t.Errorf("duplicate registry entry for %s", entry.File)
		}
		registered[entry.File] = entry
		for field, value := range map[string]string{
			"Owner": entry.Owner, "Reason": entry.Reason, "PermissionProof": entry.PermissionProof,
			"AuditSink": entry.AuditSink, "Limits": entry.Limits,
		} {
			if strings.TrimSpace(value) == "" {
				t.Errorf("%s: registry entry has no %s", entry.File, field)
			}
		}
		review, err := time.Parse("2006-01-02", entry.ReviewBy)
		if err != nil {
			t.Errorf("%s: ReviewBy %q is not a YYYY-MM-DD date: %v", entry.File, entry.ReviewBy, err)
		} else if time.Now().After(review.Add(24 * time.Hour)) {
			t.Errorf("%s: direct-executor exception passed its review date %s; re-justify it or route the effect through VirtualStore", entry.File, entry.ReviewBy)
		}
	}

	for file, count := range found {
		entry, ok := registered[file]
		if !ok {
			t.Errorf("%s constructs %d direct executor(s) outside the governed VirtualStore route and is not in tactile.DirectBypassRegistry", file, count)
			continue
		}
		if entry.Constructions != count {
			t.Errorf("%s: registry says %d direct-executor construction(s), the file has %d", file, entry.Constructions, count)
		}
	}
	for file := range registered {
		if _, ok := found[file]; !ok {
			t.Errorf("stale registry entry: %s no longer constructs a direct executor", file)
		}
	}
}

// productionDirectConstructions counts tactile.NewDirectExecutor* calls per
// non-test Go file outside internal/tactile, keyed by slash path from root.
func productionDirectConstructions(t *testing.T, root string) map[string]int {
	t.Helper()
	found := make(map[string]int)
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			switch {
			case rel == "internal/tactile", strings.HasPrefix(rel, ".git"), strings.HasPrefix(rel, ".nerd"),
				strings.HasPrefix(rel, ".claude"), rel == "vendor", strings.HasPrefix(rel, "node_modules"):
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if perr != nil {
			return nil // not this gate's concern; the build catches it
		}
		alias := tactileImportName(file)
		if alias == "" {
			return nil
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if ok && pkg.Name == alias && (sel.Sel.Name == "NewDirectExecutor" || sel.Sel.Name == "NewDirectExecutorWithConfig") {
				found[rel]++
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk module: %v", err)
	}
	return found
}

func tactileImportName(file *ast.File) string {
	for _, imp := range file.Imports {
		if strings.Trim(imp.Path.Value, `"`) != "codenerd/internal/tactile" {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return "tactile"
	}
	return ""
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test directory")
		}
		dir = parent
	}
}
