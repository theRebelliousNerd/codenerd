package prompt_evolution

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/logging"
)

// LLMClient is the interface for LLM interactions.
type LLMClient interface {
	Complete(ctx context.Context, prompt string) (string, error)
	CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// TaskJudge evaluates task executions using LLM-as-Judge pattern.
// It provides structured verdicts with explanations to enable learning.
type TaskJudge struct {
	llmClient LLMClient
	modelName string // For attribution
}

// NewTaskJudge creates a new task judge.
func NewTaskJudge(llmClient LLMClient, modelName string) *TaskJudge {
	if modelName == "" {
		modelName = "unknown"
	}
	logging.AutopoiesisDebug("Creating TaskJudge with model: %s", modelName)
	return &TaskJudge{
		llmClient: llmClient,
		modelName: modelName,
	}
}

// Evaluate assesses a single execution record and returns a verdict.
func (tj *TaskJudge) Evaluate(ctx context.Context, exec *ExecutionRecord) (*JudgeVerdict, error) {
	timer := logging.StartTimer(logging.CategoryAutopoiesis, "TaskJudge.Evaluate")
	defer timer.Stop()

	logging.AutopoiesisDebug("Evaluating execution: task=%s, shard=%s, success=%v",
		exec.TaskID, exec.ShardType, exec.ExecutionResult.Success)

	// Build the evaluation prompt
	userPrompt := tj.buildEvaluationPrompt(exec)

	// Call LLM
	llmTimer := logging.StartTimer(logging.CategoryAutopoiesis, "LLMJudgeCall")
	response, err := tj.llmClient.CompleteWithSystem(ctx, judgeSystemPrompt, userPrompt)
	llmTimer.Stop()

	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Judge LLM call failed: %v", err)
		return nil, fmt.Errorf("judge evaluation failed: %w", err)
	}

	// Parse the verdict
	verdict, err := tj.parseVerdict(response, exec)
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Failed to parse verdict: %v", err)
		return nil, fmt.Errorf("failed to parse verdict: %w", err)
	}

	verdict.EvaluatedBy = tj.modelName
	verdict.Timestamp = time.Now()

	logging.Autopoiesis("Task evaluated: task=%s, verdict=%s, category=%s, confidence=%.2f",
		exec.TaskID, verdict.Verdict, verdict.Category, verdict.Confidence)

	return verdict, nil
}

// EvaluateBatch evaluates multiple execution records efficiently.
func (tj *TaskJudge) EvaluateBatch(ctx context.Context, execs []*ExecutionRecord) ([]*JudgeVerdict, error) {
	timer := logging.StartTimer(logging.CategoryAutopoiesis, "TaskJudge.EvaluateBatch")
	defer timer.Stop()

	logging.Autopoiesis("Evaluating batch of %d executions", len(execs))

	verdicts := make([]*JudgeVerdict, 0, len(execs))
	var errors []error

	for i, exec := range execs {
		// Skip already evaluated executions
		if exec.Verdict != nil {
			verdicts = append(verdicts, exec.Verdict)
			continue
		}

		verdict, err := tj.Evaluate(ctx, exec)
		if err != nil {
			logging.Get(logging.CategoryAutopoiesis).Warn("Failed to evaluate execution %d/%d: %v",
				i+1, len(execs), err)
			errors = append(errors, err)
			continue
		}

		verdicts = append(verdicts, verdict)
		exec.Verdict = verdict // Attach verdict to execution record
	}

	if len(errors) > 0 {
		logging.Get(logging.CategoryAutopoiesis).Warn("Batch evaluation had %d errors out of %d",
			len(errors), len(execs))
	}

	logging.Autopoiesis("Batch evaluation complete: %d verdicts, %d errors",
		len(verdicts), len(errors))

	return verdicts, nil
}

