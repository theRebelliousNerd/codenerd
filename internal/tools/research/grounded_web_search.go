package research

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"codenerd/internal/tools"
	"codenerd/internal/types"
)

const (
	groundedWebSearchToolName       = "grounded_web_search"
	maxGroundedWebSearchOutputChars = 20000
	maxGroundedWebSearchCitations   = 50
	maxGroundedWebSearchQueryChars  = 10000
	maxGroundedWebSearchURLChars    = 2048
	maxGroundedWebSearchTitleChars  = 512
	groundedWebSearchTruncateSuffix = "\n[...truncated...]"
)

// GroundedWebSearchTool returns a provider-neutral tool that performs a
// Meta-native grounded web search via the injected GroundedWebSearcher.
// The tool takes a required string query, invokes the searcher with the exact
// query, and returns bounded structured JSON containing only text, citations,
// and usage. It never returns config or credentials.
func GroundedWebSearchTool(searcher types.GroundedWebSearcher) *tools.Tool {
	// Capture searcher in closure; registration helper guarantees non-nil and
	// supported before this is registered, but Execute still defensively checks.
	return &tools.Tool{
		Name:        groundedWebSearchToolName,
		Description: "Perform a grounded web search using the configured LLM provider (Meta). Returns grounded answer text with URL citations and token usage.",
		Category:    tools.CategoryResearch,
		Priority:    75,
		Execute:     executeGroundedWebSearch(searcher),
		Schema: tools.ToolSchema{
			Required: []string{"query"},
			Properties: map[string]tools.Property{
				"query": {
					Type:        "string",
					Description: "The search query (1-10000 characters)",
				},
			},
		},
	}
}

func executeGroundedWebSearch(searcher types.GroundedWebSearcher) tools.ExecuteFunc {
	return func(ctx context.Context, args map[string]any) (string, error) {
		if ctx == nil {
			ctx = context.Background()
		}
		if searcher == nil {
			return "", fmt.Errorf("grounded_web_search: searcher is nil")
		}
		if !searcher.SupportsGroundedWebSearch() {
			return "", fmt.Errorf("grounded_web_search is not supported by the configured provider")
		}
		query, _ := args["query"].(string)
		trimmed := strings.TrimSpace(query)
		if trimmed == "" {
			return "", fmt.Errorf("query is required")
		}
		if len(trimmed) > maxGroundedWebSearchQueryChars {
			return "", fmt.Errorf("query exceeds %d characters", maxGroundedWebSearchQueryChars)
		}
		// Exact query forwarding: preserve original query shape after trim check.
		// Forward trimmed query exactly; underlying client does its own trim.
		result, err := searcher.GroundedWebSearch(ctx, query)
		if err != nil {
			return "", err
		}
		if result == nil {
			return "", fmt.Errorf("grounded_web_search: empty result")
		}
		// Bound output: text (UTF-8-safe)
		text := result.Text
		if len(text) > maxGroundedWebSearchOutputChars {
			text = truncateUTF8(text, maxGroundedWebSearchOutputChars) + groundedWebSearchTruncateSuffix
		}
		citations := result.Citations
		if citations == nil {
			citations = []types.GroundedCitation{}
		}
		if len(citations) > maxGroundedWebSearchCitations {
			citations = citations[:maxGroundedWebSearchCitations]
		}
		// Bound each citation URL/title with UTF-8-safe truncation to defeat hostile searchers.
		boundedCitations := make([]types.GroundedCitation, len(citations))
		for i, c := range citations {
			boundedCitations[i] = types.GroundedCitation{
				URL:        truncateUTF8(c.URL, maxGroundedWebSearchURLChars),
				Title:      truncateUTF8(c.Title, maxGroundedWebSearchTitleChars),
				StartIndex: c.StartIndex,
				EndIndex:   c.EndIndex,
			}
		}
		// Structured bounded JSON with only text, citations, usage.
		out := struct {
			Text      string                   `json:"text"`
			Citations []types.GroundedCitation `json:"citations"`
			Usage     types.GroundedUsage      `json:"usage"`
		}{
			Text:      text,
			Citations: boundedCitations,
			Usage:     result.Usage,
		}
		data, err := json.Marshal(out)
		if err != nil {
			return "", fmt.Errorf("grounded_web_search: failed to marshal result: %w", err)
		}
		return string(data), nil
	}
}

func truncateUTF8(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	b := s[:maxBytes]
	for len(b) > 0 && !utf8.ValidString(b) {
		b = b[:len(b)-1]
	}
	return b
}

// RegisterGroundedWebSearchIfSupported conditionally registers the
// grounded_web_search tool. It skips registration when searcher is nil or does
// not support grounded search, leaving existing RegisterAll callers unchanged.
// Returns true if the tool was registered, false if skipped.
func RegisterGroundedWebSearchIfSupported(registry *tools.Registry, searcher types.GroundedWebSearcher) (bool, error) {
	if registry == nil {
		return false, fmt.Errorf("registry is nil")
	}
	if searcher == nil {
		return false, nil
	}
	if !searcher.SupportsGroundedWebSearch() {
		return false, nil
	}
	if registry.Has(groundedWebSearchToolName) {
		return false, nil
	}
	tool := GroundedWebSearchTool(searcher)
	if err := registry.Register(tool); err != nil {
		return false, err
	}
	return true, nil
}

// RegisterGroundedWebSearch is an alias for RegisterGroundedWebSearchIfSupported
// for callers that prefer the shorter name.
func RegisterGroundedWebSearch(registry *tools.Registry, searcher types.GroundedWebSearcher) (bool, error) {
	return RegisterGroundedWebSearchIfSupported(registry, searcher)
}
