package core

import (
	"fmt"
	"os"
	"strings"

	"codenerd/internal/logging"

	"github.com/google/mangle/parse"
)

// =============================================================================
// SCHEMA VALIDATION (Bug #18 Fix - Schema Drift Prevention)
// =============================================================================

// ValidateLearnedRule validates that a learned rule only uses declared predicates.
// This prevents "Schema Drift" where the agent invents predicates with no data source.
func (k *RealKernel) ValidateLearnedRule(ruleText string) error {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.schemaValidator == nil {
		// Validator not initialized - allow (defensive)
		return nil
	}

	return k.schemaValidator.ValidateLearnedRule(ruleText)
}

// ValidateLearnedRules validates multiple learned rules.
// Returns a list of errors (one per invalid rule).
func (k *RealKernel) ValidateLearnedRules(rules []string) []error {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.schemaValidator == nil {
		return nil
	}

	return k.schemaValidator.ValidateRules(rules)
}

// ValidateLearnedProgram validates an entire learned program text.
func (k *RealKernel) ValidateLearnedProgram(programText string) error {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.schemaValidator == nil {
		return nil
	}

	return k.schemaValidator.ValidateProgram(programText)
}

// healLearnedRules validates learned rules and comments out invalid ones.
// This is a self-healing mechanism to recover from corrupted learned.mg files.
// Returns the healed rules text with invalid rules commented out.
// If filePath is provided and rules were healed, persists the healed version to disk.
func (k *RealKernel) healLearnedRules(learnedText string, filePath string) string {
	result := k.validateLearnedRulesContent(learnedText, filePath, true)
	return result.healedText
}

// validateLearnedRulesContent performs startup validation of learned rules.
// Returns validation statistics and optionally the healed text.
func (k *RealKernel) validateLearnedRulesContent(learnedText string, filePath string, heal bool) learnedValidationResult {
	result := learnedValidationResult{
		stats: StartupValidationResult{
			FilePath: filePath,
		},
		healedText: learnedText,
	}

	if k.schemaValidator == nil || learnedText == "" {
		return result
	}

	lines := strings.Split(learnedText, "\n")
	var healedLines []string

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			healedLines = append(healedLines, line)
			continue
		}

		// Track previously self-healed rules
		if strings.HasPrefix(trimmed, "# SELF-HEALED:") {
			result.stats.PreviouslyHealed++
			healedLines = append(healedLines, line)
			continue
		}

		// Track commented-out rules (potential previous self-healing)
		if strings.HasPrefix(trimmed, "#") {
			// Check if this is a commented-out rule (starts with # and contains :-)
			commentContent := strings.TrimPrefix(trimmed, "#")
			commentContent = strings.TrimSpace(commentContent)
			if strings.Contains(commentContent, ":-") && !strings.HasPrefix(commentContent, "SELF-HEALED") {
				result.stats.CommentedRules++
			}
			healedLines = append(healedLines, line)
			continue
		}

		// Check if this is a rule (contains :-) or a fact (no :-)
		isRule := strings.Contains(trimmed, ":-")
		isFact := !isRule && strings.Contains(trimmed, "(") && strings.HasSuffix(trimmed, ").")

		if isRule || isFact {
			result.stats.TotalRules++

			// STEP 1: Syntax validation - try parsing the rule/fact
			if syntaxErr := checkSyntax(trimmed); syntaxErr != nil {
				result.stats.InvalidRules++
				errMsg := fmt.Sprintf("line %d: syntax error: %v", i+1, syntaxErr)
				result.stats.InvalidRuleErrors = append(result.stats.InvalidRuleErrors, errMsg)
				logging.Get(logging.CategoryKernel).Warn("Startup validation: %s", errMsg)

				if heal {
					healedLines = append(healedLines, "# SELF-HEALED: syntax error: "+syntaxErr.Error())
					healedLines = append(healedLines, "# "+line)
				} else {
					healedLines = append(healedLines, line)
				}
				continue
			}

			// STEP 2: Schema + safety validation for learned rules/facts.
			if err := k.schemaValidator.ValidateLearnedRule(trimmed); err != nil {
				result.stats.InvalidRules++
				errMsg := fmt.Sprintf("line %d: %v", i+1, err)
				result.stats.InvalidRuleErrors = append(result.stats.InvalidRuleErrors, errMsg)
				logging.Get(logging.CategoryKernel).Warn("Startup validation: invalid learned rule at %s", errMsg)

				if heal {
					healedLines = append(healedLines, "# SELF-HEALED: "+err.Error())
					healedLines = append(healedLines, "# "+line)
				} else {
					healedLines = append(healedLines, line)
				}
				continue
			}

			// Infinite loop risk detection for next_action rules
			if loopErr := k.checkInfiniteLoopRisk(trimmed); loopErr != "" {
				result.stats.InvalidRules++
				errMsg := fmt.Sprintf("line %d: %s", i+1, loopErr)
				result.stats.InvalidRuleErrors = append(result.stats.InvalidRuleErrors, errMsg)
				logging.Get(logging.CategoryKernel).Warn("Startup validation: %s", errMsg)

				if heal {
					healedLines = append(healedLines, "# SELF-HEALED: "+loopErr)
					healedLines = append(healedLines, "# "+line)
				} else {
					healedLines = append(healedLines, line)
				}
				continue
			}

			result.stats.ValidRules++
		}

		// Valid line - keep as is
		healedLines = append(healedLines, line)
	}

	result.healedText = strings.Join(healedLines, "\n")

	// Log validation summary
	if result.stats.TotalRules > 0 {
		logging.Kernel("Startup validation: %d rules total, %d valid, %d invalid, %d previously healed",
			result.stats.TotalRules, result.stats.ValidRules, result.stats.InvalidRules, result.stats.PreviouslyHealed)
	}

	if result.stats.InvalidRules > 0 && heal {
		logging.Kernel("Self-healing: commented out %d invalid learned rules", result.stats.InvalidRules)

		// Persist healed rules back to disk if we have a file path
		if filePath != "" {
			if err := os.WriteFile(filePath, []byte(result.healedText), 0644); err != nil {
				logging.Get(logging.CategoryKernel).Error("Self-healing: failed to persist healed rules to %s: %v", filePath, err)
			} else {
				logging.Kernel("Self-healing: persisted healed rules to %s", filePath)
			}
		}
	}

	if result.stats.PreviouslyHealed > 0 {
		logging.Get(logging.CategoryKernel).Warn("Startup validation: %d rules were previously self-healed (may indicate recurring issues)", result.stats.PreviouslyHealed)
	}

	return result
}

