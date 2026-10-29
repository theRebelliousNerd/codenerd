package context_harness

import (
	"math"
	"testing"
	"time"
)

func TestMetricsCollectorFinalize(t *testing.T) {
	m := NewMetricsCollector()

	m.RecordCompression(100, 20, 10*time.Millisecond)
	m.RecordCompression(50, 10, 20*time.Millisecond)

	m.RecordRetrieval(0.5, 0.25, 5*time.Millisecond)
	m.RecordRetrieval(1.0, 1.0, 15*time.Millisecond)

	m.RecordTokenBudgetViolation()
	m.RecordMemory(128.0)
	m.RecordMemory(64.0)

	metrics := m.Finalize()

	expectRatio := 5.0
	expectPrec := 0.75
	expectRecall := 0.625
	expectF1 := 2 * (expectPrec * expectRecall) / (expectPrec + expectRecall)

	if !approx(metrics.CompressionRatio, expectRatio, 0.01) {
		t.Fatalf("CompressionRatio = %.3f, want %.3f", metrics.CompressionRatio, expectRatio)
	}
	if !approx(metrics.AvgRetrievalPrec, expectPrec, 0.001) {
		t.Fatalf("AvgRetrievalPrec = %.3f, want %.3f", metrics.AvgRetrievalPrec, expectPrec)
	}
	if !approx(metrics.AvgRetrievalRecall, expectRecall, 0.001) {
		t.Fatalf("AvgRetrievalRecall = %.3f, want %.3f", metrics.AvgRetrievalRecall, expectRecall)
	}
	if !approx(metrics.AvgF1Score, expectF1, 0.001) {
		t.Fatalf("AvgF1Score = %.3f, want %.3f", metrics.AvgF1Score, expectF1)
	}
	if metrics.TokenBudgetViolations != 1 {
		t.Fatalf("TokenBudgetViolations = %d, want 1", metrics.TokenBudgetViolations)
	}
	if metrics.AvgCompressionLatency != 15*time.Millisecond {
		t.Fatalf("AvgCompressionLatency = %v, want 15ms", metrics.AvgCompressionLatency)
	}
	if metrics.AvgRetrievalLatency != 10*time.Millisecond {
		t.Fatalf("AvgRetrievalLatency = %v, want 10ms", metrics.AvgRetrievalLatency)
	}
	if metrics.PeakMemoryMB != 128.0 {
		t.Fatalf("PeakMemoryMB = %.1f, want 128.0", metrics.PeakMemoryMB)
	}
}

func approx(got, want, tol float64) bool {
	return math.Abs(got-want) <= tol
}
