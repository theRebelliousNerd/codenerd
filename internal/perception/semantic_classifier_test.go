package perception

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codenerd/internal/core"
	_ "github.com/mattn/go-sqlite3"
)

// TODO: Add tests for Null/Undefined/Empty scenarios:
// - Empty embedding vectors in search.
// - Empty intents/facts returned from the kernel.
// - Null embedding engine in classifier operations.

// TODO: Add tests for Type Coercion:
// - Verifying behavior when Mangle atoms are expected but raw strings are returned/provided.

// TODO: Add tests for User Request Extremes:
// - Provide an extremely long input string that exceeds typical context limits to ensure graceful handling or truncation without panic.
// - Query with very high topK (e.g. 1000) when the database has fewer entries.

// TODO: Add tests for State Conflicts/Race Conditions:
// - Test concurrent updates and searches in LearnedCorpusStore to verify thread safety.
// - Test searching the store while LoadFromKernel is in progress.


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
			{Args: []any{"phrase", "/verb"}},
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

	found := slices.Contains(kernel.retractedPredicates, "semantic_match")
	if !found {
		t.Errorf("Expected 'semantic_match' to be retracted, got %v", kernel.retractedPredicates)
	}
}

// TEST_GAP: State Conflicts: Concurrency lock testing
func TestSemanticClassifier_Concurrency(t *testing.T) {
	store, _ := NewEmbeddedCorpusStore(3072)
	sc := NewSemanticClassifier(&mockKernel{}, store, nil, &mockEmbedEngine{})

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sc.Classify(context.Background(), fmt.Sprintf("test %d", id))
		}(i)
	}
	wg.Wait()
}

// =============================================================================
// EMBEDDING CACHE TESTS
// =============================================================================

func TestFloat32ByteRoundTrip(t *testing.T) {
	original := []float32{1.0, -0.5, 0.0, 3.14, 1e-7}
	blob := float32ToBytes(original)
	recovered := bytesToFloat32(blob)

	if len(recovered) != len(original) {
		t.Fatalf("length mismatch: expected %d, got %d", len(original), len(recovered))
	}
	for i, v := range original {
		if recovered[i] != v {
			t.Errorf("index %d: expected %f, got %f", i, v, recovered[i])
		}
	}
}

func TestBytesToFloat32_InvalidLength(t *testing.T) {
	result := bytesToFloat32([]byte{0x01, 0x02, 0x03})
	if result != nil {
		t.Errorf("expected nil for non-aligned byte slice, got %v", result)
	}
}

func TestHashText_Deterministic(t *testing.T) {
	h1 := hashText("review my code for bugs")
	h2 := hashText("review my code for bugs")
	h3 := hashText("fix the authentication bug")

	if h1 != h2 {
		t.Errorf("same input produced different hashes: %s vs %s", h1, h2)
	}
	if h1 == h3 {
		t.Error("different inputs produced the same hash")
	}
	if len(h1) != 64 { // SHA-256 hex = 64 chars
		t.Errorf("expected 64 char hex hash, got %d", len(h1))
	}
}

func TestNewEmbeddedCorpusStoreWithCache_CreatesDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_cache.db")

	store, err := NewEmbeddedCorpusStoreWithCache(768, dbPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer store.Close()

	if store.cacheDB == nil {
		t.Fatal("expected cacheDB to be non-nil")
	}
	if store.cachePath != dbPath {
		t.Errorf("expected cachePath=%s, got %s", dbPath, store.cachePath)
	}
	if store.dimensions != 768 {
		t.Errorf("expected dimensions=768, got %d", store.dimensions)
	}

	// Verify the file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("cache DB file was not created")
	}
}

