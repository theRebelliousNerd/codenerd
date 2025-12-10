package reviewer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/logging"
)

// =============================================================================
// PIGGYBACK PROTOCOL PROCESSING
// =============================================================================
// All LLM responses MUST be processed through this layer to:
// 1. Extract and route control_packet to the kernel
// 2. Return only surface_response to the user
// This prevents control data from leaking into user-facing output.

// processLLMResponse handles Piggyback Protocol parsing for all LLM responses.
// It extracts the control_packet, routes it to the kernel, and returns only
// the surface_response for user display.
func (r *ReviewerShard) processLLMResponse(rawResponse string) (surface string, control *articulation.ControlPacket, err error) {
	processor := articulation.NewResponseProcessor()
	processor.RequireValidJSON = false // Allow fallback to raw text

	result, err := processor.Process(rawResponse)
	if err != nil {
		logging.Get(logging.CategoryReviewer).Warn("Piggyback parse failed, using raw response: %v", err)
		return strings.TrimSpace(rawResponse), nil, nil
	}

	logging.ReviewerDebug("Piggyback parse: method=%s, confidence=%.2f, surface_len=%d",
		result.ParseMethod, result.Confidence, len(result.Surface))

	// Route control packet to kernel if present and valid
	if result.ParseMethod != "fallback" {
		r.routeControlPacketToKernel(&result.Control)
		return result.Surface, &result.Control, nil
	}

	return result.Surface, nil, nil
}

// routeControlPacketToKernel processes the control_packet and routes data to the kernel.
// This handles mangle_updates, memory_operations, and self_correction signals.
func (r *ReviewerShard) routeControlPacketToKernel(control *articulation.ControlPacket) {
	if control == nil {
		return
	}

	r.mu.RLock()
	kernel := r.kernel
	r.mu.RUnlock()

	if kernel == nil {
		logging.ReviewerDebug("No kernel available for control packet routing")
		return
	}

	// 1. Assert mangle_updates as facts
	if len(control.MangleUpdates) > 0 {
		logging.ReviewerDebug("Routing %d mangle_updates to kernel", len(control.MangleUpdates))
		for _, atomStr := range control.MangleUpdates {
			// Parse and assert each atom
			// Format: "predicate(arg1, arg2, ...)" or raw strings
			if fact := parseMangleAtom(atomStr); fact != nil {
				if err := kernel.Assert(*fact); err != nil {
					logging.Get(logging.CategoryReviewer).Warn("Failed to assert mangle_update %q: %v", atomStr, err)
				}
			}
		}
	}

	// 2. Process memory_operations (route to LearningStore)
	if len(control.MemoryOperations) > 0 {
		logging.ReviewerDebug("Processing %d memory_operations", len(control.MemoryOperations))
		r.processMemoryOperations(control.MemoryOperations)
	}

	// 3. Track self-correction for autopoiesis
	if control.SelfCorrection != nil && control.SelfCorrection.Triggered {
		logging.Reviewer("Self-correction triggered: %s", control.SelfCorrection.Hypothesis)
		// Could assert this as a fact for learning
		selfCorrFact := core.Fact{
			Predicate: "self_correction_triggered",
			Args:      []interface{}{r.id, control.SelfCorrection.Hypothesis, time.Now().Unix()},
		}
		_ = kernel.Assert(selfCorrFact)
	}

	// 4. Log reasoning trace for debugging/learning
	if control.ReasoningTrace != "" {
		logging.ReviewerDebug("Reasoning trace: %.200s...", control.ReasoningTrace)
	}
}

