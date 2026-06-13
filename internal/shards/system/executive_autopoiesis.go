package system

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// executiveLLMAdapter wraps the shard's GuardedLLMCall to implement feedback.LLMClient.
type executiveLLMAdapter struct {
	shard *ExecutivePolicyShard
	ctx   context.Context
}

// Complete implements feedback.LLMClient.
func (a *executiveLLMAdapter) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return a.shard.GuardedLLMCall(ctx, systemPrompt, userPrompt)
}

func (a *executiveLLMAdapter) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return a.Complete(ctx, systemPrompt, userPrompt)
}

// CompleteWithTools implements types.LLMClient interface.
// The executive policy shard doesn't use tool-calling directly.
func (a *executiveLLMAdapter) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	// Executive policy uses standard completion, not tool calling
	return nil, fmt.Errorf("executiveLLMAdapter does not support CompleteWithTools")
}

// handleAutopoiesis uses the Mangle FeedbackLoop to propose and validate new policy rules.
func (e *ExecutivePolicyShard) handleAutopoiesis(ctx context.Context) {
	cases := e.Autopoiesis.GetUnhandledCases()
	if len(cases) == 0 {
		return
	}

	if e.LLMClient == nil {
		logging.SystemShardsDebug("[ExecutivePolicy] Autopoiesis skipped: no LLM client")
		return
	}

	if e.Kernel == nil {
		logging.SystemShardsDebug("[ExecutivePolicy] Autopoiesis skipped: no kernel")
		return
	}

	// Check if FeedbackLoop's validation budget is exhausted BEFORE attempting
	// This prevents the infinite warning spam when budget is depleted
	if e.feedbackLoop.IsBudgetExhausted() {
		e.mu.Lock()
		alreadyLogged := e.budgetExhaustedLogged
		if !alreadyLogged {
			e.budgetExhaustedLogged = true
		}
		e.mu.Unlock()

		if !alreadyLogged {
			logging.SystemShards("[ExecutivePolicy] Autopoiesis suspended: FeedbackLoop validation budget exhausted (will resume on budget reset)")
		}
		// BUG FIX: Do NOT re-queue cases when budget is exhausted.
		// Re-queuing causes an infinite loop: cases get re-added to UnhandledCases,
		// ShouldPropose() returns true, handleAutopoiesis is called again, budget is
		// still exhausted, cases get re-queued, repeat forever.
		// Cases are discarded for this session. When budget is reset (on new turn/session),
		// fresh unhandled cases will naturally accumulate if needed.
		return
	}

	can, reason := e.CostGuard.CanCall()
	if !can {
		logging.SystemShardsDebug("[ExecutivePolicy] Autopoiesis blocked: %s", reason)
		// Re-queue cases for later processing
		for _, cas := range cases {
			e.Autopoiesis.RecordUnhandled(cas.Query, cas.Context, cas.FactsAtTime)
		}
		return
	}

	// Build the user prompt describing unhandled cases
	userPrompt := e.buildPolicyProposalPrompt(cases)

	canRetry, reason := e.feedbackLoop.CanRetryPrompt(userPrompt)
	if !canRetry {
		logging.SystemShardsDebug("[ExecutivePolicy] Autopoiesis skipped: FeedbackLoop budget exhausted (%s)", reason)
		return
	}

	// Use JIT prompt compilation (no fallback - atoms in internal/prompt/atoms/system/autopoiesis.yaml)
	systemPrompt, jitUsed := e.TryJITPrompt(ctx, "executive_autopoiesis")
	if !jitUsed || systemPrompt == "" {
		logging.SystemShards("[ExecutivePolicy] [ERROR] JIT compilation failed - skipping autopoiesis (ensure atoms exist)")
		return
	}
	logging.SystemShards("[ExecutivePolicy] [JIT] Using JIT-compiled autopoiesis prompt")

	// Create LLM adapter that wraps GuardedLLMCall
	llmAdapter := &executiveLLMAdapter{
		shard: e,
		ctx:   ctx,
	}

	// Use FeedbackLoop for validated rule generation with automatic retry
	logging.SystemShards("[ExecutivePolicy] Invoking FeedbackLoop for autopoiesis rule generation")
	result, err := e.feedbackLoop.GenerateAndValidate(
		ctx,
		llmAdapter,
		e.Kernel, // RealKernel implements RuleValidator
		systemPrompt,
		userPrompt,
		"executive",
	)
	if err != nil {
		logging.Get(logging.CategorySystemShards).Warn(
			"[ExecutivePolicy] FeedbackLoop failed after %d attempts: %v",
			result.Attempts, err,
		)
		// BUG FIX: Do NOT re-queue cases when budget is exhausted.
		// This prevents the infinite loop where cases are re-added, causing
		// ShouldPropose() to return true again immediately.
		// Only re-queue for transient failures (context cancelled, LLM errors, etc.)
		// that might succeed on a later attempt.
		if strings.Contains(err.Error(), "validation budget exhausted") {
			logging.SystemShardsDebug("[ExecutivePolicy] Dropping %d autopoiesis cases due to budget exhaustion", len(cases))
			return
		}
		// For other errors (transient failures), re-queue for later processing
		for _, cas := range cases {
			e.Autopoiesis.RecordUnhandled(cas.Query, cas.Context, cas.FactsAtTime)
		}
		return
	}

	// FeedbackLoop validated the rule; extract metadata via parseProposedRule
	proposedRule := e.parseProposedRule(result.Rule, cases)
	proposedRule.MangleCode = result.Rule // Use the validated (possibly sanitized) rule

	// If parseProposedRule couldn't extract confidence, use a high default since it validated
	if proposedRule.Confidence == 0 {
		proposedRule.Confidence = 0.9 // Validated rules have high implicit confidence
	}

	e.Autopoiesis.RecordProposal(proposedRule)

	if proposedRule.Confidence >= e.Autopoiesis.RuleConfidence {
		if err := e.Kernel.HotLoadLearnedRule(proposedRule.MangleCode); err == nil {
			e.Autopoiesis.RecordApplied(proposedRule.MangleCode)
			logging.SystemShards("[ExecutivePolicy] Autopoiesis rule applied: %s (confidence: %.2f, attempts: %d, auto-fixed: %v)",
				truncateRule(proposedRule.MangleCode), proposedRule.Confidence, result.Attempts, result.AutoFixed)
		} else {
			logging.Get(logging.CategorySystemShards).Error("[ExecutivePolicy] Failed to apply validated rule: %v", err)
		}
	} else {
		// Low confidence rules are recorded but require approval
		if assertErr := e.Kernel.Assert(types.Fact{
			Predicate: "rule_proposal_pending",
			Args: []any{
				"executive_policy",
				proposedRule.MangleCode,
				proposedRule.Rationale,
				proposedRule.Confidence,
				time.Now().Unix(),
			},
		}); assertErr != nil {
			logging.Get(logging.CategorySystemShards).Error(
				"[ExecutivePolicy] Failed to assert rule_proposal_pending: %v", assertErr,
			)
		}
		logging.SystemShards("[ExecutivePolicy] Autopoiesis rule pending approval: confidence %.2f < threshold %.2f",
			proposedRule.Confidence, e.Autopoiesis.RuleConfidence)
	}
}

