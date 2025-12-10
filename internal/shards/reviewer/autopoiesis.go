package reviewer

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"codenerd/internal/core"
	"codenerd/internal/logging"
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

// =============================================================================
// SUPPRESSION CONFIDENCE TRACKING
// =============================================================================

// suppressionConfidence tracks confidence scores for suppressions in memory.
// Protected by its own mutex for fine-grained locking.
var (
	suppressionConfidenceMu    sync.RWMutex
	suppressionConfidenceCache = make(map[string]float64)
)

// makeSuppressionKey creates a unique key for a suppression entry.
func makeSuppressionKey(hypoType HypothesisType, file string, line int) string {
	return fmt.Sprintf("%s:%s:%d", hypoType, file, line)
}

// =============================================================================
// LEARNING FROM DISMISSALS
// =============================================================================

// LearnFromDismissal records a dismissed hypothesis to prevent future false positives.
// This is the core autopoiesis feedback loop: when the LLM determines a hypothesis
// is a false positive, we learn from it to improve future accuracy.
func (r *ReviewerShard) LearnFromDismissal(hypo Hypothesis, reason string) error {
	if r.learningStore == nil {
		return fmt.Errorf("learning store not initialized")
	}

	// 1. Persist suppression fact via LearningStore
	// Format: suppressed_rule(Type, File, Line, Reason)
	if err := r.learningStore.Save(
		"reviewer",
		"suppressed_rule",
		[]any{string(hypo.Type), hypo.File, int64(hypo.Line), reason},
		"",
	); err != nil {
		return fmt.Errorf("failed to persist suppression: %w", err)
	}

	// 2. Update confidence score (sigmoid growth toward 100)
	currentScore := r.getSuppressionConfidence(hypo)
	newScore := currentScore + (100-currentScore)*0.2 // Asymptotic growth
	r.updateSuppressionConfidence(hypo, newScore)

	// 3. Check for pattern promotion to global rules
	if newScore > 90 {
		r.promoteToGlobalPattern(hypo, reason)
	}

	// 4. Also assert to kernel if available for immediate rule effect
	if r.kernel != nil {
		fact := core.Fact{
			Predicate: "suppressed_rule",
			Args:      []interface{}{string(hypo.Type), hypo.File, int64(hypo.Line), reason},
		}
		if err := r.kernel.Assert(fact); err != nil {
			logging.Get(logging.CategoryReviewer).Warn(
				"Failed to assert suppression to kernel: %v", err)
		}
	}

	logging.Get(logging.CategoryReviewer).Info(
		"Learned from dismissal: %s at %s:%d (confidence: %.0f%%)",
		hypo.Type, hypo.File, hypo.Line, newScore,
	)

	return nil
}

// getSuppressionConfidence retrieves the current confidence score for a suppression.
// Higher confidence means the suppression is well-established.
func (r *ReviewerShard) getSuppressionConfidence(hypo Hypothesis) float64 {
	key := makeSuppressionKey(hypo.Type, hypo.File, hypo.Line)

	// Check in-memory cache first
	suppressionConfidenceMu.RLock()
	if score, exists := suppressionConfidenceCache[key]; exists {
		suppressionConfidenceMu.RUnlock()
		return score
	}
	suppressionConfidenceMu.RUnlock()

	// Query kernel if available
	if r.kernel != nil {
		query := fmt.Sprintf("suppression_confidence(%q, %q, %d, Score)",
			string(hypo.Type), hypo.File, hypo.Line)

		results, err := r.kernel.Query(query)
		if err == nil && len(results) > 0 {
			// Extract score from first result
			if len(results[0].Args) >= 4 {
				if score, ok := results[0].Args[3].(float64); ok {
					return score
				}
				if score, ok := results[0].Args[3].(int64); ok {
					return float64(score)
				}
				if score, ok := results[0].Args[3].(int); ok {
					return float64(score)
				}
			}
		}
	}

	// Default starting confidence for new suppressions
	return 50.0
}

