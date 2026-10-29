package store

import (
	"path/filepath"
	"strings"
	"testing"
)

func newLongGoalBoundsStore(t *testing.T, contents []string) *LocalStore {
	t.Helper()
	s, err := NewLocalStore(filepath.Join(t.TempDir(), "bounds.db"))
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

// Long campaign-goal queries must never fail with SQLite's
// "Expression tree is too large". This is the regression the bound was added for.
func TestVectorRecallBounds_LongGoalNeverOverflows(t *testing.T) {
	s := newLongGoalBoundsStore(t, []string{
		"the campaign orchestrates neutron star navigation with plasma conduits",
		"session hydration restores turns traces and similar content for reasoning",
		"unrelated filler about bakery recipes and sourdough starter",
	})

	// Several paragraphs of goal text: 400 distinct keywords plus the two
	// probe phrases above embedded in the middle.
	var sb strings.Builder
	for i := 0; i < 400; i++ {
		sb.WriteString(" fillerword")
		if i == 200 {
			sb.WriteString(" campaign orchestrates neutron star navigation")
		}
		if i == 300 {
			sb.WriteString(" session hydration restores turns")
		}
	}
	longQuery := sb.String()

	got, err := s.VectorRecall(longQuery, 10)
	if err != nil {
		t.Fatalf("VectorRecall long query failed: %v", err)
	}
	if strings.Contains(strings.ToLower(queryErrString(err)), "expression tree is too large") {
		t.Fatalf("long query hit expression overflow")
	}
	if len(got) == 0 {
		t.Fatalf("expected at least one match for long goal query, got 0")
	}
	found := false
	for _, e := range got {
		if strings.Contains(e.Content, "neutron star") || strings.Contains(e.Content, "session hydration") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("long query did not return seeded probe content: %+v", got)
	}
	if len(got) > 10 {
		t.Fatalf("expected at most limit=10 results, got %d", len(got))
	}
}

func queryErrString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Duplicate keywords with different cases must collapse to one LIKE branch
// yet still match.
func TestVectorRecallBounds_DedupeAndCaseFolding(t *testing.T) {
	s := newLongGoalBoundsStore(t, []string{
		"alpha beta gamma delta",
	})

	cases := []struct {
		name  string
		query string
	}{
		{name: "single", query: "alpha"},
		{name: "duplicates", query: "alpha alpha alpha alpha"},
		{name: "mixed_case_duplicates", query: "AlPhA ALPHA alpha aLpHa"},
		{name: "duplicates_with_other", query: "alpha beta alpha BETA Alpha"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.VectorRecall(tc.query, 10)
			if err != nil {
				t.Fatalf("VectorRecall(%q) failed: %v", tc.query, err)
			}
			if len(got) == 0 {
				t.Fatalf("VectorRecall(%q) returned 0 results, want >=1", tc.query)
			}
			if !strings.Contains(got[0].Content, "alpha") {
				t.Fatalf("VectorRecall(%q) top hit %q does not contain alpha", tc.query, got[0].Content)
			}
		})
	}
}

// A single absurd token (paste, hash, base64 blob) must be truncated on rune
// boundaries, not blow up the query and not fail.
func TestVectorRecallBounds_AbsurdTokenTruncated(t *testing.T) {
	prefix := strings.Repeat("a", 64)
	s := newLongGoalBoundsStore(t, []string{
		"row holding truncated prefix " + prefix + " tail-marker",
		"entirely unrelated row about gardening",
	})

	cases := []struct {
		name  string
		query string
	}{
		{name: "long_ascii_token", query: strings.Repeat("a", 500)},
		{name: "long_token_with_match_word", query: "gardening " + strings.Repeat("z", 600)},
		{name: "long_unicode_token", query: strings.Repeat("\u00e9", 300)},
		{name: "mixed_long_and_short", query: "gardening " + strings.Repeat("q", 500) + " " + strings.Repeat("q", 500)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.VectorRecall(tc.query, 10)
			if err != nil {
				t.Fatalf("VectorRecall long-token case %q failed: %v", tc.name, err)
			}
			// Must not error; result count is bounded by limit.
			if len(got) > 10 {
				t.Fatalf("case %q returned %d > limit", tc.name, len(got))
			}
		})
	}

	// The truncated token must still match on its surviving prefix: the first
	// 64 runes of the 500-a token equal the stored prefix.
	got, err := s.VectorRecall(strings.Repeat("a", 500), 10)
	if err != nil {
		t.Fatalf("VectorRecall truncated prefix query failed: %v", err)
	}
	found := false
	for _, e := range got {
		if strings.Contains(e.Content, prefix) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("truncated long token did not match row holding its 64-rune prefix")
	}
}