// processMemoryOperations handles memory_operations from the control packet.
// Routes operations to the appropriate storage system.
func (r *ReviewerShard) processMemoryOperations(ops []articulation.MemoryOperation) {
	r.mu.RLock()
	ls := r.learningStore
	r.mu.RUnlock()

	for _, op := range ops {
		switch op.Op {
		case "store_vector":
			// Store in vector memory - use LearningStore.Save for persistence
			if ls != nil {
				_ = ls.Save("reviewer_memory", op.Key, []any{op.Value}, "")
			}
			logging.ReviewerDebug("Memory store_vector: %s", op.Key)

		case "promote_to_long_term":
			// Promote to cold storage via LearningStore
			if ls != nil {
				_ = ls.Save("reviewer_long_term", op.Key, []any{op.Value}, "")
			}
			logging.ReviewerDebug("Memory promote_to_long_term: %s", op.Key)

		case "note":
			// Transient note - just log for now
			logging.ReviewerDebug("Memory note: %s = %s", op.Key, op.Value)

		case "forget":
			// Mark for forgetting - log for now (LearningStore doesn't have delete)
			// Could use DecayConfidence to reduce relevance
			if ls != nil {
				_ = ls.DecayConfidence("reviewer_memory", 0.0) // Decay to zero
			}
			logging.ReviewerDebug("Memory forget: %s", op.Key)
		}
	}
}

// parseMangleAtom attempts to parse a string into a Mangle fact.
// Returns nil if parsing fails.
func parseMangleAtom(atomStr string) *core.Fact {
	atomStr = strings.TrimSpace(atomStr)
	if atomStr == "" {
		return nil
	}

	// Simple parsing: predicate(arg1, arg2, ...)
	parenIdx := strings.Index(atomStr, "(")
	if parenIdx == -1 {
		// No args - just a predicate name
		return &core.Fact{Predicate: atomStr, Args: nil}
	}

	predicate := strings.TrimSpace(atomStr[:parenIdx])
	argsStr := strings.TrimSuffix(strings.TrimSpace(atomStr[parenIdx+1:]), ")")

	// Split args by comma (simple split, doesn't handle nested parens)
	args := make([]interface{}, 0)
	if argsStr != "" {
		for _, arg := range strings.Split(argsStr, ",") {
			arg = strings.TrimSpace(arg)
			// Remove quotes if present
			arg = strings.Trim(arg, `"'`)
			args = append(args, arg)
		}
	}

	return &core.Fact{Predicate: predicate, Args: args}
}

// =============================================================================
// LLM ANALYSIS
// =============================================================================

// llmAnalysis uses LLM for semantic code analysis (no dependency context).
func (r *ReviewerShard) llmAnalysis(ctx context.Context, filePath, content string) ([]ReviewFinding, string, error) {
	return r.llmAnalysisWithDeps(ctx, filePath, content, nil, nil)
}

