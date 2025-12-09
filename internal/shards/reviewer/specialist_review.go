// Package reviewer provides code review functionality with multi-shard orchestration.
// This file contains task formatting for specialist domain reviews.
package reviewer

import (
	"fmt"
	"path/filepath"
	"strings"
)

// =============================================================================
// SPECIALIST REVIEW TASK FORMATTING
// =============================================================================
// Functions for creating domain-expert review tasks for specialist shards.

// SpecialistReviewTask represents a review task for a specialist
type SpecialistReviewTask struct {
	AgentName     string              // Name of the specialist
	Files         []string            // Files to review
	Knowledge     []RetrievedKnowledge // Domain knowledge context
	DomainFocus   string              // What domain to focus on
	ContextHints  []string            // Additional context hints
}

// FormatSpecialistReviewTask creates a domain-expert review task string.
// This is injected as the task for specialist shards during multi-shard reviews.
func FormatSpecialistReviewTask(task SpecialistReviewTask) string {
	var sb strings.Builder

	sb.WriteString("SPECIALIST DOMAIN REVIEW\n\n")
	sb.WriteString(fmt.Sprintf("You are reviewing as a %s domain expert.\n\n", task.AgentName))

	// Files section
	sb.WriteString("## Files to Review\n\n")
	for _, file := range task.Files {
		sb.WriteString(fmt.Sprintf("- %s\n", file))
	}
	sb.WriteString("\n")

	// Domain focus
	if task.DomainFocus != "" {
		sb.WriteString(fmt.Sprintf("## Domain Focus: %s\n\n", task.DomainFocus))
	}

	// Review instructions
	sb.WriteString("## Your Mission\n\n")
	sb.WriteString("Using your specialized knowledge, identify:\n\n")
	sb.WriteString("1. **Domain-Specific Issues**: Patterns or practices that violate best practices for this technology\n")
	sb.WriteString("2. **Missing Best Practices**: Industry-standard patterns that should be applied but aren't\n")
	sb.WriteString("3. **Integration Concerns**: How this code integrates with the broader system architecture\n")
	sb.WriteString("4. **Performance/Safety**: Domain-specific performance or safety concerns\n")
	sb.WriteString("5. **Idiomatic Usage**: Whether the code follows idiomatic patterns for this technology\n\n")

	// Knowledge context
	if len(task.Knowledge) > 0 {
		sb.WriteString("## Your Knowledge Base\n\n")
		sb.WriteString(FormatKnowledgeContext(task.Knowledge))
	}

	// Context hints
	if len(task.ContextHints) > 0 {
		sb.WriteString("## Additional Context\n\n")
		for _, hint := range task.ContextHints {
			sb.WriteString(fmt.Sprintf("- %s\n", hint))
		}
		sb.WriteString("\n")
	}

	// Output format
	sb.WriteString("## Output Format\n\n")
	sb.WriteString("Report findings in this format:\n\n")
	sb.WriteString("```\n")
	sb.WriteString("### [SEVERITY: critical|warning|info] Issue Title\n")
	sb.WriteString("- **File**: path/to/file.ext:line\n")
	sb.WriteString("- **Issue**: Description of what's wrong\n")
	sb.WriteString("- **Recommendation**: How to fix it\n")
	sb.WriteString("- **Domain Insight**: Why this matters in this technology domain\n")
	sb.WriteString("```\n\n")

	sb.WriteString("Focus on insights that a generic code reviewer would miss.\n")
	sb.WriteString("Your domain expertise is what makes this review valuable.\n")

	return sb.String()
}

// BuildSpecialistTask creates a SpecialistReviewTask from match and files
func BuildSpecialistTask(match SpecialistMatch, allFiles []string, knowledge []RetrievedKnowledge) SpecialistReviewTask {
	// Determine which files this specialist should review
	// If the match has specific files, use those; otherwise use files matching the pattern
	files := match.Files
	if len(files) == 0 {
		files = allFiles
	}

	// Build context hints based on file types
	var hints []string
	exts := make(map[string]bool)
	for _, f := range files {
		ext := filepath.Ext(f)
		if !exts[ext] {
			exts[ext] = true
			hints = append(hints, fmt.Sprintf("File type: %s", ext))
		}
	}

	return SpecialistReviewTask{
		AgentName:    match.AgentName,
		Files:        files,
		Knowledge:    knowledge,
		DomainFocus:  match.Reason,
		ContextHints: hints,
	}
}

// FormatMultiShardReviewHeader creates the header for multi-shard review output
func FormatMultiShardReviewHeader(target string, participants []string, isComplete bool) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Multi-Shard Code Review: %s\n\n", target))

	status := "Complete"
	if !isComplete {
		status = "Partial (some specialists failed)"
	}
	sb.WriteString(fmt.Sprintf("**Status**: %s\n", status))
	sb.WriteString(fmt.Sprintf("**Participants**: %s\n\n", strings.Join(participants, ", ")))

	return sb.String()
}

