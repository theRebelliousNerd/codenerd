package feedback

import "strings"

// NormalizeRuleInput prepares a rule for parsing by fixing common escape issues
// in LLM output. It only touches backslashes inside quoted strings so the
// logical structure of the rule remains unchanged.
func NormalizeRuleInput(rule string) string {
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
