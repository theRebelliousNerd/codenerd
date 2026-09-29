package research

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/tools"
)

// Context7Tool returns a tool for fetching LLM-optimized documentation.
// It follows the llms.txt standard for AI-friendly documentation discovery.
func Context7Tool() *tools.Tool {
	return &tools.Tool{
		Name:        "context7_fetch",
		Description: "Fetch LLM-optimized documentation for a library or framework using the llms.txt standard",
		Category:    tools.CategoryResearch,
		Priority:    80, // High priority for research
		Execute:     executeContext7,
		Schema: tools.ToolSchema{
			Required: []string{"topic"},
			Properties: map[string]tools.Property{
				"topic": {
					Type:        "string",
					Description: "Library, framework, or topic name (e.g., 'react', 'go-rod/rod', 'tokio')",
				},
				"repo": {
					Type:        "string",
					Description: "Optional GitHub repo in owner/name format (e.g., 'facebook/react')",
				},
				"max_docs": {
					Type:        "integer",
					Description: "Maximum number of documents to fetch (default: 10)",
					Default:     10,
				},
			},
		},
	}
}

func executeContext7(ctx context.Context, args map[string]any) (string, error) {
	topic, _ := args["topic"].(string)
	if topic == "" {
		return "", fmt.Errorf("topic is required")
	}

	repo, _ := args["repo"].(string)
	maxDocs := 10
	if md, ok := argInt(args, "max_docs"); ok && md > 0 {
		maxDocs = md
	}

	logging.ResearcherDebug("Context7 fetch: topic=%s, repo=%s, max_docs=%d", topic, repo, maxDocs)

	// Check for API key (optional - some sources work without it)
	apiKey := config.AutoDetectContext7APIKey()
	if apiKey == "" {
		logging.ResearcherDebug("No Context7 API key configured, using public access")
	}

	// If no repo specified, try to infer from topic
	if repo == "" {
		repo = inferRepo(topic)
	}

	var results []string

	// Try to fetch llms.txt from GitHub
	if repo != "" {
		parts := strings.Split(repo, "/")
		if len(parts) == 2 {
			owner, repoName := parts[0], parts[1]
			docs, err := fetchLlmsTxt(ctx, owner, repoName, apiKey, maxDocs)
			if err == nil && len(docs) > 0 {
				results = append(results, docs...)
			}
		}
	}

	// If no llms.txt found, try common documentation patterns
	if len(results) == 0 && repo != "" {
		parts := strings.Split(repo, "/")
		if len(parts) == 2 {
			docs, err := fetchCommonDocs(ctx, parts[0], parts[1], apiKey, maxDocs)
			if err == nil {
				results = append(results, docs...)
			}
		}
	}

	// If still no results, return an error so callers do not ingest the
	// "no documentation" message as researched knowledge.
	if len(results) == 0 {
		return "", fmt.Errorf("no LLM-optimized documentation found for %q; "+
			"the repository may have no llms.txt or README.md", topic)
	}

	// Combine results with headers
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Documentation for %s\n\n", topic))
	for i, doc := range results {
		if i >= maxDocs {
			break
		}
		sb.WriteString(doc)
		sb.WriteString("\n\n---\n\n")
	}

	return sb.String(), nil
}

// inferRepo attempts to map common topic names to GitHub repos.
func inferRepo(topic string) string {
	knownMappings := map[string]string{
		"react":      "facebook/react",
		"vue":        "vuejs/vue",
		"angular":    "angular/angular",
		"svelte":     "sveltejs/svelte",
		"next":       "vercel/next.js",
		"nextjs":     "vercel/next.js",
		"nuxt":       "nuxt/nuxt",
		"rod":        "go-rod/rod",
		"go-rod":     "go-rod/rod",
		"tokio":      "tokio-rs/tokio",
		"axum":       "tokio-rs/axum",
		"mangle":     "google/mangle",
		"bubbletea":  "charmbracelet/bubbletea",
		"bubbles":    "charmbracelet/bubbles",
		"lipgloss":   "charmbracelet/lipgloss",
		"cobra":      "spf13/cobra",
		"viper":      "spf13/viper",
		"gin":        "gin-gonic/gin",
		"echo":       "labstack/echo",
		"fiber":      "gofiber/fiber",
		"gorm":       "go-gorm/gorm",
		"zap":        "uber-go/zap",
		"logrus":     "sirupsen/logrus",
		"testify":    "stretchr/testify",
		"sqlite-vec": "asg017/sqlite-vec",
	}

	// Try exact match first
	if repo, ok := knownMappings[strings.ToLower(topic)]; ok {
		return repo
	}

	// Try to parse as owner/repo
	if strings.Contains(topic, "/") {
		return topic
	}

	return ""
}

