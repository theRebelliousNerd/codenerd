// Package researcher - Content extraction and knowledge atom creation.
// This file contains LLM-based knowledge synthesis, GitHub docs parsing, and atom enrichment.
package researcher

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// isLibraryOrFrameworkTopic detects if a topic is about a library, framework, or package
// that would benefit from Context7's curated documentation.
// Returns false for general programming questions, concepts, or non-library topics.
func (r *ResearcherShard) isLibraryOrFrameworkTopic(topic string) bool {
	lower := strings.ToLower(topic)

	// Explicit documentation keywords - strong signal
	docKeywords := []string{
		"library", "package", "module", "framework", "sdk", "api docs",
		"documentation", "docs for", "how to use", "getting started with",
		"install", "npm", "pip", "go get", "cargo", "maven", "gradle",
		"import", "require", "dependency",
	}
	for _, kw := range docKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}

	// Known framework/library patterns
	knownLibraries := []string{
		// JavaScript/TypeScript
		"react", "vue", "angular", "svelte", "next.js", "nextjs", "nuxt",
		"express", "fastify", "nest.js", "nestjs", "electron", "vite",
		"webpack", "rollup", "esbuild", "tailwind", "shadcn",
		// Go
		"gin", "echo", "fiber", "chi", "gorilla", "cobra", "viper",
		"bubbletea", "bubbles", "lipgloss", "charm", "rod", "chromedp",
		// Python
		"django", "flask", "fastapi", "pytorch", "tensorflow", "pandas",
		"numpy", "scikit", "requests", "beautifulsoup", "selenium",
		// Rust
		"tokio", "actix", "axum", "serde", "clap",
		// General
		"graphql", "grpc", "protobuf", "openapi", "swagger",
		"kubernetes", "docker", "terraform", "ansible",
		"redis", "postgres", "mongodb", "sqlite", "mysql",
	}
	for _, lib := range knownLibraries {
		if strings.Contains(lower, lib) {
			return true
		}
	}

	// Package name patterns (e.g., "go-rod/rod", "@types/node", "lodash")
	packagePatterns := []regexp.Regexp{
		*regexp.MustCompile(`[a-z]+-[a-z]+`),            // hyphenated names like "go-rod"
		*regexp.MustCompile(`@[a-z]+/[a-z]+`),           // scoped npm packages
		*regexp.MustCompile(`[a-z]+/[a-z]+`),            // owner/repo format
		*regexp.MustCompile(`v\d+\.\d+`),                // version numbers
		*regexp.MustCompile(`\.(js|ts|go|py|rs|java)$`), // file extensions in queries
	}
	for _, pattern := range packagePatterns {
		if pattern.MatchString(lower) {
			return true
		}
	}

	// Negative signals - general questions that shouldn't use Context7
	generalPatterns := []string{
		"what is", "how does", "why", "explain", "difference between",
		"best practice", "when to use", "should i", "compare",
		"concept", "principle", "pattern", "algorithm", "data structure",
	}
	generalCount := 0
	for _, gp := range generalPatterns {
		if strings.Contains(lower, gp) {
			generalCount++
		}
	}
	// If mostly general question words without library names, skip Context7
	if generalCount >= 2 {
		return false
	}

	// Default: if topic is short and looks like a package name, try Context7
	words := strings.Fields(lower)
	if len(words) <= 3 {
		// Short queries like "bubbletea" or "rod browser" are likely library lookups
		return true
	}

	return false
}

