package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// element_edit_blocked/2 and edit_unsafe/2 each have two producers: the rules in
// defaults/policy/codedom_edit.mg, and Go (VirtualStore.handleEditElement and
// world.FileScope respectively). The Go side asserted its Reason as a bare
// string while the rules emitted a name constant, so the two halves of each
// relation never unified — element_edit_blocked(_, /concurrent_modification) was
// written by both and matched by neither's consumer. Nothing errored; the rows
// were simply invisible to any bound query.
//
// Both Decls now bind Reason to /name. This test pins the MANGLE half: the
// derived row and the asserted row must come back from one bound query, and the
// pre-fix string form must NOT.
//
// It does NOT pin the Go half, and it used to claim it did. The fixture below
// hardcodes MangleAtom, so reverting either Go producer to a bare string leaves
// this green — an adversarial pass reverted all three and it never noticed.
// TestCodedomReasonProducersEmitAtoms is the Go half; the two together are the
// claim this file's header used to make on its own.
func TestCodedomReasonAtomsUnify(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	facts := []Fact{
		// EDB that drives the .mg rules in policy/codedom_edit.mg.
		{Predicate: "code_element", Args: []any{"fn:demo.Rule", MangleAtom("/function"), "demo.go", int64(1), int64(5)}},
		{Predicate: "file_modified_externally", Args: []any{"demo.go"}},
		{Predicate: "code_element", Args: []any{"fn:gen.Rule", MangleAtom("/function"), "gen.go", int64(1), int64(3)}},
		{Predicate: "generated_code", Args: []any{"gen.go", MangleAtom("/protobuf"), "// Code generated"}},

		// Exactly what the fixed Go producers now emit.
		{Predicate: "element_edit_blocked", Args: []any{"fn:demo.Go", MangleAtom("/concurrent_modification")}},
		{Predicate: "element_edit_blocked", Args: []any{"fn:demo.Go", MangleAtom("/hash_verification_failed")}},
		{Predicate: "edit_unsafe", Args: []any{"gen.go", MangleAtom("/generated_code")}},

		// What the Go producers emitted BEFORE the fix, kept as a control.
		{Predicate: "element_edit_blocked", Args: []any{"fn:demo.Old", "concurrent_modification"}},
		{Predicate: "edit_unsafe", Args: []any{"old.go", "generated_code_will_be_overwritten"}},
	}
	if err := k.LoadFacts(facts); err != nil {
		t.Fatalf("LoadFacts error = %v", err)
	}

	check := func(query string, want int) {
		t.Helper()
		rows, qErr := k.Query(query)
		if qErr != nil {
			t.Fatalf("Query(%s) error = %v", query, qErr)
		}
		if len(rows) != want {
			t.Errorf("Query(%s) got %d rows, want %d: %v", query, len(rows), want, rows)
		}
	}

	// The rule-derived row and the Go-asserted row now share one relation.
	check("element_edit_blocked(R, /concurrent_modification)", 2)
	check("element_edit_blocked(R, /hash_verification_failed)", 1)
	check("edit_unsafe(R, /generated_code)", 2)

	// Control: the pre-fix string form is a different value and is invisible to
	// the same bound query. This is the bug that was being closed.
	check(`element_edit_blocked(R, "concurrent_modification")`, 1)
	check(`edit_unsafe(R, "generated_code_will_be_overwritten")`, 1)

	// Unbound consumers (virtual_store.go clearCodeDOMFacts, dom_cmd.go counts)
	// still see every row regardless of constant type.
	// 2 Go-asserted + 2 rule-derived (/concurrent_modification on fn:demo.Rule,
	// /generated_code on fn:gen.Rule) + 1 pre-fix string control.
	check("element_edit_blocked", 5)

	// Kernel still reaches a fixpoint: safe_action must be non-empty.
	sa, err := k.Query("safe_action")
	if err != nil {
		t.Fatalf("Query(safe_action) error = %v", err)
	}
	t.Logf("safe_action rows = %d", len(sa))
	if len(sa) == 0 {
		t.Fatal("safe_action derived 0 rows — analysis is broken")
	}
}

// TestCodedomReasonProducersEmitAtoms is the half the derivation test above
// cannot cover.
//
// A test that seeds its own facts proves what the RULES do with a value; it
// says nothing about what the producers actually emit, because the fixture
// supplies the value. Both sides have to be pinned separately or the pair can
// drift apart silently — which is exactly the failure the /name-vs-/string bug
// class is made of.
//
// This is an AST check, not a grep: it finds the types.Fact composite literals
// for these predicates and requires the reason argument to be a MangleAtom
// conversion rather than a string literal. A comment or an unrelated mention
// cannot satisfy it.
func TestCodedomReasonProducersEmitAtoms(t *testing.T) {
	root := codedomRepoRoot(t)

	cases := []struct {
		file      string
		predicate string
		argIndex  int
	}{
		{"internal/core/virtual_store_codedom.go", "element_edit_blocked", 1},
		{"internal/world/scope.go", "edit_unsafe", 1},
	}

	for _, tc := range cases {
		t.Run(tc.predicate, func(t *testing.T) {
			path := filepath.Join(root, filepath.FromSlash(tc.file))
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", tc.file, err)
			}

			sites := 0
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				pred, args := factLiteralParts(lit)
				if pred != tc.predicate || args == nil || tc.argIndex >= len(args.Elts) {
					return true
				}
				sites++
				if !isMangleAtomConversion(args.Elts[tc.argIndex]) {
					t.Errorf("%s:%d: %s emits a reason that is not a MangleAtom. "+
						"Reason is declared /name; a bare Go string lands as a string "+
						"constant and is invisible to every rule matching the atom form.",
						tc.file, fset.Position(lit.Pos()).Line, tc.predicate)
				}
				return true
			})
			if sites == 0 {
				t.Fatalf("found no %s fact literal in %s — the producer moved and this "+
					"guard is now inert", tc.predicate, tc.file)
			}
			t.Logf("%s: %d %s producer site(s) checked", tc.file, sites, tc.predicate)
		})
	}
}

// factLiteralParts pulls the Predicate string and the Args composite out of a
// types.Fact / core.Fact literal, or returns "" and nil if it is not one.
func factLiteralParts(lit *ast.CompositeLit) (string, *ast.CompositeLit) {
	var pred string
	var args *ast.CompositeLit
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Predicate":
			if bl, ok := kv.Value.(*ast.BasicLit); ok && bl.Kind == token.STRING {
				if v, err := strconv.Unquote(bl.Value); err == nil {
					pred = v
				}
			}
		case "Args":
			if cl, ok := kv.Value.(*ast.CompositeLit); ok {
				args = cl
			}
		}
	}
	return pred, args
}

// isMangleAtomConversion reports whether expr is MangleAtom(...) or
// types.MangleAtom(...) / core.MangleAtom(...).
func isMangleAtomConversion(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name == "MangleAtom"
	case *ast.SelectorExpr:
		return fn.Sel.Name == "MangleAtom"
	}
	return false
}

// codedomRepoRoot walks up from the package directory to the module root.
func codedomRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not locate the module root from the package directory")
	return ""
}
