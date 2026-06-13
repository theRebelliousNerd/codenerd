package context

import (
	"errors"
	"testing"

	"codenerd/internal/core"
)

// TestFactArgAsInt covers the int / int64 / float64 type-drift handling that
// back-reference scoring depends on, plus the empty-args and wrong-type paths.
func TestFactArgAsInt(t *testing.T) {
	cases := []struct {
		name   string
		fact   core.Fact
		want   int
		wantOk bool
	}{
		{"int", core.Fact{Predicate: "p", Args: []any{7}}, 7, true},
		{"int64", core.Fact{Predicate: "p", Args: []any{int64(9)}}, 9, true},
		{"float64", core.Fact{Predicate: "p", Args: []any{3.9}}, 3, true},
		{"empty args", core.Fact{Predicate: "p", Args: nil}, 0, false},
		{"wrong type", core.Fact{Predicate: "p", Args: []any{"hello"}}, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := factArgAsInt(tc.fact)
			if got != tc.want || ok != tc.wantOk {
				t.Errorf("factArgAsInt(%v)=(%d,%v), want (%d,%v)", tc.fact.Args, got, ok, tc.want, tc.wantOk)
			}
		})
	}
}

// TestFormatArg covers every branch of the Mangle argument formatter: name
// constants, quoted strings, integers, floats, and the two boolean atoms.
func TestFormatArg(t *testing.T) {
	cases := []struct {
		name string
		arg  any
		want string
	}{
		{"name constant", "/intent", "/intent"},
		{"plain string", "hello", `"hello"`},
		{"int", 42, "42"},
		{"int64", int64(42), "42"},
		{"float", 1.5, "1.50"},
		{"bool true", true, "/true"},
		{"bool false", false, "/false"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatArg(tc.arg); got != tc.want {
				t.Errorf("formatArg(%v)=%q, want %q", tc.arg, got, tc.want)
			}
		})
	}
}

// TestTokenBudgetAllocateAndRelease exercises the per-category allocation
// accounting, the over-budget rejection, the unknown-category path, and that
// Release frees capacity without underflowing past zero.
func TestTokenBudgetAllocateAndRelease(t *testing.T) {
	cfg := CompressorConfig{
		TotalBudget:    1000,
		CoreReserve:    100,
		AtomReserve:    200,
		HistoryReserve: 300,
		WorkingReserve: 400,
	}
	tb := NewTokenBudget(cfg)

	if !tb.IsHardEnforcementEnabled() {
		t.Error("NewTokenBudget should default to hard enforcement")
	}

	// Allocation within the reserve succeeds.
	if !tb.Allocate("core", 50) {
		t.Fatal("Allocate(core,50) should succeed within the 100 reserve")
	}
	// A second allocation that would exceed the reserve is rejected and must
	// not mutate the running total.
	if tb.Allocate("core", 60) {
		t.Error("Allocate(core,60) should be rejected: 50+60 > 100 reserve")
	}
	if tb.TotalUsed() != 50 {
		t.Errorf("TotalUsed=%d after rejected allocation, want 50", tb.TotalUsed())
	}

	// Unknown categories are rejected.
	if tb.Allocate("bogus", 1) {
		t.Error("Allocate(bogus,1) should be rejected as an unknown category")
	}

	// Release frees capacity so a subsequent allocation fits again.
	tb.Release("core", 50)
	if tb.TotalUsed() != 0 {
		t.Errorf("TotalUsed=%d after release, want 0", tb.TotalUsed())
	}
	// Release never drives a category negative.
	tb.Release("core", 999)
	if tb.TotalUsed() != 0 {
		t.Errorf("TotalUsed=%d after over-release, want 0 (no underflow)", tb.TotalUsed())
	}
	if !tb.Allocate("core", 80) {
		t.Error("Allocate(core,80) should succeed after release restored capacity")
	}
}

// TestTokenBudgetHardEnforcementToggle verifies AllocateWithError surfaces the
// sentinel error on rejection and that the toggle is observable.
func TestTokenBudgetHardEnforcementToggle(t *testing.T) {
	tb := NewTokenBudget(CompressorConfig{TotalBudget: 100, AtomReserve: 10})

	if err := tb.AllocateWithError("atoms", 5); err != nil {
		t.Fatalf("AllocateWithError(atoms,5) within reserve: %v", err)
	}
	err := tb.AllocateWithError("atoms", 50)
	if err == nil {
		t.Fatal("AllocateWithError(atoms,50) should fail: 5+50 > 10 reserve")
	}
	if !errors.Is(err, ErrContextWindowExceeded) {
		t.Errorf("error %v should wrap ErrContextWindowExceeded", err)
	}

	tb.SetHardEnforcement(false)
	if tb.IsHardEnforcementEnabled() {
		t.Error("IsHardEnforcementEnabled should report false after SetHardEnforcement(false)")
	}
	tb.SetHardEnforcement(true)
	if !tb.IsHardEnforcementEnabled() {
		t.Error("IsHardEnforcementEnabled should report true after SetHardEnforcement(true)")
	}
}

// TestTokenBudgetMustFitWithinBudget covers the total-budget headroom check
// used before committing a batch of additional tokens.
func TestTokenBudgetMustFitWithinBudget(t *testing.T) {
	tb := NewTokenBudget(CompressorConfig{TotalBudget: 100, WorkingReserve: 100})
	if !tb.Allocate("working", 60) {
		t.Fatal("Allocate(working,60) should succeed")
	}
	if err := tb.MustFitWithinBudget(30); err != nil {
		t.Errorf("MustFitWithinBudget(30) with 60 used should fit in 100: %v", err)
	}
	if err := tb.MustFitWithinBudget(50); err == nil {
		t.Error("MustFitWithinBudget(50) with 60 used should exceed the 100 budget")
	}
	if err := tb.CheckTotalBudget(); err != nil {
		t.Errorf("CheckTotalBudget with 60 used should pass under 100: %v", err)
	}
}
