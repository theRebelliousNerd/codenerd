package mangle_test

import (
	"strings"
	"testing"

	"codenerd/internal/mangle"
	"codenerd/internal/types"
)

func TestEngineFactStringBranches(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		fact mangle.Fact
		want string
	}{
		{"NoArgs", mangle.Fact{Predicate: "pred"}, "pred()."},
		{"AtomPassthrough", mangle.Fact{Predicate: "p", Args: []any{"/a"}}, "p(/a)."},
		{"QuotedString", mangle.Fact{Predicate: "label", Args: []any{"hello"}}, "label(\"hello\")."},
		{"MangleAtomRaw", mangle.Fact{Predicate: "p", Args: []any{types.MangleAtom("/raw")}}, "p(/raw)."},
		{"MangleStringQuoted", mangle.Fact{Predicate: "p", Args: []any{types.MangleString("hi")}}, "p(\"hi\")."},
		{"Int", mangle.Fact{Predicate: "p", Args: []any{42}}, "p(42)."},
		{"Int64", mangle.Fact{Predicate: "p", Args: []any{int64(42)}}, "p(42)."},
		{"Float", mangle.Fact{Predicate: "p", Args: []any{float64(1.5)}}, "p(1.500000)."},
		{"BoolTrue", mangle.Fact{Predicate: "p", Args: []any{true}}, "p(/true)."},
		{"BoolFalse", mangle.Fact{Predicate: "p", Args: []any{false}}, "p(/false)."},
		{"MultiArgs", mangle.Fact{Predicate: "edge", Args: []any{"/a", "/b"}}, "edge(/a, /b)."},
		{"MixedArgs", mangle.Fact{Predicate: "m", Args: []any{"/a", "hello", 7}}, "m(/a, \"hello\", 7)."},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.fact.String(); got != tc.want {
				t.Fatalf("Fact.String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEngineDefaultConfigLimits(t *testing.T) {
	t.Setenv("MANGLE_AUTO_EVAL", "1")
	cfg := mangle.DefaultConfig()
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"FactLimit", cfg.FactLimit, 100000},
		{"DerivedFactsLimit", cfg.DerivedFactsLimit, 100000},
		{"QueryTimeout", cfg.QueryTimeout, 30},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("%s = %d, want %d", tc.name, tc.got, tc.want)
			}
		})
	}
	if !cfg.AutoEval {
		t.Fatalf("AutoEval = false, want true with MANGLE_AUTO_EVAL=1")
	}
}

func TestEngineFailClosedNoSchema(t *testing.T) {
	t.Parallel()
	eng, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if eng == nil {
		t.Fatal("NewEngine returned nil engine")
	}
	if err := eng.RecomputeRules(); err == nil {
		t.Fatal("RecomputeRules before LoadSchema = nil, want fail-closed error")
	} else if !strings.Contains(strings.ToLower(err.Error()), "schema") {
		t.Fatalf("RecomputeRules error = %q, want schema-related fail-closed error", err)
	}
}

func TestEngineLoadSchemaMissingFile(t *testing.T) {
	t.Parallel()
	eng, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	cases := []struct {
		name string
		path string
	}{
		{"Missing", "/nonexistent-dir-nerd/missing.mg"},
		{"Empty", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := eng.LoadSchema(tc.path); err == nil {
				t.Fatalf("LoadSchema(%q) = nil, want error", tc.path)
			}
		})
	}
}

func TestEngineDerivedCounterLifecycle(t *testing.T) {
	t.Parallel()
	eng, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if got := eng.GetDerivedFactCount(); got != 0 {
		t.Fatalf("initial GetDerivedFactCount = %d, want 0", got)
	}
	eng.ResetDerivedFactCount()
	if got := eng.GetDerivedFactCount(); got != 0 {
		t.Fatalf("after reset GetDerivedFactCount = %d, want 0", got)
	}
	// ToggleAutoEval must not panic and must leave engine usable.
	eng.ToggleAutoEval(false)
	eng.ToggleAutoEval(true)
	if got := eng.GetDerivedFactCount(); got != 0 {
		t.Fatalf("after toggles GetDerivedFactCount = %d, want 0", got)
	}
}
