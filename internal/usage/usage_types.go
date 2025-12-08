package usage

import "time"

// UsageData represents the root structure stored in persistence.
type UsageData struct {
	Version   string          `json:"version"`
	Events    []UsageEvent    `json:"events,omitempty"` // Optional: keep raw events? potentially too large.
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
	OperationType string    `json:"operation_type"` // chat, embedding, tool_gen
}

// AggregatedStats holds counters broken down by various dimensions.
type AggregatedStats struct {
	TotalProject TokenCounts            `json:"total_project"`
	ByProvider   map[string]TokenCounts `json:"by_provider"`
	ByModel      map[string]TokenCounts `json:"by_model"`
	ByShardType  map[string]TokenCounts `json:"by_shard_type"` // ephemeral, specialist, system
	ByOperation  map[string]TokenCounts `json:"by_operation"`  // chat, embedding
	BySession    map[string]TokenCounts `json:"by_session"`
}

// TokenCounts holds input/output sums.
type TokenCounts struct {
	Input  int64   `json:"input"`
	Output int64   `json:"output"`
	Total  int64   `json:"total"`
	Cost   float64 `json:"cost_est_usd,omitempty"` // Optional
}

func (tc *TokenCounts) Add(input, output int) {
	tc.Input += int64(input)
	tc.Output += int64(output)
	tc.Total += int64(input + output)
}
