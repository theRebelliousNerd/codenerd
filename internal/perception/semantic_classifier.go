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
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"codenerd/internal/sqlpragmas"
	storepkg "codenerd/internal/store"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/sync/errgroup"
)

// intentHydrateTimeout bounds boot-time embedding of intent_definition texts.
// A cold cache of ~800 intents at ~200ms each would otherwise freeze the TUI
// for minutes; partial hydrate is preferable to an unbounded hang.
const intentHydrateTimeout = 60 * time.Second

// intentEmbedChunkSize controls how many cache-miss texts are embedded before
// writing them to the SQLite cache. Chunking makes progress durable if the
// process is killed or the hydrate deadline fires mid-batch.
const intentEmbedChunkSize = 32

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
// An optional SQLite cache (cacheDB) persists embeddings between boots so
// that static intent_definition texts are not re-embedded on every startup.
type EmbeddedCorpusStore struct {
	mu         sync.RWMutex
	embeddings map[string][]float32 // TextContent -> embedding vector
	entries    []CorpusEntry        // All corpus entries
	dimensions int                  // Embedding dimensions
	cachePath  string               // Path to SQLite cache DB (empty = no cache)
	cacheDB    *sql.DB              // SQLite connection for embedding cache
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

	// Initialize embedded corpus store with optional cache
	var cachePath string
	if SharedTaxonomy != nil && SharedTaxonomy.HasWorkspace() {
		cachePath = SharedTaxonomy.nerdPath("intent_embeddings.db")
	}

	var embeddedStore *EmbeddedCorpusStore
	if cachePath != "" {
		embeddedStore, err = NewEmbeddedCorpusStoreWithCache(embedEngine.Dimensions(), cachePath)
	} else {
		embeddedStore, err = NewEmbeddedCorpusStore(embedEngine.Dimensions())
	}
	if err != nil {
		logging.Get(logging.CategoryPerception).Warn("Failed to load embedded corpus: %v", err)
		embeddedStore = nil
	}
	if embeddedStore != nil && embedEngine != nil {
		// Bound hydrate so a cold cache (hundreds of Ollama embeds) cannot freeze
		// TUI/CLI boot indefinitely. Partial progress is cached for the next boot.
		hydrateCtx, cancelHydrate := context.WithTimeout(context.Background(), intentHydrateTimeout)
		if err := embeddedStore.LoadFromKernel(hydrateCtx, kernel, embedEngine); err != nil {
			logging.Get(logging.CategoryPerception).Warn("Failed to hydrate embedded intent corpus from kernel: %v", err)
		}
		cancelHydrate()
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
	if len(strings.TrimSpace(input)) == 0 {
		return nil, nil
	}

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
	if len(strings.TrimSpace(input)) == 0 {
		return nil, nil
	}

	const maxClassifyBytes = 32768
	if len(input) > maxClassifyBytes {
		input = input[:maxClassifyBytes] + "... [Input truncated]"
	}

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
	if cfg.TopK < 0 {
		cfg.TopK = 5
	}
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

	// Prevent state accumulation from previous turns
	_ = kernel.Retract("semantic_match")

	facts := make([]core.Fact, 0, len(matches))
	for _, match := range matches {
		var targetArg any = match.Target
		if strings.HasPrefix(match.Target, "/") {
			targetArg = core.MangleAtom(match.Target)
		}

		sim := match.Similarity
		if math.IsNaN(sim) {
			sim = 0
		}
		simInt := int64(math.Max(0, math.Min(100, sim*100)))

		// semantic_match(UserInput, CanonicalSentence, Verb, Target, Rank, Similarity)
		facts = append(facts, core.Fact{
			Predicate: "semantic_match",
			Args: []any{
				input,
				match.TextContent,
				core.MangleAtom(match.Verb),
				targetArg,
				int64(match.Rank),
				simInt,
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

	if sc.embeddedStore != nil {
		if err := sc.embeddedStore.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close embedded store cache: %w", err))
		}
	}

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

// NewEmbeddedCorpusStoreWithCache creates an EmbeddedCorpusStore backed by a SQLite cache.
// The cache persists embeddings between boots so that static intent_definition texts
// are not re-embedded on every startup. If cachePath is empty, behaves identically
// to NewEmbeddedCorpusStore (no cache).
func NewEmbeddedCorpusStoreWithCache(dimensions int, cachePath string) (*EmbeddedCorpusStore, error) {
	timer := logging.StartTimer(logging.CategoryPerception, "NewEmbeddedCorpusStoreWithCache")
	defer timer.Stop()

	logging.Perception("Loading embedded corpus store with cache (dimensions=%d, cache=%s)", dimensions, cachePath)

	store := &EmbeddedCorpusStore{
		embeddings: make(map[string][]float32),
		entries:    make([]CorpusEntry, 0),
		dimensions: dimensions,
		cachePath:  cachePath,
	}

	if cachePath != "" {
		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
			logging.Get(logging.CategoryEmbedding).Warn("Failed to create cache directory: %v (cache disabled)", err)
			return store, nil
		}

		db, err := sql.Open("sqlite3", cachePath)
		if err != nil {
			logging.Get(logging.CategoryEmbedding).Warn("Failed to open embedding cache DB: %v (cache disabled)", err)
			return store, nil
		}
		sqlpragmas.ApplyDefaultPragmas(db, sqlpragmas.ProfileHot)

		// Create cache table
		_, err = db.Exec(`CREATE TABLE IF NOT EXISTS embedding_cache (
			text_hash   TEXT NOT NULL,
			model_name  TEXT NOT NULL,
			embedding   BLOB NOT NULL,
			dimensions  INTEGER NOT NULL,
			created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (text_hash, model_name)
		)`)
		if err != nil {
			db.Close()
			logging.Get(logging.CategoryEmbedding).Warn("Failed to create cache table: %v (cache disabled)", err)
			return store, nil
		}

		store.cacheDB = db
		logging.PerceptionDebug("Embedded corpus cache DB opened: %s", cachePath)
	}

	return store, nil
}

// Close releases the cache DB connection if one is open.
func (s *EmbeddedCorpusStore) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cacheDB != nil {
		logging.PerceptionDebug("Closing embedded corpus cache DB")
		err := s.cacheDB.Close()
		s.cacheDB = nil
		return err
	}
	return nil
}

// hashText returns the hex-encoded SHA-256 hash of the given text.
func hashText(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}

// float32ToBytes converts a []float32 to a []byte for BLOB storage.
func float32ToBytes(vec []float32) []byte {
	buf := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// bytesToFloat32 converts a []byte BLOB back to []float32.
func bytesToFloat32(buf []byte) []float32 {
	if len(buf)%4 != 0 {
		return nil
	}
	vec := make([]float32, len(buf)/4)
	for i := range vec {
		vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
	}
	return vec
}

// cacheGet retrieves a cached embedding by text hash and model name.
// Returns nil if not found or on error.
func (s *EmbeddedCorpusStore) cacheGet(textHash, modelName string) []float32 {
	if s.cacheDB == nil {
		return nil
	}
	var blob []byte
	err := s.cacheDB.QueryRow(
		"SELECT embedding FROM embedding_cache WHERE text_hash = ? AND model_name = ?",
		textHash, modelName,
	).Scan(&blob)
	if err != nil {
		return nil
	}
	return bytesToFloat32(blob)
}

// cachePut stores an embedding in the cache.
func (s *EmbeddedCorpusStore) cachePut(textHash, modelName string, vec []float32) {
	if s.cacheDB == nil {
		return
	}
	blob := float32ToBytes(vec)
	_, err := s.cacheDB.Exec(
		"INSERT OR REPLACE INTO embedding_cache (text_hash, model_name, embedding, dimensions) VALUES (?, ?, ?, ?)",
		textHash, modelName, blob, len(vec),
	)
	if err != nil {
		logging.Get(logging.CategoryEmbedding).Warn("Failed to cache embedding: %v", err)
	}
}

// LoadFromKernel hydrates the embedded corpus from intent_definition facts in the kernel.
// This preserves the split-brain architecture: Mangle stores canonical patterns as data,
// while semantic matching uses embeddings over those patterns.
//
// When a SQLite cache is configured, embeddings are looked up by content hash + model
// name before falling back to the embedding engine. Only cache misses require API calls.
func (s *EmbeddedCorpusStore) LoadFromKernel(ctx context.Context, kernel core.Kernel, engine embedding.EmbeddingEngine) error {
	if s == nil || kernel == nil || engine == nil {
		return nil
	}

	// Prevent ghost duplication
	s.mu.Lock()
	s.entries = make([]CorpusEntry, 0)
	s.embeddings = make(map[string][]float32)
	s.mu.Unlock()

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

	// ─── Cache-aware embedding resolution ───────────────────────────────
	modelName := engine.Name()
	allEmbeds := make([][]float32, len(texts))
	var missTexts []string
	var missIndices []int
	hits := 0

	if s.cacheDB != nil {
		for i, text := range texts {
			h := hashText(text)
			if cached := s.cacheGet(h, modelName); cached != nil && len(cached) == s.dimensions {
				allEmbeds[i] = cached
				hits++
			} else {
				missTexts = append(missTexts, text)
				missIndices = append(missIndices, i)
			}
		}
	} else {
		// No cache — all texts are misses
		missTexts = texts
		missIndices = make([]int, len(texts))
		for i := range texts {
			missIndices[i] = i
		}
	}

	misses := len(missTexts)
	logging.Get(logging.CategoryEmbedding).Info("EmbeddedCorpusStore cache: %d hits, %d misses (model=%s)", hits, misses, modelName)

	// Embed cache misses in small chunks and write each chunk to the SQLite cache
	// immediately. Previously a single EmbedBatch of ~800 texts wrote nothing until
	// the full batch finished (~2–3 min on Ollama); killing the process mid-batch
	// forced a full re-embed on the next boot (TUI freeze loop).
	if len(missTexts) > 0 {
		taskType := embedding.SelectTaskType(embedding.ContentTypeKnowledgeAtom, false)
		chunkSize := intentEmbedChunkSize
		if chunkSize < 1 {
			chunkSize = 32
		}
		cachedMisses := 0
		for start := 0; start < len(missTexts); start += chunkSize {
			if err := ctx.Err(); err != nil {
				logging.Get(logging.CategoryEmbedding).Warn(
					"EmbeddedCorpusStore: hydrate stopped early after caching %d/%d misses (%v); remaining will embed on next boot",
					cachedMisses, misses, err,
				)
				break
			}
			end := start + chunkSize
			if end > len(missTexts) {
				end = len(missTexts)
			}
			chunkTexts := missTexts[start:end]
			chunkIndices := missIndices[start:end]

			var chunkEmbeds [][]float32
			var embedErr error
			if batchAware, ok := engine.(embedding.TaskTypeBatchAwareEngine); ok && taskType != "" {
				chunkEmbeds, embedErr = batchAware.EmbedBatchWithTask(ctx, chunkTexts, taskType)
			} else if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok && taskType != "" {
				chunkEmbeds = make([][]float32, len(chunkTexts))
				for i, text := range chunkTexts {
					if ctx.Err() != nil {
						embedErr = ctx.Err()
						break
					}
					vec, e := taskAware.EmbedWithTask(ctx, text, taskType)
					if e != nil {
						continue
					}
					chunkEmbeds[i] = vec
				}
			} else {
				chunkEmbeds, embedErr = engine.EmbedBatch(ctx, chunkTexts)
			}
			if embedErr != nil {
				// Cancellation/deadline: keep partial progress, do not fail boot.
				if errors.Is(embedErr, context.Canceled) || errors.Is(embedErr, context.DeadlineExceeded) || ctx.Err() != nil {
					logging.Get(logging.CategoryEmbedding).Warn(
						"EmbeddedCorpusStore: embed cancelled after caching %d/%d misses: %v",
						cachedMisses, misses, embedErr,
					)
					break
				}
				return embedErr
			}

			for j, idx := range chunkIndices {
				if j >= len(chunkEmbeds) {
					break
				}
				vec := chunkEmbeds[j]
				allEmbeds[idx] = vec
				if vec != nil && s.cacheDB != nil {
					s.cachePut(hashText(chunkTexts[j]), modelName, vec)
					cachedMisses++
				}
			}
			if end < len(missTexts) {
				logging.Get(logging.CategoryEmbedding).Info(
					"EmbeddedCorpusStore: cached miss progress %d/%d", end, misses,
				)
			}
		}
	}

	// ─── Populate in-memory store ────────────────────────────────────────
	s.mu.Lock()
	defer s.mu.Unlock()

	added := 0
	for i, entry := range entries {
		if i >= len(allEmbeds) {
			break
		}
		vec := allEmbeds[i]
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

func argToString(arg any) string {
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
