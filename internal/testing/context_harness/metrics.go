package context_harness

import (
	"sync"
	"time"
)

// MetricsCollector aggregates performance metrics during scenario execution.
type MetricsCollector struct {
	mu sync.Mutex

	// Compression metrics
	totalOriginalTokens   int
	totalCompressedTokens int
	compressionLatencies  []time.Duration

	// Retrieval metrics
	retrievalPrecisions []float64
	retrievalRecalls    []float64
	retrievalLatencies  []time.Duration

	// Token budget
	tokenBudgetViolations int

	// Memory
	peakMemoryMB float64
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		compressionLatencies: make([]time.Duration, 0),
		retrievalPrecisions:  make([]float64, 0),
		retrievalRecalls:     make([]float64, 0),
		retrievalLatencies:   make([]time.Duration, 0),
	}
}

// RecordCompression records compression metrics for a turn.
func (m *MetricsCollector) RecordCompression(originalTokens, compressedTokens int, latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalOriginalTokens += originalTokens
	m.totalCompressedTokens += compressedTokens
	m.compressionLatencies = append(m.compressionLatencies, latency)
}

// RecordRetrieval records retrieval metrics for a query.
func (m *MetricsCollector) RecordRetrieval(precision, recall float64, latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.retrievalPrecisions = append(m.retrievalPrecisions, precision)
	m.retrievalRecalls = append(m.retrievalRecalls, recall)
	m.retrievalLatencies = append(m.retrievalLatencies, latency)
}

// RecordTokenBudgetViolation records when token budget is exceeded.
func (m *MetricsCollector) RecordTokenBudgetViolation() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tokenBudgetViolations++
}

// RecordMemory records peak memory usage.
func (m *MetricsCollector) RecordMemory(memoryMB float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if memoryMB > m.peakMemoryMB {
		m.peakMemoryMB = memoryMB
	}
}

// Finalize computes aggregate metrics.
func (m *MetricsCollector) Finalize() Metrics {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics := Metrics{
		TokenBudgetViolations: m.tokenBudgetViolations,
		PeakMemoryMB:          m.peakMemoryMB,
	}

	// Compression ratio
	if m.totalCompressedTokens > 0 {
		metrics.CompressionRatio = float64(m.totalOriginalTokens) / float64(m.totalCompressedTokens)
	}

	// Average retrieval precision
	if len(m.retrievalPrecisions) > 0 {
		sum := 0.0
		for _, p := range m.retrievalPrecisions {
			sum += p
		}
		metrics.AvgRetrievalPrec = sum / float64(len(m.retrievalPrecisions))
	}

	// Average retrieval recall
	if len(m.retrievalRecalls) > 0 {
		sum := 0.0
		for _, r := range m.retrievalRecalls {
			sum += r
		}
		metrics.AvgRetrievalRecall = sum / float64(len(m.retrievalRecalls))
	}

	// F1 score
	if metrics.AvgRetrievalPrec+metrics.AvgRetrievalRecall > 0 {
		metrics.AvgF1Score = 2 * (metrics.AvgRetrievalPrec * metrics.AvgRetrievalRecall) /
			(metrics.AvgRetrievalPrec + metrics.AvgRetrievalRecall)
	}

	// Average compression latency
	if len(m.compressionLatencies) > 0 {
		sum := time.Duration(0)
		for _, lat := range m.compressionLatencies {
			sum += lat
		}
		metrics.AvgCompressionLatency = sum / time.Duration(len(m.compressionLatencies))
	}

	// Average retrieval latency
	if len(m.retrievalLatencies) > 0 {
		sum := time.Duration(0)
		for _, lat := range m.retrievalLatencies {
			sum += lat
		}
		metrics.AvgRetrievalLatency = sum / time.Duration(len(m.retrievalLatencies))
	}

	return metrics
}