// llmAnalysisWithDeps uses LLM for semantic code analysis with dependency and architectural context.
// Returns findings and the raw markdown analysis report.
func (r *ReviewerShard) llmAnalysisWithDeps(ctx context.Context, filePath, content string, depCtx *DependencyContext, archCtx *ArchitectureAnalysis) ([]ReviewFinding, string, error) {
	findings := make([]ReviewFinding, 0)

	// Truncate very long files for LLM
	if len(content) > 10000 {
		content = content[:10000] + "\n... (truncated)"
	}

	// Build dependency context section for the prompt
	var contextBuilder strings.Builder

	// 1. Dependency Context
	if depCtx != nil && len(depCtx.Contents) > 0 {
		contextBuilder.WriteString("\n\n## Dependency Context (1-hop)\n")
		if len(depCtx.Upstream) > 0 {
			contextBuilder.WriteString(fmt.Sprintf("Files this imports (%d): %s\n",
				len(depCtx.Upstream), strings.Join(depCtx.Upstream, ", ")))
		}
		if len(depCtx.Downstream) > 0 {
			contextBuilder.WriteString(fmt.Sprintf("Files that import this (%d): %s\n",
				len(depCtx.Downstream), strings.Join(depCtx.Downstream, ", ")))
		}
		contextBuilder.WriteString("\n### Related File Contents:\n")
		for depFile, depContent := range depCtx.Contents {
			contextBuilder.WriteString(fmt.Sprintf("\n--- %s ---\n```\n%s\n```\n", depFile, depContent))
		}
	}

	// 2. Holographic Architecture Context
	if archCtx != nil {
		contextBuilder.WriteString("\n\n## Holographic Architecture Context\n")
		contextBuilder.WriteString(fmt.Sprintf("- Module: %s\n", archCtx.Module))
		contextBuilder.WriteString(fmt.Sprintf("- Layer: %s\n", archCtx.Layer))
		contextBuilder.WriteString(fmt.Sprintf("- Role: %s\n", archCtx.Role))
		if len(archCtx.Related) > 0 {
			contextBuilder.WriteString(fmt.Sprintf("- Semantically Related: %s\n", strings.Join(archCtx.Related, ", ")))
		}
	}

	// Build session context from Blackboard (cross-shard awareness)
	sessionContext := r.buildSessionContextPrompt()
	depContextStr := contextBuilder.String()

	// Use JIT prompt compilation with fallback to legacy (non-holographic mode)
	systemPrompt := r.buildReviewSystemPrompt(ctx, sessionContext, false)

	userPrompt := fmt.Sprintf("Review this %s file (%s):\n\n```\n%s\n```%s",
		r.detectLanguage(filePath), filePath, content, depContextStr)

	rawResponse, err := r.llmCompleteWithRetry(ctx, systemPrompt, userPrompt, 3)
	if err != nil {
		return findings, "", fmt.Errorf("LLM analysis failed after retries: %w", err)
	}

	// Process through Piggyback Protocol - extract surface, route control to kernel
	surface, _, err := r.processLLMResponse(rawResponse)
	if err != nil {
		logging.Get(logging.CategoryReviewer).Warn("Piggyback processing failed: %v", err)
		surface = rawResponse // Fallback to raw
	}

	// Parse JSON findings from surface response
	findings = r.extractFindingsFromResponse(filePath, surface, "LLM001")

	return findings, surface, nil
}

// extractFindingsFromResponse parses JSON findings from an LLM surface response.
// The response may contain markdown-wrapped JSON arrays of findings.
func (r *ReviewerShard) extractFindingsFromResponse(filePath, response, ruleID string) []ReviewFinding {
	findings := make([]ReviewFinding, 0)

	var llmFindings []struct {
		Line       int    `json:"line"`
		Severity   string `json:"severity"`
		Category   string `json:"category"`
		Message    string `json:"message"`
		Suggestion string `json:"suggestion"`
	}

	// Extract JSON from Markdown code block
	var jsonStr string
	if strings.Contains(response, "```json") {
		parts := strings.Split(response, "```json")
		if len(parts) > 1 {
			jsonStr = strings.Split(parts[1], "```")[0]
		}
	} else if strings.Contains(response, "```") {
		// Fallback for unlabeled blocks
		parts := strings.Split(response, "```")
		if len(parts) > 1 {
			jsonStr = parts[1]
		}
	} else {
		// Fallback: try to find array brackets directly
		start := strings.Index(response, "[")
		end := strings.LastIndex(response, "]")
		if start != -1 && end > start {
			jsonStr = response[start : end+1]
		}
	}

	jsonStr = strings.TrimSpace(jsonStr)

	if jsonStr != "" {
		if err := json.Unmarshal([]byte(jsonStr), &llmFindings); err == nil {
			for _, f := range llmFindings {
				findings = append(findings, ReviewFinding{
					File:       filePath,
					Line:       f.Line,
					Severity:   f.Severity,
					Category:   f.Category,
					RuleID:     ruleID,
					Message:    f.Message,
					Suggestion: f.Suggestion,
				})
			}
		} else {
			logging.ReviewerDebug("Failed to parse JSON findings: %v, JSON: %s", err, jsonStr)
		}
	}

	return findings
}

