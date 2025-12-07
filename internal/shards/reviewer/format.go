package reviewer

import (
	"fmt"
	"strings"
)

// =============================================================================
// OUTPUT FORMATTING
// =============================================================================

// formatResult formats a ReviewResult for human-readable output.
func (r *ReviewerShard) formatResult(result *ReviewResult) string {
	var sb strings.Builder

	// Header
	status := "✓ PASSED"
	if result.BlockCommit {
		status = "✗ BLOCKED"
	} else if result.Severity == ReviewSeverityError || result.Severity == ReviewSeverityCritical {
		status = "⚠ ISSUES FOUND"
	}

	sb.WriteString(fmt.Sprintf("%s - %s (%s)\n", status, result.Summary, result.Duration))
	sb.WriteString(fmt.Sprintf("Files reviewed: %d\n", len(result.Files)))

	// Group findings by severity
	if len(result.Findings) > 0 {
		sb.WriteString("\nFindings:\n")

		// Critical first
		for _, f := range result.Findings {
			if f.Severity == "critical" {
				sb.WriteString(fmt.Sprintf("  [CRITICAL] %s:%d - %s\n", f.File, f.Line, f.Message))
				if f.Suggestion != "" {
					sb.WriteString(fmt.Sprintf("    → %s\n", f.Suggestion))
				}
			}
		}

		// Then errors
		for _, f := range result.Findings {
			if f.Severity == "error" {
				sb.WriteString(fmt.Sprintf("  [ERROR] %s:%d - %s\n", f.File, f.Line, f.Message))
				if f.Suggestion != "" {
					sb.WriteString(fmt.Sprintf("    → %s\n", f.Suggestion))
				}
			}
		}

		// Then warnings (limited)
		warningCount := 0
		for _, f := range result.Findings {
			if f.Severity == "warning" {
				warningCount++
				if warningCount <= 5 {
					sb.WriteString(fmt.Sprintf("  [WARN] %s:%d - %s\n", f.File, f.Line, f.Message))
				}
			}
		}
		if warningCount > 5 {
			sb.WriteString(fmt.Sprintf("  ... and %d more warnings\n", warningCount-5))
		}

		// Info count only
		infoCount := 0
		for _, f := range result.Findings {
			if f.Severity == "info" {
				infoCount++
			}
		}
		if infoCount > 0 {
			sb.WriteString(fmt.Sprintf("  (%d info-level suggestions)\n", infoCount))
		}
	}

	// Metrics summary
	if result.Metrics != nil {
		sb.WriteString(fmt.Sprintf("\nMetrics: %d lines (%d code, %d comments), %d functions\n",
			result.Metrics.TotalLines, result.Metrics.CodeLines,
			result.Metrics.CommentLines, result.Metrics.FunctionCount))
		if result.Metrics.CyclomaticMax > 10 {
			sb.WriteString(fmt.Sprintf("  ⚠ Max cyclomatic complexity: %d\n", result.Metrics.CyclomaticMax))
		}
		if result.Metrics.MaxNesting > 4 {
			sb.WriteString(fmt.Sprintf("  ⚠ Max nesting depth: %d\n", result.Metrics.MaxNesting))
		}
	}

	// Specialist recommendations
	if len(result.SpecialistRecommendations) > 0 {
		sb.WriteString("\n## Specialist Recommendations\n")
		for _, rec := range result.SpecialistRecommendations {
			sb.WriteString(fmt.Sprintf("  → **%s** (%.0f%% confidence): %s\n",
				rec.ShardName, rec.Confidence*100, rec.Reason))
			if len(rec.TaskHints) > 0 {
				sb.WriteString(fmt.Sprintf("    Suggested tasks: %s\n", strings.Join(rec.TaskHints, ", ")))
			}
		}
	}

	return sb.String()
}
