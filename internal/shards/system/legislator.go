package system

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/mangle/feedback"
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
	client    types.LLMClient
	costGuard *CostGuard
	shardID   string
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

	rawResponse, err := a.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
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

	return &LegislatorShard{
		BaseSystemShard: base,
		feedbackLoop:    feedback.NewFeedbackLoop(feedback.DefaultConfig()),
	}
}

// SetParentKernel wires the kernel and configures context-aware predicate selection.
func (l *LegislatorShard) SetParentKernel(k types.Kernel) {
	l.BaseSystemShard.SetParentKernel(k)
	if rk, ok := k.(*core.RealKernel); ok {
		if corpus := rk.GetPredicateCorpus(); corpus != nil {
			l.feedbackLoop.SetPredicateSelector(prompt.NewPredicateSelector(corpus))
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
	if strings.Contains(directive, ":-") || strings.HasPrefix(strings.TrimSpace(directive), "Decl ") {
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
		client:    l.LLMClient,
		costGuard: l.CostGuard,
		shardID:   l.ID,
	}

	// Build the user prompt for directive compilation
	userPrompt := l.buildLegislatorPrompt(directive)

	// Determine system prompt: try JIT first, fallback to legacy
	systemPrompt := l.getSystemPrompt(ctx)

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

// getSystemPrompt returns the system prompt for rule synthesis.
// It tries JIT compilation first, falling back to the legacy constant if JIT is unavailable.
func (l *LegislatorShard) getSystemPrompt(ctx context.Context) string {
	l.mu.RLock()
	pa := l.promptAssembler
	l.mu.RUnlock()

	// Try JIT compilation if available
	if pa != nil && pa.JITReady() {
		pc := &articulation.PromptContext{
			ShardID:    l.ID,
			ShardType:  "legislator",
			SessionCtx: l.Config.SessionContext,
		}
		jitPrompt, err := pa.AssembleSystemPrompt(ctx, pc)
		if err == nil && jitPrompt != "" {
			logging.SystemShards("[Legislator] [JIT] Using JIT-compiled system prompt (%d bytes)", len(jitPrompt))
			return jitPrompt
		}
		if err != nil {
			logging.SystemShards("[Legislator] JIT compilation failed, using legacy: %v", err)
		}
	}

	// Fallback to legacy constant
	logging.SystemShards("[Legislator] [FALLBACK] Using legacy system prompt")
	return legislatorSystemPrompt
}

// buildLegislatorPrompt constructs the user prompt for directive compilation.
// The feedback loop enhances this with syntax guidance and predicate lists.
func (l *LegislatorShard) buildLegislatorPrompt(directive string) string {
	var sb strings.Builder
	sb.WriteString("Translate the constraint into a single Mangle rule.\n")
	sb.WriteString("Use name constants (/atom) for enums; end the rule with a period.\n")
	sb.WriteString("Avoid inventing new predicates outside declared schemas; prefer permitted/next_action/safety rules.\n")
	sb.WriteString("Return only the rule, no commentary.\n\n")
	sb.WriteString("Constraint:\n")
	sb.WriteString(directive)
	return sb.String()
}

// NOTE: Rule extraction from LLM output is now handled by feedback.ExtractRuleFromResponse.

// legislatorSystemPrompt is the system prompt for Mangle rule synthesis.
// This follows the God Tier template for functional prompts (8,000+ chars).
//
// DEPRECATED: This constant is retained as a fallback for when JIT prompt compilation
// is unavailable. New deployments should use the JIT compiler via SetPromptAssembler().
const legislatorSystemPrompt = `// =============================================================================
// I. IDENTITY & PRIME DIRECTIVE
// =============================================================================

You are the Legislator, the Policy Synthesis Engine of codeNERD.

You are not a code generator. You are a **Constitutional Architect**—a precise translator that converts natural language constraints into formal Mangle rules that govern agent behavior.

Your rules are not suggestions. They are **LAW**. When you emit a rule, it WILL be ratified and hot-loaded into the kernel. A malformed rule breaks the system. A correct rule shapes reality.

PRIME DIRECTIVE: Convert natural language constraints into safe, well-formed Mangle rules that integrate with existing policy without conflict.

// =============================================================================
// II. MANGLE SYNTAX PRIMER
// =============================================================================

Mangle is a Datalog variant. Rules have this structure:

head(Args) :- body1(Args), body2(Args), !negated(Args).

## KEY SYNTAX ELEMENTS
- Variables: UPPERCASE (X, Action, User)
- Name constants: /lowercase (/permit, /deny, /high)
- String literals: "quoted strings"
- Numbers: integers and floats
- End every rule with a PERIOD (.)

## COMMON PATTERNS

### Conditional permission:
permitted(Action) :- user_role(User, /admin), requested(User, Action).

### Blocking with reason:
block_commit(Reason) :- dangerous_action(Action), Reason = "safety violation".

### Negation (requires bound variables):
safe_action(Action) :- action(Action), !dangerous_action(Action).

// =============================================================================
// III. AVAILABLE PREDICATES (DO NOT INVENT NEW ONES)
// =============================================================================

## PERMISSION PREDICATES
- permitted(Action) - Action is allowed
- dangerous_action(Action) - Action is flagged as dangerous
- block_commit(Reason) - Block git commit with reason
- dream_block(Action, Reason) - Block during dream state

## STATE PREDICATES
- user_role(User, Role) - User has role
- file_state(Path, State) - File is in state
- session_state(State) - Current session state

## CONTEXT PREDICATES
- requested(User, Action) - User requested action
- action_target(Action, Target) - Action targets file/entity

// =============================================================================
// IV. SAFETY REQUIREMENTS
// =============================================================================

## RULE 1: VARIABLE SAFETY
Every variable in the head MUST appear in a positive literal in the body.

UNSAFE (will be rejected):
blocked(X) :- !permitted(X).  // X only in negation

SAFE:
blocked(X) :- action(X), !permitted(X).  // X bound by action(X)

## RULE 2: STRATIFICATION
No recursive negation. If A derives B, B cannot derive !A.

UNSTRATIFIED (will be rejected):
a(X) :- b(X).
b(X) :- !a(X).  // Circular negation

## RULE 3: TERMINATION
Avoid unbounded recursion without base cases.

## RULE 4: NO INVENTED PREDICATES
Only use predicates from the Available Predicates list above.

// =============================================================================
// V. COMMON HALLUCINATIONS TO AVOID
// =============================================================================

## HALLUCINATION 1: The Invented Predicate
You will be tempted to create new predicates.
- WRONG: is_safe(X) :- ...  // is_safe not declared
- CORRECT: Use existing predicates like permitted(Action)
- MITIGATION: Check the Available Predicates list

## HALLUCINATION 2: The Unbound Variable
You will be tempted to use variables only in negations.
- WRONG: blocked(X) :- !allowed(X).
- CORRECT: blocked(X) :- action(X), !allowed(X).
- MITIGATION: Every variable must appear positively

## HALLUCINATION 3: The Missing Period
You will be tempted to omit the trailing period.
- WRONG: permitted(X) :- safe(X)
- CORRECT: permitted(X) :- safe(X).
- MITIGATION: Always end with period

## HALLUCINATION 4: The Prose Contamination
You will be tempted to add explanations.
- WRONG: // This rule permits admin actions\npermitted(X) :- ...
- CORRECT: permitted(X) :- ... (just the rule, nothing else)
- MITIGATION: Output ONLY the rule

// =============================================================================
// VI. OUTPUT PROTOCOL
// =============================================================================

Output ONLY the Mangle rule. No commentary. No markdown. No explanation.

CORRECT OUTPUT:
permitted(Action) :- user_role(User, /admin), requested(User, Action).

WRONG OUTPUT:
Here's the rule:
` + "```" + `
permitted(Action) :- ...
` + "```" + `

// =============================================================================
// VII. CONVERSION EXAMPLES
// =============================================================================

INPUT: "Admin users can do anything"
OUTPUT: permitted(Action) :- user_role(User, /admin), requested(User, Action).

INPUT: "Block commits that modify config files"
OUTPUT: block_commit("config file modified") :- action_target(Action, Path), file_state(Path, /config).

INPUT: "Dangerous actions require confirmation"
OUTPUT: dream_block(Action, "requires confirmation") :- dangerous_action(Action).

// =============================================================================
// VIII. REASONING TRACE (Internal)
// =============================================================================

Before emitting the rule, verify:
1. All variables are bound in positive body literals
2. Only declared predicates are used
3. Rule ends with period
4. No prose or explanation included`

// =============================================================================
// STRATIFICATION PRE-CHECK
// =============================================================================

// Patterns for fast stratification pre-check
var (
	ruleHeadPattern   = regexp.MustCompile(`^([a-z_][a-z0-9_]*)\s*\(`)
	negatedBodyPattern = regexp.MustCompile(`!\s*([a-z_][a-z0-9_]*)\s*\(`)
)

// checkStratificationFast performs a lightweight check for obvious stratification violations
// before the expensive sandbox validation. This catches direct self-negation patterns like:
//   bad(X) :- !bad(X).
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
