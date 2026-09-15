package mangle

import (
	"strings"
	"testing"
)

// honestError returns true when err carries a non-empty message.
// Hardening requires failures to fail closed with honest errors:
// a non-nil error must explain what went wrong, never an empty string.
func honestError(err error) bool {
	return err != nil && strings.TrimSpace(err.Error()) != ""
}

// mustNotPanic runs fn and fails the test if fn panics.
// Every hardening fix must turn a potential panic into a returned error.
func mustNotPanic(t *testing.T, name string, fn func() error) error {
	t.Helper()
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("%s panicked (must fail closed with error, not panic): %v", name, r)
			}
		}()
		err = fn()
	}()
	return err
}

func TestStep2ParseAtomEmptyFailsClosed(t *testing.T) {
	err := mustNotPanic(t, "ParseAtom(empty)", func() error {
		_, err := ParseAtom("")
		return err
	})
	if err == nil {
		t.Fatal("ParseAtom(\"\") succeeded, want fail-closed error for empty input")
	}
	if !honestError(err) {
		t.Fatalf("ParseAtom(\"\") error is not honest (empty message): %#v", err)
	}
}

func TestStep2ParseAtomMalformedFailsClosed(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"empty_paren", "("},
		{"close_only", ")"},
		{"dot_only", "."},
		{"unclosed_call", "foo("},
		{"trailing_close", "foo)"},
		{"garbage", ":::not-an-atom((("},
		{"unclosed_string", "foo(\"bar)"},
		{"leading_comma", "foo(,1)"},
		{"double_dot", "foo(1).."},
		{"whitespace_only", "   "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := mustNotPanic(t, "ParseAtom("+tc.name+")", func() error {
				_, err := ParseAtom(tc.input)
				return err
			})
			if err == nil {
				t.Fatalf("ParseAtom(%q) succeeded, want fail-closed error", tc.input)
			}
			if !honestError(err) {
				t.Fatalf("ParseAtom(%q) error is not honest (empty message): %#v", tc.input, err)
			}
		})
	}
}

func TestStep2ParseAtomDeterministic(t *testing.T) {
	// Stability: repeated failures must not corrupt global parser state.
	for i := 0; i < 3; i++ {
		err := mustNotPanic(t, "ParseAtom(empty) repeat", func() error {
			_, err := ParseAtom("")
			return err
		})
		if err == nil {
			t.Fatalf("iteration %d: ParseAtom(\"\") succeeded, want error", i)
		}
		if !honestError(err) {
			t.Fatalf("iteration %d: dishonest error: %#v", i, err)
		}
	}
}

func TestStep2ParseUnitNilReaderFailsClosed(t *testing.T) {
	err := mustNotPanic(t, "ParseUnit(nil)", func() error {
		_, err := ParseUnit(nil)
		return err
	})
	if err == nil {
		t.Fatal("ParseUnit(nil) succeeded, want fail-closed error for nil reader")
	}
	if !honestError(err) {
		t.Fatalf("ParseUnit(nil) error is not honest (empty message): %#v", err)
	}
}

func TestStep2ParseUnitEmptyNoPanic(t *testing.T) {
	// Empty program: must not panic. Either an honest error or a usable
	// empty unit is acceptable; a panic or silent corrupt state is not.
	_ = mustNotPanic(t, "ParseUnit(empty)", func() error {
		_, err := ParseUnit(strings.NewReader(""))
		if err != nil && !honestError(err) {
			t.Fatalf("ParseUnit(empty) dishonest error: %#v", err)
		}
		return err
	})
}

func TestStep2ParseUnitMalformedFailsClosed(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"garbage", "::: ((("},
		{"unclosed_rule", "foo(X) :- "},
		{"unclosed_string", "foo(\"bar)."},
		{"dangling_negation", "foo(X) :- !."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := mustNotPanic(t, "ParseUnit("+tc.name+")", func() error {
				_, err := ParseUnit(strings.NewReader(tc.input))
				return err
			})
			if err == nil {
				t.Fatalf("ParseUnit(%q) succeeded, want fail-closed error", tc.input)
			}
			if !honestError(err) {
				t.Fatalf("ParseUnit(%q) error is not honest (empty message): %#v", tc.input, err)
			}
		})
	}
}

func TestStep2DifferentialEngineZeroValueNoPanic(t *testing.T) {
	// Zero-value engine must fail closed with honest errors, never panic
	// and never silently accept corrupt state.
	var de DifferentialEngine

	if err := mustNotPanic(t, "ApplyDelta(nil)", func() error {
		return de.ApplyDelta(nil)
	}); err != nil && !honestError(err) {
		t.Fatalf("ApplyDelta(nil) dishonest error: %#v", err)
	}

	if err := mustNotPanic(t, "ApplyAtomDelta(nil)", func() error {
		return de.ApplyAtomDelta(nil)
	}); err != nil && !honestError(err) {
		t.Fatalf("ApplyAtomDelta(nil) dishonest error: %#v", err)
	}

	if err := mustNotPanic(t, "AddFactIncremental(zero)", func() error {
		var f Fact
		return de.AddFactIncremental(f)
	}); err != nil && !honestError(err) {
		t.Fatalf("AddFactIncremental(zero) dishonest error: %#v", err)
	}

	_ = mustNotPanic(t, "EnableUnifiedFastPath(zero)", func() error {
		return de.EnableUnifiedFastPath()
	})
}

func TestStep2DifferentialEngineEmptyDeltaNoPanic(t *testing.T) {
	// Empty deltas are the boundary between no-op and error: either way
	// the engine must stay stable, never panic, and any error must be honest.
	var de DifferentialEngine

	if err := mustNotPanic(t, "ApplyDelta(empty)", func() error {
		return de.ApplyDelta([]Fact{})
	}); err != nil && !honestError(err) {
		t.Fatalf("ApplyDelta(empty) dishonest error: %#v", err)
	}
}
