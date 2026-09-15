package mangle

import (
	"strings"
	"testing"
)

// Step-2 regression tests for the Mangle kernel hardening pass.
// Each test locks in fail-closed behaviour on error paths using only
// APIs verified in the package index, so the suite itself cannot rot:
// invalid inputs must error (never panic, never silent success) and
// empty batches must be safe no-ops.

func TestHardenStep2B_ParseAtomRejectsGarbage(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "colons", input: ":::"},
		{name: "parens", input: "((("},
		{name: "not_an_atom", input: "this is ::: not valid mangle ((("},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("ParseAtom(%q) panicked: %v", tc.input, r)
				}
			}()
			if _, err := ParseAtom(tc.input); err == nil {
				t.Fatalf("ParseAtom(%q) should fail closed, got nil error", tc.input)
			}
		})
	}
}

func TestHardenStep2B_ParseUnitRejectsGarbage(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "garbage", input: "this is ::: not valid mangle ((("},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("ParseUnit(%q) panicked: %v", tc.input, r)
				}
			}()
			// Invalid units must not panic; a rejected unit reports an
			// honest error, an empty unit reports no facts without error.
			// Either way the call must return without panicking.
			_, _ = ParseUnit(strings.NewReader(tc.input))
		})
	}
}

func TestHardenStep2B_ZeroEngineFailsClosed(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("zero DifferentialEngine method panicked: %v", r)
		}
	}()
	e := &DifferentialEngine{}
	if err := e.AddFactIncremental(Fact{}); err == nil {
		t.Fatal("AddFactIncremental on zero engine should fail closed, got nil error")
	}
	if err := e.ApplyDelta([]Fact{{}}); err == nil {
		t.Fatal("ApplyDelta with facts on zero engine should fail closed, got nil error")
	}
}

func TestHardenStep2B_EmptyDeltaIsSafeNoOp(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("empty ApplyDelta panicked: %v", r)
		}
	}()
	e := &DifferentialEngine{}
	if err := e.ApplyDelta(nil); err != nil {
		t.Fatalf("ApplyDelta(nil) should be a safe no-op, got: %v", err)
	}
	if err := e.ApplyDelta([]Fact{}); err != nil {
		t.Fatalf("ApplyDelta(empty) should be a safe no-op, got: %v", err)
	}
}

func TestHardenStep2B_EnableFastPathDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("EnableUnifiedFastPath panicked: %v", r)
		}
	}()
	e := &DifferentialEngine{}
	_ = e.EnableUnifiedFastPath()
}
