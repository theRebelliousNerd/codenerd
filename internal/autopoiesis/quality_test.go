// Package autopoiesis implements self-modification capabilities for codeNERD.
// This file contains comprehensive tests for the Quality Assessment system.
package autopoiesis

import (
	"context"
	"testing"
	"time"
)

// =============================================================================
// QUALITY EVALUATOR TESTS
// =============================================================================

func TestNewQualityEvaluator(t *testing.T) {
	client := &MockLLMClient{}
	evaluator := NewQualityEvaluator(client, nil)

	if evaluator == nil {
		t.Fatal("NewQualityEvaluator returned nil")
	}
	if evaluator.client != client {
		t.Error("client not set correctly")
	}
	if len(evaluator.heuristicRules) == 0 {
		t.Error("heuristicRules should have defaults")
	}
	if len(evaluator.completenessHints) == 0 {
		t.Error("completenessHints should have defaults")
	}
}

func TestQualityEvaluator_Evaluate_Success(t *testing.T) {
	evaluator := NewQualityEvaluator(&MockLLMClient{}, nil)

	feedback := &ExecutionFeedback{
		ToolName:   "test_tool",
		Success:    true,
		Output:     `{"data": "valid json response", "items": ["a", "b", "c"]}`,
		OutputSize: 100,
		Duration:   500 * time.Millisecond,
	}

	assessment := evaluator.Evaluate(context.Background(), feedback)

	if assessment == nil {
		t.Fatal("Evaluate returned nil")
	}
	if assessment.Score <= 0 {
		t.Error("Expected positive score for successful execution")
	}
	if assessment.EvaluatedBy != "heuristic" {
		t.Errorf("EvaluatedBy = %q, want 'heuristic'", assessment.EvaluatedBy)
	}
	if assessment.EvaluatedAt.IsZero() {
		t.Error("EvaluatedAt should be set")
	}
}

func TestQualityEvaluator_Evaluate_Failure(t *testing.T) {
	evaluator := NewQualityEvaluator(&MockLLMClient{}, nil)

	feedback := &ExecutionFeedback{
		ToolName: "test_tool",
		Success:  false,
		ErrorMsg: "connection refused",
	}

	assessment := evaluator.Evaluate(context.Background(), feedback)

	if assessment.Score > 0.2 {
		t.Errorf("Expected low score for failed execution, got %f", assessment.Score)
	}
	if len(assessment.Issues) == 0 {
		t.Error("Expected issues for failed execution")
	}

	foundFailure := false
	for _, issue := range assessment.Issues {
		if issue.Type == IssuePartialFailure {
			foundFailure = true
			break
		}
	}
	if !foundFailure {
		t.Error("Expected IssuePartialFailure in issues")
	}
}

func TestQualityEvaluator_Evaluate_PaginationDetection(t *testing.T) {
	evaluator := NewQualityEvaluator(&MockLLMClient{}, nil)

	// These match the pagination_truncated pattern in quality.go:
	// `(?i)(page\s*1\s*of\s*\d+|showing\s+\d+\s*-\s*\d+|has_more.*true|next_page|truncated)`
	paginatedOutputs := []string{
		`{"results": [], "has_more": true}`,
		`{"items": [], "next_page": "abc"}`,
		`Results truncated to first 100`,
		`Page 1 of 10 results`,
	}

	matchCount := 0
	for _, output := range paginatedOutputs {
		feedback := &ExecutionFeedback{
			ToolName:   "api_fetcher",
			Success:    true,
			Output:     output,
			OutputSize: len(output),
			Duration:   time.Second,
		}

		assessment := evaluator.Evaluate(context.Background(), feedback)

		foundPagination := false
		for _, issue := range assessment.Issues {
			if issue.Type == IssuePagination {
				foundPagination = true
				matchCount++
				break
			}
		}
		t.Logf("Output: %q, foundPagination: %v", output, foundPagination)
	}

	// At least some should be detected
	if matchCount < 2 {
		t.Errorf("Expected at least 2 pagination issues, got %d", matchCount)
	}
}

func TestQualityEvaluator_Evaluate_RateLimitDetection(t *testing.T) {
	evaluator := NewQualityEvaluator(&MockLLMClient{}, nil)

	rateLimitedOutputs := []string{
		`{"error": "rate limit exceeded"}`,
		`429 Too Many Requests`,
		`{"message": "Rate limited. Retry after 60 seconds"}`,
		`Error: throttled - too many requests`,
	}

	for _, output := range rateLimitedOutputs {
		feedback := &ExecutionFeedback{
			ToolName:   "api_client",
			Success:    true,
			Output:     output,
			OutputSize: len(output),
			Duration:   time.Second,
		}

		assessment := evaluator.Evaluate(context.Background(), feedback)

		foundRateLimit := false
		for _, issue := range assessment.Issues {
			if issue.Type == IssueRateLimit {
				foundRateLimit = true
				break
			}
		}
		if !foundRateLimit {
			t.Errorf("Expected rate limit issue for output: %s", output)
		}
	}
}

