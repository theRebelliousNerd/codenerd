package types

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestTruncateFactText_ShortInputUnchanged keeps the common case free: almost
// every tool result is well under the bound and must pass through untouched.
func TestTruncateFactText_ShortInputUnchanged(t *testing.T) {
	for _, s := range []string{"", "ok", strings.Repeat("x", MaxFactTextBytes)} {
		if got := TruncateFactText(s); got != s {
			t.Errorf("input of %d bytes was modified", len(s))
		}
	}
}

// TestTruncateFactText_BoundsAndAnnounces pins the two properties that make the
// bound safe: the stored value is bounded, and the truncation is visible.
//
// A silently truncated result is one an operator reading the fact store — or a
// rule author reasoning about it — will take for the whole story.
func TestTruncateFactText_BoundsAndAnnounces(t *testing.T) {
	original := strings.Repeat("a", MaxFactTextBytes*3)
	got := TruncateFactText(original)

	if len(got) >= len(original) {
		t.Fatalf("output (%d bytes) is not smaller than input (%d bytes)", len(got), len(original))
	}
	if !strings.HasPrefix(got, strings.Repeat("a", 100)) {
		t.Error("output does not begin with the original content")
	}
	if !strings.Contains(got, "truncated") {
		t.Errorf("truncation is silent: %q", got[len(got)-80:])
	}
	// The marker must state both halves so the reader can judge what is missing.
	if !strings.Contains(got, "of 12288 bytes") {
		t.Errorf("marker does not state the original size: %q", got[len(got)-80:])
	}
}

// TestTruncateFactText_CutsOnRuneBoundary keeps the value valid UTF-8. A half
// rune in a fact argument breaks the Mangle string encoding downstream, which
// surfaces as a kernel parse failure far from here.
func TestTruncateFactText_CutsOnRuneBoundary(t *testing.T) {
	// Three-byte runes, so the byte cap lands mid-rune for at least one offset.
	for _, pad := range []int{0, 1, 2} {
		s := strings.Repeat("x", pad) + strings.Repeat("世", MaxFactTextBytes)
		got := TruncateFactText(s)
		if !utf8.ValidString(got) {
			t.Fatalf("pad=%d produced invalid UTF-8", pad)
		}
		if len(got) > MaxFactTextBytes+64 {
			t.Fatalf("pad=%d output %d bytes, want <= %d plus marker", pad, len(got), MaxFactTextBytes)
		}
	}
}

// TestTruncateFactText_Idempotent: truncating an already-truncated value must
// not stack markers, which would happen if a value round-tripped through the
// fact store.
func TestTruncateFactText_Idempotent(t *testing.T) {
	once := TruncateFactText(strings.Repeat("a", MaxFactTextBytes*2))
	twice := TruncateFactText(once)
	if once != twice {
		t.Errorf("truncation is not idempotent:\nonce:  %d bytes\ntwice: %d bytes", len(once), len(twice))
	}
}

// TestTruncateFactText_NeverExceedsCap is the property the marker budget buys:
// the value handed to the kernel is bounded including its marker, whatever the
// input size or rune width.
func TestTruncateFactText_NeverExceedsCap(t *testing.T) {
	inputs := []string{
		strings.Repeat("a", MaxFactTextBytes+1),
		strings.Repeat("a", MaxFactTextBytes*100),
		strings.Repeat("世", MaxFactTextBytes),
		strings.Repeat("🙂", MaxFactTextBytes),
		strings.Repeat("a", 7) + strings.Repeat("世", MaxFactTextBytes),
	}
	for i, in := range inputs {
		got := TruncateFactText(in)
		if len(got) > MaxFactTextBytes {
			t.Errorf("input %d: output %d bytes exceeds the %d cap", i, len(got), MaxFactTextBytes)
		}
		if !utf8.ValidString(got) {
			t.Errorf("input %d: output is not valid UTF-8", i)
		}
	}
}
