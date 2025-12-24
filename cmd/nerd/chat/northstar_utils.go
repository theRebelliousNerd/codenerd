// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains utility and parsing functions for the Northstar wizard.
package chat

import (
	"os"
	"path/filepath"
	"strings"
)

// =============================================================================
// NORTHSTAR UTILITY FUNCTIONS
// =============================================================================

// getNorthstarWelcomeMessage returns the welcome message for the wizard
func getNorthstarWelcomeMessage() string {
	return `# NORTHSTAR WIZARD

Welcome to the **Northstar Definition Process**.

This wizard will guide you through defining your project's:
1. **Problem Statement** - What pain are you solving?
2. **Vision** - What does success look like?
3. **Target Users** - Who are you building for?
4. **Capabilities** - What will it do?
5. **Red Teaming** - What could go wrong?
6. **Requirements** - What must be built?
7. **Constraints** - What are the limits?

Your answers will be stored in:
- ` + "`.nerd/northstar.mg`" + ` (Mangle facts for reasoning)
- ` + "`.nerd/northstar.json`" + ` (JSON backup)
- Knowledge database (for semantic search)

---

**Do you have research documents to ingest first?** (yes/no)

_Examples: spec files, design docs, market research, competitor analysis_`
}

// parseFilePaths parses file paths from comma or newline separated input
func parseFilePaths(input string) []string {
	input = strings.ReplaceAll(input, "\n", ",")
	parts := strings.Split(input, ",")
	var paths []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

// expandPath expands a path relative to workspace or home directory
func expandPath(workspace, path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workspace, path)
}

// splitAndTrim splits input by newlines or commas and trims whitespace
func splitAndTrim(input string) []string {
	// Handle both newlines and commas
	input = strings.ReplaceAll(input, "\n", ",")
	parts := strings.Split(input, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// parseTimeline parses timeline input into a standard value
func parseTimeline(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	switch lower {
	case "1", "now", "core", "mvp":
		return "now"
	case "2", "6", "6mo", "6 months":
		return "6mo"
	case "3", "1yr", "1 year", "year":
		return "1yr"
	case "4", "3yr", "3 years", "3+":
		return "3yr"
	case "5", "moonshot", "dream":
		return "moonshot"
	default:
		return lower
	}
}

// parsePriority parses priority input into a standard value
func parsePriority(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	switch lower {
	case "1", "critical", "must":
		return "critical"
	case "2", "high", "should":
		return "high"
	case "3", "medium", "nice":
		return "medium"
	case "4", "low", "someday":
		return "low"
	default:
		return "medium"
	}
}

// parseLikelihood parses likelihood input into a standard value
func parseLikelihood(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	switch lower {
	case "1", "high", "h", "likely":
		return "high"
	case "2", "medium", "m", "moderate":
		return "medium"
	case "3", "low", "l", "unlikely":
		return "low"
	default:
		return "medium"
	}
}

// parseReqType parses requirement type input into a standard value
func parseReqType(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	switch lower {
	case "1", "functional", "func", "f":
		return "functional"
	case "2", "non-functional", "nonfunctional", "nf", "quality":
		return "non-functional"
	case "3", "constraint", "c", "limit":
		return "constraint"
	default:
		return "functional"
	}
}

// parseReqPriority parses requirement priority input into a standard value
func parseReqPriority(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	switch lower {
	case "1", "must", "must-have", "critical":
		return "must-have"
	case "2", "should", "should-have", "important":
		return "should-have"
	case "3", "nice", "nice-to-have", "optional":
		return "nice-to-have"
	default:
		return "should-have"
	}
}

// truncateWithEllipsis truncates a string and adds ellipsis if needed
func truncateWithEllipsis(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
