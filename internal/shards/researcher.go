// Package shards implements specialized ShardAgent types for the Cortex 1.5.0 architecture.
// This file implements the Deep Research ShardAgent (Type B: Persistent Specialist).
package shards

import (
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/store"
	"codenerd/internal/world"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
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

// ============================================================================
// MODE 1: CODEBASE ANALYSIS (for nerd init)
// ============================================================================

// analyzeCodebase performs deep analysis of the local codebase.
func (r *ResearcherShard) analyzeCodebase(ctx context.Context, workspace string) (*ResearchResult, error) {
	if workspace == "" || workspace == "." {
		workspace, _ = os.Getwd()
	}

	fmt.Printf("[Researcher] Analyzing codebase at: %s\n", workspace)

	result := &ResearchResult{
		Query:    "codebase_analysis:" + workspace,
		Keywords: []string{"codebase", "structure", "dependencies"},
		Atoms:    make([]KnowledgeAtom, 0),
	}

	// 1. Scan file topology
	fileFacts, err := r.scanner.ScanWorkspace(workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to scan workspace: %w", err)
	}
	result.PagesScraped = len(fileFacts)

	// 2. Detect project type
	projectType := r.detectProjectType(workspace)
	result.Atoms = append(result.Atoms, KnowledgeAtom{
		SourceURL:   workspace,
		Title:       "Project Type Detection",
		Content:     fmt.Sprintf("Detected project type: %s", projectType.Language),
		Concept:     "project_identity",
		Confidence:  projectType.Confidence,
		ExtractedAt: time.Now(),
		Metadata: map[string]interface{}{
			"language":     projectType.Language,
			"framework":    projectType.Framework,
			"build_system": projectType.BuildSystem,
			"architecture": projectType.Architecture,
		},
	})

	// 3. Analyze dependencies
	deps := r.analyzeDependencies(workspace, projectType)
	for _, dep := range deps {
		result.Atoms = append(result.Atoms, KnowledgeAtom{
			SourceURL:   workspace,
			Title:       "Dependency: " + dep.Name,
			Content:     fmt.Sprintf("Project depends on %s (version: %s)", dep.Name, dep.Version),
			Concept:     "dependency",
			Confidence:  0.95,
			ExtractedAt: time.Now(),
			Metadata: map[string]interface{}{
				"name":    dep.Name,
				"version": dep.Version,
				"type":    dep.Type,
			},
		})
	}

	// 4. Extract architectural patterns
	patterns := r.detectArchitecturalPatterns(workspace, fileFacts)
	for _, pattern := range patterns {
		result.Atoms = append(result.Atoms, KnowledgeAtom{
			SourceURL:   workspace,
			Title:       "Architectural Pattern: " + pattern,
			Content:     fmt.Sprintf("Detected architectural pattern: %s", pattern),
			Concept:     "architecture",
			Confidence:  0.8,
			ExtractedAt: time.Now(),
		})
	}

	// 5. Find important files (README, config, entry points)
	importantFiles := r.findImportantFiles(workspace)
	for _, file := range importantFiles {
		content, _ := os.ReadFile(file)
		summary := r.summarizeFile(file, string(content))
		result.Atoms = append(result.Atoms, KnowledgeAtom{
			SourceURL:   file,
			Title:       "Important File: " + filepath.Base(file),
			Content:     summary,
			Concept:     "key_file",
			Confidence:  0.9,
			ExtractedAt: time.Now(),
		})
	}

	// 6. Generate summary using LLM if available
	if r.llmClient != nil {
		summary, err := r.generateCodebaseSummary(ctx, result)
		if err == nil {
			result.Summary = summary
		}
	} else {
		result.Summary = fmt.Sprintf("Analyzed %d files. Project: %s (%s). Dependencies: %d. Patterns: %v",
			len(fileFacts), projectType.Language, projectType.Framework, len(deps), patterns)
	}

	result.Duration = time.Since(r.startTime)
	result.FactsGenerated = len(result.Atoms)

	return result, nil
}

// ProjectType represents detected project characteristics.
type ProjectType struct {
	Language     string
	Framework    string
	BuildSystem  string
	Architecture string
	Confidence   float64
}

// detectProjectType analyzes the workspace to determine project type.
func (r *ResearcherShard) detectProjectType(workspace string) ProjectType {
	pt := ProjectType{
		Language:     "unknown",
		Architecture: "unknown",
		Confidence:   0.5,
	}

	// Check for language markers
	markers := map[string]struct {
		lang      string
		framework string
		build     string
	}{
		"go.mod":           {"go", "", "go"},
		"go.sum":           {"go", "", "go"},
		"Cargo.toml":       {"rust", "", "cargo"},
		"package.json":     {"javascript", "", "npm"},
		"requirements.txt": {"python", "", "pip"},
		"pyproject.toml":   {"python", "", "poetry"},
		"pom.xml":          {"java", "", "maven"},
		"build.gradle":     {"java", "", "gradle"},
		"Gemfile":          {"ruby", "rails", "bundler"},
		"composer.json":    {"php", "", "composer"},
	}

	for file, info := range markers {
		if _, err := os.Stat(filepath.Join(workspace, file)); err == nil {
			pt.Language = info.lang
			pt.BuildSystem = info.build
			pt.Confidence = 0.95
			if info.framework != "" {
				pt.Framework = info.framework
			}
			break
		}
	}

	// Detect frameworks
	if pt.Language == "javascript" {
		if content, err := os.ReadFile(filepath.Join(workspace, "package.json")); err == nil {
			s := string(content)
			switch {
			case strings.Contains(s, `"next"`):
				pt.Framework = "nextjs"
			case strings.Contains(s, `"react"`):
				pt.Framework = "react"
			case strings.Contains(s, `"vue"`):
				pt.Framework = "vue"
			case strings.Contains(s, `"express"`):
				pt.Framework = "express"
			}
		}
	}

	if pt.Language == "go" {
		if content, err := os.ReadFile(filepath.Join(workspace, "go.mod")); err == nil {
			s := string(content)
			switch {
			case strings.Contains(s, "gin-gonic"):
				pt.Framework = "gin"
			case strings.Contains(s, "echo"):
				pt.Framework = "echo"
			case strings.Contains(s, "fiber"):
				pt.Framework = "fiber"
			case strings.Contains(s, "chi"):
				pt.Framework = "chi"
			}
		}
	}

	// Detect architecture
	dirs := []string{"cmd", "internal", "pkg", "api", "web", "services", "handlers"}
	foundDirs := 0
	for _, dir := range dirs {
		if info, err := os.Stat(filepath.Join(workspace, dir)); err == nil && info.IsDir() {
			foundDirs++
		}
	}

	if foundDirs >= 3 {
		pt.Architecture = "clean_architecture"
	} else if _, err := os.Stat(filepath.Join(workspace, "docker-compose.yml")); err == nil {
		pt.Architecture = "microservices"
	} else {
		pt.Architecture = "monolith"
	}

	return pt
}

// Dependency represents a project dependency.
type Dependency struct {
	Name    string
	Version string
	Type    string // "direct" or "indirect"
}

// analyzeDependencies extracts project dependencies.
func (r *ResearcherShard) analyzeDependencies(workspace string, pt ProjectType) []Dependency {
	var deps []Dependency

	switch pt.Language {
	case "go":
		deps = r.parseGoMod(filepath.Join(workspace, "go.mod"))
	case "javascript":
		deps = r.parsePackageJSON(filepath.Join(workspace, "package.json"))
	case "python":
		deps = r.parseRequirements(filepath.Join(workspace, "requirements.txt"))
	case "rust":
		deps = r.parseCargoToml(filepath.Join(workspace, "Cargo.toml"))
	}

	return deps
}

// parseGoMod extracts dependencies from go.mod.
func (r *ResearcherShard) parseGoMod(path string) []Dependency {
	var deps []Dependency
	content, err := os.ReadFile(path)
	if err != nil {
		return deps
	}

	lines := strings.Split(string(content), "\n")
	inRequire := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "require (") {
			inRequire = true
			continue
		}
		if line == ")" {
			inRequire = false
			continue
		}
		if inRequire || strings.HasPrefix(line, "require ") {
			parts := strings.Fields(strings.TrimPrefix(line, "require "))
			if len(parts) >= 2 {
				depType := "direct"
				if strings.Contains(line, "// indirect") {
					depType = "indirect"
				}
				deps = append(deps, Dependency{
					Name:    parts[0],
					Version: parts[1],
					Type:    depType,
				})
			}
		}
	}

	return deps
}