func TestNewEmbeddedCorpusStoreWithCache_EmptyPath(t *testing.T) {
	store, err := NewEmbeddedCorpusStoreWithCache(768, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer store.Close()

	if store.cacheDB != nil {
		t.Error("expected cacheDB to be nil for empty path")
	}
}

// mockBatchEmbedEngine tracks calls and returns deterministic embeddings.
type mockBatchEmbedEngine struct {
	mu        sync.Mutex
	callCount int
	lastBatch []string
	dims      int
}

func (m *mockBatchEmbedEngine) Name() string    { return "mock-batch" }
func (m *mockBatchEmbedEngine) Dimensions() int { return m.dims }
func (m *mockBatchEmbedEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	vec := make([]float32, m.dims)
	for i := range vec {
		vec[i] = float32(len(text)%10) * 0.1
	}
	return vec, nil
}
func (m *mockBatchEmbedEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	m.mu.Lock()
	m.callCount++
	m.lastBatch = texts
	m.mu.Unlock()

	result := make([][]float32, len(texts))
	for i, text := range texts {
		vec := make([]float32, m.dims)
		for j := range vec {
			vec[j] = float32(len(text)%10+i) * 0.01
		}
		result[i] = vec
	}
	return result, nil
}

func TestLoadFromKernel_WithCache_FirstBoot(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_cache.db")

	store, err := NewEmbeddedCorpusStoreWithCache(4, dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	engine := &mockBatchEmbedEngine{dims: 4}

	kernel := &mockKernel{
		assertedFacts: []core.Fact{
			{Predicate: "intent_definition", Args: []any{"review code", "/review", ""}},
			{Predicate: "intent_definition", Args: []any{"fix bug", "/fix", ""}},
			{Predicate: "intent_definition", Args: []any{"run tests", "/test", ""}},
		},
	}

	err = store.LoadFromKernel(context.Background(), kernel, engine)
	if err != nil {
		t.Fatalf("LoadFromKernel failed: %v", err)
	}

	// First boot: all misses, so EmbedBatch should have been called
	if engine.callCount != 1 {
		t.Errorf("expected 1 EmbedBatch call on first boot, got %d", engine.callCount)
	}
	if len(store.entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(store.entries))
	}
	if len(store.embeddings) != 3 {
		t.Errorf("expected 3 embeddings, got %d", len(store.embeddings))
	}
}

func TestLoadFromKernel_WithCache_SecondBoot(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_cache.db")

	engine := &mockBatchEmbedEngine{dims: 4}

	kernel := &mockKernel{
		assertedFacts: []core.Fact{
			{Predicate: "intent_definition", Args: []any{"review code", "/review", ""}},
			{Predicate: "intent_definition", Args: []any{"fix bug", "/fix", ""}},
		},
	}

	// First boot: populate cache
	store1, err := NewEmbeddedCorpusStoreWithCache(4, dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	err = store1.LoadFromKernel(context.Background(), kernel, engine)
	if err != nil {
		t.Fatalf("first LoadFromKernel failed: %v", err)
	}
	store1.Close()

	if engine.callCount != 1 {
		t.Fatalf("expected 1 call on first boot, got %d", engine.callCount)
	}

	// Second boot: same texts, should be all cache hits
	store2, err := NewEmbeddedCorpusStoreWithCache(4, dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store2.Close()

	err = store2.LoadFromKernel(context.Background(), kernel, engine)
	if err != nil {
		t.Fatalf("second LoadFromKernel failed: %v", err)
	}

	// Should NOT have called EmbedBatch again
	if engine.callCount != 1 {
		t.Errorf("expected no additional EmbedBatch calls on second boot, total calls: %d", engine.callCount)
	}
	if len(store2.entries) != 2 {
		t.Errorf("expected 2 entries on second boot, got %d", len(store2.entries))
	}
}

func TestLoadFromKernel_WithCache_PartialHit(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_cache.db")

	engine := &mockBatchEmbedEngine{dims: 4}

	// First boot: cache "review code" only
	kernel1 := &mockKernel{
		assertedFacts: []core.Fact{
			{Predicate: "intent_definition", Args: []any{"review code", "/review", ""}},
		},
	}
	store1, _ := NewEmbeddedCorpusStoreWithCache(4, dbPath)
	store1.LoadFromKernel(context.Background(), kernel1, engine)
	store1.Close()
	if engine.callCount != 1 {
		t.Fatalf("expected 1 call, got %d", engine.callCount)
	}

	// Second boot: add new text "fix bug" (partial miss)
	kernel2 := &mockKernel{
		assertedFacts: []core.Fact{
			{Predicate: "intent_definition", Args: []any{"review code", "/review", ""}},
			{Predicate: "intent_definition", Args: []any{"fix bug", "/fix", ""}},
		},
	}
	store2, _ := NewEmbeddedCorpusStoreWithCache(4, dbPath)
	defer store2.Close()

	store2.LoadFromKernel(context.Background(), kernel2, engine)

	// Should have called EmbedBatch for the 1 miss only
	if engine.callCount != 2 {
		t.Errorf("expected 2 total EmbedBatch calls (1 for miss), got %d", engine.callCount)
	}
	engine.mu.Lock()
	if len(engine.lastBatch) != 1 || engine.lastBatch[0] != "fix bug" {
		t.Errorf("expected last batch to contain only 'fix bug', got %v", engine.lastBatch)
	}
	engine.mu.Unlock()

	if len(store2.entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(store2.entries))
	}
}

func TestEmbeddedCorpusStore_Close_NilSafe(t *testing.T) {
	var store *EmbeddedCorpusStore
	if err := store.Close(); err != nil {
		t.Errorf("Close on nil store should return nil, got %v", err)
	}
}

func TestEmbeddedCorpusStore_Close_NoCacheDB(t *testing.T) {
	store, _ := NewEmbeddedCorpusStore(768)
	if err := store.Close(); err != nil {
		t.Errorf("Close without cacheDB should return nil, got %v", err)
	}
}

func TestLoadFromKernel_NilInputs(t *testing.T) {
	store, _ := NewEmbeddedCorpusStoreWithCache(4, "")

	// All nil inputs should return nil error
	if err := store.LoadFromKernel(context.Background(), nil, nil); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

// chunkCountingEngine wraps mockBatchEmbedEngine and can cancel after N batches.
type chunkCountingEngine struct {
	mockBatchEmbedEngine
	cancelAfter int
	cancelFn    context.CancelFunc
	batchSizes  []int
}

func (m *chunkCountingEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	m.mu.Lock()
	m.callCount++
	m.batchSizes = append(m.batchSizes, len(texts))
	m.lastBatch = texts
	n := m.callCount
	cancelAfter := m.cancelAfter
	cancelFn := m.cancelFn
	dims := m.dims
	m.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Build vectors inline — do not call mockBatchEmbedEngine.EmbedBatch or callCount doubles.
	result := make([][]float32, len(texts))
	for i, text := range texts {
		vec := make([]float32, dims)
		for j := range vec {
			vec[j] = float32(len(text)%10+i) * 0.01
		}
		result[i] = vec
	}
	// Cancel AFTER a successful chunk so LoadFromKernel can cache it, then stop
	// before the next chunk (mirrors a deadline firing mid-hydrate).
	if cancelAfter > 0 && n >= cancelAfter && cancelFn != nil {
		cancelFn()
	}
	return result, nil
}

func TestLoadFromKernel_ChunksMisses(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "chunk_cache.db")

	store, err := NewEmbeddedCorpusStoreWithCache(4, dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	// 40 intents → 2 chunks (32 + 8) with intentEmbedChunkSize=32
	facts := make([]core.Fact, 0, 40)
	for i := 0; i < 40; i++ {
		facts = append(facts, core.Fact{
			Predicate: "intent_definition",
			Args:      []any{fmt.Sprintf("phrase %d", i), "/verb", ""},
		})
	}
	kernel := &mockKernel{assertedFacts: facts}
	engine := &chunkCountingEngine{mockBatchEmbedEngine: mockBatchEmbedEngine{dims: 4}}

	if err := store.LoadFromKernel(context.Background(), kernel, engine); err != nil {
		t.Fatalf("LoadFromKernel: %v", err)
	}
	if engine.callCount != 2 {
		t.Fatalf("expected 2 chunked EmbedBatch calls, got %d sizes=%v", engine.callCount, engine.batchSizes)
	}
	if engine.batchSizes[0] != 32 || engine.batchSizes[1] != 8 {
		t.Fatalf("unexpected batch sizes: %v", engine.batchSizes)
	}
	if len(store.entries) != 40 {
		t.Fatalf("expected 40 entries, got %d", len(store.entries))
	}
}

func TestLoadFromKernel_PartialCacheOnCancel(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "partial_cache.db")

	// First boot: cancel after first chunk so only 32 of 40 are cached.
	store1, err := NewEmbeddedCorpusStoreWithCache(4, dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	facts := make([]core.Fact, 0, 40)
	for i := 0; i < 40; i++ {
		facts = append(facts, core.Fact{
			Predicate: "intent_definition",
			Args:      []any{fmt.Sprintf("cancel phrase %d", i), "/verb", ""},
		})
	}
	kernel := &mockKernel{assertedFacts: facts}
	ctx, cancel := context.WithCancel(context.Background())
	engine := &chunkCountingEngine{
		mockBatchEmbedEngine: mockBatchEmbedEngine{dims: 4},
		cancelAfter:          1,
		cancelFn:             cancel,
	}
	// LoadFromKernel must not hard-fail on cancel — partial progress is OK.
	if err := store1.LoadFromKernel(ctx, kernel, engine); err != nil {
		t.Fatalf("expected soft cancel, got error: %v", err)
	}
	store1.Close()

	// Second boot: remaining misses only (8), not full 40.
	store2, err := NewEmbeddedCorpusStoreWithCache(4, dbPath)
	if err != nil {
		t.Fatalf("create store2: %v", err)
	}
	defer store2.Close()
	engine2 := &chunkCountingEngine{mockBatchEmbedEngine: mockBatchEmbedEngine{dims: 4}}
	if err := store2.LoadFromKernel(context.Background(), kernel, engine2); err != nil {
		t.Fatalf("second LoadFromKernel: %v", err)
	}
	if engine2.callCount != 1 {
		t.Fatalf("expected 1 EmbedBatch for remaining misses, got %d", engine2.callCount)
	}
	if len(engine2.batchSizes) != 1 || engine2.batchSizes[0] != 8 {
		t.Fatalf("expected remaining batch of 8, got %v", engine2.batchSizes)
	}
	if len(store2.entries) != 40 {
		t.Fatalf("expected full 40 entries after second boot, got %d", len(store2.entries))
	}
}

func TestCacheGetPut_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_cache.db")

	store, _ := NewEmbeddedCorpusStoreWithCache(4, dbPath)
	defer store.Close()

	testHash := hashText("hello world")
	testVec := []float32{0.1, 0.2, 0.3, 0.4}

	// Should return nil before put
	if got := store.cacheGet(testHash, "test-model"); got != nil {
		t.Errorf("expected nil before put, got %v", got)
	}

	// Put
	store.cachePut(testHash, "test-model", testVec)

	// Get
	got := store.cacheGet(testHash, "test-model")
	if got == nil {
		t.Fatal("expected non-nil after put")
	}
	if len(got) != len(testVec) {
		t.Fatalf("length mismatch: expected %d, got %d", len(testVec), len(got))
	}
	for i, v := range testVec {
		if got[i] != v {
			t.Errorf("index %d: expected %f, got %f", i, v, got[i])
		}
	}

	// Different model name should miss
	if got := store.cacheGet(testHash, "other-model"); got != nil {
		t.Errorf("expected nil for different model, got %v", got)
	}
}
