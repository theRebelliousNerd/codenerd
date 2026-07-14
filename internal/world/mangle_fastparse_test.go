package world

import (
	"reflect"
	"testing"

	"codenerd/internal/core"
)

func TestExtractMangleSymbolFacts(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		content string
		want    []core.Fact
	}{
		{
			name: "basic declarations and rules",
			path: "test.mg",
			content: `
Decl foo(string, string).
foo(A, B) :- bar(A, B).
?foo(A, B).
baz(X).
`,
			want: []core.Fact{
				{Predicate: "symbol_graph", Args: []any{"pred:foo/2", "/predicate", "/public", "test.mg", "foo/2"}},
				{Predicate: "symbol_graph", Args: []any{"pred:baz/1", "/predicate", "/public", "test.mg", "baz/1"}},
			},
		},
		{
			name:    "empty input",
			path:    "empty.mg",
			content: ``,
			want:    []core.Fact{},
		},
		{
			name: "duplicate declarations",
			path: "dup.mg",
			content: `
foo(A).
foo(B).
Decl foo(int).
`,
			want: []core.Fact{
				{Predicate: "symbol_graph", Args: []any{"pred:foo/1", "/predicate", "/public", "dup.mg", "foo/1"}},
			},
		},
		{
			name: "invalid syntax",
			path: "invalid.mg",
			content: `
().
.
`,
			want: []core.Fact{},
		},
		{
			name: "comments",
			path: "comments.mg",
			content: `
# This is a comment
Decl foo(string, string).

bar(X).
`,
			want: []core.Fact{
				{Predicate: "symbol_graph", Args: []any{"pred:foo/2", "/predicate", "/public", "comments.mg", "foo/2"}},
				{Predicate: "symbol_graph", Args: []any{"pred:bar/1", "/predicate", "/public", "comments.mg", "bar/1"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMangleSymbolFacts(tt.path, tt.content)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractMangleSymbolFacts() = %v, want %v", got, tt.want)
			}
		})
	}
}