// parsePackageJSON extracts dependencies from package.json.
func (r *ResearcherShard) parsePackageJSON(path string) []Dependency {
	var deps []Dependency
	content, err := os.ReadFile(path)
	if err != nil {
		return deps
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(content, &pkg); err != nil {
		return deps
	}

	for name, version := range pkg.Dependencies {
		deps = append(deps, Dependency{Name: name, Version: version, Type: "direct"})
	}
	for name, version := range pkg.DevDependencies {
		deps = append(deps, Dependency{Name: name, Version: version, Type: "dev"})
	}

	return deps
}

// parseRequirements extracts dependencies from requirements.txt.
func (r *ResearcherShard) parseRequirements(path string) []Dependency {
	var deps []Dependency
	content, err := os.ReadFile(path)
	if err != nil {
		return deps
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Handle various formats: pkg==1.0, pkg>=1.0, pkg
		re := regexp.MustCompile(`^([a-zA-Z0-9_-]+)([<>=!]+)?(.*)$`)
		if matches := re.FindStringSubmatch(line); len(matches) > 0 {
			deps = append(deps, Dependency{
				Name:    matches[1],
				Version: matches[2] + matches[3],
				Type:    "direct",
			})
		}
	}

	return deps
}

// parseCargoToml extracts dependencies from Cargo.toml.
func (r *ResearcherShard) parseCargoToml(path string) []Dependency {
	var deps []Dependency
	content, err := os.ReadFile(path)
	if err != nil {
		return deps
	}

	lines := strings.Split(string(content), "\n")
	inDeps := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[dependencies]") {
			inDeps = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inDeps = false
			continue
		}
		if inDeps && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[0])
				version := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				deps = append(deps, Dependency{Name: name, Version: version, Type: "direct"})
			}
		}
	}

	return deps
}

// detectArchitecturalPatterns identifies common patterns in the codebase.
func (r *ResearcherShard) detectArchitecturalPatterns(workspace string, facts []core.Fact) []string {
	var patterns []string

	// Check directory structure from facts
	dirs := make(map[string]bool)
	for _, fact := range facts {
		if fact.Predicate == "directory" && len(fact.Args) > 0 {
			if path, ok := fact.Args[0].(string); ok {
				rel, _ := filepath.Rel(workspace, path)
				dirs[rel] = true
			}
		}
	}

	// Detect patterns based on structure
	if dirs["cmd"] && dirs["internal"] && dirs["pkg"] {
		patterns = append(patterns, "standard_go_layout")
	}
	if dirs["src"] && dirs["tests"] {
		patterns = append(patterns, "src_tests_separation")
	}
	if dirs["api"] || dirs["handlers"] || dirs["routes"] {
		patterns = append(patterns, "api_layer")
	}
	if dirs["services"] || dirs["usecases"] {
		patterns = append(patterns, "service_layer")
	}
	if dirs["repository"] || dirs["store"] || dirs["db"] {
		patterns = append(patterns, "repository_pattern")
	}
	if dirs["domain"] || dirs["entities"] || dirs["models"] {
		patterns = append(patterns, "domain_driven")
	}

	// Check for specific files
	if _, err := os.Stat(filepath.Join(workspace, "Dockerfile")); err == nil {
		patterns = append(patterns, "containerized")
	}
	if _, err := os.Stat(filepath.Join(workspace, ".github/workflows")); err == nil {
		patterns = append(patterns, "ci_cd_github_actions")
	}

	return patterns
}

