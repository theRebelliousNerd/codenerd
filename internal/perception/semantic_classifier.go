// Package perception - SemanticClassifier performs vector-based intent classification.
// This component bridges vector search with Mangle fact injection for the
// neuro-symbolic intent classification pipeline.
//
// Architecture:
//
//	User Input: "check my code for security issues"
//	     |
//	SemanticClassifier.Classify()
//	     |
//	1. Embed input with RETRIEVAL_QUERY task type
//	2. Search BOTH stores (embedded + learned) in parallel
//	3. Merge results by similarity score
//	4. Assert semantic_match facts into Mangle kernel
//	5. Return matches for debugging/logging
package perception

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	storepkg "codenerd/internal/store"

	"golang.org/x/sync/errgroup"
)

// =============================================================================
// SEMANTIC MATCH TYPE
// =============================================================================

// SemanticMatch represents a match from the semantic vector store.
// This is the unified type for both embedded and learned corpus matches.
type SemanticMatch struct {
	// TextContent is the canonical sentence that matched
	TextContent string

	// Verb is the Mangle name constant (e.g., /review, /fix)
	Verb string

	// Target is the default target for this verb (may be empty)
	Target string

	// Constraint is any constraint associated with this match
	Constraint string

	// Similarity is the cosine similarity score (0.0-1.0)
	Similarity float64

	// Rank is the position in the result set (1-based)
	Rank int

	// Source indicates where this match came from ("embedded" or "learned")
	Source string
}

// =============================================================================
// CORPUS STORE INTERFACES
// =============================================================================

// EmbeddedCorpusStore provides access to the baked-in intent corpus.
// This store is read-only and loaded from the embedded database at startup.
type EmbeddedCorpusStore struct {
	mu         sync.RWMutex
	embeddings map[string][]float32 // TextContent -> embedding vector
	entries    []CorpusEntry        // All corpus entries
	dimensions int                  // Embedding dimensions
}

// LearnedCorpusStore provides access to dynamically learned patterns.
// This store is backed by SQLite and grows over time via autopoiesis.
type LearnedCorpusStore struct {
	mu         sync.RWMutex
	embeddings map[string][]float32 // TextContent -> embedding vector
	entries    []CorpusEntry        // All corpus entries
	dimensions int                  // Embedding dimensions
	backend    *storepkg.LearnedCorpusStore
}

// CorpusEntry represents a single entry in either corpus store.
type CorpusEntry struct {
	TextContent string
	Verb        string
	Target      string
	Constraint  string
	Confidence  float64 // Base confidence for this pattern
}

// =============================================================================
// SEMANTIC CONFIG
// =============================================================================

// SemanticConfig holds classifier configuration.
type SemanticConfig struct {
	// TopK is the number of results per store (default: 5)
	TopK int

	// MinSimilarity is the minimum similarity threshold (default: 0.5)
	MinSimilarity float64

	// LearnedBoost is the boost for learned patterns (default: 0.1)
	// This gives user-learned patterns a slight advantage over baked-in ones
	LearnedBoost float64

	// EnableParallel enables parallel search of stores (default: true)
	EnableParallel bool
}

// DefaultSemanticConfig returns sensible defaults.
func DefaultSemanticConfig() SemanticConfig {
	return SemanticConfig{
		TopK:           5,
		MinSimilarity:  0.5,
		LearnedBoost:   0.1,
		EnableParallel: true,
	}
}

// =============================================================================
// SEMANTIC CLASSIFIER
// =============================================================================

// SemanticClassifier performs vector-based intent classification.
// It searches both embedded (baked-in) and learned (dynamic) corpus stores
// and injects semantic_match facts into the Mangle kernel.
type SemanticClassifier struct {
	mu            sync.RWMutex
	embeddedStore *EmbeddedCorpusStore
	learnedStore  *LearnedCorpusStore
	embedEngine   embedding.EmbeddingEngine
	kernel        core.Kernel
	config        SemanticConfig
}

