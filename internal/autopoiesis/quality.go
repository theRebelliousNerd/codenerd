package autopoiesis

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// =============================================================================
// QUALITY ASSESSMENT - HOW GOOD WAS THE OUTPUT?
// =============================================================================

// QualityAssessment evaluates tool output quality
type QualityAssessment struct {
	// Overall score (0.0 - 1.0)
	Score float64 `json:"score"`

	// Dimension scores
	Completeness float64 `json:"completeness"` // Did we get all available data?
	Accuracy     float64 `json:"accuracy"`     // Was the output correct?
	Efficiency   float64 `json:"efficiency"`   // Resource efficiency
	Relevance    float64 `json:"relevance"`    // Was output relevant to intent?

	// Specific issues detected
	Issues []QualityIssue `json:"issues"`

	// Improvement suggestions
	Suggestions []ImprovementSuggestion `json:"suggestions"`

	// Metadata
	EvaluatedAt time.Time `json:"evaluated_at"`
	EvaluatedBy string    `json:"evaluated_by"` // "heuristic", "llm", "user"
}

// QualityIssue describes a specific quality problem
type QualityIssue struct {
	Type        IssueType `json:"type"`
	Severity    float64   `json:"severity"`    // 0.0 - 1.0
	Description string    `json:"description"`
	Evidence    string    `json:"evidence"`    // What in the output shows this?
	Fixable     bool      `json:"fixable"`     // Can we auto-fix this?
}

// IssueType categorizes quality issues
type IssueType string

const (
	IssueIncomplete     IssueType = "incomplete"      // Missing data (like your Context7 example)
	IssueInaccurate     IssueType = "inaccurate"      // Wrong data
	IssueSlow           IssueType = "slow"            // Too slow
	IssueResourceHeavy  IssueType = "resource_heavy"  // Used too much memory/CPU
	IssuePoorFormat     IssueType = "poor_format"     // Output format issues
	IssuePartialFailure IssueType = "partial_failure" // Partially worked
	IssuePagination     IssueType = "pagination"      // Didn't handle pagination
	IssueRateLimit      IssueType = "rate_limit"      // Hit rate limits
	IssueAuth           IssueType = "auth"            // Authentication issues
)

// ImprovementSuggestion describes how to improve the tool
type ImprovementSuggestion struct {
	Type        SuggestionType `json:"type"`
	Priority    float64        `json:"priority"`
	Description string         `json:"description"`
	CodeHint    string         `json:"code_hint,omitempty"` // Specific code change
}

// SuggestionType categorizes improvement suggestions
type SuggestionType string

const (
	SuggestAddPagination SuggestionType = "add_pagination"
	SuggestIncreaseLimit SuggestionType = "increase_limit"
	SuggestAddRetry      SuggestionType = "add_retry"
	SuggestCaching       SuggestionType = "add_caching"
	SuggestBatching      SuggestionType = "add_batching"
	SuggestParallel      SuggestionType = "parallelize"
	SuggestBetterParsing SuggestionType = "better_parsing"
	SuggestErrorHandling SuggestionType = "better_errors"
	SuggestValidation    SuggestionType = "add_validation"
)

// =============================================================================
// QUALITY EVALUATOR - ASSESS TOOL OUTPUT
// =============================================================================

// QualityEvaluator assesses the quality of tool executions
type QualityEvaluator struct {
	client            LLMClient
	heuristicRules    []HeuristicRule
	completenessHints map[string]CompletenessHint
	profileStore      *ProfileStore // Tool-specific quality profiles
}

// HeuristicRule is a rule for detecting quality issues
type HeuristicRule struct {
	Name        string
	IssueType   IssueType
	Pattern     *regexp.Regexp
	Severity    float64
	Description string
}

// CompletenessHint provides context-specific completeness expectations
type CompletenessHint struct {
	ToolPattern       string // Regex for tool name
	ExpectedMinSize   int    // Minimum expected output size
	ExpectedMaxPages  int    // Expected pagination depth
	PaginationPattern string // How to detect pagination in output
	TotalPattern      string // How to detect total available
}