// updateSuppressionConfidence updates the confidence score for a suppression.
// Persists to both in-memory cache and LearningStore for durability.
func (r *ReviewerShard) updateSuppressionConfidence(hypo Hypothesis, score float64) {
	key := makeSuppressionKey(hypo.Type, hypo.File, hypo.Line)

	// Update in-memory cache
	suppressionConfidenceMu.Lock()
	suppressionConfidenceCache[key] = score
	suppressionConfidenceMu.Unlock()

	// Persist to LearningStore
	if r.learningStore != nil {
		if err := r.learningStore.Save(
			"reviewer",
			"suppression_confidence",
			[]any{string(hypo.Type), hypo.File, int64(hypo.Line), int64(score)},
			"",
		); err != nil {
			logging.Get(logging.CategoryReviewer).Warn(
				"Failed to persist suppression confidence: %v", err)
		}
	}

	// Assert to kernel for rule engine visibility
	if r.kernel != nil {
		fact := core.Fact{
			Predicate: "suppression_confidence",
			Args:      []interface{}{string(hypo.Type), hypo.File, int64(hypo.Line), int64(score)},
		}
		// Ignore error - kernel assertion is best-effort for immediate effect
		_ = r.kernel.Assert(fact)
	}
}

// promoteToGlobalPattern promotes a high-confidence suppression to project-wide.
// When a specific suppression reaches high confidence, we generalize it to a
// pattern that can apply across the codebase, reducing future false positives.
func (r *ReviewerShard) promoteToGlobalPattern(hypo Hypothesis, reason string) {
	// Extract a generalizable pattern from the specific instance
	pattern := extractSuppressionPattern(hypo, reason)
	if pattern == "" {
		return // Could not extract a meaningful pattern
	}

	// Persist global pattern
	if r.learningStore != nil {
		if err := r.learningStore.Save(
			"reviewer",
			"global_suppression_pattern",
			[]any{string(hypo.Type), pattern},
			"",
		); err != nil {
			logging.Get(logging.CategoryReviewer).Warn(
				"Failed to persist global suppression pattern: %v", err)
			return
		}
	}

	// Assert to kernel for immediate effect
	if r.kernel != nil {
		fact := core.Fact{
			Predicate: "global_suppression_pattern",
			Args:      []interface{}{string(hypo.Type), pattern},
		}
		_ = r.kernel.Assert(fact)
	}

	logging.Get(logging.CategoryReviewer).Info(
		"Promoted to global pattern: %s -> %s",
		hypo.Type, pattern,
	)
}

// extractSuppressionPattern identifies a reusable pattern from a specific suppression.
// Analyzes the reason string to find generalizable patterns that can apply elsewhere.
func extractSuppressionPattern(hypo Hypothesis, reason string) string {
	reasonLower := strings.ToLower(reason)

	// Map common dismissal reasons to reusable patterns
	patternMatchers := []struct {
		keywords []string
		pattern  string
	}{
		// Guard clause patterns
		{
			keywords: []string{"guard", "early return", "nil check", "checked above", "checked before"},
			pattern:  "guarded_by_early_return",
		},
		// Initialization patterns
		{
			keywords: []string{"sync.once", "singleton", "init", "initialized"},
			pattern:  "guarded_by_sync_once",
		},
		// Test file patterns
		{
			keywords: []string{"test file", "test code", "_test.go", "unit test", "testing"},
			pattern:  "test_file_acceptable",
		},
		// Intentional patterns
		{
			keywords: []string{"intentional", "by design", "expected", "deliberate"},
			pattern:  "intentional_by_design",
		},
		// Error handling patterns
		{
			keywords: []string{"error handled", "errors.is", "error checked", "wrapped"},
			pattern:  "error_properly_handled",
		},
		// Context patterns
		{
			keywords: []string{"context cancel", "context done", "ctx.done", "context.withcancel"},
			pattern:  "context_managed",
		},
		// Mutex patterns
		{
			keywords: []string{"mutex", "lock", "synchronized", "atomic"},
			pattern:  "mutex_protected",
		},
		// Defer patterns
		{
			keywords: []string{"defer", "deferred", "cleanup"},
			pattern:  "deferred_cleanup",
		},
		// Channel patterns
		{
			keywords: []string{"buffered channel", "channel buffer", "select default"},
			pattern:  "channel_buffered_or_select",
		},
		// External validation
		{
			keywords: []string{"validated elsewhere", "validated by", "sanitized", "escaped"},
			pattern:  "validated_externally",
		},
	}

	for _, matcher := range patternMatchers {
		for _, keyword := range matcher.keywords {
			if strings.Contains(reasonLower, keyword) {
				return matcher.pattern
			}
		}
	}

	// If no specific pattern matches, create a generic pattern from the hypothesis type
	// This allows similar issues to be grouped even without a specific reason match
	return fmt.Sprintf("dismissed_%s", hypo.Type)
}