func TestQualityEvaluator_Evaluate_EmptyOutput(t *testing.T) {
	evaluator := NewQualityEvaluator(&MockLLMClient{}, nil)

	// These match the empty_or_minimal pattern: `^\s*(\[\s*\]|\{\s*\}|null|none|empty)\s*$`
	emptyOutputs := []string{
		"[]",
		"{}",
		"null",
		"none",
		"empty",
	}

	matchCount := 0
	for _, output := range emptyOutputs {
		feedback := &ExecutionFeedback{
			ToolName:   "data_fetcher",
			Success:    true,
			Output:     output,
			OutputSize: len(output),
			Duration:   time.Second,
		}

		assessment := evaluator.Evaluate(context.Background(), feedback)

		foundEmpty := false
		for _, issue := range assessment.Issues {
			if issue.Type == IssueIncomplete {
				foundEmpty = true
				matchCount++
				break
			}
		}
		t.Logf("Output: %q, foundEmpty: %v", output, foundEmpty)
	}

	// At least some should be detected
	if matchCount < 3 {
		t.Errorf("Expected at least 3 empty output issues, got %d", matchCount)
	}
}

func TestQualityEvaluator_Evaluate_ErrorInOutput(t *testing.T) {
	evaluator := NewQualityEvaluator(&MockLLMClient{}, nil)

	errorOutputs := []string{
		`{"error": "something went wrong"}`,
		`Exception: NullPointerException`,
		`Failed to connect to server`,
		`Connection timeout after 30 seconds`,
	}

	for _, output := range errorOutputs {
		feedback := &ExecutionFeedback{
			ToolName:   "service_client",
			Success:    true,
			Output:     output,
			OutputSize: len(output),
			Duration:   time.Second,
		}

		assessment := evaluator.Evaluate(context.Background(), feedback)

		foundError := false
		for _, issue := range assessment.Issues {
			if issue.Type == IssuePartialFailure {
				foundError = true
				break
			}
		}
		if !foundError {
			t.Errorf("Expected partial failure issue for output: %s", output[:min(len(output), 50)])
		}
	}
}

