package campaign

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// =============================================================================
// FORMATTING FOR LLM CONTEXT
// =============================================================================

// holographicUnreadLine names one target gather did not render and the tools
// that read it. package_outline lists the file's declarations; get_elements
// reads the file. Those are the same tools the holographic remainder lines
// name (internal/world/holographic.go).
func holographicUnreadLine(path string) string {
	return fmt.Sprintf("`%s` was not rendered; `package_outline` path=%s lists its declarations and `get_elements` reads the file", path, path)
}

// holographicCancelled is the operator-facing record of a gather that stopped
// before every target was rendered. The same paths are named on the report,
// because GatheringErrors is not what formatIntelligenceContext shows the model.
func holographicCancelled(rendered, total int, err error, unread []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "holographic context cancelled after %d/%d targets: %v", rendered, total, err)
	for _, path := range unread {
		b.WriteString("; ")
		b.WriteString(holographicUnreadLine(path))
	}
	return b.String()
}

// formatIntelligenceContext is the planning prompt's rendering of an intelligence
// report. buildPlanProposalContext is the production caller.
//
// A second formatter used to render a different section set, and it was the
// only one that carried holographic context, so the planner never saw the
// campaign's targets. This is the union: every section the planning prompt
// already injected, plus the codebase overview, target architecture, advisory
// summary, coverage gaps, and architecture hints the other one rendered.
func formatIntelligenceContext(intel *IntelligenceReport) string {
	if intel == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("INTELLIGENCE REPORT (from 12 systems):\n\n")
	sb.WriteString(fmt.Sprintf("Gathered: %s (took %v)\n\n", intel.GatheredAt.Format(time.RFC3339), intel.Duration))

	sb.WriteString("## Codebase Overview\n")
	sb.WriteString(fmt.Sprintf("- Files scanned: %d\n", len(intel.FileTopology)))
	sb.WriteString(fmt.Sprintf("- Symbols indexed: %d\n", len(intel.SymbolGraph)))
	if len(intel.LanguageBreakdown) > 0 {
		sb.WriteString("- Languages: ")
		langs := make([]string, 0, len(intel.LanguageBreakdown))
		for lang, count := range intel.LanguageBreakdown {
			langs = append(langs, fmt.Sprintf("%s (%d)", lang, count))
		}
		sort.Strings(langs)
		sb.WriteString(strings.Join(langs, ", ") + "\n")
	}
	sb.WriteString("\n")

	// Holographic context for the campaign's targets. Placed high because it is
	// the most decision-relevant section a decomposer reads: it says what the
	// target file offers, what its package holds, and who calls into it.
	// Each section is written whole — a character cap here would cut prose the
	// model reads. Paths gather stopped before rendering are named with the
	// tools that read them; a path that produced no section is not a withheld one.
	if len(intel.HolographicSections) > 0 || len(intel.HolographicUnread) > 0 {
		sb.WriteString("## Target Architecture\n\n")
		for _, hs := range intel.HolographicSections {
			sb.WriteString(hs.Section)
			sb.WriteString("\n")
		}
		if len(intel.HolographicUnread) > 0 {
			sb.WriteString("### Not rendered\n\n")
			for _, path := range intel.HolographicUnread {
				sb.WriteString("- ")
				sb.WriteString(holographicUnreadLine(path))
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
	}

	// Churn hotspots (Chesterton's Fence). Every hotspot is named. Stopping
	// at 10 and appending "and N more" left files the planner could not name.
	// Gather already keeps at most MaxChurnHotspots.
	if len(intel.GitChurnHotspots) > 0 {
		sb.WriteString("## HIGH-CHURN FILES (Chesterton's Fence - understand before modifying)\n")
		for _, ch := range intel.GitChurnHotspots {
			sb.WriteString(fmt.Sprintf("- %s: %d changes (%s)\n", ch.Path, ch.ChurnRate, ch.Reason))
		}
		sb.WriteString("\n")
	}

	// Historical patterns. Confidence >= 0.5 is the live selection rule, not a
	// count cap: lower-confidence patterns were never injected into the plan.
	if len(intel.HistoricalPatterns) > 0 {
		sb.WriteString("## LEARNED PATTERNS (from previous sessions)\n")
		for _, lp := range intel.HistoricalPatterns {
			if lp.Confidence >= 0.5 {
				sb.WriteString(fmt.Sprintf("- [%s] %s (%.0f%% confidence)\n", lp.ShardType, lp.Description, lp.Confidence*100))
			}
		}
		sb.WriteString("\n")
	}

	if len(intel.SafetyWarnings) > 0 {
		sb.WriteString("## SAFETY WARNINGS (constitutional pre-check)\n")
		for _, sw := range intel.SafetyWarnings {
			sb.WriteString(fmt.Sprintf("- %s on %s: blocked by rule '%s'\n", sw.Action, sw.Path, sw.RuleViolated))
		}
		sb.WriteString("\n")
	}

	// Every tool is named with its description whole. A 15-tool cap hid tools
	// the planner would then treat as missing. Gather already keeps at most
	// MaxMCPTools. Descriptions are not passed through truncateField.
	if len(intel.MCPToolsAvailable) > 0 {
		sb.WriteString("## AVAILABLE TOOLS (from MCP servers)\n")
		for _, mt := range intel.MCPToolsAvailable {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", mt.Name, mt.Description))
		}
		sb.WriteString("\n")
	}

	if len(intel.ToolGaps) > 0 {
		sb.WriteString("## TOOL GAPS (capabilities needed but not available)\n")
		for _, tg := range intel.ToolGaps {
			sb.WriteString(fmt.Sprintf("- %s: %s (confidence: %.0f%%)\n", tg.Name, tg.Purpose, tg.Confidence*100))
		}
		sb.WriteString("\n")
	}

	// Shard advice at confidence >= 0.6 is the live selection rule.
	if len(intel.ShardAdvice) > 0 {
		sb.WriteString("## EXPERT RECOMMENDATIONS\n")
		for _, sa := range intel.ShardAdvice {
			if sa.Confidence >= 0.6 {
				sb.WriteString(fmt.Sprintf("### %s (%.0f%% confidence)\n%s\n\n", sa.FromSpec, sa.Confidence*100, sa.Advice))
			}
		}
	}

	// AdvisorySummary quotes each response's advice whole. It used to keep
	// 200 characters of it; the planner reads this section and had no way
	// back to the rest.
	if intel.AdvisorySummary != "" {
		sb.WriteString(intel.AdvisorySummary)
		sb.WriteString("\n")
	}

	if len(intel.TestCoverage) > 0 {
		sb.WriteString("## TEST COVERAGE (by path)\n")
		lowCoverage := make([]string, 0)
		for path, cov := range intel.TestCoverage {
			if cov < 0.5 {
				lowCoverage = append(lowCoverage, fmt.Sprintf("- %s: %.0f%%", path, cov*100))
			}
		}
		// TestCoverage is a map: sorted so the prompt is byte-stable across runs.
		sort.Strings(lowCoverage)
		if len(lowCoverage) > 0 {
			sb.WriteString("Low coverage areas:\n")
			for _, lc := range lowCoverage {
				sb.WriteString(lc + "\n")
			}
		} else {
			sb.WriteString("All areas have adequate test coverage.\n")
		}
		sb.WriteString("\n")
	}

	if len(intel.UncoveredPaths) > 0 {
		sb.WriteString("## Test Coverage Gaps\n")
		for _, p := range intel.UncoveredPaths {
			sb.WriteString(fmt.Sprintf("- %s\n", p))
		}
		sb.WriteString("\n")
	}

	if len(intel.CodePatterns) > 0 {
		sb.WriteString("## DETECTED CODE PATTERNS\n")
		for _, cp := range intel.CodePatterns {
			files := ""
			if len(cp.Files) > 0 {
				files = strings.Join(cp.Files, ", ")
			}
			sb.WriteString(fmt.Sprintf("- %s in %s\n", cp.Name, files))
		}
		sb.WriteString("\n")
	}

	if len(intel.ArchitectureHints) > 0 {
		sb.WriteString("## Architecture Hints\n")
		for _, h := range intel.ArchitectureHints {
			sb.WriteString(fmt.Sprintf("- %s\n", h))
		}
		sb.WriteString("\n")
	}

	// The goal is the identity of the prior campaign, so it is written whole.
	// A fixed prefix cannot be recovered. The list itself is already bounded
	// at gather by MaxPreviousCampaigns.
	if len(intel.PreviousCampaigns) > 0 {
		sb.WriteString("## RELEVANT PREVIOUS CAMPAIGNS\n")
		for _, ca := range intel.PreviousCampaigns {
			status := fmt.Sprintf("failed (%.0f%%)", ca.SuccessRate*100)
			if ca.SuccessRate > 0.5 {
				status = fmt.Sprintf("succeeded (%.0f%%)", ca.SuccessRate*100)
			}
			sb.WriteString(fmt.Sprintf("- %s: %s - %s\n", ca.CampaignID, ca.Goal, status))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
