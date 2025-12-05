// Package researcher implements the Deep Research ShardAgent (Type B: Persistent Specialist).
// This file contains the core ResearcherShard struct and lifecycle methods.
package researcher

import (
	"codenerd/internal/core"
	"codenerd/internal/perception"
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
	llmClient perception.LLMClient
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
		kernel:       core.NewRealKernel(),
		scanner:      world.NewScanner(),
		toolkit:      NewResearchToolkit(""), // Uses default cache dir
		llmSemaphore: make(chan struct{}, 1), // Serialize LLM calls (1 at a time)
		stopCh:       make(chan struct{}),
		visitedURLs:  make(map[string]bool),
	}
}

// SetLLMClient sets the LLM client for intelligent extraction.
func (r *ResearcherShard) SetLLMClient(client perception.LLMClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.llmClient = client
}

// SetLocalDB sets the local database for knowledge persistence.
func (r *ResearcherShard) SetLocalDB(db *store.LocalStore) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.localDB = db
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

				topicResult, err := r.conductWebResearch(ctx, topic, nil) // Don't pass keywords, topic is sufficient
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
	return r.conductWebResearch(ctx, deepTopic, keywords)
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
	topic, keywords := r.parseTask(task)

	fmt.Printf("[Researcher] Starting Deep Research\n")
	fmt.Printf("  Topic: %s\n", topic)
	fmt.Printf("  Keywords: %v\n", keywords)

	// Determine research mode
	var result *ResearchResult
	var err error

	if r.isCodebaseTask(task) {
		// Mode 1: Codebase Analysis (for nerd init)
		result, err = r.analyzeCodebase(ctx, topic)
	} else {
		// Mode 2: Web Research (for knowledge building)
		result, err = r.conductWebResearch(ctx, topic, keywords)
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

// parseTask extracts topic and keywords from the task string.
func (r *ResearcherShard) parseTask(task string) (string, []string) {
	// Format: "topic:TOPIC keywords:KW1,KW2" or just "TOPIC"
	topic := task
	var keywords []string

	if strings.Contains(task, "topic:") {
		re := regexp.MustCompile(`topic:([^\s]+)`)
		if matches := re.FindStringSubmatch(task); len(matches) > 1 {
			topic = matches[1]
		}
	}

	if strings.Contains(task, "keywords:") {
		re := regexp.MustCompile(`keywords:([^\s]+)`)
		if matches := re.FindStringSubmatch(task); len(matches) > 1 {
			keywords = strings.Split(matches[1], ",")
		}
	}

	// If no explicit keywords, extract from topic
	if len(keywords) == 0 {
		keywords = strings.Fields(strings.ToLower(topic))
	}

	return topic, keywords
}

// isCodebaseTask checks if this is a codebase analysis task.
func (r *ResearcherShard) isCodebaseTask(task string) bool {
	codebaseKeywords := []string{
		"init", "initialize", "codebase", "project", "analyze",
		"scan", "index", "inventory",
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
