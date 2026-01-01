package system

import (
	"context"
	stderrors "errors"
	"fmt"
	"regexp"
	"strings"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/mangle/feedback"
	"codenerd/internal/mangle/synth"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// LegislatorShard translates corrective feedback into durable policy rules.
// It synthesizes Mangle rules (via LLM or direct input), ratifies them in a sandbox,
// and hot-loads them into the learned policy layer.
type LegislatorShard struct {
	*BaseSystemShard
	feedbackLoop    *feedback.FeedbackLoop
	promptAssembler *articulation.PromptAssembler
}

// llmClientAdapter adapts types.LLMClient to feedback.LLMClient interface.
type llmClientAdapter struct {
	client     types.LLMClient
	costGuard  *CostGuard
	shardID    string
	traceLLMIO bool
}

// Complete implements feedback.LLMClient by delegating to types.LLMClient.CompleteWithSystem.
// Responses are processed through the Piggyback Protocol to extract surface content.
func (a *llmClientAdapter) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if a.client == nil {
		return "", fmt.Errorf("no LLM client configured")
	}

	if a.costGuard != nil {
		can, reason := a.costGuard.CanCall()
		if !can {
			logging.Get(logging.CategorySystemShards).Warn("[%s] LLM call blocked: %s", a.shardID, reason)
			return "", fmt.Errorf("LLM call blocked: %s", reason)
		}
	}

	var rawResponse string
	var err error
	schemaJSON := synth.SchemaV1SingleClauseJSON()
	schemaUsed := false
	if schemaJSON != "" {
		if schemaClient, ok := core.AsSchemaCapable(a.client); ok {
			rawResponse, err = schemaClient.CompleteWithSchema(ctx, systemPrompt, userPrompt, schemaJSON)
			if err != nil && stderrors.Is(err, core.ErrSchemaNotSupported) {
				logging.SystemShardsDebug("[%s] Schema enforcement not supported, falling back", a.shardID)
				rawResponse = ""
				err = nil
			} else if err == nil {
				schemaUsed = true
			}
		}
	}
	if rawResponse == "" && err == nil {
		rawResponse, err = a.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	}
	if a.traceLLMIO {
		fields := map[string]interface{}{
			"shard_id":      a.shardID,
			"system_prompt": systemPrompt,
			"user_prompt":   userPrompt,
			"schema_used":   schemaUsed,
			"response":      rawResponse,
			"response_len":  len(rawResponse),
		}
		if err != nil {
			fields["error"] = err.Error()
		}
		logging.Get(logging.CategorySystemShards).StructuredLog("debug", "legislator_llm_exchange", fields)
	}
	if err != nil {
		return "", err
	}

	// Process through Piggyback Protocol - extract surface response
	processed := articulation.ProcessLLMResponseAllowPlain(rawResponse)
	logging.SystemShardsDebug("[%s] Piggyback: method=%s, confidence=%.2f",
		a.shardID, processed.ParseMethod, processed.Confidence)

	return processed.Surface, nil
}

func (a *llmClientAdapter) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return a.Complete(ctx, systemPrompt, userPrompt)
}

// CompleteWithTools implements types.LLMClient interface.
// The legislator shard uses standard completion for rule generation, not tool-calling.
func (a *llmClientAdapter) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if a.client == nil {
		return nil, fmt.Errorf("no LLM client configured")
	}

	// Forward to underlying client if it supports tool calling
	return a.client.CompleteWithTools(ctx, systemPrompt, userPrompt, tools)
}

// NewLegislatorShard creates a Legislator shard.
func NewLegislatorShard() *LegislatorShard {
	logging.SystemShards("[Legislator] Initializing legislator shard")
	base := NewBaseSystemShard("legislator", StartupOnDemand)
	base.Config.Permissions = []types.ShardPermission{
		types.PermissionReadFile,
		types.PermissionWriteFile,
	}
	base.Config.Model = types.ModelConfig{
		Capability: types.CapabilityHighReasoning,
	}

	loop := feedback.NewFeedbackLoop(feedback.DefaultConfig())
	loop.SetSynthMode(feedback.SynthModeRequire, synth.Options{
		RequireSingleClause: true,
		AllowDecls:          false,
		AllowPackage:        false,
		AllowUse:            false,
		SkipAnalysis:        true,
	})
	return &LegislatorShard{
		BaseSystemShard: base,
		feedbackLoop:    loop,
	}
}

