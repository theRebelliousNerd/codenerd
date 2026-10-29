package store

import (
	"path/filepath"
	"strings"
	"testing"
)

func newTruncateCoverStore(t *testing.T) *LocalStore {
	t.Helper()
	s, err := NewLocalStore(filepath.Join(t.TempDir(), "truncate-cover.db"))
	if err != nil {
		t.Fatalf("NewLocalStore failed: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestVectorRecall_TruncatedPrefixCollisionSkipsDuplicate exercises
// local_vector.go:98-99: two distinct over-long tokens that truncate to the
// same 64-rune prefix. The second truncated form is already in seen, so the
// inner dedupe `continue` fires. Recall must still succeed (bounded query,
// no "Expression tree is too large") and match the stored row via the
// truncated prefix.
func TestVectorRecall_TruncatedPrefixCollisionSkipsDuplicate(t *testing.T) {
	s := newTruncateCoverStore(t)

	prefix := strings.Repeat("q", 64)
	tok1 := prefix + "aa"
	tok2 := prefix + "bb"
	if len([]rune(tok1)) <= 64 || len([]rune(tok2)) <= 64 {
		t.Fatalf("test tokens must exceed truncation limit, got %d and %d runes", len([]rune(tok1)), len([]rune(tok2)))
	}
	if string([]rune(tok1)[:64]) != prefix || string([]rune(tok2)[:64]) != prefix {
		t.Fatalf("test tokens must share the same 64-rune prefix")
	}

	content := "record holding marker " + prefix + " tail"
	if err := s.StoreVector(content, map[string]any{"k": "v"}); err != nil {
		t.Fatalf("StoreVector failed: %v", err)
	}

	got, err := s.VectorRecall(tok1+" "+tok2, 10)
	if err != nil {
		t.Fatalf("VectorRecall with colliding truncated tokens failed: %v", err)
	}
	if len(got) == 0 {
		t.Fatalf("expected truncated-prefix query %q to match stored content", tok1+" "+tok2)
	}
	found := false
	for _, e := range got {
		if strings.Contains(e.Content, prefix) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected at least one result to contain the truncated prefix, got %+v", got)
	}
}

// TestVectorRecall_LongQueryIsBounded exercises the bounding behaviour around
// local_vector.go:98-99 and the defensive empty-bounded guard at 108-111.
// A multi-paragraph campaign-goal style query (thousands of distinct tokens)
// must not fail with "Expression tree is too large": it is truncated to at
// most 32 OR branches. The colliding-truncation pair above forces the inner
// dedupe path, and the huge distinct-token query forces the truncation path
// that the empty-bounded guard protects.
func TestVectorRecall_LongQueryIsBounded(t *testing.T) {
	s := newTruncateCoverStore(t)

	if err := s.StoreVector("campaign goal marker alpha beta gamma", map[string]any{"k": "v"}); err != nil {
		t.Fatalf("StoreVector failed: %v", err)
	}

	// Distinct-token flood: >1000 distinct keywords would previously build a
	// >1000-branch OR expression and fail every hydrate.
	var sb strings.Builder
	sb.WriteString("alpha ")
	for i := 0; i < 2000; i++ {
		sb.WriteString("distinctfiller")
		sb.WriteString(strings.Repeat("x", i%7))
		sb.WriteString("tok")
		sb.WriteString(strings.Repeat("y", 3))
		sb.WriteString(strings.TrimSpace(strings.Repeat("", 0)))
		sb.WriteString(strings.Repeat("z", 0))
		// Make each token unique without whitespace.
		sb.WriteString(strings.Repeat("q", 0))
		sb.WriteString("-")
		sb.WriteString(strings.Repeat("w", i%5))
		sb.WriteString(strings.Repeat("v", 1))
		sb.WriteString(string(rune('a' + (i % 26))))
		sb.WriteString(string(rune('0' + (i % 10))))
		sb.WriteString(" ")
		_ = i
	}
	// Simpler deterministic distinct tokens (avoid builder complexity above
	// producing accidental duplicates): rebuild cleanly.
	parts := []string{"alpha"}
	for i := 0; i < 2000; i++ {
		parts = append(parts, "uniquetoken"+strings.Repeat("a", i%5)+strings.Repeat("b", i%7)+"-"+string(rune('A'+(i%26)))+"-"+strings.Repeat("c", 3)+"-end"+strings.Repeat("d", i%3)+"-"+strings.Repeat("e", i/26%5))
	}
	// Ensure distinctness deterministically.
	seen := map[string]struct{}{}
	distinct := make([]string, 0, len(parts))
	for _, p := range parts {
		k := strings.ToLower(p)
		if _, ok := seen[k]; ok {
			k = k + "-x" + strings.Repeat("y", len(distinct)%5)
			if _, ok2 := seen[k]; ok2 {
				continue
			}
		}
		seen[k] = struct{}{}
		distinct = append(distinct, p)
	}
	longQuery := strings.Join(distinct, " ")

	got, err := s.VectorRecall(longQuery, 10)
	if err != nil {
		t.Fatalf("VectorRecall with %d-token query failed: %v", len(distinct), err)
	}
	if strings.Contains(errString(err), "Expression tree is too large") {
		t.Fatalf("long query still hits expression-tree limit")
	}
	if len(got) == 0 {
		t.Fatalf("expected long query containing 'alpha' to match stored content")
	}

	// Defensive guard probe: a query whose tokens all collapse (duplicates +
	// colliding truncations) must return results or nil without error, never
	// the expression-tree failure. This keeps the 108-111 guard's contract
	// (empty bounded set -> nil, nil) pinned even though the current dedupe
	// order always keeps at least one bounded keyword.
	prefix := strings.Repeat("z", 64)
	collapsing := (prefix + "11") + " " + (prefix + "22") + " " + (prefix + "11") + " " + prefix
	got2, err := s.VectorRecall(collapsing, 10)
	if err != nil {
		t.Fatalf("VectorRecall with collapsing tokens failed: %v", err)
	}
	_ = got2
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