// truncateRule returns a truncated version of a rule for logging.
func truncateRule(rule string) string {
	const maxLen = 80
	rule = strings.ReplaceAll(rule, "\n", " ")
	if len(rule) > maxLen {
		return rule[:maxLen] + "..."
	}
	return rule
}

// buildPolicyProposalPrompt creates a prompt for policy rule proposals.
func (e *ExecutivePolicyShard) buildPolicyProposalPrompt(cases []UnhandledCase) string {
	var sb strings.Builder
	sb.WriteString("The executive policy could not derive actions for these situations:\n\n")

	for i, cas := range cases {
		sb.WriteString(fmt.Sprintf("%d. Query: %s\n", i+1, cas.Query))
		if cas.Context != nil {
			for k, v := range cas.Context {
				sb.WriteString(fmt.Sprintf("   %s: %s\n", k, v))
			}
		}
	}

	// Add learned patterns
	e.mu.RLock()
	if len(e.patternSuccess) > 0 {
		sb.WriteString("\nSUCCESSFUL PATTERNS (use as reference):\n")
		for pattern, count := range e.patternSuccess {
			if count >= 3 {
				sb.WriteString(fmt.Sprintf("- %s\n", pattern))
			}
		}
	}

	if len(e.patternFailure) > 0 {
		sb.WriteString("\nFAILED PATTERNS (avoid these):\n")
		for pattern, count := range e.patternFailure {
			if count >= 2 {
				sb.WriteString(fmt.Sprintf("- %s\n", pattern))
			}
		}
	}
	e.mu.RUnlock()

	sb.WriteString("\nPropose a Mangle policy rule to handle these cases.\n")
	sb.WriteString("The rule should derive next_action or active_strategy.\n")
	sb.WriteString("Format:\n")
	sb.WriteString("RULE: <mangle code>\n")
	sb.WriteString("CONFIDENCE: <0.0-1.0>\n")
	sb.WriteString("RATIONALE: <explanation>\n")

	return sb.String()
}

// parseProposedRule extracts a proposed rule from LLM output.
func (e *ExecutivePolicyShard) parseProposedRule(output string, cases []UnhandledCase) ProposedRule {
	rule := ProposedRule{
		BasedOn:    cases,
		ProposedAt: time.Now(),
	}

	lines := strings.SplitSeq(output, "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "RULE:"); ok {
			rule.MangleCode = strings.TrimSpace(after)
		} else if after, ok := strings.CutPrefix(line, "CONFIDENCE:"); ok {
			confStr := strings.TrimSpace(after)
			fmt.Sscanf(confStr, "%f", &rule.Confidence)
		} else if after, ok := strings.CutPrefix(line, "RATIONALE:"); ok {
			rule.Rationale = strings.TrimSpace(after)
		}
	}

	return rule
}

// generateShutdownSummary creates a summary of the shard's activity.
func (e *ExecutivePolicyShard) generateShutdownSummary(reason string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return fmt.Sprintf(
		"Executive Policy shutdown (%s). Decisions: %d, Blocked: %d, Strategy changes: %d, Runtime: %s",
		reason,
		e.decisionsCount,
		e.blockCount,
		e.strategyChanges,
		time.Since(e.StartTime).String(),
	)
}
