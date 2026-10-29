package retrieval

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

// The pass asserts each tier's provenance, keyed by its issue, and the
// kernel's policy -- not Go -- decides which shortfall becomes a caveat: a
// partial keyword search does, a by-design bounded semantic corpus does not.
func TestOutcomeFacts_PolicyDecidesTheCaveats(t *testing.T) {
	k := newDecisionKernel(t)
	facts := OutcomeFacts("/issue_prov", []TierOutcome{
		{Tier: TierKeyword, Status: StatusPartial, Reason: ReasonTimeout, Attempted: BackendNative, Used: BackendNative},
		{Tier: TierSemantic, Status: StatusPartial, Reason: ReasonBoundedCorpus, Attempted: BackendEmbeddings, Used: BackendEmbeddings, Eligible: 900, Covered: 256},
	})
	if err := k.LoadFacts(facts); err != nil {
		t.Fatalf("LoadFacts: %v", err)
	}
	if rows, _ := k.Query("retrieval_coverage"); len(rows) != 1 {
		t.Fatalf("retrieval_coverage rows = %d, want the semantic tier's", len(rows))
	}
	caveats, err := Caveats(k, "/issue_prov")
	if err != nil {
		t.Fatal(err)
	}
	if len(caveats) != 1 || caveats[0] != (BriefCaveat{Tier: "keyword", Status: "partial", Reason: "timeout"}) {
		t.Fatalf("caveats = %+v, want only the partial keyword search", caveats)
	}

	// The release takes the provenance with the rest of the issue.
	if err := RetractIssue(k, "/issue_prov"); err != nil {
		t.Fatal(err)
	}
	for _, pred := range []string{"retrieval_outcome", "retrieval_backend", "retrieval_coverage", "retrieval_brief_caveat"} {
		if rows, _ := k.Query(pred); len(rows) != 0 {
			t.Fatalf("%s survived the release: %v", pred, rows)
		}
	}
}

// An unavailable search with no files still reaches the model: an empty brief
// from a failed search must not read as "nothing matches".
func TestRenderBrief_CarriesCaveatsWithOrWithoutFiles(t *testing.T) {
	if RenderBrief(nil, nil) != "" {
		t.Fatal("an empty pass rendered text")
	}
	caveat := BriefCaveat{Tier: "keyword", Status: "unavailable", Reason: "membership_error"}
	only := RenderBrief(nil, []BriefCaveat{caveat})
	if !strings.Contains(only, "keyword search unavailable (membership error)") || strings.Contains(only, briefHeader) {
		t.Fatalf("caveat-only brief = %q", only)
	}
	both := RenderBrief([]BriefFile{{Path: "a.go", Tier: "tier1", Relevance: 100}}, []BriefCaveat{caveat})
	if !strings.HasPrefix(both, briefHeader) || !strings.Contains(both, "- a.go (tier1, 100)") || !strings.Contains(both, caveatHeader) {
		t.Fatalf("brief with files and caveats = %q", both)
	}
}

// End to end through the production caller: a task whose keyword search
// fails gets a brief that says so, from the kernel's caveat.
func TestTaskRetriever_PartialSearchReachesTheBrief(t *testing.T) {
	dir := writeRefundWorkspace(t)
	k := newDecisionKernel(t)
	assertIntent(t, k, "/task_intent_prov", "/mutation", "/fix", "payment_processor.go")
	cfg := DefaultSparseRetrieverConfig(dir)
	cfg.Backend = failingBackend{fail: "ProcessRefund"}
	tr := NewTaskRetriever(k, TaskRetrieverConfig{
		WorkDir:   dir,
		Retriever: NewSparseRetriever(cfg),
		Params:    config.DefaultRetrievalConfig().Params(),
	})
	brief, release := tr.Retrieve(context.Background(), "/task_intent_prov", refundIssue)
	defer release()
	if !strings.Contains(brief, "payment_processor.go (tier1,") {
		t.Fatalf("the named file is missing:\n%s", brief)
	}
	if !strings.Contains(brief, "keyword search partial (backend error)") {
		t.Fatalf("the failed search is not in the brief:\n%s", brief)
	}

	rows, _ := k.Query("retrieval_backend")
	found := false
	for _, row := range rows {
		if types.ExtractString(row.Args[1]) == "/semantic" && types.ExtractString(row.Args[3]) == "/definition_scan" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the semantic tier's actual backend was not asserted: %v", rows)
	}
}

// =============================================================================
// Semantic vectors: revision-safe reuse (R2)
// =============================================================================

// countingEmbedder counts the texts it embeds.
type countingEmbedder struct {
	texts atomic.Int64
	dims  int
}

func (e *countingEmbedder) vec(text string) []float32 {
	v := make([]float32, max(e.dims, 2))
	v[0] = float32(strings.Count(text, "zebra"))
	v[1] = 1
	return v
}