// checkInfiniteLoopRisk detects rules that could cause infinite derivation loops.
// Returns an error message if the rule is problematic, empty string if OK.
func (k *RealKernel) checkInfiniteLoopRisk(rule string) string {
	// Skip comments
	trimmed := strings.TrimSpace(rule)
	if strings.HasPrefix(trimmed, "#") {
		return ""
	}

	// Only check next_action rules (the main source of runaway loops)
	if !strings.Contains(rule, "next_action(") {
		return ""
	}

	// Parse head and body
	parts := strings.SplitN(rule, ":-", 2)
	head := strings.TrimSpace(parts[0])

	// Check 1: Unconditional next_action fact (no body) for system actions
	if len(parts) == 1 && strings.HasPrefix(head, "next_action(") {
		if strings.Contains(head, "/system_start") || strings.Contains(head, "/initialize") {
			return "infinite loop risk: unconditional next_action for system action will fire every tick"
		}
	}

	// Check 2: next_action depending on always-true or ubiquitous predicates
	if len(parts) == 2 {
		body := strings.TrimSpace(parts[1])
		bodyLower := strings.ToLower(body)

		// === UBIQUITOUS PREDICATES ===
		// These predicates are always present or nearly always true
		ubiquitousPredicates := []string{
			"current_time(",     // Always has a value - DANGEROUS!
			"current_time(_)",   // Wildcard match on time - always fires
			"entry_point(",      // Set at startup
			"current_phase(",    // Always has a phase
			"build_system(",     // System state always present
			"system_startup(",   // Present after startup
			"northstar_defined", // If northstar is set, always true
		}

		for _, pred := range ubiquitousPredicates {
			if strings.Contains(body, pred) {
				// Single-predicate body with ubiquitous fact = infinite loop
				predCount := strings.Count(body, "(")
				if predCount <= 1 {
					return fmt.Sprintf("infinite loop risk: next_action depends solely on ubiquitous predicate '%s'", strings.TrimSuffix(pred, "("))
				}
			}
		}

		// === IDLE STATE PREDICATES ===
		// Rules that fire when system is idle cause continuous loops
		idleStatePatterns := []string{
			"coder_state(/idle)",
			"current_task(/idle)",
			"_state(/idle)",
			"_status(/idle)",
			"/idle)",
		}

		for _, pattern := range idleStatePatterns {
			if strings.Contains(bodyLower, strings.ToLower(pattern)) {
				predCount := strings.Count(body, "(")
				if predCount <= 2 {
					return fmt.Sprintf("infinite loop risk: next_action fires on idle state '%s' - will loop when system is idle", pattern)
				}
			}
		}

		// === WILDCARD SESSION/SYSTEM STATE ===
		// Rules with wildcards on session_state, session_planner_status etc.
		wildcardStatePatterns := []struct {
			pred    string
			minArgs int // minimum args to be considered dangerous with wildcards
		}{
			{"session_state(", 2},
			{"session_planner_status(", 3},
			{"system_shard_state(", 2},
			{"dream_state(", 1},
		}

		for _, wp := range wildcardStatePatterns {
			if strings.Contains(body, wp.pred) {
				// Count wildcards in this predicate
				wildcardCount := strings.Count(body, "_,") + strings.Count(body, ",_)") + strings.Count(body, "(_")
				if wildcardCount >= wp.minArgs {
					predCount := strings.Count(body, "(")
					if predCount <= 2 {
						return fmt.Sprintf("infinite loop risk: next_action depends on '%s' with %d+ wildcards - too broad, will match too often", strings.TrimSuffix(wp.pred, "("), wildcardCount)
					}
				}
			}
		}

		// === NEGATION-ONLY CONDITIONS ===
		// Rules that fire when something is NOT true (negation as sole/main condition)
		if strings.HasPrefix(body, "!") || strings.Contains(body, ", !") {
			positivePredicates := 0
			for _, part := range strings.Split(body, ",") {
				part = strings.TrimSpace(part)
				if part != "" && !strings.HasPrefix(part, "!") && strings.Contains(part, "(") {
					positivePredicates++
				}
			}
			if positivePredicates == 0 {
				return "infinite loop risk: next_action depends solely on negation - fires when condition is absent"
			}
		}
	}

	return ""
}

