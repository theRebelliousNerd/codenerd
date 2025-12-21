package context_harness

import (
	"fmt"
	"io"
	"strings"
	"time"

	"codenerd/internal/core"
)

// CompressionVisualizer shows before/after compression.
type CompressionVisualizer struct {
	writer  io.Writer
	verbose bool
}

// NewCompressionVisualizer creates a new compression visualizer.
func NewCompressionVisualizer(writer io.Writer, verbose bool) *CompressionVisualizer {
	return &CompressionVisualizer{
		writer:  writer,
		verbose: verbose,
	}
}

// CompressionEvent captures a single compression event.
type CompressionEvent struct {
	Timestamp    time.Time
	TurnNumber   int
	Speaker      string // "user" or "assistant"

	// Original Content
	OriginalText   string
	OriginalTokens int

	// Compressed Output
	CompressedFacts []core.Fact
	CompressedTokens int

	// Metadata Extracted
	FilesReferenced   []string
	SymbolsReferenced []string
	ErrorMessages     []string
	Topics            []string
	ReferencesBack    *int // Turn number referenced

	// Compression Stats
	CompressionRatio  float64
	CompressionLatency time.Duration
	LossyElements     []string // Things that were discarded
}

// VisualizeCompression shows a side-by-side before/after view.
func (v *CompressionVisualizer) VisualizeCompression(event *CompressionEvent) {
	var sb strings.Builder

	// Header
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("COMPRESSION EVENT - TURN %d (%s)\n", event.TurnNumber, event.Speaker))
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("Timestamp: %s\n", event.Timestamp.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("Compression Latency: %v\n\n", event.CompressionLatency.Round(time.Millisecond)))

	// Compression Stats
	sb.WriteString("COMPRESSION STATS:\n")
	sb.WriteString("───────────────────────────────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("  Original:    %s tokens\n", formatNumber(event.OriginalTokens)))
	sb.WriteString(fmt.Sprintf("  Compressed:  %s tokens\n", formatNumber(event.CompressedTokens)))
	sb.WriteString(fmt.Sprintf("  Ratio:       %.2fx\n", event.CompressionRatio))
	sb.WriteString(fmt.Sprintf("  Saved:       %s tokens (%.1f%%)\n\n",
		formatNumber(event.OriginalTokens-event.CompressedTokens),
		(1.0-float64(event.CompressedTokens)/float64(event.OriginalTokens))*100))

	// Side-by-Side Comparison
	sb.WriteString("BEFORE (Original Text):\n")
	sb.WriteString("───────────────────────────────────────────────────────────────\n")

	// Show original text with line numbers
	lines := strings.Split(event.OriginalText, "\n")
	maxLines := 30
	if len(lines) > maxLines {
		for i := 0; i < maxLines; i++ {
			sb.WriteString(fmt.Sprintf("%3d│ %s\n", i+1, lines[i]))
		}
		sb.WriteString(fmt.Sprintf("... (%d more lines)\n", len(lines)-maxLines))
	} else {
		for i, line := range lines {
			sb.WriteString(fmt.Sprintf("%3d│ %s\n", i+1, line))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("AFTER (Compressed Facts):\n")
	sb.WriteString("───────────────────────────────────────────────────────────────\n")

	// Show compressed facts
	for i, fact := range event.CompressedFacts {
		sb.WriteString(fmt.Sprintf("%3d│ %s\n", i+1, fact.String()))
	}
	sb.WriteString("\n")

	// Extracted Metadata
	if len(event.FilesReferenced) > 0 || len(event.ErrorMessages) > 0 ||
		len(event.Topics) > 0 || event.ReferencesBack != nil {

		sb.WriteString("EXTRACTED METADATA:\n")
		sb.WriteString("───────────────────────────────────────────────────────────────\n")

		if len(event.FilesReferenced) > 0 {
			sb.WriteString("Files Referenced:\n")
			for _, file := range event.FilesReferenced {
				sb.WriteString(fmt.Sprintf("  - %s\n", file))
			}
		}

		if len(event.SymbolsReferenced) > 0 {
			sb.WriteString("Symbols Referenced:\n")
			for _, sym := range event.SymbolsReferenced {
				sb.WriteString(fmt.Sprintf("  - %s\n", sym))
			}
		}

		if len(event.ErrorMessages) > 0 {
			sb.WriteString("Error Messages:\n")
			for _, err := range event.ErrorMessages {
				sb.WriteString(fmt.Sprintf("  - %s\n", truncate(err, 100)))
			}
		}

		if len(event.Topics) > 0 {
			sb.WriteString(fmt.Sprintf("Topics: %s\n", strings.Join(event.Topics, ", ")))
		}

		if event.ReferencesBack != nil {
			sb.WriteString(fmt.Sprintf("References Back To: Turn %d\n", *event.ReferencesBack))
		}

		sb.WriteString("\n")
	}

	// Lossy Elements (what was discarded)
	if v.verbose && len(event.LossyElements) > 0 {
		sb.WriteString("DISCARDED ELEMENTS (lossy compression):\n")
		sb.WriteString("───────────────────────────────────────────────────────────────\n")
		for _, elem := range event.LossyElements {
			sb.WriteString(fmt.Sprintf("  ✗ %s\n", elem))
		}
		sb.WriteString("\n")
		sb.WriteString("Note: These elements were deemed non-essential for context retention.\n")
		sb.WriteString("      They can be safely discarded without affecting future retrieval.\n\n")
	}

	sb.WriteString("═══════════════════════════════════════════════════════════════\n\n")

	v.writer.Write([]byte(sb.String()))
}

// CompressionSummary shows aggregate compression stats across a session.
type CompressionSummary struct {
	TotalTurns        int
	TotalOriginalTokens int
	TotalCompressedTokens int
	AverageRatio      float64
	BestRatio         float64
	WorstRatio        float64
	TotalTimeSaved    time.Duration

	// Per-Speaker Stats
	UserCompressionRatio      float64
	AssistantCompressionRatio float64

	// Trends
	RatioTrend string // "improving", "stable", "degrading"
}

// VisualizeSummary shows aggregate compression statistics.
func (v *CompressionVisualizer) VisualizeSummary(summary *CompressionSummary) {
	var sb strings.Builder

	sb.WriteString("\n═══════════════════════════════════════════════════════════════\n")
	sb.WriteString("COMPRESSION SUMMARY\n")
	sb.WriteString("═══════════════════════════════════════════════════════════════\n\n")

	sb.WriteString("OVERALL STATS:\n")
	sb.WriteString("───────────────────────────────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("  Total Turns:           %d\n", summary.TotalTurns))
	sb.WriteString(fmt.Sprintf("  Original Tokens:       %s\n", formatNumber(summary.TotalOriginalTokens)))
	sb.WriteString(fmt.Sprintf("  Compressed Tokens:     %s\n", formatNumber(summary.TotalCompressedTokens)))
	sb.WriteString(fmt.Sprintf("  Average Ratio:         %.2fx\n", summary.AverageRatio))
	sb.WriteString(fmt.Sprintf("  Best Ratio:            %.2fx\n", summary.BestRatio))
	sb.WriteString(fmt.Sprintf("  Worst Ratio:           %.2fx\n", summary.WorstRatio))
	sb.WriteString(fmt.Sprintf("  Total Tokens Saved:    %s\n",
		formatNumber(summary.TotalOriginalTokens-summary.TotalCompressedTokens)))
	sb.WriteString(fmt.Sprintf("  Percentage Saved:      %.1f%%\n\n",
		(1.0-float64(summary.TotalCompressedTokens)/float64(summary.TotalOriginalTokens))*100))

	sb.WriteString("PER-SPEAKER STATS:\n")
	sb.WriteString("───────────────────────────────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("  User Ratio:            %.2fx\n", summary.UserCompressionRatio))
	sb.WriteString(fmt.Sprintf("  Assistant Ratio:       %.2fx\n\n", summary.AssistantCompressionRatio))

	sb.WriteString("TREND ANALYSIS:\n")
	sb.WriteString("───────────────────────────────────────────────────────────────\n")
	trendSymbol := "→"
	switch summary.RatioTrend {
	case "improving":
		trendSymbol = "↗"
	case "degrading":
		trendSymbol = "↘"
	}
	sb.WriteString(fmt.Sprintf("  Compression Trend:     %s %s\n", trendSymbol, summary.RatioTrend))

	if summary.RatioTrend == "degrading" {
		sb.WriteString("\n  ⚠️  Warning: Compression ratio is degrading over time.\n")
		sb.WriteString("      This may indicate context accumulation issues.\n")
		sb.WriteString("      Consider reviewing compression thresholds.\n")
	}

	sb.WriteString("\n═══════════════════════════════════════════════════════════════\n")

	v.writer.Write([]byte(sb.String()))
}
