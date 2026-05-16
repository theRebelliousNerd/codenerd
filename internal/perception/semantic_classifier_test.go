package perception

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/core"
	"github.com/google/mangle/analysis"
)

// mockKernel implements core.Kernel for testing.
type mockKernel struct {
	mu                  sync.Mutex
	assertedFacts       []core.Fact
	retractedPredicates []string
}

func (m *mockKernel) GetProgramInfo() *analysis.ProgramInfo {
	return nil
}

func (m *mockKernel) LoadFacts(facts []core.Fact) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.assertedFacts = append(m.assertedFacts, facts...)
	return nil
}

func (m *mockKernel) Query(predicate string) ([]core.Fact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var results []core.Fact
	for _, f := range m.assertedFacts {
		if f.Predicate == predicate {
			results = append(results, f)
		}
	}
	return results, nil
}

func (m *mockKernel) QueryAll() (map[string][]core.Fact, error) {
	return nil, nil
}

func (m *mockKernel) Assert(fact core.Fact) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.assertedFacts = append(m.assertedFacts, fact)
	return nil
}

func (m *mockKernel) AssertBatch(facts []core.Fact) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.assertedFacts = append(m.assertedFacts, facts...)
	return nil
}

func (m *mockKernel) Retract(predicate string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retractedPredicates = append(m.retractedPredicates, predicate)
	return nil
}

func (m *mockKernel) RetractFact(fact core.Fact) error {
	return nil
}

func (m *mockKernel) UpdateSystemFacts() error {
	return nil
}

func (m *mockKernel) AppendPolicy(policy string) {
}

func (m *mockKernel) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.assertedFacts = nil
	m.retractedPredicates = nil
}

func (m *mockKernel) RemoveFactsByPredicateSet(map[string]struct{}) error { return nil }
func (m *mockKernel) RetractExactFactsBatch([]core.Fact) error            { return nil }

// REMEDIATED: TEST_GAP: (Null/Undefined/Empty) Add tests for empty or highly whitespace-padded query strings to ensure they return fast zero-matches or handle gracefully without engine panics.
// REMEDIATED: TEST_GAP: (Type Coercion) Add tests injecting strings instead of Atoms or vice versa during fact assertion to ensure SemanticClassifier gracefully handles mismatched types in Mangle.
// REMEDIATED: TEST_GAP: (User Request Extremes) Add boundary analysis tests simulating frontier-length prompts (e.g., 200k tokens) and extreme vector sizes (e.g., 8192 dims instead of 3072) to test memory safety and latency.
// REMEDIATED: TEST_GAP: (State Conflicts) Add parallel/concurrent classification tests checking for race conditions around the in-memory maps or lock contention during `Classify()`.

func TestDefaultSemanticConfig(t *testing.T) {
	cfg := DefaultSemanticConfig()

	if cfg.TopK != 5 {
		t.Errorf("expected TopK=5, got %d", cfg.TopK)
	}
	if cfg.MinSimilarity != 0.5 {
		t.Errorf("expected MinSimilarity=0.5, got %f", cfg.MinSimilarity)
	}
	if cfg.LearnedBoost != 0.1 {
		t.Errorf("expected LearnedBoost=0.1, got %f", cfg.LearnedBoost)
	}
	if !cfg.EnableParallel {
		t.Error("expected EnableParallel=true")
	}
}

func TestNewSemanticClassifier(t *testing.T) {
	kernel := &mockKernel{}
	sc := NewSemanticClassifier(kernel, nil, nil, nil)

	if sc == nil {
		t.Fatal("expected non-nil classifier")
	}
	if sc.kernel != kernel {
		t.Error("kernel not set correctly")
	}
	if sc.config.TopK != 5 {
		t.Errorf("expected default TopK=5, got %d", sc.config.TopK)
	}
}

func TestSemanticClassifier_ClassifyWithoutEngine(t *testing.T) {
	// Test graceful degradation when no embedding engine is available
	kernel := &mockKernel{}
	sc := NewSemanticClassifier(kernel, nil, nil, nil)

	ctx := context.Background()
	matches, err := sc.ClassifyWithoutInjection(ctx, "review my code")

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if matches != nil {
		t.Errorf("expected nil matches without engine, got %v", matches)
	}
}

func TestSemanticClassifier_SetConfig(t *testing.T) {
	kernel := &mockKernel{}
	sc := NewSemanticClassifier(kernel, nil, nil, nil)

	newCfg := SemanticConfig{
		TopK:           10,
		MinSimilarity:  0.7,
		LearnedBoost:   0.2,
		EnableParallel: false,
	}
	sc.SetConfig(newCfg)

	if sc.config.TopK != 10 {
		t.Errorf("expected TopK=10, got %d", sc.config.TopK)
	}
	if sc.config.MinSimilarity != 0.7 {
		t.Errorf("expected MinSimilarity=0.7, got %f", sc.config.MinSimilarity)
	}
}

