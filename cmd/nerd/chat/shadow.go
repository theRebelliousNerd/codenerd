// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains shadow mode and counterfactual reasoning.
package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/core"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// SHADOW MODE
// =============================================================================
// Functions for shadow mode simulation and counterfactual reasoning.

// runShadowSimulation runs a full Shadow Mode simulation
func (m Model) runShadowSimulation(action string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		m.ReportStatus("Running Shadow Mode simulation...")

		// Create a simulated action from the description
		simAction := core.SimulatedAction{
			ID:          fmt.Sprintf("sim_%d", time.Now().UnixNano()),
			Type:        core.ActionTypeRefactor, // Default to refactor for general actions
			Target:      action,
			Description: action,
		}

		// Use the WhatIf API for quick counterfactual queries
		result, err := m.shadowMode.WhatIf(ctx, simAction)
		if err != nil {
			return errorMsg(fmt.Errorf("shadow mode simulation failed: %w", err))
		}

		// Format the results
		var sb strings.Builder
		sb.WriteString("## Shadow Mode Simulation\n\n")
		sb.WriteString(fmt.Sprintf("**Hypothetical**: %s\n\n", action))

		if len(result.Effects) == 0 {
			sb.WriteString("No effects derived from this hypothetical.\n")
		} else {
			sb.WriteString("### Projected Effects\n\n")
			for _, effect := range result.Effects {
				sb.WriteString(fmt.Sprintf("- %s(%v)\n", effect.Predicate, effect.Args))
			}
		}

		// Check for safety violations
		if len(result.Violations) > 0 {
			sb.WriteString("\n### Safety Violations Detected\n\n")
			for _, v := range result.Violations {
				severity := v.Severity
				if v.Blocking {
					severity = "BLOCKING"
				}
				sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", severity, v.ViolationType, v.Description))
			}
		}

		if result.IsSafe {
			sb.WriteString("\n_Simulation indicates this action is **safe** to proceed._\n")
		} else {
			sb.WriteString("\n_Simulation indicates this action has **blocking violations**._\n")
		}

		m.ReportStatus("Simulation complete")
		return responseMsg(sb.String())
	}
}

// runWhatIfQuery runs a counterfactual query
func (m Model) runWhatIfQuery(change string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		m.ReportStatus("Running counterfactual analysis...")

		// Create a simulated action for the what-if query
		simAction := core.SimulatedAction{
			ID:          fmt.Sprintf("whatif_%d", time.Now().UnixNano()),
			Type:        core.ActionTypeFileWrite, // Default to file write for impact analysis
			Target:      change,
			Description: fmt.Sprintf("What if: %s", change),
		}

		// Use the WhatIf API
		result, err := m.shadowMode.WhatIf(ctx, simAction)
		if err != nil {
			return errorMsg(fmt.Errorf("what-if query failed: %w", err))
		}

		// Get impact analysis from kernel
		impacted, _ := m.kernel.Query("impacted")

		// Format the results
		var sb strings.Builder
		sb.WriteString("## What-If Analysis\n\n")
		sb.WriteString(fmt.Sprintf("**Change**: %s\n\n", change))

		if len(result.Effects) > 0 {
			sb.WriteString("### Projected Effects\n\n")
			for _, effect := range result.Effects {
				sb.WriteString(fmt.Sprintf("- %s(%v)\n", effect.Predicate, effect.Args))
			}
		}

		if len(impacted) > 0 {
			sb.WriteString("\n### Impacted Components\n\n")
			for _, imp := range impacted {
				sb.WriteString(fmt.Sprintf("- %s\n", imp.String()))
			}
		}

		if len(result.Violations) > 0 {
			sb.WriteString("\n### Safety Concerns\n\n")
			for _, v := range result.Violations {
				sb.WriteString(fmt.Sprintf("- [%s] %s\n", v.Severity, v.Description))
			}
		}

		// Provide recommendations
		sb.WriteString("\n### Recommendations\n\n")
		if len(impacted) > 5 {
			sb.WriteString("- High impact change - consider incremental approach\n")
		}
		if len(result.Effects) > 0 {
			sb.WriteString("- Review projected effects before proceeding\n")
		}
		if !result.IsSafe {
			sb.WriteString("- Address safety violations before making changes\n")
		}
		sb.WriteString("- Run tests after making changes\n")

		m.ReportStatus("Analysis complete")
		return responseMsg(sb.String())
	}
}

// renderLogicPane renders content for the logic pane
func (m Model) renderLogicPane() string {
	if m.logicPane == nil {
		return ""
	}

	var sb strings.Builder

	// Header
	sb.WriteString(m.styles.Layout.Header.Render("Logic State"))
	sb.WriteString("\n")
	sb.WriteString(m.styles.RenderDivider(30))
	sb.WriteString("\n\n")

	// Recent facts
	facts, _ := m.kernel.Query("*")
	if len(facts) > 0 {
		sb.WriteString(m.styles.Text.Bold.Render("Recent Facts"))
		sb.WriteString("\n")
		count := min(len(facts), 10)
		for i := 0; i < count; i++ {
			sb.WriteString(fmt.Sprintf("  %s\n", facts[i].String()))
		}
		if len(facts) > 10 {
			sb.WriteString(fmt.Sprintf("  ... +%d more\n", len(facts)-10))
		}
	}

	// Current intent
	intents, _ := m.kernel.Query("user_intent")
	if len(intents) > 0 {
		sb.WriteString("\n")
		sb.WriteString(m.styles.Text.Bold.Render("Current Intent"))
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("  %s\n", intents[len(intents)-1].String()))
	}

	// Pending actions
	actions, _ := m.kernel.Query("next_action")
	if len(actions) > 0 {
		sb.WriteString("\n")
		sb.WriteString(m.styles.Text.Bold.Render("Pending Actions"))
		sb.WriteString("\n")
		for _, a := range actions {
			sb.WriteString(fmt.Sprintf("  %s\n", a.String()))
		}
	}

	return sb.String()
}

// UpdateLogicPane updates the logic pane content
func (m *Model) UpdateLogicPane() {
	if m.logicPane != nil {
		content := m.renderLogicPane()
		m.logicPane.Viewport.SetContent(content)
	}
}