// conductWebResearch performs deep web research on a topic using multi-strategy approach.
func (r *ResearcherShard) conductWebResearch(ctx context.Context, topic string, keywords []string, urls []string) (*ResearchResult, error) {
	result := &ResearchResult{
		Query:    topic,
		Keywords: keywords,
		Atoms:    make([]KnowledgeAtom, 0),
	}

	logging.Researcher("Conducting knowledge research on: %s", topic)

	// Normalize topic for lookup
	normalizedTopic := strings.ToLower(strings.TrimSpace(topic))
	normalizedTopic = strings.TrimPrefix(normalizedTopic, "research docs: ")
	normalizedTopic = strings.TrimSuffix(normalizedTopic, " (brief overview)")
	normalizedTopic = strings.TrimSpace(normalizedTopic)

	// Check for deep research flag
	isDeepResearch := strings.Contains(topic, "(deep)")

	// Workspace-first: detect local queries and answer from the repo before hitting the network
	skipRemote := false
	if r.isWorkspaceQuery(normalizedTopic) {
		atoms, err := r.workspaceReferenceSearch(ctx, normalizedTopic)
		if err != nil {
			logging.Researcher("Workspace search fallback: %v", err)
		} else if len(atoms) > 0 {
			logging.Researcher("Workspace search found %d references; skipping remote sources", len(atoms))
			result.Atoms = append(result.Atoms, atoms...)
			result.PagesScraped++
			skipRemote = true
		} else {
			logging.Researcher("Workspace search produced no matches; continuing with remote strategies")
		}
	}

	// Use wait group for parallel research strategies
	var wg sync.WaitGroup
	var mu sync.Mutex
	atomsChan := make(chan []KnowledgeAtom, 10)
	context7Found := false // Track if Context7 returned results

	// STRATEGY -1: Explicit URLs (from wizard or task)
	if len(urls) > 0 {
		logging.Researcher("Scraping %d explicit URLs...", len(urls))
		for _, url := range urls {
			r.mu.Lock()
			if r.visitedURLs[url] {
				r.mu.Unlock()
				continue
			}
			r.visitedURLs[url] = true
			r.mu.Unlock()

			wg.Add(1)
			go func(u string) {
				defer wg.Done()

				// Check for GitHub URL to use specialized fetcher (supports llms.txt/Context7 standard)
				if strings.Contains(u, "github.com/") {
					// Remove protocol for splitting
					cleanU := strings.TrimPrefix(u, "https://")
					cleanU = strings.TrimPrefix(cleanU, "http://")
					parts := strings.Split(cleanU, "/")

					if len(parts) >= 3 && parts[0] == "github.com" {
						owner := parts[1]
						repo := parts[2]
						// Clean repo name of potential .git suffix or trailing slash
						repo = strings.TrimSuffix(repo, ".git")

						source := KnowledgeSource{
							Name:       repo,
							Type:       "github",
							RepoOwner:  owner,
							RepoName:   repo,
							PackageURL: "github.com/" + owner + "/" + repo,
						}

						logging.Researcher("Detected GitHub repo: %s/%s", owner, repo)
						atoms, err := r.fetchGitHubDocs(ctx, source, keywords)
						if err == nil && len(atoms) > 0 {
							logging.Researcher("Scraped %d atoms from GitHub %s", len(atoms), u)
							atomsChan <- atoms
							return
						} else if err != nil {
							logging.Researcher("GitHub fetch failed: %v, falling back to generic scraper", err)
						}
					}
				}

				// Generic scraper for non-GitHub URLs or fallback
				atoms, err := r.fetchAndExtract(ctx, u, keywords)
				if err == nil && len(atoms) > 0 {
					logging.Researcher("Scraped %d atoms from %s", len(atoms), u)
					atomsChan <- atoms
				} else {
					logging.Researcher("Failed to scrape %s: %v", u, err)
				}
			}(url)
		}
	}

	// STRATEGY 0 (CONDITIONAL): Context7 - LLM-optimized documentation
	// Only use Context7 for library/framework/package topics, not general questions
	isLibTopic := r.isLibraryOrFrameworkTopic(normalizedTopic)
	if !skipRemote && isLibTopic && r.toolkit != nil && r.toolkit.Context7() != nil && r.toolkit.Context7().IsConfigured() {
		logging.Researcher("Topic detected as library/framework - querying Context7 for: %s", normalizedTopic)
		atoms, err := r.toolkit.Context7().ResearchTopic(ctx, normalizedTopic, keywords)
		if err == nil && len(atoms) > 0 {
			logging.Researcher("Context7 returned %d atoms (LLM-optimized docs)", len(atoms))
			result.Atoms = append(result.Atoms, atoms...)
			result.PagesScraped++
			context7Found = true
		} else if err != nil {
			logging.Researcher("Context7 unavailable: %v (falling back to other sources)", err)
		}
	} else if !skipRemote && !isLibTopic {
		logging.Researcher("Topic '%s' detected as general question - skipping Context7", normalizedTopic)
	}

	// If Context7 returned sufficient results, skip LLM synthesis to avoid timeouts
	// Only synthesize when Context7 data is insufficient (< 10 atoms)
	if context7Found && len(result.Atoms) >= 10 {
		// Context7 gave us enough - skip slow LLM synthesis
		logging.Researcher("Context7 provided sufficient data (%d atoms), skipping LLM synthesis", len(result.Atoms))
	} else if context7Found && len(result.Atoms) >= 1 {
		// Context7 gave some data but not enough - supplement with LLM
		if r.llmClient != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				logging.Researcher("Synthesizing supplemental knowledge from LLM...")
				atoms, err := r.synthesizeKnowledgeFromLLM(ctx, normalizedTopic, keywords)
				if err == nil && len(atoms) > 0 {
					atomsChan <- atoms
				}
			}()
		}
	} else if !skipRemote {
		// Context7 not available or no results - use fallback strategies

		// Strategy 1: Check if we have a known source for this topic
		if source, ok := r.findKnowledgeSource(normalizedTopic); ok {
			logging.Researcher("Found known source: %s (type: %s)", source.Name, source.Type)
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
					logging.Researcher("Known source failed: %v", err)
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
				logging.Researcher("Synthesizing knowledge from LLM...")
				atoms, err := r.synthesizeKnowledgeFromLLM(ctx, normalizedTopic, keywords)
				if err == nil && len(atoms) > 0 {
					atomsChan <- atoms
				} else if err != nil {
					logging.Researcher("LLM synthesis warning: %v", err)
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

	// Extended research: use generateSearchURLs and fetchAndExtract for deep research
	if isDeepResearch && len(result.Atoms) < 5 && !skipRemote {
		urls := r.generateSearchURLs(normalizedTopic, keywords)
		for _, url := range urls {
			if len(result.Atoms) >= 20 { // Cap at 20 atoms
				break
			}
			atoms, err := r.fetchAndExtract(ctx, url, keywords)
			if err == nil {
				result.Atoms = append(result.Atoms, atoms...)
				result.PagesScraped++
			}
		}
	}

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

	// Autopoiesis: Track research quality
	r.trackResearchQualityFromResult(normalizedTopic, result)

	// Track source reliability for successful fetches
	for _, atom := range result.Atoms {
		if atom.SourceURL != "" && atom.Confidence >= 0.7 {
			r.trackSourceSuccess(atom.SourceURL)
		} else if atom.SourceURL != "" && atom.Confidence < 0.4 {
			r.trackSourceFailure(atom.SourceURL)
		}
	}

	// Track query failure if no results
	if len(result.Atoms) == 0 {
		r.trackQueryFailure(normalizedTopic)
	}

	return result, nil
}

// isWorkspaceQuery detects whether the topic is targeting local files/symbols.
func (r *ResearcherShard) isWorkspaceQuery(topic string) bool {
	if topic == "" {
		return false
	}

	lower := strings.ToLower(topic)

	// Direct file hints
	if strings.Contains(lower, ".go") || strings.Contains(lower, ".md") || strings.Contains(lower, ".ts") {
		return true
	}

	// Repo topology hints
	if strings.Contains(lower, "internal/") || strings.Contains(lower, "internal\\") ||
		strings.Contains(lower, "cmd/") || strings.Contains(lower, "pkg/") {
		return true
	}

	// Natural language hints
	if strings.Contains(lower, "file") && (strings.Contains(lower, "/") || strings.Contains(lower, "\\")) {
		return true
	}

	// Explicit workspace references
	if strings.Contains(lower, "workspace") || strings.Contains(lower, "directory") || strings.Contains(lower, "folder") {
		return true
	}

	return false
}

// workspaceReferenceSearch performs a ripgrep-based search in the workspace to answer local dependency questions.
func (r *ResearcherShard) workspaceReferenceSearch(ctx context.Context, topic string) ([]KnowledgeAtom, error) {
	r.mu.RLock()
	root := r.workspaceRoot
	r.mu.RUnlock()

	if root == "" {
		return nil, fmt.Errorf("workspace root not configured")
	}

	targets := r.extractWorkspaceTargets(topic)
	if len(targets) == 0 {
		return nil, nil
	}

	args := []string{"--no-heading", "--line-number", "--ignore-case"}
	for _, t := range targets {
		args = append(args, "-e", t)
	}
	args = append(args, root)

	cmd := exec.CommandContext(ctx, "rg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil && len(output) == 0 {
		if errors.Is(err, exec.ErrNotFound) {
			return r.workspaceReferenceSearchFallback(ctx, root, targets)
		}
		return nil, fmt.Errorf("rg search failed: %w", err)
	}

	return r.parseWorkspaceMatches(string(output), topic), nil
}

// workspaceReferenceSearchFallback scans files manually if rg is unavailable.
func (r *ResearcherShard) workspaceReferenceSearchFallback(ctx context.Context, root string, targets []string) ([]KnowledgeAtom, error) {
	logging.Researcher("ripgrep not found; using fallback scanner for workspace references")
	var atoms []KnowledgeAtom
	lowerTargets := make([]string, len(targets))
	for i, t := range targets {
		lowerTargets[i] = strings.ToLower(t)
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip hidden dirs to avoid slow scans
			if strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil // skip unreadable files
		}
		contentLower := strings.ToLower(string(data))
		for _, target := range lowerTargets {
			if strings.Contains(contentLower, target) {
				snippet := r.truncate(strings.TrimSpace(string(data)), 120)
				atoms = append(atoms, KnowledgeAtom{
					SourceURL:   path,
					Title:       fmt.Sprintf("Workspace reference: %s", target),
					Content:     snippet,
					Concept:     "workspace_reference",
					Confidence:  0.8,
					ExtractedAt: time.Now(),
				})
				if len(atoms) >= 50 {
					return context.Canceled // stop early, treat as completion
				}
				break
			}
		}
		return nil
	})

	if err != nil && !errors.Is(err, context.Canceled) {
		return atoms, err
	}
	return atoms, nil
}

