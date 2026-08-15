package sqlpragmas

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TODO.md P0 says "when adding a profile, update this corpus". Corpus docs
// drift the moment that sentence is the only thing enforcing it: the constant
// lands, the PR is reviewed for code, and IMPLEMENTED_SPEC.md keeps describing
// four profiles forever.
//
// These tests read the package's own AST and require every exported name to be
// named in the corpus. Adding a profile — or any exported symbol — fails here
// until the docs say what it is. The check is presence-of-name only: it cannot
// tell whether the prose is correct, but it does guarantee that whoever added
// the symbol had to open the file and write a sentence about it.

// corpusDocs are the files an exported symbol must appear in.
var corpusDocs = []string{
	"Docs/architecture/sqlpragmas/IMPLEMENTED_SPEC.md",
	"Docs/architecture/sqlpragmas/06-PUBLIC-API-AND-TYPES.md",
}

func TestProfileConstants_WhenProfileAdded_ShouldAppearInCorpus(t *testing.T) {
	root := repoRootForAudit(t)
	profiles := constantsOfType(t, packageDir(t), "PragmaProfile")

	if len(profiles) == 0 {
		t.Fatal("found no PragmaProfile constants — the scanner is broken, not the package")
	}
	t.Logf("profile constants: %s", strings.Join(profiles, " "))

	for _, doc := range corpusDocs {
		body := readCorpusDoc(t, root, doc)
		for _, name := range profiles {
			if !strings.Contains(body, name) {
				t.Errorf("profile constant %s is not mentioned in %s.\n"+
					"Adding a profile means updating the corpus in the same change: describe its workload, "+
					"cache/mmap budget and which pragmas it omits.", name, doc)
			}
		}
	}
}

func TestHostClassConstants_WhenHostClassAdded_ShouldAppearInCorpus(t *testing.T) {
	root := repoRootForAudit(t)
	classes := constantsOfType(t, packageDir(t), "HostClass")

	if len(classes) == 0 {
		t.Fatal("found no HostClass constants — the scanner is broken, not the package")
	}

	for _, doc := range corpusDocs {
		body := readCorpusDoc(t, root, doc)
		for _, name := range classes {
			if !strings.Contains(body, name) {
				t.Errorf("host class %s is not mentioned in %s: document the budget divisor it applies.", name, doc)
			}
		}
	}
}

// TestExportedAPI_WhenSymbolAdded_ShouldAppearInPublicAPIDoc covers the "update
// this corpus (IMPLEMENTED_SPEC, API, failure modes)" clause for everything
// that is not a profile: a new exported helper is a new thing call sites can
// reach for, and an undocumented one gets reinvented locally instead.
func TestExportedAPI_WhenSymbolAdded_ShouldAppearInPublicAPIDoc(t *testing.T) {
	root := repoRootForAudit(t)
	const doc = "Docs/architecture/sqlpragmas/06-PUBLIC-API-AND-TYPES.md"

	exported := exportedTopLevelNames(t, packageDir(t))
	if len(exported) == 0 {
		t.Fatal("found no exported symbols — the scanner is broken, not the package")
	}
	t.Logf("exported surface (%d): %s", len(exported), strings.Join(exported, " "))

	body := readCorpusDoc(t, root, doc)
	var missing []string
	for _, name := range exported {
		if !strings.Contains(body, name) {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("exported symbols missing from %s: %s\n"+
			"Document them there (signature + contract) in the same change that adds them.",
			doc, strings.Join(missing, ", "))
	}
}

func readCorpusDoc(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read corpus doc %s: %v", rel, err)
	}
	return string(data)
}

// constantsOfType returns the names of package-level constants declared with an
// explicit type name, including the iota block members that inherit it, in
// declaration order — which for an iota block is value order.
func constantsOfType(t *testing.T, pkgDir, typeName string) []string {
	t.Helper()

	var names []string
	forEachProductionFile(t, pkgDir, func(_ string, file *ast.File) {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			// Within one const block, the type declared on the first spec
			// carries to the following specs (the iota idiom).
			inBlock := false
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				if id, ok := vs.Type.(*ast.Ident); ok {
					inBlock = id.Name == typeName
				} else if vs.Type != nil {
					inBlock = false
				}
				if !inBlock {
					continue
				}
				for _, n := range vs.Names {
					if n.IsExported() {
						names = append(names, n.Name)
					}
				}
			}
		}
	})

	return names
}

// exportedTopLevelNames returns exported package-level types, funcs, consts and
// vars. Methods are excluded: they are reached through their receiver type,
// which is itself checked.
func exportedTopLevelNames(t *testing.T, pkgDir string) []string {
	t.Helper()

	var names []string
	forEachProductionFile(t, pkgDir, func(_ string, file *ast.File) {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.IsExported() {
					names = append(names, d.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if s.Name.IsExported() {
							names = append(names, s.Name.Name)
						}
					case *ast.ValueSpec:
						for _, n := range s.Names {
							if n.IsExported() {
								names = append(names, n.Name)
							}
						}
					}
				}
			}
		}
	})

	sort.Strings(names)
	return names
}

func forEachProductionFile(t *testing.T, pkgDir string, fn func(name string, file *ast.File)) {
	t.Helper()

	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, perr := parser.ParseFile(fset, filepath.Join(pkgDir, name), nil, parser.SkipObjectResolution)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
		}
		fn(name, file)
	}
}
