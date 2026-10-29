package mangle

import (
	"math"
	"strings"
	"testing"
	"time"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codenerd/internal/types"
)

// Second hardening wave: table-driven locks for pure helpers and fail-closed
// shapes the first wave left uncovered. No schema needed; every case is
// deterministic and independent.

func TestFactString_EncodesArgTypesDeterministically(t *testing.T) {
	tests := []struct {
		name string
		fact Fact
		want string
	}{
		{
			name: "plain string is quoted",
			fact: Fact{Predicate: "p", Args: []any{"hello"}},
			want: `p("hello").`,
		},
		{
			name: "slash string stays raw atom",
			fact: Fact{Predicate: "p", Args: []any{"/foo"}},
			want: `p(/foo).`,
		},
		{
			name: "explicit atom type stays raw",
			fact: Fact{Predicate: "p", Args: []any{types.MangleAtom("/yes")}},
			want: `p(/yes).`,
		},
		{
			name: "explicit string type is quoted",
			fact: Fact{Predicate: "p", Args: []any{types.MangleString("hi")}},
			want: `p("hi").`,
		},
		{
			name: "int formats decimal",
			fact: Fact{Predicate: "p", Args: []any{42}},
			want: `p(42).`,
		},
		{
			name: "int64 formats decimal",
			fact: Fact{Predicate: "p", Args: []any{int64(-7)}},
			want: `p(-7).`,
		},
		{
			name: "bool true maps to /true",
			fact: Fact{Predicate: "p", Args: []any{true}},
			want: `p(/true).`,
		},
		{
			name: "bool false maps to /false",
			fact: Fact{Predicate: "p", Args: []any{false}},
			want: `p(/false).`,
		},
		{
			name: "multiple args join with comma space",
			fact: Fact{Predicate: "pred", Args: []any{"a", 1, true}},
			want: `pred("a", 1, /true).`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fact.String()
			if got != tt.want {
				t.Errorf("Fact.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConvertValue_UnknownTypeCoercion_Table(t *testing.T) {
	floatVal := 3.25
	tests := []struct {
		name        string
		value       any
		expected    ast.ConstantType
		wantErr     string
		wantContain string
	}{
		{name: "nil fails closed", value: nil, expected: -1, wantErr: "nil"},
		{name: "func fails closed", value: func() {}, expected: -1, wantErr: "unsupported fact argument type"},
		{name: "mixed list fails closed", value: []any{"ok", 42}, expected: -1, wantErr: "unsupported list element"},
		{name: "map with func fails closed", value: map[string]any{"f": func() {}}, expected: -1, wantErr: "unsupported fact argument type"},
		{name: "int ok", value: 42, expected: -1, wantContain: "42"},
		{name: "int64 ok", value: int64(-9), expected: -1, wantContain: "-9"},
		{name: "float64 ok", value: floatVal, expected: -1, wantContain: "3.25"},
		{name: "bool ok", value: true, expected: -1, wantContain: "true"},
		{name: "identifier atomizes to /foo", value: "foo", expected: -1, wantContain: "/foo"},
		{name: "non-identifier stays string", value: "hello world", expected: -1, wantContain: "hello world"},
		{name: "slash prefix stays name", value: "/explicit", expected: -1, wantContain: "/explicit"},
		{name: "string list ok", value: []string{"a", "b"}, expected: -1, wantContain: "a"},
		{name: "any string list ok", value: []any{"a", "b"}, expected: -1, wantContain: "a"},
		{name: "string map ok", value: map[string]string{"k": "v"}, expected: -1, wantContain: "k"},
		{name: "any map ok", value: map[string]any{"k": "v"}, expected: -1, wantContain: "k"},
		{name: "explicit atom ok", value: types.MangleAtom("yes"), expected: -1, wantContain: "/yes"},
		{name: "explicit string ok", value: types.MangleString("hi"), expected: -1, wantContain: "hi"},
		{name: "name type coerces identifier", value: "foo", expected: ast.NameType, wantContain: "/foo"},
		{name: "string type keeps identifier as string", value: "foo", expected: ast.StringType, wantContain: "foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertValueToTypedTerm(tt.value, tt.expected)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("convertValueToTypedTerm(%T) = nil error, want %q", tt.value, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("convertValueToTypedTerm(%T) unexpected error: %v", tt.value, err)
			}
			if got == nil {
				t.Fatal("convertValueToTypedTerm returned nil term without error")
			}
			if tt.wantContain != "" && !strings.Contains(got.String(), tt.wantContain) {
				t.Errorf("term %q does not contain %q", got.String(), tt.wantContain)
			}
		})
	}
}

func TestConstantToInterface_MapsEveryKnownType(t *testing.T) {
	floatBits := int64(math.Float64bits(3.14))
	ts := time.Unix(0, 0).UTC()
	dur := time.Second
	tests := []struct {
		name  string
		input ast.Constant
		want  any
	}{
		{name: "string", input: ast.Constant{Symbol: "hi", Type: ast.StringType}, want: "hi"},
		{name: "name", input: ast.Constant{Symbol: "/foo", Type: ast.NameType}, want: "/foo"},
		{name: "bytes", input: ast.Constant{Symbol: "abc", Type: ast.BytesType}, want: "abc"},
		{name: "number", input: ast.Constant{NumValue: 42, Type: ast.NumberType}, want: int64(42)},
		{name: "float64", input: ast.Constant{NumValue: floatBits, Type: ast.Float64Type}, want: 3.14},
		{name: "time", input: ast.Constant{NumValue: 0, Type: ast.TimeType}, want: ts},
		{name: "duration", input: ast.Constant{NumValue: int64(dur), Type: ast.DurationType}, want: dur},
		{name: "unknown falls back to symbol", input: ast.Constant{Symbol: "fallback", Type: ast.ConstantType(9999)}, want: "fallback"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := constantToInterface(tt.input)
			// Time needs Equal, others DeepEqual via string compare for stability.
			if wantT, ok := tt.want.(time.Time); ok {
				gotT, ok := got.(time.Time)
				if !ok || !gotT.Equal(wantT) {
					t.Errorf("constantToInterface() = %v (%T), want %v", got, got, tt.want)
				}
				return
			}
			if got != tt.want {
				t.Errorf("constantToInterface() = %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestConvertBaseTerm_VariableAndConstant(t *testing.T) {
	tests := []struct {
		name  string
		input ast.BaseTerm
		want  any
	}{
		{name: "variable yields symbol", input: ast.Variable{Symbol: "X"}, want: "X"},
		{name: "string constant yields symbol", input: ast.String("hi"), want: "hi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertBaseTermToInterface(tt.input)
			if got != tt.want {
				t.Errorf("convertBaseTermToInterface() = %v (%T), want %v", got, got, tt.want)
			}
		})
	}
}

func TestCanonicalPath_NormalizesSeparatorsGap2(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty stays empty", input: "", want: ""},
		{name: "dotdot cleans", input: "a/../b", want: "b"},
		{name: "double slash cleans", input: "a//b", want: "a/b"},
		{name: "backslash becomes slash", input: `a\b`, want: "a/b"},
		{name: "plain path unchanged", input: "a/b/c.go", want: "a/b/c.go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canonicalPath(tt.input); got != tt.want {
				t.Errorf("canonicalPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseQueryShape_PrefixSuffix_Table(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		wantErr  string
		wantVars int
		firstVar string
		firstIdx int
	}{
		{name: "empty fails", query: "", wantErr: "empty query"},
		{name: "blank fails", query: "   ", wantErr: "empty query"},
		{name: "lone dot fails", query: ".", wantErr: "failed to parse query"},
		{name: "question dot fails", query: "?.", wantErr: "failed to parse query"},
		{name: "unbalanced paren fails", query: "p(", wantErr: "failed to parse query"},
		{name: "plain shape", query: "p(X)", wantVars: 1, firstVar: "X", firstIdx: 0},
		{name: "leading question stripped", query: "?p(X)", wantVars: 1, firstVar: "X", firstIdx: 0},
		{name: "trailing dot stripped", query: "p(X).", wantVars: 1, firstVar: "X", firstIdx: 0},
		{name: "both affixes stripped", query: "?p(X).", wantVars: 1, firstVar: "X", firstIdx: 0},
		{name: "two vars keep order", query: "p(X, Y)", wantVars: 2, firstVar: "X", firstIdx: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shape, err := parseQueryShape(tt.query)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("parseQueryShape(%q) = nil error, want %q", tt.query, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("parseQueryShape(%q) error %q does not contain %q", tt.query, err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseQueryShape(%q) unexpected error: %v", tt.query, err)
			}
			if len(shape.variables) != tt.wantVars {
				t.Fatalf("parseQueryShape(%q) vars = %d, want %d", tt.query, len(shape.variables), tt.wantVars)
			}
			if tt.wantVars > 0 {
				if shape.variables[0].Name != tt.firstVar || shape.variables[0].Index != tt.firstIdx {
					t.Errorf("first var = %+v, want Name=%q Index=%d", shape.variables[0], tt.firstVar, tt.firstIdx)
				}
			}
		})
	}
}

func TestRepeatedVariablesAgree_DiagonalAndWildcard(t *testing.T) {
	v := func(s string) ast.BaseTerm { return ast.Variable{Symbol: s} }
	c := func(s string) ast.BaseTerm { return ast.String(s) }

	tests := []struct {
		name  string
		query []ast.BaseTerm
		fact  []ast.BaseTerm
		want  bool
	}{
		{name: "arity mismatch false", query: []ast.BaseTerm{v("X")}, fact: []ast.BaseTerm{c("a"), c("b")}, want: false},
		{name: "single var true", query: []ast.BaseTerm{v("X")}, fact: []ast.BaseTerm{c("a")}, want: true},
		{name: "diagonal true", query: []ast.BaseTerm{v("X"), v("X")}, fact: []ast.BaseTerm{c("a"), c("a")}, want: true},
		{name: "off diagonal false", query: []ast.BaseTerm{v("X"), v("X")}, fact: []ast.BaseTerm{c("a"), c("b")}, want: false},
		{name: "wildcard exempt", query: []ast.BaseTerm{v("_"), v("_")}, fact: []ast.BaseTerm{c("a"), c("b")}, want: true},
		{name: "distinct vars independent", query: []ast.BaseTerm{v("X"), v("Y")}, fact: []ast.BaseTerm{c("a"), c("b")}, want: true},
		{name: "constants skipped", query: []ast.BaseTerm{c("a"), v("X"), v("X")}, fact: []ast.BaseTerm{c("a"), c("b"), c("b")}, want: true},
		{name: "constants skipped still rejects", query: []ast.BaseTerm{c("a"), v("X"), v("X")}, fact: []ast.BaseTerm{c("a"), c("b"), c("c")}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := repeatedVariablesAgree(tt.query, tt.fact); got != tt.want {
				t.Errorf("repeatedVariablesAgree() = %v, want %v", got, tt.want)
			}
		})
	}
}