// parseWorkspaceMatches converts ripgrep output into knowledge atoms.
func (r *ResearcherShard) parseWorkspaceMatches(output, topic string) []KnowledgeAtom {
	lines := strings.Split(output, "\n")
	atoms := make([]KnowledgeAtom, 0, len(lines))
	targets := r.extractWorkspaceTargets(topic)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		path := parts[0]
		lineNum := parts[1]
		snippet := strings.TrimSpace(parts[2])
		title := fmt.Sprintf("Workspace reference: %s", filepath.Base(path))

		if len(targets) > 0 {
			title = fmt.Sprintf("Workspace reference: %s", targets[0])
		}

		atoms = append(atoms, KnowledgeAtom{
			SourceURL:   path,
			Title:       title,
			Content:     fmt.Sprintf("%s:%s %s", path, lineNum, snippet),
			Concept:     "workspace_reference",
			Confidence:  0.9,
			ExtractedAt: time.Now(),
			Metadata: map[string]interface{}{
				"line":  lineNum,
				"query": topic,
			},
		})

		if len(atoms) >= 50 {
			break // avoid flooding downstream logic
		}
	}

	return atoms
}

// extractWorkspaceTargets extracts filenames or symbols from the topic.
func (r *ResearcherShard) extractWorkspaceTargets(topic string) []string {
	re := regexp.MustCompile(`[\w.\-/\\]+\\.[A-Za-z0-9]+`)
	matches := re.FindAllString(topic, -1)
	seen := make(map[string]bool)
	var targets []string

	for _, m := range matches {
		base := filepath.Base(filepath.ToSlash(strings.TrimSpace(m)))
		if base == "" {
			continue
		}
		if !seen[base] {
			seen[base] = true
			targets = append(targets, base)
		}
		// Also add basename without extension for broader matches
		if ext := filepath.Ext(base); ext != "" {
			name := strings.TrimSuffix(base, ext)
			if name != "" && !seen[name] {
				seen[name] = true
				targets = append(targets, name)
			}
		}
	}

	// Fallback: use longest token if no explicit filename found
	if len(targets) == 0 {
		fields := strings.FieldsFunc(topic, func(r rune) bool {
			return r == ' ' || r == ',' || r == ';'
		})
		longest := ""
		for _, f := range fields {
			if len(f) > len(longest) && len(f) >= 5 {
				longest = f
			}
		}
		if longest != "" {
			targets = append(targets, longest)
		}
	}

	return targets
}

