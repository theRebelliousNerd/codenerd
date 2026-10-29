package mangle

import (
	"strings"
	"testing"

	"codenerd/internal/types"
)

// TestStep1GapProbeFactString surveys Fact.String branch coverage.
// Each row pins one type-switch branch in engine.go so later steps can
// tell which branches were untested before the fix.
func TestStep1GapProbeFactString(t *testing.T) {
	cases := []struct {
		name string
		fact Fact
		want string
	}{
		{
			name: "mangle atom passthrough",
			fact: Fact{Predicate: "p", Args: []any{types.MangleAtom("/myatom")}},
			want: "p(/myatom).",
		},
		{
			name: "mangle string quoted",
			fact: Fact{Predicate: "p", Args: []any{types.MangleString("hello")}},
			want: `p("hello").`,
		},
		{
			name: "slash string bare",
			fact: Fact{Predicate: "p", Args: []any{"/foo"}},
			want: "p(/foo).",
		},
		{
			name: "plain string quoted",
			fact: Fact{Predicate: "p", Args: []any{"foo"}},
			want: `p("foo").`,
		},
		{
			name: "int decimal",
			fact: Fact{Predicate: "p", Args: []any{42}},
			want: "p(42).",
		},
		{
			name: "int64 decimal",
			fact: Fact{Predicate: "p", Args: []any{int64(42)}},
			want: "p(42).",
		},
		{
			name: "float64 six decimals",
			fact: Fact{Predicate: "p", Args: []any{float64(1.5)}},
			want: "p(1.500000).",
		},
		{
			name: "bool true atom",
			fact: Fact{Predicate: "p", Args: []any{true}},
			want: "p(/true).",
		},
		{
			name: "bool false atom",
			fact: Fact{Predicate: "p", Args: []any{false}},
			want: "p(/false).",
		},
		{
			name: "fallback int32 via default",
			fact: Fact{Predicate: "p", Args: []any{int32(7)}},
			want: "p(7).",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.fact.String(); got != tc.want {
				t.Fatalf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestStep1GapProbeDefaultConfigEnv surveys the MANGLE_AUTO_EVAL branch
// in DefaultConfig, which toggles AutoEval off only when set to "0".
func TestStep1GapProbeDefaultConfigEnv(t *testing.T) {
	cases := []struct {
		name     string
		envValue *string
		wantAuto bool
	}{
		{name: "unset defaults to true", envValue: nil, wantAuto: true},
		{name: "zero disables", envValue: strPtr("0"), wantAuto: false},
		{name: "one enables", envValue: strPtr("1"), wantAuto: true},
		{name: "empty enables", envValue: strPtr(""), wantAuto: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envValue == nil {
				t.Setenv("MANGLE_AUTO_EVAL", "")
				// Unset semantics: empty string is != "0" so AutoEval stays true.
				// Explicitly clear via test env; DefaultConfig reads the live env.
			} else {
				t.Setenv("MANGLE_AUTO_EVAL", *tc.envValue)
			}
			cfg := DefaultConfig()
			if cfg.AutoEval != tc.wantAuto {
				t.Fatalf("DefaultConfig().AutoEval = %v, want %v (env=%v)", cfg.AutoEval, tc.wantAuto, tc.envValue)
			}
		})
	}
}

func strPtr(s string) *string { return &s }

// TestStep1GapProbeEnginePreconditions surveys pre-schema error branches:
// RecomputeRules, evalWithGasLimit, and LoadSchema on a missing file.
func TestStep1GapProbeEnginePreconditions(t *testing.T) {
	t.Run("recompute without schema", func(t *testing.T) {
		e, err := NewEngine(DefaultConfig(), nil)
		if err != nil {
			t.Fatalf("NewEngine failed: %v", err)
		}
		if err := e.RecomputeRules(); err == nil {
			t.Fatal("RecomputeRules without schema should fail")
		} else if !strings.Contains(err.Error(), "no schemas") {
			t.Fatalf("RecomputeRules error = %q, want no-schemas sentinel", err.Error())
		}
	})

	t.Run("eval without stratification", func(t *testing.T) {
		e, err := NewEngine(DefaultConfig(), nil)
		if err != nil {
			t.Fatalf("NewEngine failed: %v", err)
		}
		if _, err := e.evalWithGasLimit(); err == nil {
			t.Fatal("evalWithGasLimit without schema should fail")
		} else if !strings.Contains(err.Error(), "LoadSchema") {
			t.Fatalf("evalWithGasLimit error = %q, want LoadSchema hint", err.Error())
		}
	})

	t.Run("load missing file", func(t *testing.T) {
		e, err := NewEngine(DefaultConfig(), nil)
		if err != nil {
			t.Fatalf("NewEngine failed: %v", err)
		}
		if err := e.LoadSchema("testdata/does-not-exist-xyz.mg"); err == nil {
			t.Fatal("LoadSchema on missing file should fail")
		}
	})

	t.Run("derived count reset", func(t *testing.T) {
		e, err := NewEngine(DefaultConfig(), nil)
		if err != nil {
			t.Fatalf("NewEngine failed: %v", err)
		}
		e.ResetDerivedFactCount()
		if got := e.GetDerivedFactCount(); got != 0 {
			t.Fatalf("GetDerivedFactCount after reset = %d, want 0", got)
		}
	})
}