// NewSemanticClassifier creates a new classifier with both stores.
func NewSemanticClassifier(
	kernel core.Kernel,
	embeddedStore *EmbeddedCorpusStore,
	learnedStore *LearnedCorpusStore,
	embedEngine embedding.EmbeddingEngine,
) *SemanticClassifier {
	logging.Perception("Creating SemanticClassifier")

	sc := &SemanticClassifier{
		kernel:        kernel,
		embeddedStore: embeddedStore,
		learnedStore:  learnedStore,
		embedEngine:   embedEngine,
		config:        DefaultSemanticConfig(),
	}

	logging.PerceptionDebug("SemanticClassifier created with TopK=%d, MinSimilarity=%.2f, LearnedBoost=%.2f",
		sc.config.TopK, sc.config.MinSimilarity, sc.config.LearnedBoost)

	return sc
}

// NewSemanticClassifierFromConfig creates a classifier using config settings.
// This is the main constructor for production use.
func NewSemanticClassifierFromConfig(kernel core.Kernel, cfg *config.UserConfig) (*SemanticClassifier, error) {
	timer := logging.StartTimer(logging.CategoryPerception, "NewSemanticClassifierFromConfig")
	defer timer.Stop()

	logging.Perception("Initializing SemanticClassifier from config")

	// Get embedding configuration
	embedCfg := cfg.GetEmbeddingConfig()

	// Create embedding engine
	engineCfg := embedding.Config{
		Provider:       embedCfg.Provider,
		OllamaEndpoint: embedCfg.OllamaEndpoint,
		OllamaModel:    embedCfg.OllamaModel,
		GenAIAPIKey:    embedCfg.GenAIAPIKey,
		GenAIModel:     embedCfg.GenAIModel,
		TaskType:       "RETRIEVAL_QUERY", // Use RETRIEVAL_QUERY for classification
	}

	embedEngine, err := embedding.NewEngine(engineCfg)
	if err != nil {
		logging.Get(logging.CategoryPerception).Warn("Failed to create embedding engine: %v (semantic classification disabled)", err)
		// Return classifier without embedding engine (graceful degradation)
		return &SemanticClassifier{
			kernel:        kernel,
			embeddedStore: nil,
			learnedStore:  nil,
			embedEngine:   nil,
			config:        DefaultSemanticConfig(),
		}, nil
	}

	logging.PerceptionDebug("Embedding engine created: %s (dimensions=%d)", embedEngine.Name(), embedEngine.Dimensions())

	// Initialize embedded corpus store
	embeddedStore, err := NewEmbeddedCorpusStore(embedEngine.Dimensions())
	if err != nil {
		logging.Get(logging.CategoryPerception).Warn("Failed to load embedded corpus: %v", err)
		embeddedStore = nil
	}
	if embeddedStore != nil && embedEngine != nil {
		if err := embeddedStore.LoadFromKernel(context.Background(), kernel, embedEngine); err != nil {
			logging.Get(logging.CategoryPerception).Warn("Failed to hydrate embedded intent corpus from kernel: %v", err)
		}
	}

	// Initialize learned corpus store
	learnedStore, err := NewLearnedCorpusStore(cfg, embedEngine.Dimensions(), embedEngine)
	if err != nil {
		logging.Get(logging.CategoryPerception).Warn("Failed to load learned corpus: %v", err)
		learnedStore = nil
	}

	sc := &SemanticClassifier{
		kernel:        kernel,
		embeddedStore: embeddedStore,
		learnedStore:  learnedStore,
		embedEngine:   embedEngine,
		config:        DefaultSemanticConfig(),
	}

	logging.Perception("SemanticClassifier initialized successfully (embedded=%v, learned=%v)",
		embeddedStore != nil, learnedStore != nil)

	return sc, nil
}

// SetConfig updates the classifier configuration.
func (sc *SemanticClassifier) SetConfig(cfg SemanticConfig) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.config = cfg
	logging.PerceptionDebug("SemanticClassifier config updated: TopK=%d, MinSimilarity=%.2f",
		cfg.TopK, cfg.MinSimilarity)
}

// Classify performs semantic classification and injects facts into kernel.
// Returns the merged matches for debugging/logging.
func (sc *SemanticClassifier) Classify(ctx context.Context, input string) ([]SemanticMatch, error) {
	timer := logging.StartTimer(logging.CategoryPerception, "SemanticClassifier.Classify")
	defer timer.Stop()

	logging.PerceptionDebug("Classifying input: %q", truncateForLog(input, 100))

	// Perform classification without injection first
	matches, err := sc.ClassifyWithoutInjection(ctx, input)
	if err != nil {
		return nil, err
	}

	// Inject semantic_match facts into kernel
	sc.injectFacts(input, matches)

	return matches, nil
}

