package research

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"codenerd/internal/config"
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
					Description: "Optional paging window in characters. Omit to return the full content; when set with offset, returns that rune window plus how to page the rest",
				},
				"offset": {
					Type:        "integer",
					Description: "Rune offset where the max_length window starts (default: 0)",
					Default:     0,
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

	// Absent max_length means the whole content: the working-context
	// ledger archives large tool results behind recall handles, so a
	// default cut here would only destroy evidence. An explicit
	// max_length is a model-chosen paging window, never a silent drop:
	// pageRunes names the remainder and the offset that reaches it.
	maxLength := 0
	if ml, ok := argInt(args, "max_length"); ok && ml > 0 {
		maxLength = ml
	}
	offset := 0
	if off, ok := argInt(args, "offset"); ok && off > 0 {
		offset = off
	}

	includeLinks := true
	if il, ok := args["include_links"].(bool); ok {
		includeLinks = il
	}

	logging.ResearcherDebug("Web fetch: url=%s, max_length=%d, offset=%d", url, maxLength, offset)

	// Per-request bound, not a run clock: it caps one browser page read.
	// From research.web_fetch_timeout; the installed policy is the
	// defaults until LoadUserConfig installs the file.
	ctx, cancel := context.WithTimeout(ctx, config.ResolvedResearchPolicy().WebFetchTimeout)
	defer cancel()

	page, err := readBrowserPage(ctx, url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}

	// The guard is process protection: a hostile page must not exhaust
	// memory. When it trips the result names the cut. Plain and markdown
	// responses are the rendered text (Chrome wraps them in <pre>); HTML
	// is the rendered document, then the existing markdown conversion.
	source := page.HTML
	if plainResearchBody(page) {
		source = page.Text
	}
	source, guardTripped := cutGuardBytes(source, webFetchGuardBytes)

	if plainResearchBody(page) {
		result := pageRunes(source, offset, maxLength)
		return markGuardTrip(result, guardTripped), nil
	}

	markdown, err := htmlToMarkdown(source, url, includeLinks)
	if err != nil {
		return "", fmt.Errorf("failed to convert to markdown: %w", err)
	}

	markdown = pageRunes(markdown, offset, maxLength)

	logging.Researcher("Web fetch completed: %s (%d chars)", url, len(markdown))
	return markGuardTrip(markdown, guardTripped), nil
}

// webFetchGuardBytes caps one fetched response body. It is process
// protection, not a tunable (see the read site above).
const webFetchGuardBytes = 2 << 20

// markGuardTrip appends the fetch-guard marker when the body guard tripped.
func markGuardTrip(result string, tripped bool) string {
	if !tripped {
		return result
	}
	return result + "\n\n[fetch guard: source body exceeded the 2MB web_fetch guard; " +
		"content continues beyond what was read]"
}

// pageRunes returns the whole string when no paging window was asked for
// (maxLength <= 0 and offset <= 0). Otherwise it returns the
// [offset, offset+maxLength) rune window cut on rune boundaries so UTF-8 is
// never split, plus a notice naming the remainder and the offset that
// reaches it — an explicit window is paging, never a silent drop.
func pageRunes(s string, offset, maxLength int) string {
	if maxLength <= 0 && offset <= 0 {
		return s
	}
	total := len([]rune(s))
	if offset >= total {
		return fmt.Sprintf("[offset %d is past the end (%d chars total)]", offset, total)
	}
	end := total
	if maxLength > 0 && offset+maxLength < total {
		end = offset + maxLength
	}
	window := string([]rune(s)[offset:end])
	if end < total {
		return fmt.Sprintf("%s\n\n[%d more chars, page with offset=%d]", window, total-end, end)
	}
	if offset > 0 {
		return fmt.Sprintf("%s\n\n[end of content, %d chars total]", window, total)
	}
	return window
}

// htmlToMarkdown converts HTML to a simplified markdown format.
func htmlToMarkdown(htmlContent, baseURL string, includeLinks bool) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	deepNestingSkipped := false
	extractText(doc, &sb, includeLinks, baseURL, 0, &deepNestingSkipped)

	// Clean up the result
	result := sb.String()
	result = cleanMarkdown(result)
	if deepNestingSkipped {
		result += "\n\n[parse guard: elements nested past 50 levels were skipped; " +
			"content continues beyond what was converted]"
	}

	return result, nil
}

// maxExtractDepth caps HTML walker recursion. It is process protection,
// not a tunable: adversarial pages nest thousands deep and each level
// costs stack, so an unbounded walk crashes the process (Go stacks grow
// to a 1GB max, then fatal). Real pages nest far shallower, and when the
// guard trips the markdown carries an explicit marker, so the skip never
// reaches the model as a silent cut.
const maxExtractDepth = 50

func extractText(n *html.Node, sb *strings.Builder, includeLinks bool, baseURL string, depth int, deepNestingSkipped *bool) {
	if depth > maxExtractDepth {
		*deepNestingSkipped = true
		return
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
				extractText(c, sb, includeLinks, baseURL, depth+1, deepNestingSkipped)
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
		extractText(c, sb, includeLinks, baseURL, depth+1, deepNestingSkipped)
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
