// Package reviewer implements the Reviewer ShardAgent per Cortex 1.5.0 Section 7.0 Sharding.
// This file implements hypothesis generation from Mangle for LLM verification.
package reviewer

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/logging"
)

// =============================================================================
// HYPOTHESIS TYPES
// =============================================================================

// HypothesisType categorizes potential issues flagged by Mangle rules.
type HypothesisType string

const (
	HypothesisUnsafeDeref      HypothesisType = "unsafe_deref"
	HypothesisUncheckedError   HypothesisType = "unchecked_error"
	HypothesisSQLInjection     HypothesisType = "sql_injection"
	HypothesisCommandInjection HypothesisType = "command_injection"
	HypothesisPathTraversal    HypothesisType = "path_traversal"
	HypothesisRaceCondition    HypothesisType = "race_condition"
	HypothesisResourceLeak     HypothesisType = "resource_leak"
	HypothesisNilChannel       HypothesisType = "nil_channel"
	HypothesisGoroutineLeak    HypothesisType = "goroutine_leak"
	HypothesisDeadlock         HypothesisType = "deadlock"
	HypothesisUnboundedGrowth  HypothesisType = "unbounded_growth"
	HypothesisHardcodedSecret  HypothesisType = "hardcoded_secret"
	HypothesisWeakCrypto       HypothesisType = "weak_crypto"
	HypothesisXSS              HypothesisType = "xss"
	HypothesisGeneric          HypothesisType = "generic"
)

// Hypothesis represents a potential issue flagged by Mangle for LLM verification.
// The Logic-First architecture identifies candidates deterministically, then the
// LLM verifies whether they are true positives with semantic understanding.
type Hypothesis struct {
	Type       HypothesisType // "unsafe_deref", "unchecked_error", "sql_injection", etc.
	File       string         // Source file path
	Line       int            // Line number where issue was detected
	Variable   string         // Variable or symbol involved
	LogicTrace string         // Mangle derivation path for explainability
	Confidence float64        // 0.0-1.0 based on rule specificity
	Priority   int            // Derived from prioritized_hypothesis query
	Category   string         // High-level category: "security", "safety", "concurrency"
	Message    string         // Human-readable description
	RuleID     string         // Mangle rule that generated this hypothesis
}

// =============================================================================
// BASE CONFIDENCE VALUES
// =============================================================================

// baseConfidenceByType returns the base confidence score for a hypothesis type.
// These values reflect the historical accuracy of each rule category.
var baseConfidenceByType = map[HypothesisType]float64{
	HypothesisUnsafeDeref:      0.85,
	HypothesisUncheckedError:   0.75,
	HypothesisSQLInjection:     0.90,
	HypothesisCommandInjection: 0.90,
	HypothesisPathTraversal:    0.80,
	HypothesisRaceCondition:    0.70,
	HypothesisResourceLeak:     0.75,
	HypothesisNilChannel:       0.85,
	HypothesisGoroutineLeak:    0.80,
	HypothesisDeadlock:         0.65,
	HypothesisUnboundedGrowth:  0.70,
	HypothesisHardcodedSecret:  0.85,
	HypothesisWeakCrypto:       0.80,
	HypothesisXSS:              0.85,
	HypothesisGeneric:          0.50,
}

// categoryByType maps hypothesis types to their high-level categories.
var categoryByType = map[HypothesisType]string{
	HypothesisUnsafeDeref:      "safety",
	HypothesisUncheckedError:   "safety",
	HypothesisSQLInjection:     "security",
	HypothesisCommandInjection: "security",
	HypothesisPathTraversal:    "security",
	HypothesisRaceCondition:    "concurrency",
	HypothesisResourceLeak:     "resource",
	HypothesisNilChannel:       "concurrency",
	HypothesisGoroutineLeak:    "concurrency",
	HypothesisDeadlock:         "concurrency",
	HypothesisUnboundedGrowth:  "resource",
	HypothesisHardcodedSecret:  "security",
	HypothesisWeakCrypto:       "security",
	HypothesisXSS:              "security",
	HypothesisGeneric:          "general",
}

// =============================================================================
// HYPOTHESIS GENERATION
// =============================================================================

