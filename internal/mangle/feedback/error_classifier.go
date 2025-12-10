package feedback

import (
	"regexp"
	"strconv"
	"strings"
)

// ErrorClassifier parses Mangle compiler errors into structured ValidationErrors.
// It extracts line numbers, error types, and provides actionable suggestions.
type ErrorClassifier struct {
	patterns []classifierPattern
}

type classifierPattern struct {
	regex      *regexp.Regexp
	category   ErrorCategory
	msgBuilder func(matches []string) string
	suggestion string
	correct    string
}

// NewErrorClassifier creates a classifier for Mangle compiler errors.
func NewErrorClassifier() *ErrorClassifier {
	ec := &ErrorClassifier{}
	ec.compilePatterns()
	return ec
}

// Classify parses a Mangle error message and returns structured errors.
func (ec *ErrorClassifier) Classify(errMsg string) []ValidationError {
	var errors []ValidationError

	// Try each pattern
	for _, p := range ec.patterns {
		matches := p.regex.FindStringSubmatch(errMsg)
		if len(matches) > 0 {
			line, col := extractLineCol(errMsg)

			msg := errMsg
			if p.msgBuilder != nil {
				msg = p.msgBuilder(matches)
			}

			errors = append(errors, ValidationError{
				Category:   p.category,
				Line:       line,
				Column:     col,
				Message:    msg,
				Correct:    p.correct,
				Suggestion: p.suggestion,
			})
		}
	}

	// If no specific pattern matched, return generic parse error
	if len(errors) == 0 && errMsg != "" {
		line, col := extractLineCol(errMsg)
		errors = append(errors, ValidationError{
			Category:   CategoryParse,
			Line:       line,
			Column:     col,
			Message:    errMsg,
			Suggestion: "Check the syntax near the indicated line/column",
		})
	}

	return errors
}

// ClassifyWithContext adds code context to classified errors.
func (ec *ErrorClassifier) ClassifyWithContext(errMsg, code string) []ValidationError {
	errors := ec.Classify(errMsg)

	lines := strings.Split(code, "\n")
	for i := range errors {
		if errors[i].Line > 0 && errors[i].Line <= len(lines) {
			errors[i].Wrong = strings.TrimSpace(lines[errors[i].Line-1])
		}
	}

	return errors
}

