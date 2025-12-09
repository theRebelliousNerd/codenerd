package reviewer

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"
)

// =============================================================================
// REVIEWER FEEDBACK LOOP
// =============================================================================
// This file implements the self-correction system for the reviewer.
// It tracks user feedback, learns from mistakes, and flags suspect reviews.

// ReviewFeedback tracks feedback for a single review session.
type ReviewFeedback struct {
	ReviewID      string
	Findings      []ReviewFinding
	Accepted      []int // Indices of accepted findings
	Rejected      []RejectedFinding
	Timestamp     time.Time
	AccuracyScore float64
}

// RejectedFinding tracks a finding that was rejected by the user.
type RejectedFinding struct {
	FindingIndex int
	File         string
	Line         int
	Reason       string
	Timestamp    time.Time
}

// FalsePositivePattern tracks patterns that cause false positives.
type FalsePositivePattern struct {
	Pattern     string
	Category    string
	Occurrences int
	Confidence  float64
}

// =============================================================================
// FEEDBACK TRACKING METHODS
// =============================================================================

// RecordFinding records a finding to the kernel for feedback tracking.
func (r *ReviewerShard) RecordFinding(reviewID string, finding ReviewFinding) {
	if r.kernel == nil {
		return
	}

	// Assert the finding to the kernel
	r.kernel.Assert(core.Fact{
		Predicate: "review_finding",
		Args: []interface{}{
			reviewID,
			finding.File,
			finding.Line,
			finding.Severity,
			finding.Category,
			finding.Message,
		},
	})
}

// RecordReviewFindings records all findings from a review result.
func (r *ReviewerShard) RecordReviewFindings(reviewID string, result *ReviewResult) {
	if r.kernel == nil {
		return
	}

	for _, finding := range result.Findings {
		r.RecordFinding(reviewID, finding)
	}
}

// AcceptFinding marks a finding as accepted (user applied the suggestion).
func (r *ReviewerShard) AcceptFinding(reviewID, file string, line int) {
	if r.kernel == nil {
		return
	}

	r.kernel.Assert(core.Fact{
		Predicate: "user_accepted_finding",
		Args: []interface{}{
			reviewID,
			file,
			line,
			time.Now().Unix(),
		},
	})

	// Update accuracy tracking
	r.updateAccuracy(reviewID)
}

// RejectFinding marks a finding as rejected (user says it's wrong).
func (r *ReviewerShard) RejectFinding(reviewID, file string, line int, reason string) {
	if r.kernel == nil {
		return
	}

	r.kernel.Assert(core.Fact{
		Predicate: "user_rejected_finding",
		Args: []interface{}{
			reviewID,
			file,
			line,
			reason,
			time.Now().Unix(),
		},
	})

	// Learn from the rejection
	r.learnFromRejection(reviewID, file, line, reason)

	// Update accuracy tracking
	r.updateAccuracy(reviewID)
}

// learnFromRejection extracts patterns from rejected findings for future suppression.
func (r *ReviewerShard) learnFromRejection(reviewID, file string, line int, reason string) {
	if r.kernel == nil {
		return
	}

	// Get the original finding
	facts, err := r.kernel.Query("review_finding")
	if err != nil {
		return
	}

	var originalMessage, originalCategory string
	for _, fact := range facts {
		if len(fact.Args) < 6 {
			continue
		}
		if fact.Args[0] == reviewID && fact.Args[1] == file && fact.Args[2] == line {
			originalCategory, _ = fact.Args[4].(string)
			originalMessage, _ = fact.Args[5].(string)
			break
		}
	}

	if originalMessage == "" {
		return
	}

	// Extract pattern from the rejection
	pattern := extractRejectionPattern(originalMessage, reason)
	if pattern == "" {
		return
	}

	// Update or create false positive pattern
	r.updateFalsePositivePattern(pattern, originalCategory)
}

