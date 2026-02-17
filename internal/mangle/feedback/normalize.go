package feedback

import (
	"regexp"
	"strconv"
	"strings"
)

// prologNegationRe matches Prolog-style negation \+ followed by predicate
var prologNegationRe = regexp.MustCompile(`\\\+\s*`)

// NormalizeRuleInput prepares a rule for parsing by fixing common escape issues
// in LLM output. It fixes:
// 1. Prolog-style negation (\+) to Mangle negation (!)
// 2. Backslashes inside quoted strings
func NormalizeRuleInput(rule string) string {
	rule = strings.TrimSpace(rule)

	// If the entire rule is a JSON-quoted string, unquote it first.
	if strings.HasPrefix(rule, "\"") && strings.HasSuffix(rule, "\"") {
		if unquoted, err := strconv.Unquote(rule); err == nil {
			rule = strings.TrimSpace(unquoted)
		}
	}

	// First fix Prolog negation \+ -> !
	// This is a common LLM mistake when generating Mangle rules
	if strings.Contains(rule, "\\+") {
		rule = prologNegationRe.ReplaceAllString(rule, "!")
	}

	// Then handle backslashes in quoted strings
	if !strings.Contains(rule, "\\") || !strings.Contains(rule, "\"") {
		return rule
	}

	var sb strings.Builder
	sb.Grow(len(rule))

	inQuote := false
	for i := 0; i < len(rule); i++ {
		c := rule[i]

		if c == '"' {
			if i == 0 || rule[i-1] != '\\' {
				inQuote = !inQuote
			}
			sb.WriteByte(c)
			continue
		}

		if inQuote && c == '\\' {
			if i+1 < len(rule) && isKnownEscape(rule[i+1]) {
				sb.WriteByte('\\')
				sb.WriteByte(rule[i+1])
				i++
				continue
			}
			sb.WriteString("\\\\")
			continue
		}

		sb.WriteByte(c)
	}

	return sb.String()
}

func isKnownEscape(b byte) bool {
	switch b {
	case '\\', '"', 'n', 'r', 't', 'b', 'f', '0':
		return true
	default:
		return false
	}
}
