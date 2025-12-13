// Package researcher implements the Deep Research ShardAgent (Type B: Persistent Specialist).
// This file contains the core ResearcherShard struct and lifecycle methods.
//
// # JIT Prompt Compiler Integration
//
// The ResearcherShard now supports dynamic prompt generation through the JIT prompt compiler:
//
// - System prompts are assembled from kernel-derived context atoms
// - Session state (dream mode, campaign context, etc.) is automatically injected
// - Specialist knowledge from the knowledge graph is hydrated on-demand
// - Token budgets are automatically managed to prevent context overflow
//
// # Usage
//
//	researcher := researcher.NewResearcherShard()
//	researcher.SetParentKernel(kernel)  // Initializes PromptAssembler
//	researcher.SetJITCompiler(jit)      // Optional: enables JIT compilation
//	researcher.EnableJIT(true)          // Optional: toggle JIT on/off
//
// # Fallback Behavior
//
// If JIT compilation fails or is unavailable, the shard falls back to:
// 1. Legacy prompt assembly (buildLegacySystemPrompt)
// 2. Static prompt templates with session context injection
// 3. Minimal prompts for tactical operations
//
// This ensures the shard remains functional even without JIT infrastructure.
package researcher

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/world"
)

// ResearchConfig holds configuration for the researcher shard.
type ResearchConfig struct {
	MaxPages        int           // Maximum pages to scrape per research task
	MaxDepth        int           // Maximum link traversal depth
	Timeout         time.Duration // Timeout per page fetch
	ConcurrentFetch int           // Number of concurrent fetches
	AllowedDomains  []string      // Whitelist of domains (empty = all allowed)
	BlockedDomains  []string      // Blacklist of domains
	UserAgent       string        // HTTP User-Agent string
}

// DefaultResearchConfig returns sensible defaults for research.
func DefaultResearchConfig() ResearchConfig {
	return ResearchConfig{
		MaxPages:        20,
		MaxDepth:        2,
		Timeout:         90 * time.Second, // Increased for paginated Context7 fetches
		ConcurrentFetch: 2,                // Reduced to avoid overwhelming APIs
		AllowedDomains:  []string{},       // Allow all by default
		BlockedDomains: []string{
			"facebook.com", "twitter.com", "instagram.com",
			"linkedin.com", "tiktok.com", // Social media noise
		},
		UserAgent: "codeNERD/1.5.0 (Deep Research Agent)",
	}
}

