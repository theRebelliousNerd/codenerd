package feedback

import (
	"fmt"
	"strings"
)

// PromptBuilder constructs feedback prompts for LLM retry attempts.
// Uses progressive strategy: more context and constraints on each retry.
type PromptBuilder struct {
	// MangleSyntaxReminder is included in all feedback prompts.
	MangleSyntaxReminder string
}

// NewPromptBuilder creates a prompt builder with default syntax reminders.
func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{
		MangleSyntaxReminder: defaultSyntaxReminder,
	}
}

const defaultSyntaxReminder = `## Mangle Syntax Reminders

CRITICAL - Avoid these common mistakes:
- Use /atom for identifiers, NOT "string" (e.g., /active not "active")
- Negation: Use ! not \+ (e.g., !blocked(X) not \+ blocked(X))
- Aggregation: source() |> do fn:group_by(X), let N = fn:Count()
- Variables: UPPERCASE (X, User), constants: /lowercase
- End every rule with a period (.)
- Bind variables in positive predicates BEFORE using in negation
`

// BuildFeedbackPrompt constructs a feedback prompt based on the validation context.
func (pb *PromptBuilder) BuildFeedbackPrompt(ctx FeedbackContext) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("\n## MANGLE VALIDATION ERROR (attempt %d/%d)\n\n",
		ctx.AttemptNumber, ctx.MaxAttempts))

	// Original rule that failed
	if ctx.OriginalRule != "" {
		sb.WriteString("Your previous rule:\n```mangle\n")
		sb.WriteString(ctx.OriginalRule)
		sb.WriteString("\n```\n\n")
	}

	// Errors with context
	sb.WriteString("### Errors Found:\n\n")
	for i, err := range ctx.Errors {
		sb.WriteString(fmt.Sprintf("**Error %d**: ", i+1))
		sb.WriteString(FormatErrorForFeedback(err))
		sb.WriteString("\n\n")
	}

	// Syntax reminder
	sb.WriteString(pb.MangleSyntaxReminder)
	sb.WriteString("\n")

	// Progressive constraints based on attempt number
	if ctx.AttemptNumber >= 2 {
		sb.WriteString(pb.buildAttempt2Additions(ctx))
	}

	if ctx.AttemptNumber >= ctx.MaxAttempts {
		sb.WriteString(pb.buildFinalAttemptAdditions(ctx))
	}

	// Available predicates if provided
	if len(ctx.AvailablePredicates) > 0 {
		sb.WriteString("\n## Available Predicates (use ONLY these):\n")
		for _, pred := range ctx.AvailablePredicates {
			sb.WriteString("- ")
			sb.WriteString(pred)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Valid examples if provided
	if len(ctx.ValidExamples) > 0 {
		sb.WriteString("\n## Valid Rule Examples:\n```mangle\n")
		for _, ex := range ctx.ValidExamples {
			sb.WriteString(ex)
			sb.WriteString("\n")
		}
		sb.WriteString("```\n")
	}

	// Final instruction
	sb.WriteString("\nPlease regenerate the rule, fixing the above errors.\n")
	sb.WriteString("Respond with ONLY the corrected Mangle rule. No explanation.\n")

	return sb.String()
}

func (pb *PromptBuilder) buildAttempt2Additions(ctx FeedbackContext) string {
	var sb strings.Builder

	sb.WriteString("\n## Additional Guidance (Attempt 2+):\n\n")

	// Add specific guidance based on error types
	hasAtomError := false
	hasAggError := false
	hasNegError := false

	for _, err := range ctx.Errors {
		switch err.Category {
		case CategoryAtomString:
			hasAtomError = true
		case CategoryAggregation:
			hasAggError = true
		case CategoryUnboundNegation:
			hasNegError = true
		}
	}

	if hasAtomError {
		sb.WriteString("### Atom vs String:\n")
		sb.WriteString("```mangle\n")
		sb.WriteString("# WRONG: status(X, \"active\")   <- String literal\n")
		sb.WriteString("# CORRECT: status(X, /active)  <- Atom constant\n")
		sb.WriteString("```\n\n")
	}

	if hasAggError {
		sb.WriteString("### Aggregation Syntax:\n")
		sb.WriteString("```mangle\n")
		sb.WriteString("# WRONG: Total = sum(Amount)\n")
		sb.WriteString("# CORRECT:\n")
		sb.WriteString("count_by_type(Type, N) :-\n")
		sb.WriteString("    item(Type, _) |>\n")
		sb.WriteString("    do fn:group_by(Type),\n")
		sb.WriteString("    let N = fn:Count().\n")
		sb.WriteString("```\n\n")
	}

	if hasNegError {
		sb.WriteString("### Safe Negation:\n")
		sb.WriteString("```mangle\n")
		sb.WriteString("# WRONG: blocked(X) :- !permitted(X).  <- X unbound!\n")
		sb.WriteString("# CORRECT: blocked(X) :- action(X), !permitted(X).  <- X bound first\n")
		sb.WriteString("```\n\n")
	}

	return sb.String()
}

func (pb *PromptBuilder) buildFinalAttemptAdditions(ctx FeedbackContext) string {
	var sb strings.Builder

	sb.WriteString("\n## FINAL ATTEMPT - Simplification Required:\n\n")
	sb.WriteString("This is your last attempt. Please:\n")
	sb.WriteString("1. Use ONLY predicates from the 'Available Predicates' list\n")
	sb.WriteString("2. Keep the rule as simple as possible\n")
	sb.WriteString("3. If using aggregation, try rewriting WITHOUT aggregation\n")
	sb.WriteString("4. Ensure all variables in the head appear in positive body predicates\n")
	sb.WriteString("5. Double-check: use /atom not \"string\", ! not \\+, end with period\n\n")

	return sb.String()
}

// BuildInitialPromptAdditions returns syntax guidance to add to initial prompts.
func (pb *PromptBuilder) BuildInitialPromptAdditions(predicates []string) string {
	var sb strings.Builder

	sb.WriteString(pb.MangleSyntaxReminder)

	if len(predicates) > 0 {
		sb.WriteString("\n## Available Predicates:\n")
		// Group by category if possible, or just list
		maxDisplay := 50 // Don't overwhelm the prompt
		displayed := 0
		for _, pred := range predicates {
			if displayed >= maxDisplay {
				sb.WriteString(fmt.Sprintf("... and %d more\n", len(predicates)-maxDisplay))
				break
			}
			sb.WriteString("- ")
			sb.WriteString(pred)
			sb.WriteString("\n")
			displayed++
		}
	}

	sb.WriteString("\n## Example Rules:\n")
	sb.WriteString("```mangle\n")
	sb.WriteString("# Derive action based on state\n")
	sb.WriteString("next_action(/run_tests) :- test_state(/failing), !build_state(/broken).\n\n")
	sb.WriteString("# Strategy activation\n")
	sb.WriteString("active_strategy(/tdd_repair_loop) :- diagnostic(/error, _, _, _, _).\n\n")
	sb.WriteString("# Block commit on condition\n")
	sb.WriteString("block_commit(/unsafe_changes) :- modified(F), !test_coverage(F).\n")
	sb.WriteString("```\n")

	return sb.String()
}

// ExtractRuleFromResponse extracts the Mangle rule from an LLM response.
// It handles various response formats (markdown code blocks, plain text, etc.)
// Also sanitizes common LLM artifacts like "RULE:" prefixes that would cause parse errors.
func ExtractRuleFromResponse(response string) string {
	response = strings.TrimSpace(response)

	// First, handle structured output format: "RULE: <code>\nCONFIDENCE: ...\nRATIONALE: ..."
	// This format is used by autopoiesis prompts in executive.go and constitution.go
	if idx := strings.Index(response, "RULE:"); idx >= 0 {
		// Find the rule content after "RULE:"
		ruleStart := idx + len("RULE:")
		ruleContent := response[ruleStart:]

		// Find where the rule ends (at CONFIDENCE:, RATIONALE:, or end of string)
		endMarkers := []string{"CONFIDENCE:", "RATIONALE:", "\n\n"}
		ruleEnd := len(ruleContent)
		for _, marker := range endMarkers {
			if markerIdx := strings.Index(ruleContent, marker); markerIdx >= 0 && markerIdx < ruleEnd {
				ruleEnd = markerIdx
			}
		}

		rule := strings.TrimSpace(ruleContent[:ruleEnd])
		if rule != "" {
			return rule
		}
	}

	// Try to extract from markdown code block
	codeBlockPattern := "```mangle"
	if idx := strings.Index(response, codeBlockPattern); idx >= 0 {
		start := idx + len(codeBlockPattern)
		end := strings.Index(response[start:], "```")
		if end > 0 {
			return strings.TrimSpace(response[start : start+end])
		}
	}

	// Try generic code block
	codeBlockPattern = "```"
	if idx := strings.Index(response, codeBlockPattern); idx >= 0 {
		start := idx + len(codeBlockPattern)
		// Skip language identifier if present
		if newline := strings.Index(response[start:], "\n"); newline >= 0 {
			start += newline + 1
		}
		end := strings.Index(response[start:], "```")
		if end > 0 {
			return strings.TrimSpace(response[start : start+end])
		}
	}

	// Try to find rule pattern (head :- body.)
	if idx := strings.Index(response, ":-"); idx >= 0 {
		// Find the start (predicate name before the :-)
		start := idx
		for start > 0 && response[start-1] != '\n' && response[start-1] != '`' {
			start--
		}

		// Find the end (period)
		end := strings.Index(response[idx:], ".")
		if end > 0 {
			rule := strings.TrimSpace(response[start : idx+end+1])
			// Clean up any markdown artifacts
			rule = strings.TrimPrefix(rule, "`")
			rule = strings.TrimSuffix(rule, "`")
			return rule
		}
	}

	// Try to find fact pattern (just a predicate with .)
	if strings.Contains(response, "(") && strings.HasSuffix(response, ".") {
		return response
	}

	// Return as-is if nothing else worked
	return response
}

// ValidRuleExamples returns a list of valid rule examples for a given domain.
func ValidRuleExamples(domain string) []string {
	switch domain {
	case "executive", "action":
		return []string{
			"next_action(/run_tests) :- test_state(/failing).",
			"next_action(/fix_error) :- diagnostic(/error, _, _, _, _).",
			"active_strategy(/tdd_repair_loop) :- user_intent(_, _, /fix, _, _).",
		}
	case "constitution", "safety":
		return []string{
			"permitted(/read_file) :- safe_action(/read_file).",
			"permitted(A) :- dangerous_action(A), admin_override(_).",
			"block_commit(/unsafe) :- modified(F), !test_coverage(F).",
		}
	default:
		return []string{
			"result(X) :- source(X), condition(X).",
			"derived(X, Y) :- base(X, Y), !excluded(X).",
		}
	}
}
