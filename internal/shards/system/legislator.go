package system

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"context"
	"fmt"
	"strings"
)

// LegislatorShard translates corrective feedback into durable policy rules.
// It synthesizes Mangle rules (via LLM or direct input), ratifies them in a sandbox,
// and hot-loads them into the learned policy layer.
type LegislatorShard struct {
	*BaseSystemShard
}

// NewLegislatorShard creates a Legislator shard.
func NewLegislatorShard() *LegislatorShard {
	logging.SystemShards("[Legislator] Initializing legislator shard")
	base := NewBaseSystemShard("legislator", StartupOnDemand)
	base.Config.Permissions = []core.ShardPermission{
		core.PermissionReadFile,
		core.PermissionWriteFile,
	}
	base.Config.Model = core.ModelConfig{
		Capability: core.CapabilityHighReasoning,
	}

	return &LegislatorShard{
		BaseSystemShard: base,
	}
}

// Execute compiles the provided directive into a Mangle rule, validates it, and applies it.
func (l *LegislatorShard) Execute(ctx context.Context, task string) (string, error) {
	timer := logging.StartTimer(logging.CategorySystemShards, "[Legislator] Execute")
	defer timer.Stop()

	l.SetState(core.ShardStateRunning)
	defer l.SetState(core.ShardStateCompleted)

	if l.Kernel == nil {
		logging.SystemShardsDebug("[Legislator] Creating new kernel (none attached)")
		l.Kernel = core.NewRealKernel()
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

	court := core.NewRuleCourt(l.Kernel)
	if err := court.RatifyRule(rule); err != nil {
		logging.Get(logging.CategorySystemShards).Warn("[Legislator] Rule rejected by court: %v", err)
		return fmt.Sprintf("Rule rejected: %v", err), nil
	}
	logging.SystemShardsDebug("[Legislator] Rule passed court ratification")

	if err := l.Kernel.HotLoadLearnedRule(rule); err != nil {
		logging.Get(logging.CategorySystemShards).Error("[Legislator] Failed to hot-load rule: %v", err)
		return "", fmt.Errorf("failed to apply rule: %w", err)
	}

	logging.SystemShards("[Legislator] Rule ratified and hot-loaded successfully")
	return fmt.Sprintf("Rule ratified and applied:\n%s", rule), nil
}

// truncateForLog truncates a string for logging purposes.
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// compileRule turns a directive into a Mangle rule (LLM-backed when needed).
func (l *LegislatorShard) compileRule(ctx context.Context, directive string) (string, error) {
	// If it already looks like a rule, use it directly.
	if strings.Contains(directive, ":-") || strings.HasPrefix(strings.TrimSpace(directive), "Decl ") {
		logging.SystemShardsDebug("[Legislator] Directive is already a Mangle rule, using directly")
		return strings.TrimSpace(directive), nil
	}

	if l.LLMClient == nil {
		logging.Get(logging.CategorySystemShards).Error("[Legislator] No LLM client for rule synthesis")
		return "", fmt.Errorf("LLM client not configured for rule synthesis; provide a Mangle rule directly")
	}

	logging.SystemShardsDebug("[Legislator] Synthesizing rule via LLM")
	userPrompt := l.buildLegislatorPrompt(directive)
	output, err := l.GuardedLLMCall(ctx, legislatorSystemPrompt, userPrompt)
	if err != nil {
		return "", err
	}

	rule := extractLegislatorRule(output)
	if rule == "" {
		logging.Get(logging.CategorySystemShards).Warn("[Legislator] LLM output did not contain a valid rule")
		return "", fmt.Errorf("LLM did not return a usable rule")
	}
	logging.SystemShardsDebug("[Legislator] LLM synthesized rule successfully")
	return rule, nil
}

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

// extractLegislatorRule tries to pull a rule from the LLM output.
func extractLegislatorRule(output string) string {
	out := strings.TrimSpace(output)
	if out == "" {
		return ""
	}

	// Handle fenced code blocks
	if strings.Count(out, "```") >= 2 {
		parts := strings.Split(out, "```")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.Contains(part, ":-") {
				return strings.TrimSpace(part)
			}
		}
	}

	// Look for lines starting with RULE:
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(line), "RULE:") {
			line = strings.TrimSpace(line[len("RULE:"):])
		}
		if strings.Contains(line, ":-") {
			return line
		}
	}

	// Fallback: if the whole output is a rule-like string, return it.
	if strings.Contains(out, ":-") {
		return out
	}

	return ""
}

// legislatorSystemPrompt is the system prompt for Mangle rule synthesis.
// This follows the God Tier template for functional prompts (8,000+ chars).
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