// NewQualityEvaluator creates a new evaluator
func NewQualityEvaluator(client LLMClient, profileStore *ProfileStore) *QualityEvaluator {
	return &QualityEvaluator{
		client:            client,
		heuristicRules:    defaultHeuristicRules(),
		completenessHints: defaultCompletenessHints(),
		profileStore:      profileStore,
	}
}

// SetProfileStore sets the profile store (for delayed initialization)
func (qe *QualityEvaluator) SetProfileStore(store *ProfileStore) {
	qe.profileStore = store
}

// defaultHeuristicRules returns built-in quality detection rules
func defaultHeuristicRules() []HeuristicRule {
	return []HeuristicRule{
		{
			Name:        "pagination_truncated",
			IssueType:   IssuePagination,
			Pattern:     regexp.MustCompile(`(?i)(page\s*1\s*of\s*\d+|showing\s+\d+\s*-\s*\d+|has_more.*true|next_page|truncated)`),
			Severity:    0.7,
			Description: "Output appears to be paginated but only first page was fetched",
		},
		{
			Name:        "partial_results",
			IssueType:   IssueIncomplete,
			Pattern:     regexp.MustCompile(`(?i)(partial|incomplete|limited|max.*reached|rate.*limit)`),
			Severity:    0.6,
			Description: "Output indicates partial/limited results",
		},
		{
			Name:        "error_in_output",
			IssueType:   IssuePartialFailure,
			Pattern:     regexp.MustCompile(`(?i)(error|failed|exception|timeout|connection.*refused)`),
			Severity:    0.8,
			Description: "Output contains error indicators",
		},
		{
			Name:        "empty_or_minimal",
			IssueType:   IssueIncomplete,
			Pattern:     regexp.MustCompile(`^\s*(\[\s*\]|\{\s*\}|null|none|empty)\s*$`),
			Severity:    0.9,
			Description: "Output is empty or minimal",
		},
		{
			Name:        "rate_limited",
			IssueType:   IssueRateLimit,
			Pattern:     regexp.MustCompile(`(?i)(rate\s*limit|429|too\s*many\s*requests|throttl)`),
			Severity:    0.7,
			Description: "Output indicates rate limiting",
		},
	}
}

// defaultCompletenessHints returns hints for common tool types
func defaultCompletenessHints() map[string]CompletenessHint {
	return map[string]CompletenessHint{
		"api_docs": {
			ToolPattern:       "(?i)(docs|documentation|api|context7)",
			ExpectedMinSize:   10000, // Expect at least 10KB of docs
			ExpectedMaxPages:  10,
			PaginationPattern: `page.*of|has_more|next`,
			TotalPattern:      `total.*:\s*(\d+)`,
		},
		"search": {
			ToolPattern:       "(?i)(search|query|find)",
			ExpectedMinSize:   1000,
			ExpectedMaxPages:  5,
			PaginationPattern: `results\s+\d+\s*-\s*\d+`,
			TotalPattern:      `(\d+)\s*results`,
		},
		"list": {
			ToolPattern:       "(?i)(list|get_all|fetch)",
			ExpectedMinSize:   500,
			ExpectedMaxPages:  10,
			PaginationPattern: `showing.*of`,
			TotalPattern:      `total.*(\d+)`,
		},
	}
}

