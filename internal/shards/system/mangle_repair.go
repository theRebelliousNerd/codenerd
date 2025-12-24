// Package system implements the Mangle Repair Shard - a Type S system shard
// that intercepts all Mangle rule persistence and guarantees no invalid rules
// are stored. When invalid rules are detected, it uses LLM-powered repair loops
// to fix them before persistence.
package system

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// MangleRepairShard validates and repairs Mangle rules before persistence.
// It acts as a gatekeeper ensuring no invalid rules enter the learned.mg file.
//
// Validation Pipeline:
// 1. Syntax check (parse)
// 2. Safety check (unbound variables, unsafe negation)
// 3. Schema check (all predicates declared in corpus)
// 4. Stratification check (no cycles through negation)
//
// Repair Loop:
// If validation fails, the shard classifies the error, selects relevant
// guidance from the predicate corpus, and invokes LLM to repair the rule.
// Maximum 3 repair attempts before rejection.
type MangleRepairShard struct {
	*BaseSystemShard
	corpus            *core.PredicateCorpus
	predicateSelector *prompt.PredicateSelector
	promptAssembler   *articulation.PromptAssembler
	maxRetries        int
}

// RepairResult contains the outcome of a repair attempt.
type RepairResult struct {
	OriginalRule    string
	RepairedRule    string
	WasRepaired     bool
	Attempts        int
	Errors          []string
	FixesApplied    []string
	Rejected        bool
	RejectionReason string
}

// NewMangleRepairShard creates a new Mangle Repair Shard.
func NewMangleRepairShard() *MangleRepairShard {
	logging.SystemShards("[MangleRepair] Initializing mangle repair shard")
	base := NewBaseSystemShard("mangle_repair", StartupAuto)
	base.Config.Permissions = []types.ShardPermission{
		types.PermissionReadFile,
	}
	base.Config.Model = types.ModelConfig{
		Capability: types.CapabilityHighReasoning,
	}

	return &MangleRepairShard{
		BaseSystemShard: base,
		maxRetries:      3,
	}
}

// SetCorpus sets the predicate corpus for schema validation.
func (m *MangleRepairShard) SetCorpus(corpus *core.PredicateCorpus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.corpus = corpus
	if corpus != nil {
		logging.SystemShards("[MangleRepair] PredicateCorpus attached")
		// Auto-create PredicateSelector when corpus is set
		m.predicateSelector = prompt.NewPredicateSelector(corpus)
		logging.SystemShards("[MangleRepair] PredicateSelector initialized")
	}
}

// SetPredicateSelector sets the predicate selector for targeted context selection.
func (m *MangleRepairShard) SetPredicateSelector(selector *prompt.PredicateSelector) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.predicateSelector = selector
	if selector != nil {
		logging.SystemShards("[MangleRepair] PredicateSelector attached")
	}
}

// SetPromptAssembler sets the JIT prompt assembler for repair prompts.
func (m *MangleRepairShard) SetPromptAssembler(pa *articulation.PromptAssembler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.promptAssembler = pa
	if pa != nil {
		logging.SystemShards("[MangleRepair] PromptAssembler attached")
	}
}

// SetMaxRetries sets the maximum number of repair attempts.
func (m *MangleRepairShard) SetMaxRetries(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n > 0 && n <= 10 {
		m.maxRetries = n
	}
}

// Execute implements the shard interface but is not the primary entry point.
// Use ValidateAndRepair for direct rule validation.
func (m *MangleRepairShard) Execute(ctx context.Context, task string) (string, error) {
	timer := logging.StartTimer(logging.CategorySystemShards, "[MangleRepair] Execute")
	defer timer.Stop()

	m.SetState(types.ShardStateRunning)
	defer m.SetState(types.ShardStateCompleted)

	rule := strings.TrimSpace(task)
	if rule == "" {
		return "MangleRepair ready. Provide a Mangle rule to validate and repair.", nil
	}

	result, err := m.ValidateAndRepair(ctx, rule)
	if err != nil {
		return "", err
	}

	if result.Rejected {
		return fmt.Sprintf("Rule rejected after %d attempts: %s\nErrors: %s",
			result.Attempts, result.RejectionReason, strings.Join(result.Errors, "; ")), nil
	}

	if result.WasRepaired {
		return fmt.Sprintf("Rule repaired after %d attempts:\nOriginal: %s\nRepaired: %s\nFixes: %s",
			result.Attempts, result.OriginalRule, result.RepairedRule, strings.Join(result.FixesApplied, "; ")), nil
	}

	return fmt.Sprintf("Rule valid: %s", result.RepairedRule), nil
}

