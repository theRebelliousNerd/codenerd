package retrieval

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"codenerd/internal/embedding"
)

// fakeTierEmbedder is a deterministic stand-in for embedding.EmbeddingEngine:
// topic words map to orthogonal unit vectors, so the documented Tier 4
// ordering (score descending, path ascending on ties) can be asserted without
// a live Ollama endpoint.
type fakeTierEmbedder struct{}

func (fakeTierEmbedder) topicVec(s string) []float32 {
	switch {
	case strings.Contains(s, "alpha"):
		return []float32{1, 0}
	case strings.Contains(s, "beta"):
		return []float32{0, 1}
	default:
		return []float32{0.5, 0.5}
	}
}

func (f fakeTierEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	// The probe query carries "quasar" so it ranks the alpha topic first
	// without contributing a keyword any workspace file contains — that keeps
	// tiers 1-3 empty and isolates Tier 4.
	if strings.Contains(text, "quasar") {
		return []float32{1, 0}, nil
	}
	return f.topicVec(text), nil
}

func (f fakeTierEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = f.topicVec(t)
	}
	return out, nil
}

func (fakeTierEmbedder) Dimensions() int { return 2 }
func (fakeTierEmbedder) Name() string    { return "fake-tier-embedder" }

// Compile-time proof the fake satisfies both the narrow Tier 4 dependency and
// the engine interface SeedRequest accepts (the one internal/embedding
// exposes, as internal/store and internal/prompt accept it).
var (
	_ Embedder                   = fakeTierEmbedder{}
	_ embedding.EmbeddingEngine  = fakeTierEmbedder{}
)

func semanticTierWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "alpha.go", []byte("package alpha\n\n// alpha handles widgets.\nfunc Alpha() {}\n"))
	writeFile(t, dir, "beta.go", []byte("package beta\n\n// beta handles gadgets.\nfunc Beta() {}\n"))
	writeFile(t, dir, "plain.go", []byte("package plain\n\n// plain holds misc helpers.\nfunc Plain() {}\n"))
	// Twins have identical heads and therefore identical scores: their relative
	// order must follow the documented path tiebreak.
	writeFile(t, dir, "twin_a.go", []byte("package twins\n\n// alpha twin file.\nfunc TwinA() {}\n"))
	writeFile(t, dir, "twin_b.go", []byte("package twins\n\n// alpha twin file.\nfunc TwinB() {}\n"))
	return dir
}

const semanticProbeIssue = "quasar failure in the nebula module"

func tierFiles(tc *TieredContext, tier int) []ContextFile {
	var out []ContextFile
	for _, f := range tc.Files {
		if f.Tier == tier {
			out = append(out, f)
		}
	}
	return out
}

func assertEmbeddingReasons(t *testing.T, files []ContextFile) {
	t.Helper()
	for _, f := range files {
		if !strings.HasPrefix(f.SelectionReason, "Semantically similar to the issue") {
			t.Errorf("%s reason = %q, want the embedding shape", f.FilePath, f.SelectionReason)
		}
		if f.RelevanceScore < 0 || f.RelevanceScore > 1 {
			t.Errorf("%s score = %v, want clamped to 0..1", f.FilePath, f.RelevanceScore)
		}
	}
}

func assertNoEmbeddingReasons(t *testing.T, files []ContextFile) {
	t.Helper()
	for _, f := range files {
		if strings.HasPrefix(f.SelectionReason, "Semantically similar") {
			t.Errorf("%s has an embedding reason without an engine", f.FilePath)
		}
	}
}

func assertOrderPair(t *testing.T, i int, prev, cur ContextFile) {
	t.Helper()
	if cur.RelevanceScore > prev.RelevanceScore+1e-9 {
		t.Errorf("Tier 4 not score-ordered at %d: %s (%v) before %s (%v)",
			i, prev.FilePath, prev.RelevanceScore, cur.FilePath, cur.RelevanceScore)
	}
	if cur.RelevanceScore == prev.RelevanceScore && cur.FilePath < prev.FilePath {
		t.Errorf("Tier 4 tie not path-ordered at %d: %s before %s",
			i, prev.FilePath, cur.FilePath)
	}
}

func assertDocumentedOrder(t *testing.T, files []ContextFile) {
	t.Helper()
	for i := 1; i < len(files); i++ {
		assertOrderPair(t, i, files[i-1], files[i])
	}
}

func assertTier4Bases(t *testing.T, files []ContextFile, want []string) {
	t.Helper()
	bases := make([]string, 0, len(files))
	for _, f := range files {
		bases = append(bases, filepath.Base(f.FilePath))
	}
	if !reflect.DeepEqual(bases, want) {
		t.Errorf("Tier 4 order = %v, want %v", bases, want)
	}
}