// KnowledgeAtom represents a piece of extracted knowledge.
type KnowledgeAtom struct {
	SourceURL   string                 `json:"source_url"`
	Title       string                 `json:"title"`
	Content     string                 `json:"content"`
	Concept     string                 `json:"concept"`
	CodePattern string                 `json:"code_pattern,omitempty"`
	AntiPattern string                 `json:"anti_pattern,omitempty"`
	Confidence  float64                `json:"confidence"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	ExtractedAt time.Time              `json:"extracted_at"`
}

// ResearchResult represents the output of a research task.
type ResearchResult struct {
	Query          string          `json:"query"`
	Keywords       []string        `json:"keywords"`
	Atoms          []KnowledgeAtom `json:"knowledge_atoms"`
	Summary        string          `json:"summary"`
	PagesScraped   int             `json:"pages_scraped"`
	Duration       time.Duration   `json:"duration"`
	FactsGenerated int             `json:"facts_generated"`
}

// ResearcherShard implements the Deep Research ShardAgent (§9.2-9.3).
// It performs web research to build knowledge shards for specialist agents.
type ResearcherShard struct {
	mu sync.RWMutex

	// Identity
	id     string
	config core.ShardConfig
	state  core.ShardState

	// Research-specific
	researchConfig ResearchConfig
	httpClient     *http.Client

	// Components
	kernel       *core.RealKernel
	scanner      *world.Scanner
	llmClient    core.LLMClient
	localDB      *store.LocalStore
	virtualStore *core.VirtualStore

	// Research Toolkit (enhanced tools)
	toolkit *ResearchToolkit

	// JIT Prompt Compiler Integration (Phase 5)
	// The PromptAssembler dynamically generates system prompts from kernel state,
	// injecting context atoms, session state, and specialist knowledge.
	// Stored as interface{} to avoid import cycles - should be *articulation.PromptAssembler.
	// Set via SetPromptAssembler() which accepts interface{}.
	promptAssembler interface{}

	// LLM Rate Limiting - limits concurrent LLM calls to respect API rate limits
	// The API has ~2 calls/second limit, so we serialize LLM calls
	llmSemaphore chan struct{}

	// Tracking
	startTime   time.Time
	stopCh      chan struct{}
	visitedURLs map[string]bool

	// Workspace context for local queries
	workspaceRoot string

	// Autopoiesis - learning and pattern tracking for research quality
	qualityScores     map[string]float64 // Track quality of research results by topic
	sourceReliability map[string]int     // Track which sources produce good results
	sourceFailures    map[string]int     // Track which sources fail or produce poor results
	failedQueries     map[string]int     // Track queries that fail to produce results
	learningStore     core.LearningStore // Optional persistence for learnings

	// E1: Real-time progress callback for atom count reporting
	progressCallback ProgressCallback
	currentAtomCount int // Track atoms collected during research
}

// ProgressCallback is called to report real-time research progress.
type ProgressCallback func(update ProgressUpdate)

// ProgressUpdate contains real-time progress information during research.
type ProgressUpdate struct {
	AtomCount    int     // Current total atoms collected
	CurrentTopic string  // Topic currently being researched
	TopicsTotal  int     // Total topics to research
	TopicsDone   int     // Topics completed
	QualityScore float64 // Current quality score (0-100)
	Message      string  // Human-readable status message
}

// NewResearcherShard creates a new researcher shard with default config.
func NewResearcherShard() *ResearcherShard {
	return NewResearcherShardWithConfig(DefaultResearchConfig())
}

// NewResearcherShardWithConfig creates a researcher shard with custom config.
func NewResearcherShardWithConfig(researchConfig ResearchConfig) *ResearcherShard {
	return &ResearcherShard{
		config:         core.DefaultSpecialistConfig("researcher", ""),
		state:          core.ShardStateIdle,
		researchConfig: researchConfig,
		httpClient: &http.Client{
			Timeout: researchConfig.Timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		kernel:            nil, // Lazy-initialized in Execute() to handle errors
		scanner:           world.NewScanner(),
		toolkit:           NewResearchToolkit(""), // Uses default cache dir
		llmSemaphore:      make(chan struct{}, 1), // Serialize LLM calls (1 at a time)
		stopCh:            make(chan struct{}),
		visitedURLs:       make(map[string]bool),
		qualityScores:     make(map[string]float64),
		sourceReliability: make(map[string]int),
		sourceFailures:    make(map[string]int),
		failedQueries:     make(map[string]int),
	}
}

// SetLLMClient sets the LLM client for intelligent extraction.
func (r *ResearcherShard) SetLLMClient(client core.LLMClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.llmClient = client
}

// SetPromptAssembler sets the PromptAssembler for dynamic prompt generation.
// The assembler parameter should be *articulation.PromptAssembler but is interface{} to avoid import cycles.
// This should be called by the parent kernel/shard manager after initialization.
func (r *ResearcherShard) SetPromptAssembler(assembler interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.promptAssembler = assembler
	if assembler != nil {
		logging.Researcher("PromptAssembler attached to ResearcherShard")
	}
}

// GetPromptAssembler returns the internal PromptAssembler for external JIT configuration.
// Returns interface{} to avoid import cycles - cast to *articulation.PromptAssembler if needed.
//
// Usage:
//   if pa := researcher.GetPromptAssembler(); pa != nil {
//       if assembler, ok := pa.(*articulation.PromptAssembler); ok {
//           assembler.SetJITCompiler(jitCompiler)
//       }
//   }
func (r *ResearcherShard) GetPromptAssembler() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.promptAssembler
}

// SetSessionContext sets the session context (for dream mode, etc.).
func (r *ResearcherShard) SetSessionContext(ctx *core.SessionContext) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config.SessionContext = ctx
}

// SetParentKernel sets the kernel for fact extraction.
// Note: The PromptAssembler should be injected separately via SetPromptAssembler()
// to avoid import cycles.
func (r *ResearcherShard) SetParentKernel(k core.Kernel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rk, ok := k.(*core.RealKernel); ok {
		r.kernel = rk
		logging.Researcher("Kernel attached (PromptAssembler should be set separately)")
	} else {
		panic("ResearcherShard requires *core.RealKernel")
	}
}

// SetLocalDB sets the local database for knowledge persistence.
func (r *ResearcherShard) SetLocalDB(db *store.LocalStore) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.localDB = db
}

// SetVirtualStore sets the virtual store for external system access.
func (r *ResearcherShard) SetVirtualStore(vs *core.VirtualStore) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.virtualStore = vs
}

// SetLearningStore sets the learning store for autopoiesis persistence.
func (r *ResearcherShard) SetLearningStore(ls core.LearningStore) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.learningStore = ls
}

// SetWorkspaceRoot provides the workspace root for local dependency/search queries.
func (r *ResearcherShard) SetWorkspaceRoot(root string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workspaceRoot = root
}

// SetContext7APIKey sets the Context7 API key for LLM-optimized documentation.
func (r *ResearcherShard) SetContext7APIKey(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.toolkit != nil {
		r.toolkit.SetContext7APIKey(key)
	}
}

// llmComplete wraps LLM calls with rate limiting semaphore and Piggyback processing.
// This ensures only one LLM call runs at a time across all goroutines,
// and processes responses through the Piggyback Protocol.
func (r *ResearcherShard) llmComplete(ctx context.Context, prompt string) (string, error) {
	if r.llmClient == nil {
		return "", fmt.Errorf("no LLM client configured")
	}

	// Acquire semaphore (blocks if another LLM call is in progress)
	select {
	case r.llmSemaphore <- struct{}{}:
		// Got the semaphore
	case <-ctx.Done():
		return "", ctx.Err()
	}

	// Release semaphore when done
	defer func() { <-r.llmSemaphore }()

	rawResponse, err := r.llmClient.Complete(ctx, prompt)
	if err != nil {
		return "", err
	}

	// Process through Piggyback Protocol - extract surface response
	processed := articulation.ProcessLLMResponse(rawResponse)
	logging.ResearcherDebug("Piggyback: method=%s, confidence=%.2f", processed.ParseMethod, processed.Confidence)

	return processed.Surface, nil
}

// SetBrowserManager sets the browser session manager for dynamic content fetching.
func (r *ResearcherShard) SetBrowserManager(mgr interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Type assert to browser.SessionManager if available
	// This avoids circular imports - browser package imports mangle, not shards
	if r.toolkit != nil {
		// The toolkit handles the type assertion internally
		// For now, store it as interface{}
	}
}

// GetToolkit returns the research toolkit for external configuration.
func (r *ResearcherShard) GetToolkit() *ResearchToolkit {
	return r.toolkit
}

// EnableJIT enables or disables JIT prompt compilation.
// This is a convenience method that calls EnableJIT on the PromptAssembler if available.
// The caller can also use GetPromptAssembler() for more control.
func (r *ResearcherShard) EnableJIT(enable bool) {
	r.mu.RLock()
	pa := r.promptAssembler
	r.mu.RUnlock()

	if pa != nil {
		// Use type assertion since we can't import articulation
		// The caller should ensure the assembler implements EnableJIT(bool)
		type jitEnabler interface {
			EnableJIT(bool)
		}
		if enabler, ok := pa.(jitEnabler); ok {
			enabler.EnableJIT(enable)
			logging.Researcher("JIT enabled: %v", enable)
		} else {
			logging.Researcher("PromptAssembler does not support EnableJIT")
		}
	}
}

// SetProgressCallback sets a callback function for real-time progress reporting.
// E1 enhancement: Enables live atom count display during research.
func (r *ResearcherShard) SetProgressCallback(callback ProgressCallback) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.progressCallback = callback
}

// reportProgress sends a progress update if a callback is configured.
func (r *ResearcherShard) reportProgress(update ProgressUpdate) {
	r.mu.RLock()
	callback := r.progressCallback
	r.mu.RUnlock()

	if callback != nil {
		callback(update)
	}
}

// incrementAtomCount safely increments the atom counter and reports progress.
func (r *ResearcherShard) incrementAtomCount(count int, topic string, topicsDone, topicsTotal int) {
	r.mu.Lock()
	r.currentAtomCount += count
	currentCount := r.currentAtomCount
	r.mu.Unlock()

	r.reportProgress(ProgressUpdate{
		AtomCount:    currentCount,
		CurrentTopic: topic,
		TopicsTotal:  topicsTotal,
		TopicsDone:   topicsDone,
		Message:      fmt.Sprintf("Researching %s... %d atoms collected", topic, currentCount),
	})
}

// resetAtomCount resets the atom counter for a new research session.
func (r *ResearcherShard) resetAtomCount() {
	r.mu.Lock()
	r.currentAtomCount = 0
	r.mu.Unlock()
}

// GetCurrentAtomCount returns the current atom count.
func (r *ResearcherShard) GetCurrentAtomCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentAtomCount
}

// AdaptiveBatchConfig holds configuration for adaptive batch sizing.
type AdaptiveBatchConfig struct {
	MinBatchSize     int     // Minimum batch size (default: 1)
	MaxBatchSize     int     // Maximum batch size (default: 4)
	DefaultBatchSize int     // Default batch size when no history (default: 2)
	ComplexityWeight float64 // How much topic complexity affects batch size (0-1)
	HistoryWeight    float64 // How much historical performance affects batch size (0-1)
}

// DefaultAdaptiveBatchConfig returns sensible defaults for adaptive batching.
func DefaultAdaptiveBatchConfig() AdaptiveBatchConfig {
	return AdaptiveBatchConfig{
		MinBatchSize:     1,
		MaxBatchSize:     4,
		DefaultBatchSize: 2,
		ComplexityWeight: 0.4,
		HistoryWeight:    0.6,
	}
}

// calculateAdaptiveBatchSize determines the optimal batch size based on:
// 1. Topic complexity (longer/more technical topics = smaller batches)
// 2. Historical API response times (faster responses = larger batches)
// 3. Current quality scores (higher quality = can use larger batches)
func (r *ResearcherShard) calculateAdaptiveBatchSize(topics []string) int {
	config := DefaultAdaptiveBatchConfig()

	if len(topics) == 0 {
		return config.DefaultBatchSize
	}

	// Factor 1: Topic complexity (0.0 = simple, 1.0 = complex)
	complexityScore := r.calculateTopicComplexity(topics)

	// Factor 2: Historical performance (0.0 = slow/failing, 1.0 = fast/reliable)
	performanceScore := r.calculateHistoricalPerformance()

	// Combine factors: Higher complexity -> smaller batches
	// Higher performance -> larger batches
	combinedScore := (1.0-complexityScore)*config.ComplexityWeight +
		performanceScore*config.HistoryWeight

	// Map combined score (0-1) to batch size range
	batchRange := float64(config.MaxBatchSize - config.MinBatchSize)
	batchSize := config.MinBatchSize + int(combinedScore*batchRange)

	// Clamp to valid range
	if batchSize < config.MinBatchSize {
		batchSize = config.MinBatchSize
	}
	if batchSize > config.MaxBatchSize {
		batchSize = config.MaxBatchSize
	}

	// Don't exceed topic count
	if batchSize > len(topics) {
		batchSize = len(topics)
	}

	logging.Researcher("Adaptive batch sizing: complexity=%.2f, performance=%.2f, batch=%d",
		complexityScore, performanceScore, batchSize)

	return batchSize
}

// calculateTopicComplexity analyzes topics to estimate research complexity.
// Returns a score from 0.0 (simple) to 1.0 (complex).
func (r *ResearcherShard) calculateTopicComplexity(topics []string) float64 {
	if len(topics) == 0 {
		return 0.5
	}

	var totalComplexity float64

	// Technical keywords that indicate complex topics
	complexKeywords := []string{
		"advanced", "architecture", "concurrent", "distributed", "optimization",
		"security", "protocol", "algorithm", "internals", "low-level",
		"performance", "memory", "async", "parallel", "threading",
	}

	for _, topic := range topics {
		topicLower := strings.ToLower(topic)
		wordCount := len(strings.Fields(topic))

		// Base complexity from word count (more words = more specific = more complex)
		complexity := float64(wordCount) / 10.0 // Normalize to ~0.4 for 4-word topics
		if complexity > 0.5 {
			complexity = 0.5
		}

		// Add complexity for technical keywords
		for _, kw := range complexKeywords {
			if strings.Contains(topicLower, kw) {
				complexity += 0.1
				break // Only count once per topic
			}
		}

		// Cap individual topic complexity
		if complexity > 1.0 {
			complexity = 1.0
		}

		totalComplexity += complexity
	}

	return totalComplexity / float64(len(topics))
}

// calculateHistoricalPerformance uses tracked metrics to estimate API reliability.
// Returns a score from 0.0 (poor) to 1.0 (excellent).
func (r *ResearcherShard) calculateHistoricalPerformance() float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Start with neutral score
	score := 0.5

	// Factor in quality scores (if available)
	if len(r.qualityScores) > 0 {
		var totalQuality float64
		for _, q := range r.qualityScores {
			totalQuality += q
		}
		avgQuality := totalQuality / float64(len(r.qualityScores))
		// Quality heavily influences performance score
		score = 0.3 + avgQuality*0.5 // Map 0-1 quality to 0.3-0.8
	}

	// Penalize for failed queries
	failureRate := float64(len(r.failedQueries)) / float64(max(len(r.qualityScores), 1)+len(r.failedQueries))
	score -= failureRate * 0.3

	// Bonus for reliable sources
	if len(r.sourceReliability) > 3 {
		score += 0.1
	}

	// Clamp to valid range
	if score < 0.1 {
		score = 0.1
	}
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// ResearchTopicsParallel researches multiple topics in sequential batches.
// Uses batched processing to avoid overwhelming APIs and causing timeouts.
// This is the primary method for building agent knowledge bases efficiently.
func (r *ResearcherShard) ResearchTopicsParallel(ctx context.Context, topics []string) (*ResearchResult, error) {
	r.mu.Lock()
	r.state = core.ShardStateRunning
	r.startTime = time.Now()
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.state = core.ShardStateCompleted
		r.mu.Unlock()
	}()

	result := &ResearchResult{
		Query:    fmt.Sprintf("batch research: %d topics", len(topics)),
		Keywords: topics,
		Atoms:    make([]KnowledgeAtom, 0),
	}

	if len(topics) == 0 {
		return result, nil
	}

	// E1: Reset atom counter for new research session
	r.resetAtomCount()

	// E1: Report initial progress
	r.reportProgress(ProgressUpdate{
		AtomCount:    0,
		CurrentTopic: "starting",
		TopicsTotal:  len(topics),
		TopicsDone:   0,
		Message:      fmt.Sprintf("Starting research for %d topics...", len(topics)),
	})

	// Adaptive batch sizing based on topic complexity and historical performance
	batchSize := r.calculateAdaptiveBatchSize(topics)
	logging.Researcher("Starting batch research for %d topics (adaptive batch size: %d)...", len(topics), batchSize)

	topicsDone := 0

	for i := 0; i < len(topics); i += batchSize {
		// Calculate batch end
		end := i + batchSize
		if end > len(topics) {
			end = len(topics)
		}
		batch := topics[i:end]

		logging.Researcher("Processing batch %d/%d: %v", (i/batchSize)+1, (len(topics)+batchSize-1)/batchSize, batch)

		// Process batch in parallel
		var wg sync.WaitGroup
		var mu sync.Mutex
		var topicsDoneInBatch int

		for _, topic := range batch {
			topic := topic
			wg.Add(1)
			go func() {
				defer wg.Done()

				// E1: Report current topic being researched
				r.reportProgress(ProgressUpdate{
					AtomCount:    r.GetCurrentAtomCount(),
					CurrentTopic: topic,
					TopicsTotal:  len(topics),
					TopicsDone:   topicsDone,
					Message:      fmt.Sprintf("Researching: %s", topic),
				})

				topicResult, err := r.conductWebResearch(ctx, topic, nil, nil) // Don't pass keywords or urls, topic is sufficient
				if err != nil {
					logging.Researcher("Topic '%s' failed: %v", topic, err)
					mu.Lock()
					topicsDoneInBatch++
					mu.Unlock()
					return
				}

				atomCount := len(topicResult.Atoms)

				mu.Lock()
				result.Atoms = append(result.Atoms, topicResult.Atoms...)
				result.PagesScraped += topicResult.PagesScraped
				topicsDoneInBatch++
				currentDone := topicsDone + topicsDoneInBatch
				mu.Unlock()

				// E1: Increment and report atom count
				r.incrementAtomCount(atomCount, topic, currentDone, len(topics))

				logging.Researcher("Topic '%s': %d atoms (total: %d)", topic, atomCount, r.GetCurrentAtomCount())
			}()
		}

		wg.Wait()

		// E1: Update topics done counter after batch completes
		topicsDone += len(batch)

		// Brief pause between batches to let APIs breathe (except for last batch)
		if end < len(topics) {
			time.Sleep(500 * time.Millisecond)
		}
	}

	result.Duration = time.Since(r.startTime)
	result.FactsGenerated = len(result.Atoms)
	result.Summary = fmt.Sprintf("Researched %d topics in batches, gathered %d knowledge atoms in %.2fs",
		len(topics), len(result.Atoms), result.Duration.Seconds())

	// Persist to local DB if available
	if r.localDB != nil {
		r.persistKnowledge(result)
	}

	// Autopoiesis: Track overall batch quality
	batchQuality := 0.0
	if len(topics) > 0 {
		batchQuality = float64(len(result.Atoms)) / float64(len(topics)*10.0) // Expect ~10 atoms per topic
		if batchQuality > 1.0 {
			batchQuality = 1.0
		}
		r.trackResearchQuality(fmt.Sprintf("batch_%d_topics", len(topics)), batchQuality)
	}

	// E1: Report final progress with quality score
	r.reportProgress(ProgressUpdate{
		AtomCount:    r.GetCurrentAtomCount(),
		CurrentTopic: "completed",
		TopicsTotal:  len(topics),
		TopicsDone:   len(topics),
		QualityScore: batchQuality * 100, // Convert to percentage
		Message:      fmt.Sprintf("Research complete: %d atoms collected (quality: %.0f%%)", r.GetCurrentAtomCount(), batchQuality*100),
	})

	return result, nil
}

// ResearchTopicsWithExistingKnowledge performs concept-aware research that avoids
// redundant Context7 API queries by analyzing existing knowledge first.
// This is the preferred method when existing atoms are available.
//
// The method:
// 1. Analyzes existing atoms to determine coverage for each topic
// 2. Skips topics that already have sufficient coverage
// 3. Generates targeted queries for topics with gaps
// 4. Only queries Context7 for genuinely new knowledge
func (r *ResearcherShard) ResearchTopicsWithExistingKnowledge(
	ctx context.Context,
	topics []string,
	existingAtoms []KnowledgeAtom,
) (*ResearchResult, error) {

	r.mu.Lock()
	r.state = core.ShardStateRunning
	r.startTime = time.Now()
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.state = core.ShardStateCompleted
		r.mu.Unlock()
	}()

	result := &ResearchResult{
		Query:    fmt.Sprintf("concept-aware batch research: %d topics", len(topics)),
		Keywords: topics,
		Atoms:    make([]KnowledgeAtom, 0),
	}

	if len(topics) == 0 {
		return result, nil
	}

	// E1: Reset atom counter for new research session
	r.resetAtomCount()

	// Create existing knowledge wrapper
	existing := NewExistingKnowledge(existingAtoms)

	// Analyze coverage and filter topics
	topicsNeedingResearch, skippedTopics, coverageReport := FilterTopicsWithCoverage(topics, existing)

	// E1: Report initial progress with skip info
	r.reportProgress(ProgressUpdate{
		AtomCount:    0,
		CurrentTopic: "analyzing coverage",
		TopicsTotal:  len(topics),
		TopicsDone:   len(skippedTopics),
		Message: fmt.Sprintf("Coverage analysis: %d topics need research, %d skipped (sufficient coverage)",
			len(topicsNeedingResearch), len(skippedTopics)),
	})

	logging.Researcher("[ConceptAware] Starting research: %d topics need research, %d skipped",
		len(topicsNeedingResearch), len(skippedTopics))

	// Log detailed coverage report
	for topic, coverage := range coverageReport {
		if coverage.ShouldSkipAPI {
			logging.Researcher("[ConceptAware] SKIP '%s': atoms=%d, concepts=%d, quality=%.2f",
				topic, coverage.TotalAtoms, len(coverage.CoveredConcepts), coverage.QualityScore)
		} else {
			logging.Researcher("[ConceptAware] RESEARCH '%s' -> '%s': atoms=%d, gaps=%v",
				topic, coverage.RecommendedQuery, coverage.TotalAtoms, coverage.GapsIdentified)
		}
	}

	// If all topics were skipped, return early
	if len(topicsNeedingResearch) == 0 {
		r.reportProgress(ProgressUpdate{
			AtomCount:    0,
			CurrentTopic: "completed",
			TopicsTotal:  len(topics),
			TopicsDone:   len(topics),
			QualityScore: 100,
			Message:      fmt.Sprintf("All %d topics already have sufficient coverage - no API calls needed", len(topics)),
		})
		result.Summary = fmt.Sprintf("All %d topics already have sufficient knowledge coverage", len(topics))
		return result, nil
	}

	// Research only the topics that need it using the targeted queries
	researchResult, err := r.ResearchTopicsParallel(ctx, topicsNeedingResearch)
	if err != nil {
		return result, err
	}

	// Merge results
	result.Atoms = researchResult.Atoms
	result.PagesScraped = researchResult.PagesScraped
	result.Duration = time.Since(r.startTime)
	result.FactsGenerated = len(result.Atoms)
	result.Summary = fmt.Sprintf("Concept-aware research: %d/%d topics researched, %d skipped, %d atoms gathered",
		len(topicsNeedingResearch), len(topics), len(skippedTopics), len(result.Atoms))

	return result, nil
}

// DeepResearch performs comprehensive research using all available tools.
// This includes: known sources, web search, and LLM synthesis.
func (r *ResearcherShard) DeepResearch(ctx context.Context, topic string, keywords []string) (*ResearchResult, error) {
	r.mu.Lock()
	r.state = core.ShardStateRunning
	r.startTime = time.Now()
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.state = core.ShardStateCompleted
		r.mu.Unlock()
	}()

	// Add deep flag to topic to trigger web search
	deepTopic := topic + " (deep)"
	return r.conductWebResearch(ctx, deepTopic, keywords, nil)
}

// GetID returns the shard ID.
func (r *ResearcherShard) GetID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.id
}

// GetState returns the current state.
func (r *ResearcherShard) GetState() core.ShardState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

// GetConfig returns the shard configuration.
func (r *ResearcherShard) GetConfig() core.ShardConfig {
	return r.config
}

// Stop stops the shard.
func (r *ResearcherShard) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	close(r.stopCh)
	r.state = core.ShardStateCompleted
	return nil
}

// =============================================================================
// DREAM MODE (Simulation/Learning)
// =============================================================================

// buildSystemPrompt constructs the complete system prompt for the researcher shard.
// Uses the JIT prompt compiler if available, otherwise falls back to legacy assembly.
func (r *ResearcherShard) buildSystemPrompt(ctx context.Context, task string) (string, error) {
	r.mu.RLock()
	pa := r.promptAssembler
	sessionCtx := r.config.SessionContext
	shardID := r.id
	r.mu.RUnlock()

	// If PromptAssembler is not available, use legacy prompt
	if pa == nil {
		logging.Researcher("[FALLBACK] PromptAssembler not configured, using legacy prompt")
		return r.buildLegacySystemPrompt(task), nil
	}

	// Type assert to actual PromptAssembler type
	assembler, ok := pa.(*articulation.PromptAssembler)
	if !ok {
		logging.Researcher("[FALLBACK] PromptAssembler type mismatch, using legacy prompt")
		return r.buildLegacySystemPrompt(task), nil
	}

	// Check if JIT is ready
	if !assembler.JITReady() {
		logging.Researcher("[FALLBACK] JIT not ready, using legacy prompt")
		return r.buildLegacySystemPrompt(task), nil
	}

	// Build proper PromptContext
	promptCtx := &articulation.PromptContext{
		ShardID:    shardID,
		ShardType:  "researcher",
		SessionCtx: sessionCtx,
	}
	// Ensure we always provide an intent so JIT selectors (intent_verbs) can block
	// non-matching atoms deterministically. When no SessionContext is present (common
	// for user-defined specialists spawned from the TUI), infer the intent from the task.
	if sessionCtx != nil && sessionCtx.UserIntent != nil {
		promptCtx.UserIntent = sessionCtx.UserIntent
	} else {
		promptCtx.UserIntent = core.InferIntentFromTask(task)
	}

	systemPrompt, err := assembler.AssembleSystemPrompt(ctx, promptCtx)
	if err != nil {
		logging.Researcher("[FALLBACK] JIT compilation failed, using legacy: %v", err)
		return r.buildLegacySystemPrompt(task), nil
	}

	if systemPrompt == "" {
		logging.Researcher("[FALLBACK] JIT returned empty prompt, using legacy")
		return r.buildLegacySystemPrompt(task), nil
	}

	logging.Researcher("[JIT] Using JIT-compiled system prompt (%d bytes)", len(systemPrompt))
	return systemPrompt, nil
}

// buildLegacySystemPrompt returns the fallback system prompt without JIT.
func (r *ResearcherShard) buildLegacySystemPrompt(task string) string {
	var sb strings.Builder

	// Base researcher identity
	sb.WriteString("You are the ResearcherShard of codeNERD, a deep research specialist.\n\n")
	sb.WriteString("## Core Responsibilities\n")
	sb.WriteString("1. Gather knowledge from documentation, web sources, and codebases\n")
	sb.WriteString("2. Extract knowledge atoms for specialist agents\n")
	sb.WriteString("3. Provide domain-specific guidance\n")
	sb.WriteString("4. Build comprehensive knowledge bases\n\n")

	sb.WriteString("## Available Tools\n")
	sb.WriteString("- Context7 API: LLM-optimized documentation\n")
	sb.WriteString("- GitHub repository analysis\n")
	sb.WriteString("- Web search and scraping\n")
	sb.WriteString("- Local codebase analysis\n")
	sb.WriteString("- LLM knowledge synthesis\n\n")

	sb.WriteString("## Research Strategies\n")
	sb.WriteString("1. Primary: Context7 for library/framework documentation\n")
	sb.WriteString("2. Fallback: GitHub READMEs and docs\n")
	sb.WriteString("3. Extended: Web search for unknown topics\n")
	sb.WriteString("4. Synthesis: LLM-based knowledge generation\n\n")

	sb.WriteString("## Absolute Rules\n")
	sb.WriteString("1. NEVER invent information - cite sources when possible\n")
	sb.WriteString("2. NEVER provide outdated information - verify currency\n")
	sb.WriteString("3. ALWAYS synthesize knowledge into actionable insights\n")
	sb.WriteString("4. ALWAYS structure findings for easy consumption\n\n")

	// Add session context if available
	sessionCtx := r.buildSessionContextPrompt()
	if sessionCtx != "" {
		sb.WriteString(sessionCtx)
	}

	return sb.String()
}

// describeDreamPlan returns a description of what the researcher would do WITHOUT executing.
func (r *ResearcherShard) describeDreamPlan(ctx context.Context, task string) (string, error) {
	logging.Researcher("DREAM MODE - describing plan without execution")

	if r.llmClient == nil {
		return "ResearcherShard would gather knowledge and analyze sources, but no LLM client available for dream description.", nil
	}

	// Build system prompt with JIT if available
	systemPrompt, err := r.buildSystemPrompt(ctx, task)
	if err != nil {
		logging.Researcher("Failed to build system prompt: %v, using minimal prompt", err)
		systemPrompt = "You are a research agent in DREAM MODE."
	}

	// Append dream-specific instructions
	prompt := fmt.Sprintf(`%s

DREAM MODE ACTIVE: Describe what you WOULD do for this task WITHOUT actually doing it.

Task: %s

Provide a structured analysis:
1. **Understanding**: What kind of research is being asked?
2. **Sources**: What sources would I consult? (web, docs, codebase, APIs)
3. **Research Strategy**: What approach would I take?
4. **Tools Needed**: What research tools would I use?
5. **Expected Findings**: What knowledge might I gather?
6. **Questions**: What would I need clarified?

Remember: This is a simulation. Describe the plan, don't execute it.`, systemPrompt, task)

	rawResponse, err := r.llmClient.Complete(ctx, prompt)
	if err != nil {
		return fmt.Sprintf("ResearcherShard dream analysis failed: %v", err), nil
	}

	// Process through Piggyback Protocol - extract surface response
	processed := articulation.ProcessLLMResponse(rawResponse)
	return processed.Surface, nil
}