// Evaluate performs quality assessment on a tool execution
func (qe *QualityEvaluator) Evaluate(ctx context.Context, feedback *ExecutionFeedback) *QualityAssessment {
	assessment := &QualityAssessment{
		EvaluatedAt: time.Now(),
		EvaluatedBy: "heuristic",
		Issues:      []QualityIssue{},
		Suggestions: []ImprovementSuggestion{},
	}

	// If execution failed, that's the main issue
	if !feedback.Success {
		assessment.Score = 0.1
		assessment.Issues = append(assessment.Issues, QualityIssue{
			Type:        IssuePartialFailure,
			Severity:    1.0,
			Description: "Tool execution failed",
			Evidence:    feedback.ErrorMsg,
			Fixable:     true,
		})
		return assessment
	}

	// Apply heuristic rules
	for _, rule := range qe.heuristicRules {
		if rule.Pattern.MatchString(feedback.Output) {
			assessment.Issues = append(assessment.Issues, QualityIssue{
				Type:        rule.IssueType,
				Severity:    rule.Severity,
				Description: rule.Description,
				Evidence:    extractMatch(feedback.Output, rule.Pattern),
				Fixable:     true,
			})
		}
	}

	// Check completeness based on tool type hints
	completenessScore := qe.evaluateCompleteness(feedback)
	assessment.Completeness = completenessScore

	// Efficiency based on duration
	assessment.Efficiency = qe.evaluateEfficiency(feedback)

	// Calculate overall score
	issueImpact := 0.0
	for _, issue := range assessment.Issues {
		issueImpact += issue.Severity * 0.2 // Each issue can reduce score by up to 20%
	}

	assessment.Score = clamp(
		(assessment.Completeness*0.4 + assessment.Efficiency*0.2 + 0.4) - issueImpact,
		0.0, 1.0,
	)

	// Generate improvement suggestions based on issues
	assessment.Suggestions = qe.generateSuggestions(assessment.Issues, feedback)

	return assessment
}

// EvaluateWithProfile performs profile-aware quality assessment
func (qe *QualityEvaluator) EvaluateWithProfile(ctx context.Context, feedback *ExecutionFeedback, profile *ToolQualityProfile) *QualityAssessment {
	assessment := &QualityAssessment{
		EvaluatedAt: time.Now(),
		EvaluatedBy: "profile_heuristic",
		Issues:      []QualityIssue{},
		Suggestions: []ImprovementSuggestion{},
	}

	// If execution failed, that's the main issue
	if !feedback.Success {
		assessment.Score = 0.1
		assessment.Issues = append(assessment.Issues, QualityIssue{
			Type:        IssuePartialFailure,
			Severity:    1.0,
			Description: "Tool execution failed",
			Evidence:    feedback.ErrorMsg,
			Fixable:     true,
		})
		return assessment
	}

	// Profile-aware efficiency evaluation
	assessment.Efficiency = qe.evaluateEfficiencyWithProfile(feedback, profile)

	// Profile-aware completeness evaluation
	assessment.Completeness = qe.evaluateCompletenessWithProfile(feedback, profile)

	// Profile-aware output validation
	outputIssues := qe.validateOutputWithProfile(feedback, profile)
	assessment.Issues = append(assessment.Issues, outputIssues...)

	// Apply standard heuristic rules
	for _, rule := range qe.heuristicRules {
		if rule.Pattern.MatchString(feedback.Output) {
			assessment.Issues = append(assessment.Issues, QualityIssue{
				Type:        rule.IssueType,
				Severity:    rule.Severity,
				Description: rule.Description,
				Evidence:    extractMatch(feedback.Output, rule.Pattern),
				Fixable:     true,
			})
		}
	}

	// Evaluate custom dimensions
	customScore := qe.evaluateCustomDimensions(feedback, profile)

	// Calculate overall score using profile weights
	issueImpact := 0.0
	for _, issue := range assessment.Issues {
		issueImpact += issue.Severity * 0.15 // Each issue reduces score
	}

	// Weighted score based on tool type
	baseScore := assessment.Completeness*0.35 + assessment.Efficiency*0.25 +
		assessment.Accuracy*0.2 + customScore*0.2

	assessment.Score = clamp(baseScore-issueImpact, 0.0, 1.0)

	// Generate profile-aware suggestions
	assessment.Suggestions = qe.generateProfileAwareSuggestions(assessment.Issues, feedback, profile)

	return assessment
}

