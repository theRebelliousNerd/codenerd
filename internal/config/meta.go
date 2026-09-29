package config

import (
	"fmt"
	"strings"
)

// MetaProviderConfig is the Meta Model API (Muse Spark) block.
//
// Web search is a Responses-API tool, billed per search, so whether a Meta
// client grounds is a config fact rather than a decision a turn makes in Go.
// Chat Completions cannot carry the tool: with search on, the client sends
// every request to /responses.
type MetaProviderConfig struct {
	// EnableWebSearch attaches the web_search tool. A nil pointer means the
	// default (on). An explicit false keeps plain completions and the first
	// tool turn on Chat Completions, which is what a Meta client did before
	// grounding existed.
	EnableWebSearch *bool `json:"enable_web_search,omitempty"`

	// SearchContextSize is how much retrieved page text Meta attaches to the
	// tool call: low, medium, or high. Empty means the default (medium).
	SearchContextSize string `json:"search_context_size,omitempty"`
}

// DefaultMetaProviderConfig returns search grounding on at medium context.
func DefaultMetaProviderConfig() *MetaProviderConfig {
	return &MetaProviderConfig{
		EnableWebSearch:   boolConfigPointer(true),
		SearchContextSize: "medium",
	}
}

// WebSearchEnabled reports the resolved switch. Nil means on.
func (m *MetaProviderConfig) WebSearchEnabled() bool {
	if m == nil || m.EnableWebSearch == nil {
		return true
	}
	return *m.EnableWebSearch
}

// ResolvedSearchContextSize returns the size the client should send.
// An empty value is the default. An invalid value is returned unchanged so
// Check and the client constructor can refuse it; this method does not invent
// a legal size for a value the file got wrong.
func (m *MetaProviderConfig) ResolvedSearchContextSize() string {
	if m == nil {
		return "medium"
	}
	size := strings.ToLower(strings.TrimSpace(m.SearchContextSize))
	if size == "" {
		return "medium"
	}
	return size
}

// Check rejects a search_context_size outside the vendor's vocabulary.
// An empty size is the default, not an error. The bool has no illegal value.
func (m *MetaProviderConfig) Check(path string) []Problem {
	if m == nil {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(m.SearchContextSize)) {
	case "", "low", "medium", "high":
		return nil
	default:
		return []Problem{{
			Severity: SeverityError,
			Path:     path + ".search_context_size",
			Message:  fmt.Sprintf("%q is not a search context size", m.SearchContextSize),
			Fix:      "low, medium, or high",
		}}
	}
}
