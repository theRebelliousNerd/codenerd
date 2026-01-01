package feedback

import (
	"regexp"
	"strings"
)

// PreValidator performs fast regex-based validation of LLM-generated Mangle code
// BEFORE expensive Mangle compilation. It catches common AI errors with specific
// feedback for retry attempts.
type PreValidator struct {
	patterns []compiledPattern
}

type compiledPattern struct {
	ErrorPattern
	regex *regexp.Regexp
}

// NewPreValidator creates a validator with all error detection patterns.
func NewPreValidator() *PreValidator {
	pv := &PreValidator{}
	pv.compilePatterns()
	return pv
}

// Validate checks code for common AI errors and returns any found.
func (pv *PreValidator) Validate(code string) []ValidationError {
	var errors []ValidationError

	lines := strings.Split(code, "\n")

	for lineNum, line := range lines {
		lineErrors := pv.validateLine(line, lineNum+1)
		errors = append(errors, lineErrors...)
	}

	// Global checks
	globalErrors := pv.validateGlobal(code)
	errors = append(errors, globalErrors...)

	return errors
}

func (pv *PreValidator) validateLine(line string, lineNum int) []ValidationError {
	var errors []ValidationError

	// Skip comments and empty lines
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return nil
	}

	for _, p := range pv.patterns {
		if p.regex == nil {
			continue
		}

		matches := p.regex.FindStringSubmatch(line)
		if len(matches) > 0 {
			wrong := matches[0]
			if len(matches) > 1 {
				wrong = matches[1] // Use first capture group if available
			}

			errors = append(errors, ValidationError{
				Category:   p.Category,
				Line:       lineNum,
				Message:    p.Message,
				Wrong:      wrong,
				Correct:    p.CorrectFix,
				Suggestion: p.Suggestion,
				AutoFixed:  p.AutoRepairable,
			})
		}
	}

	return errors
}

func (pv *PreValidator) validateGlobal(code string) []ValidationError {
	var errors []ValidationError

	// Check for rules without periods
	rulePattern := regexp.MustCompile(`([a-z_][a-z0-9_]*\s*\([^)]*\)\s*:-[^.]+)$`)
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check if line looks like a rule but doesn't end with period
		if strings.Contains(trimmed, ":-") && !strings.HasSuffix(trimmed, ".") {
			// Could be multi-line - check next lines
			isComplete := false
			for j := i + 1; j < len(lines) && j < i+5; j++ {
				nextTrimmed := strings.TrimSpace(lines[j])
				if strings.HasSuffix(nextTrimmed, ".") {
					isComplete = true
					break
				}
				if nextTrimmed == "" || strings.HasPrefix(nextTrimmed, "#") {
					break
				}
			}

			if !isComplete && rulePattern.MatchString(trimmed) {
				errors = append(errors, ValidationError{
					Category:   CategoryMissingPeriod,
					Line:       i + 1,
					Message:    "Rule must end with period",
					Wrong:      trimmed,
					Correct:    trimmed + ".",
					Suggestion: "Add a period (.) at the end of the rule",
					AutoFixed:  true,
				})
			}
		}
	}

	// Check for unbalanced parentheses
	openCount := strings.Count(code, "(")
	closeCount := strings.Count(code, ")")
	if openCount != closeCount {
		errors = append(errors, ValidationError{
			Category:   CategorySyntax,
			Line:       0,
			Message:    "Unbalanced parentheses",
			Wrong:      "",
			Correct:    "",
			Suggestion: "Check for missing or extra parentheses",
			AutoFixed:  false,
		})
	}

	return errors
}