// GenerateHypotheses queries Mangle for all potential issues that need LLM verification.
// This implements the Logic-First pattern: deterministic rules identify candidates,
// then the LLM performs semantic verification to eliminate false positives.
func (r *ReviewerShard) GenerateHypotheses(ctx context.Context) ([]Hypothesis, error) {
	logging.ReviewerDebug("GenerateHypotheses: starting hypothesis generation from Mangle")

	if r.kernel == nil {
		logging.Get(logging.CategoryReviewer).Warn("GenerateHypotheses: kernel not initialized")
		return nil, fmt.Errorf("kernel not initialized")
	}

	var hypotheses []Hypothesis

	// Query for prioritized hypotheses (includes suppression filtering)
	// Format: prioritized_hypothesis(Type, File, Line, Var, Priority)
	results, err := r.kernel.Query("prioritized_hypothesis")
	if err != nil {
		logging.ReviewerDebug("GenerateHypotheses: prioritized_hypothesis query failed, falling back to active_hypothesis: %v", err)
		// Fall back to active_hypothesis if prioritization not available
		results, err = r.kernel.Query("active_hypothesis")
		if err != nil {
			logging.Get(logging.CategoryReviewer).Error("GenerateHypotheses: failed to query hypotheses: %v", err)
			return nil, fmt.Errorf("failed to query hypotheses: %w", err)
		}
	}

	logging.ReviewerDebug("GenerateHypotheses: query returned %d raw results", len(results))

	for _, result := range results {
		hypo, parseErr := parseHypothesisFromResult(result)
		if parseErr != nil {
			logging.ReviewerDebug("GenerateHypotheses: skipping malformed result: %v", parseErr)
			continue
		}

		hypo.LogicTrace = r.getLogicTrace(hypo)
		hypo.Confidence = r.calculateConfidence(hypo)
		hypo.Category = getCategory(hypo.Type)
		hypo.Message = generateMessage(hypo)

		hypotheses = append(hypotheses, hypo)
	}

	// Sort by priority (highest first), then by confidence
	sort.Slice(hypotheses, func(i, j int) bool {
		if hypotheses[i].Priority != hypotheses[j].Priority {
			return hypotheses[i].Priority > hypotheses[j].Priority
		}
		return hypotheses[i].Confidence > hypotheses[j].Confidence
	})

	logging.Reviewer("GenerateHypotheses: generated %d hypotheses from Mangle", len(hypotheses))
	return hypotheses, nil
}

// parseHypothesisFromResult converts a Mangle query result to a Hypothesis struct.
// Expected formats:
//   - prioritized_hypothesis(Type, File, Line, Var, Priority)
//   - active_hypothesis(Type, File, Line, Var)
func parseHypothesisFromResult(result core.Fact) (Hypothesis, error) {
	hypo := Hypothesis{}

	if len(result.Args) < 4 {
		return hypo, fmt.Errorf("insufficient arguments: expected at least 4, got %d", len(result.Args))
	}

	// Parse Type (arg 0)
	typeStr := extractString(result.Args[0])
	hypo.Type = normalizeHypothesisType(typeStr)
	hypo.RuleID = typeStr

	// Parse File (arg 1)
	hypo.File = extractString(result.Args[1])

	// Parse Line (arg 2)
	hypo.Line = extractInt(result.Args[2])

	// Parse Variable (arg 3)
	hypo.Variable = extractString(result.Args[3])

	// Parse Priority if present (arg 4)
	if len(result.Args) >= 5 {
		hypo.Priority = extractInt(result.Args[4])
	} else {
		// Default priority based on type
		hypo.Priority = defaultPriorityForType(hypo.Type)
	}

	return hypo, nil
}

// normalizeHypothesisType converts a Mangle atom string to a HypothesisType.
func normalizeHypothesisType(typeStr string) HypothesisType {
	// Strip leading "/" if present (Mangle atom format)
	normalized := strings.TrimPrefix(typeStr, "/")
	normalized = strings.ToLower(normalized)

	switch normalized {
	case "unsafe_deref", "null_deref", "nil_deref":
		return HypothesisUnsafeDeref
	case "unchecked_error", "ignored_error", "error_ignored":
		return HypothesisUncheckedError
	case "sql_injection", "sqli":
		return HypothesisSQLInjection
	case "command_injection", "cmd_injection", "os_command":
		return HypothesisCommandInjection
	case "path_traversal", "directory_traversal":
		return HypothesisPathTraversal
	case "race_condition", "data_race", "race":
		return HypothesisRaceCondition
	case "resource_leak", "leak":
		return HypothesisResourceLeak
	case "nil_channel", "nil_chan":
		return HypothesisNilChannel
	case "goroutine_leak", "goroutine_orphan":
		return HypothesisGoroutineLeak
	case "deadlock":
		return HypothesisDeadlock
	case "unbounded_growth", "memory_growth":
		return HypothesisUnboundedGrowth
	case "hardcoded_secret", "secret", "credential":
		return HypothesisHardcodedSecret
	case "weak_crypto", "insecure_crypto":
		return HypothesisWeakCrypto
	case "xss", "cross_site_scripting":
		return HypothesisXSS
	default:
		return HypothesisGeneric
	}
}

