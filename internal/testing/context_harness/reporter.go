package context_harness

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Reporter formats and outputs test results.
type Reporter struct {
	writer io.Writer
	format string // "console" or "json"
}

// NewReporter creates a new reporter.
func NewReporter(writer io.Writer, format string) *Reporter {
	return &Reporter{
		writer: writer,
		format: format,
	}
}

// Report outputs the test result.
func (r *Reporter) Report(result *TestResult) error {
	if r.format == "json" {
		return r.reportJSON(result)
	}
	return r.reportConsole(result)
}

// reportJSON outputs the result as JSON.
func (r *Reporter) reportJSON(result *TestResult) error {
	encoder := json.NewEncoder(r.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

// reportConsole outputs a human-readable report.
func (r *Reporter) reportConsole(result *TestResult) error {
	var sb strings.Builder

	// Header
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("  CONTEXT TEST HARNESS REPORT: %s\n", result.Scenario.Name))
	sb.WriteString("═══════════════════════════════════════════════════════════════\n\n")

	// Overall status
	if result.Passed {
		sb.WriteString("✓ STATUS: PASSED\n\n")
	} else {
		sb.WriteString("✗ STATUS: FAILED\n\n")
		if len(result.FailureReasons) > 0 {
			sb.WriteString("Failure Reasons:\n")
			for i, reason := range result.FailureReasons {
				sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, reason))
			}
			sb.WriteString("\n")
		}
	}

	// Metrics
	sb.WriteString("METRICS:\n")
	sb.WriteString("───────────────────────────────────────────────────────────────\n")
	m := result.ActualMetrics
	sb.WriteString(fmt.Sprintf("  Encoding Ratio:           %.2fx (< 1.0 = enrichment, > 1.0 = compression)\n", m.CompressionRatio))
	sb.WriteString(fmt.Sprintf("  Avg Retrieval Precision:  %.2f%%\n", m.AvgRetrievalPrec*100))
	sb.WriteString(fmt.Sprintf("  Avg Retrieval Recall:     %.2f%%\n", m.AvgRetrievalRecall*100))
	sb.WriteString(fmt.Sprintf("  Avg F1 Score:             %.2f%%\n", m.AvgF1Score*100))
	sb.WriteString(fmt.Sprintf("  Token Budget Violations:  %d\n", m.TokenBudgetViolations))
	sb.WriteString(fmt.Sprintf("  Avg Compression Latency:  %v\n", m.AvgCompressionLatency))
	sb.WriteString(fmt.Sprintf("  Avg Retrieval Latency:    %v\n", m.AvgRetrievalLatency))
	sb.WriteString(fmt.Sprintf("  Peak Memory:              %.2f MB\n", m.PeakMemoryMB))
	sb.WriteString("\n")

	// Expected vs Actual
	sb.WriteString("EXPECTED vs ACTUAL:\n")
	sb.WriteString("───────────────────────────────────────────────────────────────\n")
	exp := result.Scenario.ExpectedMetrics
	// For enrichment (expected < 1.0): lower actual is acceptable (more enrichment)
	// For compression (expected > 1.0): higher actual is better (more compression)
	compressionOK := false
	if exp.CompressionRatio < 1.0 {
		// Enrichment mode: actual should be <= expected (or reasonably close)
		compressionOK = m.CompressionRatio <= exp.CompressionRatio*1.5 // Allow 50% tolerance
	} else {
		// Compression mode: actual should be >= expected
		compressionOK = m.CompressionRatio >= exp.CompressionRatio*0.8 // Allow 20% tolerance
	}
	sb.WriteString(fmt.Sprintf("  Encoding Ratio:        %.2fx (expected) | %.2fx (actual) %s\n",
		exp.CompressionRatio, m.CompressionRatio, checkMark(compressionOK)))
	sb.WriteString(fmt.Sprintf("  Retrieval Recall:      %.2f%% (expected) | %.2f%% (actual) %s\n",
		exp.AvgRetrievalRecall*100, m.AvgRetrievalRecall*100, checkMark(m.AvgRetrievalRecall >= exp.AvgRetrievalRecall)))
	sb.WriteString(fmt.Sprintf("  Retrieval Precision:   %.2f%% (expected) | %.2f%% (actual) %s\n",
		exp.AvgRetrievalPrec*100, m.AvgRetrievalPrec*100, checkMark(m.AvgRetrievalPrec >= exp.AvgRetrievalPrec)))
	sb.WriteString(fmt.Sprintf("  Token Violations:      %d (max) | %d (actual) %s\n",
		exp.TokenBudgetViolations, m.TokenBudgetViolations, checkMark(m.TokenBudgetViolations <= exp.TokenBudgetViolations)))
	sb.WriteString("\n")

	// Checkpoint Results
	sb.WriteString("CHECKPOINT RESULTS:\n")
	sb.WriteString("───────────────────────────────────────────────────────────────\n")
	for i, cp := range result.CheckpointResults {
		status := "✓"
		if !cp.Passed {
			status = "✗"
		}
		sb.WriteString(fmt.Sprintf("%s Checkpoint %d (Turn %d): %s\n",
			status, i+1, cp.Checkpoint.AfterTurn, cp.Checkpoint.Description))
		sb.WriteString(fmt.Sprintf("    Precision: %.2f%% | Recall: %.2f%% | F1: %.2f%%\n",
			cp.Precision*100, cp.Recall*100, cp.F1Score*100))

		if len(cp.MissingRequired) > 0 {
			sb.WriteString(fmt.Sprintf("    Missing: %s\n", strings.Join(cp.MissingRequired, ", ")))
		}

		if len(cp.UnwantedNoise) > 0 {
			sb.WriteString(fmt.Sprintf("    Noise: %s\n", strings.Join(cp.UnwantedNoise, ", ")))
		}

		if cp.FailureReason != "" {
			sb.WriteString(fmt.Sprintf("    Reason: %s\n", cp.FailureReason))
		}
		sb.WriteString("\n")
	}

	// Footer
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")

	_, err := r.writer.Write([]byte(sb.String()))
	return err
}

// checkMark returns ✓ or ✗ based on condition.
func checkMark(condition bool) string {
	if condition {
		return "✓"
	}
	return "✗"
}

// ReportSummary outputs a summary of multiple test results.
func (r *Reporter) ReportSummary(results []*TestResult) error {
	var sb strings.Builder

	sb.WriteString("\n═══════════════════════════════════════════════════════════════\n")
	sb.WriteString("  CONTEXT TEST HARNESS - SUMMARY\n")
	sb.WriteString("═══════════════════════════════════════════════════════════════\n\n")

	passed := 0
	failed := 0

	for _, result := range results {
		status := "✗ FAILED"
		if result.Passed {
			status = "✓ PASSED"
			passed++
		} else {
			failed++
		}

		sb.WriteString(fmt.Sprintf("%s  %s\n", status, result.Scenario.Name))
	}

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Total: %d | Passed: %d | Failed: %d\n", len(results), passed, failed))
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")

	_, err := r.writer.Write([]byte(sb.String()))
	return err
}