func (pv *PreValidator) compilePatterns() {
	patterns := []ErrorPattern{
		// Atom vs String confusion - quoted lowercase identifiers
		{
			Category:       CategoryAtomString,
			Pattern:        `\(\s*[A-Z_]+\s*,\s*"([a-z][a-z0-9_]*)"\s*\)`,
			Message:        "String literal should be atom",
			WrongExample:   `status(X, "active")`,
			CorrectFix:     `status(X, /active)`,
			Suggestion:     `Replace "string" with /atom for identifiers, enums, and status values`,
			AutoRepairable: true,
		},
		{
			Category:       CategoryAtomString,
			Pattern:        `user_intent\(\s*[^,]+,\s*"([^"]+)"`,
			Message:        "user_intent category must be atom",
			WrongExample:   `user_intent(Id, "review", /fix, /codebase, _)`,
			CorrectFix:     `user_intent(Id, /review, /fix, /codebase, _)`,
			Suggestion:     "Use /atom for category, verb, and target fields in user_intent/5",
			AutoRepairable: true,
		},
		{
			Category:       CategoryAtomString,
			Pattern:        `user_intent\(\s*[^,]+,\s*[^,]+,\s*"([^"]+)"`,
			Message:        "user_intent verb must be atom",
			WrongExample:   `user_intent(Id, /review, "fix", /codebase, _)`,
			CorrectFix:     `user_intent(Id, /review, /fix, /codebase, _)`,
			Suggestion:     "Use /atom for category, verb, and target fields in user_intent/5",
			AutoRepairable: true,
		},
		{
			Category:       CategoryAtomString,
			Pattern:        `user_intent\(\s*[^,]+,\s*[^,]+,\s*[^,]+,\s*"([^"]+)"`,
			Message:        "user_intent target must be atom",
			WrongExample:   `user_intent(Id, /review, /fix, "codebase", _)`,
			CorrectFix:     `user_intent(Id, /review, /fix, /codebase, _)`,
			Suggestion:     "Use /atom for category, verb, and target fields in user_intent/5",
			AutoRepairable: true,
		},
		// Common enum-like strings that should be atoms
		{
			Category:       CategoryAtomString,
			Pattern:        `"(active|pending|done|enabled|disabled|open|closed|success|error|warning|critical|info|true|false|yes|no|on|off)"`,
			Message:        "Enum-like string should be atom",
			WrongExample:   `state(X, "pending")`,
			CorrectFix:     `state(X, /pending)`,
			Suggestion:     `Use /atom syntax for enum values, not "string"`,
			AutoRepairable: true,
		},
		// Prolog negation syntax - \\\+ matches literal \+
		{
			Category:       CategoryPrologNegation,
			Pattern:        `\\\+\s*([a-z_][a-z0-9_]*)\s*\(`,
			Message:        `Prolog negation \+ is invalid in Mangle`,
			WrongExample:   `\+ permitted(X)`,
			CorrectFix:     `!permitted(X)`,
			Suggestion:     `Use ! for negation in Mangle, not \+`,
			AutoRepairable: true,
		},
		// SQL-style aggregation
		{
			Category:       CategoryAggregation,
			Pattern:        `([A-Z][a-zA-Z0-9_]*)\s*=\s*(count|sum|min|max|avg)\s*\(`,
			Message:        "SQL-style aggregation is invalid",
			WrongExample:   `Total = sum(Amount)`,
			CorrectFix:     `source() |> do fn:group_by(Key), let Total = fn:sum(Amount)`,
			Suggestion:     "Use |> do fn:group_by(...), let Var = fn:Func() pipeline syntax",
			AutoRepairable: true,
		},
		// Missing 'do' keyword in pipeline
		{
			Category:       CategoryAggregation,
			Pattern:        `\|>\s*fn:(group_by|count|sum|min|max|avg)`,
			Message:        "Missing 'do' keyword before fn:",
			WrongExample:   `|> fn:group_by(X)`,
			CorrectFix:     `|> do fn:group_by(X)`,
			Suggestion:     "Add 'do' keyword: |> do fn:group_by(...)",
			AutoRepairable: true,
		},
		// Wrong function casing (capitalized aggregates)
		{
			Category:       CategoryAggregation,
			Pattern:        `fn:(Count|Sum|Min|Max|Avg)\s*\(`,
			Message:        "Wrong function casing - aggregates use lowercase",
			WrongExample:   `fn:Count()`,
			CorrectFix:     `fn:count()`,
			Suggestion:     "Use fn:count(), fn:sum(), fn:min(), fn:max(), fn:avg()",
			AutoRepairable: true,
		},
		// Soufflé-style declaration
		{
			Category:       CategorySyntax,
			Pattern:        `^\s*\.decl\s+`,
			Message:        "Soufflé .decl syntax is invalid in Mangle",
			WrongExample:   `.decl parent(x: string, y: string)`,
			CorrectFix:     `Decl parent(X.Type<string>, Y.Type<string>).`,
			Suggestion:     "Use Mangle declaration syntax: Decl predicate(Arg.Type<type>).",
			AutoRepairable: false,
		},
		// Direct struct field access
		{
			Category:       CategorySyntax,
			Pattern:        `([A-Z][a-zA-Z0-9_]*)\.([a-z][a-z0-9_]*)\s*[=)]`,
			Message:        "Direct field access is invalid - use :match_field",
			WrongExample:   `R.name = Name`,
			CorrectFix:     `:match_field(R, /name, Name)`,
			Suggestion:     "Use :match_field(Struct, /field, Value) for struct access",
			AutoRepairable: false,
		},
		// Struct literal in predicate argument
		{
			Category:       CategorySyntax,
			Pattern:        `[a-z_]+\s*\(\s*\{[^}]*\}\s*\)`,
			Message:        "Struct literal extraction is invalid",
			WrongExample:   `person({/name: Name})`,
			CorrectFix:     `person(R), :match_field(R, /name, Name)`,
			Suggestion:     "Bind struct to variable first, then use :match_field",
			AutoRepairable: false,
		},
		// Colon-prefixed atoms (Clojure/Scheme style)
		{
			Category:       CategoryAtomString,
			Pattern:        `[^:]:(active|pending|status|type|error|success|warning|info)\b`,
			Message:        "Colon-prefixed atom is invalid - use /atom",
			WrongExample:   `:active`,
			CorrectFix:     `/active`,
			Suggestion:     "Use /atom syntax, not :atom (Clojure/Scheme style)",
			AutoRepairable: true,
		},
		// Integer comparison on likely float field
		{
			Category:       CategoryTypeMismatch,
			Pattern:        `(confidence|score|similarity|probability|weight)\s*[><=]+\s*(\d+)\s*[,).]`,
			Message:        "Integer comparison on probable float field",
			WrongExample:   `Conf > 80`,
			CorrectFix:     `Conf > 0.8`,
			Suggestion:     "Confidence/score fields are typically floats (0.0-1.0), not integers",
			AutoRepairable: false,
		},
		// Negation as first term (unsafe pattern indicator)
		{
			Category:       CategoryUnboundNegation,
			Pattern:        `:-\s*!([a-z_][a-z0-9_]*)\s*\(`,
			Message:        "Unsafe: negation appears before positive binding",
			WrongExample:   `blocked(X) :- !permitted(X)`,
			CorrectFix:     `blocked(X) :- action(X), !permitted(X)`,
			Suggestion:     "Bind all variables with positive predicates BEFORE negation",
			AutoRepairable: true,
		},
		// NULL/UNKNOWN/UNDEFINED keywords
		{
			Category:       CategorySyntax,
			Pattern:        `\b(NULL|UNKNOWN|UNDEFINED|null|undefined|None|nil)\b`,
			Message:        "NULL/UNKNOWN concepts don't exist in Mangle (Closed World Assumption)",
			WrongExample:   `status(X, NULL)`,
			CorrectFix:     `unknown_status(X) :- item(X), !known_status(X, _)`,
			Suggestion:     "Mangle uses Closed World: facts either exist (true) or don't (false)",
			AutoRepairable: false,
		},
		// Case/when/else statements
		{
			Category:       CategorySyntax,
			Pattern:        `\b(case\s+when|when\s+.*\s+then|else\s+/[a-z])`,
			Message:        "Case/when/else statements don't exist in Mangle",
			WrongExample:   `case when X > 0 then /positive else /negative`,
			CorrectFix:     `positive(X) :- item(X), X > 0. negative(X) :- item(X), !positive(X).`,
			Suggestion:     "Use separate rules for each case",
			AutoRepairable: false,
		},
		// Findall/bagof/setof (Prolog predicates)
		{
			Category:       CategorySyntax,
			Pattern:        `\b(findall|bagof|setof)\s*\(`,
			Message:        "Prolog findall/bagof/setof don't exist in Mangle",
			WrongExample:   `findall(X, parent(X, Y), List)`,
			CorrectFix:     `Use aggregation: source() |> do fn:group_by(Y), let List = fn:Collect(X)`,
			Suggestion:     "Use Mangle aggregation transforms for collection operations",
			AutoRepairable: false,
		},
	}

	for _, p := range patterns {
		compiled := compiledPattern{ErrorPattern: p}
		if p.Pattern != "" {
			regex, err := regexp.Compile(p.Pattern)
			if err == nil {
				compiled.regex = regex
			}
		}
		pv.patterns = append(pv.patterns, compiled)
	}
}

