// Package shards implements research tools for the ResearcherShard.
// These tools provide real research capabilities: browser automation,
// web search, GitHub API, and caching.
package shards

import (
	"codenerd/internal/browser"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ResearchTool defines the interface for research tools.
type ResearchTool interface {
	Name() string
	Execute(ctx context.Context, params map[string]interface{}) ([]KnowledgeAtom, error)
}

// ResearchToolkit bundles all research tools for the ResearcherShard.
type ResearchToolkit struct {
	mu sync.RWMutex

	// Tools
	browserTool  *BrowserResearchTool
	searchTool   *WebSearchTool
	githubTool   *GitHubResearchTool
	context7Tool *Context7Tool
	cacheTool    *ResearchCache

	// Configuration
	httpClient *http.Client
	cacheDir   string
	userAgent  string
}

// NewResearchToolkit creates a new toolkit with all research tools.
func NewResearchToolkit(cacheDir string) *ResearchToolkit {
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "codenerd-research-cache")
	}
	_ = os.MkdirAll(cacheDir, 0755)

	toolkit := &ResearchToolkit{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		cacheDir:   cacheDir,
		userAgent:  "codeNERD/1.5.0 ResearchAgent (+https://github.com/codenerd)",
	}

	toolkit.cacheTool = NewResearchCache(cacheDir)
	toolkit.githubTool = NewGitHubResearchTool(toolkit.httpClient, toolkit.cacheTool, toolkit.userAgent)
	toolkit.searchTool = NewWebSearchTool(toolkit.httpClient, toolkit.cacheTool, toolkit.userAgent)
	toolkit.context7Tool = NewContext7Tool(toolkit.httpClient, toolkit.cacheTool)

	return toolkit
}

// SetBrowserManager sets the browser session manager for browser-based research.
func (t *ResearchToolkit) SetBrowserManager(mgr *browser.SessionManager) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.browserTool = NewBrowserResearchTool(mgr, t.cacheTool)
}

// GitHub returns the GitHub research tool.
func (t *ResearchToolkit) GitHub() *GitHubResearchTool {
	return t.githubTool
}

// Search returns the web search tool.
func (t *ResearchToolkit) Search() *WebSearchTool {
	return t.searchTool
}

// Browser returns the browser research tool.
func (t *ResearchToolkit) Browser() *BrowserResearchTool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.browserTool
}

// Cache returns the research cache.
func (t *ResearchToolkit) Cache() *ResearchCache {
	return t.cacheTool
}

// Context7 returns the Context7 research tool.
func (t *ResearchToolkit) Context7() *Context7Tool {
	return t.context7Tool
}

// SetContext7APIKey sets the API key for the Context7 tool.
func (t *ResearchToolkit) SetContext7APIKey(key string) {
	if t.context7Tool != nil && key != "" {
		t.context7Tool.SetAPIKey(key)
	}
}

// =============================================================================
// RESEARCH CACHE
// =============================================================================

// CachedContent represents cached research content.
type CachedContent struct {
	URL       string    `json:"url"`
	Content   string    `json:"content"`
	FetchedAt time.Time `json:"fetched_at"`
	TTL       int       `json:"ttl_seconds"`
}

// ResearchCache provides caching for research fetches.
type ResearchCache struct {
	mu       sync.RWMutex
	cacheDir string
	memory   map[string]*CachedContent // In-memory hot cache
}

// NewResearchCache creates a new research cache.
func NewResearchCache(cacheDir string) *ResearchCache {
	return &ResearchCache{
		cacheDir: cacheDir,
		memory:   make(map[string]*CachedContent),
	}
}

// cacheKey generates a cache key from URL.
func (c *ResearchCache) cacheKey(url string) string {
	h := sha256.Sum256([]byte(url))
	return hex.EncodeToString(h[:16])
}

