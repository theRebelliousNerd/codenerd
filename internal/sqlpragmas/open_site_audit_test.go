package sqlpragmas

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This file is the "periodic audit" from Docs/architecture/sqlpragmas/TODO.md
// P1, expressed as a test instead of a calendar reminder. A periodic audit
// nobody schedules is a rule that decays the day it is written; a test fails
// on the commit that breaks it.
//
// Every production sql.Open / sql.OpenDB in the repo must be classified:
// either its enclosing function applies pragmas (ApplyDefaultPragmas, or the
// connector hook via NewConnector / OpenWithPragmas), or the file appears in
// unpragmaedOpens with a reason. A new untuned open fails this test; a STALE
// exemption (the file stopped opening SQLite, or adopted pragmas) also fails,
// so the inventory cannot quietly rot.
//
// Design mirrors internal/build/go_invocation_inventory_test.go, which does the
// same for exec.Command("go", …).
//
// Scope: non-test .go files. Test files open scratch databases whose tuning is
// irrelevant, and holding every fixture to this rule would only teach people to
// work around it.

// unpragmaedOpens maps a repo-relative file path to the reason its SQLite
// opens do not apply a pragma profile.
//
// Empty is the correct state and the current one: every one of the ~32
// production open sites applies a profile. An entry here is a debt marker, not
// a resting place.
var unpragmaedOpens = map[string]string{}

// storeFacadeCallers pins the packages allowed to reach pragmas through
// store.ApplyDefaultPragmas rather than importing this leaf directly.
//
// TODO.md P1 says new mid-layer packages should prefer the sqlpragmas import so
// they do not pick up a dependency on internal/store (which is what forced this
// leaf to exist). "Prefer" is unenforceable prose, so it is a pin: these four
// predate the leaf, and a fifth entry fails until it either imports sqlpragmas
// or justifies itself here.
// internal/store itself is absent by construction: its own opens call the
// unqualified re-exported name, so they never read as a façade call.
var storeFacadeCallers = map[string]string{
	"cmd/query-kb":                       "CLI already imports store for the query helpers it exists to run",
	"cmd/tools/prompt_builder":           "builder already imports store for schema constants",
	"cmd/tools/predicate_corpus_builder": "builder already imports store for schema constants",
}

// pragmaMarkers are the identifiers whose presence in a function means the
// opens in that function end up tuned.
var pragmaMarkers = map[string]bool{
	"ApplyDefaultPragmas": true, // direct apply, leaf or store re-export
	"NewConnector":        true, // connector hook: every pooled conn is tuned
	"OpenWithPragmas":     true, // sql.Open + connector hook in one call
}

type sqlOpenSite struct {
	file      string // repo-relative, slash-separated
	dir       string // repo-relative package dir
	line      int
	fn        string
	pragmaed  bool
	viaFacade bool // reached pragmas through store.ApplyDefaultPragmas
}

func TestSQLOpenSites_WhenOpeningSQLite_ShouldApplyPragmasOrBeExempt(t *testing.T) {
	root := repoRootForAudit(t)
	sites := scanSQLOpenSites(t, root)

	if len(sites) == 0 {
		t.Fatal("scanner found no sql.Open sites at all — the scanner is broken, not the repo")
	}

	byFile := map[string][]sqlOpenSite{}
	for _, s := range sites {
		byFile[s.file] = append(byFile[s.file], s)
	}
	files := make([]string, 0, len(byFile))
	for f := range byFile {
		files = append(files, f)
	}
	sort.Strings(files)

	// Emit the inventory so `go test -v ./internal/sqlpragmas/...` prints the
	// current, authoritative answer to "who opens SQLite, and how is it tuned?".
	for _, file := range files {
		status := "applies pragmas"
		if reason, ok := unpragmaedOpens[file]; ok {
			status = reason
		} else if !byFile[file][0].pragmaed {
			status = "UNTUNED"
		} else if byFile[file][0].viaFacade {
			status = "applies pragmas (store façade)"
		}
		locs := make([]string, 0, len(byFile[file]))
		for _, s := range byFile[file] {
			locs = append(locs, s.fn+":"+strconv.Itoa(s.line))
		}
		t.Logf("%-52s %-40s [%s]", file, strings.Join(locs, " "), status)
	}

	var untuned []string
	filesWithUntuned := map[string]bool{}
	for _, s := range sites {
		if s.pragmaed {
			continue
		}
		filesWithUntuned[s.file] = true
		if _, ok := unpragmaedOpens[s.file]; !ok {
			untuned = append(untuned, s.file+":"+strconv.Itoa(s.line)+" ("+s.fn+")")
		}
	}
	sort.Strings(untuned)

	if len(untuned) > 0 {
		t.Errorf("SQLite opens with no pragma profile:\n  %s\n\n"+
			"Fix one of three ways:\n"+
			"  1. call sqlpragmas.ApplyDefaultPragmas(db, <profile>) right after the open, or\n"+
			"  2. open via sqlpragmas.OpenWithPragmas(driver, dsn, <profile>) so every pooled connection is tuned, or\n"+
			"  3. add the file to unpragmaedOpens in this file with a real reason.",
			strings.Join(untuned, "\n  "))
	}

	for file, reason := range unpragmaedOpens {
		if !filesWithUntuned[file] {
			t.Errorf("stale exemption for %q (%s): it no longer has an untuned SQLite open. Remove the entry from unpragmaedOpens.", file, reason)
		}
	}
}

