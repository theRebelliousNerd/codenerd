package retrieval

import (
	"fmt"
	"testing"
	"time"
)

// Equal scores are routine (single-keyword files share one weight); without a
// tiebreak the limit below the sort keeps a different subset run to run.
func TestRankFiles_TiesBreakByPath(t *testing.T) {
	r := NewSparseRetriever(DefaultSparseRetrieverConfig("."))
	hits := []KeywordHit{
		{FilePath: "zebra.go", Keyword: "alpha"},
		{FilePath: "apple.go", Keyword: "alpha"},
		{FilePath: "mango.go", Keyword: "alpha"},
	}
	kw := &IssueKeywords{Weights: map[string]float64{"alpha": 0.5}}
	got := r.RankFiles(hits, kw, 2)
	if len(got) != 2 {
		t.Fatalf("limited rank = %d candidates, want 2", len(got))
	}
	if got[0].FilePath != "apple.go" || got[1].FilePath != "mango.go" {
		t.Errorf("tie order = [%s %s], want [apple.go mango.go]", got[0].FilePath, got[1].FilePath)
	}
	for _, c := range got {
		if len(c.Keywords) != 1 || c.Keywords[0] != "alpha" {
			t.Errorf("candidate %s keywords = %v", c.FilePath, c.Keywords)
		}
	}
}

// The keyword list renders into Tier 2 selection reasons; map iteration order
// would make those strings nondeterministic.
func TestRankFiles_KeywordsSorted(t *testing.T) {
	r := NewSparseRetriever(DefaultSparseRetrieverConfig("."))
	hits := []KeywordHit{
		{FilePath: "a.go", Keyword: "zebra"},
		{FilePath: "a.go", Keyword: "apple"},
		{FilePath: "a.go", Keyword: "mango"},
	}
	got := r.RankFiles(hits, &IssueKeywords{}, 0)
	if len(got) != 1 {
		t.Fatalf("candidates = %d, want 1", len(got))
	}
	want := []string{"apple", "mango", "zebra"}
	for i, kw := range want {
		if got[0].Keywords[i] != kw {
			t.Fatalf("keywords = %v, want %v", got[0].Keywords, want)
		}
	}
}

func TestRankFiles_NilKeywords(t *testing.T) {
	r := NewSparseRetriever(DefaultSparseRetrieverConfig("."))
	got := r.RankFiles([]KeywordHit{{FilePath: "a.go", Keyword: "x"}}, nil, 0)
	if len(got) != 1 {
		t.Fatalf("candidates = %d, want 1", len(got))
	}
	if got[0].RelevanceScore != 0.3 {
		t.Errorf("default-weighted score = %v, want 0.3", got[0].RelevanceScore)
	}
}

// Zero means default throughout both configs: a bare config must behave like
// the default one, not yield an empty context or an unbounded never-hit cache.
func TestConfigs_ZeroMeansDefault(t *testing.T) {
	r := NewSparseRetriever(&SparseRetrieverConfig{WorkDir: "."})
	if r.cache.maxSize != 1000 {
		t.Errorf("cache size = %d, want 1000", r.cache.maxSize)
	}
	if r.cache.ttl != 5*time.Minute {
		t.Errorf("cache ttl = %v, want 5m", r.cache.ttl)
	}
	b := NewTieredContextBuilder(&TieredContextConfig{WorkDir: "."})
	if b.maxTier1 != 15 || b.maxTier2 != 20 || b.maxTier3 != 10 || b.maxTier4 != 5 {
		t.Errorf("tier caps = %d/%d/%d/%d, want 15/20/10/5",
			b.maxTier1, b.maxTier2, b.maxTier3, b.maxTier4)
	}
	// An explicit zero on one tier still disables that tier.
	single := NewTieredContextBuilder(&TieredContextConfig{
		WorkDir: ".", Tier1Budget: 0.5, Tier2Budget: 0.5, Tier3Budget: 0, Tier4Budget: 0, MaxTotal: 10,
	})
	if single.maxTier3 != 0 || single.maxTier4 != 0 || single.maxTier1 != 5 {
		t.Errorf("explicit-zero tiers not honored: %+v", single)
	}
}

// keyword_hit facts must emit in a stable order so EDB snapshots diff cleanly.
func TestTieredContextFacts_KeywordHitsOrdered(t *testing.T) {
	tc := &TieredContext{
		Files: []ContextFile{{FilePath: "a.go", Tier: 2, RelevanceScore: 0.5}},
		Candidates: []CandidateFile{{
			FilePath: "a.go", RelevanceScore: 0.5,
			Hits: []KeywordHit{
				{FilePath: "a.go", Keyword: "zebra"},
				{FilePath: "a.go", Keyword: "apple"},
				{FilePath: "a.go", Keyword: "mango"},
			},
		}},
	}
	var first string
	for i := 0; i < 10; i++ {
		facts := TieredContextFacts("/issue_1", tc, ".")
		var order string
		for _, f := range facts {
			if f.Predicate == "keyword_hit" {
				order += fmt.Sprintf("%v;", f.Args[1])
			}
		}
		if i == 0 {
			first = order
			if first == "" {
				t.Fatal("no keyword_hit facts emitted")
			}
		} else if order != first {
			t.Fatalf("run %d order %q differs from first %q", i, order, first)
		}
	}
}
