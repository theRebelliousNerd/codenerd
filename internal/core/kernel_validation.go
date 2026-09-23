package core

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/mangle"
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
		return fmt.Errorf("schema validator uninitialized")
	}

	return k.schemaValidator.ValidateLearnedRule(ruleText)
}

// ValidateLearnedRules validates multiple learned rules.
// Returns a list of errors (one per invalid rule).
func (k *RealKernel) ValidateLearnedRules(rules []string) []error {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.schemaValidator == nil {
		return []error{fmt.Errorf("schema validator uninitialized")}
	}

	return k.schemaValidator.ValidateRules(rules)
}

// ValidateLearnedProgram validates an entire learned program text.
func (k *RealKernel) ValidateLearnedProgram(programText string) error {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.schemaValidator == nil {
		return fmt.Errorf("schema validator uninitialized")
	}

	return k.schemaValidator.ValidateProgram(programText)
}

func (k *RealKernel) refreshSchemaValidatorLocked() {
	if k.schemas == "" {
		k.schemaValidator = nil
		return
	}
	k.schemaValidator = mangle.NewSchemaValidator(k.schemas, k.learned)
	if err := k.schemaValidator.LoadDeclaredPredicates(); err != nil {
		k.schemaValidator = nil
		logging.Get(logging.CategoryKernel).Warn("Schema validator inventory failed, validation disabled: %v", err)
	} else {
		logging.KernelDebug("Schema validator refreshed")
	}
}

func (k *RealKernel) refreshSchemaValidator() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.refreshSchemaValidatorLocked()
}

// healLearnedRules validates learned rules and comments out invalid ones.
// This is a self-healing mechanism to recover from corrupted learned.mg files.
// Returns the healed rules text with invalid rules commented out.
// If filePath is provided and rules were healed, persists the healed version to disk.
func (k *RealKernel) healLearnedRules(learnedText string, filePath string) string {
	result := k.validateLearnedRulesContent(learnedText, filePath, true)
	return result.healedText
}

// learnedStatement is one validatable unit: usually a single line, or a
// multi-line rule joined with its continuations by groupLearnedStatements.
type learnedStatement struct {
	text      string   // joined text, for parsing and validation
	lines     []string // original source lines, for healing output
	startLine int      // 1-based start line, for messages
}