// Execute performs the research task.
// Task format: "topic:TOPIC keywords:KW1,KW2,KW3" or just "TOPIC"
func (r *ResearcherShard) Execute(ctx context.Context, task string) (string, error) {
	r.mu.Lock()
	r.state = core.ShardStateRunning
	r.startTime = time.Now()
	r.visitedURLs = make(map[string]bool)
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.state = core.ShardStateCompleted
		r.mu.Unlock()
	}()

	// Initialize kernel if not set
	if r.kernel == nil {
		kernel, err := core.NewRealKernel()
		if err != nil {
			return "", fmt.Errorf("failed to create kernel: %w", err)
		}
		r.kernel = kernel
	}

	// DREAM MODE: Only describe what we would do, don't execute
	if r.config.SessionContext != nil && r.config.SessionContext.DreamMode {
		return r.describeDreamPlan(ctx, task)
	}

	// Parse task
	topic, keywords, urls := r.parseTask(task)

	logging.Researcher("Starting Deep Research")
	logging.Researcher("  Topic: %s", topic)
	logging.Researcher("  Keywords: %v", keywords)
	if len(urls) > 0 {
		logging.Researcher("  Explicit URLs: %v", urls)
	}

	// Determine research mode
	var result *ResearchResult
	var err error

	if r.isCodebaseTask(task) {
		// Mode 1: Codebase Analysis (for nerd init)
		result, err = r.analyzeCodebase(ctx, topic)
	} else {
		// Mode 2: Web Research (for knowledge building)
		result, err = r.conductWebResearch(ctx, topic, keywords, urls)
	}

	if err != nil {
		return "", fmt.Errorf("research failed: %w", err)
	}

	// Persist knowledge atoms to local DB
	if r.localDB != nil {
		r.persistKnowledge(result)
	}

	// Generate facts for the kernel
	facts := r.generateFacts(result)
	if err := r.kernel.LoadFacts(facts); err != nil {
		logging.Researcher("Warning: failed to load facts: %v", err)
	}

	// Build summary
	summary := r.buildSummary(result)

	return summary, nil
}

