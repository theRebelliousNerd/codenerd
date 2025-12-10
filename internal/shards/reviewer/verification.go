package reviewer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"
)

// =============================================================================
// LLM VERIFICATION FLOW
// =============================================================================
// This file implements the LLM verification flow for Mangle-generated hypotheses.
// The neuro-symbolic pattern: Mangle (deterministic) generates hypotheses,
// LLM (semantic) verifies or dismisses them.

// Verdict represents the LLM's decision on a hypothesis.
type Verdict struct {
	HypothesisType  HypothesisType `json:"hypothesis_type"`
	File            string         `json:"file"`
	Line            int            `json:"line"`
	Decision        string         `json:"decision"` // "CONFIRMED" or "DISMISSED"
	Reasoning       string         `json:"reasoning"`
	Fix             string         `json:"fix,omitempty"`    // Only for CONFIRMED
	Confidence      float64        `json:"confidence"`       // LLM's confidence in verdict
	FalsePositive   bool           `json:"false_positive"`   // If DISMISSED, was this a false positive?
	PatternNote     string         `json:"pattern_note"`     // Learning note for autopoiesis
	AlternativeRisk string         `json:"alternative_risk"` // Did LLM find different issue?
}

// VerifiedFinding represents a confirmed hypothesis converted to a finding.
type VerifiedFinding struct {
	ReviewFinding
	LogicTrace string `json:"logic_trace"` // Preserved from hypothesis
	Reasoning  string `json:"reasoning"`   // From LLM verification
	SourceRule string `json:"source_rule"` // Mangle rule that generated hypothesis
}

// =============================================================================
// HYPOTHESIS VERIFICATION
// =============================================================================

// VerifyHypotheses sends hypotheses to LLM for verification.
// This implements the neuro-symbolic verification loop:
// Mangle (deterministic) -> hypotheses -> LLM (semantic) -> verified findings
func (r *ReviewerShard) VerifyHypotheses(ctx context.Context, hypos []Hypothesis, code map[string]string) ([]VerifiedFinding, error) {
	if len(hypos) == 0 {
		logging.ReviewerDebug("VerifyHypotheses: no hypotheses to verify")
		return nil, nil
	}

	logging.Reviewer("Verifying %d hypotheses through LLM", len(hypos))
	startTime := time.Now()

	var findings []VerifiedFinding
	var dismissedCount, confirmedCount int

	// Group hypotheses by file for efficient prompting
	byFile := GroupByFile(hypos)
	logging.ReviewerDebug("Hypotheses grouped into %d files", len(byFile))

	for file, fileHypos := range byFile {
		select {
		case <-ctx.Done():
			return findings, ctx.Err()
		default:
		}

		fileCode, ok := code[file]
		if !ok {
			logging.Get(logging.CategoryReviewer).Warn("No code available for file %s, skipping %d hypotheses", file, len(fileHypos))
			continue
		}

		logging.ReviewerDebug("Verifying %d hypotheses for file: %s", len(fileHypos), file)

		// Build verification prompt with full context
		prompt := r.buildVerificationPrompt(fileHypos, fileCode, file)

		// Get system prompt from template (or fallback)
		systemPrompt := r.buildVerificationSystemPrompt()

		// Call LLM with retry logic
		response, err := r.llmCompleteWithRetry(ctx, systemPrompt, prompt, 3)
		if err != nil {
			logging.Get(logging.CategoryReviewer).Warn("LLM verification failed for %s: %v", file, err)
			// Continue with other files rather than failing entirely
			continue
		}

		// Process through Piggyback Protocol
		surface, control, err := r.processLLMResponse(response)
		if err != nil {
			logging.Get(logging.CategoryReviewer).Warn("Response processing failed for %s: %v", file, err)
			surface = response
		}

		// Route any control packet data to kernel
		if control != nil {
			r.routeControlPacketToKernel(control)
		}

		// Parse verdicts from response
		verdicts, err := parseVerdicts(surface)
		if err != nil {
			logging.Get(logging.CategoryReviewer).Warn("Failed to parse verdicts for %s: %v", file, err)
			// Continue with other files
			continue
		}

		// Convert confirmed verdicts to findings and learn from dismissals
		for _, v := range verdicts {
			hypo := findMatchingHypothesis(fileHypos, v)
			if hypo == nil {
				logging.ReviewerDebug("No matching hypothesis for verdict at %s:%d", v.File, v.Line)
				continue
			}

			if v.Decision == "CONFIRMED" {
				finding := r.verdictToFinding(v, hypo)
				findings = append(findings, finding)
				confirmedCount++

				// Assert confirmed finding to kernel for downstream rules
				r.assertVerifiedFinding(hypo, v)
			} else {
				// Learn from dismissal for autopoiesis
				// Use existing LearnFromDismissal signature: (Hypothesis, string) error
				reason := buildDismissalReason(v)
				if err := r.LearnFromDismissal(*hypo, reason); err != nil {
					logging.ReviewerDebug("Failed to learn from dismissal: %v", err)
				}
				// Also track additional verdict metadata
				r.recordDismissalMetadata(hypo, v)
				dismissedCount++
			}
		}
	}

	duration := time.Since(startTime)
	logging.Reviewer("Verification complete: %d confirmed, %d dismissed (duration: %v)",
		confirmedCount, dismissedCount, duration)

	// Assert verification summary to kernel
	r.assertVerificationSummary(len(hypos), confirmedCount, dismissedCount, duration)

	return findings, nil
}

