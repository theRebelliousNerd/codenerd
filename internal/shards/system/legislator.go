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

const legislatorSystemPrompt = `You are the Legislator. Convert the constraint into a single safe Mangle rule.
- Use name constants (/atom) when possible.
- Keep it to one rule ending with a period.
- Do not invent undeclared predicates; prefer permitted(Action), dangerous_action(Action), block_commit(Reason), or dream_block(Action, Reason).
- No prose or explanation.`