// defaultPriorityForType returns a default priority for hypothesis types
// when not provided by the Mangle query.
func defaultPriorityForType(t HypothesisType) int {
	switch t {
	case HypothesisSQLInjection, HypothesisCommandInjection:
		return 100 // Critical security
	case HypothesisHardcodedSecret, HypothesisXSS:
		return 90
	case HypothesisUnsafeDeref, HypothesisNilChannel:
		return 80
	case HypothesisRaceCondition, HypothesisDeadlock:
		return 75
	case HypothesisGoroutineLeak, HypothesisResourceLeak:
		return 70
	case HypothesisUncheckedError:
		return 60
	case HypothesisPathTraversal, HypothesisWeakCrypto:
		return 65
	default:
		return 50
	}
}

// =============================================================================
// LOGIC TRACE CONSTRUCTION
// =============================================================================

// getLogicTrace builds a human-readable derivation path explaining how Mangle
// arrived at this hypothesis. This provides explainability for the LLM reviewer.
func (r *ReviewerShard) getLogicTrace(hypo Hypothesis) string {
	var trace strings.Builder

	// Base pattern description
	switch hypo.Type {
	case HypothesisUnsafeDeref:
		trace.WriteString(fmt.Sprintf("assigns(%s, /nullable) + uses(%s) + !is_guarded(%s)",
			hypo.Variable, hypo.Variable, hypo.Variable))
	case HypothesisUncheckedError:
		trace.WriteString(fmt.Sprintf("returns_error(%s) + !error_checked(%s)",
			hypo.Variable, hypo.Variable))
	case HypothesisSQLInjection:
		trace.WriteString(fmt.Sprintf("tainted_source(%s) + flows_to(%s, /sql_sink) + !sanitized(%s)",
			hypo.Variable, hypo.Variable, hypo.Variable))
	case HypothesisCommandInjection:
		trace.WriteString(fmt.Sprintf("user_input(%s) + flows_to(%s, /exec_sink) + !escaped(%s)",
			hypo.Variable, hypo.Variable, hypo.Variable))
	case HypothesisRaceCondition:
		trace.WriteString(fmt.Sprintf("shared_var(%s) + concurrent_access(%s) + !synchronized(%s)",
			hypo.Variable, hypo.Variable, hypo.Variable))
	case HypothesisGoroutineLeak:
		trace.WriteString(fmt.Sprintf("goroutine_spawn(%s) + !has_termination_path(%s)",
			hypo.Variable, hypo.Variable))
	case HypothesisNilChannel:
		trace.WriteString(fmt.Sprintf("channel_var(%s) + !initialized(%s) + channel_op(%s)",
			hypo.Variable, hypo.Variable, hypo.Variable))
	case HypothesisResourceLeak:
		trace.WriteString(fmt.Sprintf("resource_acquired(%s) + !closed_on_all_paths(%s)",
			hypo.Variable, hypo.Variable))
	case HypothesisHardcodedSecret:
		trace.WriteString(fmt.Sprintf("literal_match(%s, /secret_pattern)",
			hypo.Variable))
	default:
		trace.WriteString(fmt.Sprintf("hypothesis(%s, %s, %d, %s)",
			hypo.Type, hypo.File, hypo.Line, hypo.Variable))
	}

	// Add file/line context
	trace.WriteString(fmt.Sprintf(" @ %s:%d", hypo.File, hypo.Line))

	return trace.String()
}

// =============================================================================
// CONFIDENCE CALCULATION
// =============================================================================