func TestMergeResults(t *testing.T) {
	kernel := &mockKernel{}
	sc := NewSemanticClassifier(kernel, nil, nil, nil)

	embedded := []SemanticMatch{
		{TextContent: "review code", Verb: "/review", Similarity: 0.8, Source: "embedded"},
		{TextContent: "fix bug", Verb: "/fix", Similarity: 0.7, Source: "embedded"},
	}
	learned := []SemanticMatch{
		{TextContent: "check code quality", Verb: "/review", Similarity: 0.75, Source: "learned"},
	}

	cfg := DefaultSemanticConfig()
	merged := sc.mergeResults(embedded, learned, cfg)

	// Learned patterns should get boost
	// Original: 0.75 + 0.1 = 0.85
	if len(merged) != 3 {
		t.Errorf("expected 3 merged results, got %d", len(merged))
	}

	// First result should be the boosted learned pattern (0.85)
	if merged[0].Similarity < 0.84 || merged[0].Similarity > 0.86 {
		t.Errorf("expected first result to have similarity ~0.85, got %f", merged[0].Similarity)
	}

	// Check ranks are assigned correctly
	for i, m := range merged {
		if m.Rank != i+1 {
			t.Errorf("expected rank %d, got %d", i+1, m.Rank)
		}
	}
}

func TestFilterByThreshold(t *testing.T) {
	kernel := &mockKernel{}
	sc := NewSemanticClassifier(kernel, nil, nil, nil)

	matches := []SemanticMatch{
		{TextContent: "high", Similarity: 0.9, Rank: 1},
		{TextContent: "medium", Similarity: 0.6, Rank: 2},
		{TextContent: "low", Similarity: 0.3, Rank: 3},
	}

	filtered := sc.filterByThreshold(matches, 0.5)

	if len(filtered) != 2 {
		t.Errorf("expected 2 filtered results, got %d", len(filtered))
	}

	// Check ranks are reassigned
	if filtered[0].Rank != 1 || filtered[1].Rank != 2 {
		t.Error("ranks not reassigned correctly after filtering")
	}
}

func TestInjectFacts(t *testing.T) {
	kernel := &mockKernel{}
	sc := NewSemanticClassifier(kernel, nil, nil, nil)

	matches := []SemanticMatch{
		{TextContent: "review code", Verb: "/review", Target: "codebase", Similarity: 0.8, Rank: 1},
		{TextContent: "fix bug", Verb: "/fix", Target: "", Similarity: 0.7, Rank: 2},
	}

	sc.injectFacts("check my code", matches)

	if len(kernel.assertedFacts) != 2 {
		t.Errorf("expected 2 asserted facts, got %d", len(kernel.assertedFacts))
	}

	// Verify first fact structure
	fact := kernel.assertedFacts[0]
	if fact.Predicate != "semantic_match" {
		t.Errorf("expected predicate 'semantic_match', got %s", fact.Predicate)
	}
	if len(fact.Args) != 6 {
		t.Errorf("expected 6 args, got %d", len(fact.Args))
	}

	// Check similarity is scaled to 0-100
	similarity, ok := fact.Args[5].(int64)
	if !ok {
		t.Errorf("expected int64 similarity, got %T", fact.Args[5])
	}
	if similarity != 80 { // 0.8 * 100 = 80
		t.Errorf("expected similarity=80, got %d", similarity)
	}
}

func TestEmbeddedCorpusStore_Search_Empty(t *testing.T) {
	store, err := NewEmbeddedCorpusStore(3072)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	results, err := store.Search(make([]float32, 3072), 5)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results from empty store, got %v", results)
	}
}

func TestLearnedCorpusStore_Add(t *testing.T) {
	store, err := NewLearnedCorpusStore(nil, 3072, nil)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	entry := CorpusEntry{
		TextContent: "test pattern",
		Verb:        "/test",
		Target:      "codebase",
		Confidence:  0.9,
	}
	embed := make([]float32, 3072)
	for i := range embed {
		embed[i] = float32(i) / 3072.0
	}

	if err := store.Add(entry, embed); err != nil {
		t.Errorf("failed to add entry: %v", err)
	}

	if len(store.entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(store.entries))
	}
	if _, ok := store.embeddings["test pattern"]; !ok {
		t.Error("embedding not stored")
	}
}

func TestLearnedCorpusStore_Add_DimensionMismatch(t *testing.T) {
	store, err := NewLearnedCorpusStore(nil, 3072, nil)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	entry := CorpusEntry{TextContent: "test", Verb: "/test"}
	wrongDimEmbed := make([]float32, 768) // Wrong dimensions (old size)

	err = store.Add(entry, wrongDimEmbed)
	if err == nil {
		t.Error("expected error for dimension mismatch")
	}
}