// TestTieredBuilder_WithEmbeddings_ShouldOrderSemanticTierAsDocumented proves
// the live Tier 4 path: with an engine, the builder returns vector hits
// ordered by score descending with path-ascending ties.
func TestTieredBuilder_WithEmbeddings_ShouldOrderSemanticTierAsDocumented(t *testing.T) {
	dir := semanticTierWorkspace(t)

	cfg := DefaultTieredContextConfig(dir)
	cfg.Semantic = NewEmbeddingSemanticSearcher(fakeTierEmbedder{}, dir)
	if cfg.Semantic == nil {
		t.Fatal("NewEmbeddingSemanticSearcher returned nil for a non-nil engine")
	}
	tc, err := NewTieredContextBuilder(cfg).BuildContext(context.Background(), semanticProbeIssue)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if tc.Tier1Count != 0 || tc.Tier2Count != 0 || tc.Tier3Count != 0 {
		t.Fatalf("probe issue should isolate Tier 4, got tiers %d/%d/%d",
			tc.Tier1Count, tc.Tier2Count, tc.Tier3Count)
	}

	tier4 := tierFiles(tc, 4)
	if len(tier4) != 5 {
		t.Fatalf("Tier 4 with embeddings = %d files, want all 5 workspace files", len(tier4))
	}
	assertEmbeddingReasons(t, tier4)
	assertDocumentedOrder(t, tier4)
	assertTier4Bases(t, tier4, []string{"alpha.go", "twin_a.go", "twin_b.go", "plain.go", "beta.go"})
}

// TestTieredBuilder_WithNilEngine_ShouldMatchHeuristicFallbackByteIdentical
// proves the nil path is today's behavior exactly: an explicit nil-engine
// searcher builds byte-identical output to a config with no searcher at all.
func TestTieredBuilder_WithNilEngine_ShouldMatchHeuristicFallbackByteIdentical(t *testing.T) {
	dir := semanticTierWorkspace(t)
	ctx := context.Background()

	if NewEmbeddingSemanticSearcher(nil, dir) != nil {
		t.Fatal("NewEmbeddingSemanticSearcher(nil) must return nil for the fallback")
	}

	plainCfg := DefaultTieredContextConfig(dir)
	nilCfg := DefaultTieredContextConfig(dir)
	nilCfg.Semantic = NewEmbeddingSemanticSearcher(nil, dir)

	a, errA := NewTieredContextBuilder(plainCfg).BuildContext(ctx, semanticProbeIssue)
	b, errB := NewTieredContextBuilder(nilCfg).BuildContext(ctx, semanticProbeIssue)
	if errA != nil || errB != nil {
		t.Fatalf("BuildContext errors: %v / %v", errA, errB)
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("nil-engine build differs from no-searcher build:\n%#v\nvs\n%#v", a, b)
	}
	assertNoEmbeddingReasons(t, b.Files)
}

// seedProbe runs one seed pass with a fresh kernel and the given engine (nil
// for the heuristic fallback), failing the test on any pass error.
func seedProbe(t *testing.T, dir, issueID string, engine embedding.EmbeddingEngine) *SeedReport {
	t.Helper()
	report, err := SeedIssueFacts(context.Background(), newSeedKernel(t), SeedRequest{
		IssueID:         issueID,
		IssueText:       semanticProbeIssue,
		WorkDir:         dir,
		Timeout:         30 * time.Second,
		EmbeddingEngine: engine,
	})
	if err != nil {
		t.Fatalf("SeedIssueFacts: %v", err)
	}
	return report
}

// TestSeedIssueFacts_WithEngine_ShouldUseEmbeddingsTier proves the seed wire:
// with an engine the report records the embeddings tier and Tier 4 fills.
func TestSeedIssueFacts_WithEngine_ShouldUseEmbeddingsTier(t *testing.T) {
	report := seedProbe(t, semanticTierWorkspace(t), "/issue_sem_tier", fakeTierEmbedder{})
	if report.SemanticTier != "embeddings" {
		t.Errorf("SemanticTier = %q, want %q", report.SemanticTier, "embeddings")
	}
	if report.TierCounts[3] == 0 {
		t.Error("Tier 4 is empty despite a working engine")
	}
	if s := report.Summary(); strings.Contains(s, "heuristic fallback") {
		t.Errorf("embeddings summary must not claim a fallback: %q", s)
	}
}

// TestSeedIssueFacts_WithoutEngine_ShouldSurfaceFallbackAndKeepOtherTiers
// proves the degradation is visible in the seed summary itself, and that
// swapping the backend leaves tiers 1-3 (and their ranking weights) untouched.
func TestSeedIssueFacts_WithoutEngine_ShouldSurfaceFallbackAndKeepOtherTiers(t *testing.T) {
	dir := semanticTierWorkspace(t)
	embedReport := seedProbe(t, dir, "/issue_sem_tier", fakeTierEmbedder{})
	heurReport := seedProbe(t, dir, "/issue_sem_heur", nil)

	if heurReport.SemanticTier != "heuristic fallback" {
		t.Errorf("SemanticTier = %q, want %q", heurReport.SemanticTier, "heuristic fallback")
	}
	if s := heurReport.Summary(); !strings.Contains(s, "heuristic fallback") {
		t.Errorf("nil-engine summary must surface the degradation: %q", s)
	}
	for tier := 1; tier <= 3; tier++ {
		if embedReport.TierCounts[tier-1] != heurReport.TierCounts[tier-1] {
			t.Errorf("tier %d count changed with the backend swap: %d vs %d",
				tier, embedReport.TierCounts[tier-1], heurReport.TierCounts[tier-1])
		}
	}
}
