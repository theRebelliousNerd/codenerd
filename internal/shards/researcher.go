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
		Timeout:         30 * time.Second,
		ConcurrentFetch: 3,
		AllowedDomains:  []string{}, // Allow all by default
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
		kernel:      core.NewRealKernel(),
		scanner:     world.NewScanner(),
		stopCh:      make(chan struct{}),
		visitedURLs: make(map[string]bool),
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
	if r.llmClient == nil {
		return "", fmt.Errorf("no LLM client configured")
	}

	// Build context from atoms
	var context strings.Builder
	context.WriteString("Analyzed codebase with the following findings:\n\n")
	for _, atom := range result.Atoms {
		context.WriteString(fmt.Sprintf("- %s: %s\n", atom.Title, atom.Content))
	}

	prompt := fmt.Sprintf(`Based on this codebase analysis, provide a concise 2-3 sentence summary suitable for an AI coding agent to understand the project context:

%s

Summary (2-3 sentences):`, context.String())

	summary, err := r.llmClient.Complete(ctx, prompt)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(summary), nil
}

// ============================================================================
// MODE 2: WEB RESEARCH (for knowledge building)
// ============================================================================

// conductWebResearch performs deep web research on a topic.
func (r *ResearcherShard) conductWebResearch(ctx context.Context, topic string, keywords []string) (*ResearchResult, error) {
	result := &ResearchResult{
		Query:    topic,
		Keywords: keywords,
		Atoms:    make([]KnowledgeAtom, 0),
	}

	fmt.Printf("[Researcher] Conducting web research on: %s\n", topic)

	// 1. Generate search URLs
	searchURLs := r.generateSearchURLs(topic, keywords)

	// 2. Fetch and extract from each URL
	sem := make(chan struct{}, r.researchConfig.ConcurrentFetch)
	var wg sync.WaitGroup
	atomsCh := make(chan KnowledgeAtom, 100)

	for _, url := range searchURLs {
		if r.visitedURLs[url] {
			continue
		}
		if len(r.visitedURLs) >= r.researchConfig.MaxPages {
			break
		}

		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			select {
			case <-ctx.Done():
				return
			case <-r.stopCh:
				return
			default:
			}

			atoms, err := r.fetchAndExtract(ctx, u, keywords)
			if err != nil {
				fmt.Printf("[Researcher] Failed to fetch %s: %v\n", u, err)
				return
			}

			for _, atom := range atoms {
				atomsCh <- atom
			}

			r.mu.Lock()
			r.visitedURLs[u] = true
			r.mu.Unlock()
		}(url)
	}

	// Collect results
	go func() {
		wg.Wait()
		close(atomsCh)
	}()

	for atom := range atomsCh {
		result.Atoms = append(result.Atoms, atom)
	}

	result.PagesScraped = len(r.visitedURLs)
	result.Duration = time.Since(r.startTime)
	result.FactsGenerated = len(result.Atoms)

	// Generate summary
	if r.llmClient != nil && len(result.Atoms) > 0 {
		summary, err := r.generateResearchSummary(ctx, result)
		if err == nil {
			result.Summary = summary
		}
	} else {
		result.Summary = fmt.Sprintf("Researched '%s': found %d knowledge atoms from %d pages",
			topic, len(result.Atoms), result.PagesScraped)
	}

	return result, nil
}

// generateSearchURLs creates a list of URLs to research.
func (r *ResearcherShard) generateSearchURLs(topic string, keywords []string) []string {
	var urls []string

	// Documentation sites
	docSites := map[string]string{
		"go":         "https://pkg.go.dev/search?q=%s",
		"rust":       "https://docs.rs/releases/search?query=%s",
		"python":     "https://pypi.org/search/?q=%s",
		"javascript": "https://www.npmjs.com/search?q=%s",
	}

	// Add documentation URLs based on keywords
	for lang, urlTemplate := range docSites {
		for _, kw := range keywords {
			if strings.Contains(strings.ToLower(topic), lang) || strings.Contains(strings.ToLower(kw), lang) {
				urls = append(urls, fmt.Sprintf(urlTemplate, strings.ReplaceAll(topic, " ", "+")))
				break
			}
		}
	}

	// Add generic documentation sites
	genericSites := []string{
		"https://devdocs.io/#q=%s",
		"https://stackoverflow.com/search?q=%s",
	}
	for _, site := range genericSites {
		urls = append(urls, fmt.Sprintf(site, strings.ReplaceAll(topic, " ", "+")))
	}

	return urls
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
	if r.llmClient == nil {
		return "", fmt.Errorf("no LLM client")
	}

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

	return r.llmClient.Complete(ctx, prompt)
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
