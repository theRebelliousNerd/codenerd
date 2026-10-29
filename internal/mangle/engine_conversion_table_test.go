package mangle

import (
	"math"
	"strings"
	"testing"
	"time"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codenerd/internal/types"
)

type tableStringer struct{ s string }

func (g tableStringer) String() string { return g.s }

func TestConvertValueToTypedTerm_TypeTable(t *testing.T) {
	strType := ast.StringType
	nameType := ast.NameType
	unknown := ast.ConstantType(-1)

	tests := []struct {
		name      string
		value     any
		expType   ast.ConstantType
		wantStr   string
		wantErr   bool
		errSubstr string
	}{
		{name: "name coerces bare string", value: "foo", expType: nameType, wantStr: "/foo"},
		{name: "name keeps slash string", value: "/foo", expType: nameType, wantStr: "/foo"},
		{name: "name with non-string falls through to number", value: 42, expType: nameType, wantStr: "42"},
		{name: "string forces plain identifier to string", value: "abc", expType: strType, wantStr: `"abc"`},
		{name: "string forces slash to string not name", value: "/foo", expType: strType, wantStr: `"/foo"`},
		{name: "auto-atomizer promotes identifier", value: "abc", expType: unknown, wantStr: "/abc"},
		{name: "auto-atomizer keeps spaced string", value: "hello world", expType: unknown, wantStr: `"hello world"`},
		{name: "explicit slash string wins as name", value: "/explicit", expType: unknown, wantStr: "/explicit"},
		{name: "mangle atom without slash", value: types.MangleAtom("foo"), expType: unknown, wantStr: "/foo"},
		{name: "mangle atom with slash", value: types.MangleAtom("/foo"), expType: unknown, wantStr: "/foo"},
		{name: "mangle string stays string", value: types.MangleString("hi"), expType: unknown, wantStr: `"hi"`},
		{name: "base term passthrough", value: ast.String("hi"), expType: unknown, wantStr: `"hi"`},
		{name: "stringer becomes string", value: tableStringer{s: "s-val"}, expType: unknown, wantStr: `"s-val"`},
		{name: "int becomes number", value: 7, expType: unknown, wantStr: "7"},
		{name: "int32 becomes number", value: int32(-3), expType: unknown, wantStr: "-3"},
		{name: "int64 becomes number", value: int64(99), expType: unknown, wantStr: "99"},
		{name: "float32 becomes float", value: float32(1.5), expType: unknown, wantStr: "1.5"},
		{name: "float64 becomes float", value: 2.5, expType: unknown, wantStr: "2.5"},
		{name: "bool true", value: true, expType: unknown, wantStr: "/true"},
		{name: "bool false", value: false, expType: unknown, wantStr: "/false"},
		{name: "string slice becomes list", value: []string{"a", "b"}, expType: unknown, wantStr: `["a", "b"]`},
		{name: "any string slice becomes list", value: []any{"a"}, expType: unknown, wantStr: `["a"]`},
		{name: "any non-string errors", value: []any{123}, expType: unknown, wantErr: true, errSubstr: "only strings supported"},
		{name: "string map marshals", value: map[string]string{"k": "v"}, expType: unknown, wantStr: `"{\"k\":\"v\"}"`},
		{name: "any map marshals", value: map[string]any{"k": "v"}, expType: unknown, wantStr: `"{\"k\":\"v\"}"`},
		{name: "any map with NaN errors", value: map[string]any{"x": math.NaN()}, expType: unknown, wantErr: true},
		{name: "unsupported type errors", value: func() {}, expType: unknown, wantErr: true},
		{name: "struct marshals to json string", value: struct {
			A int `json:"a"`
		}{A: 1}, expType: unknown, wantStr: "\"{\\\"a\\\":1}\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertValueToTypedTerm(tt.value, tt.expType)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("convertValueToTypedTerm(%v) expected error, got nil (term %v)", tt.value, got)
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("convertValueToTypedTerm(%v) unexpected error: %v", tt.value, err)
			}
			if got == nil {
				t.Fatalf("convertValueToTypedTerm(%v) returned nil term", tt.value)
			}
			if got.String() != tt.wantStr {
				t.Errorf("convertValueToTypedTerm(%v) = %q, want %q", tt.value, got.String(), tt.wantStr)
			}
		})
	}
}

