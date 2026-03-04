package perception

import (
	"sync"
)

// LLMMetrics records aggregated statistics about LLM calls per shard or system.
type LLMMetrics struct {
	Calls      int64
	TokensUsed int64
	DurationMs int64
	Errors     int64
}

var (
	metricsMu sync.RWMutex
	metrics   = make(map[string]*LLMMetrics)
)

// RecordLLMCall increments the global LLM metric counters for a given shard category and type.
func RecordLLMCall(shardCategory, shardType string, tokensUsed int, durationMs int64, err error) {
	// Use the pair formatting from TracingLLMClient
	key := shardCategory + ":" + shardType

	metricsMu.Lock()
	defer metricsMu.Unlock()

	m, ok := metrics[key]
	if !ok {
		m = &LLMMetrics{}
		metrics[key] = m
	}

	m.Calls++
	m.TokensUsed += int64(tokensUsed)
	m.DurationMs += durationMs
	if err != nil {
		m.Errors++
	}
}

// GetLLMMetrics returns a snapshot of the current aggregated LLM call metrics.
func GetLLMMetrics() map[string]LLMMetrics {
	metricsMu.RLock()
	defer metricsMu.RUnlock()

	snapshot := make(map[string]LLMMetrics, len(metrics))
	for k, v := range metrics {
		snapshot[k] = *v
	}
	return snapshot
}
