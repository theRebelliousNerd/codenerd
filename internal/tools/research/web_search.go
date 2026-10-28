package research

import (
	"context"
	"fmt"
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
	results, read, err := searchDuckDuckGo(ctx, query, maxResults)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		// "No results" is a claim about the provider's answer. A page with
		// no recognized result cards and no recognized empty-results state
		// is not a search answer at all -- a challenge, a consent page, a
		// changed layout -- and reporting it as an empty search turned an
		// unknown into negative evidence.
		if !read.emptyConfirmed {
			logging.Researcher("Web search page for %q held no recognized results or empty-results state", query)
			return "", fmt.Errorf("search for %q returned a page that is not a recognized DuckDuckGo answer: no result cards and no empty-results marker (a challenge, consent page or changed layout). Nothing is established about results for this query%s", query, read.cleanup)
		}
		logging.Researcher("Web search returned no results for: %s", query)
		return markSearchGuardTrip("No results found for: "+query+" (DuckDuckGo reported no results for this query; that is the provider's answer at this time, not proof the information does not exist)", read.guardTripped) + read.cleanup, nil
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
	return markSearchGuardTrip(sb.String(), read.guardTripped) + read.cleanup, nil
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

// searchRead is what the page read says about itself beyond the results: the
// body guard tripped (the caller marks the cut), or closing the tab failed
// after the results were read (cleanup is the notice to append).
type searchRead struct {
	guardTripped bool
	cleanup      string
	// emptyConfirmed is set when a page with no result cards carries the
	// provider's own empty-results state.
	emptyConfirmed bool
}

// searchDuckDuckGo performs a search using DuckDuckGo HTML interface.
// maxResults <= 0 means every result the page holds.
func searchDuckDuckGo(ctx context.Context, query string, maxResults int) ([]SearchResult, searchRead, error) {
	searchURL := fmt.Sprintf(duckDuckGoSearchURL, url.QueryEscape(query))

	// Per-request bound, not a run clock: it caps one browser page read.
	// From research.web_search_timeout; the installed policy is the
	// defaults until LoadUserConfig installs the file.
	ctx, cancel := context.WithTimeout(ctx, config.ResolvedResearchPolicy().WebSearchTimeout)
	defer cancel()

	page, err := readBrowserPage(ctx, searchURL)
	if err != nil {
		return nil, searchRead{}, fmt.Errorf("request failed: %w", err)
	}

	body, guardTripped := cutGuardBytes(page.HTML, webSearchGuardBytes)
	results, err := parseDuckDuckGoResults(body, maxResults)
	read := searchRead{guardTripped: guardTripped, cleanup: page.cleanupNotice()}
	if err == nil && len(results) == 0 {
		read.emptyConfirmed = duckDuckGoReportsNoResults(body)
	}
	return results, read, err
}

// duckDuckGoReportsNoResults reports whether a page carries DuckDuckGo's
// empty-results state: an element whose class list holds "no-results". The
// absence of result cards alone is not that state.
func duckDuckGoReportsNoResults(htmlContent string) bool {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return false
	}
	var found bool
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found {
			return
		}
		if n.Type == html.ElementNode {
			for _, attr := range n.Attr {
				if attr.Key == "class" && hasClassToken(attr.Val, "no-results") {
					found = true
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return found
}

func hasClassToken(classes, token string) bool {
	for _, c := range strings.Fields(classes) {
		if c == token {
			return true
		}
	}
	return false
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

	// Clean up the URL if it's a DuckDuckGo redirect. Parse the outer query
	// before decoding uddg; unescaping the whole tail turns the destination's
	// own query separators into outer separators and truncates its URL.
	if target, isRedirect, ok := duckDuckGoRedirectTarget(result.URL); isRedirect {
		if !ok {
			// Do not return an unusable DuckDuckGo tracking URL as a search hit.
			result.URL = ""
		} else {
			result.URL = target
		}
	}

	return result
}

// duckDuckGoRedirectTarget decodes the destination from DuckDuckGo's outer
// redirect URL. The bools report whether raw is a DuckDuckGo redirect and
// whether that redirect contains exactly one valid uddg parameter.
func duckDuckGoRedirectTarget(raw string) (target string, isRedirect bool, valid bool) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", looksLikeDuckDuckGoRedirect(raw), false
	}
	if !strings.EqualFold(parsed.Hostname(), "duckduckgo.com") || parsed.Path != "/l/" {
		return "", false, false
	}

	values, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return "", true, false
	}
	destinations := values["uddg"]
	if len(destinations) != 1 || destinations[0] == "" {
		return "", true, false
	}
	return destinations[0], true, true
}

func looksLikeDuckDuckGoRedirect(raw string) bool {
	for _, prefix := range []string{
		"//duckduckgo.com/l/",
		"http://duckduckgo.com/l/",
		"https://duckduckgo.com/l/",
	} {
		if strings.HasPrefix(strings.ToLower(raw), prefix) {
			return true
		}
	}
	return false
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
