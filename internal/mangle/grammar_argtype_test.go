package mangle

import (
	"testing"

	"codeberg.org/TauCeti/mangle-go/ast"
)

func TestBoundToArgType(t *testing.T) {
	cases := []struct {
		name  string
		bound ast.BaseTerm
		want  ArgType
	}{
		{"name", ast.NameBound, ArgTypeName},
		{"string", ast.StringBound, ArgTypeString},
		{"number", ast.NumberBound, ArgTypeNumber},
		{"float64", ast.Float64Bound, ArgTypeNumber},
		{"any", ast.AnyBound, ArgTypeAny},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := boundToArgType(c.bound); got != c.want {
				t.Errorf("boundToArgType(%s)=%v, want %v", c.name, got, c.want)
			}
		})
	}

	// A non-constant base term falls back to ArgTypeAny.
	if got := boundToArgType(ast.Variable{Symbol: "X"}); got != ArgTypeAny {
		t.Errorf("boundToArgType(variable)=%v, want ArgTypeAny", got)
	}
}
