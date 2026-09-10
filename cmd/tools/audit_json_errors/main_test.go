package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"
)

// The two ways an error gets discarded, and the neighbours that must not be
// confused with them. The quiet one is the bare ExprStmt: there is no `_` in
// the source, so nothing marks it as a decision.
func TestDiscarded(t *testing.T) {
	for _, tc := range []struct {
		body string
		want string
	}{
		{`b, _ := json.Marshal(v)`, "Marshal"},
		{`b, _ = json.Marshal(v)`, "Marshal"},
		{`b, _ := json.MarshalIndent(v, "", "  ")`, "MarshalIndent"},
		{`json.Unmarshal(b, &v)`, "Unmarshal"},

		// Handled, so not findings.
		{`b, err := json.Marshal(v)`, ""},
		{`if err := json.Unmarshal(b, &v); err != nil { return err }`, ""},

		// Discarding the VALUE and keeping the error is fine, and is not the
		// same statement shape -- the blank has to be in the error position.
		{`_, err := json.Marshal(v)`, ""},

		// Other packages and other functions in this one.
		{`b, _ := yaml.Marshal(v)`, ""},
		{`b, _ := json.NewEncoder(w)`, ""},
		{`b, _ := proto.Marshal(v)`, ""},
	} {
		file, err := parser.ParseFile(token.NewFileSet(), "fixture.go",
			"package fixture\nfunc probe(){"+tc.body+"}", 0)
		if err != nil {
			t.Fatalf("%s: %v", tc.body, err)
		}
		var got string
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(ast.Stmt)
			if !ok {
				return true
			}
			if fn, _, ok := discarded(stmt); ok {
				got = fn
				return false
			}
			return true
		})
		if got != tc.want {
			t.Errorf("discarded(%q) = %q, want %q", tc.body, got, tc.want)
		}
	}
}

// The key is what makes the baseline survive ordinary edits. It carries the
// package, the enclosing function including its receiver, and the argument
// text -- but never a line number, so inserting code above a call does not
// report it as new.
func TestEnclosingFuncNamesTheReceiver(t *testing.T) {
	src := `package fixture
func plain() { b, _ := json.Marshal(v); _ = b }
func (s *LocalStore) method() { b, _ := json.Marshal(v); _ = b }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	ast.Inspect(file, func(n ast.Node) bool {
		stmt, ok := n.(ast.Stmt)
		if !ok {
			return true
		}
		if _, _, ok := discarded(stmt); ok {
			names = append(names, enclosingFunc(file, stmt.Pos()))
		}
		return true
	})
	want := []string{"plain", "LocalStore.method"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("enclosing functions = %v, want %v", names, want)
	}
}

// diff compares multisets, not sets. Two identical calls in one function are
// two entries, so replacing a fixed one with a fresh copy cannot slip through
// as a no-op.
func TestDiffCountsDuplicates(t *testing.T) {
	base := []string{"p\tf\tjson.Marshal(v)", "p\tf\tjson.Marshal(v)"}

	if added, removed := diff(base, base); len(added) != 0 || len(removed) != 0 {
		t.Errorf("identical multisets differ: added=%v removed=%v", added, removed)
	}

	added, removed := diff(base, base[:1])
	if len(added) != 0 || len(removed) != 1 {
		t.Errorf("losing one of two duplicates: added=%v removed=%v", added, removed)
	}

	added, removed = diff(base[:1], base)
	if len(added) != 1 || len(removed) != 0 {
		t.Errorf("gaining a duplicate: added=%v removed=%v", added, removed)
	}

	added, removed = diff([]string{"a"}, []string{"b"})
	if len(added) != 1 || len(removed) != 1 {
		t.Errorf("a swap should report both sides: added=%v removed=%v", added, removed)
	}
}