// Get retrieves cached content if available and not expired.
func (c *ResearchCache) Get(url string) (string, bool) {
	c.mu.RLock()
	if cached, ok := c.memory[url]; ok {
		if time.Since(cached.FetchedAt) < time.Duration(cached.TTL)*time.Second {
			c.mu.RUnlock()
			return cached.Content, true
		}
	}
	c.mu.RUnlock()

	// Check disk cache
	key := c.cacheKey(url)
	path := filepath.Join(c.cacheDir, key+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	var cached CachedContent
	if err := json.Unmarshal(data, &cached); err != nil {
		return "", false
	}

	if time.Since(cached.FetchedAt) >= time.Duration(cached.TTL)*time.Second {
		return "", false
	}

	// Promote to memory cache
	c.mu.Lock()
	c.memory[url] = &cached
	c.mu.Unlock()

	return cached.Content, true
}

// Set stores content in cache.
func (c *ResearchCache) Set(url, content string, ttlSeconds int) {
	cached := &CachedContent{
		URL:       url,
		Content:   content,
		FetchedAt: time.Now(),
		TTL:       ttlSeconds,
	}

	// Store in memory
	c.mu.Lock()
	c.memory[url] = cached
	c.mu.Unlock()

	// Store on disk
	key := c.cacheKey(url)
	path := filepath.Join(c.cacheDir, key+".json")
	data, _ := json.Marshal(cached)
	_ = os.WriteFile(path, data, 0644)
}

// =============================================================================
// GITHUB RESEARCH TOOL
// =============================================================================

// GitHubResearchTool fetches documentation from GitHub repositories.
type GitHubResearchTool struct {
	client    *http.Client
	cache     *ResearchCache
	userAgent string
}

// NewGitHubResearchTool creates a new GitHub research tool.
func NewGitHubResearchTool(client *http.Client, cache *ResearchCache, userAgent string) *GitHubResearchTool {
	return &GitHubResearchTool{
		client:    client,
		cache:     cache,
		userAgent: userAgent,
	}
}

func (g *GitHubResearchTool) Name() string { return "github" }

// FetchRepository fetches comprehensive documentation from a GitHub repo.
func (g *GitHubResearchTool) FetchRepository(ctx context.Context, owner, repo string, keywords []string) ([]KnowledgeAtom, error) {
	var atoms []KnowledgeAtom
	var wg sync.WaitGroup
	var mu sync.Mutex
	errCh := make(chan error, 10)

	// Fetch in parallel: llms.txt, README, docs/, examples/
	fetchTasks := []struct {
		name string
		urls []string
	}{
		{
			name: "llms.txt",
			urls: []string{
				fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/llms.txt", owner, repo),
				fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/master/llms.txt", owner, repo),
				fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/.llms.txt", owner, repo),
			},
		},
		{
			name: "readme",
			urls: []string{
				fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/README.md", owner, repo),
				fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/master/README.md", owner, repo),
			},
		},
		{
			name: "docs",
			urls: []string{
				fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/docs/README.md", owner, repo),
				fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/docs/getting-started.md", owner, repo),
				fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/GETTING_STARTED.md", owner, repo),
			},
		},
		{
			name: "examples",
			urls: []string{
				fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/examples/README.md", owner, repo),
				fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/_examples/README.md", owner, repo),
			},
		},
	}

	for _, task := range fetchTasks {
		task := task
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, url := range task.urls {
				content, err := g.fetchURL(ctx, url)
				if err != nil || len(content) < 50 {
					continue
				}

				var taskAtoms []KnowledgeAtom
				switch task.name {
				case "llms.txt":
					taskAtoms = g.parseLlmsTxt(ctx, owner, repo, content)
				case "readme":
					taskAtoms = g.parseReadme(owner+"/"+repo, content, url)
				case "docs", "examples":
					taskAtoms = []KnowledgeAtom{{
						SourceURL:   url,
						Title:       fmt.Sprintf("%s/%s %s", owner, repo, task.name),
						Content:     truncateContent(content, 2000),
						Concept:     "documentation",
						Confidence:  0.85,
						ExtractedAt: time.Now(),
					}}
				}

				mu.Lock()
				atoms = append(atoms, taskAtoms...)
				mu.Unlock()
				break // Got content from one URL, skip others
			}
		}()
	}

	wg.Wait()
	close(errCh)

	return atoms, nil
}

