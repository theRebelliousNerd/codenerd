package chat

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/northstar"

	tea "github.com/charmbracelet/bubbletea"
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

// isDreamExecutionTrigger checks if user wants to execute the dream plan.
// DISTINCT from isDreamConfirmation (which persists learnings, not executes).
func isDreamExecutionTrigger(input string) bool {
	lower := strings.ToLower(input)
	triggers := []string{
		"do it", "execute that", "run the plan", "go ahead",
		"make it so", "proceed", "execute the plan", "run that",
		"let's do it", "implement that", "start execution",
		"yes, do it", "yes, execute", "carry it out", "perform that",
	}
	for _, t := range triggers {
		if strings.Contains(lower, t) {
			return true
		}
	}
	return false
}

func isAffirmativeResponse(input string) bool {
	lower := strings.ToLower(strings.TrimSpace(input))
	if lower == "" {
		return false
	}
	triggers := []string{
		"/learn_yes",
		"yes", "y", "yeah", "yep", "sure", "ok", "okay",
		"correct", "learn this", "confirm", "do it",
	}
	for _, t := range triggers {
		if strings.Contains(lower, t) {
			return true
		}
	}
	return false
}

func isNegativeResponse(input string) bool {
	lower := strings.ToLower(strings.TrimSpace(input))
	if lower == "" {
		return false
	}
	triggers := []string{
		"/learn_no",
		"no", "n", "nope", "nah", "don't", "do not", "never",
		"reject", "skip", "not now",
	}
	for _, t := range triggers {
		if strings.Contains(lower, t) {
			return true
		}
	}
	return false
}

func escapeMangleString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

func normalizeVerbAtom(verb string) string {
	v := strings.TrimSpace(verb)
	if v == "" || v == "none" || v == "_" {
		return ""
	}
	if !strings.HasPrefix(v, "/") {
		return "/" + v
	}
	return v
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

// runAlignmentCheck runs a Northstar alignment check and returns the result.
func (m *Model) runAlignmentCheck(subject string) tea.Cmd {
	return func() tea.Msg {
		// Get nerd directory
		nerdDir := filepath.Join(m.workspace, ".nerd")

		// Load the Northstar store
		store, err := northstar.NewStore(nerdDir)
		if err != nil {
			return alignmentCheckMsg{
				Subject: subject,
				Result:  "error",
				Err:     err,
			}
		}
		defer store.Close()

		// Create guardian with default config
		config := northstar.DefaultGuardianConfig()
		guardian := northstar.NewGuardian(store, config)

		// Set LLM client if available
		if m.client != nil {
			guardian.SetLLMClient(m.client)
		}

		// Initialize the guardian
		if err := guardian.Initialize(); err != nil {
			return alignmentCheckMsg{
				Subject: subject,
				Result:  "error",
				Err:     err,
			}
		}

		// Check if vision is defined
		if !guardian.HasVision() {
			return alignmentCheckMsg{
				Subject:     subject,
				Result:      "skipped",
				Score:       1.0,
				Explanation: "No vision defined. Use /northstar to define your project vision first.",
			}
		}

		// Build context from session state
		contextStr := m.buildAlignmentContext()

		// Run alignment check with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		check, err := guardian.CheckAlignment(ctx, northstar.TriggerManual, subject, contextStr)
		if err != nil {
			return alignmentCheckMsg{
				Subject: subject,
				Result:  "error",
				Err:     err,
			}
		}

		return alignmentCheckMsg{
			Subject:     check.Subject,
			Result:      string(check.Result),
			Score:       check.Score,
			Explanation: check.Explanation,
			Suggestions: check.Suggestions,
		}
	}
}

// buildAlignmentContext builds context for alignment check from current session state.
func (m *Model) buildAlignmentContext() string {
	var sb strings.Builder

	// Add recent conversation context
	if len(m.history) > 0 {
		sb.WriteString("## Recent Conversation\n")
		// Last 3 messages
		start := len(m.history) - 3
		if start < 0 {
			start = 0
		}
		for i := start; i < len(m.history); i++ {
			msg := m.history[i]
			sb.WriteString(msg.Role + ": " + truncateStr(msg.Content, 200) + "\n")
		}
		sb.WriteString("\n")
	}

	// Add last shard result if available
	if m.lastShardResult != nil {
		sb.WriteString("## Recent Task\n")
		sb.WriteString("Type: " + m.lastShardResult.ShardType + "\n")
		sb.WriteString("Task: " + m.lastShardResult.Task + "\n")
		sb.WriteString("\n")
	}

	// Add active campaign info if running
	if m.activeCampaign != nil {
		sb.WriteString("## Active Campaign\n")
		sb.WriteString("Goal: " + m.activeCampaign.Goal + "\n")
		if m.campaignProgress != nil {
			sb.WriteString("Phase: " + m.campaignProgress.CurrentPhase + "\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// truncateStr truncates a string to the given length with ellipsis.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// formatAlignmentCheckResult formats an alignment check result for display.
func (m *Model) formatAlignmentCheckResult(msg alignmentCheckMsg) string {
	var sb strings.Builder
	sb.WriteString("## Northstar Alignment Check\n\n")

	// Result emoji and status
	var emoji, color string
	switch msg.Result {
	case "passed":
		emoji = "✅"
		color = "green"
	case "warning":
		emoji = "⚠️"
		color = "yellow"
	case "failed":
		emoji = "❌"
		color = "red"
	case "blocked":
		emoji = "🚫"
		color = "red"
	case "skipped":
		emoji = "⏭️"
		color = "gray"
	default:
		emoji = "❓"
		color = "gray"
	}
	_ = color // For future styling

	sb.WriteString("**Subject:** " + msg.Subject + "\n\n")
	sb.WriteString("**Result:** " + emoji + " " + strings.ToUpper(msg.Result) + "\n\n")
	sb.WriteString("**Alignment Score:** " + formatScore(msg.Score) + "\n\n")

	if msg.Explanation != "" {
		sb.WriteString("**Explanation:** " + msg.Explanation + "\n\n")
	}

	if len(msg.Suggestions) > 0 {
		sb.WriteString("**Suggestions:**\n")
		for _, s := range msg.Suggestions {
			sb.WriteString("- " + s + "\n")
		}
		sb.WriteString("\n")
	}

	// Add hints based on result
	switch msg.Result {
	case "skipped":
		sb.WriteString("*Tip: Run `/northstar` to define your project vision.*\n")
	case "warning":
		sb.WriteString("*Consider reviewing the suggestions above to improve alignment.*\n")
	case "failed", "blocked":
		sb.WriteString("*This indicates significant drift from your project vision. Review the suggestions carefully.*\n")
	}

	return sb.String()
}

// formatScore formats a 0.0-1.0 score as a percentage with visual bar.
func formatScore(score float64) string {
	pct := int(score * 100)

	// Visual bar (10 chars)
	filled := pct / 10
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 10-filled)

	return bar + " " + fmt.Sprintf("%d%%", pct)
}
