package reviewer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// =============================================================================
// LLM ANALYSIS
// =============================================================================

// llmAnalysis uses LLM for semantic code analysis (no dependency context).
func (r *ReviewerShard) llmAnalysis(ctx context.Context, filePath, content string) ([]ReviewFinding, error) {
	return r.llmAnalysisWithDeps(ctx, filePath, content, nil)
}

// llmAnalysisWithDeps uses LLM for semantic code analysis with dependency context.
func (r *ReviewerShard) llmAnalysisWithDeps(ctx context.Context, filePath, content string, depCtx *DependencyContext) ([]ReviewFinding, error) {
	findings := make([]ReviewFinding, 0)

	// Truncate very long files for LLM
	if len(content) > 10000 {
		content = content[:10000] + "\n... (truncated)"
	}

	// Build dependency context section for the prompt
	depContextStr := ""
	if depCtx != nil && len(depCtx.Contents) > 0 {
		var depBuilder strings.Builder
		depBuilder.WriteString("\n\n## Dependency Context (1-hop)\n")

		if len(depCtx.Upstream) > 0 {
			depBuilder.WriteString(fmt.Sprintf("Files this imports (%d): %s\n",
				len(depCtx.Upstream), strings.Join(depCtx.Upstream, ", ")))
		}
		if len(depCtx.Downstream) > 0 {
			depBuilder.WriteString(fmt.Sprintf("Files that import this (%d): %s\n",
				len(depCtx.Downstream), strings.Join(depCtx.Downstream, ", ")))
		}

		depBuilder.WriteString("\n### Related File Contents:\n")
		for depFile, depContent := range depCtx.Contents {
			depBuilder.WriteString(fmt.Sprintf("\n--- %s ---\n```\n%s\n```\n", depFile, depContent))
		}
		depContextStr = depBuilder.String()
	}

	// Build session context from Blackboard (cross-shard awareness)
	sessionContext := r.buildSessionContextPrompt()

	systemPrompt := fmt.Sprintf(`You are a principal engineer performing a holistic code review. Analyze the code for:
1. Functional correctness against the intended behavior and edge cases (invariants, error paths, nil handling).
2. Concurrency and state safety (locks, races, ordering, goroutine leaks, context cancellation, atomicity).
3. Security vulnerabilities (SQLi, XSS, command/OS injection, path traversal, authz/authn gaps, secret handling).
4. Resilience and observability (timeouts, retries/backoff, circuit-breaking, logging quality, metrics/tracing).
5. Performance and resource efficiency (allocation churn, blocking I/O, N+1 queries, hot-path costs, cache use).
6. API/interface and data contracts (backward compatibility, validation, schema mismatches, error surface design).
7. Data integrity and configuration risks (defaults, feature flags, persistence consistency, unsafe fallbacks).
8. Testability and coverage gaps (high-risk areas lacking unit/integration tests or fakes).
9. Maintainability and readability (complexity, duplication, dead code, magic values, missing docs for non-obvious logic).
10. Dependency interactions and module responsibilities (upstream/downstream impact, change-risk to consumers).
%s
Return findings as JSON array:
[{"line": N, "severity": "critical|error|warning|info", "category": "security|bug|performance|maintainability|interface|reliability|testing|documentation", "message": "...", "suggestion": "..."}]

Prefer precise, non-duplicative, actionable findings with a single root cause each. Return [] if the code is clean. Do not omit issues due to brevity; be concise but complete.`, sessionContext)

	userPrompt := fmt.Sprintf("Review this %s file (%s):\n\n```\n%s\n```%s",
		r.detectLanguage(filePath), filePath, content, depContextStr)

	response, err := r.llmCompleteWithRetry(ctx, systemPrompt, userPrompt, 3)
	if err != nil {
		return findings, fmt.Errorf("LLM analysis failed after retries: %w", err)
	}

	// Parse JSON response
	var llmFindings []struct {
		Line       int    `json:"line"`
		Severity   string `json:"severity"`
		Category   string `json:"category"`
		Message    string `json:"message"`
		Suggestion string `json:"suggestion"`
	}

	// Extract JSON from response
	jsonStart := strings.Index(response, "[")
	jsonEnd := strings.LastIndex(response, "]")
	if jsonStart != -1 && jsonEnd > jsonStart {
		jsonStr := response[jsonStart : jsonEnd+1]
		if err := json.Unmarshal([]byte(jsonStr), &llmFindings); err == nil {
			for _, f := range llmFindings {
				findings = append(findings, ReviewFinding{
					File:       filePath,
					Line:       f.Line,
					Severity:   f.Severity,
					Category:   f.Category,
					RuleID:     "LLM001",
					Message:    f.Message,
					Suggestion: f.Suggestion,
				})
			}
		}
	}

	return findings, nil
}

