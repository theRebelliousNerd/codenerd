package core

import (
	"bytes"
	"testing"
	"unsafe"
)

// stringHeader returns the data pointer of a string. Two interned strings
// with equal contents will share the same backing array, so their data
// pointers compare equal — that's what makes unique.Handle a memory win.
func stringHeader(s string) uintptr {
	return uintptr(unsafe.Pointer(unsafe.StringData(s)))
}

// freshString returns a string allocated at runtime so the compiler
// can't fold two identical-content callers into the same constant.
// bytes.Buffer.String() forces a copy into a new []byte → string.
func freshString(s string) string {
	var b bytes.Buffer
	b.WriteString(s)
	return b.String()
}

// TestInternFact_PredicateShared verifies that two facts with the same
// predicate name end up pointing at the same underlying string memory
// after going through internFact. This is the dominant memory win.
func TestInternFact_PredicateShared(t *testing.T) {
	a := freshString("user_intent")
	b := freshString("user_intent")
	if stringHeader(a) == stringHeader(b) {
		t.Fatalf("test setup failure: a and b share memory before interning; freshString is not freshening")
	}

	fa := internFact(Fact{Predicate: a, Args: []any{1}})
	fb := internFact(Fact{Predicate: b, Args: []any{2}})

	if fa.Predicate != fb.Predicate {
		t.Fatalf("intern produced different strings: %q vs %q", fa.Predicate, fb.Predicate)
	}
	if stringHeader(fa.Predicate) != stringHeader(fb.Predicate) {
		t.Errorf("intern did not share backing memory across facts: %x vs %x",
			stringHeader(fa.Predicate), stringHeader(fb.Predicate))
	}
}

// TestInternFact_StringArgsShared verifies that repeated string-valued
// args also share interned backing memory.
func TestInternFact_StringArgsShared(t *testing.T) {
	a := freshString("repeated-value")
	b := freshString("repeated-value")
	if stringHeader(a) == stringHeader(b) {
		t.Fatalf("test setup failure: a and b share memory before interning")
	}

	fa := internFact(Fact{Predicate: "p", Args: []any{a, 1}})
	fb := internFact(Fact{Predicate: "p", Args: []any{b, 2}})

	sa, ok := fa.Args[0].(string)
	if !ok {
		t.Fatalf("expected string arg, got %T", fa.Args[0])
	}
	sb, ok := fb.Args[0].(string)
	if !ok {
		t.Fatalf("expected string arg, got %T", fb.Args[0])
	}
	if stringHeader(sa) != stringHeader(sb) {
		t.Errorf("intern did not share backing memory across string args: %x vs %x",
			stringHeader(sa), stringHeader(sb))
	}
}

// TestInternFact_NonStringArgsPreserved asserts that non-string args are
// passed through unchanged (no allocation when interning is a no-op).
func TestInternFact_NonStringArgsPreserved(t *testing.T) {
	f := Fact{Predicate: "p", Args: []any{int64(42), float64(3.14), true, nil}}
	got := internFact(f)
	if v, ok := got.Args[0].(int64); !ok || v != 42 {
		t.Errorf("Args[0]: got %v, want 42 (int64)", got.Args[0])
	}
	if v, ok := got.Args[1].(float64); !ok || v != 3.14 {
		t.Errorf("Args[1]: got %v, want 3.14 (float64)", got.Args[1])
	}
	if v, ok := got.Args[2].(bool); !ok || v != true {
		t.Errorf("Args[2]: got %v, want true (bool)", got.Args[2])
	}
	if got.Args[3] != nil {
		t.Errorf("Args[3]: got %v, want nil", got.Args[3])
	}
}

// TestInternFact_NoArgCopyWhenNoStrings asserts that we don't pay for a
// new args slice when there are no string-shaped args to intern. This
// is the small-allocation guard around the copy path.
func TestInternFact_NoArgCopyWhenNoStrings(t *testing.T) {
	args := []any{int64(1), int64(2), int64(3)}
	f := Fact{Predicate: "p", Args: args}
	got := internFact(f)
	// Compare the underlying slice pointer via the first element address.
	if &got.Args[0] != &args[0] {
		t.Errorf("internFact copied Args even with no string args; want same slice")
	}
}

// TestInternFact_EmptyPredicate exercises the empty-string corner case.
func TestInternFact_EmptyPredicate(t *testing.T) {
	got := internFact(Fact{Predicate: "", Args: nil})
	if got.Predicate != "" {
		t.Errorf("empty predicate mangled: got %q", got.Predicate)
	}
}
