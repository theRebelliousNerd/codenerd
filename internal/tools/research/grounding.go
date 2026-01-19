// Package research provides research tools including Gemini grounding support.
//
// Gemini Grounding enables LLM responses to be grounded with real-time information:
//   - Google Search: Ground responses with live search results
//   - URL Context: Ground responses with specific documentation URLs (max 20)
//
// This file provides helpers for any system (init, shards, campaigns) to use
// Gemini's built-in grounding when available.
package research

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// GroundingHelper provides utilities for Gemini grounding features.
// Use NewGroundingHelper to create an instance from an LLM client.
type GroundingHelper struct {
	client        types.LLMClient
	controller    types.GroundingController // nil if client doesn't support grounding control
	provider      types.GroundingProvider   // nil if client doesn't support grounding
	isGrounding   bool
	mu            sync.RWMutex
	lastSources   []string
	totalSearches int
	totalURLs     int
}

// NewGroundingHelper creates a grounding helper from an LLM client.
// Returns a helper that works with any client - grounding features are
// only active when the client implements GroundingController.
func NewGroundingHelper(client types.LLMClient) *GroundingHelper {
	h := &GroundingHelper{
		client:      client,
		lastSources: make([]string, 0),
	}

	// Check if client supports grounding control (Gemini)
	if gc, ok := client.(types.GroundingController); ok {
		h.controller = gc
		h.provider = gc
		h.isGrounding = true
	} else if gp, ok := client.(types.GroundingProvider); ok {
		// Read-only grounding access
		h.provider = gp
	}

	return h
}

// IsGemini returns true if the underlying client supports grounding control.
// This is typically only true for Gemini clients.
func (h *GroundingHelper) IsGemini() bool {
	return h.isGrounding
}

// IsGroundingAvailable returns true if grounding features can be used.
func (h *GroundingHelper) IsGroundingAvailable() bool {
	return h.isGrounding && h.controller != nil
}

// EnableGoogleSearch enables Google Search grounding for subsequent calls.
// No-op if client doesn't support grounding control.
func (h *GroundingHelper) EnableGoogleSearch() {
	if h.controller != nil {
		h.controller.SetEnableGoogleSearch(true)
		logging.ResearcherDebug("Gemini grounding: Google Search enabled")
	}
}

// DisableGoogleSearch disables Google Search grounding.
func (h *GroundingHelper) DisableGoogleSearch() {
	if h.controller != nil {
		h.controller.SetEnableGoogleSearch(false)
		logging.ResearcherDebug("Gemini grounding: Google Search disabled")
	}
}

// EnableURLContext enables URL Context grounding with the specified URLs.
// Max 20 URLs, 34MB each per Gemini API limits.
// No-op if client doesn't support grounding control.
func (h *GroundingHelper) EnableURLContext(urls []string) {
	if h.controller != nil {
		// Enforce API limit
		if len(urls) > 20 {
			logging.ResearcherWarn("Gemini grounding: truncating URLs from %d to 20 (API limit)", len(urls))
			urls = urls[:20]
		}
		h.controller.SetEnableURLContext(true)
		h.controller.SetURLContextURLs(urls)
		h.mu.Lock()
		h.totalURLs += len(urls)
		h.mu.Unlock()
		logging.ResearcherDebug("Gemini grounding: URL Context enabled with %d URLs", len(urls))
	}
}

// DisableURLContext disables URL Context grounding.
func (h *GroundingHelper) DisableURLContext() {
	if h.controller != nil {
		h.controller.SetEnableURLContext(false)
		h.controller.SetURLContextURLs(nil)
		logging.ResearcherDebug("Gemini grounding: URL Context disabled")
	}
}

// SetURLContextURLs updates the URLs for URL Context grounding.
func (h *GroundingHelper) SetURLContextURLs(urls []string) {
	if h.controller != nil {
		if len(urls) > 20 {
			urls = urls[:20]
		}
		h.controller.SetURLContextURLs(urls)
	}
}

// IsGoogleSearchEnabled returns whether Google Search grounding is active.
func (h *GroundingHelper) IsGoogleSearchEnabled() bool {
	if h.provider != nil {
		return h.provider.IsGoogleSearchEnabled()
	}
	return false
}

// IsURLContextEnabled returns whether URL Context grounding is active.
func (h *GroundingHelper) IsURLContextEnabled() bool {
	if h.provider != nil {
		return h.provider.IsURLContextEnabled()
	}
	return false
}

// CaptureGroundingSources captures grounding sources after an LLM call.
// Call this after Complete/CompleteWithSystem to get the sources used.
func (h *GroundingHelper) CaptureGroundingSources() []string {
	if h.provider != nil {
		sources := h.provider.GetLastGroundingSources()
		h.mu.Lock()
		h.lastSources = sources
		if len(sources) > 0 {
			h.totalSearches++
		}
		h.mu.Unlock()
		return sources
	}
	return nil
}

// GetLastGroundingSources returns the sources from the last capture.
func (h *GroundingHelper) GetLastGroundingSources() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastSources
}