// buildSessionContextPrompt builds comprehensive session context for cross-shard awareness (Blackboard Pattern).
// This injects all available context into the LLM prompt to enable informed code review.
func (r *ReviewerShard) buildSessionContextPrompt() string {
	if r.config.SessionContext == nil {
		return ""
	}

	var sb strings.Builder
	ctx := r.config.SessionContext

	// ==========================================================================
	// CURRENT DIAGNOSTICS (Context for Review)
	// ==========================================================================
	if len(ctx.CurrentDiagnostics) > 0 {
		sb.WriteString("\nKNOWN BUILD/LINT ISSUES:\n")
		for _, diag := range ctx.CurrentDiagnostics {
			sb.WriteString(fmt.Sprintf("  %s\n", diag))
		}
	}

	// ==========================================================================
	// TEST STATE (Review in context of test status)
	// ==========================================================================
	if ctx.TestState == "/failing" || len(ctx.FailingTests) > 0 {
		sb.WriteString("\nTEST STATE: FAILING (consider test coverage in review)\n")
		for _, test := range ctx.FailingTests {
			sb.WriteString(fmt.Sprintf("  - %s\n", test))
		}
	}

	// ==========================================================================
	// IMPACTED FILES (Scope of Review)
	// ==========================================================================
	if len(ctx.ImpactedFiles) > 0 {
		sb.WriteString("\nTRANSITIVELY IMPACTED FILES:\n")
		for _, file := range ctx.ImpactedFiles {
			sb.WriteString(fmt.Sprintf("  - %s\n", file))
		}
	}

	// ==========================================================================
	// SYMBOL CONTEXT (What's in scope)
	// ==========================================================================
	if len(ctx.SymbolContext) > 0 {
		sb.WriteString("\nRELEVANT SYMBOLS:\n")
		for _, sym := range ctx.SymbolContext {
			sb.WriteString(fmt.Sprintf("  - %s\n", sym))
		}
	}

	// ==========================================================================
	// DEPENDENCY CONTEXT
	// ==========================================================================
	if len(ctx.DependencyContext) > 0 {
		sb.WriteString("\nDEPENDENCY CONTEXT:\n")
		for _, dep := range ctx.DependencyContext {
			sb.WriteString(fmt.Sprintf("  - %s\n", dep))
		}
	}

	// ==========================================================================
	// GIT STATE (Chesterton's Fence - Why code exists)
	// ==========================================================================
	if ctx.GitBranch != "" || len(ctx.GitRecentCommits) > 0 {
		sb.WriteString("\nGIT CONTEXT:\n")
		if ctx.GitBranch != "" {
			sb.WriteString(fmt.Sprintf("  Branch: %s\n", ctx.GitBranch))
		}
		if len(ctx.GitRecentCommits) > 0 {
			sb.WriteString("  Recent commits (consider why code was written this way):\n")
			for _, commit := range ctx.GitRecentCommits {
				sb.WriteString(fmt.Sprintf("    - %s\n", commit))
			}
		}
	}

	// ==========================================================================
	// CAMPAIGN CONTEXT (if in campaign)
	// ==========================================================================
	if ctx.CampaignActive {
		sb.WriteString("\nCAMPAIGN CONTEXT:\n")
		if ctx.CampaignPhase != "" {
			sb.WriteString(fmt.Sprintf("  Phase: %s\n", ctx.CampaignPhase))
		}
		if ctx.CampaignGoal != "" {
			sb.WriteString(fmt.Sprintf("  Goal: %s\n", ctx.CampaignGoal))
		}
		if len(ctx.LinkedRequirements) > 0 {
			sb.WriteString("  Review against requirements: ")
			sb.WriteString(strings.Join(ctx.LinkedRequirements, ", "))
			sb.WriteString("\n")
		}
	}

	// ==========================================================================
	// PRIOR SHARD OUTPUTS (What Coder Did)
	// ==========================================================================
	if len(ctx.PriorShardOutputs) > 0 {
		sb.WriteString("\nPRIOR SHARD RESULTS:\n")
		for _, output := range ctx.PriorShardOutputs {
			status := "SUCCESS"
			if !output.Success {
				status = "FAILED"
			}
			sb.WriteString(fmt.Sprintf("  [%s] %s: %s - %s\n",
				output.ShardType, status, output.Task, output.Summary))
		}
	}

	// ==========================================================================
	// RECENT SESSION ACTIONS
	// ==========================================================================
	if len(ctx.RecentActions) > 0 {
		sb.WriteString("\nSESSION ACTIONS:\n")
		for _, action := range ctx.RecentActions {
			sb.WriteString(fmt.Sprintf("  - %s\n", action))
		}
	}

	// ==========================================================================
	// DOMAIN KNOWLEDGE (Type B Specialist Hints)
	// ==========================================================================
	if len(ctx.KnowledgeAtoms) > 0 || len(ctx.SpecialistHints) > 0 {
		sb.WriteString("\nDOMAIN KNOWLEDGE:\n")
		for _, atom := range ctx.KnowledgeAtoms {
			sb.WriteString(fmt.Sprintf("  - %s\n", atom))
		}
		for _, hint := range ctx.SpecialistHints {
			sb.WriteString(fmt.Sprintf("  - HINT: %s\n", hint))
		}
	}

	// ==========================================================================
	// SAFETY CONSTRAINTS (Review Focus)
	// ==========================================================================
	if len(ctx.BlockedActions) > 0 || len(ctx.SafetyWarnings) > 0 {
		sb.WriteString("\nSAFETY FOCUS:\n")
		for _, blocked := range ctx.BlockedActions {
			sb.WriteString(fmt.Sprintf("  MUST CHECK: %s usage\n", blocked))
		}
		for _, warning := range ctx.SafetyWarnings {
			sb.WriteString(fmt.Sprintf("  WARNING: %s\n", warning))
		}
	}

	// ==========================================================================
	// COMPRESSED SESSION HISTORY (Long-range context)
	// ==========================================================================
	if ctx.CompressedHistory != "" && len(ctx.CompressedHistory) < 1500 {
		sb.WriteString("\nSESSION HISTORY (compressed):\n")
		sb.WriteString(ctx.CompressedHistory)
		sb.WriteString("\n")
	}

	return sb.String()
}

