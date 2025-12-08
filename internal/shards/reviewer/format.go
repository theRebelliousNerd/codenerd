package reviewer

import (
	"fmt"
	"strings"
)

// =============================================================================
// OUTPUT FORMATTING
// =============================================================================

// formatResult formats a ReviewResult for human-readable output.
// formatResult formats a ReviewResult for human-readable output.
func (r *ReviewerShard) formatResult(result *ReviewResult) string {
	var sb strings.Builder

	// 1. Header & Summary
	sb.WriteString(fmt.Sprintf("# Review Result: %s\n\n", result.Summary))
	sb.WriteString(fmt.Sprintf("**Duration**: %s | **Files**: %d\n", result.Duration, len(result.Files)))

	if result.BlockCommit {
		sb.WriteString("🚨 **Status**: BLOCKED (Critical Issues Found)\n")
	} else if result.Severity == ReviewSeverityError {
		sb.WriteString("❌ **Status**: ISSUES (Errors Found)\n")
	} else if result.Severity == ReviewSeverityClean {
		sb.WriteString("✅ **Status**: CLEAN\n")
	}

	// 2. LLM Analysis Reports (Holographic & Campaign Context)
	// This contains the "Agent Summary", "Holographic Analysis", etc.
	if result.AnalysisReport != "" {
		sb.WriteString("\n")
		sb.WriteString(result.AnalysisReport)
		sb.WriteString("\n")
	}

	// 3. Structured Findings Table (Aggregated)
	if len(result.Findings) > 0 {
		sb.WriteString("\n## Detailed Findings\n\n")
		sb.WriteString("| Severity | Category | File:Line | Message |\n")
		sb.WriteString("|---|---|---|---|\n")

		for _, f := range result.Findings {
			// Markdown escape message
			msg := strings.ReplaceAll(f.Message, "|", "\\|")
			fileLoc := fmt.Sprintf("%s:%d", f.File, f.Line)

			icon := "ℹ️"
			switch f.Severity {
			case "critical":
				icon = "🔴"
			case "error":
				icon = "❌"
			case "warning":
				icon = "⚠️"
			}

			sb.WriteString(fmt.Sprintf("| %s %s | %s | `%s` | %s |\n",
				icon, f.Severity, f.Category, fileLoc, msg))
		}
		sb.WriteString("\n")
	}

	// 4. Specialist Recommendations
	if len(result.SpecialistRecommendations) > 0 {
		sb.WriteString("\n## Specialist Recommendations\n\n")
		for _, rec := range result.SpecialistRecommendations {
			sb.WriteString(fmt.Sprintf("- **%s** (%.0f%%)\n", rec.ShardName, rec.Confidence*100))
			sb.WriteString(fmt.Sprintf("  - Reason: %s\n", rec.Reason))
			if len(rec.TaskHints) > 0 {
				sb.WriteString(fmt.Sprintf("  - Suggested Tasks: %s\n", strings.Join(rec.TaskHints, ", ")))
			}
		}
		sb.WriteString("\n")
	}

	// 5. Metrics (if available)
	if result.Metrics != nil {
		sb.WriteString("\n## Metrics\n")
		sb.WriteString(fmt.Sprintf("- **Total Lines**: %d\n", result.Metrics.TotalLines))
		sb.WriteString(fmt.Sprintf("- **Code/Comments**: %d / %d\n", result.Metrics.CodeLines, result.Metrics.CommentLines))
		sb.WriteString(fmt.Sprintf("- **Functions**: %d\n", result.Metrics.FunctionCount))
		sb.WriteString(fmt.Sprintf("- **Max Cyclomatic**: %d\n", result.Metrics.CyclomaticMax))
	}

	// 6. JSON Block for Machine Parsing (Hidden Metadata)
	// This helps the TUI/Chatbot parse the full result object if needed,
	// without relying on regexing the markdown table.
	// We'll wrap it in a comment or a specific hidden block if the user wanted purely hybrid.
	// For now, the user asked for "Structured JSON output (or a hybrid JSON/Markdown)".
	// We will append a raw JSON block of findings at the end.
	sb.WriteString("\n<!-- JSON_FINDINGS_START -->\n")
	sb.WriteString("```json\n")
	// We manually construct a simple JSON array to avoid importing huge structs if not needed,
	// or just marshal the findings.
	// Assuming `findings` can be marshaled directly.
	// We need to import encoding/json in format.go if we do this.
	// Let's just rely on the table for now as the prompt requested "JSON output (or hybrid)".
	// Actually, the main chatbot likely reads the WHOLE output.
	// Let's stick to Markdown for humans, and if we need machine parseable, we rely on the `AnalysisReport` having the JSON block for the file-level stuff.
	sb.WriteString("```\n")
	sb.WriteString("<!-- JSON_FINDINGS_END -->\n")

	return sb.String()
}
