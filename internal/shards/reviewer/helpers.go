// Package reviewer implements the Reviewer ShardAgent per §7.0 Sharding.
// This file contains utility helper functions.
package reviewer

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"codenerd/internal/core"
)

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// readFile reads a file via VirtualStore or directly.
func (r *ReviewerShard) readFile(ctx context.Context, filePath string) (string, error) {
	if r.virtualStore != nil {
		action := core.Fact{
			Predicate: "next_action",
			Args:      []interface{}{"/read_file", filePath},
		}
		return r.virtualStore.RouteAction(ctx, action)
	}
	return "", fmt.Errorf("virtualStore required for file operations")
}

// shouldIgnore checks if a file should be ignored.
func (r *ReviewerShard) shouldIgnore(filePath string) bool {
	for _, pattern := range r.reviewerConfig.IgnorePatterns {
		if matched, _ := filepath.Match(pattern, filePath); matched {
			return true
		}
		// Also check if pattern is contained in path
		if strings.Contains(filePath, strings.TrimSuffix(pattern, "*")) {
			return true
		}
	}
	return false
}

// detectLanguage detects programming language from file extension.
func (r *ReviewerShard) detectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".cs":
		return "csharp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".hpp":
		return "cpp"
	default:
		return "unknown"
	}
}

// parseDiffFiles extracts file names from git diff output.
func (r *ReviewerShard) parseDiffFiles(diffOutput string) []string {
	files := make([]string, 0)
	lines := strings.Split(diffOutput, "\n")

	diffFileRegex := regexp.MustCompile(`^diff --git a/(.+) b/`)
	for _, line := range lines {
		if matches := diffFileRegex.FindStringSubmatch(line); len(matches) > 1 {
			files = append(files, matches[1])
		}
	}

	return files
}

// calculateOverallSeverity determines the highest severity from findings.
func (r *ReviewerShard) calculateOverallSeverity(findings []ReviewFinding) ReviewSeverity {
	if len(findings) == 0 {
		return ReviewSeverityClean
	}

	hasCritical := false
	hasError := false
	hasWarning := false

	for _, f := range findings {
		switch f.Severity {
		case "critical":
			hasCritical = true
		case "error":
			hasError = true
		case "warning":
			hasWarning = true
		}
	}

	if hasCritical {
		return ReviewSeverityCritical
	}
	if hasError {
		return ReviewSeverityError
	}
	if hasWarning {
		return ReviewSeverityWarning
	}
	return ReviewSeverityInfo
}

// shouldBlockCommit determines if the review should block commits.
func (r *ReviewerShard) shouldBlockCommit(result *ReviewResult) bool {
	if !r.reviewerConfig.BlockOnCritical {
		return false
	}
	return result.Severity == ReviewSeverityCritical
}

// generateSummary creates a human-readable summary.
func (r *ReviewerShard) generateSummary(result *ReviewResult) string {
	criticalCount := 0
	errorCount := 0
	warningCount := 0
	infoCount := 0

	for _, f := range result.Findings {
		switch f.Severity {
		case "critical":
			criticalCount++
		case "error":
			errorCount++
		case "warning":
			warningCount++
		case "info":
			infoCount++
		}
	}

	return fmt.Sprintf("Review complete: %d critical, %d errors, %d warnings, %d info",
		criticalCount, errorCount, warningCount, infoCount)
}

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// boolToStatus converts a boolean to a status string.
func boolToStatus(b bool) string {
	if b {
		return "PASSED"
	}
	return "FAILED"
}

// toStartInt safely converts interface{} to int.
func toStartInt(v interface{}) int {
	if i, ok := v.(int); ok {
		return i
	}
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}
