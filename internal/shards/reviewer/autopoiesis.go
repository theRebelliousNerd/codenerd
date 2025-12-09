package reviewer

import (
	"codenerd/internal/logging"
	"regexp"
	"strings"
)

// =============================================================================
// AUTOPOIESIS (SELF-IMPROVEMENT)
// =============================================================================

// trackReviewPatterns tracks patterns for Autopoiesis (§8.3).
func (r *ReviewerShard) trackReviewPatterns(result *ReviewResult) {
	logging.ReviewerDebug("Tracking review patterns for autopoiesis: %d findings", len(result.Findings))
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, finding := range result.Findings {
		// Track flagged patterns
		if finding.Severity == "critical" || finding.Severity == "error" {
			pattern := normalizeReviewPattern(finding.Message)
			r.flaggedPatterns[pattern]++

			// Persist to LearningStore if count exceeds threshold
			if r.learningStore != nil && r.flaggedPatterns[pattern] >= 3 {
				logging.ReviewerDebug("Persisting flagged pattern to learning store: %s (count: %d)", pattern, r.flaggedPatterns[pattern])
				_ = r.learningStore.Save("reviewer", "flagged_pattern", []any{pattern, finding.Category, finding.Severity}, "")
			}
		}

		// Track approved patterns (clean code)
		if result.Severity == ReviewSeverityClean || finding.Severity == "info" {
			pattern := normalizeReviewPattern(finding.File)
			r.approvedPatterns[pattern]++

			// Persist to LearningStore if count exceeds threshold
			if r.learningStore != nil && r.approvedPatterns[pattern] >= 5 {
				_ = r.learningStore.Save("reviewer", "approved_pattern", []any{pattern}, "")
			}
		}
	}
}

// LearnAntiPattern adds a new anti-pattern to watch for.
func (r *ReviewerShard) LearnAntiPattern(pattern, reason string) {
	logging.Reviewer("Learning new anti-pattern: %s (reason: %s)", pattern, reason)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.learnedAntiPatterns[pattern] = reason

	// Persist to LearningStore immediately for anti-patterns
	if r.learningStore != nil {
		logging.ReviewerDebug("Persisting anti-pattern to learning store")
		_ = r.learningStore.Save("reviewer", "anti_pattern", []any{pattern, reason}, "")
	}
}

// loadLearnedPatterns loads existing patterns from LearningStore on initialization.
// Must be called with lock held.
func (r *ReviewerShard) loadLearnedPatterns() {
	logging.ReviewerDebug("Loading learned patterns from LearningStore")
	if r.learningStore == nil {
		logging.ReviewerDebug("No LearningStore available, skipping pattern load")
		return
	}

	// Load flagged patterns
	flaggedLearnings, err := r.learningStore.LoadByPredicate("reviewer", "flagged_pattern")
	if err == nil {
		for _, learning := range flaggedLearnings {
			if len(learning.FactArgs) >= 1 {
				pattern, _ := learning.FactArgs[0].(string)
				// Initialize with threshold count to avoid re-learning
				r.flaggedPatterns[pattern] = 3
			}
		}
		logging.ReviewerDebug("Loaded %d flagged patterns", len(flaggedLearnings))
	}

	// Load approved patterns
	approvedLearnings, err := r.learningStore.LoadByPredicate("reviewer", "approved_pattern")
	if err == nil {
		for _, learning := range approvedLearnings {
			if len(learning.FactArgs) >= 1 {
				pattern, _ := learning.FactArgs[0].(string)
				// Initialize with threshold count
				r.approvedPatterns[pattern] = 5
			}
		}
		logging.ReviewerDebug("Loaded %d approved patterns", len(approvedLearnings))
	}

	// Load anti-patterns
	antiPatternLearnings, err := r.learningStore.LoadByPredicate("reviewer", "anti_pattern")
	if err == nil {
		for _, learning := range antiPatternLearnings {
			if len(learning.FactArgs) >= 2 {
				pattern, _ := learning.FactArgs[0].(string)
				reason, _ := learning.FactArgs[1].(string)
				r.learnedAntiPatterns[pattern] = reason
			}
		}
		logging.ReviewerDebug("Loaded %d anti-patterns", len(antiPatternLearnings))
	}
}

// normalizeReviewPattern normalizes a finding message into a pattern key.
func normalizeReviewPattern(s string) string {
	// Remove specific values, keep structure
	re := regexp.MustCompile(`\d+`)
	normalized := re.ReplaceAllString(s, "N")
	if len(normalized) > 100 {
		normalized = normalized[:100]
	}
	return strings.ToLower(normalized)
}