// parseTask extracts topic, keywords, and URLs from the task string.
func (r *ResearcherShard) parseTask(task string) (string, []string, []string) {
	// Format: "topic:TOPIC keywords:KW1,KW2" or just "TOPIC"
	topic := task
	var keywords []string
	var urls []string

	// Extract URLs
	// Match http/https and allow typical URL chars, stopping at space, comma, or end of line
	// We strip trailing punctuation (.,;) manually after extraction if needed, but a stricter regex is better.
	urlRegex := regexp.MustCompile(`https?://[^,\s]+[^.,;)\s]`)
	rawUrls := urlRegex.FindAllString(task, -1)

	for _, u := range rawUrls {
		// Clean trailing punctuation often caught in NLP text
		u = strings.TrimRight(u, ".,;)")
		urls = append(urls, u)
	}

	// Clean task string for topic extraction
	cleanTask := urlRegex.ReplaceAllString(task, "")
	cleanTask = strings.TrimSpace(strings.ReplaceAll(cleanTask, "  ", " "))

	// Extract keywords first and strip them from the task to avoid contaminating topic parsing
	if strings.Contains(strings.ToLower(cleanTask), "keywords:") {
		re := regexp.MustCompile(`(?i)keywords:\s*([^\n]+)`)
		if matches := re.FindStringSubmatch(cleanTask); len(matches) > 1 {
			keywordBlock := strings.TrimSpace(matches[1])
			keywordParts := strings.FieldsFunc(keywordBlock, func(r rune) bool {
				return r == ',' || r == ';'
			})
			for _, kw := range keywordParts {
				trimmed := strings.TrimSpace(kw)
				if trimmed != "" {
					keywords = append(keywords, trimmed)
				}
			}
			cleanTask = strings.TrimSpace(strings.Replace(cleanTask, matches[0], "", 1))
		}
	}

	// Extract topic (allow multi-word up to end of string)
	if strings.Contains(strings.ToLower(cleanTask), "topic:") {
		re := regexp.MustCompile(`(?i)topic:\s*([^\n]+)`)
		if matches := re.FindStringSubmatch(cleanTask); len(matches) > 1 {
			topic = strings.TrimSpace(matches[1])
		}
	} else {
		topic = strings.TrimSpace(cleanTask)
	}

	// If no explicit keywords, extract from topic
	if len(keywords) == 0 {
		keywords = strings.Fields(strings.ToLower(topic))
	}

	return topic, keywords, urls
}