// buildEvaluationPrompt constructs the prompt for evaluation.
func (tj *TaskJudge) buildEvaluationPrompt(exec *ExecutionRecord) string {
	var sb strings.Builder

	sb.WriteString("## Task Request\n")
	sb.WriteString(exec.TaskRequest)
	sb.WriteString("\n\n")

	sb.WriteString("## Agent's Actions\n")
	if len(exec.AgentActions) == 0 {
		sb.WriteString("No actions recorded.\n")
	} else {
		for i, action := range exec.AgentActions {
			if i >= 10 {
				sb.WriteString(fmt.Sprintf("... and %d more actions\n", len(exec.AgentActions)-10))
				break
			}
			sb.WriteString(fmt.Sprintf("- [%s] %s", action.Type, action.Description))
			if action.Target != "" {
				sb.WriteString(fmt.Sprintf(" → %s", action.Target))
			}
			if !action.Success {
				sb.WriteString(" (FAILED)")
			}
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n")

	sb.WriteString("## Execution Result\n")
	sb.WriteString(fmt.Sprintf("- **Success**: %v\n", exec.ExecutionResult.Success))
	if exec.ExecutionResult.TestsPassed > 0 || exec.ExecutionResult.TestsFailed > 0 {
		sb.WriteString(fmt.Sprintf("- **Tests**: %d passed, %d failed\n",
			exec.ExecutionResult.TestsPassed, exec.ExecutionResult.TestsFailed))
	}
	if len(exec.ExecutionResult.BuildErrors) > 0 {
		sb.WriteString("- **Build Errors**:\n")
		for _, err := range exec.ExecutionResult.BuildErrors {
			sb.WriteString(fmt.Sprintf("  - %s\n", truncateString(err, 200)))
		}
	}
	if exec.ExecutionResult.Output != "" {
		output := truncateString(exec.ExecutionResult.Output, 1000)
		sb.WriteString(fmt.Sprintf("- **Output**: %s\n", output))
	}
	sb.WriteString("\n")

	sb.WriteString("## Context\n")
	sb.WriteString(fmt.Sprintf("- **Shard Type**: %s\n", exec.ShardType))
	sb.WriteString(fmt.Sprintf("- **Duration**: %s\n", exec.Duration))
	if exec.ProblemType != "" {
		sb.WriteString(fmt.Sprintf("- **Problem Type**: %s\n", exec.ProblemType))
	}
	if exec.ThinkingTokens > 0 {
		sb.WriteString(fmt.Sprintf("- **Thinking Tokens**: %d\n", exec.ThinkingTokens))
	}

	// Include model's reasoning process if available (for learning)
	if exec.ThoughtSummary != "" {
		sb.WriteString("\n## Model's Reasoning Process\n")
		summary := truncateString(exec.ThoughtSummary, 2000)
		sb.WriteString(summary)
		sb.WriteString("\n")
	}

	// Include grounding sources if available
	if len(exec.GroundingSources) > 0 {
		sb.WriteString("\n## Grounding Sources\n")
		for _, source := range exec.GroundingSources {
			sb.WriteString(fmt.Sprintf("- %s\n", source))
		}
	}

	return sb.String()
}

// parseVerdict extracts the verdict from the LLM response.
func (tj *TaskJudge) parseVerdict(response string, exec *ExecutionRecord) (*JudgeVerdict, error) {
	// Try to extract JSON from response
	jsonStr := extractJSONBlock(response)
	if jsonStr == "" {
		// Try to find JSON object directly
		jsonStr = extractJSONObject(response)
	}

	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON found in response")
	}

	var parsed struct {
		Verdict         string  `json:"verdict"`
		Explanation     string  `json:"explanation"`
		Category        string  `json:"category"`
		ImprovementRule string  `json:"improvement_rule"`
		Confidence      float64 `json:"confidence,omitempty"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate verdict
	verdict := strings.ToUpper(strings.TrimSpace(parsed.Verdict))
	if verdict != "PASS" && verdict != "FAIL" {
		return nil, fmt.Errorf("invalid verdict: %s", parsed.Verdict)
	}

	// Map category
	category := mapToErrorCategory(parsed.Category)

	// Default confidence if not provided
	confidence := parsed.Confidence
	if confidence == 0 {
		if verdict == "PASS" {
			confidence = 0.85
		} else {
			confidence = 0.80
		}
	}

	return &JudgeVerdict{
		Verdict:         verdict,
		Explanation:     parsed.Explanation,
		Category:        category,
		ImprovementRule: parsed.ImprovementRule,
		Confidence:      confidence,
		TaskID:          exec.TaskID,
		ShardType:       exec.ShardType,
		AtomIDs:         exec.AtomIDs,
	}, nil
}

// mapToErrorCategory maps a string category to ErrorCategory.
func mapToErrorCategory(s string) ErrorCategory {
	normalized := strings.ToUpper(strings.TrimSpace(s))
	switch normalized {
	case "LOGIC_ERROR":
		return CategoryLogicError
	case "SYNTAX_ERROR":
		return CategorySyntaxError
	case "API_MISUSE":
		return CategoryAPIMisuse
	case "EDGE_CASE":
		return CategoryEdgeCase
	case "CONTEXT_MISS":
		return CategoryContextMiss
	case "INSTRUCTION_MISS":
		return CategoryInstructionMiss
	case "HALLUCINATION":
		return CategoryHallucination
	case "CORRECT":
		return CategoryCorrect
	default:
		if strings.Contains(normalized, "LOGIC") {
			return CategoryLogicError
		}
		if strings.Contains(normalized, "SYNTAX") {
			return CategorySyntaxError
		}
		if strings.Contains(normalized, "API") {
			return CategoryAPIMisuse
		}
		if strings.Contains(normalized, "EDGE") {
			return CategoryEdgeCase
		}
		if strings.Contains(normalized, "CONTEXT") {
			return CategoryContextMiss
		}
		if strings.Contains(normalized, "INSTRUCTION") {
			return CategoryInstructionMiss
		}
		if strings.Contains(normalized, "HALLUCIN") {
			return CategoryHallucination
		}
		return CategoryLogicError // Default fallback
	}
}

// extractJSONBlock extracts JSON from a ```json ... ``` code block.
func extractJSONBlock(s string) string {
	start := strings.Index(s, "```json")
	if start == -1 {
		start = strings.Index(s, "```")
		if start == -1 {
			return ""
		}
	}

	// Find the newline after the opening
	start = strings.Index(s[start:], "\n")
	if start == -1 {
		return ""
	}
	start += strings.Index(s, "```") + 1

	// Find closing ```
	end := strings.LastIndex(s, "```")
	if end == -1 || end <= start {
		return ""
	}

	return strings.TrimSpace(s[start:end])
}

// extractJSONObject extracts a JSON object from a string.
func extractJSONObject(s string) string {
	start := strings.Index(s, "{")
	if start == -1 {
		return ""
	}

	// Find matching closing brace
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

// truncateString truncates a string to maxLen characters.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// judgeSystemPrompt is the system prompt for the LLM judge.
var judgeSystemPrompt = `You are an expert evaluator for an AI coding agent. Your job is to assess whether the agent successfully completed its task.

You MUST provide structured evaluation with:
1. A clear PASS or FAIL verdict
2. A specific explanation of what succeeded or failed
3. An error category classification
4. An improvement rule for failed tasks

Be objective and precise. Focus on:
- Did the agent accomplish the stated goal?
- Were there any errors, bugs, or incomplete work?
- Did the agent follow the user's instructions?
- Was the approach appropriate for the problem?

Error Categories:
- LOGIC_ERROR: Wrong approach or algorithm
- SYNTAX_ERROR: Code syntax issues
- API_MISUSE: Wrong API or library usage
- EDGE_CASE: Missing edge case handling
- CONTEXT_MISS: Missed relevant context from the codebase
- INSTRUCTION_MISS: Didn't follow user instructions
- HALLUCINATION: Made up information (files, APIs, etc.)
- CORRECT: Task completed correctly (use with PASS verdict)

Output your evaluation as JSON:
{
  "verdict": "PASS" or "FAIL",
  "explanation": "2-3 sentences explaining the verdict",
  "category": "ERROR_CATEGORY",
  "improvement_rule": "When [situation], always [action]"
}

For improvement_rule:
- Only include for FAIL verdicts
- Make it specific and actionable
- Format: "When [situation], always [action]"
- Examples:
  - "When working with Go channels, always check if they are nil before sending"
  - "When the user mentions a specific file, always read it before making changes"
  - "When implementing API calls, always handle rate limiting and retries"`