func (ec *ErrorClassifier) compilePatterns() {
	ec.patterns = []classifierPattern{
		// Stratification error
		{
			regex:    regexp.MustCompile(`(?i)stratification|cyclic.*negation|cannot.*stratif|negative.*cycle`),
			category: CategoryStratification,
			msgBuilder: func(matches []string) string {
				return "Stratification violation: rule creates cyclic negation dependency"
			},
			suggestion: "Break the cycle by adding a base case or using a helper predicate without negation",
			correct:    "losing(X) :- position(X), !has_move(X). has_move(X) :- move(X, _).",
		},
		// Undeclared predicate
		{
			regex:    regexp.MustCompile(`(?i)undeclared.*predicate|unknown.*predicate|predicate.*not.*declared|no.*declaration.*for`),
			category: CategoryUndeclaredPredicate,
			msgBuilder: func(matches []string) string {
				return "Undeclared predicate: predicate used but not declared in schemas"
			},
			suggestion: "Use only predicates declared in schemas, or add a Decl statement",
		},
		// Parse error with line:col format
		{
			regex:    regexp.MustCompile(`(\d+):(\d+)\s*(?:parse error|syntax error|no viable alternative|expected|mismatched input|token recognition error)`),
			category: CategoryParse,
			msgBuilder: func(matches []string) string {
				return "Parse error at line " + matches[1] + ", column " + matches[2]
			},
			suggestion: "Check syntax near the indicated position - common issues: missing period, unbalanced parentheses, invalid characters",
		},
		// Expected base term
		{
			regex:    regexp.MustCompile(`expected base term got ([a-z_]+)\(`),
			category: CategorySyntax,
			msgBuilder: func(matches []string) string {
				return "Unexpected predicate call where base term expected: " + matches[1]
			},
			suggestion: "A base term (variable, constant, or atom) was expected, not a predicate call",
		},
		// Token recognition error (often backslash issues)
		{
			regex:    regexp.MustCompile(`token recognition error at: '([^']*)'`),
			category: CategorySyntax,
			msgBuilder: func(matches []string) string {
				return "Unrecognized token: " + matches[1]
			},
			suggestion: "Check for invalid characters or escape sequences (e.g., \\+ should be !)",
		},
		// No viable alternative
		{
			regex:    regexp.MustCompile(`no viable alternative at input '([^']*)'`),
			category: CategoryParse,
			msgBuilder: func(matches []string) string {
				return "Parser couldn't process: " + matches[1]
			},
			suggestion: "Check syntax around this text - ensure proper Mangle grammar",
		},
		// Mismatched input
		{
			regex:    regexp.MustCompile(`mismatched input '([^']*)' expecting '([^']*)'`),
			category: CategorySyntax,
			msgBuilder: func(matches []string) string {
				return "Found '" + matches[1] + "' but expected '" + matches[2] + "'"
			},
			suggestion: "Replace the mismatched token with the expected one",
		},
		// Safety violation (variable binding)
		{
			regex:    regexp.MustCompile(`(?i)unsafe.*variable|variable.*not.*bound|unbound.*variable`),
			category: CategoryUnboundNegation,
			msgBuilder: func(matches []string) string {
				return "Unsafe variable: variable appears in head or negation but not bound in positive body"
			},
			suggestion: "Ensure all variables in the head appear in at least one positive body predicate",
			correct:    "safe(X) :- source(X), !blocked(X)",
		},
		// Type mismatch
		{
			regex:    regexp.MustCompile(`(?i)type.*mismatch|incompatible.*types|expected.*type.*got`),
			category: CategoryTypeMismatch,
			msgBuilder: func(matches []string) string {
				return "Type mismatch in predicate arguments"
			},
			suggestion: "Check argument types match the predicate declaration (atom vs string, int vs float)",
		},
		// Arity mismatch
		{
			regex:    regexp.MustCompile(`(?i)arity.*mismatch|wrong.*number.*arguments|expected.*\d+.*arguments`),
			category: CategorySyntax,
			msgBuilder: func(matches []string) string {
				return "Wrong number of arguments to predicate"
			},
			suggestion: "Check the predicate declaration for correct arity",
		},
	}
}

// extractLineCol attempts to extract line and column numbers from an error message.
func extractLineCol(errMsg string) (line, col int) {
	// Pattern: "123:45" at start or after space
	lineColPattern := regexp.MustCompile(`(?:^|\s)(\d+):(\d+)`)
	matches := lineColPattern.FindStringSubmatch(errMsg)
	if len(matches) >= 3 {
		line, _ = strconv.Atoi(matches[1])
		col, _ = strconv.Atoi(matches[2])
		return line, col
	}

	// Pattern: "line 123" or "Line 123"
	linePattern := regexp.MustCompile(`(?i)line\s+(\d+)`)
	matches = linePattern.FindStringSubmatch(errMsg)
	if len(matches) >= 2 {
		line, _ = strconv.Atoi(matches[1])
		return line, 0
	}

	return 0, 0
}

// ExtractPredicateFromError attempts to extract a predicate name from an error.
func ExtractPredicateFromError(errMsg string) string {
	// Pattern: predicate name followed by (
	predPattern := regexp.MustCompile(`'?([a-z_][a-z0-9_]*)\s*\(`)
	matches := predPattern.FindStringSubmatch(errMsg)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// FormatErrorForFeedback formats a validation error for LLM feedback.
func FormatErrorForFeedback(err ValidationError) string {
	var sb strings.Builder

	if err.Line > 0 {
		sb.WriteString("LINE ")
		sb.WriteString(strconv.Itoa(err.Line))
		sb.WriteString(": ")
	}

	if err.Wrong != "" {
		sb.WriteString(err.Wrong)
		sb.WriteString("\n\n")
	}

	sb.WriteString("PROBLEM: ")
	sb.WriteString(err.Message)
	sb.WriteString("\n")

	if err.Wrong != "" && err.Correct != "" {
		sb.WriteString("WRONG:   ")
		sb.WriteString(err.Wrong)
		sb.WriteString("\n")
		sb.WriteString("CORRECT: ")
		sb.WriteString(err.Correct)
		sb.WriteString("\n")
	}

	if err.Suggestion != "" {
		sb.WriteString("\nFIX: ")
		sb.WriteString(err.Suggestion)
	}

	return sb.String()
}