// evaluateEfficiencyWithProfile scores efficiency against profile expectations
func (qe *QualityEvaluator) evaluateEfficiencyWithProfile(feedback *ExecutionFeedback, profile *ToolQualityProfile) float64 {
	if profile == nil {
		return qe.evaluateEfficiency(feedback)
	}

	perf := profile.Performance
	duration := feedback.Duration

	// Too fast might indicate incomplete execution
	if perf.ExpectedDurationMin > 0 && duration < perf.ExpectedDurationMin {
		return 0.5 // Suspiciously fast
	}

	// Within acceptable range
	if duration <= perf.AcceptableDuration {
		return 1.0
	}

	// Within expected max
	if duration <= perf.ExpectedDurationMax {
		// Linear degradation from acceptable to max
		ratio := float64(duration-perf.AcceptableDuration) / float64(perf.ExpectedDurationMax-perf.AcceptableDuration)
		return 1.0 - (ratio * 0.3) // 1.0 to 0.7
	}

	// Exceeds expected max but within timeout
	if duration <= perf.TimeoutDuration {
		// Degradation from 0.7 to 0.3
		ratio := float64(duration-perf.ExpectedDurationMax) / float64(perf.TimeoutDuration-perf.ExpectedDurationMax)
		return 0.7 - (ratio * 0.4)
	}

	// Beyond timeout
	return 0.1
}

// evaluateCompletenessWithProfile checks if output meets profile expectations
func (qe *QualityEvaluator) evaluateCompletenessWithProfile(feedback *ExecutionFeedback, profile *ToolQualityProfile) float64 {
	if profile == nil {
		return qe.evaluateCompleteness(feedback)
	}

	output := profile.Output
	outputSize := feedback.OutputSize

	// Check minimum size
	if output.ExpectedMinSize > 0 && outputSize < output.ExpectedMinSize {
		ratio := float64(outputSize) / float64(output.ExpectedMinSize)
		return clamp(ratio, 0.1, 0.7) // Below minimum = 0.1 to 0.7
	}

	// Check if output is within expected range
	if outputSize >= output.ExpectedMinSize && outputSize <= output.ExpectedMaxSize {
		// Perfect if around typical size
		if output.ExpectedTypicalSize > 0 {
			deviation := float64(outputSize-output.ExpectedTypicalSize) / float64(output.ExpectedTypicalSize)
			if deviation < 0 {
				deviation = -deviation
			}
			if deviation < 0.5 {
				return 1.0 // Within 50% of typical
			}
			return 0.9 // Further from typical but in range
		}
		return 0.95
	}

	// Output too large - might indicate issues
	if outputSize > output.ExpectedMaxSize {
		return 0.7 // Suspicious but not necessarily bad
	}

	return 0.8
}

// validateOutputWithProfile checks output against profile expectations
func (qe *QualityEvaluator) validateOutputWithProfile(feedback *ExecutionFeedback, profile *ToolQualityProfile) []QualityIssue {
	issues := []QualityIssue{}
	if profile == nil {
		return issues
	}

	output := profile.Output

	// Check required fields (for JSON output)
	if output.ExpectedFormat == "json" && len(output.RequiredFields) > 0 {
		for _, field := range output.RequiredFields {
			if !strings.Contains(feedback.Output, `"`+field+`"`) {
				issues = append(issues, QualityIssue{
					Type:        IssueIncomplete,
					Severity:    0.6,
					Description: fmt.Sprintf("Missing required field: %s", field),
					Evidence:    fmt.Sprintf("Field '%s' not found in JSON output", field),
					Fixable:     true,
				})
			}
		}
	}

	// Check must-contain patterns
	for _, pattern := range output.MustContain {
		if !strings.Contains(feedback.Output, pattern) {
			issues = append(issues, QualityIssue{
				Type:        IssueIncomplete,
				Severity:    0.5,
				Description: fmt.Sprintf("Missing expected content: %s", pattern),
				Fixable:     true,
			})
		}
	}

	// Check must-not-contain patterns (error indicators)
	for _, pattern := range output.MustNotContain {
		if strings.Contains(feedback.Output, pattern) {
			issues = append(issues, QualityIssue{
				Type:        IssuePartialFailure,
				Severity:    0.7,
				Description: fmt.Sprintf("Output contains error indicator: %s", pattern),
				Evidence:    pattern,
				Fixable:     true,
			})
		}
	}

	// Check pagination expectations
	if output.ExpectsPagination {
		// Look for signs that pagination wasn't handled
		paginationIndicators := regexp.MustCompile(`(?i)(page\s*1\s*of|has_more.*true|next_page|offset.*0)`)
		if paginationIndicators.MatchString(feedback.Output) {
			issues = append(issues, QualityIssue{
				Type:        IssuePagination,
				Severity:    0.7,
				Description: "Pagination expected but only first page may have been fetched",
				Evidence:    extractMatch(feedback.Output, paginationIndicators),
				Fixable:     true,
			})
		}
	}

	return issues
}

