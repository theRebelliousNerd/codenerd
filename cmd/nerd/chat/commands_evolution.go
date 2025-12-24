// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains evolution-related helper methods for the Prompt Evolution System (SPL).
//
// Commands (defined in commands.go):
//   /evolve          - Trigger manual evolution cycle
//   /evolution-stats - Show evolution statistics
//   /evolved-atoms   - List evolved atoms
//   /promote-atom    - Promote pending atom to corpus
//   /reject-atom     - Reject evolved atom
//   /strategies      - Show strategy database
package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	pe "codenerd/internal/autopoiesis/prompt_evolution"

	tea "github.com/charmbracelet/bubbletea"
)

// evolutionResultMsg is sent when an evolution cycle completes.
type evolutionResultMsg struct {
	result *pe.EvolutionResult
	err    error
}

// runEvolutionCycle triggers an async evolution cycle.
func (m Model) runEvolutionCycle() tea.Cmd {
	return func() tea.Msg {
		if m.promptEvolver == nil {
			return evolutionResultMsg{err: fmt.Errorf("prompt evolver not initialized")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		result, err := m.promptEvolver.RunEvolutionCycle(ctx)
		return evolutionResultMsg{result: result, err: err}
	}
}

// renderEvolutionStats generates a formatted display of evolution statistics.
func (m Model) renderEvolutionStats() string {
	if m.promptEvolver == nil {
		return "Prompt Evolution system not initialized.\n\nEnable it in config with `evolution.enabled: true`"
	}

	stats := m.promptEvolver.GetStats()

	var sb strings.Builder
	sb.WriteString("## Prompt Evolution Statistics\n\n")

	// Overall stats
	sb.WriteString("### Overview\n")
	sb.WriteString(fmt.Sprintf("- **Evolution Cycles**: %d (%d successful)\n", stats.TotalCycles, stats.SuccessfulCycles))
	sb.WriteString(fmt.Sprintf("- **Last Evolution**: %s\n", formatTimeAgo(stats.LastEvolutionAt)))
	sb.WriteString(fmt.Sprintf("- **Executions Recorded**: %d\n", stats.TotalExecutionsRecorded))
	sb.WriteString(fmt.Sprintf("- **Failures Analyzed**: %d\n", stats.TotalFailuresAnalyzed))
	sb.WriteString(fmt.Sprintf("- **Success Rate**: %.1f%%\n", stats.OverallSuccessRate*100))
	sb.WriteString("\n")

	// Atom generation stats
	sb.WriteString("### Atom Generation\n")
	sb.WriteString(fmt.Sprintf("- **Atoms Generated**: %d\n", stats.TotalAtomsGenerated))
	sb.WriteString(fmt.Sprintf("- **Pending Promotion**: %d\n", stats.AtomsPending))
	sb.WriteString(fmt.Sprintf("- **Promoted to Corpus**: %d\n", stats.AtomsPromoted))
	sb.WriteString(fmt.Sprintf("- **Rejected**: %d\n", stats.AtomsRejected))
	sb.WriteString("\n")

	// Strategy stats
	sb.WriteString("### Strategy Database\n")
	sb.WriteString(fmt.Sprintf("- **Total Strategies**: %d\n", stats.TotalStrategies))
	sb.WriteString(fmt.Sprintf("- **Avg Strategy Success**: %.1f%%\n", stats.AvgStrategySuccessRate*100))
	if stats.AvgCycleDuration > 0 {
		sb.WriteString(fmt.Sprintf("- **Avg Cycle Duration**: %s\n", stats.AvgCycleDuration.Round(time.Millisecond)))
	}

	return sb.String()
}

// renderEvolvedAtoms lists all evolved atoms with their status.
func (m Model) renderEvolvedAtoms() string {
	if m.promptEvolver == nil {
		return "Prompt Evolution system not initialized."
	}

	atoms := m.promptEvolver.GetEvolvedAtoms()
	if len(atoms) == 0 {
		return "No evolved atoms yet.\n\nRun `/evolve` to trigger an evolution cycle after some task executions."
	}

	var sb strings.Builder
	sb.WriteString("## Evolved Atoms\n\n")

	// Group by status
	pending := make([]*pe.GeneratedAtom, 0)
	promoted := make([]*pe.GeneratedAtom, 0)

	for _, atom := range atoms {
		if !atom.PromotedAt.IsZero() {
			promoted = append(promoted, atom)
		} else {
			pending = append(pending, atom)
		}
	}

	if len(pending) > 0 {
		sb.WriteString("### Pending Promotion\n")
		for _, atom := range pending {
			sb.WriteString(fmt.Sprintf("- **%s** (confidence: %.0f%%)\n", atom.Atom.ID, atom.Confidence*100))
			sb.WriteString(fmt.Sprintf("  Source: %s | Uses: %d\n", atom.Source, atom.UsageCount))
			// Truncate content for display
			content := strings.TrimSpace(atom.Atom.Content)
			if len(content) > 100 {
				content = content[:100] + "..."
			}
			sb.WriteString(fmt.Sprintf("  `%s`\n", content))
		}
		sb.WriteString("\n")
	}

	if len(promoted) > 0 {
		sb.WriteString("### Promoted to Corpus\n")
		for _, atom := range promoted {
			sb.WriteString(fmt.Sprintf("- **%s** (uses: %d)\n", atom.Atom.ID, atom.UsageCount))
		}
	}

	sb.WriteString("\nCommands:\n")
	sb.WriteString("- `/promote-atom <id>` - Promote an atom to the corpus\n")
	sb.WriteString("- `/reject-atom <id>` - Reject an evolved atom\n")

	return sb.String()
}

// renderStrategies displays the strategy database.
func (m Model) renderStrategies() string {
	if m.promptEvolver == nil {
		return "Prompt Evolution system not initialized."
	}

	strategies := m.promptEvolver.GetStrategies()
	if len(strategies) == 0 {
		return "No strategies in database yet.\n\nStrategies are created and refined through evolution cycles."
	}

	var sb strings.Builder
	sb.WriteString("## Strategy Database\n\n")
	sb.WriteString("Strategies are selected per problem type to guide agent behavior.\n\n")

	// Group by problem type
	byType := make(map[string][]*pe.Strategy)
	for _, s := range strategies {
		byType[string(s.ProblemType)] = append(byType[string(s.ProblemType)], s)
	}

	for problemType, typeStrategies := range byType {
		sb.WriteString(fmt.Sprintf("### %s\n", problemType))
		for _, s := range typeStrategies {
			successRate := float64(0)
			if s.SuccessCount+s.FailureCount > 0 {
				successRate = float64(s.SuccessCount) / float64(s.SuccessCount+s.FailureCount) * 100
			}
			sb.WriteString(fmt.Sprintf("- **%s** v%d (%.0f%% success, %d uses)\n",
				s.ID, s.Version, successRate, s.SuccessCount+s.FailureCount))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatTimeAgo formats a time as a human-readable "ago" string.
func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	}
}

// formatEvolutionResult formats the result of an evolution cycle for display.
func formatEvolutionResult(result *pe.EvolutionResult) string {
	if result == nil {
		return "Evolution cycle completed (no results)."
	}

	var sb strings.Builder
	sb.WriteString("## Evolution Cycle Complete\n\n")
	sb.WriteString(fmt.Sprintf("- **Groups Processed**: %d\n", result.GroupsProcessed))
	sb.WriteString(fmt.Sprintf("- **Failures Analyzed**: %d\n", result.FailuresAnalyzed))
	sb.WriteString(fmt.Sprintf("- **Atoms Generated**: %d\n", result.AtomsGenerated))
	sb.WriteString(fmt.Sprintf("- **Atoms Promoted**: %d\n", result.AtomsPromoted))
	sb.WriteString(fmt.Sprintf("- **Strategies Created**: %d\n", result.StrategiesCreated))
	sb.WriteString(fmt.Sprintf("- **Strategies Refined**: %d\n", result.StrategiesRefined))
	sb.WriteString(fmt.Sprintf("- **Duration**: %s\n", result.Duration.Round(time.Millisecond)))

	if len(result.AtomIDs) > 0 {
		sb.WriteString("\n### New Atoms Generated\n")
		for _, id := range result.AtomIDs {
			sb.WriteString(fmt.Sprintf("- `%s`\n", id))
		}
	}

	if len(result.Errors) > 0 {
		sb.WriteString("\n### Errors\n")
		for _, err := range result.Errors {
			sb.WriteString(fmt.Sprintf("- %s\n", err))
		}
	}

	return sb.String()
}