// calculateConfidence determines confidence based on rule type and context.
// Adjusts base confidence by:
// - Specificity of the match
// - Number of supporting facts
// - Historical accuracy (if available from autopoiesis)
func (r *ReviewerShard) calculateConfidence(hypo Hypothesis) float64 {
	// Start with base confidence for this type
	confidence, exists := baseConfidenceByType[hypo.Type]
	if !exists {
		confidence = 0.50 // Default for unknown types
	}

	// Adjustment factors
	adjustments := 0.0

	// Boost confidence if variable name suggests the issue
	if suggestsIssue(hypo.Variable, hypo.Type) {
		adjustments += 0.05
	}

	// Reduce confidence for generic/common variable names
	if isGenericVariableName(hypo.Variable) {
		adjustments -= 0.05
	}

	// Boost for test files (more likely to be intentional edge cases)
	if strings.HasSuffix(hypo.File, "_test.go") {
		adjustments -= 0.10 // Lower priority for test files
	}

	// Check historical accuracy from autopoiesis patterns
	r.mu.RLock()
	patternKey := fmt.Sprintf("%s:%s", hypo.Type, hypo.Variable)
	if count, ok := r.flaggedPatterns[patternKey]; ok && count >= 3 {
		// This pattern has been flagged multiple times - boost confidence
		adjustments += 0.10
	}
	if count, ok := r.approvedPatterns[patternKey]; ok && count >= 3 {
		// This pattern has been approved multiple times - reduce confidence
		adjustments -= 0.15
	}
	r.mu.RUnlock()

	// Apply adjustments with bounds
	confidence += adjustments
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.1 {
		confidence = 0.1
	}

	return confidence
}

// suggestsIssue checks if the variable name suggests the hypothesized issue.
func suggestsIssue(variable string, t HypothesisType) bool {
	lower := strings.ToLower(variable)

	switch t {
	case HypothesisUnsafeDeref:
		return strings.Contains(lower, "nil") || strings.Contains(lower, "null") ||
			strings.Contains(lower, "optional") || strings.Contains(lower, "maybe")
	case HypothesisUncheckedError:
		return strings.Contains(lower, "err") || strings.Contains(lower, "_")
	case HypothesisSQLInjection:
		return strings.Contains(lower, "query") || strings.Contains(lower, "sql") ||
			strings.Contains(lower, "input") || strings.Contains(lower, "param")
	case HypothesisCommandInjection:
		return strings.Contains(lower, "cmd") || strings.Contains(lower, "command") ||
			strings.Contains(lower, "exec") || strings.Contains(lower, "shell")
	case HypothesisHardcodedSecret:
		return strings.Contains(lower, "password") || strings.Contains(lower, "secret") ||
			strings.Contains(lower, "token") || strings.Contains(lower, "key") ||
			strings.Contains(lower, "credential")
	default:
		return false
	}
}

// isGenericVariableName checks if a variable name is too generic to be meaningful.
func isGenericVariableName(variable string) bool {
	genericNames := []string{"x", "y", "z", "i", "j", "k", "v", "val", "value", "tmp", "temp", "data", "result", "ret"}
	lower := strings.ToLower(variable)
	for _, generic := range genericNames {
		if lower == generic {
			return true
		}
	}
	return false
}

// getCategory returns the high-level category for a hypothesis type.
func getCategory(t HypothesisType) string {
	if cat, ok := categoryByType[t]; ok {
		return cat
	}
	return "general"
}

// generateMessage creates a human-readable message for the hypothesis.
func generateMessage(hypo Hypothesis) string {
	switch hypo.Type {
	case HypothesisUnsafeDeref:
		return fmt.Sprintf("Potential nil/null dereference of '%s'", hypo.Variable)
	case HypothesisUncheckedError:
		return fmt.Sprintf("Error from '%s' may not be handled", hypo.Variable)
	case HypothesisSQLInjection:
		return fmt.Sprintf("Potential SQL injection via '%s'", hypo.Variable)
	case HypothesisCommandInjection:
		return fmt.Sprintf("Potential command injection via '%s'", hypo.Variable)
	case HypothesisPathTraversal:
		return fmt.Sprintf("Potential path traversal via '%s'", hypo.Variable)
	case HypothesisRaceCondition:
		return fmt.Sprintf("Potential race condition on '%s'", hypo.Variable)
	case HypothesisResourceLeak:
		return fmt.Sprintf("Resource '%s' may not be closed on all paths", hypo.Variable)
	case HypothesisNilChannel:
		return fmt.Sprintf("Channel '%s' may be nil (blocks forever)", hypo.Variable)
	case HypothesisGoroutineLeak:
		return fmt.Sprintf("Goroutine '%s' may leak (no termination path)", hypo.Variable)
	case HypothesisDeadlock:
		return fmt.Sprintf("Potential deadlock involving '%s'", hypo.Variable)
	case HypothesisUnboundedGrowth:
		return fmt.Sprintf("Unbounded growth detected in '%s'", hypo.Variable)
	case HypothesisHardcodedSecret:
		return fmt.Sprintf("Hardcoded secret detected in '%s'", hypo.Variable)
	case HypothesisWeakCrypto:
		return fmt.Sprintf("Weak cryptographic algorithm used: '%s'", hypo.Variable)
	case HypothesisXSS:
		return fmt.Sprintf("Potential XSS vulnerability via '%s'", hypo.Variable)
	default:
		return fmt.Sprintf("Potential issue with '%s'", hypo.Variable)
	}
}