// evaluateCustomDimensions evaluates tool-specific quality metrics
func (qe *QualityEvaluator) evaluateCustomDimensions(feedback *ExecutionFeedback, profile *ToolQualityProfile) float64 {
	if profile == nil || len(profile.CustomDimensions) == 0 {
		return 1.0 // No custom dimensions, assume good
	}

	totalWeight := 0.0
	weightedScore := 0.0

	for _, dim := range profile.CustomDimensions {
		totalWeight += dim.Weight

		// Try to extract value using pattern
		if dim.ExtractPattern != "" {
			pattern := regexp.MustCompile(dim.ExtractPattern)
			match := pattern.FindStringSubmatch(feedback.Output)
			if len(match) > 1 {
				// Try to parse as float
				var extractedValue float64
				if _, err := fmt.Sscanf(match[1], "%f", &extractedValue); err == nil {
					// Check if within tolerance
					deviation := extractedValue - dim.ExpectedValue
					if deviation < 0 {
						deviation = -deviation
					}
					if deviation <= dim.Tolerance {
						weightedScore += dim.Weight * 1.0
					} else {
						// Score based on how far outside tolerance
						outsideBy := deviation - dim.Tolerance
						penalty := outsideBy / dim.ExpectedValue
						score := clamp(1.0-penalty, 0.0, 1.0)
						weightedScore += dim.Weight * score
					}
					continue
				}
			}
		}

		// Pattern didn't match or couldn't extract - neutral score
		weightedScore += dim.Weight * 0.7
	}

	if totalWeight == 0 {
		return 1.0
	}
	return weightedScore / totalWeight
}

// generateProfileAwareSuggestions creates suggestions based on profile context
func (qe *QualityEvaluator) generateProfileAwareSuggestions(issues []QualityIssue, feedback *ExecutionFeedback, profile *ToolQualityProfile) []ImprovementSuggestion {
	suggestions := qe.generateSuggestions(issues, feedback)

	if profile == nil {
		return suggestions
	}

	// Add profile-specific suggestions
	usage := profile.UsagePattern

	// If frequently called and slow, suggest caching more aggressively
	if usage.Frequency == FrequencyFrequent || usage.Frequency == FrequencyConstant {
		if feedback.Duration > profile.Performance.AcceptableDuration {
			suggestions = append(suggestions, ImprovementSuggestion{
				Type:        SuggestCaching,
				Priority:    0.9,
				Description: fmt.Sprintf("Tool is frequently called but takes %.1fs - add aggressive caching", feedback.Duration.Seconds()),
				CodeHint:    fmt.Sprintf("Cache results for %v based on cache key: %s", profile.Caching.CacheDuration, profile.Caching.CacheKey),
			})
		}
	}

	// If idempotent but output varies, flag as issue
	if usage.IsIdempotent && profile.Caching.Cacheable {
		// Could compare with previous outputs for same input
		suggestions = append(suggestions, ImprovementSuggestion{
			Type:        SuggestValidation,
			Priority:    0.5,
			Description: "Tool is idempotent - consider adding output validation",
		})
	}

	// If depends on external and slow, suggest retry/timeout handling
	if usage.DependsOnExternal && profile.Performance.MaxRetries > 0 {
		if feedback.RetryCount == 0 && !feedback.Success {
			suggestions = append(suggestions, ImprovementSuggestion{
				Type:        SuggestAddRetry,
				Priority:    0.8,
				Description: fmt.Sprintf("External dependency failure - add retry logic (max %d retries)", profile.Performance.MaxRetries),
			})
		}
	}

	return suggestions
}