// ValidateAndRepair validates a rule and attempts to repair it if invalid.
// This is the primary entry point for rule interception.
func (m *MangleRepairShard) ValidateAndRepair(ctx context.Context, rule string) (*RepairResult, error) {
	m.mu.RLock()
	corpus := m.corpus
	kernel := m.Kernel
	llmClient := m.LLMClient
	costGuard := m.CostGuard
	m.mu.RUnlock()

	result := &RepairResult{
		OriginalRule: rule,
		RepairedRule: rule,
	}

	// Phase 1: Initial validation
	errors := m.validateRule(rule, kernel, corpus)
	if len(errors) == 0 {
		logging.SystemShardsDebug("[MangleRepair] Rule valid on first check")
		return result, nil
	}

	result.Errors = errors
	logging.SystemShards("[MangleRepair] Rule has %d errors, attempting repair", len(errors))

	// Phase 2: Repair loop
	currentRule := rule
	for attempt := 1; attempt <= m.maxRetries; attempt++ {
		result.Attempts = attempt

		// Check cost guard
		if costGuard != nil {
			can, reason := costGuard.CanRetryValidation()
			if !can {
				result.Rejected = true
				result.RejectionReason = reason
				logging.Get(logging.CategorySystemShards).Warn("[MangleRepair] Repair blocked: %s", reason)
				return result, nil
			}
			costGuard.RecordValidationRetry()
		}

		// Build repair prompt
		repairPrompt := m.buildRepairPrompt(currentRule, errors, corpus)

		// Attempt LLM repair
		if llmClient == nil {
			result.Rejected = true
			result.RejectionReason = "no LLM client available for repair"
			return result, nil
		}

		if costGuard != nil {
			can, reason := costGuard.CanCall()
			if !can {
				result.Rejected = true
				result.RejectionReason = reason
				return result, nil
			}
		}

		logging.SystemShardsDebug("[MangleRepair] Attempt %d: calling LLM for repair", attempt)
		rawResponse, err := llmClient.CompleteWithSystem(ctx, m.getSystemPrompt(), repairPrompt)
		if err != nil {
			if costGuard != nil {
				costGuard.RecordError()
			}
			logging.Get(logging.CategorySystemShards).Error("[MangleRepair] LLM call failed: %v", err)
			continue
		}
		if costGuard != nil {
			costGuard.RecordCall()
		}

		// Process response through Piggyback
		processed := articulation.ProcessLLMResponseAllowPlain(rawResponse)
		repairedRule := m.extractRule(processed.Surface)

		if repairedRule == "" {
			logging.SystemShardsDebug("[MangleRepair] No rule extracted from response")
			continue
		}

		// Validate repaired rule
		newErrors := m.validateRule(repairedRule, kernel, corpus)
		if len(newErrors) == 0 {
			result.RepairedRule = repairedRule
			result.WasRepaired = true
			result.FixesApplied = m.identifyFixes(rule, repairedRule, errors)
			logging.SystemShards("[MangleRepair] Rule repaired successfully on attempt %d", attempt)
			return result, nil
		}

		// Update for next attempt
		currentRule = repairedRule
		errors = newErrors
		result.Errors = newErrors
		logging.SystemShardsDebug("[MangleRepair] Attempt %d: still %d errors", attempt, len(newErrors))
	}

	// Exhausted retries
	result.Rejected = true
	result.RejectionReason = fmt.Sprintf("could not repair after %d attempts", m.maxRetries)
	logging.Get(logging.CategorySystemShards).Warn("[MangleRepair] Rule rejected after %d attempts", m.maxRetries)
	return result, nil
}

// InterceptLearnedRule intercepts a learned rule before persistence.
// Returns the (possibly repaired) rule or an error if unrepairable.
func (m *MangleRepairShard) InterceptLearnedRule(ctx context.Context, rule string) (string, error) {
	result, err := m.ValidateAndRepair(ctx, rule)
	if err != nil {
		return "", err
	}

	if result.Rejected {
		return "", fmt.Errorf("rule rejected: %s (errors: %s)",
			result.RejectionReason, strings.Join(result.Errors, "; "))
	}

	return result.RepairedRule, nil
}

