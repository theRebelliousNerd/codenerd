package research

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/tools"

	"golang.org/x/net/html"
)

// SearchResult represents a single search result.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// WebSearchTool returns a tool for searching the web.
func WebSearchTool() *tools.Tool {
	return &tools.Tool{
		Name:        "web_search",
		Description: "Search the web for information using DuckDuckGo",
		Category:    tools.CategoryResearch,
		Priority:    75, // Higher than web_fetch, lower than context7
		Execute:     executeWebSearch,
		Schema: tools.ToolSchema{
			Required: []string{"query"},
			Properties: map[string]tools.Property{
				"query": {
					Type:        "string",
					Description: "The search query",
				},
				"max_results": {
					Type:        "integer",
					Description: "Optional bound on results returned. Omit to return every result the page holds; an explicit value is the model's own choice",
				},
			},
		},
	}
}

func executeWebSearch(ctx context.Context, args map[string]any) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	// Absent max_results means every result the page holds: the old
	// default-10 plus hard-30 cap silently dropped results the model
	// never asked to drop. An explicit max_results is the model's own
	// choice and is honored as given (limits cleanup 2026-09-29).
	maxResults := 0
	if mr, ok := argInt(args, "max_results"); ok && mr > 0 {
		maxResults = mr
	}

	logging.ResearcherDebug("Web search: query=%q, max_results=%d", query, maxResults)

	// Use DuckDuckGo HTML search (no API key required)
	results, guardTripped, err := searchDuckDuckGo(ctx, query, maxResults)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		logging.Researcher("Web search returned no results for: %s", query)
		return markSearchGuardTrip("No results found for: "+query, guardTripped), nil
	}

	// Format results as markdown
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Search Results for: %s\n\n", query))
	sb.WriteString(fmt.Sprintf("Found %d results:\n\n", len(results)))

	for i, result := range results {
		sb.WriteString(fmt.Sprintf("## %d. %s\n", i+1, result.Title))
		sb.WriteString(fmt.Sprintf("**URL:** %s\n", result.URL))
		if result.Snippet != "" {
			sb.WriteString(fmt.Sprintf("\n%s\n", result.Snippet))
		}
		sb.WriteString("\n---\n\n")
	}

	logging.Researcher("Web search completed: %d results for %q", len(results), query)
	return markSearchGuardTrip(sb.String(), guardTripped), nil
}

// webSearchGuardBytes caps one search-response body. It is process
// protection, not a tunable: without it a hostile or runaway response
// exhausts process memory. When the guard trips the result carries an
// explicit marker naming the cut, so it never reaches the model as a
// silent truncation.
const webSearchGuardBytes = 1 << 20

// markSearchGuardTrip appends the fetch-guard marker when the body guard tripped.
func markSearchGuardTrip(result string, tripped bool) string {
	if !tripped {
		return result
	}
	return result + "\n\n[fetch guard: search response exceeded the 1MB web_search guard; " +
		"results past the cut were not parsed]"
}

// duckDuckGoSearchURL formats the search URL. A variable, not a
// constant, so tests can point the search at a local server;
// production never sets it.
var duckDuckGoSearchURL = "https://html.duckduckgo.com/html/?q=%s"

// searchDuckDuckGo performs a search using DuckDuckGo HTML interface.
// maxResults <= 0 means every result the page holds. It also reports
// whether the body guard tripped so the caller can mark the cut.
func searchDuckDuckGo(ctx context.Context, query string, maxResults int) ([]SearchResult, bool, error) {
	searchURL := fmt.Sprintf(duckDuckGoSearchURL, url.QueryEscape(query))

	// Per-request network bound, not a run clock: it caps one HTTP round
	// trip. From research.web_search_timeout; the installed policy is
	// the defaults until LoadUserConfig installs the file.
	ctx, cancel := context.WithTimeout(ctx, config.ResolvedResearchPolicy().WebSearchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers to look like a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, webSearchGuardBytes+1))
	if err != nil {
		return nil, false, fmt.Errorf("failed to read response: %w", err)
	}
	guardTripped := len(body) > webSearchGuardBytes
	if guardTripped {
		body = body[:webSearchGuardBytes]
	}

	results, err := parseDuckDuckGoResults(string(body), maxResults)
	return results, guardTripped, err
}

// parseDuckDuckGoResults extracts search results from DuckDuckGo HTML.
// maxResults <= 0 means every result the page holds.
func parseDuckDuckGoResults(htmlContent string, maxResults int) ([]SearchResult, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var results []SearchResult

	// DuckDuckGo HTML uses class="result" for search results
	var findResults func(*html.Node)
	findResults = func(n *html.Node) {
		if maxResults > 0 && len(results) >= maxResults {
			return
		}

		if n.Type == html.ElementNode && n.Data == "div" {
			for _, attr := range n.Attr {
				if attr.Key == "class" && strings.Contains(attr.Val, "result") && strings.Contains(attr.Val, "results_links") {
					result := extractResult(n)
					if result.URL != "" && result.Title != "" {
						results = append(results, result)
					}
					return
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findResults(c)
		}
	}

	findResults(doc)
	return results, nil
}

// extractResult extracts a single search result from a result div.
func extractResult(n *html.Node) SearchResult {
	var result SearchResult

	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "a":
				// Check if this is the result link or snippet
				for _, attr := range n.Attr {
					if attr.Key == "class" {
						if strings.Contains(attr.Val, "result__a") {
							result.URL = getAttrValue(n, "href")
							result.Title = getTextContent(n)
						} else if strings.Contains(attr.Val, "result__snippet") {
							result.Snippet = getTextContent(n)
						}
					}
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}

	extract(n)

	// Clean up the URL if it's a DuckDuckGo redirect
	if after, ok := strings.CutPrefix(result.URL, "//duckduckgo.com/l/?uddg="); ok {
		if decoded, err := url.QueryUnescape(after); err == nil {
			if idx := strings.Index(decoded, "&"); idx > 0 {
				decoded = decoded[:idx]
			}
			result.URL = decoded
		}
	}

	return result
}

// getAttrValue returns the value of an attribute.
func getAttrValue(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// getTextContent returns all text content within a node.
func getTextContent(n *html.Node) string {
	var sb strings.Builder
	var getText func(*html.Node)
	getText = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(strings.TrimSpace(n.Data))
			sb.WriteString(" ")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			getText(c)
		}
	}
	getText(n)
	return strings.TrimSpace(sb.String())
}