// isCodebaseTask checks if this is a codebase analysis task.
func (r *ResearcherShard) isCodebaseTask(task string) bool {
	codebaseKeywords := []string{
		"init", "initialize", "codebase", "project", "analyze",
		"init", "initialize", "codebase", "project", "analyze",
		"scan", "index", "inventory", "workspace", "structure",
		"directory", "directories", "folder", "folders",
	}
	lower := strings.ToLower(task)
	for _, kw := range codebaseKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// GetKernel returns the researcher's kernel for fact extraction.
func (r *ResearcherShard) GetKernel() *core.RealKernel {
	return r.kernel
}

// buildSummary creates the final output summary.
func (r *ResearcherShard) buildSummary(result *ResearchResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Research Complete: %s\n\n", result.Query))
	sb.WriteString(fmt.Sprintf("**Duration:** %.2fs\n", result.Duration.Seconds()))
	sb.WriteString(fmt.Sprintf("**Sources Analyzed:** %d\n", result.PagesScraped))
	sb.WriteString(fmt.Sprintf("**Knowledge Atoms Generated:** %d\n\n", result.FactsGenerated))

	if result.Summary != "" {
		sb.WriteString("### Summary\n")
		sb.WriteString(result.Summary)
		sb.WriteString("\n\n")
	}

	if len(result.Atoms) > 0 {
		sb.WriteString("### Key Findings\n")
		for i, atom := range result.Atoms {
			if i >= 5 {
				sb.WriteString(fmt.Sprintf("... and %d more atoms\n", len(result.Atoms)-5))
				break
			}
			sb.WriteString(fmt.Sprintf("- **%s**: %s (%.0f%% confidence)\n",
				atom.Concept, atom.Title, atom.Confidence*100))
		}
	}

	return sb.String()
}

// trackResearchQuality tracks the quality of research results for a given topic.
// Quality scores are used to learn which topics the researcher performs well on
// and which might need different strategies.
func (r *ResearcherShard) trackResearchQuality(topic string, quality float64) {
	logging.ResearcherDebug("Tracking research quality for '%s': %.2f", topic, quality)
	r.mu.Lock()
	defer r.mu.Unlock()

	// Update rolling average quality for this topic
	if existing, ok := r.qualityScores[topic]; ok {
		// Blend with existing score (70% old, 30% new)
		newScore := existing*0.7 + quality*0.3
		logging.ResearcherDebug("Updated quality score for '%s': %.2f -> %.2f", topic, existing, newScore)
		r.qualityScores[topic] = newScore
	} else {
		r.qualityScores[topic] = quality
	}

	// Persist to learning store if available
	if r.learningStore != nil && quality >= 0.7 {
		// High quality research - save as preferred topic
		logging.ResearcherDebug("Persisting high-quality topic to learning store: %s", topic)
		_ = r.learningStore.Save("researcher", "high_quality_topic", []any{topic, quality}, "")
	} else if r.learningStore != nil && quality < 0.4 {
		// Low quality - save as difficult topic for future improvement
		logging.ResearcherDebug("Persisting difficult topic to learning store: %s", topic)
		_ = r.learningStore.Save("researcher", "difficult_topic", []any{topic, quality}, "")
	}
}

// trackResearchQualityFromResult calculates and tracks quality based on research results.
// Quality is determined by: number of atoms, average confidence, and diversity of sources.
func (r *ResearcherShard) trackResearchQualityFromResult(topic string, result *ResearchResult) {
	if result == nil {
		return
	}

	quality := 0.0

	// Factor 1: Atom count (more atoms = better, up to 0.4)
	atomScore := float64(len(result.Atoms)) / 20.0 // 20 atoms = max score
	if atomScore > 0.4 {
		atomScore = 0.4
	}
	quality += atomScore

	// Factor 2: Average confidence (up to 0.4)
	if len(result.Atoms) > 0 {
		totalConfidence := 0.0
		for _, atom := range result.Atoms {
			totalConfidence += atom.Confidence
		}
		avgConfidence := totalConfidence / float64(len(result.Atoms))
		quality += avgConfidence * 0.4
	}

	// Factor 3: Source diversity (up to 0.2)
	uniqueSources := make(map[string]bool)
	for _, atom := range result.Atoms {
		if atom.SourceURL != "" {
			uniqueSources[atom.SourceURL] = true
		}
	}
	sourceScore := float64(len(uniqueSources)) / 10.0 // 10 sources = max score
	if sourceScore > 0.2 {
		sourceScore = 0.2
	}
	quality += sourceScore

	// Track the calculated quality
	r.trackResearchQuality(topic, quality)
}

// trackSourceSuccess tracks successful research from a source URL.
// Sources that consistently produce good results are prioritized in future research.
func (r *ResearcherShard) trackSourceSuccess(sourceURL string) {
	logging.ResearcherDebug("Tracking source success: %s", sourceURL)
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sourceReliability[sourceURL]++

	// Reset failure count on success
	if _, exists := r.sourceFailures[sourceURL]; exists {
		delete(r.sourceFailures, sourceURL)
	}

	// Persist highly reliable sources to learning store
	if r.learningStore != nil && r.sourceReliability[sourceURL] >= 3 {
		logging.ResearcherDebug("Persisting reliable source: %s (count: %d)", sourceURL, r.sourceReliability[sourceURL])
		_ = r.learningStore.Save("researcher", "reliable_source", []any{sourceURL, r.sourceReliability[sourceURL]}, "")
	}
}

// trackSourceFailure tracks failed or poor quality research from a source URL.
// Sources that consistently fail are deprioritized or avoided in future research.
func (r *ResearcherShard) trackSourceFailure(sourceURL string) {
	logging.ResearcherDebug("Tracking source failure: %s", sourceURL)
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sourceFailures[sourceURL]++

	// Persist problematic sources to learning store
	if r.learningStore != nil && r.sourceFailures[sourceURL] >= 2 {
		logging.ResearcherDebug("Persisting unreliable source: %s (failures: %d)", sourceURL, r.sourceFailures[sourceURL])
		_ = r.learningStore.Save("researcher", "unreliable_source", []any{sourceURL, r.sourceFailures[sourceURL]}, "")
	}
}

// trackQueryFailure tracks queries that fail to produce useful results.
// This helps identify gaps in research capability and improve query formulation.
func (r *ResearcherShard) trackQueryFailure(query string) {
	logging.ResearcherDebug("Tracking query failure: %s", query)
	r.mu.Lock()
	defer r.mu.Unlock()

	r.failedQueries[query]++

	// Persist persistent failures to learning store
	if r.learningStore != nil && r.failedQueries[query] >= 2 {
		logging.ResearcherDebug("Persisting failed query: %s (failures: %d)", query, r.failedQueries[query])
		_ = r.learningStore.Save("researcher", "failed_query", []any{query, r.failedQueries[query]}, "")
	}
}

// GetQualityScore returns the quality score for a topic if available.
func (r *ResearcherShard) GetQualityScore(topic string) (float64, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	score, ok := r.qualityScores[topic]
	return score, ok
}

// GetSourceReliability returns the reliability count for a source.
func (r *ResearcherShard) GetSourceReliability(sourceURL string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sourceReliability[sourceURL]
}

// GetLearningStats returns statistics about the researcher's learning.
func (r *ResearcherShard) GetLearningStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["quality_scores_count"] = len(r.qualityScores)
	stats["reliable_sources_count"] = len(r.sourceReliability)
	stats["failed_sources_count"] = len(r.sourceFailures)
	stats["failed_queries_count"] = len(r.failedQueries)

	// Calculate average quality
	if len(r.qualityScores) > 0 {
		sum := 0.0
		for _, score := range r.qualityScores {
			sum += score
		}
		stats["avg_quality"] = sum / float64(len(r.qualityScores))
	}

	return stats
}