// fetchURL fetches content from a URL with caching.
func (g *GitHubResearchTool) fetchURL(ctx context.Context, url string) (string, error) {
	// Check cache first
	if cached, ok := g.cache.Get(url); ok {
		return cached, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", g.userAgent)

	resp, err := g.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 500*1024))
	if err != nil {
		return "", err
	}

	content := string(body)

	// Cache for 1 hour
	g.cache.Set(url, content, 3600)

	return content, nil
}

// parseLlmsTxt parses llms.txt content and fetches linked docs.
func (g *GitHubResearchTool) parseLlmsTxt(ctx context.Context, owner, repo, content string) []KnowledgeAtom {
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
			docURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/%s",
				owner, repo, strings.TrimPrefix(line, "/"))
		}

		docContent, err := g.fetchURL(ctx, docURL)
		if err != nil || len(docContent) < 50 {
			continue
		}

		atoms = append(atoms, KnowledgeAtom{
			SourceURL:   docURL,
			Title:       "AI-Optimized Documentation (llms.txt)",
			Content:     truncateContent(docContent, 3000),
			Concept:     "llms_optimized",
			Confidence:  0.95, // Highest confidence for llms.txt
			ExtractedAt: time.Now(),
			Metadata: map[string]interface{}{
				"source_type": "llms_txt",
				"repo":        owner + "/" + repo,
			},
		})
	}

	return atoms
}

// parseReadme parses README content into knowledge atoms.
func (g *GitHubResearchTool) parseReadme(repoName, content, sourceURL string) []KnowledgeAtom {
	var atoms []KnowledgeAtom

	// Extract overview (first paragraph after heading)
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
			SourceURL:   sourceURL,
			Title:       repoName + " Overview",
			Content:     strings.TrimSpace(description.String()),
			Concept:     "overview",
			Confidence:  0.95,
			ExtractedAt: time.Now(),
		})
	}

	// Extract code examples
	codeBlockRegex := regexp.MustCompile("(?s)```(?:go|golang|python|javascript|typescript|rust)?\\s*\\n(.+?)```")
	matches := codeBlockRegex.FindAllStringSubmatch(content, 5)
	for i, match := range matches {
		if len(match) > 1 && len(match[1]) > 20 && len(match[1]) < 2000 {
			atoms = append(atoms, KnowledgeAtom{
				SourceURL:   sourceURL,
				Title:       fmt.Sprintf("%s Code Example %d", repoName, i+1),
				Content:     "Code example from documentation",
				CodePattern: strings.TrimSpace(match[1]),
				Concept:     "code_example",
				Confidence:  0.9,
				ExtractedAt: time.Now(),
			})
		}
	}

	// Extract sections
	sectionRegex := regexp.MustCompile(`(?m)^##\s+(.+?)\n([\s\S]*?)(?=^##|\z)`)
	sectionMatches := sectionRegex.FindAllStringSubmatch(content, 10)
	for _, match := range sectionMatches {
		if len(match) > 2 {
			sectionTitle := strings.TrimSpace(match[1])
			sectionContent := strings.TrimSpace(match[2])

			// Skip non-informative sections
			lowerTitle := strings.ToLower(sectionTitle)
			if lowerTitle == "license" || lowerTitle == "contributing" || lowerTitle == "changelog" {
				continue
			}

			if len(sectionContent) > 50 && len(sectionContent) < 3000 {
				atoms = append(atoms, KnowledgeAtom{
					SourceURL:   sourceURL,
					Title:       sectionTitle,
					Content:     truncateContent(sectionContent, 1000),
					Concept:     "documentation_section",
					Confidence:  0.85,
					ExtractedAt: time.Now(),
				})
			}
		}
	}

	return atoms
}

// =============================================================================
// WEB SEARCH TOOL
// =============================================================================

// WebSearchTool provides web search capabilities.
type WebSearchTool struct {
	client    *http.Client
	cache     *ResearchCache
	userAgent string
}

// NewWebSearchTool creates a new web search tool.
func NewWebSearchTool(client *http.Client, cache *ResearchCache, userAgent string) *WebSearchTool {
	return &WebSearchTool{
		client:    client,
		cache:     cache,
		userAgent: userAgent,
	}
}