// findImportantFiles locates key files in the workspace.
func (r *ResearcherShard) findImportantFiles(workspace string) []string {
	important := []string{
		"README.md", "README", "readme.md",
		"CLAUDE.md", ".claude",
		"go.mod", "package.json", "Cargo.toml", "requirements.txt",
		"Makefile", "docker-compose.yml", "Dockerfile",
		".env.example", "config.yaml", "config.json",
	}

	var found []string
	for _, file := range important {
		path := filepath.Join(workspace, file)
		if _, err := os.Stat(path); err == nil {
			found = append(found, path)
		}
	}

	// Also find main entry points
	entryPatterns := []string{
		"main.go", "cmd/*/main.go",
		"index.js", "index.ts", "app.js", "app.ts",
		"main.py", "app.py", "__main__.py",
		"main.rs", "lib.rs",
	}
	for _, pattern := range entryPatterns {
		matches, _ := filepath.Glob(filepath.Join(workspace, pattern))
		found = append(found, matches...)
	}

	return found
}

// summarizeFile creates a brief summary of a file's purpose.
func (r *ResearcherShard) summarizeFile(path string, content string) string {
	base := filepath.Base(path)

	// For known files, provide specific summaries
	switch base {
	case "go.mod":
		lines := strings.Split(content, "\n")
		if len(lines) > 0 && strings.HasPrefix(lines[0], "module ") {
			return fmt.Sprintf("Go module: %s", strings.TrimPrefix(lines[0], "module "))
		}
	case "package.json":
		var pkg struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if json.Unmarshal([]byte(content), &pkg) == nil {
			return fmt.Sprintf("NPM package: %s - %s", pkg.Name, pkg.Description)
		}
	case "README.md", "readme.md":
		// Extract first paragraph
		lines := strings.Split(content, "\n")
		var summary strings.Builder
		for _, line := range lines {
			if strings.TrimSpace(line) == "" && summary.Len() > 0 {
				break
			}
			if !strings.HasPrefix(line, "#") && strings.TrimSpace(line) != "" {
				summary.WriteString(line + " ")
			}
		}
		if summary.Len() > 200 {
			return summary.String()[:200] + "..."
		}
		return summary.String()
	}

	// Generic summary: first non-empty line
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "//") && !strings.HasPrefix(line, "#") {
			if len(line) > 100 {
				return line[:100] + "..."
			}
			return line
		}
	}

	return fmt.Sprintf("File: %s (%d bytes)", base, len(content))
}

// generateCodebaseSummary uses LLM to create a comprehensive summary.
func (r *ResearcherShard) generateCodebaseSummary(ctx context.Context, result *ResearchResult) (string, error) {
	// Build context from atoms
	var contextStr strings.Builder
	contextStr.WriteString("Analyzed codebase with the following findings:\n\n")
	for _, atom := range result.Atoms {
		contextStr.WriteString(fmt.Sprintf("- %s: %s\n", atom.Title, atom.Content))
	}

	prompt := fmt.Sprintf(`Based on this codebase analysis, provide a concise 2-3 sentence summary suitable for an AI coding agent to understand the project context:

%s

Summary (2-3 sentences):`, contextStr.String())

	summary, err := r.llmComplete(ctx, prompt)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(summary), nil
}

// ============================================================================
// MODE 2: WEB RESEARCH (for knowledge building)
// ============================================================================

// KnowledgeSource defines a documentation source with API access
type KnowledgeSource struct {
	Name       string
	Type       string // "github", "pkggodev", "llm", "raw"
	RepoOwner  string // for GitHub sources
	RepoName   string // for GitHub sources
	PackageURL string // direct package URL
	DocURL     string // documentation URL
}

// knownSources maps technology names to their documentation sources
var knownSources = map[string]KnowledgeSource{
	"rod": {
		Name:       "Rod Browser Automation",
		Type:       "github",
		RepoOwner:  "go-rod",
		RepoName:   "rod",
		PackageURL: "github.com/go-rod/rod",
		DocURL:     "https://go-rod.github.io",
	},
	"mangle": {
		Name:       "Google Mangle Datalog",
		Type:       "github",
		RepoOwner:  "google",
		RepoName:   "mangle",
		PackageURL: "github.com/google/mangle",
	},
	"bubbletea": {
		Name:       "Bubble Tea TUI Framework",
		Type:       "github",
		RepoOwner:  "charmbracelet",
		RepoName:   "bubbletea",
		PackageURL: "github.com/charmbracelet/bubbletea",
	},
	"cobra": {
		Name:       "Cobra CLI Framework",
		Type:       "github",
		RepoOwner:  "spf13",
		RepoName:   "cobra",
		PackageURL: "github.com/spf13/cobra",
	},
	"gin": {
		Name:       "Gin Web Framework",
		Type:       "github",
		RepoOwner:  "gin-gonic",
		RepoName:   "gin",
		PackageURL: "github.com/gin-gonic/gin",
	},
	"echo": {
		Name:       "Echo Web Framework",
		Type:       "github",
		RepoOwner:  "labstack",
		RepoName:   "echo",
		PackageURL: "github.com/labstack/echo",
	},
	"fiber": {
		Name:       "Fiber Web Framework",
		Type:       "github",
		RepoOwner:  "gofiber",
		RepoName:   "fiber",
		PackageURL: "github.com/gofiber/fiber",
	},
	"zap": {
		Name:       "Zap Logging",
		Type:       "github",
		RepoOwner:  "uber-go",
		RepoName:   "zap",
		PackageURL: "go.uber.org/zap",
	},
	"sqlite": {
		Name:       "SQLite Database",
		Type:       "github",
		RepoOwner:  "mattn",
		RepoName:   "go-sqlite3",
		PackageURL: "github.com/mattn/go-sqlite3",
	},
	"gorm": {
		Name:       "GORM ORM",
		Type:       "github",
		RepoOwner:  "go-gorm",
		RepoName:   "gorm",
		PackageURL: "gorm.io/gorm",
	},
	"react": {
		Name:       "React Frontend Framework",
		Type:       "llm",
	},
	"typescript": {
		Name:       "TypeScript Language",
		Type:       "llm",
	},
	"kubernetes": {
		Name:       "Kubernetes Container Orchestration",
		Type:       "llm",
	},
	"docker": {
		Name:       "Docker Containerization",
		Type:       "llm",
	},
	"security": {
		Name:       "Security Best Practices",
		Type:       "llm",
	},
	"testing": {
		Name:       "Testing Best Practices",
		Type:       "llm",
	},
}

