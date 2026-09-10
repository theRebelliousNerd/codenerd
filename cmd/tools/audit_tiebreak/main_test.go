package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// The shape, and the neighbours that must not be confused with it.
func TestSelectsKeyByComparison(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{
			name: "the defect: the key becomes the running best",
			body: `for name, n := range m { if n > max { max = n; best = name } }`,
			want: true,
		},
		{
			name: "same shape with a less-than",
			body: `for name, n := range m { if n < min { min = n; best = name } }`,
			want: true,
		},
		{
			name: "multi-assign form",
			body: `for name, n := range m { if n > max { max, best = n, name } }`,
			want: true,
		},

		// Not the defect.
		{
			name: "the VALUE is recorded, not the key",
			body: `for _, v := range m { if v > max { max = v } }`,
			want: false,
		},
		{
			name: "a size limit, not a selection",
			body: `for _, v := range m { if len(v) > maxBytes { return errTooBig } }`,
			want: false,
		},
		{
			name: "equality, not an ordering",
			body: `for name, n := range m { if n == want { best = name } }`,
			want: false,
		},
		{
			name: "a blank key cannot be selected",
			body: `for _, n := range m { if n > max { max = n } }`,
			want: false,
		},
		{
			name: "no comparison at all",
			body: `for name, n := range m { total += n; last = name }`,
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go",
				"package fixture\nfunc probe(){"+tc.body+"}", 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := false
			ast.Inspect(file, func(n ast.Node) bool {
				if rng, ok := n.(*ast.RangeStmt); ok && selectsKeyByComparison(rng) {
					got = true
					return false
				}
				return true
			})
			if got != tc.want {
				t.Errorf("selectsKeyByComparison = %v, want %v for: %s", got, tc.want, tc.body)
			}
		})
	}
}

// The key must survive ordinary edits, or the baseline churns and stops being
// read. It carries the package, the enclosing function with its receiver, and
// the range variable — never a line number.
func TestEnclosingFuncNamesTheReceiver(t *testing.T) {
	src := `package fixture
func plain() { for k, v := range m { if v > max { max = v; best = k } } }
func (s *Store) method() { for k, v := range m { if v > max { max = v; best = k } } }
`
	file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	ast.Inspect(file, func(n ast.Node) bool {
		if rng, ok := n.(*ast.RangeStmt); ok && selectsKeyByComparison(rng) {
			names = append(names, enclosingFunc(file, rng.Pos()))
		}
		return true
	})
	if len(names) != 2 || names[0] != "plain" || names[1] != "Store.method" {
		t.Errorf("enclosing functions = %v, want [plain Store.method]", names)
	}
}