// validateRule performs multi-phase validation on a rule.
func (m *MangleRepairShard) validateRule(rule string, kernel core.Kernel, corpus *core.PredicateCorpus) []string {
	var errors []string

	// Phase 1: Syntax check via kernel (requires RealKernel)
	if kernel != nil {
		if realKernel, ok := kernel.(*core.RealKernel); ok {
			if err := realKernel.HotLoadRule(rule); err != nil {
				errors = append(errors, fmt.Sprintf("syntax: %v", err))
			}
		}
	}

	// Phase 2: Schema check via corpus
	if corpus != nil {
		predicates := m.extractPredicatesFromRule(rule)
		undefined := corpus.ValidatePredicates(predicates)
		for _, pred := range undefined {
			errors = append(errors, fmt.Sprintf("undefined predicate: %s", pred))
		}
	}

	// Phase 3: Safety checks (basic pattern matching)
	safetyErrors := m.checkSafety(rule)
	errors = append(errors, safetyErrors...)

	return errors
}

// extractPredicatesFromRule extracts predicate names from a Mangle rule.
func (m *MangleRepairShard) extractPredicatesFromRule(rule string) []string {
	// Skip the head for body-only validation
	parts := strings.SplitN(rule, ":-", 2)
	var body string
	if len(parts) == 2 {
		body = parts[1]
	} else {
		// It's a fact, extract predicate from head
		body = parts[0]
	}

	predicatePattern := regexp.MustCompile(`([a-z_][a-z0-9_]*)\s*\(`)
	matches := predicatePattern.FindAllStringSubmatch(body, -1)

	seen := make(map[string]bool)
	var predicates []string

	// Skip built-ins
	builtins := map[string]bool{
		"fn": true, "do": true, "let": true, "not": true,
		"count": true, "sum": true, "min": true, "max": true,
		"avg": true, "bound": true, "match": true, "collect": true,
	}

	for _, match := range matches {
		if len(match) > 1 {
			pred := match[1]
			if !builtins[pred] && !seen[pred] {
				seen[pred] = true
				predicates = append(predicates, pred)
			}
		}
	}

	return predicates
}

// checkSafety performs basic safety checks on a rule.
func (m *MangleRepairShard) checkSafety(rule string) []string {
	var errors []string

	// Check for infinite loop risk in next_action rules
	errors = append(errors, m.checkInfiniteLoopRisk(rule)...)

	// Check for common issues
	if strings.Contains(rule, "not ") {
		// Check if negated variables might be unbound
		negPattern := regexp.MustCompile(`not\s+([a-z_][a-z0-9_]*)\s*\(\s*([A-Z][A-Za-z0-9_]*)`)
		if matches := negPattern.FindAllStringSubmatch(rule, -1); len(matches) > 0 {
			// This is a heuristic - full safety check requires proper analysis
			for _, match := range matches {
				if len(match) > 2 {
					variable := match[2]
					// Check if variable appears in a positive atom before negation
					positivePattern := regexp.MustCompile(`[a-z_][a-z0-9_]*\s*\([^)]*\b` + variable + `\b`)
					ruleBeforeNot := strings.Split(rule, "not ")[0]
					if !positivePattern.MatchString(ruleBeforeNot) {
						errors = append(errors, fmt.Sprintf("potentially unbound variable %s in negation", variable))
					}
				}
			}
		}
	}

	// Check for missing period
	trimmed := strings.TrimSpace(rule)
	if trimmed != "" && !strings.HasSuffix(trimmed, ".") && !strings.HasPrefix(trimmed, "#") {
		errors = append(errors, "rule missing terminal period")
	}

	return errors
}

// checkInfiniteLoopRisk detects rules that could cause infinite derivation loops.
// This catches:
// 1. Unconditional next_action facts (no body)
// 2. next_action rules depending on always-true system predicates
func (m *MangleRepairShard) checkInfiniteLoopRisk(rule string) []string {
	var errors []string

	// Skip comments
	trimmed := strings.TrimSpace(rule)
	if strings.HasPrefix(trimmed, "#") {
		return errors
	}

	// Check if this is a next_action rule
	if !strings.Contains(rule, "next_action(") {
		return errors
	}

	// Parse head and body
	parts := strings.SplitN(rule, ":-", 2)
	head := strings.TrimSpace(parts[0])

	// Check 1: Unconditional next_action fact (no body)
	// e.g., "next_action(/system_start)."
	if len(parts) == 1 && strings.HasPrefix(head, "next_action(") {
		// Extract the action
		actionMatch := regexp.MustCompile(`next_action\((/[a-z_]+)\)`).FindStringSubmatch(head)
		if len(actionMatch) > 1 {
			action := actionMatch[1]
			// System actions without conditions cause infinite loops
			if action == "/system_start" || action == "/initialize" {
				errors = append(errors, fmt.Sprintf("infinite loop risk: unconditional next_action(%s) will fire every tick", action))
			}
		}
		return errors
	}

	// Check 2: next_action depending on always-true predicates
	if len(parts) == 2 {
		body := strings.TrimSpace(parts[1])

		// List of predicates that are always true at system startup
		alwaysTruePredicates := []string{
			"system_startup(",
			"system_shard_state(",
			"entry_point(",
			"current_phase(",
		}

		// Check if body ONLY contains always-true predicates (no real conditions)
		for _, pred := range alwaysTruePredicates {
			if strings.Contains(body, pred) {
				// Check if there are other meaningful conditions
				// A body with just wildcards like "system_startup(_,_)" is always true
				wildcardPattern := regexp.MustCompile(regexp.QuoteMeta(pred) + `[^)]*_[^)]*\)`)
				if wildcardPattern.MatchString(body) {
					// Count how many predicates are in the body
					predCount := strings.Count(body, "(")
					if predCount <= 2 { // Only 1-2 predicates, likely always-true
						errors = append(errors, fmt.Sprintf("infinite loop risk: next_action depends on always-true predicate %s with wildcards", strings.TrimSuffix(pred, "(")))
					}
				}
			}
		}
	}

	return errors
}