// stmtStartPattern recognizes a new statement head at column zero.
var stmtStartPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*\s*\(`)

// isSingleStatement reports whether text parses as exactly one statement.
// The parser auto-generates a Decl for undeclared predicates, so Decls is
// not a statement count: one fact or rule yields exactly one Clause (plus
// its auto Decl), while a bare Decl yields none.
func isSingleStatement(text string) bool {
	unit, err := parseUnit(strings.NewReader(text))
	if err != nil {
		return false
	}
	if len(unit.Clauses) == 1 && len(unit.Decls) <= 1 {
		return true
	}
	return len(unit.Clauses) == 0 && len(unit.Decls) >= 1
}

// isStatementStart reports whether the line looks like a new statement head
// at column zero. Indented lines are continuations, never new statements.
func isStatementStart(line string) bool {
	if line == "" || line[0] == ' ' || line[0] == '\t' {
		return false
	}
	return stmtStartPattern.MatchString(line)
}

// groupLearnedStatements splits text into validatable statements, joining
// continuation lines so a multi-line rule validates (and heals) as one
// unit. Single-line statements group exactly as before; joining only
// triggers for lines that do not end with the statement terminator:
//
//   - a blank line terminates (the missing-dot case: the fragment heals
//     alone and the follower is processed fresh);
//   - comment lines ride along (the parser strips them);
//   - an indented line is a genuine continuation and is absorbed;
//   - a column-zero statement head is usually a new statement — but it
//     could be an unindented continuation, so it is absorbed only when the
//     tentative join parses as exactly one statement.
//
// If a joined block still does not parse, only its first line is emitted
// (it heals as malformed) and the rest is reprocessed, so one garbage line
// can never swallow a valid neighbor.
func groupLearnedStatements(learnedText string) []learnedStatement {
	lines := strings.Split(learnedText, "\n")
	var out []learnedStatement
	emit := func(start int, group []string) {
		out = append(out, learnedStatement{
			text:      strings.Join(group, "\n"),
			lines:     group,
			startLine: start + 1,
		})
	}
	i := 0
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		// Empty lines and comments never join; the validator counts them.
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			emit(i, lines[i:i+1])
			i++
			continue
		}
		start := i
		group := []string{lines[i]}
		i++
		for {
			acc := strings.Join(group, "\n")
			if strings.HasSuffix(strings.TrimSpace(acc), ".") {
				break // complete statement
			}
			if i >= len(lines) {
				break // truncated at EOF
			}
			next := lines[i]
			nextTrimmed := strings.TrimSpace(next)
			if nextTrimmed == "" {
				break // blank line terminates (missing-dot case)
			}
			if strings.HasPrefix(nextTrimmed, "#") {
				group = append(group, next)
				i++
				continue
			}
			if isStatementStart(next) {
				tent := acc + "\n" + next
				if !isSingleStatement(tent) {
					break
				}
			}
			group = append(group, next)
			i++
		}
		joined := strings.Join(group, "\n")
		if len(group) > 1 && !isSingleStatement(joined) {
			emit(start, group[:1])
			i = start + 1
			continue
		}
		emit(start, group)
	}
	return out
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

	statements := groupLearnedStatements(learnedText)
	var healedLines []string

	for _, stmt := range statements {
		line := stmt.text
		lineNo := stmt.startLine
		emitRaw := func() {
			healedLines = append(healedLines, stmt.lines...)
		}
		emitHealed := func(marker string) {
			healedLines = append(healedLines, marker)
			for _, l := range stmt.lines {
				healedLines = append(healedLines, "# "+l)
			}
		}
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			emitRaw()
			continue
		}

		// Track previously self-healed rules
		if strings.HasPrefix(trimmed, "# SELF-HEALED:") {
			result.stats.PreviouslyHealed++
			emitRaw()
			continue
		}

		// Track commented-out rules (potential previous self-healing)
		if after, ok := strings.CutPrefix(trimmed, "#"); ok {
			// Check if this is a commented-out rule (starts with # and contains :-)
			commentContent := after
			commentContent = strings.TrimSpace(commentContent)
			if strings.Contains(commentContent, ":-") && !strings.HasPrefix(commentContent, "SELF-HEALED") {
				result.stats.CommentedRules++
			}
			emitRaw()
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
				errMsg := fmt.Sprintf("line %d: syntax error: %v", lineNo, syntaxErr)
				result.stats.InvalidRuleErrors = append(result.stats.InvalidRuleErrors, errMsg)
				logging.Get(logging.CategoryKernel).Warn("Startup validation: %s", errMsg)

				if heal {
					emitHealed("# SELF-HEALED: syntax error: " + syntaxErr.Error())
				} else {
					emitRaw()
				}
				continue
			}

			// STEP 2: Schema + safety validation for learned rules/facts.
			if err := k.schemaValidator.ValidateLearnedRule(trimmed); err != nil {
				result.stats.InvalidRules++
				errMsg := fmt.Sprintf("line %d: %v", lineNo, err)
				result.stats.InvalidRuleErrors = append(result.stats.InvalidRuleErrors, errMsg)
				logging.Get(logging.CategoryKernel).Warn("Startup validation: invalid learned rule at %s", errMsg)

				if heal {
					emitHealed("# SELF-HEALED: " + err.Error())
				} else {
					emitRaw()
				}
				continue
			}

			// Infinite loop risk detection for next_action rules
			if loopErr := k.checkInfiniteLoopRisk(trimmed); loopErr != "" {
				result.stats.InvalidRules++
				errMsg := fmt.Sprintf("line %d: %s", lineNo, loopErr)
				result.stats.InvalidRuleErrors = append(result.stats.InvalidRuleErrors, errMsg)
				logging.Get(logging.CategoryKernel).Warn("Startup validation: %s", errMsg)

				if heal {
					emitHealed("# SELF-HEALED: " + loopErr)
				} else {
					emitRaw()
				}
				continue
			}

			result.stats.ValidRules++
			emitRaw()
			continue
		}

		// CATCH-ALL: Non-empty, non-comment line that doesn't match rule or fact structure.
		// This catches malformed content: unclosed strings, gibberish, missing-dot expressions, etc.
		// Run through syntax check — if it fails, treat as corrupted and heal.
		result.stats.TotalRules++
		if syntaxErr := checkSyntax(trimmed); syntaxErr != nil {
			result.stats.InvalidRules++
			errMsg := fmt.Sprintf("line %d: malformed statement: %v", lineNo, syntaxErr)
			result.stats.InvalidRuleErrors = append(result.stats.InvalidRuleErrors, errMsg)
			logging.Get(logging.CategoryKernel).Warn("Startup validation: %s", errMsg)

			if heal {
				emitHealed("# SELF-HEALED: malformed statement: " + syntaxErr.Error())
			} else {
				emitRaw()
			}
			continue
		}

		// Syntactically valid but structurally unrecognized — keep it
		result.stats.ValidRules++
		emitRaw()
	}

	result.healedText = strings.Join(healedLines, "\n")

	// Log validation summary
	if result.stats.TotalRules > 0 {
		logging.Kernel("Startup validation: %d rules total, %d valid, %d invalid, %d previously healed",
			result.stats.TotalRules, result.stats.ValidRules, result.stats.InvalidRules, result.stats.PreviouslyHealed)
	}

	if result.stats.InvalidRules > 0 && heal {
		logging.Kernel("Self-healing: commented out %d invalid learned rules", result.stats.InvalidRules)

		// Persist healed rules back to disk atomically if we have a file path
		if filePath != "" {
			tmpPath := filePath + ".tmp"
			if err := os.WriteFile(tmpPath, []byte(result.healedText), 0644); err != nil {
				logging.Get(logging.CategoryKernel).Error("Self-healing: failed to write temp file %s: %v", tmpPath, err)
			} else if err := os.Rename(tmpPath, filePath); err != nil {
				os.Remove(tmpPath) // cleanup
				logging.Get(logging.CategoryKernel).Error("Self-healing: failed to rename temp file to %s: %v", filePath, err)
			} else {
				logging.Kernel("Self-healing: persisted healed rules atomically to %s", filePath)
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
		bodyNoSpace := strings.ReplaceAll(bodyLower, " ", "")
		bodyNoSpace = strings.ReplaceAll(bodyNoSpace, "\t", "")

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
			if strings.Contains(bodyNoSpace, strings.ToLower(pred)) {
				// Single-predicate body with ubiquitous fact = infinite loop
				predCount := strings.Count(bodyNoSpace, "(")
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
			for part := range strings.SplitSeq(body, ",") {
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
	// Snapshot the path first so disk I/O stays out from under the
	// kernel lock (a slow read must not stall writers system-wide).
	k.mu.RLock()
	path := k.userLearnedPath
	k.mu.RUnlock()

	if path == "" {
		return nil
	}

	// Re-validate current learned rules (read-only, no healing)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	k.mu.RLock()
	defer k.mu.RUnlock()
	result := k.validateLearnedRulesContent(string(data), path, false)
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
	k.markPolicyDirtyLocked()
	k.refreshSchemaValidatorLocked()
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
	_, err := parseUnit(strings.NewReader(programText))
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
