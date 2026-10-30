package research

import (
	"context"
	"fmt"
	"strings"

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
					Description: "Optional repo: owner/name on GitHub (e.g., 'facebook/react'), or host/owner/name for Codeberg (e.g., 'codeberg.org/TauCeti/mangle-go')",
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

	// A Context7 bearer is not a credential for the documentation URLs
	// this tool reads (raw GitHub files). It is not sent. The key is
	// still detected so the log can say whether one is configured.
	apiKey := config.AutoDetectContext7APIKey()
	if apiKey == "" {
		logging.ResearcherDebug("No Context7 API key configured, using public access")
	} else {
		logging.ResearcherDebug("Context7 API key is configured; research page reads do not send it")
	}

	// If no repo specified, try to infer from topic
	if repo == "" {
		repo = inferRepo(topic)
	}

	var results []string

	ref, haveRef := parseRepoRef(repo)

	// Try to fetch llms.txt from the repository host
	if haveRef {
		docs, err := fetchLlmsTxt(ctx, ref, maxDocs)
		if err == nil && len(docs) > 0 {
			results = append(results, docs...)
		}
	}

	// If no llms.txt found, try common documentation patterns
	if len(results) == 0 && haveRef {
		docs, err := fetchCommonDocs(ctx, ref, maxDocs)
		if err == nil {
			results = append(results, docs...)
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

// Hosts a documentation repository can live on. Raw files are read from the
// host that owns the repository: a repository on Codeberg has no GitHub copy to
// read.
const (
	hostGitHub   = "github.com"
	hostCodeberg = "codeberg.org"
)

// repoRef names a documentation repository on a supported host.
type repoRef struct {
	host, owner, name string
}

// githubRepo is the GitHub repository owner/name.
func githubRepo(owner, name string) repoRef {
	return repoRef{host: hostGitHub, owner: owner, name: name}
}

// parseRepoRef reads "owner/name" (GitHub) or "host/owner/name" for a supported
// host. Anything else is not a repository this tool can read.
func parseRepoRef(s string) (repoRef, bool) {
	parts := strings.Split(strings.TrimSpace(s), "/")
	switch len(parts) {
	case 2:
		if parts[0] != "" && parts[1] != "" {
			return githubRepo(parts[0], parts[1]), true
		}
	case 3:
		if (parts[0] == hostGitHub || parts[0] == hostCodeberg) && parts[1] != "" && parts[2] != "" {
			return repoRef{host: parts[0], owner: parts[1], name: parts[2]}, true
		}
	}
	return repoRef{}, false
}

// inferRepo attempts to map common topic names to repositories: owner/name for
// GitHub, host/owner/name for another host.
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
		"mangle":     "codeberg.org/TauCeti/mangle-go",
		"mangle-go":  "codeberg.org/TauCeti/mangle-go",
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

	// A Go module path on GitHub ("github.com/spf13/cobra/doc") names its
	// repository in its first two path elements: the knowledge sources ask
	// about a pinned module by its path.
	if rest, ok := strings.CutPrefix(strings.ToLower(topic), "github.com/"); ok {
		parts := strings.SplitN(topic[len("github.com/"):], "/", 3)
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" && rest != "" {
			return parts[0] + "/" + parts[1]
		}
		return ""
	}

	// Try to parse as owner/repo
	if strings.Contains(topic, "/") {
		return topic
	}

	return ""
}

// rawGitHubFileURL builds a raw documentation URL. A variable, not a
// constant, so tests can point reads at a local server; production never
// sets it. Absolute links inside an llms.txt stay absolute and still go
// through the same browser read.
var rawGitHubFileURL = func(owner, repo, ref, path string) string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, ref, strings.TrimPrefix(path, "/"))
}

// rawCodebergFileURL builds a raw documentation URL on Codeberg. A variable
// for the same reason as rawGitHubFileURL.
var rawCodebergFileURL = func(owner, repo, ref, path string) string {
	return fmt.Sprintf("https://codeberg.org/%s/%s/raw/branch/%s/%s", owner, repo, ref, strings.TrimPrefix(path, "/"))
}

// rawFileURL builds the raw URL of one file of r on the host that owns r.
func rawFileURL(r repoRef, ref, path string) string {
	if r.host == hostCodeberg {
		return rawCodebergFileURL(r.owner, r.name, ref, path)
	}
	return rawGitHubFileURL(r.owner, r.name, ref, path)
}

// fetchLlmsTxt fetches and parses the llms.txt file from a repository.
func fetchLlmsTxt(ctx context.Context, r repoRef, maxDocs int) ([]string, error) {
	locations := []string{
		rawFileURL(r, "main", "llms.txt"),
		rawFileURL(r, "master", "llms.txt"),
		rawFileURL(r, "main", ".llms.txt"),
	}

	for _, url := range locations {
		content, err := fetchURL(ctx, url)
		if err == nil && len(content) > 10 {
			logging.Researcher("Found llms.txt at %s", url)
			return parseLlmsTxt(ctx, r, content, maxDocs)
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
func parseLlmsTxt(ctx context.Context, r repoRef, content string, maxDocs int) ([]string, error) {
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

		// Resolve relative paths to raw URLs on the repository's host
		if !strings.HasPrefix(docURL, "http") {
			docURL = rawFileURL(r, "main", docURL)
		}

		if docCount >= maxDocs {
			skipped++
			continue
		}
		content, err := fetchURL(ctx, docURL)
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

// fetchCommonDocs fetches common documentation files from a repository.
func fetchCommonDocs(ctx context.Context, r repoRef, maxDocs int) ([]string, error) {
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

		url := rawFileURL(r, "main", path)
		content, err := fetchURL(ctx, url)
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

// fetchURL reads one documentation URL through the browser. Plain and
// markdown responses come back as the rendered text (Chrome wraps them in
// <pre>). HTML comes back as the rendered document, not a second markdown
// conversion: callers keep the source text they asked to archive.
func fetchURL(ctx context.Context, url string) (string, error) {
	// Per-request bound, not a run clock: it caps one browser page read.
	// From research.context7_timeout; the installed policy is the
	// defaults until LoadUserConfig installs the file.
	ctx, cancel := context.WithTimeout(ctx, config.ResolvedResearchPolicy().Context7Timeout)
	defer cancel()

	page, err := readBrowserPage(ctx, url)
	if err != nil {
		return "", err
	}
	if page.CleanupErr != nil {
		// The body is archived as the source document, so a notice cannot be
		// appended to it. The pending cleanup stays in the research log.
		logging.Researcher("Context7 read of %s succeeded; browser cleanup pending: %v", url, page.CleanupErr)
	}
	body := page.Text
	ct := strings.ToLower(page.ContentType)
	if strings.Contains(ct, "text/html") || strings.Contains(ct, "xhtml") {
		body = page.HTML
	}
	cut, tripped := cutGuardBytes(body, fetchURLGuardBytes)
	if tripped {
		return cut + "\n\n[fetch guard: source exceeded the 1MB single-document guard; " +
			"content continues beyond what was read]", nil
	}
	return body, nil
}
