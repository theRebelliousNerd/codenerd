package world

import (
	"codenerd/internal/core"
	"testing"
)

func TestWorldFactPathArg(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{name: "atom function", input: "/function", want: ""},
		{name: "atom method", input: "/method", want: ""},
		{name: "atom struct", input: "/struct", want: ""},
		{name: "atom public", input: "/public", want: ""},
		{name: "real path relative", input: "internal/world/ast.go", want: "internal/world/ast.go"},
		{name: "real path windows", input: `C:\CodeProjects\codeNERD\internal\x.go`, want: `C:\CodeProjects\codeNERD\internal\x.go`},
		{name: "real path absolute with second slash", input: "/etc/hosts", want: "/etc/hosts"},
		{name: "bare id func:main", input: "func:main", want: ""},
		{name: "bare id method:Foo.Bar", input: "method:Foo.Bar", want: ""},
		{name: "empty string", input: "", want: ""},
		{name: "non-string int", input: 42, want: ""},
		{name: "non-string nil", input: nil, want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := worldFactPathArg(tc.input)
			if got != tc.want {
				t.Errorf("worldFactPathArg(%#v) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestGroupFactsByPath_SymbolGraphAtom(t *testing.T) {
	facts := []core.Fact{
		{
			Predicate: "symbol_graph",
			Args:      []any{"func:Foo", "/function", "/public", "internal/world/x.go", "func Foo()"},
		},
	}
	out := groupFactsByPath(facts)

	if _, ok := out["/function"]; ok {
		t.Errorf("groupFactsByPath incorrectly keyed under \"/function\"; out keys: %v", keys(out))
	}
	got, ok := out["internal/world/x.go"]
	if !ok {
		t.Fatalf("groupFactsByPath did not key under \"internal/world/x.go\"; out keys: %v", keys(out))
	}
	if len(got) != 1 {
		t.Errorf("expected 1 fact under \"internal/world/x.go\", got %d", len(got))
	}
	if len(got) > 0 && got[0].Predicate != "symbol_graph" {
		t.Errorf("expected symbol_graph predicate, got %q", got[0].Predicate)
	}
}

func keys(m map[string][]core.Fact) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