// ClassifyWithoutInjection performs classification without kernel injection.
// Useful for testing or preview mode.
func (sc *SemanticClassifier) ClassifyWithoutInjection(ctx context.Context, input string) ([]SemanticMatch, error) {
	timer := logging.StartTimer(logging.CategoryPerception, "SemanticClassifier.ClassifyWithoutInjection")
	defer timer.Stop()

	sc.mu.RLock()
	embedEngine := sc.embedEngine
	embeddedStore := sc.embeddedStore
	learnedStore := sc.learnedStore
	cfg := sc.config
	sc.mu.RUnlock()

	// 1. Generate query embedding with RETRIEVAL_QUERY task type
	if embedEngine == nil {
		logging.PerceptionDebug("No embedding engine available, returning empty matches")
		return nil, nil
	}

	queryTask := embedding.SelectTaskType(embedding.ContentTypeQuery, true)
	var queryEmbed []float32
	var err error
	if taskAware, ok := embedEngine.(embedding.TaskTypeAwareEngine); ok && queryTask != "" {
		queryEmbed, err = taskAware.EmbedWithTask(ctx, input, queryTask)
	} else {
		queryEmbed, err = embedEngine.Embed(ctx, input)
	}
	if err != nil {
		// Graceful degradation: return empty matches, don't fail
		logging.Get(logging.CategoryPerception).Warn("Semantic embedding failed: %v, falling back to regex-only", err)
		return nil, nil
	}

	logging.PerceptionDebug("Query embedding generated: %d dimensions", len(queryEmbed))

	// 2. Search both stores
	var embeddedMatches, learnedMatches []SemanticMatch

	if cfg.EnableParallel {
		// Parallel search using errgroup
		g, gctx := errgroup.WithContext(ctx)

		g.Go(func() error {
			if embeddedStore == nil {
				return nil
			}
			select {
			case <-gctx.Done():
				return gctx.Err()
			default:
				var searchErr error
				embeddedMatches, searchErr = embeddedStore.Search(queryEmbed, cfg.TopK)
				if searchErr != nil {
					logging.Get(logging.CategoryPerception).Warn("Embedded store search failed: %v", searchErr)
				}
				return nil // Don't fail the group on search error
			}
		})

		g.Go(func() error {
			if learnedStore == nil {
				return nil
			}
			select {
			case <-gctx.Done():
				return gctx.Err()
			default:
				var searchErr error
				learnedMatches, searchErr = learnedStore.Search(queryEmbed, cfg.TopK)
				if searchErr != nil {
					logging.Get(logging.CategoryPerception).Warn("Learned store search failed: %v", searchErr)
				}
				return nil // Don't fail the group on search error
			}
		})

		if err := g.Wait(); err != nil {
			// Only fail on context cancellation
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			logging.Get(logging.CategoryPerception).Warn("Semantic search partial failure: %v", err)
		}
	} else {
		// Sequential search
		if embeddedStore != nil {
			embeddedMatches, _ = embeddedStore.Search(queryEmbed, cfg.TopK)
		}
		if learnedStore != nil {
			learnedMatches, _ = learnedStore.Search(queryEmbed, cfg.TopK)
		}
	}

	logging.PerceptionDebug("Search results: embedded=%d, learned=%d", len(embeddedMatches), len(learnedMatches))

	// 3. Merge results with learned pattern boost
	merged := sc.mergeResults(embeddedMatches, learnedMatches, cfg)

	// 4. Filter by minimum similarity
	filtered := sc.filterByThreshold(merged, cfg.MinSimilarity)

	logging.PerceptionDebug("After merge and filter: %d matches (threshold=%.2f)", len(filtered), cfg.MinSimilarity)

	return filtered, nil
}

