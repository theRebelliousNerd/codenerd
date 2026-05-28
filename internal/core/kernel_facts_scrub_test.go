package core

import "testing"

// TestLooksLikePointerHex pins the detector that gates the
// persisted-fact pointer-leak scrubber. False positives here would
// corrupt user EDB facts; false negatives leak the original bug.
func TestLooksLikePointerHex(t *testing.T) {
	yes := []string{
		"0x7ff63be770e0", // the observed leak from kernel.log
		"0X7ff63be770e0", // tolerate uppercase X
		"0xDEADBEEF",
		"0x00000000",
		"0xabcdef12",
	}
	for _, s := range yes {
		if !looksLikePointerHex(s) {
			t.Errorf("expected pointer-hex true for %q", s)
		}
	}

	no := []string{
		"",
		"0",
		"0x",         // too short
		"0xz",        // non-hex
		"7ff63be77",  // no 0x prefix
		"0x7ff 63b",  // space inside
		"path/to/file",
		"\"0x7ff63be770e0\"", // quoted — sanitizer sees the raw string, not the encoded form
		"0x7ff63be770e0abc1234567", // too long (>20)
		"42",
		"hello",
	}
	for _, s := range no {
		if looksLikePointerHex(s) {
			t.Errorf("expected pointer-hex false for %q", s)
		}
	}
}

// TestSanitizeFactScrubs proves the LoadFacts path rescues a poisoned
// prompt_atom row (the exact shape recorded in kernel.log on 2026-05-28)
// instead of forwarding the bad string to the kernel and crashing
// fn:plus during evaluation.
func TestSanitizeFactScrubs(t *testing.T) {
	in := Fact{
		Predicate: "prompt_atom",
		Args:      []any{"id", "/identity", 80, "0x7ff63be770e0", "/true"},
	}
	out := sanitizeFactForNumericPredicates(in)
	v, ok := out.Args[3].(int64)
	if !ok {
		t.Fatalf("arg #3 should be coerced to int64, got %T (%v)", out.Args[3], out.Args[3])
	}
	if v != 0 {
		t.Fatalf("scrubbed value should be 0, got %d", v)
	}
}

// TestSanitizeFactPreservesGoodNumbers proves the scrubber doesn't
// touch valid TokenCount / Priority numbers.
func TestSanitizeFactPreservesGoodNumbers(t *testing.T) {
	in := Fact{
		Predicate: "prompt_atom",
		Args:      []any{"id", "/identity", 80, 1024, "/true"},
	}
	out := sanitizeFactForNumericPredicates(in)
	if out.Args[3] != 1024 {
		t.Fatalf("good TokenCount mutated; got %v", out.Args[3])
	}
}