// =============================================================================
// SESSION CONTEXT (BLACKBOARD PATTERN)
// =============================================================================

// buildSessionContextPrompt builds comprehensive session context for cross-shard awareness (Blackboard Pattern).
// This injects all available context into the LLM prompt to enable informed research.
func (r *ResearcherShard) buildSessionContextPrompt() string {
	if r.config.SessionContext == nil {
		return ""
	}

	var sb strings.Builder
	ctx := r.config.SessionContext

	// ==========================================================================
	// USER INTENT (What to Research)
	// ==========================================================================
	if ctx.UserIntent != nil {
		sb.WriteString("\nRESEARCH INTENT:\n")
		if ctx.UserIntent.Category != "" {
			sb.WriteString(fmt.Sprintf("  Category: %s\n", ctx.UserIntent.Category))
		}
		if ctx.UserIntent.Verb != "" {
			sb.WriteString(fmt.Sprintf("  Action: %s\n", ctx.UserIntent.Verb))
		}
		if ctx.UserIntent.Target != "" {
			sb.WriteString(fmt.Sprintf("  Target: %s\n", ctx.UserIntent.Target))
		}
	}

	// ==========================================================================
	// PRIOR SHARD OUTPUTS (Context for Research)
	// ==========================================================================
	if len(ctx.PriorShardOutputs) > 0 {
		sb.WriteString("\nPRIOR SHARD CONTEXT:\n")
		for _, output := range ctx.PriorShardOutputs {
			status := "SUCCESS"
			if !output.Success {
				status = "FAILED"
			}
			sb.WriteString(fmt.Sprintf("  [%s] %s: %s - %s\n",
				output.ShardType, status, output.Task, output.Summary))
		}
	}

	// ==========================================================================
	// CAMPAIGN CONTEXT (Research Scope)
	// ==========================================================================
	if ctx.CampaignActive {
		sb.WriteString("\nCAMPAIGN CONTEXT:\n")
		if ctx.CampaignPhase != "" {
			sb.WriteString(fmt.Sprintf("  Phase: %s\n", ctx.CampaignPhase))
		}
		if ctx.CampaignGoal != "" {
			sb.WriteString(fmt.Sprintf("  Goal: %s\n", ctx.CampaignGoal))
		}
		if len(ctx.LinkedRequirements) > 0 {
			sb.WriteString("  Research for requirements: ")
			sb.WriteString(strings.Join(ctx.LinkedRequirements, ", "))
			sb.WriteString("\n")
		}
	}

	// ==========================================================================
	// SYMBOL CONTEXT (What Exists in Codebase)
	// ==========================================================================
	if len(ctx.SymbolContext) > 0 {
		sb.WriteString("\nCODEBASE SYMBOLS (for research context):\n")
		for _, sym := range ctx.SymbolContext {
			sb.WriteString(fmt.Sprintf("  - %s\n", sym))
		}
	}

	// ==========================================================================
	// RECENT SESSION ACTIONS
	// ==========================================================================
	if len(ctx.RecentActions) > 0 {
		sb.WriteString("\nSESSION ACTIONS:\n")
		for _, action := range ctx.RecentActions {
			sb.WriteString(fmt.Sprintf("  - %s\n", action))
		}
	}

	// ==========================================================================
	// EXISTING DOMAIN KNOWLEDGE (Build Upon)
	// ==========================================================================
	if len(ctx.KnowledgeAtoms) > 0 {
		sb.WriteString("\nEXISTING KNOWLEDGE (build upon):\n")
		for _, atom := range ctx.KnowledgeAtoms {
			sb.WriteString(fmt.Sprintf("  - %s\n", atom))
		}
	}
	if len(ctx.SpecialistHints) > 0 {
		for _, hint := range ctx.SpecialistHints {
			sb.WriteString(fmt.Sprintf("  - HINT: %s\n", hint))
		}
	}

	// ==========================================================================
	// KERNEL-DERIVED CONTEXT (Spreading Activation)
	// ==========================================================================
	// Query the Mangle kernel for injectable context atoms derived from
	// spreading activation rules (injectable_context, specialist_knowledge).
	if r.kernel != nil {
		kernelContext, err := articulation.GetKernelContext(r.kernel, r.id)
		if err != nil {
			logging.ResearcherDebug("Failed to get kernel context: %v", err)
		} else if kernelContext != "" {
			sb.WriteString("\n")
			sb.WriteString(kernelContext)
		}
	}

	// ==========================================================================
	// COMPRESSED SESSION HISTORY
	// ==========================================================================
	if ctx.CompressedHistory != "" && len(ctx.CompressedHistory) < 1500 {
		sb.WriteString("\nSESSION HISTORY (compressed):\n")
		sb.WriteString(ctx.CompressedHistory)
		sb.WriteString("\n")
	}

	return sb.String()
}

