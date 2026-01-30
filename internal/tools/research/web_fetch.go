package research

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/tools"

	"golang.org/x/net/html"
)

// Pre-compile regex patterns to avoid recompilation overhead
var (
	multiNewlinePattern = regexp.MustCompile(`\n{3,}`)
	multiSpacePattern   = regexp.MustCompile(`[ \t]{2,}`)
)

// WebFetchTool returns a tool for fetching web pages and converting to markdown.
func WebFetchTool() *tools.Tool {
	return &tools.Tool{
		Name:        "web_fetch",
		Description: "Fetch a web page and convert its content to markdown format",
		Category:    tools.CategoryResearch,
		Priority:    70,
		Execute:     executeWebFetch,
		Schema: tools.ToolSchema{
			Required: []string{"url"},
			Properties: map[string]tools.Property{
				"url": {
					Type:        "string",
					Description: "The URL to fetch",
				},
				"max_length": {
					Type:        "integer",
					Description: "Maximum content length in characters (default: 50000)",
					Default:     50000,
				},
				"include_links": {
					Type:        "boolean",
					Description: "Whether to include links in the output (default: true)",
					Default:     true,
				},
			},
		},
	}
}

func executeWebFetch(ctx context.Context, args map[string]any) (string, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return "", fmt.Errorf("url is required")
	}

	maxLength := 50000
	if ml, ok := args["max_length"].(int); ok && ml > 0 {
		maxLength = ml
	}

	includeLinks := true
	if il, ok := args["include_links"].(bool); ok {
		includeLinks = il
	}

	logging.ResearcherDebug("Web fetch: url=%s, max_length=%d", url, maxLength)

	// Fetch the page
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; codeNERD/1.0; +https://github.com/codenerd)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Read the body with a limit
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2MB limit
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")

	// If it's already plain text or markdown, return as-is
	if strings.Contains(contentType, "text/plain") ||
		strings.Contains(contentType, "text/markdown") {
		result := string(body)
		if len(result) > maxLength {
			result = result[:maxLength] + "\n\n[...truncated...]"
		}
		return result, nil
	}

	// Convert HTML to markdown
	markdown, err := htmlToMarkdown(string(body), url, includeLinks)
	if err != nil {
		return "", fmt.Errorf("failed to convert to markdown: %w", err)
	}

	if len(markdown) > maxLength {
		markdown = markdown[:maxLength] + "\n\n[...truncated...]"
	}

	logging.Researcher("Web fetch completed: %s (%d chars)", url, len(markdown))
	return markdown, nil
}

// htmlToMarkdown converts HTML to a simplified markdown format.
func htmlToMarkdown(htmlContent, baseURL string, includeLinks bool) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	extractText(doc, &sb, includeLinks, baseURL, 0)

	// Clean up the result
	result := sb.String()
	result = cleanMarkdown(result)

	return result, nil
}

func extractText(n *html.Node, sb *strings.Builder, includeLinks bool, baseURL string, depth int) {
	if depth > 50 {
		return // Prevent excessive recursion
	}

	switch n.Type {
	case html.TextNode:
		text := strings.TrimSpace(n.Data)
		if text != "" {
			sb.WriteString(text)
			sb.WriteString(" ")
		}
	case html.ElementNode:
		switch n.Data {
		case "script", "style", "noscript", "iframe", "svg", "nav", "footer", "header":
			return // Skip these elements
		case "title":
			sb.WriteString("# ")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				extractText(c, sb, includeLinks, baseURL, depth+1)
			}
			sb.WriteString("\n\n")
			return
		case "h1":
			sb.WriteString("\n\n# ")
		case "h2":
			sb.WriteString("\n\n## ")
		case "h3":
			sb.WriteString("\n\n### ")
		case "h4":
			sb.WriteString("\n\n#### ")
		case "h5":
			sb.WriteString("\n\n##### ")
		case "h6":
			sb.WriteString("\n\n###### ")
		case "p", "div":
			sb.WriteString("\n\n")
		case "br":
			sb.WriteString("\n")
		case "li":
			sb.WriteString("\n- ")
		case "code":
			sb.WriteString("`")
		case "pre":
			sb.WriteString("\n\n```\n")
		case "strong", "b":
			sb.WriteString("**")
		case "em", "i":
			sb.WriteString("*")
		case "a":
			if includeLinks {
				href := getAttr(n, "href")
				if href != "" && !strings.HasPrefix(href, "#") {
					sb.WriteString("[")
				}
			}
		case "img":
			alt := getAttr(n, "alt")
			if alt != "" {
				sb.WriteString(fmt.Sprintf("[Image: %s]", alt))
			}
			return
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractText(c, sb, includeLinks, baseURL, depth+1)
	}

	if n.Type == html.ElementNode {
		switch n.Data {
		case "h1", "h2", "h3", "h4", "h5", "h6":
			sb.WriteString("\n\n")
		case "code":
			sb.WriteString("`")
		case "pre":
			sb.WriteString("\n```\n\n")
		case "strong", "b":
			sb.WriteString("**")
		case "em", "i":
			sb.WriteString("*")
		case "a":
			if includeLinks {
				href := getAttr(n, "href")
				if href != "" && !strings.HasPrefix(href, "#") {
					sb.WriteString(fmt.Sprintf("](%s)", href))
				}
			}
		}
	}
}

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// cleanMarkdown removes excessive whitespace and cleans up the markdown.
func cleanMarkdown(s string) string {
	// Replace multiple newlines with max 2
	s = multiNewlinePattern.ReplaceAllString(s, "\n\n")

	// Replace multiple spaces with single space
	s = multiSpacePattern.ReplaceAllString(s, " ")

	// Trim each line
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	s = strings.Join(lines, "\n")

	// Final trim
	s = strings.TrimSpace(s)

	return s
}