// =============================================================================
// PROMPT CONSTRUCTION
// =============================================================================

// verificationSystemPrompt is the legacy system prompt for hypothesis verification.
// DEPRECATED: Use buildVerificationSystemPrompt() which loads from verification.yaml with fallback.
const verificationSystemPrompt = `You are a principal engineer performing semantic verification of code analysis hypotheses.

Your task is to review each hypothesis and determine if it represents a REAL issue or a false positive.

DECISION CRITERIA:
- CONFIRMED: The hypothesis represents a real bug, security issue, or code quality problem
- DISMISSED: The hypothesis is a false positive, the code is actually correct/safe

For each hypothesis, you MUST provide:
1. Decision: CONFIRMED or DISMISSED
2. Reasoning: Clear explanation of why (cite specific code if possible)
3. Confidence: 0.0-1.0 how certain you are
4. Fix (if CONFIRMED): Specific code fix suggestion
5. Pattern note (if DISMISSED): What made this a false positive? (helps improve future detection)

IMPORTANT CONTEXT:
- You have full file context, not just the flagged line
- Consider package-level scope for Go (symbols in same package are accessible)
- Consider the flow of data through the function
- Look for compensating controls (error handling, nil checks elsewhere)
- Consider if the issue is theoretical vs. practically exploitable

OUTPUT FORMAT:
Return a JSON array of verdicts:
` + "```json" + `
[
  {
    "hypothesis_type": "...",
    "file": "...",
    "line": N,
    "decision": "CONFIRMED" or "DISMISSED",
    "reasoning": "...",
    "confidence": 0.0-1.0,
    "fix": "..." (only if CONFIRMED),
    "false_positive": true/false,
    "pattern_note": "..." (learning note),
    "alternative_risk": "..." (if you found different issue)
  }
]
` + "```" + `

Be thorough but fair. Don't confirm issues that aren't real, but don't dismiss genuine problems.`

