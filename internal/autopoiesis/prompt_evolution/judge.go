package prompt_evolution

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	nerdconfig "codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/prompt"
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

	// The judge's system prompt is compiled from its atom
	// (eval/judge/task_evaluator) under the configured prompt budget.
	compiler PromptCompiler
	jit      nerdconfig.JITConfig
}

// PromptCompiler compiles a system prompt from the atom corpus; the booted
// session's *prompt.JITPromptCompiler is the one production passes.
type PromptCompiler interface {
	Compile(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error)
}

// NewTaskJudge creates a new task judge: its client, the model it names in
// its verdicts, and the compiler and budget its system prompt is compiled
// with.
func NewTaskJudge(llmClient LLMClient, modelName string, compiler PromptCompiler, jit nerdconfig.JITConfig) *TaskJudge {
	if modelName == "" {
		modelName = "unknown"
	}
	logging.AutopoiesisDebug("Creating TaskJudge with model: %s", modelName)
	return &TaskJudge{
		llmClient: llmClient,
		modelName: modelName,
		compiler:  compiler,
		jit:       jit,
	}
}

// Evaluate assesses a single execution record and returns a verdict.
func (tj *TaskJudge) Evaluate(ctx context.Context, exec *ExecutionRecord) (*JudgeVerdict, error) {
	timer := logging.StartTimer(logging.CategoryAutopoiesis, "TaskJudge.Evaluate")
	defer timer.Stop()
	if exec == nil {
		return nil, fmt.Errorf("execution record is nil")
	}
	if tj == nil || tj.llmClient == nil {
		return nil, fmt.Errorf("task judge has no LLM client configured")
	}

	logging.AutopoiesisDebug("Evaluating execution: task=%s, shard=%s, success=%v",
		exec.TaskID, exec.ShardType, exec.ExecutionResult.Success)

	// Build the evaluation prompt
	userPrompt := tj.buildEvaluationPrompt(exec)

	// Call LLM
	llmTimer := logging.StartTimer(logging.CategoryAutopoiesis, "LLMJudgeCall")
	systemPrompt, err := tj.systemPrompt(ctx)
	if err != nil {
		llmTimer.Stop()
		return nil, err
	}
	response, err := tj.llmClient.CompleteWithSystem(ctx, systemPrompt, userPrompt)
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

// systemPrompt compiles the judge's system prompt from its atom
// (internal/prompt/atoms/eval/judge_explanation.yaml, eval/judge/task_evaluator):
// shard type /prompt_judge (a regime value, so only the evaluator's atoms answer
// to it) and intent /judge (no worker intent atom claims it). The atom
// supersedes the Piggyback output protocol and the tool and methodology atoms:
// the reply is one JSON verdict. Until 2026-09-23 the judge ran on a Go copy of
// that atom that had drifted from it; nothing compiled the atom.
func (tj *TaskJudge) systemPrompt(ctx context.Context) (string, error) {
	if tj.compiler == nil {
		return "", fmt.Errorf("task judge has no prompt compiler configured")
	}
	cc := prompt.NewCompilationContext()
	cc.ShardType = "/prompt_judge"
	cc.ShardName = "Prompt Evolution Judge"
	cc.OperationalMode = "/active"
	cc.IntentVerb = "/judge"
	cc.TokenBudget = tj.jit.TokenBudget
	cc.ReservedTokens = tj.jit.ReservedTokens
	cc.ReservedTokensFallbackRatio = tj.jit.ReservedTokensFallbackRatio
	compiled, err := tj.compiler.Compile(ctx, cc)
	if err != nil {
		return "", fmt.Errorf("compile the task judge's prompt: %w", err)
	}
	if compiled == nil || strings.TrimSpace(compiled.Prompt) == "" {
		return "", fmt.Errorf("the task judge's prompt compiled empty")
	}
	return compiled.Prompt, nil
}

// EvaluateBatch evaluates multiple execution records efficiently.
func (tj *TaskJudge) EvaluateBatch(ctx context.Context, execs []*ExecutionRecord) ([]*JudgeVerdict, error) {
	timer := logging.StartTimer(logging.CategoryAutopoiesis, "TaskJudge.EvaluateBatch")
	defer timer.Stop()

	logging.Autopoiesis("Evaluating batch of %d executions", len(execs))

	verdicts := make([]*JudgeVerdict, len(execs))
	var wg sync.WaitGroup

	// Limit to 5 concurrent LLM calls to avoid API rate-limiting
	sem := make(chan struct{}, 5)

	for i, exec := range execs {
		// Skip already evaluated executions
		if exec.Verdict != nil {
			verdicts[i] = exec.Verdict
			continue
		}

		idx, ex := i, exec
		wg.Go(func() {
			sem <- struct{}{}        // acquire token
			defer func() { <-sem }() // release token

			// A panic in a goroutine is unrecoverable from the caller's frame,
			// so it has to be contained here. The batch now runs unattended on
			// the Cortex maintenance schedule, where one malformed record
			// taking the whole process down would be a very expensive way to
			// learn nothing.
			defer func() {
				if r := recover(); r != nil {
					logging.Get(logging.CategoryAutopoiesis).Error(
						"Judge panicked evaluating execution %d/%d: %v", idx+1, len(execs), r)
				}
			}()

			verdict, err := tj.Evaluate(ctx, ex)
			if err != nil {
				logging.Get(logging.CategoryAutopoiesis).Warn("Failed to evaluate execution %d/%d: %v",
					idx+1, len(execs), err)
				return
			}

			verdicts[idx] = verdict
			ex.Verdict = verdict // Attach verdict to execution record
		})
	}
	wg.Wait()

	completed := 0
	for _, v := range verdicts {
		if v != nil {
			completed++
		}
	}

	logging.Autopoiesis("Batch evaluation complete: %d/%d verdicts", completed, len(execs))

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
		Confidence      float64 `json:"confidence,omitzero"`
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
		// Carry serving provenance onto the verdict so a failure stays
		// attributable to the model that produced it once it is separated from
		// its ExecutionRecord -- which is exactly what happens on the way into
		// the atom generator, which sees only []*JudgeVerdict.
		Provider: exec.Provider,
		Model:    exec.Model,
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