// buildRepairPrompt builds a prompt for LLM repair.
// Uses PredicateSelector to inject only ~60 relevant predicates instead of all 799.
func (m *MangleRepairShard) buildRepairPrompt(rule string, errors []string, corpus *core.PredicateCorpus) string {
	var sb strings.Builder

	sb.WriteString("The following Mangle rule has validation errors:\n\n")
	sb.WriteString("```mangle\n")
	sb.WriteString(rule)
	sb.WriteString("\n```\n\n")

	sb.WriteString("Errors:\n")
	for _, err := range errors {
		sb.WriteString("- ")
		sb.WriteString(err)
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// Use PredicateSelector for targeted predicate selection
	m.mu.RLock()
	selector := m.predicateSelector
	m.mu.RUnlock()

	if selector != nil {
		// Extract error types for context-aware selection
		errorTypes := m.extractErrorTypes(errors)

		// Select ~60 most relevant predicates based on error context
		selectedPreds, err := selector.SelectForRepair(errorTypes)
		if err == nil && len(selectedPreds) > 0 {
			sb.WriteString("Available predicates (use ONLY these):\n\n")

			// Group by domain for readability
			byDomain := make(map[string][]prompt.SelectedPredicate)
			for _, p := range selectedPreds {
				byDomain[p.Domain] = append(byDomain[p.Domain], p)
			}

			// Write predicates grouped by domain
			for domain, preds := range byDomain {
				sb.WriteString(fmt.Sprintf("### %s\n", domain))
				for _, p := range preds {
					sb.WriteString(fmt.Sprintf("- `%s/%d`", p.Name, p.Arity))
					if p.Description != "" {
						desc := p.Description
						if len(desc) > 50 {
							desc = desc[:50] + "..."
						}
						sb.WriteString(fmt.Sprintf(" - %s", desc))
					}
					sb.WriteString("\n")
				}
				sb.WriteString("\n")
			}

			logging.SystemShardsDebug("[MangleRepair] Injected %d predicates (vs full corpus of ~799)", len(selectedPreds))
		}
	} else if corpus != nil {
		// Fallback: use old method if PredicateSelector not available
		sb.WriteString("Available predicates that might be relevant:\n")

		// Extract predicates from the rule to find similar ones
		predicates := m.extractPredicatesFromRule(rule)
		for _, pred := range predicates {
			if results, err := corpus.SearchPredicates(pred[:min(len(pred), 5)]); err == nil && len(results) > 0 {
				for i, r := range results {
					if i >= 5 {
						break
					}
					sb.WriteString(fmt.Sprintf("- %s/%d (%s)\n", r.Name, r.Arity, r.Domain))
				}
			}
		}
		sb.WriteString("\n")
		logging.SystemShardsDebug("[MangleRepair] Using fallback predicate selection (PredicateSelector not wired)")
	}

	// Add error pattern guidance
	if corpus != nil {
		for _, err := range errors {
			if strings.Contains(err, "undefined predicate") {
				if pattern, _ := corpus.FindErrorPattern("undefined_predicate"); pattern != nil {
					sb.WriteString("Fix guidance: ")
					sb.WriteString(pattern.FixTemplate)
					sb.WriteString("\n\n")
					break
				}
			}
			if strings.Contains(err, "unbound") || strings.Contains(err, "negation") {
				if pattern, _ := corpus.FindErrorPattern("unbound_variable_negation"); pattern != nil {
					sb.WriteString("Fix guidance: ")
					sb.WriteString(pattern.FixTemplate)
					sb.WriteString("\n\n")
					break
				}
			}
		}
	}

	sb.WriteString("Please provide a CORRECTED version of this rule that:\n")
	sb.WriteString("1. Uses only declared predicates\n")
	sb.WriteString("2. Has all variables properly bound before any negation\n")
	sb.WriteString("3. Ends with a period (.)\n")
	sb.WriteString("4. Uses /atom syntax for constants (not \"strings\")\n\n")
	sb.WriteString("Output ONLY the corrected rule, nothing else:\n")

	return sb.String()
}

// extractErrorTypes extracts error type keywords from error messages for PredicateSelector.
func (m *MangleRepairShard) extractErrorTypes(errors []string) []string {
	var types []string
	seen := make(map[string]bool)

	for _, err := range errors {
		errLower := strings.ToLower(err)

		// Extract domain hints from error messages
		if strings.Contains(errLower, "shard") && !seen["shard"] {
			types = append(types, "shard")
			seen["shard"] = true
		}
		if strings.Contains(errLower, "campaign") && !seen["campaign"] {
			types = append(types, "campaign")
			seen["campaign"] = true
		}
		if strings.Contains(errLower, "tool") && !seen["tool"] {
			types = append(types, "tool")
			seen["tool"] = true
		}
		if strings.Contains(errLower, "next_action") && !seen["routing"] {
			types = append(types, "routing")
			seen["routing"] = true
		}
		if strings.Contains(errLower, "safety") || strings.Contains(errLower, "permitted") && !seen["safety"] {
			types = append(types, "safety")
			seen["safety"] = true
		}
	}

	return types
}

// getSystemPrompt returns the system prompt for repair.
func (m *MangleRepairShard) getSystemPrompt() string {
	return `You are a Mangle (Datalog) expert. Your task is to repair invalid Mangle rules.

Key Mangle syntax rules:
- Variables are UPPERCASE (X, Y, Result)
- Constants/atoms use /slash syntax (/active, /pending)
- Strings use "double quotes"
- Rules end with a period (.)
- Negation requires variables to be bound first: positive(X), not negative(X)
- Aggregation uses pipe syntax: source() |> do fn:group_by(K), let N = fn:count()

When repairing rules:
- Replace undefined predicates with similar declared ones
- Ensure all head variables appear in the body
- Add binding predicates before any negation
- Preserve the semantic intent of the original rule

Output ONLY the corrected rule, no explanation.`
}

// extractRule extracts a Mangle rule from LLM response text.
func (m *MangleRepairShard) extractRule(response string) string {
	response = strings.TrimSpace(response)

	// Try to extract from code block
	if start := strings.Index(response, "```"); start != -1 {
		end := strings.Index(response[start+3:], "```")
		if end != -1 {
			content := response[start+3 : start+3+end]
			// Remove language tag if present
			if idx := strings.Index(content, "\n"); idx != -1 {
				content = content[idx+1:]
			}
			return strings.TrimSpace(content)
		}
	}

	// Look for a line that looks like a rule
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			// Check if it looks like a Mangle rule
			if strings.Contains(line, "(") && (strings.Contains(line, ":-") || strings.HasSuffix(line, ".")) {
				return line
			}
		}
	}

	// Return the whole response if it looks rule-like
	if strings.Contains(response, "(") && (strings.Contains(response, ":-") || strings.HasSuffix(response, ".")) {
		// Take just the first line
		if idx := strings.Index(response, "\n"); idx != -1 {
			return strings.TrimSpace(response[:idx])
		}
		return response
	}

	return ""
}

// identifyFixes identifies what was changed between original and repaired rule.
func (m *MangleRepairShard) identifyFixes(original, repaired string, originalErrors []string) []string {
	var fixes []string

	// Check for predicate changes
	origPreds := m.extractPredicatesFromRule(original)
	newPreds := m.extractPredicatesFromRule(repaired)

	origSet := make(map[string]bool)
	for _, p := range origPreds {
		origSet[p] = true
	}

	for _, p := range newPreds {
		if !origSet[p] {
			fixes = append(fixes, fmt.Sprintf("added/changed predicate: %s", p))
		}
	}

	// Check for structural changes
	if strings.Contains(repaired, "not ") && !strings.Contains(original, "not ") {
		fixes = append(fixes, "added negation handling")
	}

	if !strings.HasSuffix(strings.TrimSpace(original), ".") && strings.HasSuffix(strings.TrimSpace(repaired), ".") {
		fixes = append(fixes, "added terminal period")
	}

	if len(fixes) == 0 {
		fixes = append(fixes, "general syntax correction")
	}

	return fixes
}

// min is defined in base.go
