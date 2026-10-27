package usage

import "time"

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
	CacheReadTokens  int     `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens int     `json:"cache_write_tokens,omitempty"`
	ShardType        string  `json:"shard_type"` // ephemeral, specialist, system, user
	ShardName        string  `json:"shard_name"`
	SessionID        string  `json:"session_id"`
	OperationType    string  `json:"operation_type"`     // chat, embedding, tool_gen
	CostUSD          float64 `json:"cost_usd,omitempty"` // estimate; 0 when the model is unpriced
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
	// CacheRead/CacheWrite are sub-counts of Input (see UsageEvent), kept
	// so a reader can tell how much of the prompt spend was cache traffic.
	CacheRead  int64 `json:"cache_read,omitempty"`
	CacheWrite int64 `json:"cache_write,omitempty"`
}

// Add accumulates tokens without cost attribution.
func (tc *TokenCounts) Add(input, output int) {
	tc.AddCost(input, output, 0)
}

// AddCost accumulates tokens and an estimated USD cost.
func (tc *TokenCounts) AddCost(input, output int, cost float64) {
	tc.AddDetailed(input, output, 0, 0, cost)
}

// AddDetailed is AddCost with the cache sub-counts of input. They are not
// added to Total: input already contains them.
func (tc *TokenCounts) AddDetailed(input, output, cacheRead, cacheWrite int, cost float64) {
	tc.Input += int64(input)
	tc.Output += int64(output)
	tc.Total += int64(input + output)
	tc.CacheRead += int64(cacheRead)
	tc.CacheWrite += int64(cacheWrite)
	tc.Cost += cost
}