// fetchLlmsTxt fetches and parses the llms.txt file from a GitHub repo.
func fetchLlmsTxt(ctx context.Context, owner, repo, apiKey string, maxDocs int) ([]string, error) {
	locations := []string{
		fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/llms.txt", owner, repo),
		fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/master/llms.txt", owner, repo),
		fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/.llms.txt", owner, repo),
	}

	for _, url := range locations {
		content, err := fetchURL(ctx, url, apiKey)
		if err == nil && len(content) > 10 {
			logging.Researcher("Found llms.txt at %s", url)
			return parseLlmsTxt(ctx, owner, repo, content, apiKey, maxDocs)
		}
	}

	return nil, fmt.Errorf("no llms.txt found")
}

// parseLlmsTxt parses the llms.txt content and fetches referenced documents.
// Fetched documents are returned whole: since 63aef15f the working-context
// ledger archives large tool results behind recall handles, so cutting doc
// text here would only destroy evidence the ledger must size for the window.
// maxDocs still bounds how many documents are fetched, but the cap is never
// silent: when linked docs are left unfetched the result names them and the
// max_docs argument that raises the bound (limits cleanup 2026-09-29).
func parseLlmsTxt(ctx context.Context, owner, repo, content, apiKey string, maxDocs int) ([]string, error) {
	var results []string
	lines := strings.Split(content, "\n")
	docCount := 0
	skipped := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ">") {
			continue
		}

		// Parse markdown links: [title](url) or just paths
		var docURL string
		if strings.Contains(line, "](") {
			// Markdown link format
			start := strings.Index(line, "](")
			end := strings.Index(line[start:], ")")
			if start > 0 && end > 0 {
				docURL = line[start+2 : start+end]
			}
		} else if strings.HasPrefix(line, "-") {
			// List item with path
			parts := strings.SplitN(line, ":", 2)
			docURL = strings.TrimSpace(strings.TrimPrefix(parts[0], "-"))
		} else {
			docURL = line
		}

		if docURL == "" {
			continue
		}

		// Resolve relative paths to GitHub raw URLs
		if !strings.HasPrefix(docURL, "http") {
			docURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/%s",
				owner, repo, strings.TrimPrefix(docURL, "/"))
		}

		if docCount >= maxDocs {
			skipped++
			continue
		}
		content, err := fetchURL(ctx, docURL, apiKey)
		if err == nil && len(content) > 50 {
			results = append(results, fmt.Sprintf("## Source: %s\n\n%s", docURL, content))
			docCount++
		}
	}
	if skipped > 0 {
		results = append(results, fmt.Sprintf("[%d more linked docs not fetched; raise max_docs to include them (fetched %d)]", skipped, docCount))
	}

	return results, nil
}

// fetchCommonDocs fetches common documentation files from a GitHub repo.
func fetchCommonDocs(ctx context.Context, owner, repo, apiKey string, maxDocs int) ([]string, error) {
	commonPaths := []string{
		"README.md",
		"docs/README.md",
		"documentation/README.md",
		"docs/getting-started.md",
		"docs/quickstart.md",
		"GETTING_STARTED.md",
	}

	var results []string
	for _, path := range commonPaths {
		if len(results) >= maxDocs {
			break
		}

		url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/%s", owner, repo, path)
		content, err := fetchURL(ctx, url, apiKey)
		if err == nil && len(content) > 100 {
			results = append(results, fmt.Sprintf("## Source: %s\n\n%s", url, content))
		}
	}

	return results, nil
}

// fetchURLGuardBytes caps one fetched document. It is process protection,
// not a tunable: without it a hostile or runaway response (Content-Length
// lies; bodies stream until OOM) exhausts process memory. When the guard
// trips the returned content carries an explicit marker naming the cut, so
// it never reaches the model as a silent truncation.
const fetchURLGuardBytes = 1 << 20

// fetchURL fetches content from a URL with timeout and optional auth.
func fetchURL(ctx context.Context, url, apiKey string) (string, error) {
	// Per-request network bound, not a run clock: it caps one HTTP round
	// trip. OPEN (limits cleanup 2026-09-29): the value must come from
	// internal/config, but that package is outside this lane's scope.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "codeNERD/1.0 (Context7 Research Tool)")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, fetchURLGuardBytes+1))
	if err != nil {
		return "", err
	}
	if len(body) > fetchURLGuardBytes {
		return string(body[:fetchURLGuardBytes]) +
			"\n\n[fetch guard: source exceeded the 1MB single-document guard; " +
			"content continues beyond what was read]", nil
	}

	return string(body), nil
}