// conductWebResearch performs deep web research on a topic using multi-strategy approach.
func (r *ResearcherShard) conductWebResearch(ctx context.Context, topic string, keywords []string) (*ResearchResult, error) {
	result := &ResearchResult{
		Query:    topic,
		Keywords: keywords,
		Atoms:    make([]KnowledgeAtom, 0),
	}

	fmt.Printf("[Researcher] Conducting knowledge research on: %s\n", topic)

	// Normalize topic for lookup
	normalizedTopic := strings.ToLower(strings.TrimSpace(topic))
	normalizedTopic = strings.TrimPrefix(normalizedTopic, "research docs: ")
	normalizedTopic = strings.TrimSuffix(normalizedTopic, " (brief overview)")
	normalizedTopic = strings.TrimSpace(normalizedTopic)

	// Check for deep research flag
	isDeepResearch := strings.Contains(topic, "(deep)")

	// Use wait group for parallel research strategies
	var wg sync.WaitGroup
	var mu sync.Mutex
	atomsChan := make(chan []KnowledgeAtom, 5)
	context7Found := false // Track if Context7 returned results

	// STRATEGY 0 (PRIMARY): Context7 - LLM-optimized documentation
	// This is the preferred source - curated docs designed for AI consumption
	if r.toolkit != nil && r.toolkit.Context7() != nil && r.toolkit.Context7().IsConfigured() {
		fmt.Printf("[Researcher] Querying Context7 for: %s\n", normalizedTopic)
		atoms, err := r.toolkit.Context7().ResearchTopic(ctx, normalizedTopic, keywords)
		if err == nil && len(atoms) > 0 {
			fmt.Printf("[Researcher] Context7 returned %d atoms (LLM-optimized docs)\n", len(atoms))
			result.Atoms = append(result.Atoms, atoms...)
			result.PagesScraped++
			context7Found = true
		} else if err != nil {
			fmt.Printf("[Researcher] Context7 unavailable: %v (falling back to other sources)\n", err)
		}
	}

	// If Context7 returned sufficient results, skip LLM synthesis to avoid timeouts
	// Only synthesize when Context7 data is insufficient (< 10 atoms)
	if context7Found && len(result.Atoms) >= 10 {
		// Context7 gave us enough - skip slow LLM synthesis
		fmt.Printf("[Researcher] Context7 provided sufficient data (%d atoms), skipping LLM synthesis\n", len(result.Atoms))
	} else if context7Found && len(result.Atoms) >= 1 {
		// Context7 gave some data but not enough - supplement with LLM
		if r.llmClient != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				fmt.Printf("[Researcher] Synthesizing supplemental knowledge from LLM...\n")
				atoms, err := r.synthesizeKnowledgeFromLLM(ctx, normalizedTopic, keywords)
				if err == nil && len(atoms) > 0 {
					atomsChan <- atoms
				}
			}()
		}
	} else {
		// Context7 not available or no results - use fallback strategies

		// Strategy 1: Check if we have a known source for this topic
		if source, ok := r.findKnowledgeSource(normalizedTopic); ok {
			fmt.Printf("[Researcher] Found known source: %s (type: %s)\n", source.Name, source.Type)
			wg.Add(1)
			go func() {
				defer wg.Done()

				// Use toolkit if available for enhanced fetching
				if r.toolkit != nil && r.toolkit.GitHub() != nil && source.Type == "github" {
					atoms, err := r.toolkit.GitHub().FetchRepository(ctx, source.RepoOwner, source.RepoName, keywords)
					if err == nil && len(atoms) > 0 {
						atomsChan <- atoms
						return
					}
				}

				// Fallback to original method
				atoms, err := r.fetchFromKnownSource(ctx, source, keywords)
				if err == nil && len(atoms) > 0 {
					atomsChan <- atoms
				} else if err != nil {
					fmt.Printf("[Researcher] Known source failed: %v\n", err)
				}
			}()
		}

		// Strategy 2: Web search (for deep research or unknown topics)
		if isDeepResearch || result.PagesScraped == 0 {
			if r.toolkit != nil && r.toolkit.Search() != nil {
				wg.Add(1)
				go func() {
					defer wg.Done()
					searchQuery := fmt.Sprintf("%s documentation tutorial", normalizedTopic)
					atoms, err := r.toolkit.Search().SearchAndFetch(ctx, searchQuery, 5)
					if err == nil && len(atoms) > 0 {
						// Score and filter atoms
						filtered := make([]KnowledgeAtom, 0, len(atoms))
						for _, atom := range atoms {
							score := r.calculateC7Score(atom)
							if score >= 0.5 {
								atom.Confidence = score
								filtered = append(filtered, atom)
							}
						}
						if len(filtered) > 0 {
							atomsChan <- filtered
						}
					}
				}()
			}
		}

		// Strategy 3: LLM knowledge synthesis (always run in parallel)
		if r.llmClient != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				fmt.Printf("[Researcher] Synthesizing knowledge from LLM...\n")
				atoms, err := r.synthesizeKnowledgeFromLLM(ctx, normalizedTopic, keywords)
				if err == nil && len(atoms) > 0 {
					atomsChan <- atoms
				} else if err != nil {
					fmt.Printf("[Researcher] LLM synthesis warning: %v\n", err)
				}
			}()
		}
	}

	// Collect results in background
	go func() {
		wg.Wait()
		close(atomsChan)
	}()

	// Gather all atoms
	for atoms := range atomsChan {
		mu.Lock()
		result.Atoms = append(result.Atoms, atoms...)
		result.PagesScraped++
		mu.Unlock()
	}

	result.Duration = time.Since(r.startTime)
	result.FactsGenerated = len(result.Atoms)

	// Generate summary
	if r.llmClient != nil && len(result.Atoms) > 0 {
		summary, err := r.generateResearchSummary(ctx, result)
		if err == nil {
			result.Summary = summary
		}
	} else if len(result.Atoms) > 0 {
		result.Summary = fmt.Sprintf("Researched '%s': found %d knowledge atoms",
			topic, len(result.Atoms))
	} else {
		result.Summary = fmt.Sprintf("Limited knowledge available for '%s'", topic)
	}

	return result, nil
}