// SetParentKernel wires the kernel and configures context-aware predicate selection.
func (l *LegislatorShard) SetParentKernel(k types.Kernel) {
	l.BaseSystemShard.SetParentKernel(k)
	if rk, ok := k.(*core.RealKernel); ok {
		if corpus := rk.GetPredicateCorpus(); corpus != nil {
			selector := prompt.NewPredicateSelector(corpus)
			if vs := rk.GetVirtualStore(); vs != nil {
				if db := vs.GetLocalDB(); db != nil {
					selector.SetVectorStore(db)
				}
			}
			l.feedbackLoop.SetPredicateSelector(selector)
		}
	}
}

// SetPromptAssembler sets the JIT prompt assembler for dynamic prompt compilation.
func (l *LegislatorShard) SetPromptAssembler(pa *articulation.PromptAssembler) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.promptAssembler = pa
	if pa != nil {
		logging.SystemShards("[Legislator] PromptAssembler attached")
	}
}

// GetPromptAssembler returns the current prompt assembler (may be nil).
func (l *LegislatorShard) GetPromptAssembler() *articulation.PromptAssembler {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.promptAssembler
}

// Execute compiles the provided directive into a Mangle rule, validates it, and applies it.
func (l *LegislatorShard) Execute(ctx context.Context, task string) (string, error) {
	timer := logging.StartTimer(logging.CategorySystemShards, "[Legislator] Execute")
	defer timer.Stop()

	l.SetState(types.ShardStateRunning)
	defer l.SetState(types.ShardStateCompleted)

	if l.Kernel == nil {
		logging.SystemShardsDebug("[Legislator] Creating new kernel (none attached)")
		kernel, err := core.NewRealKernel()
		if err != nil {
			return "", fmt.Errorf("failed to create kernel: %w", err)
		}
		l.Kernel = kernel
	}

	directive := strings.TrimSpace(task)
	if directive == "" {
		logging.SystemShardsDebug("[Legislator] No directive provided, returning ready status")
		return "Legislator ready. Provide a natural-language constraint or a Mangle rule to ratify.", nil
	}

	logging.SystemShards("[Legislator] Compiling rule from directive: %s", truncateForLog(directive, 100))
	rule, err := l.compileRule(ctx, directive)
	if err != nil {
		logging.Get(logging.CategorySystemShards).Error("[Legislator] Rule compilation failed: %v", err)
		return "", err
	}
	logging.SystemShardsDebug("[Legislator] Compiled rule: %s", truncateForLog(rule, 200))

	// Fast stratification pre-check before expensive sandbox validation (audit item 5.2)
	if err := checkStratificationFast(rule); err != nil {
		logging.Get(logging.CategorySystemShards).Warn("[Legislator] Fast stratification check failed: %v", err)
		return fmt.Sprintf("Rule rejected (stratification): %v", err), nil
	}

	court := core.NewRuleCourt(l.Kernel)
	if err := court.RatifyRule(rule); err != nil {
		logging.Get(logging.CategorySystemShards).Warn("[Legislator] Rule rejected by court: %v", err)
		return fmt.Sprintf("Rule rejected: %v", err), nil
	}
	logging.SystemShardsDebug("[Legislator] Rule passed court ratification")

	// POWER-USER-FEATURE: Validate rule against schema before hot-loading
	if errs := l.Kernel.ValidateLearnedRules([]string{rule}); len(errs) > 0 {
		logging.Get(logging.CategorySystemShards).Warn("[Legislator] Schema validation failed: %v", errs[0])
		return fmt.Sprintf("Rule rejected (schema): %v", errs[0]), nil
	}
	logging.SystemShardsDebug("[Legislator] Rule passed schema validation")

	if err := l.Kernel.HotLoadLearnedRule(rule); err != nil {
		logging.Get(logging.CategorySystemShards).Error("[Legislator] Failed to hot-load rule: %v", err)
		return "", fmt.Errorf("failed to apply rule: %w", err)
	}

	logging.SystemShards("[Legislator] Rule ratified and hot-loaded successfully")
	return fmt.Sprintf("Rule ratified and applied:\n%s", rule), nil
}