// GetStartupValidationResult returns the result of the last startup validation.
// This can be called after kernel initialization to check learned rule health.
func (k *RealKernel) GetStartupValidationResult() *StartupValidationResult {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.userLearnedPath == "" {
		return nil
	}

	// Re-validate current learned rules (read-only, no healing)
	data, err := os.ReadFile(k.userLearnedPath)
	if err != nil {
		return nil
	}

	result := k.validateLearnedRulesContent(string(data), k.userLearnedPath, false)
	return &result.stats
}

// IsPredicateDeclared checks if a predicate is declared in schemas.
func (k *RealKernel) IsPredicateDeclared(predicate string) bool {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.schemaValidator == nil {
		return false
	}

	return k.schemaValidator.IsDeclared(predicate)
}

// GetDeclaredPredicates returns all declared predicate signatures.
// Each signature is in the format "predicate_name/arity" (e.g., "user_intent/5").
// This method satisfies the feedback.RuleValidator interface.
func (k *RealKernel) GetDeclaredPredicates() []string {
	k.mu.RLock()
	defer k.mu.RUnlock()

	// Prefer programInfo.Decls for accurate arity information
	if k.programInfo != nil && k.programInfo.Decls != nil {
		signatures := make([]string, 0, len(k.programInfo.Decls))
		for predSym := range k.programInfo.Decls {
			signatures = append(signatures, fmt.Sprintf("%s/%d", predSym.Symbol, predSym.Arity))
		}
		return signatures
	}

	// Fallback to schema validator (names only, no arity)
	if k.schemaValidator == nil {
		return nil
	}

	return k.schemaValidator.GetDeclaredPredicates()
}

// SetSchemas allows loading custom schemas (for testing or shard isolation).
func (k *RealKernel) SetSchemas(schemas string) {
	logging.KernelDebug("SetSchemas: loading custom schemas (%d bytes)", len(schemas))
	k.mu.Lock()
	defer k.mu.Unlock()
	k.schemas = schemas
	k.policyDirty = true
	logging.KernelDebug("SetSchemas: policyDirty set to true, will rebuild on next evaluate")
}

// GetSchemas returns the current schemas.
func (k *RealKernel) GetSchemas() string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.schemas
}

// checkSyntax attempts to parse a single Mangle rule/fact to validate syntax.
// Returns nil if syntax is valid, error otherwise.
func checkSyntax(ruleText string) error {
	// Wrap in minimal program context for parsing
	programText := ruleText

	// Try parsing
	_, err := parse.Unit(strings.NewReader(programText))
	if err != nil {
		// CRITICAL: Return ONLY the first line of error to prevent multi-line
		// error messages from corrupting the healed file.
		errStr := err.Error()

		// Take only first line
		if idx := strings.Index(errStr, "\n"); idx > 0 {
			errStr = errStr[:idx]
		}

		// Strip line/column prefix (e.g., "1:7 ") since we're parsing single rules
		if idx := strings.Index(errStr, " "); idx > 0 {
			if strings.Contains(errStr[:idx], ":") {
				parts := strings.SplitN(errStr, " ", 2)
				if len(parts) > 1 {
					errStr = parts[1]
				}
			}
		}

		// Truncate to avoid extremely long error messages
		if len(errStr) > 100 {
			errStr = errStr[:100] + "..."
		}

		return fmt.Errorf("%s", errStr)
	}
	return nil
}