// findKnowledgeSource looks up a known source for the topic
func (r *ResearcherShard) findKnowledgeSource(topic string) (KnowledgeSource, bool) {
	// Direct lookup
	if source, ok := knownSources[topic]; ok {
		return source, true
	}

	// Partial match lookup
	for key, source := range knownSources {
		if strings.Contains(topic, key) || strings.Contains(key, topic) {
			return source, true
		}
	}

	return KnowledgeSource{}, false
}

// fetchFromKnownSource fetches documentation from a known source
func (r *ResearcherShard) fetchFromKnownSource(ctx context.Context, source KnowledgeSource, keywords []string) ([]KnowledgeAtom, error) {
	switch source.Type {
	case "github":
		return r.fetchGitHubDocs(ctx, source)
	case "pkggodev":
		return r.fetchPkgGoDev(ctx, source)
	case "llm":
		// LLM sources are handled separately
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown source type: %s", source.Type)
	}
}

// fetchGitHubDocs fetches README and docs from GitHub using raw URLs (no API key needed)
// Implements Context7-like multi-stage ingestion:
// 1. Check for llms.txt (AI-optimized docs pointer)
// 2. Fetch and parse documentation
// 3. Enrich with LLM metadata
// 4. Score content quality
func (r *ResearcherShard) fetchGitHubDocs(ctx context.Context, source KnowledgeSource) ([]KnowledgeAtom, error) {
	var atoms []KnowledgeAtom

	// Stage 1: Check for llms.txt (Context7-style AI docs pointer)
	llmsTxtURLs := []string{
		fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/llms.txt", source.RepoOwner, source.RepoName),
		fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/master/llms.txt", source.RepoOwner, source.RepoName),
		fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/.llms.txt", source.RepoOwner, source.RepoName),
	}

	var llmsContent string
	for _, url := range llmsTxtURLs {
		content, err := r.fetchRawContent(ctx, url)
		if err == nil && len(content) > 10 {
			llmsContent = content
			fmt.Printf("[Researcher] Found llms.txt at %s - using AI-optimized docs\n", url)
			break
		}
	}

	// If llms.txt exists, parse it for doc pointers
	if llmsContent != "" {
		llmsAtoms := r.parseLlmsTxt(ctx, source, llmsContent)
		atoms = append(atoms, llmsAtoms...)
	}

	// Stage 2: Fetch README (primary documentation)
	readmeURLs := []string{
		fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/README.md", source.RepoOwner, source.RepoName),
		fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/master/README.md", source.RepoOwner, source.RepoName),
	}

	var readmeContent string
	var readmeURL string
	for _, url := range readmeURLs {
		content, err := r.fetchRawContent(ctx, url)
		if err == nil && len(content) > 100 {
			readmeContent = content
			readmeURL = url
			fmt.Printf("[Researcher] Fetched README from %s (%d bytes)\n", url, len(content))
			break
		}
	}

	if readmeContent != "" {
		// Parse raw content into atoms
		readmeAtoms := r.parseReadmeContent(source.Name, readmeContent)

		// Stage 3: Enrich atoms with LLM metadata (Context7-style enrichment)
		for i := range readmeAtoms {
			readmeAtoms[i].SourceURL = readmeURL
			readmeAtoms[i] = r.enrichAtomWithLLM(ctx, readmeAtoms[i])

			// Stage 4: Score content quality
			score := r.calculateC7Score(readmeAtoms[i])
			if score >= 0.5 { // Only keep atoms with good quality score
				readmeAtoms[i].Confidence = score
				atoms = append(atoms, readmeAtoms[i])
			} else {
				fmt.Printf("[Researcher] Discarding low-quality atom: %s (score: %.2f)\n", readmeAtoms[i].Title, score)
			}
		}
	}

	// Also try to fetch examples or docs if available
	docsURLs := []string{
		fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/docs/README.md", source.RepoOwner, source.RepoName),
		fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/GETTING_STARTED.md", source.RepoOwner, source.RepoName),
		fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/examples/README.md", source.RepoOwner, source.RepoName),
		fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/docs/getting-started.md", source.RepoOwner, source.RepoName),
	}

	for _, url := range docsURLs {
		content, err := r.fetchRawContent(ctx, url)
		if err == nil && len(content) > 100 {
			atom := KnowledgeAtom{
				SourceURL:   url,
				Title:       "Additional Documentation",
				Content:     r.truncate(content, 2000),
				Concept:     "documentation",
				Confidence:  0.85,
				ExtractedAt: time.Now(),
			}
			atom = r.enrichAtomWithLLM(ctx, atom)
			if r.calculateC7Score(atom) >= 0.5 {
				atoms = append(atoms, atom)
			}
		}
	}

	return atoms, nil
}