// =============================================================================
// MISSING TEST COVERAGE (BOUNDARY ANALYSIS & NEGATIVE TESTING)
// =============================================================================

// TEST_GAP: Null/Undefined/Empty: Classify with empty string
func TestSemanticClassifier_EmptyInput(t *testing.T) {
	sc := NewSemanticClassifier(&mockKernel{}, nil, nil, nil)
	matches, err := sc.Classify(context.Background(), "   ")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if matches != nil {
		t.Errorf("Expected nil matches for empty string, got %v", matches)
	}
}

// TEST_GAP: Type Coercion: mergeResults negative cfg.TopK
func TestSemanticClassifier_NegativeTopK(t *testing.T) {
	sc := NewSemanticClassifier(&mockKernel{}, nil, nil, nil)
	cfg := DefaultSemanticConfig()
	cfg.TopK = -5
	sc.SetConfig(cfg)
	// should not panic
	merged := sc.mergeResults([]SemanticMatch{{TextContent: "a", Similarity: 0.9}}, []SemanticMatch{{TextContent: "b", Similarity: 0.8}}, cfg)
	if len(merged) != 2 {
		t.Errorf("Expected 2 results, got %d", len(merged))
	}
}

type mockEmbedEngine struct {
	mu        sync.Mutex
	lastInput string
}

func (m *mockEmbedEngine) Name() string    { return "mock" }
func (m *mockEmbedEngine) Dimensions() int { return 3072 }
func (m *mockEmbedEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	m.mu.Lock()
	m.lastInput = text
	m.mu.Unlock()
	return make([]float32, 3072), nil
}
func (m *mockEmbedEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, nil
}

// TEST_GAP: User Request Extremes: Massive Input Exhaustion
func TestSemanticClassifier_MassiveInput(t *testing.T) {
	engine := &mockEmbedEngine{}
	sc := NewSemanticClassifier(&mockKernel{}, nil, nil, engine)

	massiveInput := strings.Repeat("A", 100000)
	_, _ = sc.ClassifyWithoutInjection(context.Background(), massiveInput)

	if len(engine.lastInput) > 40000 {
		t.Errorf("Input was not truncated, length is %d", len(engine.lastInput))
	}
}

// TEST_GAP: State Conflicts: LoadFromKernel Ghost Duplication Memory Leak
func TestEmbeddedCorpusStore_GhostDuplication(t *testing.T) {
	store, _ := NewEmbeddedCorpusStore(3072)
	kernel := &mockKernel{
		assertedFacts: []core.Fact{
			{Args: []interface{}{"phrase", "/verb"}},
		},
	}
	engine := &mockEmbedEngine{}

	_ = store.LoadFromKernel(context.Background(), kernel, engine)
	count1 := len(store.entries)
	_ = store.LoadFromKernel(context.Background(), kernel, engine)
	count2 := len(store.entries)

	if count1 != count2 {
		t.Errorf("Expected entries to not duplicate, got %d and %d", count1, count2)
	}
}

// TEST_GAP: Type Coercion: Target Loss of MangleAtom Identity
func TestSemanticClassifier_TargetMangleAtom(t *testing.T) {
	kernel := &mockKernel{}
	sc := NewSemanticClassifier(kernel, nil, nil, nil)

	matches := []SemanticMatch{
		{TextContent: "do", Verb: "/fix", Target: "/codebase", Similarity: 0.8},
	}
	sc.injectFacts("test", matches)

	if len(kernel.assertedFacts) == 0 {
		t.Fatal("No facts asserted")
	}
	fact := kernel.assertedFacts[0]
	targetArg := fact.Args[3]
	if _, ok := targetArg.(core.MangleAtom); !ok {
		t.Errorf("Expected core.MangleAtom for target starting with '/', got %T", targetArg)
	}
}

// TEST_GAP: State Conflicts: Semantic Match Accumulation
func TestSemanticClassifier_StatePollution(t *testing.T) {
	kernel := &mockKernel{}
	sc := NewSemanticClassifier(kernel, nil, nil, nil)

	matches := []SemanticMatch{
		{TextContent: "do", Verb: "/fix", Similarity: 0.8},
	}
	sc.injectFacts("test", matches)

	found := false
	for _, p := range kernel.retractedPredicates {
		if p == "semantic_match" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'semantic_match' to be retracted, got %v", kernel.retractedPredicates)
	}
}

// TEST_GAP: State Conflicts: Concurrency lock testing
func TestSemanticClassifier_Concurrency(t *testing.T) {
	store, _ := NewEmbeddedCorpusStore(3072)
	sc := NewSemanticClassifier(&mockKernel{}, store, nil, &mockEmbedEngine{})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sc.Classify(context.Background(), fmt.Sprintf("test %d", id))
		}(i)
	}
	wg.Wait()
}
