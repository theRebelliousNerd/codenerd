package retrieval

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// saturationWorkspace lays down the audit's witness: forty early files that
// fill one keyword's 20,000-hit cap, then more empty files than the walk's
// channel holds, so a walk that does not stop at saturation blocks sending.
func saturationWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dense := strings.Repeat("quasar\n", maxHitsPerFile)
	for i := range 40 {
		writeFile(t, dir, fmt.Sprintf("a%03d.txt", i), []byte(dense))
	}
	for i := range 2500 {
		writeFile(t, dir, filepath.Join("z", fmt.Sprintf("b%04d.txt", i)), []byte("nothing here\n"))
	}
	return dir
}

// S1 of Docs/audits/retrieval-saturation-outcomes-and-producer-lifecycle.md:
// once every keyword reached its cap the readers returned, the walk blocked on
// the full channel until the deadline, and the whole capped result came back
// as a timeout -- failed, uncached, and the seed budget spent.
func TestSearchKeywordsNative_SaturationEndsTheWalkNotTheDeadline(t *testing.T) {
	dir := saturationWorkspace(t)
	cfg := DefaultSparseRetrieverConfig(dir)
	cfg.Parallelism = 1
	cfg.SearchTimeout = 20 * time.Second
	r := NewSparseRetriever(cfg)

	start := time.Now()
	hits, outcome := r.SearchKeywords(context.Background(), &IssueKeywords{Primary: []string{"quasar"}})
	elapsed := time.Since(start)
	if elapsed > 10*time.Second {
		t.Fatalf("saturated search took %v: the walk waited for the deadline", elapsed)
	}
	if len(hits) != maxHitsPerKeyword {
		t.Fatalf("hits = %d, want the cap %d", len(hits), maxHitsPerKeyword)
	}
	if outcome.Status != StatusPartial || outcome.Reason != ReasonOverflow || outcome.Detail != "" {
		t.Fatalf("outcome = %s (detail %q), want partial (overflow) with no error", outcome, outcome.Detail)
	}
	// Cached as what it is: the repeat reads the cap from the cache.
	_, again := r.SearchKeywords(context.Background(), &IssueKeywords{Primary: []string{"quasar"}})
	if again.CacheHits != 1 || again.Reason != ReasonOverflow {
		t.Fatalf("repeat outcome = %s, want a cached overflow", again)
	}
}

// One saturated keyword must not stop the walk for one still open: the open
// keyword's only hit is in the last file.
func TestSearchKeywordsNative_AnOpenKeywordKeepsTheWalkGoing(t *testing.T) {
	dir := saturationWorkspace(t)
	writeFile(t, dir, filepath.Join("z", "zzzz_last.txt"), []byte("pulsar\n"))
	cfg := DefaultSparseRetrieverConfig(dir)
	cfg.Parallelism = 1
	r := NewSparseRetriever(cfg)
	found, err := r.searchKeywordsNative(context.Background(), []string{"quasar", "pulsar"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(found["quasar"]) != maxHitsPerKeyword || len(found["pulsar"]) != 1 {
		t.Fatalf("quasar=%d pulsar=%d, want the cap and the last file's hit", len(found["quasar"]), len(found["pulsar"]))
	}
}

// S2: a backend that cut its result at the cap said nothing, and the caller
// reported (and cached) it complete.
type cappingBackend struct{}

func (cappingBackend) Name() string { return "capping" }
func (cappingBackend) Search(_ context.Context, root, keyword string, _ []string) (BackendResult, error) {
	return BackendResult{Hits: []KeywordHit{{FilePath: filepath.Join(root, "a.go"), Keyword: keyword, Line: 1, Column: 1, Count: 1}}, CapReached: true}, nil
}

func TestSearchKeywords_ABackendAtItsCapIsPartial(t *testing.T) {
	cfg := DefaultSparseRetrieverConfig(t.TempDir())
	cfg.Backend = cappingBackend{}
	r := NewSparseRetriever(cfg)
	for round := range 2 {
		_, outcome := r.SearchKeywords(context.Background(), &IssueKeywords{Primary: []string{"kw"}})
		if outcome.Status != StatusPartial || outcome.Reason != ReasonOverflow {
			t.Fatalf("round %d outcome = %s, want partial (overflow)", round, outcome)
		}
	}
}