// IngestDocumentation scanning and ingesting project documentation.
// It specifically looks for Docs/ folders, key markdown files, and heuristically scans other files for "Ground Truth".
func (r *ResearcherShard) IngestDocumentation(ctx context.Context, workspace string) ([]KnowledgeAtom, error) {
	r.mu.Lock()
	r.state = core.ShardStateRunning
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.state = core.ShardStateCompleted
		r.mu.Unlock()
	}()

	logging.Researcher("Starting Documentation Ingestion in: %s", workspace)

	var atoms []KnowledgeAtom
	var mu sync.Mutex

	// 1. Identify Documentation Files
	// We look for:
	// - Any file in a directory named "Docs", "docs", "documentation", "spec", "specs", "planning", "design", "research", "analysis", "audits", "results", "surveys"
	// - Root level markdown files (handled by analyzeCodebase mostly, but good to double check specific ones like ROADMAP.md)
	// - Specific goal-oriented files anywhere: "architecture.md", "roadmap.md"
	// - HEURISTIC: Read header of random markdown files to see if they contain signal keywords.

	docFiles := make([]string, 0)
	targetDirs := []string{
		"docs", "Docs", "documentation", "spec", "specs", "planning", "design",
		"research", "analysis", "audits", "tests", "results", "surveys", "data",
	}
	targetFiles := []string{
		"roadmap.md", "architecture.md", "goals.md", "vision.md", "changelog.md",
		"strategy.md", "hypothesis.md",
	}

	// Signal keywords for heuristic scanning
	signalKeywords := []string{
		"Specification", "Analysis", "Audit", "Vision", "Strategy", "Roadmap",
		"Architecture", "Hypothesis", "Conclusion", "Test Result", "Experiment",
		"Objective", "Goals", "Overview",
	}

	err := filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if info.IsDir() {
			// Skip hidden directories (except .nerd if we decided to document there, but usually skip)
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}

		// Check extensions
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" && ext != ".txt" && ext != ".rst" && ext != ".pdf" {
			return nil
		}

		// Check if in target directory
		relPath, _ := filepath.Rel(workspace, path)
		parts := strings.Split(relPath, string(os.PathSeparator))

		isTarget := false

		// check if any part of the path is a target dir
		for _, part := range parts {
			for _, target := range targetDirs {
				if strings.EqualFold(part, target) {
					isTarget = true
					break
				}
			}
			if isTarget {
				break
			}
		}

		// check if filename is a target file
		if !isTarget {
			name := strings.ToLower(info.Name())
			for _, target := range targetFiles {
				if strings.Contains(name, target) {
					isTarget = true
					break
				}
			}
		}

		// HEURISTIC CONTENT SNIFFING
		// If not a known target, perform a quick "sniff" of the file content
		if !isTarget && ext == ".md" {
			file, err := os.Open(path)
			if err == nil {
				// Read first 1KB
				buf := make([]byte, 1024)
				n, _ := file.Read(buf)
				file.Close()

				if n > 0 {
					header := string(buf[:n])
					// Check for strong signals in the first 1KB (likely headers)
					for _, signal := range signalKeywords {
						// Simple check: Is the signal word present in a header or start of line?
						// e.g. "# Project Vision" or "Vision:"
						if strings.Contains(header, "# "+signal) || strings.Contains(header, signal+":") {
							isTarget = true
							// fmt.Printf("[Researcher] Heuristic match: %s (found '%s')\n", filepath.Base(path), signal)
							break
						}
					}
				}
			}
		}

		if isTarget {
			docFiles = append(docFiles, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk workspace for docs: %w", err)
	}

	if len(docFiles) == 0 {
		logging.Researcher("No dedicated documentation files found.")
		return atoms, nil
	}

	logging.Researcher("Found %d documentation files. Processing...", len(docFiles))

	// 2. Process Files in Batches (Robustness)
	// We use the existing llmSemaphore if we need to summarize, but for pure reading we can go faster.
	// However, if the files are large, we might want to chunk them.
	// For "Ground Truth", we want to read the whole thing if possible, or chunk it intelligently.

	// We'll process sequentially to be safe on memory and CPU, or with low concurrency.

	for i, file := range docFiles {
		select {
		case <-ctx.Done():
			return atoms, ctx.Err()
		default:
		}

		logging.Researcher("   [%d/%d] Ingesting: %s", i+1, len(docFiles), filepath.Base(file))

		content, err := os.ReadFile(file)
		if err != nil {
			logging.Researcher("   Error reading %s: %v", file, err)
			continue
		}

		fileContent := string(content)
		if len(fileContent) == 0 {
			continue
		}

		// Auto-generate title from filename or first header
		title := filepath.Base(file)
		lines := strings.Split(fileContent, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "# ") {
				title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
				break
			}
		}

		// Check complexity - if too large, might need summary.
		// For now, we assume we simply ingest unless it's huge.
		// If > 20KB, we might want to chunk it.

		// For robustness with LLM limits, we'll create a "raw" atom first.
		// If we had a chunker, we'd use it here.
		// For this implementation, we will treat it as a single high-value atom.

		relPath, _ := filepath.Rel(workspace, file)

		mu.Lock()
		atoms = append(atoms, KnowledgeAtom{
			SourceURL:   "file://" + relPath,
			Title:       title,
			Content:     fileContent, // Embedder will handle chunking typically
			Concept:     "project_truth",
			Confidence:  1.0, // High confidence for explicit docs
			ExtractedAt: time.Now(),
			Metadata: map[string]interface{}{
				"is_doc": true,
				"path":   relPath,
				"type":   "documentation",
			},
		})
		mu.Unlock()

		// Brief pause to allow system to breathe if processing many files
		if i%5 == 0 && i > 0 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return atoms, nil
}