// buildVerificationPrompt creates the prompt for hypothesis verification.
func (r *ReviewerShard) buildVerificationPrompt(hypos []Hypothesis, code string, filePath string) string {
	var sb strings.Builder

	// File context
	sb.WriteString(fmt.Sprintf("## File: %s\n\n", filePath))
	sb.WriteString("### Full Code Context:\n```")
	sb.WriteString(r.detectLanguage(filePath))
	sb.WriteString("\n")

	// Truncate very long files but preserve critical sections
	if len(code) > 15000 {
		code = truncatePreservingHypothesisLines(code, hypos, 15000)
		sb.WriteString("// Note: File truncated, hypothesis lines preserved\n")
	}
	sb.WriteString(code)
	sb.WriteString("\n```\n\n")

	// Hypotheses to verify
	sb.WriteString("### Hypotheses to Verify:\n\n")

	for i, h := range hypos {
		sb.WriteString(fmt.Sprintf("#### Hypothesis %d: %s\n", i+1, h.Type))
		sb.WriteString(fmt.Sprintf("- **Location**: Line %d\n", h.Line))

		if h.Variable != "" {
			sb.WriteString(fmt.Sprintf("- **Variable/Symbol**: `%s`\n", h.Variable))
		}
		if h.Message != "" {
			sb.WriteString(fmt.Sprintf("- **Issue**: %s\n", h.Message))
		}
		sb.WriteString(fmt.Sprintf("- **Category**: %s\n", h.Category))
		sb.WriteString(fmt.Sprintf("- **Detection Confidence**: %.2f\n", h.Confidence))
		sb.WriteString(fmt.Sprintf("- **Rule**: %s\n", h.RuleID))

		if h.LogicTrace != "" {
			sb.WriteString(fmt.Sprintf("- **Logic Trace**: %s\n", h.LogicTrace))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\nPlease verify each hypothesis and return your verdicts as JSON.\n")

	return sb.String()
}

// truncatePreservingHypothesisLines truncates code while keeping hypothesis lines visible.
func truncatePreservingHypothesisLines(code string, hypos []Hypothesis, maxLen int) string {
	lines := strings.Split(code, "\n")

	// Collect lines we must preserve (hypothesis lines +/- context)
	preserveLines := make(map[int]bool)
	for _, h := range hypos {
		// Preserve 5 lines before and after each hypothesis
		for i := h.Line - 6; i <= h.Line+5; i++ {
			if i >= 0 && i < len(lines) {
				preserveLines[i] = true
			}
		}
	}

	// Build truncated version
	var result strings.Builder
	inPreserved := false
	lastPreservedEnd := -10

	for i, line := range lines {
		if preserveLines[i] {
			if !inPreserved && i > lastPreservedEnd+1 {
				result.WriteString(fmt.Sprintf("// ... lines %d-%d omitted ...\n", lastPreservedEnd+2, i))
			}
			result.WriteString(line)
			result.WriteString("\n")
			inPreserved = true
		} else {
			if inPreserved {
				lastPreservedEnd = i - 1
				inPreserved = false
			}
		}

		if result.Len() > maxLen {
			result.WriteString("\n// ... remainder truncated ...\n")
			break
		}
	}

	return result.String()
}

// =============================================================================
// VERDICT PARSING
// =============================================================================

// parseVerdicts extracts structured verdicts from LLM response.
func parseVerdicts(response string) ([]Verdict, error) {
	// Try to extract JSON from the response
	jsonStr := extractJSONFromResponse(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in response")
	}

	var verdicts []Verdict
	if err := json.Unmarshal([]byte(jsonStr), &verdicts); err != nil {
		// Try parsing as single verdict
		var singleVerdict Verdict
		if singleErr := json.Unmarshal([]byte(jsonStr), &singleVerdict); singleErr == nil {
			return []Verdict{singleVerdict}, nil
		}
		return nil, fmt.Errorf("failed to parse verdicts JSON: %w", err)
	}

	// Validate and normalize verdicts
	for i := range verdicts {
		verdicts[i].Decision = strings.ToUpper(strings.TrimSpace(verdicts[i].Decision))
		if verdicts[i].Decision != "CONFIRMED" && verdicts[i].Decision != "DISMISSED" {
			logging.Get(logging.CategoryReviewer).Warn("Invalid verdict decision: %s, defaulting to DISMISSED", verdicts[i].Decision)
			verdicts[i].Decision = "DISMISSED"
		}

		// Clamp confidence
		if verdicts[i].Confidence < 0 {
			verdicts[i].Confidence = 0
		}
		if verdicts[i].Confidence > 1 {
			verdicts[i].Confidence = 1
		}
	}

	logging.ReviewerDebug("Parsed %d verdicts from LLM response", len(verdicts))
	return verdicts, nil
}

// extractJSONFromResponse extracts JSON array or object from mixed content.
func extractJSONFromResponse(response string) string {
	// Try markdown code block first
	if strings.Contains(response, "```json") {
		parts := strings.Split(response, "```json")
		if len(parts) > 1 {
			jsonPart := strings.Split(parts[1], "```")[0]
			return strings.TrimSpace(jsonPart)
		}
	}

	// Try generic code block
	if strings.Contains(response, "```") {
		parts := strings.Split(response, "```")
		if len(parts) > 1 {
			// Try each code block
			for i := 1; i < len(parts); i += 2 {
				candidate := strings.TrimSpace(parts[i])
				// Remove language identifier if present
				if idx := strings.Index(candidate, "\n"); idx != -1 {
					firstLine := candidate[:idx]
					if !strings.Contains(firstLine, "[") && !strings.Contains(firstLine, "{") {
						candidate = candidate[idx+1:]
					}
				}
				if strings.HasPrefix(candidate, "[") || strings.HasPrefix(candidate, "{") {
					return candidate
				}
			}
		}
	}

	// Try to find raw JSON
	start := strings.Index(response, "[")
	if start == -1 {
		start = strings.Index(response, "{")
	}
	if start == -1 {
		return ""
	}

	// Find matching end bracket
	depth := 0
	isArray := response[start] == '['
	for i := start; i < len(response); i++ {
		switch response[i] {
		case '[', '{':
			depth++
		case ']', '}':
			depth--
			if depth == 0 {
				// Verify it's the right type of bracket
				if (isArray && response[i] == ']') || (!isArray && response[i] == '}') {
					return response[start : i+1]
				}
			}
		}
	}

	return ""
}

// =============================================================================
// FINDING CONVERSION
// =============================================================================

// verdictToFinding converts a confirmed verdict to a VerifiedFinding.
func (r *ReviewerShard) verdictToFinding(v Verdict, hypo *Hypothesis) VerifiedFinding {
	// Determine severity from hypothesis type and LLM confidence
	severity := determineSeverity(hypo.Type, v.Confidence)

	return VerifiedFinding{
		ReviewFinding: ReviewFinding{
			File:       hypo.File,
			Line:       hypo.Line,
			Severity:   severity,
			Category:   hypo.Category,
			RuleID:     hypo.RuleID,
			Message:    v.Reasoning,
			Suggestion: v.Fix,
		},
		LogicTrace: hypo.LogicTrace,
		Reasoning:  v.Reasoning,
		SourceRule: hypo.RuleID,
	}
}

// determineSeverity maps hypothesis type and confidence to severity level.
func determineSeverity(ht HypothesisType, confidence float64) string {
	// Base severity from type
	var baseSeverity string
	switch ht {
	case HypothesisSQLInjection, HypothesisCommandInjection, HypothesisHardcodedSecret:
		baseSeverity = "critical"
	case HypothesisUnsafeDeref, HypothesisNilChannel, HypothesisXSS, HypothesisDeadlock:
		baseSeverity = "error"
	case HypothesisUncheckedError, HypothesisRaceCondition, HypothesisGoroutineLeak,
		HypothesisResourceLeak, HypothesisPathTraversal, HypothesisWeakCrypto:
		baseSeverity = "warning"
	default:
		baseSeverity = "info"
	}

	// Adjust based on confidence
	if confidence >= 0.9 && baseSeverity == "warning" {
		return "error"
	}
	if confidence < 0.6 && baseSeverity == "critical" {
		return "error" // Downgrade low-confidence criticals
	}

	return baseSeverity
}

// =============================================================================
// HYPOTHESIS MATCHING
// =============================================================================

// findMatchingHypothesis finds the hypothesis that matches a verdict.
func findMatchingHypothesis(hypos []Hypothesis, v Verdict) *Hypothesis {
	// Exact match on file and line
	for i := range hypos {
		if hypos[i].File == v.File && hypos[i].Line == v.Line {
			return &hypos[i]
		}
	}

	// Fuzzy match: same file, same type, within 3 lines
	for i := range hypos {
		if hypos[i].File == v.File &&
			hypos[i].Type == v.HypothesisType &&
			absInt(hypos[i].Line-v.Line) <= 3 {
			return &hypos[i]
		}
	}

	// Last resort: same file and type
	for i := range hypos {
		if hypos[i].File == v.File && hypos[i].Type == v.HypothesisType {
			logging.ReviewerDebug("Fuzzy match for verdict at line %d to hypothesis at line %d", v.Line, hypos[i].Line)
			return &hypos[i]
		}
	}

	return nil
}

// absInt returns absolute value of an integer.
func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// =============================================================================
// DISMISSAL HELPERS
// =============================================================================

// buildDismissalReason constructs a reason string from a Verdict for the
// existing LearnFromDismissal method signature.
func buildDismissalReason(v Verdict) string {
	var parts []string

	if v.Reasoning != "" {
		parts = append(parts, v.Reasoning)
	}
	if v.PatternNote != "" {
		parts = append(parts, fmt.Sprintf("Pattern: %s", v.PatternNote))
	}
	if v.FalsePositive {
		parts = append(parts, "Confirmed false positive")
	}

	if len(parts) == 0 {
		return "Dismissed by LLM verification"
	}
	return strings.Join(parts, "; ")
}

// recordDismissalMetadata records additional metadata from a verdict that
// doesn't fit into the standard LearnFromDismissal flow.
func (r *ReviewerShard) recordDismissalMetadata(hypo *Hypothesis, v Verdict) {
	if hypo == nil {
		return
	}

	// Record alternative risks discovered during verification
	if v.AlternativeRisk != "" {
		logging.Reviewer("Alternative risk found during verification: %s", v.AlternativeRisk)
		if r.kernel != nil {
			fact := core.Fact{
				Predicate: "alternative_risk_discovered",
				Args:      []interface{}{hypo.File, hypo.Line, v.AlternativeRisk},
			}
			_ = r.kernel.Assert(fact)
		}
	}

	// Track confidence delta for learning
	if r.kernel != nil && v.Confidence > 0 {
		confidenceDelta := hypo.Confidence - v.Confidence
		fact := core.Fact{
			Predicate: "hypothesis_dismissed",
			Args: []interface{}{
				hypo.RuleID,
				string(hypo.Type),
				v.Confidence,
				confidenceDelta,
				v.FalsePositive,
			},
		}
		_ = r.kernel.Assert(fact)
	}
}

// =============================================================================
// KERNEL INTEGRATION
// =============================================================================

// assertVerifiedFinding asserts a confirmed finding to the kernel.
func (r *ReviewerShard) assertVerifiedFinding(hypo *Hypothesis, verdict Verdict) {
	if r.kernel == nil {
		return
	}

	// verified_finding(File, Line, Type, Severity, Confidence, Reasoning)
	severity := determineSeverity(hypo.Type, verdict.Confidence)
	fact := core.Fact{
		Predicate: "verified_finding",
		Args: []interface{}{
			hypo.File,
			hypo.Line,
			string(hypo.Type),
			severity,
			verdict.Confidence,
			verdict.Reasoning,
		},
	}
	if err := r.kernel.Assert(fact); err != nil {
		logging.Get(logging.CategoryReviewer).Warn("Failed to assert verified_finding: %v", err)
	}

	// Track hypothesis confirmation for autopoiesis
	// hypothesis_confirmed(RuleID, Type, ConfidenceImprovement)
	confidenceDelta := verdict.Confidence - hypo.Confidence
	confirmFact := core.Fact{
		Predicate: "hypothesis_confirmed",
		Args:      []interface{}{hypo.RuleID, string(hypo.Type), confidenceDelta},
	}
	_ = r.kernel.Assert(confirmFact)
}

// assertVerificationSummary asserts summary statistics to the kernel.
func (r *ReviewerShard) assertVerificationSummary(total, confirmed, dismissed int, duration time.Duration) {
	if r.kernel == nil {
		return
	}

	// verification_summary(Timestamp, Total, Confirmed, Dismissed, DurationMs)
	fact := core.Fact{
		Predicate: "verification_summary",
		Args: []interface{}{
			time.Now().Unix(),
			total,
			confirmed,
			dismissed,
			duration.Milliseconds(),
		},
	}
	if err := r.kernel.Assert(fact); err != nil {
		logging.ReviewerDebug("Failed to assert verification_summary: %v", err)
	}

	// Calculate and track precision for learning
	if total > 0 {
		precision := float64(confirmed) / float64(total)
		precisionFact := core.Fact{
			Predicate: "verification_precision",
			Args:      []interface{}{time.Now().Unix(), precision},
		}
		_ = r.kernel.Assert(precisionFact)

		logging.ReviewerDebug("Verification precision: %.2f%% (%d/%d confirmed)",
			precision*100, confirmed, total)
	}
}

// =============================================================================
// BATCH VERIFICATION
// =============================================================================

// VerifyHypothesesBatch verifies hypotheses in batches to respect context limits.
func (r *ReviewerShard) VerifyHypothesesBatch(ctx context.Context, hypos []Hypothesis, code map[string]string, batchSize int) ([]VerifiedFinding, error) {
	if batchSize <= 0 {
		batchSize = 10 // Default batch size
	}

	var allFindings []VerifiedFinding

	// Process in batches
	for i := 0; i < len(hypos); i += batchSize {
		select {
		case <-ctx.Done():
			return allFindings, ctx.Err()
		default:
		}

		end := i + batchSize
		if end > len(hypos) {
			end = len(hypos)
		}

		batch := hypos[i:end]
		logging.ReviewerDebug("Processing hypothesis batch %d-%d of %d", i+1, end, len(hypos))

		findings, err := r.VerifyHypotheses(ctx, batch, code)
		if err != nil {
			logging.Get(logging.CategoryReviewer).Warn("Batch verification error: %v", err)
			// Continue with next batch
			continue
		}

		allFindings = append(allFindings, findings...)
	}

	return allFindings, nil
}

// =============================================================================
// VERIFICATION STATISTICS
// =============================================================================

// VerificationStats tracks verification performance metrics.
type VerificationStats struct {
	TotalHypotheses int           `json:"total_hypotheses"`
	Confirmed       int           `json:"confirmed"`
	Dismissed       int           `json:"dismissed"`
	Precision       float64       `json:"precision"` // confirmed / total
	AvgDuration     time.Duration `json:"avg_duration"`
	FalsePositives  int           `json:"false_positives"`
}

// GetVerificationStats returns current verification statistics.
func (r *ReviewerShard) GetVerificationStats() VerificationStats {
	if r.kernel == nil {
		return VerificationStats{}
	}

	stats := VerificationStats{}

	// Query verification_summary facts
	summaries, _ := r.kernel.Query("verification_summary")
	for _, s := range summaries {
		if len(s.Args) >= 5 {
			stats.TotalHypotheses += toStartInt(s.Args[1])
			stats.Confirmed += toStartInt(s.Args[2])
			stats.Dismissed += toStartInt(s.Args[3])
		}
	}

	if stats.TotalHypotheses > 0 {
		stats.Precision = float64(stats.Confirmed) / float64(stats.TotalHypotheses)
	}

	// Count false positive patterns
	fpPatterns, _ := r.kernel.Query("false_positive_pattern")
	stats.FalsePositives = len(fpPatterns)

	return stats
}

// =============================================================================
// CONVENIENCE FUNCTIONS
// =============================================================================

// VerifyAndReport generates hypotheses, verifies them, and returns findings.
// This is a high-level function combining hypothesis generation with verification.
func (r *ReviewerShard) VerifyAndReport(ctx context.Context, code map[string]string) ([]VerifiedFinding, error) {
	// Generate hypotheses from Mangle
	hypos, err := r.GenerateHypotheses(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate hypotheses: %w", err)
	}

	if len(hypos) == 0 {
		logging.ReviewerDebug("No hypotheses generated, skipping verification")
		return nil, nil
	}

	// Verify through LLM
	return r.VerifyHypotheses(ctx, hypos, code)
}

// ToFindings converts VerifiedFindings to standard ReviewFindings.
func ToFindings(verified []VerifiedFinding) []ReviewFinding {
	findings := make([]ReviewFinding, len(verified))
	for i, v := range verified {
		findings[i] = v.ReviewFinding
	}
	return findings
}
