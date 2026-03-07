package autopoiesis

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// =============================================================================
// FEEDBACK AND LEARNING WRAPPERS
// =============================================================================
// These methods expose the feedback loop for tool execution evaluation and improvement.

// RecordExecution records a tool execution and evaluates its quality.
// It also syncs learning facts to the kernel for logic-driven refinement triggers.
func (o *Orchestrator) RecordExecution(ctx context.Context, feedback *ExecutionFeedback) {
	if o == nil || feedback == nil || strings.TrimSpace(feedback.ToolName) == "" {
		return
	}

	if feedback.Timestamp.IsZero() {
		feedback.Timestamp = time.Now()
	}

	// Evaluate quality
	if feedback.Quality == nil && o.evaluator != nil {
		feedback.Quality = o.evaluator.Evaluate(ctx, feedback)
	}

	// Record in pattern detector
	if o.patterns != nil {
		o.patterns.RecordExecution(*feedback)
	}

	// Get patterns for this tool
	var patterns []*DetectedPattern
	if o.patterns != nil {
		patterns = o.patterns.GetToolPatterns(feedback.ToolName)
	}

	// Update learning store
	if o.learnings != nil {
		o.learnings.RecordLearning(feedback.ToolName, feedback, patterns)
	}

	// Wire to kernel: Assert learning facts for logic-driven refinement
	var learning *ToolLearning
	if o.learnings != nil {
		learning = o.learnings.GetLearning(feedback.ToolName)
	}
	if learning != nil {
		o.assertToolLearning(
			learning.ToolName,
			learning.TotalExecutions,
			learning.SuccessRate,
			learning.AverageQuality,
		)

		// Assert known issues for pattern detection
		for _, issue := range learning.KnownIssues {
			o.assertToolKnownIssue(learning.ToolName, string(issue))
		}
	}

	// Keep the active generation loop hot with the latest learnings instead of
	// waiting for the next explicit refresh call.
	o.RefreshLearningsContext()
}

// EvaluateToolQuality assesses the quality of a tool execution
func (o *Orchestrator) EvaluateToolQuality(ctx context.Context, feedback *ExecutionFeedback) *QualityAssessment {
	return o.evaluator.Evaluate(ctx, feedback)
}

// EvaluateToolQualityWithLLM uses LLM for deeper quality assessment
func (o *Orchestrator) EvaluateToolQualityWithLLM(ctx context.Context, feedback *ExecutionFeedback) (*QualityAssessment, error) {
	return o.evaluator.EvaluateWithLLM(ctx, feedback)
}

// GetToolPatterns returns detected issues patterns for a tool
func (o *Orchestrator) GetToolPatterns(toolName string) []*DetectedPattern {
	return o.patterns.GetToolPatterns(toolName)
}

// GetAllPatterns returns all detected patterns above confidence threshold
func (o *Orchestrator) GetAllPatterns(minConfidence float64) []*DetectedPattern {
	return o.patterns.GetPatterns(minConfidence)
}

// ShouldRefineTool checks if a tool needs improvement based on learnings
func (o *Orchestrator) ShouldRefineTool(toolName string) (bool, []ImprovementSuggestion) {
	learning := o.learnings.GetLearning(toolName)
	if learning == nil {
		return false, nil
	}

	// Check if quality is poor
	if learning.AverageQuality < 0.5 && learning.TotalExecutions >= 3 {
		patterns := o.patterns.GetToolPatterns(toolName)
		suggestions := []ImprovementSuggestion{}
		for _, p := range patterns {
			suggestions = append(suggestions, p.Suggestions...)
		}
		return true, suggestions
	}

	// Check for known issues that are fixable
	if len(learning.KnownIssues) > 0 {
		patterns := o.patterns.GetToolPatterns(toolName)
		for _, p := range patterns {
			if p.Confidence > 0.7 && len(p.Suggestions) > 0 {
				return true, p.Suggestions
			}
		}
	}

	return false, nil
}