func (w *WebSearchTool) Name() string { return "search" }

// SearchResult represents a search result.
type SearchResult struct {
	Title   string
	URL     string
	Snippet string
}

// Search performs a web search using DuckDuckGo HTML.
func (w *WebSearchTool) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	if maxResults <= 0 {
		maxResults = 10
	}

	// Use DuckDuckGo HTML (no API key needed)
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	// Check cache
	if cached, ok := w.cache.Get(searchURL); ok {
		return w.parseSearchResults(cached, maxResults), nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", w.userAgent)

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 500*1024))
	if err != nil {
		return nil, err
	}

	content := string(body)
	w.cache.Set(searchURL, content, 1800) // 30 min cache

	return w.parseSearchResults(content, maxResults), nil
}

// parseSearchResults extracts search results from DuckDuckGo HTML.
func (w *WebSearchTool) parseSearchResults(html string, maxResults int) []SearchResult {
	var results []SearchResult

	// Extract result links and titles using regex
	// DuckDuckGo HTML results are in <a class="result__a" href="...">title</a>
	linkRegex := regexp.MustCompile(`<a[^>]*class="result__a"[^>]*href="([^"]+)"[^>]*>([^<]+)</a>`)
	snippetRegex := regexp.MustCompile(`<a[^>]*class="result__snippet"[^>]*>([^<]+)</a>`)

	linkMatches := linkRegex.FindAllStringSubmatch(html, maxResults*2)
	snippetMatches := snippetRegex.FindAllStringSubmatch(html, maxResults*2)

	for i, match := range linkMatches {
		if len(match) < 3 || len(results) >= maxResults {
			continue
		}

		rawURL := match[1]
		title := strings.TrimSpace(match[2])

		// DuckDuckGo uses redirect URLs, extract actual URL
		if strings.Contains(rawURL, "uddg=") {
			if parsed, err := url.Parse(rawURL); err == nil {
				if uddg := parsed.Query().Get("uddg"); uddg != "" {
					rawURL = uddg
				}
			}
		}

		snippet := ""
		if i < len(snippetMatches) && len(snippetMatches[i]) > 1 {
			snippet = strings.TrimSpace(snippetMatches[i][1])
		}

		results = append(results, SearchResult{
			Title:   title,
			URL:     rawURL,
			Snippet: snippet,
		})
	}

	return results
}

// SearchAndFetch searches and fetches content from top results.
func (w *WebSearchTool) SearchAndFetch(ctx context.Context, query string, maxResults int) ([]KnowledgeAtom, error) {
	results, err := w.Search(ctx, query, maxResults)
	if err != nil {
		return nil, err
	}

	var atoms []KnowledgeAtom
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Limit concurrent fetches
	semaphore := make(chan struct{}, 3)

	for _, result := range results {
		result := result
		wg.Add(1)
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			content, err := w.fetchPage(ctx, result.URL)
			if err != nil || len(content) < 100 {
				return
			}

			// Extract main content (simplified)
			mainContent := extractMainContent(content)
			if len(mainContent) < 100 {
				return
			}

			mu.Lock()
			atoms = append(atoms, KnowledgeAtom{
				SourceURL:   result.URL,
				Title:       result.Title,
				Content:     truncateContent(mainContent, 1500),
				Concept:     "web_research",
				Confidence:  0.75,
				ExtractedAt: time.Now(),
				Metadata: map[string]interface{}{
					"snippet": result.Snippet,
				},
			})
			mu.Unlock()
		}()
	}

	wg.Wait()
	return atoms, nil
}

// fetchPage fetches a web page.
func (w *WebSearchTool) fetchPage(ctx context.Context, pageURL string) (string, error) {
	if cached, ok := w.cache.Get(pageURL); ok {
		return cached, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", w.userAgent)

	resp, err := w.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return "", err
	}

	content := string(body)
	w.cache.Set(pageURL, content, 3600) // 1 hour cache

	return content, nil
}

// =============================================================================
// BROWSER RESEARCH TOOL
// =============================================================================

// BrowserResearchTool uses browser automation for dynamic content.
type BrowserResearchTool struct {
	manager *browser.SessionManager
	cache   *ResearchCache
}

