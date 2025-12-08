// Package researcher implements the Deep Research ShardAgent (Type B: Persistent Specialist).
// This file contains the core ResearcherShard struct and lifecycle methods.
package researcher

import (
	"codenerd/internal/core"
	"codenerd/internal/store"
	"codenerd/internal/world"
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
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
	kernel    *core.RealKernel
	scanner   *world.Scanner
	llmClient core.LLMClient
	localDB   *store.LocalStore

	// Research Toolkit (enhanced tools)
	toolkit *ResearchToolkit

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
		kernel:            core.NewRealKernel(),
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

// SetParentKernel sets the kernel for fact extraction.
func (r *ResearcherShard) SetParentKernel(k core.Kernel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rk, ok := k.(*core.RealKernel); ok {
		r.kernel = rk
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

// llmComplete wraps LLM calls with rate limiting semaphore.
// This ensures only one LLM call runs at a time across all goroutines.
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

	return r.llmClient.Complete(ctx, prompt)
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

	// Process topics in sequential batches of 2 to avoid API overload
	batchSize := 2
	fmt.Printf("[Researcher] Starting batch research for %d topics (batch size: %d)...\n", len(topics), batchSize)

	for i := 0; i < len(topics); i += batchSize {
		// Calculate batch end
		end := i + batchSize
		if end > len(topics) {
			end = len(topics)
		}
		batch := topics[i:end]

		fmt.Printf("[Researcher] Processing batch %d/%d: %v\n", (i/batchSize)+1, (len(topics)+batchSize-1)/batchSize, batch)

		// Process batch in parallel
		var wg sync.WaitGroup
		var mu sync.Mutex

		for _, topic := range batch {
			topic := topic
			wg.Add(1)
			go func() {
				defer wg.Done()

				topicResult, err := r.conductWebResearch(ctx, topic, nil, nil) // Don't pass keywords or urls, topic is sufficient
				if err != nil {
					fmt.Printf("[Researcher] Topic '%s' failed: %v\n", topic, err)
					return
				}

				mu.Lock()
				result.Atoms = append(result.Atoms, topicResult.Atoms...)
				result.PagesScraped += topicResult.PagesScraped
				mu.Unlock()

				fmt.Printf("[Researcher] Topic '%s': %d atoms\n", topic, len(topicResult.Atoms))
			}()
		}

		wg.Wait()

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
	if len(topics) > 0 {
		batchQuality := float64(len(result.Atoms)) / float64(len(topics)*10.0) // Expect ~10 atoms per topic
		if batchQuality > 1.0 {
			batchQuality = 1.0
		}
		r.trackResearchQuality(fmt.Sprintf("batch_%d_topics", len(topics)), batchQuality)
	}

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

	// Parse task
	topic, keywords, urls := r.parseTask(task)

	fmt.Printf("[Researcher] Starting Deep Research\n")
	fmt.Printf("  Topic: %s\n", topic)
	fmt.Printf("  Keywords: %v\n", keywords)
	if len(urls) > 0 {
		fmt.Printf("  Explicit URLs: %v\n", urls)
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
		fmt.Printf("[Researcher] Warning: failed to load facts: %v\n", err)
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
	r.mu.Lock()
	defer r.mu.Unlock()

	// Update rolling average quality for this topic
	if existing, ok := r.qualityScores[topic]; ok {
		// Blend with existing score (70% old, 30% new)
		r.qualityScores[topic] = existing*0.7 + quality*0.3
	} else {
		r.qualityScores[topic] = quality
	}

	// Persist to learning store if available
	if r.learningStore != nil && quality >= 0.7 {
		// High quality research - save as preferred topic
		_ = r.learningStore.Save("researcher", "high_quality_topic", []any{topic, quality}, "")
	} else if r.learningStore != nil && quality < 0.4 {
		// Low quality - save as difficult topic for future improvement
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
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sourceReliability[sourceURL]++

	// Reset failure count on success
	if _, exists := r.sourceFailures[sourceURL]; exists {
		delete(r.sourceFailures, sourceURL)
	}

	// Persist highly reliable sources to learning store
	if r.learningStore != nil && r.sourceReliability[sourceURL] >= 3 {
		_ = r.learningStore.Save("researcher", "reliable_source", []any{sourceURL, r.sourceReliability[sourceURL]}, "")
	}
}

// trackSourceFailure tracks failed or poor quality research from a source URL.
// Sources that consistently fail are deprioritized or avoided in future research.
func (r *ResearcherShard) trackSourceFailure(sourceURL string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sourceFailures[sourceURL]++

	// Persist problematic sources to learning store
	if r.learningStore != nil && r.sourceFailures[sourceURL] >= 2 {
		_ = r.learningStore.Save("researcher", "unreliable_source", []any{sourceURL, r.sourceFailures[sourceURL]}, "")
	}
}

// trackQueryFailure tracks queries that fail to produce useful results.
// This helps identify gaps in research capability and improve query formulation.
func (r *ResearcherShard) trackQueryFailure(query string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.failedQueries[query]++

	// Persist persistent failures to learning store
	if r.learningStore != nil && r.failedQueries[query] >= 2 {
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
	// COMPRESSED SESSION HISTORY
	// ==========================================================================
	if ctx.CompressedHistory != "" && len(ctx.CompressedHistory) < 1500 {
		sb.WriteString("\nSESSION HISTORY (compressed):\n")
		sb.WriteString(ctx.CompressedHistory)
		sb.WriteString("\n")
	}

	return sb.String()
}