// More distinct keywords than the cap must be cut to the cap: still
// succeeds, still respects limit, still finds a probe word placed early.
func TestVectorRecallBounds_KeywordCapEnforced(t *testing.T) {
	s := newLongGoalBoundsStore(t, []string{
		"zebrastripe unique probe word for cap test",
		"another unrelated row",
	})

	var sb strings.Builder
	sb.WriteString("zebrastripe")
	for i := 0; i < 200; i++ {
		sb.WriteString(" distinctfiller")
		sb.WriteString(strings.Repeat("x", i%7))
	}
	// Make every filler token unique so dedupe cannot save us; only the cap can.
	var sb2 strings.Builder
	sb2.WriteString("zebrastripe")
	for i := 0; i < 200; i++ {
		sb2.WriteString(" capword")
		sb2.WriteString(strings.TrimSpace(strings.Repeat(" ", 0)))
		sb2.WriteString(strings.Repeat("y", 1))
		// ensure uniqueness via index embedded as letters
		sb2.WriteString(string(rune('a' + (i % 26))))
		sb2.WriteString(string(rune('a' + ((i / 26) % 26))))
		sb2.WriteString(string(rune('a' + ((i / 676) % 26))))
	}
	_ = sb.String()

	got, err := s.VectorRecall(sb2.String(), 5)
	if err != nil {
		t.Fatalf("VectorRecall over-cap query failed: %v", err)
	}
	if len(got) == 0 {
		t.Fatalf("over-cap query returned 0 results, want probe hit")
	}
	if len(got) > 5 {
		t.Fatalf("over-cap query returned %d > limit 5", len(got))
	}
	found := false
	for _, e := range got {
		if strings.Contains(e.Content, "zebrastripe") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("over-cap query lost the probe word zebrastripe")
	}
}

// Empty / whitespace queries and limit edges must keep their contracts.
func TestVectorRecallBounds_EmptyAndLimitEdges(t *testing.T) {
	s := newLongGoalBoundsStore(t, []string{"hello world"})

	for _, q := range []string{"", "   ", "\n\t  \n"} {
		got, err := s.VectorRecall(q, 10)
		if err != nil {
			t.Fatalf("VectorRecall(%q) failed: %v", q, err)
		}
		if len(got) != 0 {
			t.Fatalf("VectorRecall(%q) = %d results, want 0", q, len(got))
		}
	}

	// Non-positive limit falls back to the default instead of erroring.
	for _, lim := range []int{0, -1, -100} {
		got, err := s.VectorRecall("hello", lim)
		if err != nil {
			t.Fatalf("VectorRecall limit=%d failed: %v", lim, err)
		}
		if len(got) == 0 {
			t.Fatalf("VectorRecall limit=%d returned 0, want default-limit hit", lim)
		}
	}

	// Limit of 1 is honored even for a multi-keyword query.
	got, err := s.VectorRecall("hello world extra filler words here", 1)
	if err != nil {
		t.Fatalf("VectorRecall limit=1 failed: %v", err)
	}
	if len(got) > 1 {
		t.Fatalf("VectorRecall limit=1 returned %d results", len(got))
	}
}
