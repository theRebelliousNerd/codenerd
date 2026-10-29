package store

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func newExtraCoverStore(t *testing.T, contents []string) *LocalStore {
	t.Helper()
	s, err := NewLocalStore(filepath.Join(t.TempDir(), "extracover.db"))
	if err != nil {
		t.Fatalf("NewLocalStore failed: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	for i, c := range contents {
		if err := s.StoreVector(c, nil); err != nil {
			t.Fatalf("StoreVector %d failed: %v", i, err)
		}
	}
	return s
}

// Tokens that vanish during cleaning (punctuation-only, symbols, SQL
// wildcards) must be skipped without failing and without matching everything.
func TestVectorRecallExtra_SkippedTokensDoNotMatchAll(t *testing.T) {
	s := newExtraCoverStore(t, []string{
		"plain row about hydration",
		"another plain row about campaigns",
	})

	skipped := []string{
		"!!! ??? ... ,,, ;;; :::",
		"%%% ___ ((( )))",
		"\" ' \\",
		"% _ % _ % _",
		"!!! ???",
		"\u2603 \U0001F680 \u00e9\u0301",
		"hello !!! ??? world",
		"   !!!   ???   ",
		"...",
		"--- ___ ---",
	}
	for _, q := range skipped {
		got, err := s.VectorRecall(q, 10)
		if err != nil {
			t.Fatalf("VectorRecall(%q) failed: %v", q, err)
		}
		if len(got) > 10 {
			t.Fatalf("VectorRecall(%q) returned %d > limit", q, len(got))
		}
	}

	// A punctuation-only query must not match every row (that would mean the
	// empty branch built a degenerate LIKE '%%' query).
	got, err := s.VectorRecall("!!! ??? ...", 10)
	if err != nil {
		t.Fatalf("punct-only recall failed: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("punct-only query should match nothing, got %d: %+v", len(got), got)
	}

	// Mixed valid + skipped tokens still finds the valid word.
	got, err = s.VectorRecall("!!! hydration ???", 10)
	if err != nil {
		t.Fatalf("mixed recall failed: %v", err)
	}
	if len(got) == 0 {
		t.Fatalf("mixed valid+punct query returned 0, want hydration hit")
	}
}

// Any number of distinct keywords must be capped: throw far more than any
// plausible bound so the cap/break branch is forced.
func TestVectorRecallExtra_HugeDistinctCountIsCapped(t *testing.T) {
	s := newExtraCoverStore(t, []string{
		"capprobe alpha beta",
		"unrelated gardening row",
	})

	// 5000 distinct tokens with the probe first so a first-N cap keeps it.
	var sb strings.Builder
	sb.WriteString("capprobe")
	for i := 0; i < 5000; i++ {
		fmt.Fprintf(&sb, " kw%05d", i)
	}
	huge := sb.String()

	got, err := s.VectorRecall(huge, 10)
	if err != nil {
		t.Fatalf("huge distinct recall failed: %v", err)
	}
	if len(got) > 10 {
		t.Fatalf("huge query returned %d > limit 10", len(got))
	}
	if len(got) == 0 {
		t.Fatalf("huge query with leading probe returned 0, want capprobe hit")
	}

	// Same huge query with tiny limit still honors the limit.
	got, err = s.VectorRecall(huge, 3)
	if err != nil {
		t.Fatalf("huge recall limit=3 failed: %v", err)
	}
	if len(got) > 3 {
		t.Fatalf("huge recall limit=3 returned %d", len(got))
	}

	// Reversed: probe last. Must still succeed (cap may drop it, but must not error).
	var sb2 strings.Builder
	for i := 0; i < 5000; i++ {
		fmt.Fprintf(&sb2, " zz%05d", i)
	}
	sb2.WriteString(" capprobe")
	got, err = s.VectorRecall(sb2.String(), 10)
	if err != nil {
		t.Fatalf("huge trailing-probe recall failed: %v", err)
	}
	if len(got) > 10 {
		t.Fatalf("trailing-probe query returned %d > limit", len(got))
	}
}
