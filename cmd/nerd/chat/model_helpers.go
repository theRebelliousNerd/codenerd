package chat

import (
	"strings"
)

// extractFindings parses findings from shard output (reviewer/tester results).
// Looks for structured patterns like "- [ERROR] file:line: message"
func extractFindings(result string) []map[string]any {
	var findings []map[string]any
	// Simple line-based extraction - look for patterns like "- [ERROR] file:line: message"
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- [") || strings.HasPrefix(line, "• [") ||
			strings.Contains(line, "[WARN]") || strings.Contains(line, "[INFO]") ||
			strings.Contains(line, "[CRIT]") || strings.Contains(line, "[ERR]") {
			finding := map[string]any{
				"raw": line,
			}
			// Extract severity
			if strings.Contains(line, "[CRIT]") || strings.Contains(line, "[CRITICAL]") {
				finding["severity"] = "critical"
			} else if strings.Contains(line, "[ERR]") || strings.Contains(line, "[ERROR]") {
				finding["severity"] = "error"
			} else if strings.Contains(line, "[WARN]") || strings.Contains(line, "[WARNING]") {
				finding["severity"] = "warning"
			} else if strings.Contains(line, "[INFO]") {
				finding["severity"] = "info"
			}
			findings = append(findings, finding)
		}
	}
	return findings
}

// extractMetrics parses metrics section from output.
// Looks for patterns like "Key: Value" or "Key = Value"
func extractMetrics(result string) map[string]any {
	metrics := make(map[string]any)
	// Look for common metric patterns
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "lines") || strings.Contains(line, "functions") ||
			strings.Contains(line, "complexity") || strings.Contains(line, "nesting") {
			// Parse "Key: Value" or "Key = Value" patterns
			for _, sep := range []string{": ", "= ", "="} {
				if strings.Contains(line, sep) {
					parts := strings.SplitN(line, sep, 2)
					if len(parts) == 2 {
						key := strings.TrimSpace(parts[0])
						value := strings.TrimSpace(parts[1])
						metrics[key] = value
					}
					break
				}
			}
		}
	}
	return metrics
}

// hardWrap wraps text at the given width, splitting long lines.
func hardWrap(s string, width int) string {
	if width < 1 || s == "" {
		return s
	}

	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		remaining := []rune(line)
		for len(remaining) > width {
			out = append(out, string(remaining[:width]))
			remaining = remaining[width:]
		}
		out = append(out, string(remaining))
	}
	return strings.Join(out, "\n")
}

// refreshErrorViewport updates the error viewport content with wrapped text.
func (m *Model) refreshErrorViewport() {
	if m.err == nil {
		m.errorVP.SetContent("")
		return
	}
	width := m.errorVP.Width
	if width < 1 {
		width = 1
	}
	m.errorVP.SetContent(hardWrap(m.err.Error(), width))
}

// extractClarificationQuestion extracts the question from an error message
func extractClarificationQuestion(errMsg string) string {
	// Look for "USER_INPUT_REQUIRED:" prefix
	if idx := strings.Index(errMsg, "USER_INPUT_REQUIRED:"); idx != -1 {
		return strings.TrimSpace(errMsg[idx+len("USER_INPUT_REQUIRED:"):])
	}
	// Fallback: return the full error message
	return errMsg
}

// isNegativeFeedback checks for common frustration signals
func isNegativeFeedback(input string) bool {
	lower := strings.ToLower(input)
	triggers := []string{
		"bad bot", "wrong", "stop", "no that's not right",
		"you didn't", "fail", "incorrect", "mistake",
	}
	for _, t := range triggers {
		if strings.Contains(lower, t) {
			return true
		}
	}
	return false
}

// isDreamConfirmation checks if user is confirming dream state learnings
func isDreamConfirmation(input string) bool {
	lower := strings.ToLower(input)
	triggers := []string{
		"correct!", "correct", "learn this", "learn that", "remember this",
		"remember that", "yes, do that", "that's right", "exactly!",
		"yes!", "perfect", "good approach", "sounds right",
	}
	for _, t := range triggers {
		if strings.Contains(lower, t) {
			return true
		}
	}
	return false
}

// isDreamCorrection checks if user is correcting dream state learnings
func isDreamCorrection(input string) bool {
	lower := strings.ToLower(input)
	triggers := []string{
		"no, actually", "actually, we", "wrong, we", "instead, we",
		"not that way", "we don't", "we always", "remember:",
		"learn:", "actually:",
	}
	for _, t := range triggers {
		if strings.Contains(lower, t) {
			return true
		}
	}
	return false
}

// extractCorrectionContent extracts the corrective content from user input
func extractCorrectionContent(input string) string {
	lower := strings.ToLower(input)

	// Try to find correction markers
	markers := []string{
		"no, actually", "actually, we", "wrong, we", "instead, we",
		"remember:", "learn:", "actually:",
	}

	for _, marker := range markers {
		if idx := strings.Index(lower, marker); idx != -1 {
			// Extract everything after the marker
			content := strings.TrimSpace(input[idx+len(marker):])
			if content != "" {
				return content
			}
		}
	}

	// Fallback: return the full input
	return input
}
