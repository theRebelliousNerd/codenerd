package research

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

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
					Description: "Maximum number of results to return (default: 10)",
					Default:     10,
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

	maxResults := 10
	if mr, ok := args["max_results"].(int); ok && mr > 0 {
		maxResults = mr
	}
	if maxResults > 30 {
		maxResults = 30 // Cap at 30 results
	}

	logging.ResearcherDebug("Web search: query=%q, max_results=%d", query, maxResults)

	// Use DuckDuckGo HTML search (no API key required)
	results, err := searchDuckDuckGo(ctx, query, maxResults)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		logging.Researcher("Web search returned no results for: %s", query)
		return "No results found for: " + query, nil
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
	return sb.String(), nil
}

// searchDuckDuckGo performs a search using DuckDuckGo HTML interface.
func searchDuckDuckGo(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	// DuckDuckGo HTML search URL
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers to look like a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return parseDuckDuckGoResults(string(body), maxResults)
}

// parseDuckDuckGoResults extracts search results from DuckDuckGo HTML.
func parseDuckDuckGoResults(htmlContent string, maxResults int) ([]SearchResult, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var results []SearchResult

	// DuckDuckGo HTML uses class="result" for search results
	var findResults func(*html.Node)
	findResults = func(n *html.Node) {
		if len(results) >= maxResults {
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
	if strings.HasPrefix(result.URL, "//duckduckgo.com/l/?uddg=") {
		if decoded, err := url.QueryUnescape(strings.TrimPrefix(result.URL, "//duckduckgo.com/l/?uddg=")); err == nil {
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

// SearchResultsToJSON converts results to JSON for structured output.
func SearchResultsToJSON(results []SearchResult) (string, error) {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
