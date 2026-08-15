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
	Timestamp     time.Time `json:"timestamp"`
	Model         string    `json:"model"`
	Provider      string    `json:"provider"`
	InputTokens   int       `json:"input_tokens"`
	OutputTokens  int       `json:"output_tokens"`
	ShardType     string    `json:"shard_type"` // ephemeral, specialist, system, user
	ShardName     string    `json:"shard_name"`
	SessionID     string    `json:"session_id"`
	OperationType string    `json:"operation_type"`     // chat, embedding, tool_gen
	CostUSD       float64   `json:"cost_usd,omitempty"` // estimate; 0 when the model is unpriced
}

// AggregatedStats holds counters broken down by various dimensions.
type AggregatedStats struct {
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
}

// Add accumulates tokens without cost attribution.
func (tc *TokenCounts) Add(input, output int) {
	tc.AddCost(input, output, 0)
}

// AddCost accumulates tokens and an estimated USD cost.
func (tc *TokenCounts) AddCost(input, output int, cost float64) {
	tc.Input += int64(input)
	tc.Output += int64(output)
	tc.Total += int64(input + output)
	tc.Cost += cost
}
