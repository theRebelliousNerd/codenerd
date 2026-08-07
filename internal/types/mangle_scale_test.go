package types

import (
	"math"
	"testing"

	"codeberg.org/TauCeti/mangle-go/ast"
)

func TestPercentScale(t *testing.T) {
	cases := []struct {
		name string
		in   float64
		want int64
	}{
		{"zero", 0, 0},
		{"negative clamps to floor", -0.4, 0},
		{"typical ratio", 0.85, 85},
		{"rounds to nearest", 0.855, 86},
		{"threshold boundary", 0.7, 70},
		{"already a percent", 95, 95},
		{"percent above range clamps", 140, 100},
		{"ratio of one reads as one percent", 1, 1},
		{"NaN floors", math.NaN(), 0},
		{"positive infinity clamps", math.Inf(1), 100},
		{"negative infinity floors", math.Inf(-1), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PercentScale(tc.in); got != tc.want {
				t.Errorf("PercentScale(%v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// The point of the helper is the Mangle constant it ultimately produces: an
// int64-typed ast.Number, which the fork's comparison builtins accept. A raw
// float64 becomes ast.Float64 and makes those builtins error out, which aborts
// the whole kernel fixpoint rather than just failing one rule.
func TestPercentScale_ProducesNumberNotFloat64Constant(t *testing.T) {
	scaled, err := Fact{Predicate: "p", Args: []any{PercentScale(0.85)}}.ToAtom()
	if err != nil {
		t.Fatalf("ToAtom: %v", err)
	}
	c, ok := scaled.Args[0].(ast.Constant)
	if !ok {
		t.Fatalf("arg is %T, want ast.Constant", scaled.Args[0])
	}
	if c.Type != ast.NumberType {
		t.Errorf("scaled arg has Type %v, want NumberType (%v)", c.Type, ast.NumberType)
	}

	// Contrast: the unscaled float is exactly what used to poison the store.
	raw, err := Fact{Predicate: "p", Args: []any{0.85}}.ToAtom()
	if err != nil {
		t.Fatalf("ToAtom: %v", err)
	}
	if rc := raw.Args[0].(ast.Constant); rc.Type != ast.Float64Type {
		t.Fatalf("precondition changed: a bare float64 now maps to %v, not Float64Type — "+
			"if ToAtom became Decl-aware on its own, PercentScale's rationale needs revisiting", rc.Type)
	}
}