// NewBrowserResearchTool creates a new browser research tool.
func NewBrowserResearchTool(manager *browser.SessionManager, cache *ResearchCache) *BrowserResearchTool {
	return &BrowserResearchTool{
		manager: manager,
		cache:   cache,
	}
}

func (b *BrowserResearchTool) Name() string { return "browser" }

// FetchDynamicPage fetches a page that requires JavaScript rendering.
func (b *BrowserResearchTool) FetchDynamicPage(ctx context.Context, pageURL string) ([]KnowledgeAtom, error) {
	if b.manager == nil {
		return nil, fmt.Errorf("browser not available")
	}

	// Check cache first
	if cached, ok := b.cache.Get(pageURL); ok {
		return []KnowledgeAtom{{
			SourceURL:   pageURL,
			Title:       "Cached Dynamic Content",
			Content:     cached,
			Concept:     "browser_fetched",
			Confidence:  0.85,
			ExtractedAt: time.Now(),
		}}, nil
	}

	// Ensure browser is connected
	if !b.manager.IsConnected() {
		if err := b.manager.Start(ctx); err != nil {
			return nil, fmt.Errorf("failed to start browser: %w", err)
		}
	}

	// Create session and navigate
	session, err := b.manager.CreateSession(ctx, pageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Wait for page to load
	page, ok := b.manager.Page(session.ID)
	if !ok {
		return nil, fmt.Errorf("failed to get page")
	}

	// Wait for dynamic content
	_ = page.WaitStable(2 * time.Second)

	// Extract text content
	content, err := page.Eval(`() => document.body.innerText`)
	if err != nil {
		return nil, err
	}

	// Get title
	title := ""
	titleRes, _ := page.Eval(`() => document.title`)
	if titleRes != nil && !titleRes.Value.Nil() {
		title = titleRes.Value.String()
	}

	textContent := ""
	if content != nil && !content.Value.Nil() {
		textContent = content.Value.String()
	}

	// Cache the content
	b.cache.Set(pageURL, textContent, 3600)

	return []KnowledgeAtom{{
		SourceURL:   pageURL,
		Title:       title,
		Content:     truncateContent(textContent, 3000),
		Concept:     "browser_fetched",
		Confidence:  0.85,
		ExtractedAt: time.Now(),
		Metadata: map[string]interface{}{
			"rendered": true,
		},
	}}, nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// truncateContent truncates content to maxLen.
func truncateContent(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// extractMainContent extracts main content from HTML (simplified).
func extractMainContent(html string) string {
	// Remove scripts and styles
	scriptRegex := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleRegex := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = scriptRegex.ReplaceAllString(html, "")
	html = styleRegex.ReplaceAllString(html, "")

	// Try to find main content areas
	mainPatterns := []string{
		`(?is)<main[^>]*>(.*?)</main>`,
		`(?is)<article[^>]*>(.*?)</article>`,
		`(?is)<div[^>]*class="[^"]*content[^"]*"[^>]*>(.*?)</div>`,
		`(?is)<div[^>]*id="content"[^>]*>(.*?)</div>`,
	}

	for _, pattern := range mainPatterns {
		re := regexp.MustCompile(pattern)
		if match := re.FindStringSubmatch(html); len(match) > 1 {
			html = match[1]
			break
		}
	}

	// Strip remaining HTML tags
	tagRegex := regexp.MustCompile(`<[^>]+>`)
	text := tagRegex.ReplaceAllString(html, " ")

	// Clean up whitespace
	spaceRegex := regexp.MustCompile(`\s+`)
	text = spaceRegex.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

// =============================================================================
// CONTEXT7 TOOL - Primary LLM-optimized documentation source
// API Reference: https://context7.com/docs/api-reference
// =============================================================================

// Context7Tool fetches curated, LLM-optimized documentation from Context7.
// This is the preferred source for developer documentation as it's specifically
// designed for AI consumption - no web scraping noise.
type Context7Tool struct {
	client  *http.Client
	cache   *ResearchCache
	baseURL string
	apiKey  string
}

// Context7SearchResult represents a library from search results.
// GET /v2/search
type Context7SearchResult struct {
	ID             string   `json:"id"`             // e.g., "/vercel/next.js"
	Title          string   `json:"title"`          // e.g., "Next.js"
	Description    string   `json:"description"`    // Short summary
	Branch         string   `json:"branch"`         // Git branch tracked
	LastUpdateDate string   `json:"lastUpdateDate"` // ISO 8601 timestamp
	State          string   `json:"state"`          // finalized, initial, processing, error, delete
	TotalTokens    int      `json:"totalTokens"`    // Documentation token count
	TotalSnippets  int      `json:"totalSnippets"`  // Code snippet count
	Stars          int      `json:"stars"`          // GitHub stars
	TrustScore     int      `json:"trustScore"`     // Reputation 0-10
	BenchmarkScore float64  `json:"benchmarkScore"` // Quality 0-100
	Versions       []string `json:"versions"`       // Available version tags
}

// Context7SearchResponse represents the search API response.
type Context7SearchResponse struct {
	Results  []Context7SearchResult `json:"results"`
	Metadata struct {
		Authentication string `json:"authentication"` // none, personal, team
	} `json:"metadata"`
}

// Context7CodeSnippet represents a code example from /v2/docs/code/{owner}/{repo}
type Context7CodeSnippet struct {
	CodeTitle       string `json:"codeTitle"`
	CodeDescription string `json:"codeDescription"`
	CodeLanguage    string `json:"codeLanguage"`
	CodeTokens      int    `json:"codeTokens"`
	CodeID          string `json:"codeId"`   // GitHub URL to source
	PageTitle       string `json:"pageTitle"` // Parent page title
	CodeList        []struct {
		Language string `json:"language"`
		Code     string `json:"code"`
	} `json:"codeList"`
}

// Context7CodeResponse represents code snippets API response.
// GET /v2/docs/code/{owner}/{repo}
type Context7CodeResponse struct {
	Snippets    []Context7CodeSnippet `json:"snippets"`
	TotalTokens int                   `json:"totalTokens"`
	Pagination  struct {
		Page       int  `json:"page"`
		Limit      int  `json:"limit"`
		TotalPages int  `json:"totalPages"`
		HasNext    bool `json:"hasNext"`
		HasPrev    bool `json:"hasPrev"`
	} `json:"pagination"`
	Metadata struct {
		Authentication string `json:"authentication"`
	} `json:"metadata"`
}

// Context7InfoSnippet represents documentation content from /v2/docs/info/{owner}/{repo}
type Context7InfoSnippet struct {
	PageID        string `json:"pageId"`        // Unique page identifier URL
	Breadcrumb    string `json:"breadcrumb"`    // Navigation path
	Content       string `json:"content"`       // Documentation text
	ContentTokens int    `json:"contentTokens"` // Token count
}

// Context7InfoResponse represents documentation content API response.
// GET /v2/docs/info/{owner}/{repo}
type Context7InfoResponse struct {
	Snippets    []Context7InfoSnippet `json:"snippets"`
	TotalTokens int                   `json:"totalTokens"`
	Pagination  struct {
		Page       int  `json:"page"`
		Limit      int  `json:"limit"`
		TotalPages int  `json:"totalPages"`
		HasNext    bool `json:"hasNext"`
		HasPrev    bool `json:"hasPrev"`
	} `json:"pagination"`
	Metadata struct {
		Authentication string `json:"authentication"`
	} `json:"metadata"`
}

// NewContext7Tool creates a new Context7 research tool.
func NewContext7Tool(client *http.Client, cache *ResearchCache) *Context7Tool {
	apiKey := os.Getenv("CONTEXT7_API_KEY")
	return &Context7Tool{
		client:  client,
		cache:   cache,
		baseURL: "https://context7.com/api/v2",
		apiKey:  apiKey,
	}
}

// SetAPIKey sets the Context7 API key.
func (c *Context7Tool) SetAPIKey(key string) {
	c.apiKey = key
}

// IsConfigured returns true if the API key is set.
func (c *Context7Tool) IsConfigured() bool {
	return c.apiKey != ""
}

func (c *Context7Tool) Name() string { return "context7" }

// Search searches Context7 for relevant libraries/frameworks.
func (c *Context7Tool) Search(ctx context.Context, query string) ([]Context7SearchResult, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("CONTEXT7_API_KEY not configured")
	}

	// Check cache
	cacheKey := "context7:search:" + query
	if cached, ok := c.cache.Get(cacheKey); ok {
		var results []Context7SearchResult
		if err := json.Unmarshal([]byte(cached), &results); err == nil {
			return results, nil
		}
	}

	reqURL := fmt.Sprintf("%s/search?query=%s", c.baseURL, url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Context7 search failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var searchResp Context7SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode Context7 response: %w", err)
	}

	// Cache results
	if data, err := json.Marshal(searchResp.Results); err == nil {
		c.cache.Set(cacheKey, string(data), 3600) // 1 hour
	}

	return searchResp.Results, nil
}

// FetchCodeSnippets fetches code examples from /v2/docs/code/{owner}/{repo}
func (c *Context7Tool) FetchCodeSnippets(ctx context.Context, owner, repo, topic string) ([]KnowledgeAtom, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("CONTEXT7_API_KEY not configured")
	}

	// Check cache
	cacheKey := fmt.Sprintf("context7:code:%s/%s:%s", owner, repo, topic)
	if cached, ok := c.cache.Get(cacheKey); ok {
		var atoms []KnowledgeAtom
		if err := json.Unmarshal([]byte(cached), &atoms); err == nil {
			return atoms, nil
		}
	}

	// Build request URL - use JSON to get structured snippets
	reqURL := fmt.Sprintf("%s/docs/code/%s/%s?type=json&limit=10", c.baseURL, owner, repo)
	if topic != "" {
		reqURL += "&topic=" + url.QueryEscape(topic)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Context7 code failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var codeResp Context7CodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&codeResp); err != nil {
		return nil, fmt.Errorf("failed to decode Context7 code response: %w", err)
	}

	// Convert snippets to KnowledgeAtoms
	var atoms []KnowledgeAtom
	for _, snippet := range codeResp.Snippets {
		// Build content from code list
		var codeContent strings.Builder
		codeContent.WriteString(snippet.CodeDescription + "\n\n")
		for _, code := range snippet.CodeList {
			codeContent.WriteString(fmt.Sprintf("```%s\n%s\n```\n\n", code.Language, code.Code))
		}

		atoms = append(atoms, KnowledgeAtom{
			SourceURL:   snippet.CodeID,
			Title:       snippet.CodeTitle,
			Content:     codeContent.String(),
			Concept:     "context7_code",
			Confidence:  0.95,
			ExtractedAt: time.Now(),
			Metadata: map[string]interface{}{
				"source":    "context7",
				"owner":     owner,
				"repo":      repo,
				"topic":     topic,
				"language":  snippet.CodeLanguage,
				"pageTitle": snippet.PageTitle,
				"tokens":    snippet.CodeTokens,
			},
		})
	}

	// Cache results
	if len(atoms) > 0 {
		if data, err := json.Marshal(atoms); err == nil {
			c.cache.Set(cacheKey, string(data), 7200) // 2 hours
		}
	}

	return atoms, nil
}

// FetchDocContent fetches documentation text from /v2/docs/info/{owner}/{repo}
func (c *Context7Tool) FetchDocContent(ctx context.Context, owner, repo, topic string) ([]KnowledgeAtom, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("CONTEXT7_API_KEY not configured")
	}

	// Check cache
	cacheKey := fmt.Sprintf("context7:info:%s/%s:%s", owner, repo, topic)
	if cached, ok := c.cache.Get(cacheKey); ok {
		var atoms []KnowledgeAtom
		if err := json.Unmarshal([]byte(cached), &atoms); err == nil {
			return atoms, nil
		}
	}

	// Build request URL
	reqURL := fmt.Sprintf("%s/docs/info/%s/%s?type=json&limit=10", c.baseURL, owner, repo)
	if topic != "" {
		reqURL += "&topic=" + url.QueryEscape(topic)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Context7 info failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var infoResp Context7InfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&infoResp); err != nil {
		return nil, fmt.Errorf("failed to decode Context7 info response: %w", err)
	}

	// Convert snippets to KnowledgeAtoms
	var atoms []KnowledgeAtom
	for _, snippet := range infoResp.Snippets {
		atoms = append(atoms, KnowledgeAtom{
			SourceURL:   snippet.PageID,
			Title:       snippet.Breadcrumb,
			Content:     snippet.Content,
			Concept:     "context7_docs",
			Confidence:  0.95,
			ExtractedAt: time.Now(),
			Metadata: map[string]interface{}{
				"source": "context7",
				"owner":  owner,
				"repo":   repo,
				"topic":  topic,
				"tokens": snippet.ContentTokens,
			},
		})
	}

	// Cache results
	if len(atoms) > 0 {
		if data, err := json.Marshal(atoms); err == nil {
			c.cache.Set(cacheKey, string(data), 7200) // 2 hours
		}
	}

	return atoms, nil
}

// ResearchTopic searches Context7 for a topic and fetches relevant docs.
// This is the main entry point for topic-based research.
func (c *Context7Tool) ResearchTopic(ctx context.Context, topic string, keywords []string) ([]KnowledgeAtom, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("CONTEXT7_API_KEY not configured")
	}

	var allAtoms []KnowledgeAtom

	// Search for relevant libraries
	searchQuery := topic
	if len(keywords) > 0 && len(keywords) <= 3 {
		searchQuery = topic + " " + strings.Join(keywords, " ")
	} else if len(keywords) > 3 {
		searchQuery = topic + " " + strings.Join(keywords[:3], " ")
	}

	results, err := c.Search(ctx, searchQuery)
	if err != nil {
		return nil, fmt.Errorf("Context7 search failed: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no Context7 results for: %s", searchQuery)
	}

	// Fetch docs from top result only (most relevant)
	// Context7 search already ranks by relevance
	result := results[0]

	// Only use finalized libraries
	if result.State != "finalized" {
		if len(results) > 1 {
			result = results[1]
		} else {
			return nil, fmt.Errorf("no finalized Context7 library for: %s", searchQuery)
		}
	}

	// Parse owner/repo from ID (e.g., "/vercel/next.js" -> "vercel", "next.js")
	owner, repo := c.parseLibraryID(result.ID)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("invalid Context7 library ID: %s", result.ID)
	}

	fmt.Printf("[Context7] Found: %s (%d snippets, %d tokens, score: %.1f)\n",
		result.Title, result.TotalSnippets, result.TotalTokens, result.BenchmarkScore)

	// Build topic filter from keywords
	topicFilter := ""
	if len(keywords) > 0 {
		topicFilter = strings.Join(keywords, " ")
	}

	// Fetch both code snippets and documentation content in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	errCh := make(chan error, 2)

	// Fetch code snippets
	wg.Add(1)
	go func() {
		defer wg.Done()
		atoms, err := c.FetchCodeSnippets(ctx, owner, repo, topicFilter)
		if err != nil {
			errCh <- err
			return
		}
		mu.Lock()
		allAtoms = append(allAtoms, atoms...)
		mu.Unlock()
	}()

	// Fetch documentation content
	wg.Add(1)
	go func() {
		defer wg.Done()
		atoms, err := c.FetchDocContent(ctx, owner, repo, topicFilter)
		if err != nil {
			errCh <- err
			return
		}
		mu.Lock()
		allAtoms = append(allAtoms, atoms...)
		mu.Unlock()
	}()

	wg.Wait()
	close(errCh)

	// Log any errors but don't fail if we got some results
	for err := range errCh {
		fmt.Printf("[Context7] Warning: %v\n", err)
	}

	if len(allAtoms) == 0 {
		return nil, fmt.Errorf("no Context7 content for %s/%s", owner, repo)
	}

	return allAtoms, nil
}

// parseLibraryID parses a Context7 library ID like "/vercel/next.js" into owner and repo.
func (c *Context7Tool) parseLibraryID(id string) (owner, repo string) {
	// Remove leading slash and split
	id = strings.TrimPrefix(id, "/")
	parts := strings.SplitN(id, "/", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