func TestFactString_TypeTable(t *testing.T) {
	tests := []struct {
		name string
		fact Fact
		want string
	}{
		{name: "slash string raw", fact: Fact{Predicate: "p", Args: []any{"/a"}}, want: "p(/a)."},
		{name: "plain string quoted", fact: Fact{Predicate: "p", Args: []any{"hi"}}, want: `p("hi").`},
		{name: "mangle atom raw", fact: Fact{Predicate: "p", Args: []any{types.MangleAtom("/yes")}}, want: "p(/yes)."},
		{name: "mangle string quoted", fact: Fact{Predicate: "p", Args: []any{types.MangleString("hi")}}, want: `p("hi").`},
		{name: "int", fact: Fact{Predicate: "p", Args: []any{42}}, want: "p(42)."},
		{name: "int64", fact: Fact{Predicate: "p", Args: []any{int64(-7)}}, want: "p(-7)."},
		{name: "bool true is /true", fact: Fact{Predicate: "p", Args: []any{true}}, want: "p(/true)."},
		{name: "bool false is /false", fact: Fact{Predicate: "p", Args: []any{false}}, want: "p(/false)."},
		{name: "float uses %f", fact: Fact{Predicate: "p", Args: []any{1.5}}, want: "p(1.500000)."},
		{name: "multiple args", fact: Fact{Predicate: "p", Args: []any{"a", 1, true}}, want: `p("a", 1, /true).`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fact.String(); got != tt.want {
				t.Errorf("Fact.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRepeatedVariablesAgree_Table(t *testing.T) {
	mustAtom := func(t *testing.T, s string) ast.Atom {
		t.Helper()
		a, err := ParseAtom(s)
		if err != nil {
			t.Fatalf("ParseAtom(%q) error: %v", s, err)
		}
		return a
	}
	factAB := mustAtom(t, `pair("a", "b").`)
	factAA := mustAtom(t, `pair("a", "a").`)

	tests := []struct {
		name  string
		query string
		fact  ast.Atom
		want  bool
	}{
		{name: "distinct vars always agree", query: `pair(X, Y).`, fact: factAB, want: true},
		{name: "repeated var diagonal agrees", query: `pair(X, X).`, fact: factAA, want: true},
		{name: "repeated var off-diagonal rejects", query: `pair(X, X).`, fact: factAB, want: false},
		{name: "wildcard repeats independently", query: `pair(_, _).`, fact: factAB, want: true},
		{name: "arity mismatch rejects", query: `pair(X).`, fact: factAB, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := mustAtom(t, tt.query)
			if got := repeatedVariablesAgree(q.Args, tt.fact.Args); got != tt.want {
				t.Errorf("repeatedVariablesAgree(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestParseQueryShape_Table(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		wantVars []string
		wantErr  bool
	}{
		{name: "empty rejects", query: "", wantErr: true},
		{name: "blank rejects", query: "   ", wantErr: true},
		{name: "simple var", query: "item(X).", wantVars: []string{"X"}},
		{name: "leading question mark", query: "?item(X).", wantVars: []string{"X"}},
		{name: "missing period tolerated", query: "item(X)", wantVars: []string{"X"}},
		{name: "no vars", query: `item("a").`, wantVars: nil},
		{name: "two vars", query: "pair(X, Y).", wantVars: []string{"X", "Y"}},
		{name: "garbage rejects", query: "Decl ::: not valid (((", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseQueryShape(tt.query)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseQueryShape(%q) expected error, got %+v", tt.query, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseQueryShape(%q) unexpected error: %v", tt.query, err)
			}
			var names []string
			for _, v := range got.variables {
				names = append(names, v.Name)
			}
			if len(names) != len(tt.wantVars) {
				t.Fatalf("parseQueryShape(%q) vars = %v, want %v", tt.query, names, tt.wantVars)
			}
			for i := range names {
				if names[i] != tt.wantVars[i] {
					t.Fatalf("parseQueryShape(%q) vars = %v, want %v", tt.query, names, tt.wantVars)
				}
			}
		})
	}
}

func TestCanonicalPath_Table(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty stays empty", input: "", want: ""},
		{name: "slash kept", input: "a/b", want: "a/b"},
		{name: "backslash normalized", input: `a\b`, want: "a/b"},
		{name: "clean removes dotdot", input: "a/../b", want: "b"},
		{name: "clean collapses slashes", input: "a//b", want: "a/b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canonicalPath(tt.input); got != tt.want {
				t.Errorf("canonicalPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestConstantRoundTrip_Table(t *testing.T) {
	t.Run("string constant", func(t *testing.T) {
		if got := constantToInterface(ast.String("hi")); got != "hi" {
			t.Errorf("constantToInterface(String) = %v, want hi", got)
		}
	})
	t.Run("name constant", func(t *testing.T) {
		n, err := ast.Name("/hi")
		if err != nil {
			t.Fatalf("ast.Name error: %v", err)
		}
		if got := constantToInterface(n); got != "/hi" {
			t.Errorf("constantToInterface(Name) = %v, want /hi", got)
		}
	})
	t.Run("number constant", func(t *testing.T) {
		if got := constantToInterface(ast.Number(42)); got != int64(42) {
			t.Errorf("constantToInterface(Number) = %v (%T), want 42", got, got)
		}
	})
	t.Run("float constant", func(t *testing.T) {
		if got := constantToInterface(ast.Float64(1.5)); got != 1.5 {
			t.Errorf("constantToInterface(Float64) = %v, want 1.5", got)
		}
	})
	t.Run("duration constant", func(t *testing.T) {
		c := ast.Constant{Type: ast.DurationType, NumValue: int64(5 * time.Second)}
		if got := constantToInterface(c); got != 5*time.Second {
			t.Errorf("constantToInterface(Duration) = %v, want %v", got, 5*time.Second)
		}
	})
	t.Run("base term variable", func(t *testing.T) {
		v := ast.Variable{Symbol: "X"}
		if got := convertBaseTermToInterface(v); got != "X" {
			t.Errorf("convertBaseTermToInterface(Variable) = %v, want X", got)
		}
	})
	t.Run("base term constant", func(t *testing.T) {
		if got := convertBaseTermToInterface(ast.String("k")); got != "k" {
			t.Errorf("convertBaseTermToInterface(String) = %v, want k", got)
		}
	})
}