// RefineTool generates an improved version of a tool
func (o *Orchestrator) RefineTool(ctx context.Context, toolName string, originalCode string) (*RefinementResult, error) {
	// Gather feedback history
	patterns := o.patterns.GetToolPatterns(toolName)
	recentFeedback := o.getRecentExecutionsForTool(toolName, 5)

	// Collect all suggestions
	suggestions := []ImprovementSuggestion{}
	for _, p := range patterns {
		suggestions = append(suggestions, p.Suggestions...)
	}

	req := RefinementRequest{
		ToolName:     toolName,
		OriginalCode: originalCode,
		Feedback:     recentFeedback,
		Patterns:     patterns,
		Suggestions:  suggestions,
	}

	return o.refiner.Refine(ctx, req)
}

func (o *Orchestrator) getRecentExecutionsForTool(toolName string, limit int) []ExecutionFeedback {
	if o == nil || o.patterns == nil || strings.TrimSpace(toolName) == "" {
		return nil
	}
	if limit <= 0 {
		limit = 5
	}

	o.patterns.mu.RLock()
	defer o.patterns.mu.RUnlock()

	if len(o.patterns.history) == 0 {
		return nil
	}

	results := make([]ExecutionFeedback, 0, limit)
	for i := len(o.patterns.history) - 1; i >= 0; i-- {
		fb := o.patterns.history[i]
		if fb.ToolName != toolName {
			continue
		}
		results = append(results, fb)
		if len(results) == limit {
			break
		}
	}

	return results
}

// GetToolLearning retrieves accumulated learnings for a tool
func (o *Orchestrator) GetToolLearning(toolName string) *ToolLearning {
	return o.learnings.GetLearning(toolName)
}

// GetAllLearnings returns all accumulated tool learnings
func (o *Orchestrator) GetAllLearnings() []*ToolLearning {
	return o.learnings.GetAllLearnings()
}

// AggregateLearningsForPrompt converts learnings into a prompt-friendly format.
// This extracts actionable patterns from past tool generation to inject into
// future generation prompts, enabling cross-tool learning.
func (o *Orchestrator) AggregateLearningsForPrompt() string {
	learnings := o.learnings.GetAllLearnings()
	if len(learnings) == 0 {
		return ""
	}

	var sb strings.Builder

	commonIssues := make(map[IssueType]int)
	antiPatterns := make(map[string]int)

	for _, l := range learnings {
		for _, issue := range l.KnownIssues {
			commonIssues[issue]++
		}
		for _, ap := range l.AntiPatterns {
			if ap != "" {
				antiPatterns[ap]++
			}
		}
	}

	sb.WriteString(fmt.Sprintf("Based on %d previous tool generations:\n", len(learnings)))

	// Top issues to avoid (Top-K to limit prompt growth)
	if len(commonIssues) > 0 {
		type issueCount struct {
			issue IssueType
			count int
		}
		issueCounts := make([]issueCount, 0, len(commonIssues))
		for issue, count := range commonIssues {
			if count >= 2 {
				issueCounts = append(issueCounts, issueCount{issue: issue, count: count})
			}
		}
		sort.Slice(issueCounts, func(i, j int) bool {
			return issueCounts[i].count > issueCounts[j].count
		})
		if len(issueCounts) > 0 {
			sb.WriteString("\nTOP COMMON ISSUES TO AVOID:\n")
			limit := 5
			if len(issueCounts) < limit {
				limit = len(issueCounts)
			}
			for i := 0; i < limit; i++ {
				sb.WriteString(fmt.Sprintf("- %s (occurred %d times)\n", issueCounts[i].issue, issueCounts[i].count))
			}
		}
	}

	// Top anti-patterns (Top-K to limit prompt growth)
	if len(antiPatterns) > 0 {
		type antiPatternCount struct {
			name  string
			count int
		}
		apCounts := make([]antiPatternCount, 0, len(antiPatterns))
		for ap, count := range antiPatterns {
			apCounts = append(apCounts, antiPatternCount{name: ap, count: count})
		}
		sort.Slice(apCounts, func(i, j int) bool {
			return apCounts[i].count > apCounts[j].count
		})
		sb.WriteString("\nTOP ANTI-PATTERNS DETECTED:\n")
		limit := 5
		if len(apCounts) < limit {
			limit = len(apCounts)
		}
		for i := 0; i < limit; i++ {
			sb.WriteString(fmt.Sprintf("- %s\n", apCounts[i].name))
		}
	}

	sb.WriteString("\nBEST PRACTICES:\n")
	sb.WriteString("- Always check context.Done() for cancellation\n")
	sb.WriteString("- Handle empty/nil inputs gracefully\n")
	sb.WriteString("- Don't import unused packages\n")
	sb.WriteString("- Return descriptive errors\n")

	// Tool-specific learnings for the worst quality tools (Top-K)
	type lowQualityTool struct {
		name    string
		quality float64
		issues  []IssueType
	}
	badTools := make([]lowQualityTool, 0)
	for _, l := range learnings {
		if l.AverageQuality < 0.5 && l.TotalExecutions >= 2 {
			badTools = append(badTools, lowQualityTool{name: l.ToolName, quality: l.AverageQuality, issues: l.KnownIssues})
		}
	}
	sort.Slice(badTools, func(i, j int) bool {
		return badTools[i].quality < badTools[j].quality
	})
	toolLimit := 3
	if len(badTools) < toolLimit {
		toolLimit = len(badTools)
	}
	for i := 0; i < toolLimit; i++ {
		sb.WriteString(fmt.Sprintf("\nTool '%s' had issues (quality=%.1f):\n", badTools[i].name, badTools[i].quality))
		for _, issue := range badTools[i].issues {
			sb.WriteString(fmt.Sprintf("  - %s\n", issue))
		}
	}

	result := sb.String()
	runes := []rune(result)
	if len(runes) > 2000 {
		result = string(runes[:2000]) + "...[truncated]"
	}

	return result
}