// llmCompleteWithRetry calls LLM with exponential backoff retry logic.
func (r *ReviewerShard) llmCompleteWithRetry(ctx context.Context, systemPrompt, userPrompt string, maxRetries int) (string, error) {
	if r.llmClient == nil {
		return "", fmt.Errorf("no LLM client configured")
	}

	var lastErr error
	baseDelay := 500 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			fmt.Printf("[ReviewerShard:%s] LLM retry attempt %d/%d\n", r.id, attempt+1, maxRetries)

			delay := baseDelay * time.Duration(1<<uint(attempt))
			select {
			case <-ctx.Done():
				return "", fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		response, err := r.llmClient.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err == nil {
			return response, nil
		}

		lastErr = err

		if !isRetryableError(err) {
			return "", fmt.Errorf("non-retryable error: %w", err)
		}
	}

	return "", fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// isRetryableError determines if an error should be retried.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Network errors - retryable
	retryablePatterns := []string{
		"timeout",
		"connection",
		"network",
		"temporary",
		"rate limit",
		"503",
		"502",
		"429",
		"context deadline exceeded",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// Auth errors - not retryable
	nonRetryablePatterns := []string{
		"unauthorized",
		"forbidden",
		"invalid api key",
		"401",
		"403",
	}

	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(errStr, pattern) {
			return false
		}
	}

	// Default: retry
	return true
}
