package autopoiesis

import (
	"context"
	"fmt"
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
	// Evaluate quality
	if feedback.Quality == nil {
		feedback.Quality = o.evaluator.Evaluate(ctx, feedback)
	}

	// Record in pattern detector
	o.patterns.RecordExecution(*feedback)

	// Get patterns for this tool
	patterns := o.patterns.GetToolPatterns(feedback.ToolName)

	// Update learning store
	o.learnings.RecordLearning(feedback.ToolName, feedback, patterns)

	// Wire to kernel: Assert learning facts for logic-driven refinement
	learning := o.learnings.GetLearning(feedback.ToolName)
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

	// Collect all suggestions
	suggestions := []ImprovementSuggestion{}
	for _, p := range patterns {
		suggestions = append(suggestions, p.Suggestions...)
	}

	req := RefinementRequest{
		ToolName:     toolName,
		OriginalCode: originalCode,
		Patterns:     patterns,
		Suggestions:  suggestions,
	}

	return o.refiner.Refine(ctx, req)
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

	// Aggregate success/failure statistics
	var totalSuccess, totalFail int
	var commonIssues = make(map[IssueType]int)
	var antiPatterns []string

	for _, l := range learnings {
		if l.SuccessRate >= 0.5 {
			totalSuccess++
		} else {
			totalFail++
		}
		for _, issue := range l.KnownIssues {
			commonIssues[issue]++
		}
		antiPatterns = append(antiPatterns, l.AntiPatterns...)
	}

	// Generate summary
	sb.WriteString(fmt.Sprintf("Based on %d previous tool generations:\n", len(learnings)))

	// Top issues to avoid
	if len(commonIssues) > 0 {
		sb.WriteString("\nCOMMON ISSUES TO AVOID:\n")
		for issue, count := range commonIssues {
			if count >= 2 {
				sb.WriteString(fmt.Sprintf("- %s (occurred %d times)\n", issue, count))
			}
		}
	}

	// Anti-patterns from patterns detector
	if len(antiPatterns) > 0 {
		sb.WriteString("\nANTI-PATTERNS DETECTED:\n")
		seen := make(map[string]bool)
		for _, ap := range antiPatterns {
			if !seen[ap] && len(ap) > 0 {
				sb.WriteString(fmt.Sprintf("- %s\n", ap))
				seen[ap] = true
			}
		}
	}

	// Best practices (from successful tools)
	sb.WriteString("\nBEST PRACTICES:\n")
	sb.WriteString("- Always check context.Done() for cancellation\n")
	sb.WriteString("- Handle empty/nil inputs gracefully\n")
	sb.WriteString("- Don't import unused packages\n")
	sb.WriteString("- Return descriptive errors\n")

	// Tool-specific learnings for low-quality tools
	for _, l := range learnings {
		if l.AverageQuality < 0.5 && l.TotalExecutions >= 2 {
			sb.WriteString(fmt.Sprintf("\nTool '%s' had issues (quality=%.1f):\n", l.ToolName, l.AverageQuality))
			for _, issue := range l.KnownIssues {
				sb.WriteString(fmt.Sprintf("  - %s\n", issue))
			}
		}
	}

	result := sb.String()
	if len(result) > 2000 {
		result = result[:2000] + "...[truncated]"
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