// RefreshLearningsContext updates the ToolGenerator with latest learnings.
// Call this before tool generation to inject accumulated knowledge.
func (o *Orchestrator) RefreshLearningsContext() {
	ctx := o.AggregateLearningsForPrompt()
	o.toolGen.SetLearningsContext(ctx)
	if o.ouroboros != nil {
		o.ouroboros.SetLearningsContext(ctx)
	}
}

// GenerateLearningFacts creates Mangle facts from all learnings
func (o *Orchestrator) GenerateLearningFacts() []string {
	return o.learnings.GenerateMangleFacts()
}

// ExecuteAndEvaluate runs a tool and automatically evaluates quality
func (o *Orchestrator) ExecuteAndEvaluate(ctx context.Context, toolName string, input string) (string, *QualityAssessment, error) {
	start := time.Now()

	output, err := o.ouroboros.ExecuteTool(ctx, toolName, input)

	feedback := &ExecutionFeedback{
		ToolName:   toolName,
		Timestamp:  start,
		Input:      input,
		Output:     output,
		OutputSize: len(output),
		Duration:   time.Since(start),
		Success:    err == nil,
	}

	if err != nil {
		feedback.ErrorMsg = err.Error()
	}

	// Evaluate and record
	o.RecordExecution(ctx, feedback)

	return output, feedback.Quality, err
}

// =============================================================================
// REASONING TRACE WRAPPERS
// =============================================================================
// These methods expose the trace system for capturing tool generation reasoning.

// StartToolTrace begins capturing reasoning for a tool generation
func (o *Orchestrator) StartToolTrace(toolName string, need *ToolNeed, userRequest string) *ReasoningTrace {
	return o.traces.StartTrace(toolName, need, userRequest)
}

// RecordTracePrompt records the prompts sent to the LLM
func (o *Orchestrator) RecordTracePrompt(trace *ReasoningTrace, systemPrompt, userPrompt string) {
	o.traces.RecordPrompt(trace, systemPrompt, userPrompt)
}

// RecordTraceResponse records the LLM response and extracts reasoning
func (o *Orchestrator) RecordTraceResponse(ctx context.Context, trace *ReasoningTrace, response string, tokensUsed int, duration time.Duration) {
	o.traces.RecordResponse(ctx, trace, response, tokensUsed, duration)
}

