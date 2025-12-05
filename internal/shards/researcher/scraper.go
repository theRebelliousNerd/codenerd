// Package researcher - Web scraping and HTML parsing functions.
// This file contains HTTP client operations, URL fetching, and HTML parsing logic.
package researcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

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
		Name: "React Frontend Framework",
		Type: "llm",
	},
	"typescript": {
		Name: "TypeScript Language",
		Type: "llm",
	},
	"kubernetes": {
		Name: "Kubernetes Container Orchestration",
		Type: "llm",
	},
	"docker": {
		Name: "Docker Containerization",
		Type: "llm",
	},
	"security": {
		Name: "Security Best Practices",
		Type: "llm",
	},
	"testing": {
		Name: "Testing Best Practices",
		Type: "llm",
	},
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
		return r.fetchGitHubDocs(ctx, source, keywords)
	case "pkggodev":
		return r.fetchPkgGoDev(ctx, source)
	case "llm":
		// LLM sources are handled separately
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown source type: %s", source.Type)
	}
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

// generateSearchURLs creates a list of URLs to research (kept for compatibility but unused)
func (r *ResearcherShard) generateSearchURLs(topic string, keywords []string) []string {
	// This method is deprecated in favor of the multi-strategy approach
	// Kept for backward compatibility
	return []string{}
}
