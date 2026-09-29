package core

import (
	"strings"
	"testing"
)

func TestNewPendingEditFact_IdentityNotPrefix(t *testing.T) {
	a := strings.Repeat("A", 250) + "one"
	b := strings.Repeat("A", 250) + "two"

	fa := NewPendingEditFact("a.go", a)
	fb := NewPendingEditFact("b.go", b)
	if len(fa.Args) != 2 || len(fb.Args) != 2 {
		t.Fatalf("arity fa=%d fb=%d, want 2", len(fa.Args), len(fb.Args))
	}
	gotA, _ := fa.Args[1].(string)
	gotB, _ := fb.Args[1].(string)
	if gotA == gotB {
		t.Fatal("two bodies that share a 250-byte prefix collapsed to one fact arg")
	}
	if gotA != factContentIdentity(a) || gotB != factContentIdentity(b) {
		t.Fatalf("args = %q, %q", gotA, gotB)
	}
	if strings.Contains(gotA, "AAA") || strings.Contains(gotA, "...") {
		t.Fatalf("content arg is a prefix: %q", gotA)
	}
	if !strings.HasPrefix(gotA, "sha256:") || !strings.Contains(gotA, " bytes:") {
		t.Fatalf("content arg = %q, want sha256:<hex> bytes:<n>", gotA)
	}

	empty := NewPendingEditFact("a.go", "")
	if empty.Args[1] != "" {
		t.Fatalf("empty content = %#v, want \"\"", empty.Args[1])
	}
}