// FinalizeTrace marks a generation trace as complete
func (o *Orchestrator) FinalizeTrace(trace *ReasoningTrace, success bool, code string, failureReason string) {
	o.traces.FinalizeTrace(trace, success, code, failureReason)
}

// UpdateTraceWithFeedback adds execution feedback to a tool's trace
func (o *Orchestrator) UpdateTraceWithFeedback(toolName string, quality float64, issues []string, notes []string) {
	o.traces.UpdateWithFeedback(toolName, quality, issues, notes)
}

// GetToolTraces retrieves all reasoning traces for a tool
func (o *Orchestrator) GetToolTraces(toolName string) []*ReasoningTrace {
	return o.traces.GetToolTraces(toolName)
}

// GetAllTraces returns all reasoning traces
func (o *Orchestrator) GetAllTraces() []*ReasoningTrace {
	return o.traces.GetAllTraces()
}

// AnalyzeGenerations performs broad analysis across all tool generations
func (o *Orchestrator) AnalyzeGenerations(ctx context.Context) (*GenerationAudit, error) {
	return o.traces.AnalyzeGenerations(ctx)
}

// =============================================================================
// LOGGING INJECTION WRAPPERS
// =============================================================================
// These methods expose the logging injection system for mandatory tool logging.

// InjectLogging adds mandatory logging to generated tool code
func (o *Orchestrator) InjectLogging(code string, toolName string) (string, error) {
	return o.logInjector.InjectLogging(code, toolName)
}

// ValidateLogging checks that required logging is present in tool code
func (o *Orchestrator) ValidateLogging(code string) *LoggingValidation {
	return o.logInjector.ValidateLogging(code)
}

// GenerateToolWithTracing generates a tool with full reasoning trace capture
func (o *Orchestrator) GenerateToolWithTracing(ctx context.Context, need *ToolNeed, userRequest string) (*GeneratedTool, *ReasoningTrace, error) {
	// Start trace
	trace := o.StartToolTrace(need.Name, need, userRequest)

	// Generate tool (the toolgen will populate trace details)
	tool, err := o.toolGen.GenerateTool(ctx, need)
	if err != nil {
		o.FinalizeTrace(trace, false, "", err.Error())
		return nil, trace, err
	}

	// Inject mandatory logging into generated code
	loggedCode, logErr := o.InjectLogging(tool.Code, tool.Name)
	if logErr == nil {
		tool.Code = loggedCode
	}

	// Validate logging
	validation := o.ValidateLogging(tool.Code)
	if !validation.Valid {
		trace.PostExecutionNotes = append(trace.PostExecutionNotes,
			fmt.Sprintf("Logging validation failed: missing %v", validation.Missing))
	}

	// Finalize trace
	o.FinalizeTrace(trace, true, tool.Code, "")

	return tool, trace, nil
}

// ExecuteOuroborosLoopWithTracing runs the full loop with reasoning trace capture
func (o *Orchestrator) ExecuteOuroborosLoopWithTracing(ctx context.Context, need *ToolNeed, userRequest string) (*LoopResult, *ReasoningTrace) {
	// Inject learnings from past tool generation into prompts
	o.RefreshLearningsContext()

	// Start trace
	trace := o.StartToolTrace(need.Name, need, userRequest)

	// Execute the loop
	result := o.ouroboros.Execute(ctx, need)

	// Record generation learning for persistence
	o.recordGenerationLearning(ctx, need, result)

	// Inject logging if successful
	if result.Success && result.ToolHandle != nil {
		// The tool is already compiled, but we record for future generations
		trace.PostExecutionNotes = append(trace.PostExecutionNotes,
			"Tool compiled and registered successfully")
	}

	// Finalize trace
	failureReason := ""
	if result.Error != "" {
		failureReason = result.Error
	}
	code := ""
	if result.ToolHandle != nil {
		code = fmt.Sprintf("[compiled binary at %s]", result.ToolHandle.BinaryPath)
	}
	o.FinalizeTrace(trace, result.Success, code, failureReason)

	return result, trace
}