// fetchGitHubDocs fetches README and docs from GitHub using raw URLs (no API key needed)
// Implements Context7-like multi-stage ingestion:
// 1. Check for llms.txt (AI-optimized docs pointer)
// 2. Fetch and parse documentation
// 3. Enrich with LLM metadata
// 4. Score content quality
func (r *ResearcherShard) fetchGitHubDocs(ctx context.Context, source KnowledgeSource, keywords []string) ([]KnowledgeAtom, error) {
	var atoms []KnowledgeAtom

	// Use keywords for filtering if provided
	keywordFilter := strings.Join(keywords, " ")
	if keywordFilter != "" {
		logging.Researcher("Fetching GitHub docs with keyword filter: %s", keywordFilter)
	}

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
			logging.Researcher("Found llms.txt at %s - using AI-optimized docs", url)
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
			logging.Researcher("Fetched README from %s (%d bytes)", url, len(content))
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
				logging.Researcher("Discarding low-quality atom: %s (score: %.2f)", readmeAtoms[i].Title, score)
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
			logging.Researcher("llms.txt pointer failed: %s - %v", docURL, err)
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
			logging.Researcher("Ingested llms.txt doc: %s (%d bytes)", docURL, len(content))
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

	// Extract sections (## headings)
	// Go regex doesn't support lookaheads, so we split by "\n## " manually
	// Normalize line endings first
	normalized := strings.ReplaceAll(content, "\r\n", "\n")

	// Split by "## " at start of line (or start of file)
	// We add a newline prefix to match the loop logic easier if it starts with ##
	if strings.HasPrefix(normalized, "## ") {
		normalized = "\n" + normalized
	}

	sections := strings.Split(normalized, "\n## ")

	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}

		// First line is title, rest is content
		parts := strings.SplitN(section, "\n", 2)
		title := strings.TrimSpace(parts[0])

		if len(parts) < 2 {
			continue // Skip if no content
		}

		body := strings.TrimSpace(parts[1])

		if len(body) > 50 && len(body) < 3000 {
			// Skip common non-informative sections
			lowerTitle := strings.ToLower(title)
			if lowerTitle == "license" || lowerTitle == "contributing" || lowerTitle == "changelog" || lowerTitle == "badges" {
				continue
			}

			atoms = append(atoms, KnowledgeAtom{
				Title:       title,
				Content:     r.truncate(body, 1000),
				Concept:     "documentation_section",
				Confidence:  0.85,
				ExtractedAt: time.Now(),
			})
		}
	}

	return atoms
}

