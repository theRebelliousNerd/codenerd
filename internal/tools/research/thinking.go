// Package research provides research tools including Gemini thinking support.
//
// ThinkingHelper enables access to Gemini 3's thinking mode features:
//   - Thought Summary: The model's reasoning process (for SPL learning)
//   - Thought Signature: Encrypted blob for multi-turn function calling continuity
//   - Thinking Tokens: Token count for budget monitoring
//
// This file provides helpers for any system (shards, campaigns) to access
// Gemini's thinking mode metadata when available.
package research

import (
	"sync"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// ThinkingHelper provides utilities for Gemini thinking mode features.
// Use NewThinkingHelper to create an instance from an LLM client.
type ThinkingHelper struct {
	client            types.LLMClient
	thinkingProvider  types.ThinkingProvider         // nil if client doesn't support thinking
	signatureProvider types.ThoughtSignatureProvider // nil if client doesn't support signatures
	isThinking        bool
	mu                sync.RWMutex

	// Captured metadata
	lastThoughtSummary   string
	lastThoughtSignature string
	lastThinkingTokens   int
	totalThinkingTokens  int64
	totalCaptures        int
}

// NewThinkingHelper creates a thinking helper from an LLM client.
// Returns a helper that works with any client - thinking features are
// only active when the client implements ThinkingProvider.
func NewThinkingHelper(client types.LLMClient) *ThinkingHelper {
	h := &ThinkingHelper{
		client: client,
	}

	// Check if client supports thinking mode
	if tp, ok := client.(types.ThinkingProvider); ok {
		h.thinkingProvider = tp
		h.isThinking = tp.IsThinkingEnabled()
	}

	// Check if client supports thought signatures
	if tsp, ok := client.(types.ThoughtSignatureProvider); ok {
		h.signatureProvider = tsp
	}

	return h
}

// IsThinkingAvailable returns true if thinking mode is enabled.
func (h *ThinkingHelper) IsThinkingAvailable() bool {
	return h.isThinking && h.thinkingProvider != nil
}

// IsSignatureAvailable returns true if thought signatures can be used.
func (h *ThinkingHelper) IsSignatureAvailable() bool {
	return h.signatureProvider != nil
}

// GetThinkingLevel returns the current thinking level (e.g., "minimal", "low", "medium", "high").
// Returns empty string if thinking mode is disabled.
func (h *ThinkingHelper) GetThinkingLevel() string {
	if h.thinkingProvider != nil {
		return h.thinkingProvider.GetThinkingLevel()
	}
	return ""
}

// CaptureThinkingMetadata captures thinking metadata after an LLM call.
// Call this after Complete/CompleteWithSystem/CompleteWithTools to get:
// - Thought summary (reasoning process)
// - Thought signature (for multi-turn function calling)
// - Thinking tokens used
func (h *ThinkingHelper) CaptureThinkingMetadata() ThinkingMetadata {
	var metadata ThinkingMetadata

	if h.thinkingProvider != nil {
		metadata.ThoughtSummary = h.thinkingProvider.GetLastThoughtSummary()
		metadata.ThinkingTokens = h.thinkingProvider.GetLastThinkingTokens()
		metadata.ThinkingLevel = h.thinkingProvider.GetThinkingLevel()
	}

	if h.signatureProvider != nil {
		metadata.ThoughtSignature = h.signatureProvider.GetLastThoughtSignature()
	}

	// Update internal state
	h.mu.Lock()
	h.lastThoughtSummary = metadata.ThoughtSummary
	h.lastThoughtSignature = metadata.ThoughtSignature
	h.lastThinkingTokens = metadata.ThinkingTokens
	h.totalThinkingTokens += int64(metadata.ThinkingTokens)
	h.totalCaptures++
	h.mu.Unlock()

	if metadata.ThinkingTokens > 0 {
		logging.ResearcherDebug("Thinking mode: captured %d tokens, level=%s",
			metadata.ThinkingTokens, metadata.ThinkingLevel)
	}

	return metadata
}

// GetLastThoughtSummary returns the thought summary from the last capture.
func (h *ThinkingHelper) GetLastThoughtSummary() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastThoughtSummary
}

// GetLastThoughtSignature returns the thought signature from the last capture.
// This should be passed back in multi-turn function calling scenarios.
func (h *ThinkingHelper) GetLastThoughtSignature() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastThoughtSignature
}

// GetLastThinkingTokens returns the token count from the last capture.
func (h *ThinkingHelper) GetLastThinkingTokens() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastThinkingTokens
}

// GetStats returns thinking usage statistics.
func (h *ThinkingHelper) GetStats() ThinkingStats {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return ThinkingStats{
		TotalThinkingTokens: h.totalThinkingTokens,
		TotalCaptures:       h.totalCaptures,
		LastTokenCount:      h.lastThinkingTokens,
		IsEnabled:           h.isThinking,
		ThinkingLevel:       h.GetThinkingLevel(),
	}
}

// ThinkingMetadata contains metadata captured from a thinking-enabled response.
type ThinkingMetadata struct {
	// ThoughtSummary is the model's reasoning process (for SPL learning).
	// May be empty if the model didn't produce thoughts for this response.
	ThoughtSummary string `json:"thought_summary,omitempty"`

	// ThoughtSignature is an encrypted blob for multi-turn function calling.
	// Must be passed back in subsequent turns to maintain reasoning continuity.
	ThoughtSignature string `json:"thought_signature,omitempty"`

	// ThinkingTokens is the number of tokens used for reasoning.
	// Useful for budget monitoring and cost estimation.
	ThinkingTokens int `json:"thinking_tokens,omitempty"`

	// ThinkingLevel is the configured level (e.g., "minimal", "low", "medium", "high").
	ThinkingLevel string `json:"thinking_level,omitempty"`
}

// ThinkingStats contains usage statistics for thinking operations.
type ThinkingStats struct {
	TotalThinkingTokens int64  `json:"total_thinking_tokens"`
	TotalCaptures       int    `json:"total_captures"`
	LastTokenCount      int    `json:"last_token_count"`
	IsEnabled           bool   `json:"is_enabled"`
	ThinkingLevel       string `json:"thinking_level"`
}

// =============================================================================
// Multi-Turn Function Calling Support
// =============================================================================

// MultiTurnContext maintains context for multi-turn function calling.
// Use this when executing tool calls and passing results back to the model.
type MultiTurnContext struct {
	ThoughtSignature string `json:"thought_signature,omitempty"`
	TurnNumber       int    `json:"turn_number"`
}

// NewMultiTurnContext creates a context for multi-turn function calling.
// Call this after getting a response with tool calls.
func (h *ThinkingHelper) NewMultiTurnContext() *MultiTurnContext {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return &MultiTurnContext{
		ThoughtSignature: h.lastThoughtSignature,
		TurnNumber:       1,
	}
}

// UpdateFromResponse updates the multi-turn context with new metadata.
// Call this after each turn to capture new thought signatures.
func (mtc *MultiTurnContext) UpdateFromResponse(helper *ThinkingHelper) {
	helper.CaptureThinkingMetadata()
	helper.mu.RLock()
	defer helper.mu.RUnlock()
	if helper.lastThoughtSignature != "" {
		mtc.ThoughtSignature = helper.lastThoughtSignature
	}
	mtc.TurnNumber++
}

// HasSignature returns true if a thought signature is available.
func (mtc *MultiTurnContext) HasSignature() bool {
	return mtc.ThoughtSignature != ""
}