// mergeResults combines embedded and learned matches with proper scoring.
func (sc *SemanticClassifier) mergeResults(embedded, learned []SemanticMatch, cfg SemanticConfig) []SemanticMatch {
	// Apply boost to learned patterns
	for i := range learned {
		learned[i].Similarity += cfg.LearnedBoost
		if learned[i].Similarity > 1.0 {
			learned[i].Similarity = 1.0
		}
	}

	// Combine all matches
	all := make([]SemanticMatch, 0, len(embedded)+len(learned))
	all = append(all, embedded...)
	all = append(all, learned...)

	// Sort by similarity descending
	sort.Slice(all, func(i, j int) bool {
		return all[i].Similarity > all[j].Similarity
	})

	// Deduplicate by verb+text (keep highest similarity)
	seen := make(map[string]bool)
	deduped := make([]SemanticMatch, 0, len(all))
	for _, m := range all {
		key := m.Verb + "|" + m.TextContent
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, m)
		}
	}

	// Limit to 2x TopK
	maxResults := cfg.TopK * 2
	if len(deduped) > maxResults {
		deduped = deduped[:maxResults]
	}

	// Re-assign ranks (1-based)
	for i := range deduped {
		deduped[i].Rank = i + 1
	}

	return deduped
}

// filterByThreshold removes matches below the minimum similarity threshold.
func (sc *SemanticClassifier) filterByThreshold(matches []SemanticMatch, minSimilarity float64) []SemanticMatch {
	filtered := make([]SemanticMatch, 0, len(matches))
	for _, m := range matches {
		if m.Similarity >= minSimilarity {
			filtered = append(filtered, m)
		}
	}

	// Re-assign ranks after filtering
	for i := range filtered {
		filtered[i].Rank = i + 1
	}

	return filtered
}

// injectFacts asserts semantic_match facts into the Mangle kernel.
func (sc *SemanticClassifier) injectFacts(input string, matches []SemanticMatch) {
	sc.mu.RLock()
	kernel := sc.kernel
	sc.mu.RUnlock()

	if kernel == nil {
		logging.PerceptionDebug("No kernel available, skipping fact injection")
		return
	}

	facts := make([]core.Fact, 0, len(matches))
	for _, match := range matches {
		// semantic_match(UserInput, CanonicalSentence, Verb, Target, Rank, Similarity)
		// Note: Similarity is scaled to 0-100 integer for Mangle compatibility
		facts = append(facts, core.Fact{
			Predicate: "semantic_match",
			Args: []interface{}{
				input,
				match.TextContent,
				core.MangleAtom(match.Verb),
				match.Target,
				int64(match.Rank),
				int64(match.Similarity * 100), // 0-100 scale
			},
		})
	}

	if len(facts) == 0 {
		logging.PerceptionDebug("Injected 0 semantic_match facts")
		return
	}

	// Batch load to reduce kernel rebuild frequency. Fallback to per-assert on error.
	if err := kernel.LoadFacts(facts); err != nil {
		injectedCount := 0
		for _, fact := range facts {
			if err := kernel.Assert(fact); err != nil {
				logging.Get(logging.CategoryPerception).Warn("Failed to assert semantic_match: %v", err)
			} else {
				injectedCount++
			}
		}
		logging.PerceptionDebug("Injected %d/%d semantic_match facts (fallback)", injectedCount, len(facts))
		return
	}

	logging.PerceptionDebug("Injected %d semantic_match facts", len(facts))
}