// =============================================================================
// SUPPRESSION LOADING
// =============================================================================

// LoadSuppressions loads learned suppressions into the kernel at startup.
// This ensures previously learned false positives are immediately filtered.
func (r *ReviewerShard) LoadSuppressions() error {
	if r.learningStore == nil || r.kernel == nil {
		return nil
	}

	var loadedCount int

	// Load suppressed_rule facts
	suppressions, err := r.learningStore.LoadByPredicate("reviewer", "suppressed_rule")
	if err != nil {
		logging.Get(logging.CategoryReviewer).Warn(
			"Failed to load suppressed_rule learnings: %v", err)
	} else {
		for _, learning := range suppressions {
			if len(learning.FactArgs) < 4 {
				continue
			}

			hypoType, _ := learning.FactArgs[0].(string)
			file, _ := learning.FactArgs[1].(string)
			line := toInt64(learning.FactArgs[2])
			reason, _ := learning.FactArgs[3].(string)

			fact := core.Fact{
				Predicate: "suppressed_rule",
				Args:      []interface{}{hypoType, file, line, reason},
			}
			if err := r.kernel.Assert(fact); err != nil {
				logging.Get(logging.CategoryReviewer).Debug(
					"Failed to assert suppression %s:%d: %v", file, line, err)
				continue
			}
			loadedCount++
		}
	}

	// Load suppression_confidence facts and populate cache
	confidences, err := r.learningStore.LoadByPredicate("reviewer", "suppression_confidence")
	if err != nil {
		logging.Get(logging.CategoryReviewer).Warn(
			"Failed to load suppression_confidence learnings: %v", err)
	} else {
		suppressionConfidenceMu.Lock()
		for _, learning := range confidences {
			if len(learning.FactArgs) < 4 {
				continue
			}

			hypoType, _ := learning.FactArgs[0].(string)
			file, _ := learning.FactArgs[1].(string)
			line := toInt64(learning.FactArgs[2])
			score := toFloat64(learning.FactArgs[3])

			key := fmt.Sprintf("%s:%s:%d", hypoType, file, line)
			suppressionConfidenceCache[key] = score

			fact := core.Fact{
				Predicate: "suppression_confidence",
				Args:      []interface{}{hypoType, file, line, int64(score)},
			}
			_ = r.kernel.Assert(fact)
		}
		suppressionConfidenceMu.Unlock()
	}

	// Load global_suppression_pattern facts
	patterns, err := r.learningStore.LoadByPredicate("reviewer", "global_suppression_pattern")
	if err != nil {
		logging.Get(logging.CategoryReviewer).Warn(
			"Failed to load global_suppression_pattern learnings: %v", err)
	} else {
		for _, learning := range patterns {
			if len(learning.FactArgs) < 2 {
				continue
			}

			hypoType, _ := learning.FactArgs[0].(string)
			pattern, _ := learning.FactArgs[1].(string)

			fact := core.Fact{
				Predicate: "global_suppression_pattern",
				Args:      []interface{}{hypoType, pattern},
			}
			if err := r.kernel.Assert(fact); err != nil {
				logging.Get(logging.CategoryReviewer).Debug(
					"Failed to assert global pattern %s: %v", pattern, err)
			}
		}
	}

	logging.Get(logging.CategoryReviewer).Debug(
		"Loaded %d suppression rules from LearningStore", loadedCount)
	return nil
}

// =============================================================================
// BATCH LEARNING OPERATIONS
// =============================================================================