// llmAnalysisWithHolographic uses LLM for semantic analysis with FULL holographic package context.
// This is the enhanced version that prevents false positives from package-scope blindness.
func (r *ReviewerShard) llmAnalysisWithHolographic(ctx context.Context, filePath, content string, depCtx *DependencyContext, archCtx *ArchitectureAnalysis, holoCtx *HolographicContext) ([]ReviewFinding, string, error) {
	findings := make([]ReviewFinding, 0)

	// Truncate very long files for LLM
	if len(content) > 10000 {
		content = content[:10000] + "\n... (truncated)"
	}

	// Build comprehensive context from all sources
	var contextBuilder strings.Builder

	// 1. HOLOGRAPHIC PACKAGE CONTEXT (NEW - prevents package-scope blindness)
	if holoCtx != nil {
		contextBuilder.WriteString("\n\n## Package Context (CRITICAL FOR ACCURATE REVIEW)\n")
		contextBuilder.WriteString("The following symbols are defined in OTHER files in the SAME PACKAGE.\n")
		contextBuilder.WriteString("These are accessible without import - do NOT flag them as undefined.\n\n")

		if holoCtx.TargetPkg != "" {
			contextBuilder.WriteString(fmt.Sprintf("Package: `%s`\n", holoCtx.TargetPkg))
		}

		// Sibling files
		if len(holoCtx.PackageSiblings) > 0 {
			contextBuilder.WriteString(fmt.Sprintf("Sibling files: %d\n", len(holoCtx.PackageSiblings)))
		}

		// Functions available in package scope
		if len(holoCtx.PackageSignatures) > 0 {
			contextBuilder.WriteString("\n### Functions Defined Elsewhere in Package\n```go\n")
			for _, sig := range holoCtx.PackageSignatures {
				if sig.Receiver != "" {
					contextBuilder.WriteString(fmt.Sprintf("func (%s) %s%s %s  // %s:%d\n",
						sig.Receiver, sig.Name, sig.Params, sig.Returns, sig.File, sig.Line))
				} else {
					contextBuilder.WriteString(fmt.Sprintf("func %s%s %s  // %s:%d\n",
						sig.Name, sig.Params, sig.Returns, sig.File, sig.Line))
				}
			}
			contextBuilder.WriteString("```\n")
		}

		// Types available in package scope
		if len(holoCtx.PackageTypes) > 0 {
			contextBuilder.WriteString("\n### Types Defined Elsewhere in Package\n```go\n")
			for _, t := range holoCtx.PackageTypes {
				switch t.Kind {
				case "struct":
					contextBuilder.WriteString(fmt.Sprintf("type %s struct { ... }  // %s:%d\n", t.Name, t.File, t.Line))
				case "interface":
					contextBuilder.WriteString(fmt.Sprintf("type %s interface { ... }  // %s:%d\n", t.Name, t.File, t.Line))
				default:
					contextBuilder.WriteString(fmt.Sprintf("type %s = ...  // %s:%d\n", t.Name, t.File, t.Line))
				}
			}
			contextBuilder.WriteString("```\n")
		}

		// Architectural context from holographic analysis
		contextBuilder.WriteString("\n### Architectural Position\n")
		if holoCtx.Layer != "" {
			contextBuilder.WriteString(fmt.Sprintf("- Layer: %s\n", holoCtx.Layer))
		}
		if holoCtx.Module != "" {
			contextBuilder.WriteString(fmt.Sprintf("- Module: %s\n", holoCtx.Module))
		}
		if holoCtx.Role != "" {
			contextBuilder.WriteString(fmt.Sprintf("- Role: %s\n", holoCtx.Role))
		}
		if holoCtx.SystemPurpose != "" {
			contextBuilder.WriteString(fmt.Sprintf("- System Purpose: %s\n", holoCtx.SystemPurpose))
		}
		if holoCtx.HasTests {
			contextBuilder.WriteString("- Has test coverage: yes\n")
		}
	}

	// 2. Dependency Context (imports/importers)
	if depCtx != nil && len(depCtx.Contents) > 0 {
		contextBuilder.WriteString("\n\n## Import/Export Dependencies\n")
		if len(depCtx.Upstream) > 0 {
			contextBuilder.WriteString(fmt.Sprintf("Imports from: %s\n", strings.Join(depCtx.Upstream, ", ")))
		}
		if len(depCtx.Downstream) > 0 {
			contextBuilder.WriteString(fmt.Sprintf("Imported by: %s\n", strings.Join(depCtx.Downstream, ", ")))
		}
	}

	// 3. Legacy Architecture Context (if holographic not available)
	if holoCtx == nil && archCtx != nil {
		contextBuilder.WriteString("\n\n## Architecture Context\n")
		contextBuilder.WriteString(fmt.Sprintf("- Module: %s\n", archCtx.Module))
		contextBuilder.WriteString(fmt.Sprintf("- Layer: %s\n", archCtx.Layer))
		contextBuilder.WriteString(fmt.Sprintf("- Role: %s\n", archCtx.Role))
		if len(archCtx.Related) > 0 {
			contextBuilder.WriteString(fmt.Sprintf("- Related: %s\n", strings.Join(archCtx.Related, ", ")))
		}
	}

	// Build session context from Blackboard
	sessionContext := r.buildSessionContextPrompt()
	fullContext := contextBuilder.String()

	// Use JIT prompt compilation with fallback to legacy (holographic mode)
	systemPrompt := r.buildReviewSystemPrompt(ctx, sessionContext, true)

	userPrompt := fmt.Sprintf("Review this %s file (%s):\n\n```\n%s\n```%s",
		r.detectLanguage(filePath), filePath, content, fullContext)

	rawResponse, err := r.llmCompleteWithRetry(ctx, systemPrompt, userPrompt, 3)
	if err != nil {
		return findings, "", fmt.Errorf("LLM analysis failed after retries: %w", err)
	}

	// Process through Piggyback Protocol - extract surface, route control to kernel
	surface, _, err := r.processLLMResponse(rawResponse)
	if err != nil {
		logging.Get(logging.CategoryReviewer).Warn("Piggyback processing failed: %v", err)
		surface = rawResponse // Fallback to raw
	}

	// Parse JSON findings from surface response (LLM002 = holographic-aware)
	findings = r.extractFindingsFromResponse(filePath, surface, "LLM002")

	return findings, surface, nil
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
	// KERNEL-DERIVED CONTEXT (Spreading Activation)
	// ==========================================================================
	// Query the Mangle kernel for injectable context atoms derived from
	// spreading activation rules (injectable_context, specialist_knowledge).
	if r.kernel != nil {
		kernelContext, err := articulation.GetKernelContext(r.kernel, r.id)
		if err != nil {
			logging.ReviewerDebug("Failed to get kernel context: %v", err)
		} else if kernelContext != "" {
			sb.WriteString("\n")
			sb.WriteString(kernelContext)
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
			logging.ReviewerDebug("LLM retry attempt %d/%d", attempt+1, maxRetries)

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

// =============================================================================
// JIT PROMPT COMPILATION
// =============================================================================

// buildReviewSystemPrompt constructs the system prompt for code review.
// Uses JIT compilation if available, otherwise falls back to legacy constants.
// The holographic parameter controls whether to use the package-aware variant.
func (r *ReviewerShard) buildReviewSystemPrompt(ctx context.Context, sessionContext string, holographic bool) string {
	r.mu.RLock()
	pa := r.promptAssembler
	r.mu.RUnlock()

	// Try JIT compilation first if promptAssembler is available and ready
	if pa != nil && pa.JITReady() {
		shardType := "reviewer"
		if holographic {
			shardType = "reviewer_holographic"
		}
		pc := &articulation.PromptContext{
			ShardID:   r.id,
			ShardType: shardType,
		}

		jitPrompt, err := pa.AssembleSystemPrompt(ctx, pc)
		if err == nil && jitPrompt != "" {
			logging.Reviewer("[JIT] Using JIT-compiled system prompt (%d bytes)", len(jitPrompt))
			// Inject session context into JIT prompt
			return fmt.Sprintf("%s\n\n%s", jitPrompt, sessionContext)
		}
		if err != nil {
			logging.Reviewer("[JIT] Compilation failed, falling back to legacy: %v", err)
		}
	}

	// Fallback to legacy prompts
	logging.Reviewer("[FALLBACK] Using legacy template-based system prompt")
	if holographic {
		return fmt.Sprintf(reviewerHolographicPromptTemplate, sessionContext)
	}
	return fmt.Sprintf(reviewerSystemPromptTemplate, sessionContext)
}

// =============================================================================
// DEPRECATED LEGACY PROMPTS
// =============================================================================
// These constants are retained as fallbacks for when JIT prompt compilation
// is unavailable. New code should use the JIT compiler via SetPromptAssembler().

// reviewerSystemPromptTemplate is the legacy basic review prompt.
// DEPRECATED: Prefer JIT prompt compilation via buildReviewSystemPrompt().
const reviewerSystemPromptTemplate = `You are a principal engineer performing a holistic code review. Analyze the code for:
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
11. **Completeness & Debt**: Identify incomplete implementations (TODOs, FIXMEs, stubs, mocks) and assess if they block the current goal.
12. **Campaign Alignment**: If a Campaign Goal is provided, assess if this code advances that goal or introduces unrelated churn.

%s

Format your response as a Markdown report with the following structure:

# Review Report

## Agent Summary
(A concise 1-2 sentence summary for an AI agent to read)

## Holographic Analysis
(Assess the architectural impact based on the provided context)

## Campaign Status
(Assess alignment with the campaign goal, if active)

## Findings
Return a JSON array of findings in a code block:
` + "```json" + `
[{"line": N, "severity": "critical|error|warning|info", "category": "security|bug|performance|maintainability|interface|reliability|testing|documentation|completeness|campaign", "message": "...", "suggestion": "..."}]
` + "```" + `

Prefer precise, non-duplicative, actionable findings.`

// reviewerHolographicPromptTemplate is the legacy holographic-aware review prompt.
// DEPRECATED: Prefer JIT prompt compilation via buildReviewSystemPrompt().
const reviewerHolographicPromptTemplate = `You are a principal engineer performing a holistic code review with FULL PACKAGE CONTEXT.

CRITICAL INSTRUCTION: You have been provided with a list of functions, types, and constants defined in OTHER files of the SAME Go package. These symbols are accessible without import. DO NOT report them as "undefined", "missing", or "not found". This is how Go packages work - all exported AND unexported symbols within a package are visible to all files in that package.

Analyze the code for:
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
11. **Completeness & Debt**: Identify incomplete implementations (TODOs, FIXMEs, stubs, mocks) and assess if they block the current goal.
12. **Campaign Alignment**: If a Campaign Goal is provided, assess if this code advances that goal or introduces unrelated churn.

BEFORE FLAGGING A SYMBOL AS UNDEFINED:
- Check the "Package Context" section above for functions/types defined in sibling files
- Remember that Go packages share all symbols across files
- Only flag as undefined if the symbol is NOT in the package context AND NOT imported

%s

Format your response as a Markdown report with the following structure:

# Review Report

## Agent Summary
(A concise 1-2 sentence summary for an AI agent to read)

## Holographic Analysis
(Assess the architectural impact based on the provided context. Consider the file's role in its module and system.)

## Campaign Status
(Assess alignment with the campaign goal, if active)

## Findings
Return a JSON array of findings in a code block:
` + "```json" + `
[{"line": N, "severity": "critical|error|warning|info", "category": "security|bug|performance|maintainability|interface|reliability|testing|documentation|completeness|campaign", "message": "...", "suggestion": "..."}]
` + "```" + `

Prefer precise, non-duplicative, actionable findings. Verify symbols exist in package context before flagging as undefined.`