// AddLearnedPattern adds a new learned pattern to the dynamic store.
// Called by the autopoiesis/learning system.
func (sc *SemanticClassifier) AddLearnedPattern(ctx context.Context, pattern, verb, target, constraint string, confidence float64) error {
	timer := logging.StartTimer(logging.CategoryPerception, "SemanticClassifier.AddLearnedPattern")
	defer timer.Stop()

	sc.mu.RLock()
	learnedStore := sc.learnedStore
	embedEngine := sc.embedEngine
	sc.mu.RUnlock()

	if learnedStore == nil {
		return fmt.Errorf("learned store not available")
	}
	if embedEngine == nil {
		return fmt.Errorf("embedding engine not available")
	}

	logging.Perception("Adding learned pattern: verb=%s, pattern=%q", verb, truncateForLog(pattern, 50))

	// Generate embedding for the new pattern (document-side of retrieval)
	patternTask := embedding.SelectTaskType(embedding.ContentTypeKnowledgeAtom, false)
	var patternEmbed []float32
	var err error
	if taskAware, ok := embedEngine.(embedding.TaskTypeAwareEngine); ok && patternTask != "" {
		patternEmbed, err = taskAware.EmbedWithTask(ctx, pattern, patternTask)
	} else {
		patternEmbed, err = embedEngine.Embed(ctx, pattern)
	}
	if err != nil {
		return fmt.Errorf("failed to generate embedding for pattern: %w", err)
	}

	// Add to learned store
	entry := CorpusEntry{
		TextContent: pattern,
		Verb:        verb,
		Target:      target,
		Constraint:  constraint,
		Confidence:  confidence,
	}

	if err := learnedStore.Add(entry, patternEmbed); err != nil {
		return fmt.Errorf("failed to add pattern to learned store: %w", err)
	}

	logging.Perception("Learned pattern added successfully: verb=%s", verb)
	return nil
}

// Close cleans up resources.
func (sc *SemanticClassifier) Close() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	logging.Perception("Closing SemanticClassifier")

	var errs []error

	if sc.learnedStore != nil {
		if err := sc.learnedStore.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close learned store: %w", err))
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// =============================================================================
// EMBEDDED CORPUS STORE IMPLEMENTATION
// =============================================================================

// NewEmbeddedCorpusStore loads the baked-in intent corpus.
func NewEmbeddedCorpusStore(dimensions int) (*EmbeddedCorpusStore, error) {
	timer := logging.StartTimer(logging.CategoryPerception, "NewEmbeddedCorpusStore")
	defer timer.Stop()

	logging.Perception("Loading embedded corpus store (dimensions=%d)", dimensions)

	store := &EmbeddedCorpusStore{
		embeddings: make(map[string][]float32),
		entries:    make([]CorpusEntry, 0),
		dimensions: dimensions,
	}

	// Base store is empty; canonical patterns can be hydrated from kernel at runtime.
	logging.PerceptionDebug("Embedded corpus store initialized (entries=%d)", len(store.entries))

	return store, nil
}

// LoadFromKernel hydrates the embedded corpus from intent_definition facts in the kernel.
// This preserves the split-brain architecture: Mangle stores canonical patterns as data,
// while semantic matching uses embeddings over those patterns.
func (s *EmbeddedCorpusStore) LoadFromKernel(ctx context.Context, kernel core.Kernel, engine embedding.EmbeddingEngine) error {
	if s == nil || kernel == nil || engine == nil {
		return nil
	}

	facts, err := kernel.Query("intent_definition")
	if err != nil {
		return err
	}
	if len(facts) == 0 {
		return nil
	}

	entries := make([]CorpusEntry, 0, len(facts))
	texts := make([]string, 0, len(facts))
	for _, f := range facts {
		if len(f.Args) < 2 {
			continue
		}
		phrase := argToString(f.Args[0])
		verb := argToString(f.Args[1])
		target := ""
		if len(f.Args) > 2 {
			target = argToString(f.Args[2])
		}
		if phrase == "" || verb == "" {
			continue
		}
		entries = append(entries, CorpusEntry{
			TextContent: phrase,
			Verb:        verb,
			Target:      target,
			Confidence:  0.9,
		})
		texts = append(texts, phrase)
	}

	if len(texts) == 0 {
		return nil
	}

	taskType := embedding.SelectTaskType(embedding.ContentTypeKnowledgeAtom, false)
	var embeds [][]float32
	if batchAware, ok := engine.(embedding.TaskTypeBatchAwareEngine); ok && taskType != "" {
		embeds, err = batchAware.EmbedBatchWithTask(ctx, texts, taskType)
	} else if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok && taskType != "" {
		embeds = make([][]float32, len(texts))
		for i, text := range texts {
			vec, embedErr := taskAware.EmbedWithTask(ctx, text, taskType)
			if embedErr != nil {
				continue
			}
			embeds[i] = vec
		}
	} else {
		embeds, err = engine.EmbedBatch(ctx, texts)
	}
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	added := 0
	for i, entry := range entries {
		if i >= len(embeds) {
			break
		}
		vec := embeds[i]
		if len(vec) != s.dimensions {
			continue
		}
		s.entries = append(s.entries, entry)
		s.embeddings[entry.TextContent] = vec
		added++
	}

	logging.PerceptionDebug("Hydrated embedded intent corpus from kernel: added=%d", added)
	return nil
}

func argToString(arg interface{}) string {
	switch v := arg.(type) {
	case string:
		return v
	case core.MangleAtom:
		return string(v)
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// Search performs cosine similarity search on the embedded corpus.
func (s *EmbeddedCorpusStore) Search(queryEmbed []float32, topK int) ([]SemanticMatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.entries) == 0 {
		return nil, nil
	}

	if topK <= 0 {
		topK = 5
	}

	// Calculate similarity for each entry
	type scored struct {
		entry      CorpusEntry
		similarity float64
	}

	candidates := make([]scored, 0, len(s.entries))
	for _, entry := range s.entries {
		entryEmbed, ok := s.embeddings[entry.TextContent]
		if !ok {
			continue
		}

		sim, err := embedding.CosineSimilarity(queryEmbed, entryEmbed)
		if err != nil {
			continue
		}

		candidates = append(candidates, scored{
			entry:      entry,
			similarity: sim,
		})
	}

	// Sort by similarity descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].similarity > candidates[j].similarity
	})

	// Take top K
	if len(candidates) > topK {
		candidates = candidates[:topK]
	}

	// Convert to SemanticMatch
	results := make([]SemanticMatch, len(candidates))
	for i, c := range candidates {
		results[i] = SemanticMatch{
			TextContent: c.entry.TextContent,
			Verb:        c.entry.Verb,
			Target:      c.entry.Target,
			Constraint:  c.entry.Constraint,
			Similarity:  c.similarity,
			Rank:        i + 1,
			Source:      "embedded",
		}
	}

	return results, nil
}

