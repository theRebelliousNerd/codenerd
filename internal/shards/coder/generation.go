package coder

import (
	"context"
	"fmt"
	"strings"
)

// =============================================================================
// CODE GENERATION
// =============================================================================

// generateCode uses LLM to generate code based on the task and context.
func (c *CoderShard) generateCode(ctx context.Context, task CoderTask, fileContext string) (*CoderResult, error) {
	if c.llmClient == nil {
		return nil, fmt.Errorf("no LLM client configured")
	}

	// Build system prompt
	systemPrompt := c.buildSystemPrompt(task)

	// Build user prompt
	userPrompt := c.buildUserPrompt(task, fileContext)

	// Call LLM with retry
	response, err := c.llmCompleteWithRetry(ctx, systemPrompt, userPrompt, 3)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed after retries: %w", err)
	}

	// Parse response into edits
	edits := c.parseCodeResponse(response, task)

	result := &CoderResult{
		Summary: fmt.Sprintf("%s: %s (%d edits)", task.Action, task.Target, len(edits)),
		Edits:   edits,
	}

	return result, nil
}

// buildSystemPrompt creates the system prompt for code generation.
func (c *CoderShard) buildSystemPrompt(task CoderTask) string {
	lang := detectLanguage(task.Target)
	langName := languageDisplayName(lang)

	// Build Code DOM context if we have safety information
	codeDOMContext := c.buildCodeDOMContext(task)

	// Build session context from Blackboard (cross-shard awareness)
	sessionContext := c.buildSessionContextPrompt()

	return fmt.Sprintf(`You are an expert %s programmer. Generate clean, idiomatic, well-documented code.

RULES:
1. Follow language-specific best practices and idioms
2. Include appropriate error handling
3. Add concise comments for complex logic only
4. Do not include unnecessary imports or dependencies
5. Match the existing code style if modifying
%s%s
OUTPUT FORMAT:
Return your response as JSON with this structure:
{
  "file": "path/to/file",
  "content": "full file content here",
  "rationale": "brief explanation of changes"
}

For modifications, include the COMPLETE new file content, not a diff.
`, langName, codeDOMContext, sessionContext)
}

// buildSessionContextPrompt builds comprehensive session context for cross-shard awareness (Blackboard Pattern).
// This injects all available context into the LLM prompt to enable informed code generation.
func (c *CoderShard) buildSessionContextPrompt() string {
	if c.config.SessionContext == nil {
		return ""
	}

	var sb strings.Builder
	ctx := c.config.SessionContext

	// ==========================================================================
	// CURRENT DIAGNOSTICS (Highest Priority - Must Fix)
	// ==========================================================================
	if len(ctx.CurrentDiagnostics) > 0 {
		sb.WriteString("\nCURRENT BUILD/LINT ERRORS (must address):\n")
		for _, diag := range ctx.CurrentDiagnostics {
			sb.WriteString(fmt.Sprintf("  %s\n", diag))
		}
	}

	// ==========================================================================
	// TEST STATE (TDD Loop Awareness)
	// ==========================================================================
	if ctx.TestState == "/failing" || len(ctx.FailingTests) > 0 {
		sb.WriteString("\nTEST STATE: FAILING\n")
		if ctx.TDDRetryCount > 0 {
			sb.WriteString(fmt.Sprintf("  TDD Retry: %d (fix the root cause, not symptoms)\n", ctx.TDDRetryCount))
		}
		for _, test := range ctx.FailingTests {
			sb.WriteString(fmt.Sprintf("  - %s\n", test))
		}
	}

	// ==========================================================================
	// RECENT FINDINGS TO ADDRESS (from reviewer/tester)
	// ==========================================================================
	if len(ctx.RecentFindings) > 0 {
		sb.WriteString("\nRECENT FINDINGS TO ADDRESS:\n")
		for _, finding := range ctx.RecentFindings {
			sb.WriteString(fmt.Sprintf("  - %s\n", finding))
		}
	}

	// ==========================================================================
	// IMPACT ANALYSIS (Transitive Effects)
	// ==========================================================================
	if len(ctx.ImpactedFiles) > 0 {
		sb.WriteString("\nIMPACTED FILES (may need updates):\n")
		for _, file := range ctx.ImpactedFiles {
			sb.WriteString(fmt.Sprintf("  - %s\n", file))
		}
	}

	// ==========================================================================
	// DEPENDENCY CONTEXT (1-hop)
	// ==========================================================================
	if len(ctx.DependencyContext) > 0 {
		sb.WriteString("\nDEPENDENCY CONTEXT:\n")
		for _, dep := range ctx.DependencyContext {
			sb.WriteString(fmt.Sprintf("  - %s\n", dep))
		}
	}

	// ==========================================================================
	// GIT STATE (Chesterton's Fence)
	// ==========================================================================
	if ctx.GitBranch != "" || len(ctx.GitModifiedFiles) > 0 {
		sb.WriteString("\nGIT STATE:\n")
		if ctx.GitBranch != "" {
			sb.WriteString(fmt.Sprintf("  Branch: %s\n", ctx.GitBranch))
		}
		if len(ctx.GitModifiedFiles) > 0 {
			sb.WriteString(fmt.Sprintf("  Modified files: %d\n", len(ctx.GitModifiedFiles)))
		}
		if len(ctx.GitRecentCommits) > 0 {
			sb.WriteString("  Recent commits (context for why code exists):\n")
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
			sb.WriteString(fmt.Sprintf("  Current Phase: %s\n", ctx.CampaignPhase))
		}
		if ctx.CampaignGoal != "" {
			sb.WriteString(fmt.Sprintf("  Phase Goal: %s\n", ctx.CampaignGoal))
		}
		if len(ctx.TaskDependencies) > 0 {
			sb.WriteString("  Blocked by: ")
			sb.WriteString(strings.Join(ctx.TaskDependencies, ", "))
			sb.WriteString("\n")
		}
		if len(ctx.LinkedRequirements) > 0 {
			sb.WriteString("  Fulfills requirements: ")
			sb.WriteString(strings.Join(ctx.LinkedRequirements, ", "))
			sb.WriteString("\n")
		}
	}

	// ==========================================================================
	// PRIOR SHARD OUTPUTS (Cross-Shard Context)
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
		sb.WriteString("\nRECENT SESSION ACTIONS:\n")
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
	// SAFETY CONSTRAINTS
	// ==========================================================================
	if len(ctx.BlockedActions) > 0 || len(ctx.SafetyWarnings) > 0 {
		sb.WriteString("\nSAFETY CONSTRAINTS:\n")
		for _, blocked := range ctx.BlockedActions {
			sb.WriteString(fmt.Sprintf("  BLOCKED: %s\n", blocked))
		}
		for _, warning := range ctx.SafetyWarnings {
			sb.WriteString(fmt.Sprintf("  WARNING: %s\n", warning))
		}
	}

	// ==========================================================================
	// COMPRESSED SESSION HISTORY (Long-range context)
	// ==========================================================================
	if ctx.CompressedHistory != "" && len(ctx.CompressedHistory) < 2000 {
		sb.WriteString("\nSESSION HISTORY (compressed):\n")
		sb.WriteString(ctx.CompressedHistory)
		sb.WriteString("\n")
	}

	return sb.String()
}

