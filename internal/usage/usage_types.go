package usage

import (
	"time"

	"codenerd/internal/types"
)

// UsageData represents the root structure stored in persistence.
type UsageData struct {
	Version string `json:"version"`

	// Events is a bounded ring of the most recent transactions, retained only
	// when the tracker is created WithEventLog. It is never an exhaustive log;
	// aggregates are the durable record.
	Events    []UsageEvent    `json:"events,omitempty"`
	Aggregate AggregatedStats `json:"aggregate"`
}

// UsageEvent represents a single LLM transaction.
type UsageEvent struct {
	CredentialAlias string    `json:"credential_alias,omitempty"`
	Timestamp       time.Time `json:"timestamp"`
	Model           string    `json:"model"`
	Provider        string    `json:"provider"`
	InputTokens     int       `json:"input_tokens"`
	OutputTokens    int       `json:"output_tokens"`
	// CacheReadTokens/CacheWriteTokens are sub-counts of InputTokens: every
	// provider's input figure here already includes cached prompt tokens
	// (Anthropic's is folded by anthropicUsage.promptTokens), so they are
	// recorded for visibility and pricing, never added to Total.
	CacheReadTokens  int `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int `json:"cache_write_tokens,omitempty"`
	// ReasoningTokens is the reasoning sub-count of OutputTokens, when the
	// provider reports one. Never added to a total.
	ReasoningTokens int `json:"reasoning_tokens,omitempty"`
	// Known names the categories the provider reported (types.UsageFields), so
	// a zero cache count reads as "observed zero" only when it was observed.
	Known types.UsageFields `json:"known,omitempty"`
	// Partial marks a report whose counts are lower bounds: the stream ended
	// before the provider's final usage block.
	Partial       bool    `json:"partial,omitempty"`
	ShardType     string  `json:"shard_type"` // ephemeral, specialist, system, user
	ShardName     string  `json:"shard_name"`
	SessionID     string  `json:"session_id"`
	OperationType string  `json:"operation_type"`     // chat, embedding, tool_gen
	CostUSD       float64 `json:"cost_usd,omitempty"` // estimate; 0 when the model is unpriced
}

// AggregatedStats holds counters broken down by various dimensions.
type AggregatedStats struct {
	ByCredential map[string]TokenCounts `json:"by_credential,omitempty"`
	TotalProject TokenCounts            `json:"total_project"`
	ByProvider   map[string]TokenCounts `json:"by_provider"`
	ByModel      map[string]TokenCounts `json:"by_model"`
	ByShardType  map[string]TokenCounts `json:"by_shard_type"` // ephemeral, specialist, system
	ByShardName  map[string]TokenCounts `json:"by_shard_name"` // specialist-level spend
	ByOperation  map[string]TokenCounts `json:"by_operation"`  // chat, embedding
	BySession    map[string]TokenCounts `json:"by_session"`

	// UnpricedTokens counts tokens spent on models absent from the price table.
	// Without it a small Cost total is ambiguous between "cheap" and "unpriced".
	UnpricedTokens int64 `json:"unpriced_tokens,omitempty"`
}

// TokenCounts holds input/output sums and an estimated cost.
type TokenCounts struct {
	Input  int64   `json:"input"`
	Output int64   `json:"output"`
	Total  int64   `json:"total"`
	Cost   float64 `json:"cost_est_usd,omitempty"` // USD estimate, not billing truth
	// CacheRead/CacheWrite are sub-counts of Input and Reasoning of Output
	// (see types.UsageMetadata), kept so a reader can tell how much of the
	// spend was cache traffic or reasoning. None is ever added to Total.
	CacheRead  int64 `json:"cache_read,omitempty"`
	CacheWrite int64 `json:"cache_write,omitempty"`
	Reasoning  int64 `json:"reasoning,omitempty"`
	// PartialReports counts reports whose counts are lower bounds, so a total
	// built from them is visibly incomplete rather than quietly cheap.
	PartialReports int64 `json:"partial_reports,omitempty"`
}

// AddUsage accumulates one normalized report and its estimated USD cost.
func (tc *TokenCounts) AddUsage(u types.UsageMetadata, cost float64) {
	tc.Input += int64(u.InputTokens)
	tc.Output += int64(u.OutputTokens)
	tc.Total += int64(u.InputTokens + u.OutputTokens)
	tc.CacheRead += int64(u.CachedContentTokens)
	tc.CacheWrite += int64(u.CacheWriteTokens)
	tc.Reasoning += int64(u.ThinkingTokens)
	if u.Partial {
		tc.PartialReports++
	}
	tc.Cost += cost
}