// evaluateCompleteness checks if we got all available data
func (qe *QualityEvaluator) evaluateCompleteness(feedback *ExecutionFeedback) float64 {
	// Find matching completeness hint
	var hint *CompletenessHint
	for _, h := range qe.completenessHints {
		if matched, _ := regexp.MatchString(h.ToolPattern, feedback.ToolName); matched {
			hint = &h
			break
		}
	}

	if hint == nil {
		// No specific hint, use generic assessment
		if feedback.OutputSize < 100 {
			return 0.3 // Suspiciously small
		}
		return 0.8 // Assume OK
	}

	// Check against expected minimum
	if feedback.OutputSize < hint.ExpectedMinSize {
		ratio := float64(feedback.OutputSize) / float64(hint.ExpectedMinSize)
		return clamp(ratio, 0.1, 0.9)
	}

	// Check for pagination indicators
	if hint.PaginationPattern != "" {
		if matched, _ := regexp.MatchString(hint.PaginationPattern, feedback.Output); matched {
			// Found pagination - check if we got all pages
			if totalPattern := regexp.MustCompile(hint.TotalPattern); totalPattern != nil {
				// Could extract total and compare, for now penalize
				return 0.5 // Pagination detected but may not be complete
			}
		}
	}

	return 0.9 // Looks complete
}

// evaluateEfficiency scores execution efficiency
func (qe *QualityEvaluator) evaluateEfficiency(feedback *ExecutionFeedback) float64 {
	// Simple duration-based efficiency
	// < 1s = excellent, 1-5s = good, 5-30s = acceptable, >30s = poor
	seconds := feedback.Duration.Seconds()
	switch {
	case seconds < 1:
		return 1.0
	case seconds < 5:
		return 0.8
	case seconds < 30:
		return 0.6
	default:
		return 0.3
	}
}

// generateSuggestions creates improvement suggestions from detected issues
func (qe *QualityEvaluator) generateSuggestions(issues []QualityIssue, feedback *ExecutionFeedback) []ImprovementSuggestion {
	suggestions := []ImprovementSuggestion{}

	for _, issue := range issues {
		switch issue.Type {
		case IssuePagination:
			suggestions = append(suggestions, ImprovementSuggestion{
				Type:        SuggestAddPagination,
				Priority:    0.9,
				Description: "Add pagination handling to fetch all pages",
				CodeHint:    "Add loop to follow next_page/has_more until exhausted",
			})
		case IssueIncomplete:
			suggestions = append(suggestions, ImprovementSuggestion{
				Type:        SuggestIncreaseLimit,
				Priority:    0.8,
				Description: "Increase result limit or add pagination",
				CodeHint:    "Set limit/max_results to higher value or maximum allowed",
			})
		case IssueRateLimit:
			suggestions = append(suggestions, ImprovementSuggestion{
				Type:        SuggestAddRetry,
				Priority:    0.7,
				Description: "Add exponential backoff retry logic",
				CodeHint:    "Implement retry with backoff when rate limited",
			})
		case IssueSlow:
			suggestions = append(suggestions, ImprovementSuggestion{
				Type:        SuggestCaching,
				Priority:    0.5,
				Description: "Add caching for repeated requests",
			})
			suggestions = append(suggestions, ImprovementSuggestion{
				Type:        SuggestParallel,
				Priority:    0.6,
				Description: "Parallelize independent requests",
			})
		}
	}

	return suggestions
}