// parseLlmsTxt parses an llms.txt file (Context7 standard) to find AI-optimized doc pointers
// Format: Each line is a URL or path to documentation optimized for LLMs
func (r *ResearcherShard) parseLlmsTxt(ctx context.Context, source KnowledgeSource, content string) []KnowledgeAtom {
	var atoms []KnowledgeAtom
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle relative paths
		var docURL string
		if strings.HasPrefix(line, "http") {
			docURL = line
		} else {
			// Relative path - construct GitHub raw URL
			docURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/%s",
				source.RepoOwner, source.RepoName, strings.TrimPrefix(line, "/"))
		}

		content, err := r.fetchRawContent(ctx, docURL)
		if err != nil {
			fmt.Printf("[Researcher] llms.txt pointer failed: %s - %v\n", docURL, err)
			continue
		}

		if len(content) > 50 {
			// llms.txt content is pre-optimized for LLMs - higher base confidence
			atom := KnowledgeAtom{
				SourceURL:   docURL,
				Title:       "AI-Optimized Documentation",
				Content:     r.truncate(content, 3000), // Allow more content for llms.txt docs
				Concept:     "llms_optimized",
				Confidence:  0.95, // Higher confidence for llms.txt content
				ExtractedAt: time.Now(),
				Metadata: map[string]interface{}{
					"source_type": "llms_txt",
				},
			}
			atoms = append(atoms, atom)
			fmt.Printf("[Researcher] Ingested llms.txt doc: %s (%d bytes)\n", docURL, len(content))
		}
	}

	return atoms
}

// enrichAtomWithLLM uses LLM to add metadata and summaries (Context7-style enrichment)
func (r *ResearcherShard) enrichAtomWithLLM(ctx context.Context, atom KnowledgeAtom) KnowledgeAtom {
	// Only enrich substantial content and if LLM is available
	if r.llmClient == nil || len(atom.Content) < 100 || atom.Concept == "llms_optimized" {
		return atom
	}

	// Generate a concise, AI-optimized summary
	prompt := fmt.Sprintf(`Summarize this documentation for an AI coding assistant in 1-2 sentences. Focus on: what it does, when to use it, and any important caveats.

Documentation:
%s

Summary:`, r.truncate(atom.Content, 1000))

	summary, err := r.llmComplete(ctx, prompt)
	if err != nil {
		return atom
	}

	summary = strings.TrimSpace(summary)
	if len(summary) > 10 && len(summary) < len(atom.Content) {
		// Store original content in metadata, use summary as main content
		if atom.Metadata == nil {
			atom.Metadata = make(map[string]interface{})
		}
		atom.Metadata["original_content"] = atom.Content
		atom.Metadata["enriched"] = true
		atom.Content = summary
	}

	return atom
}

// calculateC7Score implements a Context7-style quality scoring algorithm
// Returns a score from 0.0 to 1.0 based on content quality indicators
func (r *ResearcherShard) calculateC7Score(atom KnowledgeAtom) float64 {
	score := 0.5 // Base score

	// Content length checks
	contentLen := len(atom.Content)
	if contentLen > 50 {
		score += 0.1
	}
	if contentLen > 200 {
		score += 0.1
	}
	if contentLen < 20 {
		score -= 0.3 // Too short
	}

	// Code example bonus
	if atom.CodePattern != "" && len(atom.CodePattern) > 20 {
		score += 0.15
	}

	// Title quality
	if atom.Title != "" && len(atom.Title) > 5 {
		score += 0.05
	}

	// Source quality
	if atom.SourceURL != "" && strings.Contains(atom.SourceURL, "github") {
		score += 0.05
	}

	// Penalize garbage content indicators
	content := strings.ToLower(atom.Content)
	garbageIndicators := []string{
		"captcha", "robot", "verify you are human",
		"access denied", "403 forbidden", "404 not found",
		"please enable javascript", "cloudflare",
	}
	for _, indicator := range garbageIndicators {
		if strings.Contains(content, indicator) {
			score -= 0.5 // Heavy penalty for garbage content
		}
	}

	// Ensure score is in valid range
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}

// fetchRawContent fetches raw content from a URL
func (r *ResearcherShard) fetchRawContent(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", r.researchConfig.UserAgent)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 500*1024)) // 500KB limit
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// parseReadmeContent extracts structured knowledge atoms from README content
func (r *ResearcherShard) parseReadmeContent(name, content string) []KnowledgeAtom {
	var atoms []KnowledgeAtom

	// Extract title/description (first paragraph after # heading)
	lines := strings.Split(content, "\n")
	var description strings.Builder
	inDescription := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			inDescription = true
			continue
		}
		if inDescription && line != "" && !strings.HasPrefix(line, "#") {
			description.WriteString(line + " ")
			if description.Len() > 500 {
				break
			}
		}
		if inDescription && line == "" && description.Len() > 50 {
			break
		}
	}

	if description.Len() > 0 {
		atoms = append(atoms, KnowledgeAtom{
			Title:       name + " Overview",
			Content:     strings.TrimSpace(description.String()),
			Concept:     "overview",
			Confidence:  0.95,
			ExtractedAt: time.Now(),
		})
	}

	// Extract code examples (```go or ``` blocks)
	codeBlockRegex := regexp.MustCompile("(?s)```(?:go|golang)?\\s*\\n(.+?)```")
	matches := codeBlockRegex.FindAllStringSubmatch(content, 5) // Max 5 examples
	for i, match := range matches {
		if len(match) > 1 && len(match[1]) > 20 && len(match[1]) < 2000 {
			atoms = append(atoms, KnowledgeAtom{
				Title:       fmt.Sprintf("%s Code Example %d", name, i+1),
				Content:     "Code example from documentation",
				CodePattern: strings.TrimSpace(match[1]),
				Concept:     "code_example",
				Confidence:  0.9,
				ExtractedAt: time.Now(),
			})
		}
	}

	// Extract sections (## headings with content)
	sectionRegex := regexp.MustCompile(`(?m)^##\s+(.+?)\n([\s\S]*?)(?=^##|\z)`)
	sectionMatches := sectionRegex.FindAllStringSubmatch(content, 10)
	for _, match := range sectionMatches {
		if len(match) > 2 {
			sectionTitle := strings.TrimSpace(match[1])
			sectionContent := strings.TrimSpace(match[2])
			if len(sectionContent) > 50 && len(sectionContent) < 3000 {
				// Skip common non-informative sections
				lowerTitle := strings.ToLower(sectionTitle)
				if lowerTitle == "license" || lowerTitle == "contributing" || lowerTitle == "changelog" {
					continue
				}
				atoms = append(atoms, KnowledgeAtom{
					Title:       sectionTitle,
					Content:     r.truncate(sectionContent, 1000),
					Concept:     "documentation_section",
					Confidence:  0.85,
					ExtractedAt: time.Now(),
				})
			}
		}
	}

	return atoms
}