// TestPragmaSurface_WhenNewPackageAppliesPragmas_ShouldPreferTheLeaf enforces
// TODO.md P1 "prefer sqlpragmas import in new mid-layer packages that must not
// touch store". A package that reaches pragmas through store.ApplyDefaultPragmas
// has taken a dependency on the very package this leaf was split out to avoid.
func TestPragmaSurface_WhenNewPackageAppliesPragmas_ShouldPreferTheLeaf(t *testing.T) {
	root := repoRootForAudit(t)

	seen := map[string]bool{}
	for _, s := range scanSQLOpenSites(t, root) {
		if s.viaFacade {
			seen[s.dir] = true
		}
	}

	for dir := range seen {
		if _, ok := storeFacadeCallers[dir]; !ok {
			t.Errorf("package %q reaches pragmas through store.ApplyDefaultPragmas: import codenerd/internal/sqlpragmas directly instead "+
				"(the leaf exists so mid-layer packages need not depend on internal/store), or pin it in storeFacadeCallers with a reason", dir)
		}
	}
	for dir, reason := range storeFacadeCallers {
		if !seen[dir] {
			t.Errorf("stale storeFacadeCallers entry %q (%s): it no longer calls store.ApplyDefaultPragmas at an open site. Remove it.", dir, reason)
		}
	}
}

var auditSkipDirs = map[string]bool{
	".git": true, "vendor": true, "node_modules": true, "testdata": true,
	".nerd": true, "sqlite_headers": true,
}

func scanSQLOpenSites(t *testing.T, root string) []sqlOpenSite {
	t.Helper()

	var sites []sqlOpenSite
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable trees are not this test's problem
		}
		if d.IsDir() {
			if auditSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil // a package mid-edit by another worker should not fail this test
		}

		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		dir := filepath.ToSlash(filepath.Dir(rel))

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			lines := sqlOpenLines(fset, fn)
			if len(lines) == 0 {
				continue
			}
			pragmaed, facade := functionPragmaSurface(fn)
			for _, line := range lines {
				sites = append(sites, sqlOpenSite{
					file:      rel,
					dir:       dir,
					line:      line,
					fn:        fn.Name.Name,
					pragmaed:  pragmaed,
					viaFacade: facade,
				})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return sites
}

// sqlOpenLines returns the source lines of every sql.Open / sql.OpenDB call in fn.
func sqlOpenLines(fset *token.FileSet, fn *ast.FuncDecl) []int {
	var lines []int
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "sql" {
			return true
		}
		if sel.Sel.Name != "Open" && sel.Sel.Name != "OpenDB" {
			return true
		}
		lines = append(lines, fset.Position(call.Pos()).Line)
		return true
	})
	return lines
}

// functionPragmaSurface reports whether fn applies a pragma profile anywhere in
// its body, and whether it did so through the store re-export.
func functionPragmaSurface(fn *ast.FuncDecl) (pragmaed, viaFacade bool) {
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.SelectorExpr:
			if !pragmaMarkers[node.Sel.Name] {
				return true
			}
			pragmaed = true
			if pkg, ok := node.X.(*ast.Ident); ok && pkg.Name == "store" {
				viaFacade = true
			}
			return false
		case *ast.Ident:
			// Bare call: inside package store, or inside this package.
			if pragmaMarkers[node.Name] {
				pragmaed = true
			}
		}
		return true
	})
	return pragmaed, viaFacade
}

// repoRootForAudit walks up from the test's working directory to the module root.
func repoRootForAudit(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if data, rerr := os.ReadFile(filepath.Join(dir, "go.mod")); rerr == nil {
			if strings.HasPrefix(string(data), "module codenerd") {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate the codenerd module root")
		}
		dir = parent
	}
}