// EvaluateWithLLM uses LLM for deeper quality assessment
func (qe *QualityEvaluator) EvaluateWithLLM(ctx context.Context, feedback *ExecutionFeedback) (*QualityAssessment, error) {
	// First get heuristic assessment
	assessment := qe.Evaluate(ctx, feedback)

	// Then enhance with LLM analysis
	prompt := fmt.Sprintf(`Analyze this tool execution and assess its quality.

Tool: %s
Input: %s
Output (truncated to 2000 chars): %s
Duration: %v
Success: %v

Existing issues detected: %v

Please provide:
1. Completeness score (0.0-1.0) - Did the tool fetch ALL available data?
2. Accuracy score (0.0-1.0) - Is the output correct and well-formed?
3. Additional issues not detected by heuristics
4. Specific improvement suggestions with code hints

Focus especially on:
- Pagination: Did it only get the first page when more were available?
- Limits: Did it use default limits instead of maximum?
- Error handling: Did it properly handle edge cases?

Return JSON:
{
  "completeness": 0.0-1.0,
  "accuracy": 0.0-1.0,
  "relevance": 0.0-1.0,
  "additional_issues": [{"type": "...", "description": "...", "severity": 0.0-1.0}],
  "suggestions": [{"type": "...", "description": "...", "code_hint": "..."}],
  "reasoning": "..."
}`,
		feedback.ToolName,
		truncate(feedback.Input, 500),
		truncate(feedback.Output, 2000),
		feedback.Duration,
		feedback.Success,
		assessment.Issues,
	)

	resp, err := qe.client.Complete(ctx, prompt)
	if err != nil {
		return assessment, nil // Return heuristic assessment on LLM failure
	}

	// Parse LLM response and merge with heuristic assessment
	var llmResult struct {
		Completeness     float64 `json:"completeness"`
		Accuracy         float64 `json:"accuracy"`
		Relevance        float64 `json:"relevance"`
		AdditionalIssues []struct {
			Type        string  `json:"type"`
			Description string  `json:"description"`
			Severity    float64 `json:"severity"`
		} `json:"additional_issues"`
		Suggestions []struct {
			Type        string `json:"type"`
			Description string `json:"description"`
			CodeHint    string `json:"code_hint"`
		} `json:"suggestions"`
	}

	jsonStr := extractJSON(resp)
	if err := json.Unmarshal([]byte(jsonStr), &llmResult); err == nil {
		// Merge LLM assessment
		assessment.Completeness = (assessment.Completeness + llmResult.Completeness) / 2
		assessment.Accuracy = llmResult.Accuracy
		assessment.Relevance = llmResult.Relevance
		assessment.EvaluatedBy = "llm"

		// Add LLM-detected issues
		for _, issue := range llmResult.AdditionalIssues {
			assessment.Issues = append(assessment.Issues, QualityIssue{
				Type:        IssueType(issue.Type),
				Severity:    issue.Severity,
				Description: issue.Description,
				Fixable:     true,
			})
		}

		// Add LLM suggestions
		for _, sug := range llmResult.Suggestions {
			assessment.Suggestions = append(assessment.Suggestions, ImprovementSuggestion{
				Type:        SuggestionType(sug.Type),
				Description: sug.Description,
				CodeHint:    sug.CodeHint,
				Priority:    0.7,
			})
		}

		// Recalculate score
		assessment.Score = (assessment.Completeness*0.3 + assessment.Accuracy*0.3 +
			assessment.Relevance*0.2 + assessment.Efficiency*0.2)
	}

	return assessment, nil
}

func extractMatch(text string, pattern *regexp.Regexp) string {
	match := pattern.FindString(text)
	if len(match) > 100 {
		return match[:100] + "..."
	}
	return match
}