// buildCodeDOMContext builds Code DOM safety context for the prompt.
func (c *CoderShard) buildCodeDOMContext(task CoderTask) string {
	if c.kernel == nil {
		return ""
	}

	var warnings []string

	// Check if file is generated code
	generatedResults, _ := c.kernel.Query("generated_code")
	for _, fact := range generatedResults {
		if len(fact.Args) >= 2 {
			if file, ok := fact.Args[0].(string); ok && file == task.Target {
				if generator, ok := fact.Args[1].(string); ok {
					warnings = append(warnings, fmt.Sprintf("WARNING: This is generated code (%s). Changes will be overwritten on regeneration.", generator))
				}
			}
		}
	}

	// Check for breaking change risk
	breakingResults, _ := c.kernel.Query("breaking_change_risk")
	for _, fact := range breakingResults {
		if len(fact.Args) >= 3 {
			if ref, ok := fact.Args[0].(string); ok && strings.Contains(ref, task.Target) {
				if level, ok := fact.Args[1].(string); ok {
					if reason, ok := fact.Args[2].(string); ok {
						warnings = append(warnings, fmt.Sprintf("BREAKING CHANGE RISK (%s): %s", level, reason))
					}
				}
			}
		}
	}

	// Check for API client/handler functions
	apiClientResults, _ := c.kernel.Query("api_client_function")
	for _, fact := range apiClientResults {
		if len(fact.Args) >= 2 {
			if file, ok := fact.Args[1].(string); ok && file == task.Target {
				warnings = append(warnings, "NOTE: This file contains API client code. Ensure error handling for network failures.")
			}
			break // Only add once
		}
	}

	apiHandlerResults, _ := c.kernel.Query("api_handler_function")
	for _, fact := range apiHandlerResults {
		if len(fact.Args) >= 2 {
			if file, ok := fact.Args[1].(string); ok && file == task.Target {
				warnings = append(warnings, "NOTE: This file contains API handlers. Validate inputs and handle errors appropriately.")
			}
			break
		}
	}

	// Check for CGo code
	cgoResults, _ := c.kernel.Query("cgo_code")
	for _, fact := range cgoResults {
		if len(fact.Args) >= 1 {
			if file, ok := fact.Args[0].(string); ok && file == task.Target {
				warnings = append(warnings, "WARNING: This file contains CGo code. Be careful with memory management and type conversions.")
			}
		}
	}

	if len(warnings) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\nCODE CONTEXT:\n")
	for _, w := range warnings {
		sb.WriteString(fmt.Sprintf("- %s\n", w))
	}
	sb.WriteString("\n")
	return sb.String()
}

// buildUserPrompt creates the user prompt with task and context.
func (c *CoderShard) buildUserPrompt(task CoderTask, fileContext string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Task: %s\n", task.Action))
	sb.WriteString(fmt.Sprintf("Target: %s\n", task.Target))
	sb.WriteString(fmt.Sprintf("Instruction: %s\n", task.Instruction))

	if fileContext != "" {
		sb.WriteString("\nExisting file content:\n```\n")
		sb.WriteString(fileContext)
		sb.WriteString("\n```\n")
	}

	// Add any learned preferences
	if len(c.rejectionCount) > 0 {
		sb.WriteString("\nAvoid these patterns (previously rejected):\n")
		for pattern, count := range c.rejectionCount {
			if count >= 2 {
				sb.WriteString(fmt.Sprintf("- %s\n", pattern))
			}
		}
	}

	return sb.String()
}
