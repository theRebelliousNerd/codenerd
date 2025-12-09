package tester

import (
	"codenerd/internal/logging"
	"regexp"
	"strings"
)

// =============================================================================
// AUTOPOIESIS (SELF-IMPROVEMENT)
// =============================================================================

// trackFailurePattern tracks recurring test failure patterns for Autopoiesis (§8.3).
func (t *TesterShard) trackFailurePattern(result *TestResult) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, failed := range result.FailedTests {
		// Create pattern key from failure message
		pattern := normalizePattern(failed.Message)
		t.failurePatterns[pattern]++

		// Persist to LearningStore if count exceeds threshold
		if t.learningStore != nil && t.failurePatterns[pattern] >= 3 {
			_ = t.learningStore.Save("tester", "failure_pattern", []any{pattern, failed.Message}, "")
		}
	}
}

// trackSuccessPattern tracks successful test patterns for Autopoiesis (§8.3).
func (t *TesterShard) trackSuccessPattern(result *TestResult) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, passed := range result.PassedTests {
		// Create pattern key from test name structure
		pattern := normalizePattern(passed)
		t.successPatterns[pattern]++

		// Persist to LearningStore if count exceeds threshold
		if t.learningStore != nil && t.successPatterns[pattern] >= 5 {
			_ = t.learningStore.Save("tester", "success_pattern", []any{pattern, passed}, "")
		}
	}
}

// loadLearnedPatterns loads existing patterns from LearningStore on initialization.
// Must be called with lock held.
func (t *TesterShard) loadLearnedPatterns() {
	if t.learningStore == nil {
		return
	}

	// Load failure patterns
	failureLearnings, err := t.learningStore.LoadByPredicate("tester", "failure_pattern")
	if err == nil {
		for _, learning := range failureLearnings {
			if len(learning.FactArgs) >= 1 {
				pattern, _ := learning.FactArgs[0].(string)
				// Initialize with threshold count to avoid re-learning
				t.failurePatterns[pattern] = 3
			}
		}
	}

	// Load success patterns
	successLearnings, err := t.learningStore.LoadByPredicate("tester", "success_pattern")
	if err == nil {
		for _, learning := range successLearnings {
			if len(learning.FactArgs) >= 1 {
				pattern, _ := learning.FactArgs[0].(string)
				// Initialize with threshold count
				t.successPatterns[pattern] = 5
			}
		}
	}
}

// normalizePattern normalizes a string into a pattern key.
func normalizePattern(s string) string {
	// Remove numbers and specific values, keep structure
	re := regexp.MustCompile(`\d+`)
	normalized := re.ReplaceAllString(s, "N")
	// Limit length
	if len(normalized) > 100 {
		normalized = normalized[:100]
	}
	return strings.ToLower(normalized)
}
