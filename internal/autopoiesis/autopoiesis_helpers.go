package autopoiesis

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"codenerd/internal/logging"
)

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// shouldCheckToolNeed determines if we should check for tool needs
func shouldCheckToolNeed(input string) bool {
	// Check for explicit tool need indicators
	for _, pattern := range missingCapabilityPatterns {
		if pattern.MatchString(input) {
			return true
		}
	}
	return false
}

// shouldGenerateToolNeed applies a conservative, evidence-based gate before
// scheduling Ouroboros tool generation. This protects general-purpose runs
// from expensive or distracting tool creation unless clearly warranted.
func (o *Orchestrator) shouldGenerateToolNeed(need *ToolNeed) bool {
	if need == nil {
		return false
	}

	// Baseline confidence gate (shared with other autopoiesis actions).
	if need.Confidence < o.config.MinConfidence {
		logging.AutopoiesisDebug("Tool need below MinConfidence: %.2f < %.2f", need.Confidence, o.config.MinConfidence)
		return false
	}

	strongEvidence := hasStrongToolEvidence(need)
	if need.Confidence < o.config.MinToolConfidence && !strongEvidence {
		logging.AutopoiesisDebug("Tool need gated: confidence %.2f below MinToolConfidence %.2f without strong evidence",
			need.Confidence, o.config.MinToolConfidence)
		return false
	}

	// Avoid duplicate generation.
	if o.toolGen != nil && o.toolGen.HasTool(need.Name) {
		logging.AutopoiesisDebug("Tool need gated: tool already exists (%s)", need.Name)
		return false
	}

	// Session-local cap.
	if o.config.MaxToolsPerSession > 0 && o.toolsGenerated >= o.config.MaxToolsPerSession {
		logging.AutopoiesisDebug("Tool need gated: MaxToolsPerSession reached (%d)", o.config.MaxToolsPerSession)
		return false
	}

	// Cooldown between generations unless strong evidence overrides.
	if o.config.ToolGenerationCooldown > 0 && !o.lastToolGen.IsZero() && !strongEvidence {
		if time.Since(o.lastToolGen) < o.config.ToolGenerationCooldown {
			logging.AutopoiesisDebug("Tool need gated: cooldown active (%v)", o.config.ToolGenerationCooldown)
			return false
		}
	}

	return true
}

func hasStrongToolEvidence(need *ToolNeed) bool {
	// Strong evidence includes failed attempts or multiple independent triggers.
	for _, t := range need.Triggers {
		lower := strings.ToLower(t)
		if strings.Contains(lower, "previous attempt failed") || strings.Contains(lower, "failed") {
			return true
		}
	}
	return len(need.Triggers) >= 2
}

// sortActionsByPriority sorts actions by priority (highest first)
func sortActionsByPriority(actions []AutopoiesisAction) {
	for i := 0; i < len(actions); i++ {
		for j := i + 1; j < len(actions); j++ {
			if actions[j].Priority > actions[i].Priority {
				actions[i], actions[j] = actions[j], actions[i]
			}
		}
	}
}

// hashString creates a simple hash of a string
func hashString(s string) string {
	// Simple hash for deduplication
	h := uint32(0)
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return fmt.Sprintf("%08x", h)
}

// complexityLevelString returns string representation of complexity level
func complexityLevelString(level ComplexityLevel) string {
	switch level {
	case ComplexitySimple:
		return "Simple"
	case ComplexityModerate:
		return "Moderate"
	case ComplexityComplex:
		return "Complex"
	case ComplexityEpic:
		return "Epic"
	default:
		return "Unknown"
	}
}

// truncateCode truncates code for LLM prompts while preserving structure
func truncateCode(code string, maxLen int) string {
	if len(code) <= maxLen {
		return code
	}
	// Keep the beginning and note truncation
	return code[:maxLen] + "\n// ... (truncated)"
}

// extractJSON extracts JSON from a potentially mixed-format response
func extractJSON(text string) string {
	// Try to find JSON object or array
	start := strings.Index(text, "{")
	if start == -1 {
		start = strings.Index(text, "[")
	}
	if start == -1 {
		return "{}" // Return empty object when no JSON found
	}

	// Find matching closing brace/bracket
	depth := 0
	inString := false
	escaped := false
	startChar := rune(text[start])
	endChar := '}'
	if startChar == '[' {
		endChar = ']'
	}

	for i := start; i < len(text); i++ {
		ch := rune(text[i])

		if escaped {
			escaped = false
			continue
		}

		if ch == '\\' {
			escaped = true
			continue
		}

		if ch == '"' {
			inString = !inString
			continue
		}

		if !inString {
			if ch == startChar || ch == '{' || ch == '[' {
				depth++
			} else if ch == endChar || ch == '}' || ch == ']' {
				depth--
				if depth == 0 {
					return text[start : i+1]
				}
			}
		}
	}

	return ""
}

// missingCapabilityPatterns are regex patterns that indicate tool needs
var missingCapabilityPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)no tool (for|to)`),
	regexp.MustCompile(`(?i)missing capability`),
	regexp.MustCompile(`(?i)need a tool (for|to)`),
	regexp.MustCompile(`(?i)create a tool (for|to)`),
	regexp.MustCompile(`(?i)generate a tool (for|to)`),
	// Question patterns suggesting capability gaps
	regexp.MustCompile(`(?i)can'?t (you|we|i)\s+\w+`),   // "Can't you validate..."
	regexp.MustCompile(`(?i)is there a way (to|for)`),   // "Is there a way to..."
	regexp.MustCompile(`(?i)how do (i|we|you)\s+\w+`),   // "How do I validate..."
}