// FormatShardSection formats a section for one shard's findings
func FormatShardSection(shardName string, findings []ParsedFinding) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## %s (%d findings)\n\n", shardName, len(findings)))

	if len(findings) == 0 {
		sb.WriteString("_No issues found._\n\n")
		return sb.String()
	}

	// Group by severity
	bySeverity := make(map[string][]ParsedFinding)
	for _, f := range findings {
		sev := strings.ToLower(f.Severity)
		if sev == "" {
			sev = "info"
		}
		bySeverity[sev] = append(bySeverity[sev], f)
	}

	// Output in severity order
	severityOrder := []string{"critical", "error", "warning", "info"}
	for _, sev := range severityOrder {
		items, ok := bySeverity[sev]
		if !ok || len(items) == 0 {
			continue
		}

		for _, f := range items {
			sb.WriteString(fmt.Sprintf("- **%s:%d** [%s] %s\n",
				f.File, f.Line, strings.ToUpper(sev), f.Message))
			if f.Recommendation != "" {
				sb.WriteString(fmt.Sprintf("  - _Recommendation_: %s\n", f.Recommendation))
			}
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

// ParsedFinding represents a parsed finding from shard output
type ParsedFinding struct {
	File           string
	Line           int
	Severity       string
	Category       string
	Message        string
	Recommendation string
	ShardSource    string // Which shard found this
}

// ParseShardOutput extracts findings from a shard's output text
func ParseShardOutput(output string, shardName string) []ParsedFinding {
	var findings []ParsedFinding

	lines := strings.Split(output, "\n")

	var currentFinding *ParsedFinding

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for finding headers
		if strings.HasPrefix(line, "### [") || strings.HasPrefix(line, "- **") {
			// Save previous finding
			if currentFinding != nil && currentFinding.Message != "" {
				findings = append(findings, *currentFinding)
			}

			currentFinding = &ParsedFinding{ShardSource: shardName}

			// Parse severity from header
			if strings.Contains(strings.ToLower(line), "critical") {
				currentFinding.Severity = "critical"
			} else if strings.Contains(strings.ToLower(line), "error") {
				currentFinding.Severity = "error"
			} else if strings.Contains(strings.ToLower(line), "warning") {
				currentFinding.Severity = "warning"
			} else {
				currentFinding.Severity = "info"
			}

			// Try to extract file:line
			if idx := strings.Index(line, "**File**:"); idx >= 0 {
				rest := strings.TrimSpace(line[idx+9:])
				if colonIdx := strings.Index(rest, ":"); colonIdx >= 0 {
					currentFinding.File = strings.TrimSpace(rest[:colonIdx])
					lineStr := strings.TrimSpace(rest[colonIdx+1:])
					fmt.Sscanf(lineStr, "%d", &currentFinding.Line)
				}
			}

			// Extract message from header
			if idx := strings.LastIndex(line, "]"); idx >= 0 {
				currentFinding.Message = strings.TrimSpace(line[idx+1:])
			} else {
				currentFinding.Message = line
			}
		} else if currentFinding != nil {
			// Look for additional fields
			if strings.HasPrefix(line, "- **File**:") {
				rest := strings.TrimPrefix(line, "- **File**:")
				rest = strings.TrimSpace(rest)
				if colonIdx := strings.Index(rest, ":"); colonIdx >= 0 {
					currentFinding.File = strings.TrimSpace(rest[:colonIdx])
					lineStr := strings.TrimSpace(rest[colonIdx+1:])
					fmt.Sscanf(lineStr, "%d", &currentFinding.Line)
				} else {
					currentFinding.File = rest
				}
			} else if strings.HasPrefix(line, "- **Issue**:") {
				currentFinding.Message = strings.TrimPrefix(line, "- **Issue**:")
				currentFinding.Message = strings.TrimSpace(currentFinding.Message)
			} else if strings.HasPrefix(line, "- **Recommendation**:") {
				currentFinding.Recommendation = strings.TrimPrefix(line, "- **Recommendation**:")
				currentFinding.Recommendation = strings.TrimSpace(currentFinding.Recommendation)
			} else if strings.HasPrefix(line, "- _Recommendation_:") {
				currentFinding.Recommendation = strings.TrimPrefix(line, "- _Recommendation_:")
				currentFinding.Recommendation = strings.TrimSpace(currentFinding.Recommendation)
			}
		}
	}

	// Save last finding
	if currentFinding != nil && currentFinding.Message != "" {
		findings = append(findings, *currentFinding)
	}

	return findings
}
