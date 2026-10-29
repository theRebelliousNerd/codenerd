package research

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"codenerd/internal/tools"
	"codenerd/internal/types"
)

const (
	groundedWebSearchToolName = "grounded_web_search"
	// maxGroundedWebSearchQueryChars is a loud input-validation bound: an
	// overlong query is rejected with an error naming the limit, never cut.
	// It guards the provider request against pathological input, so it stays
	// a Go literal rather than a tunable (limits cleanup 2026-09-29).
	maxGroundedWebSearchQueryChars = 10000
)

// GroundedWebSearchTool returns a provider-neutral tool that performs a
// Meta-native grounded web search via the injected GroundedWebSearcher.
// The tool takes a required string query, invokes the searcher with the exact
// query, and returns the answer text, citations, retrieved pages and usage whole as structured
// JSON: since the working-context ledger (internal/context working_set.mg)
// archives large tool results behind recall handles, this tool never cuts
// what the ledger must size for the window. It never returns config or
// credentials.
func GroundedWebSearchTool(searcher types.GroundedWebSearcher) *tools.Tool {
	// Capture searcher in closure; registration helper guarantees non-nil and
	// supported before this is registered, but Execute still defensively checks.
	return &tools.Tool{
		Name:        groundedWebSearchToolName,
		Description: "Perform a grounded web search using the configured LLM provider (Meta). Returns the grounded answer text, the URL citations the answer rests on, every page the search retrieved (title, url, snippet), and token usage. Read a retrieved page in full with web_fetch.",
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
		// Whole output: the answer text and every citation are returned
		// uncut. A silently shortened URL is a broken link, not a safety
		// bound, and the working-context ledger sizes tool results for the
		// window behind recall handles, so cutting here only destroys
		// evidence (limits cleanup 2026-09-29).
		text := result.Text
		citations := result.Citations
		if citations == nil {
			citations = []types.GroundedCitation{}
		}
		// The retrieved pages ride along: citations are the subset the answer
		// cites, results are everything the search found, and a model that
		// wants to read a page in full needs its URL.
		results := result.Results
		if results == nil {
			results = []types.GroundedSearchResult{}
		}
		out := struct {
			Text      string                       `json:"text"`
			Citations []types.GroundedCitation     `json:"citations"`
			Results   []types.GroundedSearchResult `json:"results"`
			Usage     types.GroundedUsage          `json:"usage"`
		}{
			Text:      text,
			Citations: citations,
			Results:   results,
			Usage:     result.Usage,
		}
		data, err := json.Marshal(out)
		if err != nil {
			return "", fmt.Errorf("grounded_web_search: failed to marshal result: %w", err)
		}
		return string(data), nil
	}
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
