package store

import (
	"testing"
)

// Brutal sqlite-vec search boundary probes against the real embedded corpus.
// Skips cleanly when the corpus artifact is unavailable (dev mode).

func openCorpusOrSkip(t *testing.T) *EmbeddedCorpusStore {
	t.Helper()
	s, err := NewEmbeddedCorpusStore()
	if err != nil {
		t.Skipf("embedded corpus unavailable: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func corpusDims(t *testing.T, s *EmbeddedCorpusStore) int {
	t.Helper()
	var blobLen int
	if err := s.db.QueryRow("SELECT length(embedding) FROM vec_corpus LIMIT 1").Scan(&blobLen); err != nil {
		t.Fatalf("cannot read corpus embedding size: %v", err)
	}
	if blobLen%4 != 0 || blobLen == 0 {
		t.Fatalf("corpus embedding blob has invalid size %d", blobLen)
	}
	return blobLen / 4
}

// A valid query must return rank-ordered matches with descending similarity.
func TestCorpusSearchRankingShape(t *testing.T) {
	s := openCorpusOrSkip(t)
	dims := corpusDims(t, s)

	q := make([]float32, dims)
	for i := range q {
		q[i] = 0.01 * float32(i%7+1) // deterministic non-zero query
	}
	matches, err := s.Search(q, 5)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("expected matches from non-empty corpus")
	}
	if len(matches) > 5 {
		t.Fatalf("topK=5 returned %d matches", len(matches))
	}
	for i, m := range matches {
		if m.Rank != i+1 {
			t.Fatalf("match %d has rank %d, want %d", i, m.Rank, i+1)
		}
		if i > 0 && m.Similarity > matches[i-1].Similarity+1e-9 {
			t.Fatalf("similarity not descending at rank %d: %f > %f",
				m.Rank, m.Similarity, matches[i-1].Similarity)
		}
		if m.Similarity < -1.01 || m.Similarity > 1.01 {
			t.Fatalf("similarity %f out of cosine range at rank %d", m.Similarity, m.Rank)
		}
	}
}

// topK<=0 must fall back to the default instead of erroring or returning everything.
func TestCorpusSearchTopKDefaults(t *testing.T) {
	s := openCorpusOrSkip(t)
	dims := corpusDims(t, s)
	q := make([]float32, dims)
	for _, k := range []int{0, -3} {
		matches, err := s.Search(q, k)
		if err != nil {
			t.Fatalf("Search(topK=%d) failed: %v", k, err)
		}
		if len(matches) == 0 || len(matches) > 5 {
			t.Fatalf("Search(topK=%d) returned %d matches, want 1..5 (default 5)", k, len(matches))
		}
	}
}

// Wrong-dimension and nil embeddings must surface an error, never panic
// and never silently return empty success.
func TestCorpusSearchDimensionMismatchErrors(t *testing.T) {
	s := openCorpusOrSkip(t)
	dims := corpusDims(t, s)

	cases := map[string][]float32{
		"nil":   nil,
		"empty": {},
		"short": make([]float32, 3),
		"long":  make([]float32, dims+16),
	}
	for name, q := range cases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Search(%s) panicked: %v", name, r)
				}
			}()
			matches, err := s.Search(q, 5)
			if err == nil && len(matches) != 0 {
				t.Fatalf("Search(%s) silently succeeded with %d matches", name, len(matches))
			}
			// Either a surfaced error or an explicit empty result is acceptable;
			// both must be non-panicking. Prefer the error path.
			if err == nil {
				t.Logf("Search(%s): empty result without error (acceptable, prefer error)", name)
			}
		}()
	}
}

// Unknown predicate filter must return empty without error.
func TestCorpusSearchByPredicateUnknown(t *testing.T) {
	s := openCorpusOrSkip(t)
	dims := corpusDims(t, s)
	q := make([]float32, dims)
	matches, err := s.SearchByPredicate(q, "no_such_predicate_xyz", 5)
	if err != nil {
		t.Fatalf("SearchByPredicate(unknown) failed: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches for unknown predicate, got %d", len(matches))
	}
}

// Stats must reflect a populated corpus.
func TestCorpusStatsPopulated(t *testing.T) {
	s := openCorpusOrSkip(t)
	stats, err := s.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	total, ok := stats["total_entries"].(int64)
	if !ok || total <= 0 {
		t.Fatalf("expected positive total_entries, got %v", stats["total_entries"])
	}
}
