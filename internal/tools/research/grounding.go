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

// maxURLContextURLs is the Gemini API's hard per-request URL Context limit.
// It is an external provider constraint, not a tunable: it must never move
// to config, and sending more is rejected by the provider. Helpers that hit
// it send the first maxURLContextURLs entries and hand the remainder back to
// the caller, so the limit is never a silent cut (limits cleanup 2026-09-29).
const maxURLContextURLs = 20

// GroundingHelper provides utilities for Gemini grounding features.
// Use NewGroundingHelper to create an instance from an LLM client.
type GroundingHelper struct {
	client        types.LLMClient
	controller    types.GroundingController // nil if client doesn't support grounding control
	provider      types.GroundingProvider   // nil if client doesn't support grounding
	isGrounding   bool
	mu            sync.RWMutex
	lastSources   []string
	lastDropped   []string
	totalSearches int
	totalURLs     int
	totalDropped  int
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
		// A method set is conclusive for a client nothing has wrapped, and
		// misleading for one that has been: the broker implements these
		// methods for EVERY client it meters, so the assertion above succeeds
		// whatever is underneath. A wrapper that reports its underlying
		// client's capability is believed over the shape of its own methods --
		// measured 2026-09-19, when all 11 runs on a Meta provider logged
		// "Gemini grounding: Google Search enabled" and fourteen call sites
		// took the grounded path against a client that grounds nothing.
		h.isGrounding = true
		if cap, ok := client.(types.GroundingCapable); ok {
			h.isGrounding = cap.SupportsGrounding()
		}
		if !h.isGrounding {
			h.controller = nil
		}
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
// At most maxURLContextURLs are sent (Gemini API limit, 34MB each); any
// remainder is returned so the caller can name the URLs that were not sent
// or issue a follow-up request with them. The limit is never silent.
// Dropped URLs are reported rather than queued because URL Context
// configures ONE provider request: queueing leftovers for an implicit later
// request would ground an unrelated completion against URLs the caller never
// chose for it. No-op if client doesn't support grounding control.
func (h *GroundingHelper) EnableURLContext(urls []string) []string {
	if h.controller == nil {
		return nil
	}
	sent, dropped := splitURLContextURLs(urls)
	if len(dropped) > 0 {
		logging.ResearcherWarn("Gemini grounding: %d of %d URLs not sent (API limit %d); first dropped: %s", len(dropped), len(urls), maxURLContextURLs, dropped[0])
	}
	h.controller.SetEnableURLContext(true)
	h.controller.SetURLContextURLs(sent)
	h.mu.Lock()
	h.totalURLs += len(sent)
	h.totalDropped += len(dropped)
	h.lastDropped = append([]string(nil), dropped...)
	h.mu.Unlock()
	logging.ResearcherDebug("Gemini grounding: URL Context enabled with %d URLs", len(sent))
	return dropped
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
// Like EnableURLContext it sends at most maxURLContextURLs and returns the
// remainder so the caller can account for every URL that was not sent.
func (h *GroundingHelper) SetURLContextURLs(urls []string) []string {
	if h.controller == nil {
		return nil
	}
	sent, dropped := splitURLContextURLs(urls)
	if len(dropped) > 0 {
		logging.ResearcherWarn("Gemini grounding: %d of %d URLs not sent (API limit %d); first dropped: %s", len(dropped), len(urls), maxURLContextURLs, dropped[0])
	}
	h.controller.SetURLContextURLs(sent)
	h.mu.Lock()
	h.totalDropped += len(dropped)
	h.lastDropped = append([]string(nil), dropped...)
	h.mu.Unlock()
	return dropped
}

// splitURLContextURLs divides URLs into the API-limited send set and the
// remainder the caller must account for. The remainder is nil when
// everything fits, so callers can test len(dropped) > 0.
func splitURLContextURLs(urls []string) (sent, dropped []string) {
	if len(urls) <= maxURLContextURLs {
		return urls, nil
	}
	return urls[:maxURLContextURLs], urls[maxURLContextURLs:]
}

// GetLastDroppedURLs returns the URLs withheld from the most recent
// EnableURLContext or SetURLContextURLs call by the API limit, for callers
// that did not capture the return value. It is empty when the last call sent
// everything.
func (h *GroundingHelper) GetLastDroppedURLs() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]string(nil), h.lastDropped...)
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
		TotalURLsDropped: h.totalDropped,
		LastSourcesCount: len(h.lastSources),
		IsGemini:         h.isGrounding,
	}
}

// GroundingStats contains usage statistics for grounding operations.
type GroundingStats struct {
	TotalSearches    int  `json:"total_searches"`
	TotalURLsUsed    int  `json:"total_urls_used"`
	TotalURLsDropped int  `json:"total_urls_dropped"`
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
	var dropped []string
	if len(docURLs) > 0 {
		dropped = h.EnableURLContext(docURLs)
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
		Query:          query,
		Response:       response,
		Sources:        sources,
		DocURLs:        docURLs,
		DroppedDocURLs: dropped,
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
	// DroppedDocURLs names the provided URLs withheld from the request by
	// the Gemini per-request URL limit. It is empty when everything fit;
	// the caller decides whether to run a follow-up request with them.
	DroppedDocURLs []string `json:"doc_urls_not_sent,omitempty"`
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
		"https://codeberg.org/TauCeti/mangle-go",
	},
	"rod": {
		"https://go-rod.github.io/",
		"https://pkg.go.dev/github.com/go-rod/rod",
	},
	"bubbletea": {
		"https://github.com/charmbracelet/bubbletea",
		"https://pkg.go.dev/github.com/charmbracelet/bubbletea",
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
