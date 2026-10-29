package world

import (
	"strings"
	"testing"
)

// TestParseGo_ShouldEmitAPackageSymbol is the regression test for a branch that
// existed, compiled, was walked on every Go file the world model has ever
// scanned, and emitted nothing.
//
// The package_clause case asked ChildByFieldName("package_identifier"). In
// tree-sitter, package_identifier is a node TYPE; the Go grammar binds no field
// name to it. ChildByFieldName for a type name is not an error — it is a silent
// nil — so the branch was skipped every time and no symbol_graph package fact
// was ever produced. Nothing failed, no log said anything, and the symbol graph
// was quietly missing an entire symbol kind.
//
// This is also the shape the world decl-conformance guards cannot see: they
// check the facts that ARE emitted against their Decl, and a branch that emits
// nothing has no facts to check.
func TestParseGo_ShouldEmitAPackageSymbol(t *testing.T) {
	p := NewTreeSitterParser()
	defer p.Close()

	src := "package cartography\n\nfunc Exported() {}\n\nfunc unexported() {}\n"
	facts, err := p.ParseGo("a.go", []byte(src))
	if err != nil {
		t.Fatalf("ParseGo: %v", err)
	}

	var pkg *Fact
	for i := range facts {
		if facts[i].Predicate != "symbol_graph" || len(facts[i].Args) != 5 {
			continue
		}
		if kind, _ := facts[i].Args[1].(string); kind == "/package" {
			pkg = &facts[i]
			break
		}
	}
	if pkg == nil {
		var got []string
		for _, f := range facts {
			got = append(got, f.String())
		}
		t.Fatalf("no /package symbol_graph fact emitted; got:\n  %s", strings.Join(got, "\n  "))
	}

	if id, _ := pkg.Args[0].(string); id != "package:cartography" {
		t.Errorf("package symbol id = %v, want %q", pkg.Args[0], "package:cartography")
	}
	if vis, _ := pkg.Args[2].(string); vis != "/public" {
		t.Errorf("package visibility = %v, want /public", pkg.Args[2])
	}
	if file, _ := pkg.Args[3].(string); file != "a.go" {
		t.Errorf("package file = %v, want a.go", pkg.Args[3])
	}
	if sig, _ := pkg.Args[4].(string); sig != "package cartography" {
		t.Errorf("package signature = %v, want %q", pkg.Args[4], "package cartography")
	}

	// The functions were already being extracted; assert them so a fix to the
	// package branch that broke the walk would not pass.
	var funcs int
	for _, f := range facts {
		if f.Predicate == "symbol_graph" && len(f.Args) == 5 {
			if kind, _ := f.Args[1].(string); kind == "/function" {
				funcs++
			}
		}
	}
	if funcs != 2 {
		t.Errorf("extracted %d /function symbols, want 2 — the package fix must not disturb the walk", funcs)
	}
}