// GetStats returns grounding usage statistics.
func (h *GroundingHelper) GetStats() GroundingStats {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return GroundingStats{
		TotalSearches:    h.totalSearches,
		TotalURLsUsed:    h.totalURLs,
		LastSourcesCount: len(h.lastSources),
		IsGemini:         h.isGrounding,
	}
}

// GroundingStats contains usage statistics for grounding operations.
type GroundingStats struct {
	TotalSearches    int  `json:"total_searches"`
	TotalURLsUsed    int  `json:"total_urls_used"`
	LastSourcesCount int  `json:"last_sources_count"`
	IsGemini         bool `json:"is_gemini"`
}

// CompleteWithGrounding performs an LLM completion with grounding enabled.
// Automatically captures grounding sources after the call.
// If client doesn't support grounding, performs regular completion.
func (h *GroundingHelper) CompleteWithGrounding(ctx context.Context, prompt string) (string, []string, error) {
	response, err := h.client.Complete(ctx, prompt)
	if err != nil {
		return "", nil, err
	}

	sources := h.CaptureGroundingSources()
	return response, sources, nil
}

// CompleteWithSystemAndGrounding performs an LLM completion with system prompt
// and grounding enabled. Automatically captures grounding sources.
func (h *GroundingHelper) CompleteWithSystemAndGrounding(ctx context.Context, systemPrompt, userPrompt string) (string, []string, error) {
	response, err := h.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", nil, err
	}

	sources := h.CaptureGroundingSources()
	return response, sources, nil
}

// GroundedResearch performs research with optimal grounding configuration.
// Enables Google Search, optionally adds documentation URLs, performs the query,
// and returns results with sources.
func (h *GroundingHelper) GroundedResearch(ctx context.Context, query string, docURLs []string) (*GroundedResearchResult, error) {
	// Enable grounding
	h.EnableGoogleSearch()
	if len(docURLs) > 0 {
		h.EnableURLContext(docURLs)
	}

	// Build research prompt
	prompt := fmt.Sprintf(`Research the following topic and provide accurate, up-to-date information.

Topic: %s

Provide a comprehensive answer with specific details. If you use information from web searches or documentation, ensure accuracy.`, query)

	response, sources, err := h.CompleteWithGrounding(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("grounded research failed: %w", err)
	}

	result := &GroundedResearchResult{
		Query:    query,
		Response: response,
		Sources:  sources,
		DocURLs:  docURLs,
	}

	// Log results
	if len(sources) > 0 {
		logging.Researcher("Grounded research completed: %d sources used for %q", len(sources), truncateQuery(query))
	}

	return result, nil
}

// GroundedResearchResult contains the results of a grounded research query.
type GroundedResearchResult struct {
	Query    string   `json:"query"`
	Response string   `json:"response"`
	Sources  []string `json:"sources"`
	DocURLs  []string `json:"doc_urls_provided"`
}

// FormatSourcesMarkdown formats grounding sources as markdown for display.
func FormatSourcesMarkdown(sources []string) string {
	if len(sources) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n**Sources:**\n")
	for _, src := range sources {
		sb.WriteString(fmt.Sprintf("- %s\n", src))
	}
	return sb.String()
}

// truncateQuery truncates a query for logging.
func truncateQuery(q string) string {
	if len(q) > 50 {
		return q[:47] + "..."
	}
	return q
}

// =============================================================================
// Documentation URL Helpers
// =============================================================================

// CommonDocURLs provides well-known documentation URLs for common technologies.
// These can be passed to EnableURLContext for grounding.
var CommonDocURLs = map[string][]string{
	"go": {
		"https://go.dev/doc/",
		"https://pkg.go.dev/std",
		"https://go.dev/blog/",
	},
	"python": {
		"https://docs.python.org/3/",
		"https://peps.python.org/",
	},
	"typescript": {
		"https://www.typescriptlang.org/docs/",
	},
	"react": {
		"https://react.dev/reference/react",
		"https://react.dev/learn",
	},
	"mangle": {
		"https://github.com/google/mangle",
	},
	"rod": {
		"https://go-rod.github.io/",
		"https://pkg.go.dev/github.com/go-rod/rod",
	},
	"bubbletea": {
		"https://github.com/charmbracelet/bubbletea",
		"https://pkg.go.dev/github.com/charmbracelet/bubbletea",
	},
	"antigravity": {
		"https://cloud.google.com/code/docs",
		"https://cloud.google.com/gemini/docs",
	},
	"cloudcode": {
		"https://cloud.google.com/code/docs",
	},
}

// GetDocURLsForTech returns documentation URLs for a technology.
func GetDocURLsForTech(tech string) []string {
	tech = strings.ToLower(tech)
	if urls, ok := CommonDocURLs[tech]; ok {
		return urls
	}
	return nil
}

// GetDocURLsForTechs returns documentation URLs for multiple technologies.
// Deduplicates and limits to 20 URLs (Gemini API limit).
func GetDocURLsForTechs(techs []string) []string {
	seen := make(map[string]bool)
	var urls []string

	for _, tech := range techs {
		for _, url := range GetDocURLsForTech(tech) {
			if !seen[url] && len(urls) < 20 {
				seen[url] = true
				urls = append(urls, url)
			}
		}
	}

	return urls
}
