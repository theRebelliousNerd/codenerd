// Package reviewer implements the Reviewer ShardAgent per §7.0 Sharding.
// This file contains core type definitions for review results.
package reviewer

import "time"

// =============================================================================
// REVIEW RESULT TYPES
// =============================================================================

// ReviewSeverity represents the overall severity level of a review.
type ReviewSeverity string

const (
	ReviewSeverityClean    ReviewSeverity = "clean"
	ReviewSeverityInfo     ReviewSeverity = "info"
	ReviewSeverityWarning  ReviewSeverity = "warning"
	ReviewSeverityError    ReviewSeverity = "error"
	ReviewSeverityCritical ReviewSeverity = "critical"
)

// ReviewFinding represents a single issue found during review.
type ReviewFinding struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Column      int    `json:"column,omitempty"`
	EndLine     int    `json:"end_line,omitempty"`
	Severity    string `json:"severity"` // "critical", "error", "warning", "info", "suggestion"
	Category    string `json:"category"` // "security", "style", "performance", "maintainability", "bug"
	RuleID      string `json:"rule_id"`
	Message     string `json:"message"`
	Suggestion  string `json:"suggestion,omitempty"`
	CodeSnippet string `json:"code_snippet,omitempty"`
}

// ReviewResult represents the outcome of a code review.
type ReviewResult struct {
	Files       []string        `json:"files"`
	Findings    []ReviewFinding `json:"findings"`
	Severity    ReviewSeverity  `json:"severity"`
	Summary     string          `json:"summary"`
	Duration    time.Duration   `json:"duration"`
	BlockCommit bool            `json:"block_commit"`
	Metrics     *CodeMetrics    `json:"metrics,omitempty"`

	// Textual analysis report from LLM (Markdown)
	AnalysisReport string `json:"analysis_report,omitempty"`

	// Specialist recommendations based on detected technologies
	SpecialistRecommendations []SpecialistRecommendation `json:"specialist_recommendations,omitempty"`
}

// CodeMetrics holds code complexity metrics.
type CodeMetrics struct {
	TotalLines      int     `json:"total_lines"`
	CodeLines       int     `json:"code_lines"`
	CommentLines    int     `json:"comment_lines"`
	BlankLines      int     `json:"blank_lines"`
	CyclomaticAvg   float64 `json:"cyclomatic_avg"`
	CyclomaticMax   int     `json:"cyclomatic_max"`
	MaxNesting      int     `json:"max_nesting"`
	FunctionCount   int     `json:"function_count"`
	LongFunctions   int     `json:"long_functions"` // Functions > 50 lines
	DuplicateBlocks int     `json:"duplicate_blocks"`
}