func (e *countingEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	return e.vec(text), nil
}

func (e *countingEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	e.texts.Add(int64(len(texts)))
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = []float32{float32(strings.Count(t, "zebra")), 1}
	}
	return out, nil
}

// Fixture 7: unchanged content reuses its vector across passes; an edited
// file is embedded again and ranked by what it says now, never by its old
// text.
func TestSemanticVectors_ReuseOnlyTheSameContent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.go", []byte("package a // zebra zebra\n"))
	writeFile(t, dir, "b.go", []byte("package b // plain\n"))
	engine := &countingEmbedder{}
	s := NewEmbeddingSemanticSearcher(engine, dir)

	first, err := s.SimilarFiles(context.Background(), "zebra", 5)
	if err != nil || len(first.Matches) == 0 || !strings.HasSuffix(first.Matches[0].FilePath, "a.go") {
		t.Fatalf("first pass: %+v %v", first, err)
	}
	if _, err := s.SimilarFiles(context.Background(), "zebra", 5); err != nil || engine.texts.Load() != 2 {
		t.Fatalf("unchanged files were embedded again: %d texts", engine.texts.Load())
	}

	writeFile(t, dir, "a.go", []byte("package a // plain now\n"))
	writeFile(t, dir, "b.go", []byte("package b // zebra zebra zebra\n"))
	after, err := s.SimilarFiles(context.Background(), "zebra", 5)
	if err != nil {
		t.Fatal(err)
	}
	if engine.texts.Load() != 4 {
		t.Fatalf("embedded %d texts, want the 2 edited files again", engine.texts.Load())
	}
	if !strings.HasSuffix(after.Matches[0].FilePath, "b.go") {
		t.Fatalf("ranked by stale vectors: %+v", after.Matches)
	}
}

// Vectors of files that left the corpus are dropped: the cache is bounded by
// the corpus, not by history.
func TestSemanticVectors_ForgetFilesOutsideTheCorpus(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.go", []byte("package a\n"))
	path := writeFile(t, dir, "b.go", []byte("package b\n"))
	s := NewEmbeddingSemanticSearcher(&countingEmbedder{}, dir)
	if _, err := s.SimilarFiles(context.Background(), "q", 5); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SimilarFiles(context.Background(), "q", 5); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, kept := s.vectors[path]; kept || len(s.vectors) != 1 {
		t.Fatalf("vectors = %d, a deleted file's vector kept = %v", len(s.vectors), kept)
	}
}

// Concurrent passes over one searcher embed each file once.
func TestSemanticVectors_ConcurrentMissesEmbedOnce(t *testing.T) {
	dir := t.TempDir()
	const files = 40
	for i := range files {
		writeFile(t, dir, fmt.Sprintf("f%02d.go", i), []byte(fmt.Sprintf("package f // file %d\n", i)))
	}
	engine := &countingEmbedder{}
	s := NewEmbeddingSemanticSearcher(engine, dir)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			if _, err := s.SimilarFiles(context.Background(), "q", 5); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if got := engine.texts.Load(); got != files {
		t.Fatalf("embedded %d texts for %d files", got, files)
	}
}

// A vector of another dimension than the query's is another model's: it is
// not ranked, and the result says the corpus was not covered.
func TestSemanticVectors_DimensionMismatchIsNotCoverage(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.go", []byte("package a\n"))
	s := NewEmbeddingSemanticSearcher(&countingEmbedder{dims: 3}, dir)
	result, err := s.SimilarFiles(context.Background(), "q", 5)
	if err != nil {
		t.Fatal(err)
	}
	if result.Covered != 0 || len(result.Matches) != 0 || result.Incomplete == "" {
		t.Fatalf("mismatched vectors were ranked: %+v", result)
	}
}

// Fixture 8: a corpus past the limit says how much it left out, and the
// outcome is partial (bounded_corpus) -- a fact policy reads, not a caveat
// it renders.
func TestSemanticTier_BoundedCorpusIsReported(t *testing.T) {
	dir := t.TempDir()
	for i := range 6 {
		writeFile(t, dir, fmt.Sprintf("f%d.go", i), []byte("package f // zebra\n"))
	}
	s := NewEmbeddingSemanticSearcher(&countingEmbedder{}, dir)
	s.corpusLimit = 4
	cfg := DefaultTieredContextConfig(dir)
	cfg.Semantic = s
	b := NewTieredContextBuilder(cfg)
	_, outcome := b.semanticExpansion(context.Background(), "zebra", &IssueKeywords{}, map[string]bool{})
	if outcome.Status != StatusPartial || outcome.Reason != ReasonBoundedCorpus || outcome.Eligible != 6 || outcome.Covered != 4 {
		t.Fatalf("outcome = %s, want partial (bounded_corpus), 4 of 6", outcome)
	}
}