// =============================================================================
// LEARNED CORPUS STORE IMPLEMENTATION
// =============================================================================

// NewLearnedCorpusStore initializes the learned patterns store.
// In production this is backed by `.nerd/learned_patterns.db`; tests/dev fall back to memory.
func NewLearnedCorpusStore(cfg *config.UserConfig, dimensions int, embedEngine embedding.EmbeddingEngine) (*LearnedCorpusStore, error) {
	timer := logging.StartTimer(logging.CategoryPerception, "NewLearnedCorpusStore")
	defer timer.Stop()

	logging.Perception("Loading learned corpus store (dimensions=%d)", dimensions)

	store := &LearnedCorpusStore{
		embeddings: make(map[string][]float32),
		entries:    make([]CorpusEntry, 0),
		dimensions: dimensions,
	}

	// If no config or embedding engine, fall back to in-memory store (tests/dev).
	if cfg == nil || embedEngine == nil {
		logging.PerceptionDebug("Learned corpus store initialized in-memory (entries=%d)", len(store.entries))
		return store, nil
	}

	// Only create DB if we have a proper workspace root to avoid creating .nerd in wrong directories
	if SharedTaxonomy == nil || !SharedTaxonomy.HasWorkspace() {
		logging.PerceptionDebug("Learned corpus store initialized in-memory (no workspace root)")
		return store, nil
	}

	dbPath := SharedTaxonomy.nerdPath("learned_patterns.db")
	backend, err := storepkg.NewLearnedCorpusStore(dbPath, embedEngine)
	if err != nil {
		return nil, err
	}
	store.backend = backend
	logging.PerceptionDebug("Learned corpus store initialized with DB backend (path=%s)", dbPath)

	return store, nil
}