// fetchPkgGoDev fetches documentation from pkg.go.dev
func (r *ResearcherShard) fetchPkgGoDev(ctx context.Context, source KnowledgeSource) ([]KnowledgeAtom, error) {
	// pkg.go.dev doesn't have a public API, so we fall back to GitHub
	return r.fetchGitHubDocs(ctx, source)
}

// synthesizeKnowledgeFromLLM uses the LLM to generate knowledge about a topic
func (r *ResearcherShard) synthesizeKnowledgeFromLLM(ctx context.Context, topic string, keywords []string) ([]KnowledgeAtom, error) {
	prompt := fmt.Sprintf(`You are a technical documentation specialist. Generate structured knowledge about "%s" for a developer assistant agent.

Generate the following in JSON format:
{
  "overview": "A 2-3 sentence overview of what this technology/library does",
  "key_concepts": ["concept1", "concept2", "concept3"],
  "best_practices": ["practice1", "practice2", "practice3"],
  "common_patterns": [
    {"name": "pattern name", "description": "brief description", "code": "example code if applicable"}
  ],
  "common_pitfalls": ["pitfall1", "pitfall2"],
  "related_technologies": ["tech1", "tech2"]
}

Be accurate and concise. Only include information you are confident about.
Topic: %s
Keywords: %s

JSON:`, topic, topic, strings.Join(keywords, ", "))

	response, err := r.llmComplete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	// Parse JSON response
	return r.parseLLMKnowledgeResponse(topic, response)
}

// parseLLMKnowledgeResponse parses the LLM's JSON response into knowledge atoms
func (r *ResearcherShard) parseLLMKnowledgeResponse(topic, response string) ([]KnowledgeAtom, error) {
	var atoms []KnowledgeAtom

	// Find JSON in response (might have surrounding text)
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		// Fallback: treat entire response as overview
		atoms = append(atoms, KnowledgeAtom{
			Title:       topic + " Overview (LLM)",
			Content:     r.truncate(response, 1000),
			Concept:     "llm_synthesized",
			Confidence:  0.7,
			ExtractedAt: time.Now(),
		})
		return atoms, nil
	}

	jsonStr := response[jsonStart : jsonEnd+1]

	var knowledge struct {
		Overview            string `json:"overview"`
		KeyConcepts         []string `json:"key_concepts"`
		BestPractices       []string `json:"best_practices"`
		CommonPatterns      []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Code        string `json:"code"`
		} `json:"common_patterns"`
		CommonPitfalls      []string `json:"common_pitfalls"`
		RelatedTechnologies []string `json:"related_technologies"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &knowledge); err != nil {
		// Fallback on parse error
		atoms = append(atoms, KnowledgeAtom{
			Title:       topic + " Overview (LLM)",
			Content:     r.truncate(response, 1000),
			Concept:     "llm_synthesized",
			Confidence:  0.7,
			ExtractedAt: time.Now(),
		})
		return atoms, nil
	}

	// Convert to atoms
	if knowledge.Overview != "" {
		atoms = append(atoms, KnowledgeAtom{
			Title:       topic + " Overview",
			Content:     knowledge.Overview,
			Concept:     "overview",
			Confidence:  0.85,
			ExtractedAt: time.Now(),
			Metadata:    map[string]interface{}{"source": "llm_synthesis"},
		})
	}

	for _, concept := range knowledge.KeyConcepts {
		atoms = append(atoms, KnowledgeAtom{
			Title:       "Key Concept: " + concept,
			Content:     concept,
			Concept:     "key_concept",
			Confidence:  0.8,
			ExtractedAt: time.Now(),
		})
	}

	for _, practice := range knowledge.BestPractices {
		atoms = append(atoms, KnowledgeAtom{
			Title:       "Best Practice",
			Content:     practice,
			Concept:     "best_practice",
			Confidence:  0.8,
			ExtractedAt: time.Now(),
		})
	}

	for _, pattern := range knowledge.CommonPatterns {
		atom := KnowledgeAtom{
			Title:       "Pattern: " + pattern.Name,
			Content:     pattern.Description,
			Concept:     "pattern",
			Confidence:  0.75,
			ExtractedAt: time.Now(),
		}
		if pattern.Code != "" {
			atom.CodePattern = pattern.Code
		}
		atoms = append(atoms, atom)
	}

	for _, pitfall := range knowledge.CommonPitfalls {
		atoms = append(atoms, KnowledgeAtom{
			Title:       "Common Pitfall",
			Content:     pitfall,
			Concept:     "anti_pattern",
			AntiPattern: pitfall,
			Confidence:  0.8,
			ExtractedAt: time.Now(),
		})
	}

	return atoms, nil
}

// generateSearchURLs creates a list of URLs to research (kept for compatibility but unused)
func (r *ResearcherShard) generateSearchURLs(topic string, keywords []string) []string {
	// This method is deprecated in favor of the multi-strategy approach
	// Kept for backward compatibility
	return []string{}
}

// fetchAndExtract fetches a URL and extracts knowledge atoms.
func (r *ResearcherShard) fetchAndExtract(ctx context.Context, url string, keywords []string) ([]KnowledgeAtom, error) {
	// Check domain restrictions
	if !r.isDomainAllowed(url) {
		return nil, fmt.Errorf("domain not allowed: %s", url)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", r.researchConfig.UserAgent)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Read body
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return nil, err
	}

	// Parse HTML
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	// Extract atoms
	atoms := r.extractAtomsFromHTML(doc, url, keywords)

	return atoms, nil
}

// isDomainAllowed checks if a URL's domain is allowed.
func (r *ResearcherShard) isDomainAllowed(url string) bool {
	for _, blocked := range r.researchConfig.BlockedDomains {
		if strings.Contains(url, blocked) {
			return false
		}
	}

	if len(r.researchConfig.AllowedDomains) == 0 {
		return true
	}

	for _, allowed := range r.researchConfig.AllowedDomains {
		if strings.Contains(url, allowed) {
			return true
		}
	}

	return false
}

// extractAtomsFromHTML extracts knowledge atoms from parsed HTML.
func (r *ResearcherShard) extractAtomsFromHTML(doc *html.Node, url string, keywords []string) []KnowledgeAtom {
	var atoms []KnowledgeAtom

	// Extract title
	title := r.extractTitle(doc)

	// Extract content sections
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "article", "main", "section", "div":
				content := r.extractTextContent(n)
				if len(content) > 100 && r.containsKeywords(content, keywords) {
					atoms = append(atoms, KnowledgeAtom{
						SourceURL:   url,
						Title:       title,
						Content:     r.truncate(content, 500),
						Concept:     "documentation",
						Confidence:  r.calculateConfidence(content, keywords),
						ExtractedAt: time.Now(),
					})
				}
			case "pre", "code":
				// Extract code snippets
				code := r.extractTextContent(n)
				if len(code) > 20 && len(code) < 2000 {
					atoms = append(atoms, KnowledgeAtom{
						SourceURL:   url,
						Title:       title,
						Content:     "Code example",
						CodePattern: code,
						Concept:     "code_example",
						Confidence:  0.8,
						ExtractedAt: time.Now(),
					})
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(doc)

	return atoms
}

// extractTitle extracts the page title from HTML.
func (r *ResearcherShard) extractTitle(doc *html.Node) string {
	var title string
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "title" && n.FirstChild != nil {
			title = n.FirstChild.Data
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(doc)
	return title
}

// extractTextContent extracts text from an HTML node.
func (r *ResearcherShard) extractTextContent(n *html.Node) string {
	var sb strings.Builder
	var traverse func(*html.Node)
	traverse = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
			sb.WriteString(" ")
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(n)
	return strings.TrimSpace(sb.String())
}

// containsKeywords checks if content contains any keywords.
func (r *ResearcherShard) containsKeywords(content string, keywords []string) bool {
	lower := strings.ToLower(content)
	for _, kw := range keywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// calculateConfidence calculates keyword-based confidence.
func (r *ResearcherShard) calculateConfidence(content string, keywords []string) float64 {
	lower := strings.ToLower(content)
	matches := 0
	for _, kw := range keywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			matches++
		}
	}
	if len(keywords) == 0 {
		return 0.5
	}
	return float64(matches) / float64(len(keywords))
}

// truncate shortens content to maxLen.
func (r *ResearcherShard) truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// generateResearchSummary uses LLM to summarize research findings.
func (r *ResearcherShard) generateResearchSummary(ctx context.Context, result *ResearchResult) (string, error) {
	var contentBuilder strings.Builder
	contentBuilder.WriteString(fmt.Sprintf("Research topic: %s\n\n", result.Query))
	for i, atom := range result.Atoms {
		if i >= 10 {
			break
		}
		contentBuilder.WriteString(fmt.Sprintf("- %s: %s\n", atom.Title, atom.Content))
	}

	prompt := fmt.Sprintf(`Summarize these research findings in 2-3 sentences for a developer:

%s

Summary:`, contentBuilder.String())

	return r.llmComplete(ctx, prompt)
}

// ============================================================================
// PERSISTENCE & FACT GENERATION
// ============================================================================

// persistKnowledge saves knowledge atoms to the local database.
func (r *ResearcherShard) persistKnowledge(result *ResearchResult) {
	if r.localDB == nil {
		return
	}

	for _, atom := range result.Atoms {
		// Store in vector store for semantic retrieval
		metadata := map[string]interface{}{
			"source_url": atom.SourceURL,
			"concept":    atom.Concept,
			"confidence": atom.Confidence,
		}
		r.localDB.StoreVector(atom.Content, metadata)

		// Store in knowledge graph
		r.localDB.StoreLink(atom.Concept, "has_instance", atom.Title, atom.Confidence, nil)
		if atom.CodePattern != "" {
			r.localDB.StoreLink(atom.Title, "has_pattern", atom.CodePattern, 0.9, nil)
		}

		// Store as cold storage fact
		r.localDB.StoreFact("knowledge_atom", []interface{}{
			atom.SourceURL, atom.Concept, atom.Title, atom.Content,
		}, "research", int(atom.Confidence*100))
	}
}

// generateFacts converts research results to Mangle facts.
func (r *ResearcherShard) generateFacts(result *ResearchResult) []core.Fact {
	var facts []core.Fact

	// Research completion fact
	facts = append(facts, core.Fact{
		Predicate: "research_complete",
		Args: []interface{}{
			result.Query,
			len(result.Atoms),
			result.Duration.Seconds(),
		},
	})

	// Knowledge atom facts
	for _, atom := range result.Atoms {
		facts = append(facts, core.Fact{
			Predicate: "knowledge_atom",
			Args: []interface{}{
				atom.SourceURL,
				atom.Concept,
				atom.Title,
				atom.Confidence,
			},
		})

		if atom.CodePattern != "" {
			facts = append(facts, core.Fact{
				Predicate: "code_pattern",
				Args: []interface{}{
					atom.Concept,
					atom.CodePattern,
				},
			})
		}
	}

	// Project profile facts (for codebase analysis)
	for _, atom := range result.Atoms {
		if atom.Concept == "project_identity" {
			if meta := atom.Metadata; meta != nil {
				if lang, ok := meta["language"].(string); ok {
					facts = append(facts, core.Fact{
						Predicate: "project_language",
						Args:      []interface{}{"/" + lang},
					})
				}
				if fw, ok := meta["framework"].(string); ok && fw != "" {
					facts = append(facts, core.Fact{
						Predicate: "project_framework",
						Args:      []interface{}{"/" + fw},
					})
				}
				if arch, ok := meta["architecture"].(string); ok {
					facts = append(facts, core.Fact{
						Predicate: "project_architecture",
						Args:      []interface{}{"/" + arch},
					})
				}
			}
		}
	}

	return facts
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
