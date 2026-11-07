package core

import (
	"bytes"
	"fmt"
	"sync/atomic"
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

// TestInternFact_StringArgsShared documents the current write-path
// contract: plain string args are passed through to avoid re-boxing
// 150k distinct boot strings (spec refs, fingerprints) through unique.Make
// for zero sharing. Small-vocabulary MangleAtom args still share backing
// memory via the predicate cache; that is the surviving memory win.
func TestInternFact_StringArgsShared(t *testing.T) {
	a := freshString("repeated-value")
	b := freshString("repeated-value")
	if stringHeader(a) == stringHeader(b) {
		t.Fatalf("test setup failure: a and b share memory before interning")
	}

	fa := internFact(Fact{Predicate: "p", Args: []any{MangleAtom(freshString("/repeated")), 1}})
	fb := internFact(Fact{Predicate: "p", Args: []any{MangleAtom(freshString("/repeated")), 2}})

	sa, ok := fa.Args[0].(MangleAtom)
	if !ok {
		t.Fatalf("expected atom arg, got %T", fa.Args[0])
	}
	sb, ok := fb.Args[0].(MangleAtom)
	if !ok {
		t.Fatalf("expected atom arg, got %T", fb.Args[0])
	}
	if stringHeader(string(sa)) != stringHeader(string(sb)) {
		t.Errorf("intern did not share backing memory across atom args: %x vs %x",
			stringHeader(string(sa)), stringHeader(string(sb)))
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

var internPinSeq uint64

func freshPinPredicate() string {
	n := atomic.AddUint64(&internPinSeq, 1)
	return freshString(fmt.Sprintf("pin-pred-%d", n))
}

// TestInternPredicate_HitReturnsCached takes the hit branch of the first
// check: a poisoned entry must win over unique.Make. Forcing ok to false
// skips the hit and returns the re-hashed input instead.
func TestInternPredicate_HitReturnsCached(t *testing.T) {
	key := freshPinPredicate()
	poison := freshString("poison-canonical")
	predicateInternCacheMu.Lock()
	predicateInternCache[key] = poison
	predicateInternCacheMu.Unlock()
	defer func() {
		predicateInternCacheMu.Lock()
		delete(predicateInternCache, key)
		predicateInternCacheMu.Unlock()
	}()
	got := internPredicate(key)
	if got != poison {
		t.Fatalf("hit returned %q, want cached %q", got, poison)
	}
	if stringHeader(got) != stringHeader(poison) {
		t.Errorf("hit did not reuse cached backing")
	}
}

func TestInternPredicate_CacheHitSharesBacking(t *testing.T) {
	p := freshString("test-line43-hit-pin")
	first := internPredicate(p)
	second := internPredicate(freshString("test-line43-hit-pin"))
	if first != second {
		t.Fatalf("cache hit returned different string: %q vs %q", first, second)
	}
	if stringHeader(first) != stringHeader(second) {
		t.Errorf("line 43 hit did not share backing memory: %x vs %x", stringHeader(first), stringHeader(second))
	}
}

func TestInternPredicate_ConcurrentInternSharesBacking(t *testing.T) {
	p := freshString("test-line50-race-pin")
	const n = 32
	start := make(chan struct{})
	out := make(chan string, n)
	for i := 0; i < n; i++ {
		go func() {
			<-start
			out <- internPredicate(p)
		}()
	}
	close(start)
	first := <-out
	for i := 1; i < n; i++ {
		got := <-out
		if got != first {
			t.Fatalf("concurrent intern returned different string: %q vs %q", got, first)
		}
		if stringHeader(got) != stringHeader(first) {
			t.Errorf("concurrent intern did not share backing memory: %x vs %x", stringHeader(got), stringHeader(first))
		}
	}
}

// Every string-shaped argument is interned through unique, plain strings
// included: facts that repeat a path then share one copy (570k world facts
// hold 533 MB against 592 MB without). Atoms do not enter the predicate
// cache, a strong map that would grow with every id for the life of the
// process; only predicate names, bounded by the schema, live there.
func TestInternFact_ArgumentsSharedAndCacheBounded(t *testing.T) {
	seq := atomic.AddUint64(&internPinSeq, 1)
	atom := fmt.Sprintf("/atom-%d", seq)
	path := fmt.Sprintf("internal/pkg/file-%d.go", seq)
	f1 := internFact(Fact{Predicate: "p", Args: []any{MangleAtom(freshString(atom)), freshString(path), int64(7)}})
	f2 := internFact(Fact{Predicate: "p", Args: []any{MangleAtom(freshString(atom)), freshString(path), int64(8)}})
	a1, a2 := f1.Args[0].(MangleAtom), f2.Args[0].(MangleAtom)
	if stringHeader(string(a1)) != stringHeader(string(a2)) {
		t.Fatal("two copies of one atom were not shared")
	}
	s1, s2 := f1.Args[1].(string), f2.Args[1].(string)
	if stringHeader(s1) != stringHeader(s2) {
		t.Fatal("two copies of one path were not shared")
	}
	if v, ok := f1.Args[2].(int64); !ok || v != 7 {
		t.Fatalf("non-string argument changed: %v", f1.Args[2])
	}
	predicateInternCacheMu.RLock()
	_, atomCached := predicateInternCache[atom]
	_, pathCached := predicateInternCache[path]
	predicateInternCacheMu.RUnlock()
	if atomCached || pathCached {
		t.Fatal("an argument entered the unbounded predicate cache")
	}
}