// fetchPkgGoDev fetches documentation from pkg.go.dev
func (r *ResearcherShard) fetchPkgGoDev(ctx context.Context, source KnowledgeSource) ([]KnowledgeAtom, error) {
	// pkg.go.dev doesn't have a public API, so we fall back to GitHub
	return r.fetchGitHubDocs(ctx, source, nil)
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
		Overview       string   `json:"overview"`
		KeyConcepts    []string `json:"key_concepts"`
		BestPractices  []string `json:"best_practices"`
		CommonPatterns []struct {
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

// persistKnowledge saves knowledge atoms to the local database.
// Also generates and stores prompt atoms for JIT prompt compilation.
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

	// Generate and store prompt atoms from research results
	// This creates domain-specific prompt fragments for JIT compilation
	ctx := context.Background()
	if err := r.generateAndStorePromptAtoms(ctx, result); err != nil {
		logging.Get(logging.CategoryResearcher).Warn("Failed to generate prompt atoms: %v", err)
		// Non-fatal: continue with normal persistence
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

	// Autopoiesis: Promote learned patterns to long-term memory
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Promote high-quality topics
	for topic, quality := range r.qualityScores {
		if quality >= 0.7 {
			facts = append(facts, core.Fact{
				Predicate: "promote_to_long_term",
				Args:      []interface{}{"high_quality_topic", topic, quality},
			})
		}
	}

	// Promote reliable sources
	for source, count := range r.sourceReliability {
		if count >= 3 {
			facts = append(facts, core.Fact{
				Predicate: "promote_to_long_term",
				Args:      []interface{}{"reliable_source", source, count},
			})
		}
	}

	// Note unreliable sources
	for source, count := range r.sourceFailures {
		if count >= 2 {
			facts = append(facts, core.Fact{
				Predicate: "promote_to_long_term",
				Args:      []interface{}{"unreliable_source", source, count},
			})
		}
	}

	return facts
}