// =============================================================================
// GROUPING UTILITIES
// =============================================================================

// GroupByFile groups hypotheses by file for efficient LLM prompting.
// This allows the LLM to review all issues in a file together with full context.
func GroupByFile(hypos []Hypothesis) map[string][]Hypothesis {
	result := make(map[string][]Hypothesis)
	for _, h := range hypos {
		result[h.File] = append(result[h.File], h)
	}
	return result
}

// GroupByCategory groups hypotheses by category for organized reporting.
func GroupByCategory(hypos []Hypothesis) map[string][]Hypothesis {
	result := make(map[string][]Hypothesis)
	for _, h := range hypos {
		result[h.Category] = append(result[h.Category], h)
	}
	return result
}

// GroupByType groups hypotheses by type for statistical analysis.
func GroupByType(hypos []Hypothesis) map[HypothesisType][]Hypothesis {
	result := make(map[HypothesisType][]Hypothesis)
	for _, h := range hypos {
		result[h.Type] = append(result[h.Type], h)
	}
	return result
}

// FilterByMinConfidence returns only hypotheses above a confidence threshold.
func FilterByMinConfidence(hypos []Hypothesis, minConfidence float64) []Hypothesis {
	result := make([]Hypothesis, 0, len(hypos))
	for _, h := range hypos {
		if h.Confidence >= minConfidence {
			result = append(result, h)
		}
	}
	return result
}

// FilterByCategory returns only hypotheses matching the specified category.
func FilterByCategory(hypos []Hypothesis, category string) []Hypothesis {
	result := make([]Hypothesis, 0)
	for _, h := range hypos {
		if h.Category == category {
			result = append(result, h)
		}
	}
	return result
}

// TopN returns the top N hypotheses by priority and confidence.
func TopN(hypos []Hypothesis, n int) []Hypothesis {
	if n <= 0 || len(hypos) == 0 {
		return nil
	}
	if n >= len(hypos) {
		return hypos
	}

	// Already sorted by GenerateHypotheses, just take first N
	result := make([]Hypothesis, n)
	copy(result, hypos[:n])
	return result
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// extractString safely extracts a string from an interface{} value.
func extractString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return strings.TrimPrefix(val, "/") // Strip Mangle atom prefix
	case core.MangleAtom:
		return strings.TrimPrefix(string(val), "/")
	default:
		return fmt.Sprintf("%v", v)
	}
}

// extractInt safely extracts an int from an interface{} value.
func extractInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	default:
		return 0
	}
}

// =============================================================================
// HYPOTHESIS TO FINDING CONVERSION
// =============================================================================

// ToReviewFinding converts a verified hypothesis to a ReviewFinding.
// Called after LLM verification confirms the hypothesis is a true positive.
func (h Hypothesis) ToReviewFinding() ReviewFinding {
	severity := "warning"
	switch h.Type {
	case HypothesisSQLInjection, HypothesisCommandInjection, HypothesisHardcodedSecret:
		severity = "critical"
	case HypothesisUnsafeDeref, HypothesisNilChannel, HypothesisXSS:
		severity = "error"
	case HypothesisUncheckedError, HypothesisRaceCondition, HypothesisGoroutineLeak:
		severity = "warning"
	}

	return ReviewFinding{
		File:     h.File,
		Line:     h.Line,
		Severity: severity,
		Category: h.Category,
		RuleID:   h.RuleID,
		Message:  h.Message,
	}
}

// HypothesesToFindings converts a slice of verified hypotheses to findings.
func HypothesesToFindings(hypos []Hypothesis) []ReviewFinding {
	findings := make([]ReviewFinding, 0, len(hypos))
	for _, h := range hypos {
		findings = append(findings, h.ToReviewFinding())
	}
	return findings
}
