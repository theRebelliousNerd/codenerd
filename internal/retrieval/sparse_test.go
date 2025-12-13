package retrieval

import (
	"reflect"
	"testing"
	"time"
)

func TestExtractKeywords_WeightsAndOrder(t *testing.T) {
	issue := "panic: FooError in internal/core/kernel.go:123 when calling do_thing() and obj.process() with `specialVar`."

	kw := ExtractKeywords(issue)

	if got, want := kw.MentionedFiles, []string{"internal/core/kernel.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("MentionedFiles = %#v, want %#v", got, want)
	}
	if got, want := kw.Weights["internal/core/kernel.go"], 1.0; got != want {
		t.Fatalf("Weights[file] = %v, want %v", got, want)
	}

	if got, want := kw.Primary, []string{"FooError"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Primary = %#v, want %#v", got, want)
	}
	if got, want := kw.MentionedSymbols, []string{"FooError"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("MentionedSymbols = %#v, want %#v", got, want)
	}
	if got, want := kw.Weights["FooError"], 0.9; got != want {
		t.Fatalf("Weights[FooError] = %v, want %v", got, want)
	}

	if got, want := kw.Secondary, []string{"do_thing", "process"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Secondary = %#v, want %#v", got, want)
	}
	if got, want := kw.Weights["do_thing"], 0.7; got != want {
		t.Fatalf("Weights[do_thing] = %v, want %v", got, want)
	}
	if got, want := kw.Weights["process"], 0.7; got != want {
		t.Fatalf("Weights[process] = %v, want %v", got, want)
	}

	if got, want := kw.Tertiary, []string{"specialVar"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Tertiary = %#v, want %#v", got, want)
	}
	if got, want := kw.Weights["specialVar"], 0.5; got != want {
		t.Fatalf("Weights[specialVar] = %v, want %v", got, want)
	}

	if got, want := kw.AllKeywords(), []string{"FooError", "do_thing", "process", "specialVar"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("AllKeywords() = %#v, want %#v", got, want)
	}
}

func TestExtractKeywords_NormalizesBackslashPaths(t *testing.T) {
	issue := "panic: FooError in internal\\core\\kernel.go:123"
	kw := ExtractKeywords(issue)

	if got, want := kw.MentionedFiles, []string{"internal/core/kernel.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("MentionedFiles = %#v, want %#v", got, want)
	}
	if got, want := kw.Weights["internal/core/kernel.go"], 1.0; got != want {
		t.Fatalf("Weights[file] = %v, want %v", got, want)
	}
}

func TestKeywordHitCache_TTLAndEviction(t *testing.T) {
	hits := []KeywordHit{{FilePath: "a.go", Keyword: "k", Line: 1}}

	t.Run("ttl_expired", func(t *testing.T) {
		cache := NewKeywordHitCache(10, -1*time.Second)
		cache.Set("k", hits)
		if _, ok := cache.Get("k"); ok {
			t.Fatalf("Get() ok=true, want false for expired entry")
		}
	})

	t.Run("evicts_oldest", func(t *testing.T) {
		cache := NewKeywordHitCache(2, time.Hour)
		cache.Set("a", hits)
		cache.Set("b", hits)

		cache.mu.Lock()
		cache.entries["a"].timestamp = time.Unix(0, 0)
		cache.entries["b"].timestamp = time.Unix(100, 0)
		cache.mu.Unlock()

		cache.Set("c", hits)

		cache.mu.RLock()
		_, hasA := cache.entries["a"]
		_, hasB := cache.entries["b"]
		_, hasC := cache.entries["c"]
		cache.mu.RUnlock()

		if hasA || !hasB || !hasC {
			t.Fatalf("cache eviction unexpected (a=%v b=%v c=%v)", hasA, hasB, hasC)
		}
	})
}

func TestParseRipgrepOutput_CountsPerFile(t *testing.T) {
	r := &SparseRetriever{}
	output := "a.go:1:2:first\n" +
		"a.go:3:4:second\n" +
		"b.go:5:6:third\n"

	hits := r.parseRipgrepOutput(output, "kw")
	if len(hits) != 3 {
		t.Fatalf("parseRipgrepOutput len=%d, want 3", len(hits))
	}
	if hits[0].FilePath != "a.go" || hits[0].Count != 1 {
		t.Fatalf("hits[0]=%+v, want FilePath=a.go Count=1", hits[0])
	}
	if hits[1].FilePath != "a.go" || hits[1].Count != 2 {
		t.Fatalf("hits[1]=%+v, want FilePath=a.go Count=2", hits[1])
	}
	if hits[2].FilePath != "b.go" || hits[2].Count != 1 {
		t.Fatalf("hits[2]=%+v, want FilePath=b.go Count=1", hits[2])
	}
}

func TestRankFiles_ScoreAndTier(t *testing.T) {
	r := &SparseRetriever{}

	keywords := &IssueKeywords{
		Weights: map[string]float64{
			"K1": 1.0,
			"K2": 1.0,
			// K3 omitted to exercise default weight path.
		},
		MentionedFiles: []string{"src/mentioned.go"},
	}

	hits := []KeywordHit{
		{FilePath: "repo\\src\\mentioned.go", Keyword: "K3"},
		{FilePath: "repo/src/high.go", Keyword: "K1"},
		{FilePath: "repo/src/high.go", Keyword: "K2"},
		{FilePath: "repo/src/medium.go", Keyword: "K1"},
		{FilePath: "repo/src/low.go", Keyword: "K3"},
	}

	candidates := r.RankFiles(hits, keywords, 0)
	if len(candidates) != 4 {
		t.Fatalf("RankFiles len=%d, want 4", len(candidates))
	}

	if candidates[0].FilePath != "repo/src/high.go" {
		t.Fatalf("candidates[0].FilePath=%q, want repo/src/high.go", candidates[0].FilePath)
	}
	if candidates[0].Tier != 2 {
		t.Fatalf("high.go tier=%d, want 2", candidates[0].Tier)
	}

	var mentioned *CandidateFile
	for i := range candidates {
		if candidates[i].FilePath == "repo\\src\\mentioned.go" {
			mentioned = &candidates[i]
			break
		}
	}
	if mentioned == nil {
		t.Fatalf("expected mentioned file candidate not found: %#v", candidates)
	}
	if mentioned.Tier != 1 {
		t.Fatalf("mentioned.go tier=%d, want 1", mentioned.Tier)
	}
	if mentioned.RelevanceScore != 0.3 {
		t.Fatalf("mentioned.go score=%v, want 0.3 (default weight)", mentioned.RelevanceScore)
	}
}
