package campaign

import (
	"fmt"
	"strings"
	"time"
)

// =============================================================================
// FORMATTING FOR LLM CONTEXT
// =============================================================================

// FormatForContext formats the intelligence report for LLM context injection.
func (r *IntelligenceReport) FormatForContext() string {
	var sb strings.Builder

	sb.WriteString("# INTELLIGENCE REPORT\n\n")
	sb.WriteString(fmt.Sprintf("Gathered: %s (took %v)\n\n", r.GatheredAt.Format(time.RFC3339), r.Duration))

	// Codebase Overview
	sb.WriteString("## Codebase Overview\n")
	sb.WriteString(fmt.Sprintf("- Files scanned: %d\n", len(r.FileTopology)))
	sb.WriteString(fmt.Sprintf("- Symbols indexed: %d\n", len(r.SymbolGraph)))
	if len(r.LanguageBreakdown) > 0 {
		sb.WriteString("- Languages: ")
		langs := make([]string, 0, len(r.LanguageBreakdown))
		for lang, count := range r.LanguageBreakdown {
			langs = append(langs, fmt.Sprintf("%s (%d)", lang, count))
		}
		sb.WriteString(strings.Join(langs, ", ") + "\n")
	}
	sb.WriteString("\n")

	// High Churn Files (Chesterton's Fence)
	if len(r.GitChurnHotspots) > 0 {
		sb.WriteString("## High Churn Files (Chesterton's Fence)\n")
		sb.WriteString("⚠️ These files change frequently. Understand WHY before modifying.\n\n")
		for i, h := range r.GitChurnHotspots {
			if i >= 10 {
				sb.WriteString(fmt.Sprintf("... and %d more\n", len(r.GitChurnHotspots)-10))
				break
			}
			sb.WriteString(fmt.Sprintf("- `%s`: %d changes\n", h.Path, h.ChurnRate))
		}
		sb.WriteString("\n")
	}

	// Historical Patterns
	if len(r.HistoricalPatterns) > 0 {
		sb.WriteString("## Learned Patterns\n")
		for i, p := range r.HistoricalPatterns {
			if i >= 10 {
				break
			}
			sb.WriteString(fmt.Sprintf("- %s (%.0f%% confidence)\n", p.Description, p.Confidence*100))
		}
		sb.WriteString("\n")
	}

	// Safety Warnings
	if len(r.SafetyWarnings) > 0 {
		sb.WriteString("## ⚠️ Safety Warnings\n")
		for _, w := range r.SafetyWarnings {
			sb.WriteString(fmt.Sprintf("- **%s**: %s (severity: %s)\n", w.Action, w.RuleViolated, w.Severity))
		}
		sb.WriteString("\n")
	}

	// Available Tools
	if len(r.MCPToolsAvailable) > 0 {
		sb.WriteString("## Available MCP Tools\n")
		for i, t := range r.MCPToolsAvailable {
			if i >= 10 {
				sb.WriteString(fmt.Sprintf("... and %d more tools\n", len(r.MCPToolsAvailable)-10))
				break
			}
			sb.WriteString(fmt.Sprintf("- `%s`: %s\n", t.Name, truncateField(t.Description, 2048)))
		}
		sb.WriteString("\n")
	}

	// Tool Gaps
	if len(r.ToolGaps) > 0 {
		sb.WriteString("## Tool Gaps Detected\n")
		for _, g := range r.ToolGaps {
			sb.WriteString(fmt.Sprintf("- %s: %s (confidence: %.0f%%)\n", g.Name, g.Purpose, g.Confidence*100))
		}
		sb.WriteString("\n")
	}

	// Expert Advice
	if r.AdvisorySummary != "" {
		sb.WriteString(truncateField(r.AdvisorySummary, 8192))
		sb.WriteString("\n")
	}

	// Test Coverage
	if len(r.UncoveredPaths) > 0 {
		sb.WriteString("## Test Coverage Gaps\n")
		for i, p := range r.UncoveredPaths {
			if i >= 10 {
				break
			}
			sb.WriteString(fmt.Sprintf("- %s\n", p))
		}
		sb.WriteString("\n")
	}

	// Architecture Hints
	if len(r.ArchitectureHints) > 0 {
		sb.WriteString("## Architecture Hints\n")
		for _, h := range r.ArchitectureHints {
			sb.WriteString(fmt.Sprintf("- %s\n", h))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