// QuickFix attempts to auto-fix simple errors and returns the fixed code.
// Returns the original code if no fixes were applied.
func (pv *PreValidator) QuickFix(code string) string {
	fixed := code

	// Remove all backticks (markdown artifacts) - critical for LLM-generated rules
	fixed = strings.ReplaceAll(fixed, "`", "")

	// Convert quoted atoms to /atoms: "compile" -> /compile
	quotedAtomRegex := regexp.MustCompile(`"([a-z][a-z0-9_]*)"`)
	fixed = quotedAtomRegex.ReplaceAllString(fixed, "/$1")

	// Fix Prolog negation \+ -> ! (regex \\\+ matches literal \+)
	fixed = regexp.MustCompile(`\\\+\s*`).ReplaceAllString(fixed, "!")

	// Fix capitalized aggregation functions to lowercase (engine expects lowercase)
	fixed = regexp.MustCompile(`fn:Count\(`).ReplaceAllString(fixed, "fn:count(")
	fixed = regexp.MustCompile(`fn:Sum\(`).ReplaceAllString(fixed, "fn:sum(")
	fixed = regexp.MustCompile(`fn:Min\(`).ReplaceAllString(fixed, "fn:min(")
	fixed = regexp.MustCompile(`fn:Max\(`).ReplaceAllString(fixed, "fn:max(")
	fixed = regexp.MustCompile(`fn:Avg\(`).ReplaceAllString(fixed, "fn:avg(")

	// Fix missing 'do' keyword: |> fn: -> |> do fn:
	fixed = regexp.MustCompile(`\|>\s*fn:`).ReplaceAllString(fixed, "|> do fn:")

	// Fix colon atoms :atom -> /atom
	for _, word := range []string{"active", "pending", "status", "type", "error", "success", "warning", "info", "enabled", "disabled"} {
		pattern := regexp.MustCompile(`([^:]):` + word + `\b`)
		fixed = pattern.ReplaceAllString(fixed, "${1}/"+word)
	}

	return fixed
}

// GetPatterns returns all configured error patterns (for testing/debugging).
func (pv *PreValidator) GetPatterns() []ErrorPattern {
	patterns := make([]ErrorPattern, len(pv.patterns))
	for i, p := range pv.patterns {
		patterns[i] = p.ErrorPattern
	}
	return patterns
}
