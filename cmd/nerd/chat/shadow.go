// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains shadow mode and counterfactual reasoning.
package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codenerd/cmd/nerd/ui"
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

		return responseMsg(sb.String())
	}
}

// runWhatIfQuery runs a counterfactual query
func (m Model) runWhatIfQuery(change string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

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

		return responseMsg(sb.String())
	}
}

// buildDerivationTrace builds a trace explaining why a fact was derived
func (m Model) buildDerivationTrace(fact string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Derivation Trace for: %s\n\n", fact))

	// Query for the fact
	facts, err := m.kernel.Query(fact)
	if err != nil || len(facts) == 0 {
		sb.WriteString("Fact not found in the knowledge base.\n")
		return sb.String()
	}

	// Build the trace tree
	sb.WriteString("### Derivation Tree\n\n")
	sb.WriteString("```\n")

	for _, f := range facts {
		sb.WriteString(fmt.Sprintf("%s\n", f.String()))

		// Get the rule that derived this fact
		rule := getRuleForPredicate(f.Predicate)
		if rule != "" {
			sb.WriteString(fmt.Sprintf("  <- Rule: %s\n", rule))
		}

		// Get child facts (premises)
		children := getChildNodes(m.kernel, f)
		for _, child := range children {
			sb.WriteString(fmt.Sprintf("    <- %s\n", child.String()))
		}
	}

	sb.WriteString("```\n")

	return sb.String()
}

// getRuleForPredicate returns the rule that derives a predicate
func getRuleForPredicate(predicate string) string {
	// Common rules in the system
	rules := map[string]string{
		"next_action":           "next_action(X) :- user_intent(_, Verb, _), action_for_verb(Verb, X).",
		"impacted":              "impacted(X) :- dependency_link(X, Y, _), modified(Y).",
		"clarification_needed":  "clarification_needed(Ref) :- focus_resolution(Ref, _, _, Score), Score < 0.85.",
		"block_commit":          "block_commit(Reason) :- diagnostic(/error, _, _, _, _).",
		"permitted":             "permitted(Action) :- safe_action(Action).",
		"context_to_inject":     "context_to_inject(Fact) :- activation(Fact, Score), Score > 30.",
		"unsafe_to_refactor":    "unsafe_to_refactor(Target) :- impacted(Dep), not test_coverage(Dep).",
		"needs_research":        "needs_research(Agent) :- shard_profile(Agent, _, Topics, _), not knowledge_ingested(Agent).",
	}

	if rule, ok := rules[predicate]; ok {
		return rule
	}
	return ""
}

// getChildNodes returns the child facts (premises) for a derived fact
func getChildNodes(kernel *core.RealKernel, fact core.Fact) []core.Fact {
	children := []core.Fact{}

	// Query for related facts based on the predicate
	switch fact.Predicate {
	case "next_action":
		// Look for user_intent
		intents, _ := kernel.Query("user_intent")
		children = append(children, intents...)

	case "impacted":
		// Look for dependency_link and modified
		deps, _ := kernel.Query("dependency_link")
		children = append(children, deps...)
		mods, _ := kernel.Query("modified")
		children = append(children, mods...)

	case "clarification_needed":
		// Look for focus_resolution
		focus, _ := kernel.Query("focus_resolution")
		children = append(children, focus...)
	}

	// Limit to first 5 children
	if len(children) > 5 {
		children = children[:5]
	}

	return children
}

// renderLogicPane renders content for the logic pane
func (m Model) renderLogicPane() string {
	if m.logicPane == nil {
		return ""
	}

	var sb strings.Builder

	// Header
	sb.WriteString(m.styles.Header.Render("Logic State"))
	sb.WriteString("\n")
	sb.WriteString(m.styles.RenderDivider(30))
	sb.WriteString("\n\n")

	// Recent facts
	facts, _ := m.kernel.Query("*")
	if len(facts) > 0 {
		sb.WriteString(m.styles.Bold.Render("Recent Facts"))
		sb.WriteString("\n")
		count := 10
		if len(facts) < count {
			count = len(facts)
		}
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
		sb.WriteString(m.styles.Bold.Render("Current Intent"))
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf("  %s\n", intents[len(intents)-1].String()))
	}

	// Pending actions
	actions, _ := m.kernel.Query("next_action")
	if len(actions) > 0 {
		sb.WriteString("\n")
		sb.WriteString(m.styles.Bold.Render("Pending Actions"))
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

// getStyles returns the current UI styles (for helper functions)
func (m Model) getStyles() ui.Styles {
	return m.styles
}