// compileRule turns a directive into a Mangle rule (LLM-backed when needed).
// For direct rules (containing :- or starting with Decl), it validates only.
// For natural language directives, it uses the feedback loop for generation and validation.
func (l *LegislatorShard) compileRule(ctx context.Context, directive string) (string, error) {
	if l.Kernel == nil {
		return "", fmt.Errorf("kernel not configured for rule validation")
	}

	// If it already looks like a rule, validate it directly via the feedback loop.
	if looksLikeMangleRule(directive) {
		logging.SystemShardsDebug("[Legislator] Directive is already a Mangle rule, validating via feedback loop")
		rule := strings.TrimSpace(directive)

		result := l.feedbackLoop.ValidateOnly(rule, l.Kernel)
		if !result.Valid {
			errMsgs := make([]string, 0, len(result.Errors))
			for _, e := range result.Errors {
				errMsgs = append(errMsgs, fmt.Sprintf("[%s] %s", e.Category.String(), e.Message))
			}
			logging.Get(logging.CategorySystemShards).Warn("[Legislator] Direct rule validation failed: %v", errMsgs)
			return "", fmt.Errorf("rule validation failed: %s", strings.Join(errMsgs, "; "))
		}

		// Return the sanitized version if auto-repair was applied
		if result.Sanitized != "" && result.Sanitized != rule {
			logging.SystemShardsDebug("[Legislator] Rule auto-repaired by feedback loop")
			return result.Sanitized, nil
		}
		return rule, nil
	}

	// Natural language directive requires LLM synthesis via feedback loop.
	if l.LLMClient == nil {
		logging.Get(logging.CategorySystemShards).Error("[Legislator] No LLM client for rule synthesis")
		return "", fmt.Errorf("LLM client not configured for rule synthesis; provide a Mangle rule directly")
	}

	logging.SystemShardsDebug("[Legislator] Synthesizing rule via feedback loop")

	// Create adapter for the feedback loop's LLMClient interface
	adapter := &llmClientAdapter{
		client:     l.LLMClient,
		costGuard:  l.CostGuard,
		shardID:    l.ID,
		traceLLMIO: l.TraceLLMIOEnabled(),
	}

	// Build the user prompt for directive compilation
	userPrompt := l.buildLegislatorPrompt(directive)

	// Get JIT-compiled system prompt (no fallback)
	systemPrompt := l.getSystemPrompt(ctx)
	if systemPrompt == "" {
		logging.Get(logging.CategorySystemShards).Error("[Legislator] JIT prompt compilation failed - ensure legislator atoms exist")
		return "", fmt.Errorf("JIT prompt compilation failed - ensure legislator atoms exist in internal/prompt/atoms/identity/legislator.yaml")
	}

	// Use the feedback loop for generation with automatic validation and retry
	result, err := l.feedbackLoop.GenerateAndValidate(
		ctx,
		adapter,
		l.Kernel,
		systemPrompt,
		userPrompt,
		"legislator", // domain for valid examples
	)
	if err != nil {
		logging.Get(logging.CategorySystemShards).Error("[Legislator] Feedback loop failed: %v", err)
		return "", fmt.Errorf("rule synthesis failed: %w", err)
	}

	if !result.Valid {
		errMsgs := make([]string, 0, len(result.Errors))
		for _, e := range result.Errors {
			errMsgs = append(errMsgs, fmt.Sprintf("[%s] %s", e.Category.String(), e.Message))
		}
		logging.Get(logging.CategorySystemShards).Warn("[Legislator] Rule synthesis validation failed after %d attempts: %v",
			result.Attempts, errMsgs)
		return "", fmt.Errorf("rule synthesis failed after %d attempts: %s",
			result.Attempts, strings.Join(errMsgs, "; "))
	}

	if result.AutoFixed {
		logging.SystemShardsDebug("[Legislator] Rule auto-repaired by feedback loop sanitizer")
	}
	logging.SystemShardsDebug("[Legislator] LLM synthesized and validated rule in %d attempt(s)", result.Attempts)
	return result.Rule, nil
}

func looksLikeMangleRule(directive string) bool {
	trimmed := strings.TrimSpace(directive)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "Decl ") {
		return true
	}
	return ruleHeadPattern.MatchString(trimmed)
}