// LearnFromDismissals processes multiple dismissals in batch.
// More efficient than calling LearnFromDismissal individually.
func (r *ReviewerShard) LearnFromDismissals(dismissals []DismissedHypothesis) error {
	if r.learningStore == nil {
		return fmt.Errorf("learning store not initialized")
	}

	var errs []error
	for _, d := range dismissals {
		if err := r.LearnFromDismissal(d.Hypothesis, d.Reason); err != nil {
			errs = append(errs, fmt.Errorf("dismissal %s:%d: %w",
				d.Hypothesis.File, d.Hypothesis.Line, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to process %d/%d dismissals: %v",
			len(errs), len(dismissals), errs[0])
	}

	return nil
}

// DismissedHypothesis pairs a hypothesis with its dismissal reason.
type DismissedHypothesis struct {
	Hypothesis Hypothesis
	Reason     string
}

// GetSuppressionStats returns statistics about learned suppressions.
func (r *ReviewerShard) GetSuppressionStats() SuppressionStats {
	stats := SuppressionStats{
		ByType:    make(map[HypothesisType]int),
		ByPattern: make(map[string]int),
	}

	if r.learningStore == nil {
		return stats
	}

	// Count suppressions by type
	suppressions, err := r.learningStore.LoadByPredicate("reviewer", "suppressed_rule")
	if err == nil {
		stats.TotalSuppressions = len(suppressions)
		for _, s := range suppressions {
			if len(s.FactArgs) >= 1 {
				if typeStr, ok := s.FactArgs[0].(string); ok {
					stats.ByType[HypothesisType(typeStr)]++
				}
			}
		}
	}

	// Count global patterns
	patterns, err := r.learningStore.LoadByPredicate("reviewer", "global_suppression_pattern")
	if err == nil {
		stats.GlobalPatterns = len(patterns)
		for _, p := range patterns {
			if len(p.FactArgs) >= 2 {
				if pattern, ok := p.FactArgs[1].(string); ok {
					stats.ByPattern[pattern]++
				}
			}
		}
	}

	// Calculate high confidence count
	suppressionConfidenceMu.RLock()
	for _, score := range suppressionConfidenceCache {
		if score > 90 {
			stats.HighConfidenceCount++
		}
	}
	suppressionConfidenceMu.RUnlock()

	return stats
}

// SuppressionStats holds statistics about learned suppressions.
type SuppressionStats struct {
	TotalSuppressions   int
	GlobalPatterns      int
	HighConfidenceCount int
	ByType              map[HypothesisType]int
	ByPattern           map[string]int
}

// =============================================================================
// SUPPRESSION QUERYING
// =============================================================================

// IsHypothesisSuppressed checks if a hypothesis should be suppressed based on
// learned patterns. Returns true if the hypothesis matches a known false positive.
func (r *ReviewerShard) IsHypothesisSuppressed(hypo Hypothesis) (bool, string) {
	// Check specific suppression first
	key := makeSuppressionKey(hypo.Type, hypo.File, hypo.Line)

	suppressionConfidenceMu.RLock()
	score, exists := suppressionConfidenceCache[key]
	suppressionConfidenceMu.RUnlock()

	if exists && score > 70 {
		return true, fmt.Sprintf("previously dismissed with %.0f%% confidence", score)
	}

	// Check global patterns via kernel query
	if r.kernel != nil {
		query := fmt.Sprintf("global_suppression_pattern(%q, Pattern)", string(hypo.Type))
		results, err := r.kernel.Query(query)
		if err == nil && len(results) > 0 {
			// Has a global suppression pattern for this type
			if len(results[0].Args) >= 2 {
				if pattern, ok := results[0].Args[1].(string); ok {
					return true, fmt.Sprintf("matches global pattern: %s", pattern)
				}
			}
		}
	}

	return false, ""
}

// ClearSuppressions removes all learned suppressions.
// Use with caution - this resets all autopoiesis learning.
func (r *ReviewerShard) ClearSuppressions() error {
	suppressionConfidenceMu.Lock()
	suppressionConfidenceCache = make(map[string]float64)
	suppressionConfidenceMu.Unlock()

	logging.Get(logging.CategoryReviewer).Info("Cleared all suppression learnings")
	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// toInt64 safely converts interface{} to int64.
func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case float64:
		return int64(val)
	default:
		return 0
	}
}

// toFloat64 safely converts interface{} to float64.
func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	case int:
		return float64(val)
	default:
		return 0.0
	}
}