// extractRejectionPattern extracts a generalizable pattern from a rejection.
func extractRejectionPattern(message, reason string) string {
	// Common rejection reasons that indicate false positive patterns
	reasonLower := strings.ToLower(reason)

	// "Symbol exists" type rejections
	if strings.Contains(reasonLower, "exists") || strings.Contains(reasonLower, "defined") {
		// Extract the symbol name from the message
		undefinedPattern := regexp.MustCompile(`(?:undefined|not found|missing).*['"\` + "`" + `]([^'"\` + "`" + `]+)['"\` + "`" + `]`)
		if matches := undefinedPattern.FindStringSubmatch(message); len(matches) > 1 {
			return "undefined:" + matches[1]
		}
	}

	// "False positive" type rejections
	if strings.Contains(reasonLower, "false positive") || strings.Contains(reasonLower, "not a bug") {
		// Normalize the message to create a pattern
		normalized := normalizeMessageForPattern(message)
		return "fp:" + normalized
	}

	// "Package scope" type rejections
	if strings.Contains(reasonLower, "package") || strings.Contains(reasonLower, "same package") {
		return "package_scope_blindness"
	}

	return ""
}

// normalizeMessageForPattern creates a generalizable pattern from a message.
func normalizeMessageForPattern(message string) string {
	// Remove specific identifiers to create a pattern
	re := regexp.MustCompile(`['"\` + "`" + `][^'"\` + "`" + `]+['"\` + "`" + `]`)
	normalized := re.ReplaceAllString(message, "IDENTIFIER")

	// Remove line numbers
	re = regexp.MustCompile(`line \d+`)
	normalized = re.ReplaceAllString(normalized, "line N")

	// Truncate
	if len(normalized) > 80 {
		normalized = normalized[:80]
	}

	return strings.ToLower(normalized)
}

// updateFalsePositivePattern updates or creates a false positive pattern.
func (r *ReviewerShard) updateFalsePositivePattern(pattern, category string) {
	if r.kernel == nil {
		return
	}

	// Check if pattern already exists
	facts, err := r.kernel.Query("false_positive_pattern")
	if err != nil {
		return
	}

	var existingOccurrences int
	for _, fact := range facts {
		if len(fact.Args) >= 4 {
			if fact.Args[0] == pattern && fact.Args[1] == category {
				if occ, ok := fact.Args[2].(int64); ok {
					existingOccurrences = int(occ)
				}
				break
			}
		}
	}

	// Increment occurrences and update confidence
	newOccurrences := existingOccurrences + 1
	confidence := calculatePatternConfidence(newOccurrences)

	// Retract old fact if exists
	if existingOccurrences > 0 {
		r.kernel.Retract("false_positive_pattern")
	}

	// Assert updated pattern
	r.kernel.Assert(core.Fact{
		Predicate: "false_positive_pattern",
		Args: []interface{}{
			pattern,
			category,
			int64(newOccurrences),
			confidence,
		},
	})

	// Also persist to LearningStore for cross-session learning
	if r.learningStore != nil {
		_ = r.learningStore.Save("reviewer", "false_positive_pattern",
			[]any{pattern, category, newOccurrences, confidence}, "")
	}

	logging.Reviewer("Learned false positive pattern: %s (occurrences: %d, confidence: %.2f)",
		pattern, newOccurrences, confidence)
}

// calculatePatternConfidence calculates confidence based on occurrences.
func calculatePatternConfidence(occurrences int) float64 {
	// Sigmoid-like growth: starts at 0.3, asymptotes to 0.95
	// 3 occurrences = ~0.7, 5 occurrences = ~0.85
	base := 0.3
	growth := 0.65 * (1 - 1/float64(occurrences+1))
	return base + growth
}

// updateAccuracy recalculates accuracy for a review session.
func (r *ReviewerShard) updateAccuracy(reviewID string) {
	if r.kernel == nil {
		return
	}

	// Count total findings
	findingFacts, _ := r.kernel.Query("review_finding")
	totalFindings := 0
	for _, fact := range findingFacts {
		if len(fact.Args) > 0 && fact.Args[0] == reviewID {
			totalFindings++
		}
	}

	// Count accepted
	acceptedFacts, _ := r.kernel.Query("user_accepted_finding")
	accepted := 0
	for _, fact := range acceptedFacts {
		if len(fact.Args) > 0 && fact.Args[0] == reviewID {
			accepted++
		}
	}

	// Count rejected
	rejectedFacts, _ := r.kernel.Query("user_rejected_finding")
	rejected := 0
	for _, fact := range rejectedFacts {
		if len(fact.Args) > 0 && fact.Args[0] == reviewID {
			rejected++
		}
	}

	// Calculate score
	var score float64
	if accepted+rejected > 0 {
		score = float64(accepted) / float64(accepted+rejected)
	} else {
		score = 1.0 // No feedback yet, assume accurate
	}

	// Assert accuracy fact
	r.kernel.Assert(core.Fact{
		Predicate: "review_accuracy",
		Args: []interface{}{
			reviewID,
			int64(totalFindings),
			int64(accepted),
			int64(rejected),
			score,
		},
	})
}

// =============================================================================
// VALIDATION TRIGGERS
// =============================================================================

// NeedsValidation checks if this review should be spot-checked.
func (r *ReviewerShard) NeedsValidation(reviewID string) bool {
	if r.kernel == nil {
		return false
	}

	// Query the derived predicate
	results, err := r.kernel.Query("reviewer_needs_validation")
	if err != nil {
		return false
	}

	for _, fact := range results {
		if len(fact.Args) > 0 && fact.Args[0] == reviewID {
			return true
		}
	}

	return false
}

// GetSuspectReasons returns why a review is flagged as suspect.
func (r *ReviewerShard) GetSuspectReasons(reviewID string) []string {
	if r.kernel == nil {
		return nil
	}

	results, err := r.kernel.Query("review_suspect")
	if err != nil {
		return nil
	}

	var reasons []string
	for _, fact := range results {
		if len(fact.Args) >= 2 && fact.Args[0] == reviewID {
			if reason, ok := fact.Args[1].(string); ok {
				reasons = append(reasons, reason)
			}
		}
	}

	return reasons
}

// VerifySymbolExists records that a symbol was verified to exist in a file.
// This helps prevent future false positives about "undefined" symbols.
func (r *ReviewerShard) VerifySymbolExists(symbol, file string) {
	if r.kernel == nil {
		return
	}

	r.kernel.Assert(core.Fact{
		Predicate: "symbol_verified_exists",
		Args: []interface{}{
			symbol,
			file,
			time.Now().Unix(),
		},
	})
}

// =============================================================================
// FEEDBACK REPORT
// =============================================================================

// GetAccuracyReport returns a summary of review accuracy for a session.
func (r *ReviewerShard) GetAccuracyReport(reviewID string) string {
	if r.kernel == nil {
		return "No kernel available for accuracy tracking"
	}

	facts, err := r.kernel.Query("review_accuracy")
	if err != nil {
		return fmt.Sprintf("Error querying accuracy: %v", err)
	}

	for _, fact := range facts {
		if len(fact.Args) >= 5 && fact.Args[0] == reviewID {
			total, _ := fact.Args[1].(int64)
			accepted, _ := fact.Args[2].(int64)
			rejected, _ := fact.Args[3].(int64)
			score, _ := fact.Args[4].(float64)

			return fmt.Sprintf(
				"Review %s: %d findings, %d accepted, %d rejected, accuracy %.0f%%",
				reviewID, total, accepted, rejected, score*100,
			)
		}
	}

	return fmt.Sprintf("No accuracy data for review %s", reviewID)
}

// GetLearnedFalsePositives returns all learned false positive patterns.
func (r *ReviewerShard) GetLearnedFalsePositives() []FalsePositivePattern {
	if r.kernel == nil {
		return nil
	}

	facts, err := r.kernel.Query("false_positive_pattern")
	if err != nil {
		return nil
	}

	var patterns []FalsePositivePattern
	for _, fact := range facts {
		if len(fact.Args) >= 4 {
			pattern := FalsePositivePattern{
				Pattern:  fmt.Sprintf("%v", fact.Args[0]),
				Category: fmt.Sprintf("%v", fact.Args[1]),
			}
			if occ, ok := fact.Args[2].(int64); ok {
				pattern.Occurrences = int(occ)
			}
			if conf, ok := fact.Args[3].(float64); ok {
				pattern.Confidence = conf
			}
			patterns = append(patterns, pattern)
		}
	}

	return patterns
}