// getSystemPrompt returns the system prompt for rule synthesis.
// Uses JIT compilation - returns empty string if JIT is unavailable.
// Legislator system prompts are JIT-compiled from:
//
//	internal/prompt/atoms/identity/legislator.yaml
//	internal/prompt/atoms/system/legislator.yaml
func (l *LegislatorShard) getSystemPrompt(ctx context.Context) string {
	l.mu.RLock()
	pa := l.promptAssembler
	l.mu.RUnlock()

	// JIT compilation required - no fallback
	if pa == nil {
		logging.SystemShards("[Legislator] [ERROR] No PromptAssembler configured - cannot compile prompt")
		return ""
	}

	if !pa.JITReady() {
		logging.SystemShards("[Legislator] [ERROR] JIT not ready - cannot compile prompt (ensure legislator atoms exist)")
		return ""
	}

	pc := &articulation.PromptContext{
		ShardID:    l.ID,
		ShardType:  "legislator",
		SessionCtx: l.Config.SessionContext,
	}
	jitPrompt, err := pa.AssembleSystemPrompt(ctx, pc)
	if err != nil {
		logging.SystemShards("[Legislator] [ERROR] JIT compilation failed: %v", err)
		return ""
	}
	if jitPrompt == "" {
		logging.SystemShards("[Legislator] [ERROR] JIT returned empty prompt - ensure legislator atoms exist")
		return ""
	}

	logging.SystemShards("[Legislator] [JIT] Using JIT-compiled system prompt (%d bytes)", len(jitPrompt))
	return jitPrompt
}

// buildLegislatorPrompt constructs the user prompt for directive compilation.
// The feedback loop enhances this with syntax guidance and predicate lists.
func (l *LegislatorShard) buildLegislatorPrompt(directive string) string {
	var sb strings.Builder
	sb.WriteString("Translate the constraint into a single MangleSynth JSON rule spec.\n")
	sb.WriteString("Only include program.clauses (no package/use/decls).\n")
	sb.WriteString("Use name constants (/atom) for enums; ensure predicates are declared.\n")
	sb.WriteString("Avoid inventing new predicates outside declared schemas; prefer permitted/next_action/safety rules.\n")
	sb.WriteString("Return only the JSON object, no commentary.\n")
	sb.WriteString("If Piggyback Protocol is active, the surface_response field must be the JSON object and nothing else.\n\n")
	sb.WriteString("Constraint:\n")
	sb.WriteString(directive)
	return sb.String()
}

// NOTE: Rule extraction from LLM output is now handled by feedback.ExtractRuleFromResponse.

// NOTE: Legacy legislatorBaselinePrompt constant has been DELETED.
// Legislator system prompts are now JIT-compiled from:
//   internal/prompt/atoms/identity/legislator.yaml
//   internal/prompt/atoms/system/legislator.yaml

// =============================================================================
// STRATIFICATION PRE-CHECK
// =============================================================================

// Patterns for fast stratification pre-check
var (
	ruleHeadPattern    = regexp.MustCompile(`^([a-z_][a-z0-9_]*)\s*\(`)
	negatedBodyPattern = regexp.MustCompile(`!\s*([a-z_][a-z0-9_]*)\s*\(`)
)

// checkStratificationFast performs a lightweight check for obvious stratification violations
// before the expensive sandbox validation. This catches direct self-negation patterns like:
//
//	bad(X) :- !bad(X).
//
// More complex cycles (A -> B -> !A) are caught by the full sandbox evaluation.
func checkStratificationFast(rule string) error {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return nil
	}

	// Extract head predicate name
	headMatch := ruleHeadPattern.FindStringSubmatch(rule)
	if headMatch == nil {
		return nil // Not a standard rule format, let sandbox handle it
	}
	headPred := headMatch[1]

	// Find the rule body (after :-)
	parts := strings.SplitN(rule, ":-", 2)
	if len(parts) != 2 {
		return nil // Fact, not a rule
	}
	body := parts[1]

	// Check if head predicate appears negated in body (direct self-negation)
	negatedMatches := negatedBodyPattern.FindAllStringSubmatch(body, -1)
	for _, match := range negatedMatches {
		if len(match) > 1 && match[1] == headPred {
			return fmt.Errorf("direct self-negation: %s appears negated in its own body", headPred)
		}
	}

	return nil
}