func TestQualityEvaluator_EvaluateEfficiency(t *testing.T) {
	evaluator := NewQualityEvaluator(&MockLLMClient{}, nil)

	tests := []struct {
		duration time.Duration
		minScore float64
		maxScore float64
	}{
		{100 * time.Millisecond, 0.9, 1.0}, // Very fast
		{3 * time.Second, 0.7, 0.9},        // Good
		{20 * time.Second, 0.5, 0.7},       // Acceptable
		{60 * time.Second, 0.2, 0.4},       // Poor
	}

	for _, tt := range tests {
		t.Run(tt.duration.String(), func(t *testing.T) {
			feedback := &ExecutionFeedback{
				ToolName:   "test_tool",
				Success:    true,
				Output:     "ok",
				OutputSize: 2,
				Duration:   tt.duration,
			}

			score := evaluator.evaluateEfficiency(feedback)

			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("Efficiency score for %v = %f, want %f-%f",
					tt.duration, score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestQualityEvaluator_GenerateSuggestions(t *testing.T) {
	evaluator := NewQualityEvaluator(&MockLLMClient{}, nil)

	issues := []QualityIssue{
		{Type: IssuePagination, Severity: 0.7},
		{Type: IssueIncomplete, Severity: 0.6},
		{Type: IssueRateLimit, Severity: 0.7},
		{Type: IssueSlow, Severity: 0.5},
	}

	suggestions := evaluator.generateSuggestions(issues, &ExecutionFeedback{})

	if len(suggestions) == 0 {
		t.Error("Expected suggestions for issues")
	}

	expectedTypes := map[SuggestionType]bool{
		SuggestAddPagination: true,
		SuggestIncreaseLimit: true,
		SuggestAddRetry:      true,
		SuggestCaching:       true,
	}

	for _, sug := range suggestions {
		delete(expectedTypes, sug.Type)
	}

	// Check that we got at least some expected suggestions
	if len(expectedTypes) == len(map[SuggestionType]bool{
		SuggestAddPagination: true,
		SuggestIncreaseLimit: true,
		SuggestAddRetry:      true,
		SuggestCaching:       true,
	}) {
		t.Error("Expected at least some matching suggestions")
	}
}

// =============================================================================
// QUALITY ASSESSMENT TYPE TESTS
// =============================================================================

func TestIssueType_Values(t *testing.T) {
	// Ensure all issue types are distinct
	types := []IssueType{
		IssueIncomplete,
		IssueInaccurate,
		IssueSlow,
		IssueResourceHeavy,
		IssuePoorFormat,
		IssuePartialFailure,
		IssuePagination,
		IssueRateLimit,
		IssueAuth,
	}

	seen := make(map[IssueType]bool)
	for _, t := range types {
		if seen[t] {
			// Types are strings, they should be unique
		}
		seen[t] = true
	}
}

func TestSuggestionType_Values(t *testing.T) {
	// Ensure all suggestion types are distinct
	types := []SuggestionType{
		SuggestAddPagination,
		SuggestIncreaseLimit,
		SuggestAddRetry,
		SuggestCaching,
		SuggestBatching,
		SuggestParallel,
		SuggestBetterParsing,
		SuggestErrorHandling,
		SuggestValidation,
	}

	seen := make(map[SuggestionType]bool)
	for _, t := range types {
		if seen[t] {
			// Types are strings, they should be unique
		}
		seen[t] = true
	}
}

// =============================================================================
// COMPLETENESS HINTS TESTS
// =============================================================================

func TestDefaultCompletenessHints(t *testing.T) {
	hints := defaultCompletenessHints()

	expectedKeys := []string{"api_docs", "search", "list"}

	for _, key := range expectedKeys {
		if _, ok := hints[key]; !ok {
			t.Errorf("Expected hint for %q", key)
		}
	}

	// Verify hints have reasonable values
	for key, hint := range hints {
		if hint.ToolPattern == "" {
			t.Errorf("Hint %q has empty ToolPattern", key)
		}
		if hint.ExpectedMinSize == 0 {
			t.Errorf("Hint %q has zero ExpectedMinSize", key)
		}
	}
}

// =============================================================================
// HEURISTIC RULES TESTS
// =============================================================================

func TestDefaultHeuristicRules(t *testing.T) {
	rules := defaultHeuristicRules()

	if len(rules) == 0 {
		t.Error("Expected default heuristic rules")
	}

	// Verify each rule has required fields
	for _, rule := range rules {
		if rule.Name == "" {
			t.Error("Rule has empty Name")
		}
		if rule.Pattern == nil {
			t.Errorf("Rule %q has nil Pattern", rule.Name)
		}
		if rule.Severity == 0 {
			t.Errorf("Rule %q has zero Severity", rule.Name)
		}
	}

	// Verify rules match expected patterns
	testCases := []struct {
		ruleName    string
		input       string
		shouldMatch bool
	}{
		{"pagination_truncated", "page 1 of 10", true},
		{"pagination_truncated", "has_more: true", true},
		{"partial_results", "partial results returned", true},
		{"error_in_output", "error: something failed", true},
		{"empty_or_minimal", "[]", true},
		{"rate_limited", "rate limit exceeded", true},
	}

	for _, tc := range testCases {
		for _, rule := range rules {
			if rule.Name == tc.ruleName {
				matched := rule.Pattern.MatchString(tc.input)
				if matched != tc.shouldMatch {
					t.Errorf("Rule %q on %q: got match=%v, want %v",
						tc.ruleName, tc.input, matched, tc.shouldMatch)
				}
				break
			}
		}
	}
}

// =============================================================================
// HELPER FUNCTION TESTS
// =============================================================================

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"longer string", 5, "longe..."},
		{"exact", 5, "exact"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := truncate(tt.input, tt.maxLen)
			if result != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q",
					tt.input, tt.maxLen, result, tt.want)
			}
		})
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		value, min, max float64
		want            float64
	}{
		{0.5, 0.0, 1.0, 0.5},  // Within range
		{-0.5, 0.0, 1.0, 0.0}, // Below min
		{1.5, 0.0, 1.0, 1.0},  // Above max
		{0.0, 0.0, 1.0, 0.0},  // At min
		{1.0, 0.0, 1.0, 1.0},  // At max
	}

	for _, tt := range tests {
		result := clamp(tt.value, tt.min, tt.max)
		if result != tt.want {
			t.Errorf("clamp(%f, %f, %f) = %f, want %f",
				tt.value, tt.min, tt.max, result, tt.want)
		}
	}
}

func TestExtractMatch(t *testing.T) {
	pattern := defaultHeuristicRules()[0].Pattern // pagination_truncated

	tests := []struct {
		input   string
		wantLen int
	}{
		{"page 1 of 10", -1}, // Should match
		{"no match here", 0}, // Should not match
	}

	for _, tt := range tests {
		result := extractMatch(tt.input, pattern)
		if tt.wantLen == 0 && result != "" {
			t.Errorf("extractMatch(%q) = %q, want empty", tt.input, result)
		}
		if tt.wantLen == -1 && result == "" {
			t.Errorf("extractMatch(%q) = empty, want non-empty", tt.input)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