// Search performs cosine similarity search on the learned corpus.
func (s *LearnedCorpusStore) Search(queryEmbed []float32, topK int) ([]SemanticMatch, error) {
	s.mu.RLock()
	backend := s.backend
	s.mu.RUnlock()

	if backend != nil {
		matches, err := backend.Search(queryEmbed, topK)
		if err != nil {
			return nil, err
		}
		results := make([]SemanticMatch, len(matches))
		for i, m := range matches {
			results[i] = SemanticMatch{
				TextContent: m.TextContent,
				Verb:        m.Verb,
				Target:      m.Target,
				Constraint:  "",
				Similarity:  m.Similarity,
				Rank:        m.Rank,
				Source:      "learned",
			}
		}
		return results, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.entries) == 0 {
		return nil, nil
	}

	if topK <= 0 {
		topK = 5
	}

	// Calculate similarity for each entry
	type scored struct {
		entry      CorpusEntry
		similarity float64
	}

	candidates := make([]scored, 0, len(s.entries))
	for _, entry := range s.entries {
		entryEmbed, ok := s.embeddings[entry.TextContent]
		if !ok {
			continue
		}

		sim, err := embedding.CosineSimilarity(queryEmbed, entryEmbed)
		if err != nil {
			continue
		}

		candidates = append(candidates, scored{
			entry:      entry,
			similarity: sim,
		})
	}

	// Sort by similarity descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].similarity > candidates[j].similarity
	})

	// Take top K
	if len(candidates) > topK {
		candidates = candidates[:topK]
	}

	// Convert to SemanticMatch
	results := make([]SemanticMatch, len(candidates))
	for i, c := range candidates {
		results[i] = SemanticMatch{
			TextContent: c.entry.TextContent,
			Verb:        c.entry.Verb,
			Target:      c.entry.Target,
			Constraint:  c.entry.Constraint,
			Similarity:  c.similarity,
			Rank:        i + 1,
			Source:      "learned",
		}
	}

	return results, nil
}

// Add adds a new pattern to the learned store.
func (s *LearnedCorpusStore) Add(entry CorpusEntry, entryEmbed []float32) error {
	s.mu.RLock()
	backend := s.backend
	dims := s.dimensions
	s.mu.RUnlock()

	if backend != nil {
		if dims > 0 && len(entryEmbed) != 0 && len(entryEmbed) != dims {
			return fmt.Errorf("embedding dimension mismatch: expected %d, got %d", dims, len(entryEmbed))
		}
		return backend.AddPattern(context.Background(), entry.TextContent, entry.Verb, entry.Target, entry.Constraint, entry.Confidence)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate embedding dimensions
	if len(entryEmbed) != s.dimensions {
		return fmt.Errorf("embedding dimension mismatch: expected %d, got %d", s.dimensions, len(entryEmbed))
	}

	// Add to in-memory store
	s.entries = append(s.entries, entry)
	s.embeddings[entry.TextContent] = entryEmbed

	logging.PerceptionDebug("Added learned pattern: verb=%s, text=%q", entry.Verb, truncateForLog(entry.TextContent, 50))
	return nil
}

// Close persists any pending changes and closes the store.
func (s *LearnedCorpusStore) Close() error {
	s.mu.RLock()
	backend := s.backend
	s.mu.RUnlock()

	if backend != nil {
		return backend.Close()
	}

	logging.PerceptionDebug("Learned corpus store closed (in-memory)")
	return nil
}

// =============================================================================
// SHARED INSTANCE (Package-level)
// =============================================================================

// SharedSemanticClassifier is the global instance.
// Initialized by InitSemanticClassifier().
var SharedSemanticClassifier *SemanticClassifier

// sharedClassifierMu protects SharedSemanticClassifier initialization.
var sharedClassifierMu sync.Mutex

// InitSemanticClassifier initializes the shared classifier.
func InitSemanticClassifier(kernel core.Kernel, cfg *config.UserConfig) error {
	sharedClassifierMu.Lock()
	defer sharedClassifierMu.Unlock()

	if SharedSemanticClassifier != nil {
		logging.PerceptionDebug("SemanticClassifier already initialized, skipping")
		return nil
	}

	logging.Perception("Initializing shared SemanticClassifier")

	var err error
	SharedSemanticClassifier, err = NewSemanticClassifierFromConfig(kernel, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize semantic classifier: %w", err)
	}

	logging.Perception("Shared SemanticClassifier initialized successfully")
	return nil
}

// CloseSemanticClassifier closes the shared classifier and releases resources.
func CloseSemanticClassifier() error {
	sharedClassifierMu.Lock()
	defer sharedClassifierMu.Unlock()

	if SharedSemanticClassifier == nil {
		return nil
	}

	err := SharedSemanticClassifier.Close()
	SharedSemanticClassifier = nil
	return err
}
