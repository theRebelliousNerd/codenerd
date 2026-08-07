package types

import (
	"math"
	"testing"

	"codeberg.org/TauCeti/mangle-go/ast"
)

func TestPercentFromRatio(t *testing.T) {
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
		{"just under one", 0.9999, 100},
		// The regression this pair of cases exists for. The old dual-scale
		// PercentScale guessed with `if v < 1 { v *= 100 }`, so 1.0 fell through
		// to the passthrough branch and became 1. tool_quality_* compares
		// SuccessRate against 50, so a tool that had never once failed was
		// scored as a 1% failure and queued for deprecation.
		{"perfect ratio saturates, does not invert", 1, 100},
		{"float noise above one still saturates", 1.0000000000000002, 100},
		{"NaN floors", math.NaN(), 0},
		{"positive infinity clamps", math.Inf(1), 100},
		{"negative infinity floors", math.Inf(-1), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PercentFromRatio(tc.in); got != tc.want {
				t.Errorf("PercentFromRatio(%v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestPercentClamp(t *testing.T) {
	cases := []struct {
		name string
		in   float64
		want int64
	}{
		{"zero", 0, 0},
		{"negative floors", -4, 0},
		{"typical percent", 85, 85},
		{"rounds to nearest", 85.5, 86},
		{"one percent stays one percent", 1, 1},
		{"hundred", 100, 100},
		{"above range clamps", 140, 100},
		{"NaN floors", math.NaN(), 0},
		{"positive infinity clamps", math.Inf(1), 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PercentClamp(tc.in); got != tc.want {
				t.Errorf("PercentClamp(%v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// The two entry points must disagree on 1.0 — that disagreement is the whole
// reason the ambiguous single function was split. If they ever agree here,
// someone has collapsed them back together and reintroduced the guess.
func TestPercentScales_DisagreeAtTheAmbiguousBoundary(t *testing.T) {
	if PercentFromRatio(1) == PercentClamp(1) {
		t.Fatal("PercentFromRatio(1) must be 100 (a whole ratio) and PercentClamp(1) must be 1 " +
			"(one percent); a single function cannot serve both, which is why guessing inverted " +
			"every perfect success rate")
	}
}

// The point of the helper is the Mangle constant it ultimately produces: an
// int64-typed ast.Number, which the fork's comparison builtins accept. A raw
// float64 becomes ast.Float64 and makes those builtins error out, which aborts
// the whole kernel fixpoint rather than just failing one rule.
func TestPercentFromRatio_ProducesNumberNotFloat64Constant(t *testing.T) {
	scaled, err := Fact{Predicate: "p", Args: []any{PercentFromRatio(0.85)}}.ToAtom()
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
			"if ToAtom became Decl-aware on its own, PercentFromRatio's rationale needs revisiting", rc.Type)
	}
}
