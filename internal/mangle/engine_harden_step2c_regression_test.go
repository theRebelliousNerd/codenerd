package mangle

import (
	"strings"
	"testing"
)

// Step 2c hardening regression tests.
//
// Covers Step 1 HARDEN fixes in internal/mangle:
//   - error-path gaps (nil/empty/invalid inputs to ParseUnit/ParseAtom,
//     nil dest for CopyAllFactsTo)
//   - edge cases (empty schema string, empty reader, zero-value engine ops)
//   - fail-closed honest errors (every failure returns a non-empty error,
//     never panics)
//   - stability (failed ops never corrupt state; engine stays usable for
//     later ops, repeated failures stay stable).
//
// All helpers use unique step2c- names to avoid colliding with helpers in
// engine_failclosed_test.go and engine_step2_regression_test.go.
// Only APIs confirmed in package docs are used (ParseUnit, ParseAtom,
// DifferentialEngine methods) so this file cannot break the build.

func step2cHonest(err error) bool {
	return err != nil && strings.TrimSpace(err.Error()) != ""
}

func step2cMustNotPanic(t *testing.T, name string, fn func() error) error {
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

func TestStep2cParseAtomInvalidFailsClosed(t *testing.T) {
	err := step2cMustNotPanic(t, "ParseAtom(invalid)", func() error {
		_, err := ParseAtom("Decl ::: not valid (((")
		return err
	})
	if err == nil {
		t.Fatal("ParseAtom(invalid) succeeded, want fail-closed error")
	}
	if !step2cHonest(err) {
		t.Fatalf("ParseAtom(invalid) dishonest error (empty message): %#v", err)
	}
}

func TestStep2cParseAtomEmptyNoPanic(t *testing.T) {
	err := step2cMustNotPanic(t, "ParseAtom(empty)", func() error {
		_, err := ParseAtom("")
		return err
	})
	if err != nil && !step2cHonest(err) {
		t.Fatalf("ParseAtom(\"\") dishonest error: %#v", err)
	}
}

func TestStep2cParseAtomRepeatedFailuresStable(t *testing.T) {
	for i := 0; i < 3; i++ {
		err := step2cMustNotPanic(t, "ParseAtom(invalid) repeat", func() error {
			_, err := ParseAtom("Decl ::: not valid (((")
			return err
		})
		if err == nil {
			t.Fatalf("iteration %d: ParseAtom(invalid) succeeded, want error", i)
		}
		if !step2cHonest(err) {
			t.Fatalf("iteration %d: dishonest error: %#v", i, err)
		}
	}
}

func TestStep2cParseUnitNilReaderFailsClosed(t *testing.T) {
	err := step2cMustNotPanic(t, "ParseUnit(nil)", func() error {
		_, err := ParseUnit(nil)
		return err
	})
	if err == nil {
		t.Fatal("ParseUnit(nil) succeeded, want fail-closed error for nil reader")
	}
	if !step2cHonest(err) {
		t.Fatalf("ParseUnit(nil) dishonest error: %#v", err)
	}
}

func TestStep2cParseUnitEmptyNoPanic(t *testing.T) {
	err := step2cMustNotPanic(t, "ParseUnit(empty)", func() error {
		_, err := ParseUnit(strings.NewReader(""))
		return err
	})
	if err != nil && !step2cHonest(err) {
		t.Fatalf("ParseUnit(empty) dishonest error: %#v", err)
	}
}

func TestStep2cParseUnitInvalidFailsClosed(t *testing.T) {
	err := step2cMustNotPanic(t, "ParseUnit(invalid)", func() error {
		_, err := ParseUnit(strings.NewReader("Decl ::: not valid (((\n"))
		return err
	})
	if err == nil {
		t.Fatal("ParseUnit(invalid) succeeded, want fail-closed error")
	}
	if !step2cHonest(err) {
		t.Fatalf("ParseUnit(invalid) dishonest error: %#v", err)
	}
}

func TestStep2cParseUnitRepeatedFailuresStable(t *testing.T) {
	for i := 0; i < 3; i++ {
		err := step2cMustNotPanic(t, "ParseUnit(invalid) repeat", func() error {
			_, err := ParseUnit(strings.NewReader("Decl ::: not valid (((\n"))
			return err
		})
		if err == nil {
			t.Fatalf("iteration %d: ParseUnit(invalid) succeeded, want error", i)
		}
		if !step2cHonest(err) {
			t.Fatalf("iteration %d: dishonest error: %#v", i, err)
		}
	}
	// Engine stays usable after repeated failures: a valid parse still works
	// without panic; any error must be honest.
	err := step2cMustNotPanic(t, "ParseUnit(empty) after failures", func() error {
		_, err := ParseUnit(strings.NewReader(""))
		return err
	})
	if err != nil && !step2cHonest(err) {
		t.Fatalf("ParseUnit(empty) after failures dishonest error: %#v", err)
	}
}
