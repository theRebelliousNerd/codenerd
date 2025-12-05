// Package main provides the codeNERD CLI entry point.
// This file contains Shadow Mode simulation and What-If queries.
package main

import (
	"codenerd/cmd/nerd/ui"
	"codenerd/internal/core"
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// SHADOW MODE & WHAT-IF QUERIES
// =============================================================================
// Shadow Mode allows simulating actions before execution.
// What-If queries enable counterfactual reasoning about system state.

func (m chatModel) runShadowSimulation(actionType, target string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Map action type string to SimActionType
		var at core.SimActionType
		switch actionType {
		case "write":
			at = core.ActionTypeFileWrite
		case "delete":
			at = core.ActionTypeFileDelete
		case "refactor":
			at = core.ActionTypeRefactor
		case "exec":
			at = core.ActionTypeExec
		case "commit":
			at = core.ActionTypeGitCommit
		default:
			return errorMsg(fmt.Errorf("unknown action type: %s", actionType))
		}

		// Start simulation
		sim, err := m.shadowMode.StartSimulation(ctx, fmt.Sprintf("%s %s", actionType, target))
		if err != nil {
			return errorMsg(fmt.Errorf("failed to start simulation: %w", err))
		}

		// Create the action
		action := core.SimulatedAction{
			ID:          fmt.Sprintf("action_%d", time.Now().UnixNano()),
			Type:        at,
			Target:      target,
			Description: fmt.Sprintf("%s on %s", actionType, target),
		}

		// Run simulation
		result, err := m.shadowMode.SimulateAction(ctx, action)
		if err != nil {
			m.shadowMode.AbortSimulation(err.Error())
			return errorMsg(fmt.Errorf("simulation failed: %w", err))
		}

		// Build response
		var sb strings.Builder
		sb.WriteString("## 🌑 Shadow Mode Simulation Complete\n\n")
		sb.WriteString(fmt.Sprintf("**Simulation ID**: `%s`\n", sim.ID))
		sb.WriteString(fmt.Sprintf("**Action**: %s → %s\n\n", actionType, target))

		// Show projected effects
		sb.WriteString("### Projected Effects\n\n")
		if len(result.Effects) == 0 {
			sb.WriteString("_No effects projected._\n\n")
		} else {
			sb.WriteString("```datalog\n")
			for _, effect := range result.Effects {
				op := "+"
				if !effect.IsPositive {
					op = "-"
				}
				sb.WriteString(fmt.Sprintf("%s %s(%v)\n", op, effect.Predicate, effect.Args))
			}
			sb.WriteString("```\n\n")
		}

		// Show violations
		sb.WriteString("### Safety Analysis\n\n")
		if len(result.Violations) == 0 {
			sb.WriteString("✅ **No violations detected** - Action appears safe.\n\n")
		} else {
			for _, v := range result.Violations {
				icon := "⚠️"
				if v.Blocking {
					icon = "🛑"
				}
				sb.WriteString(fmt.Sprintf("%s **%s**: %s\n", icon, v.ViolationType, v.Description))
			}
			sb.WriteString("\n")
		}

		// Overall verdict
		if result.IsSafe {
			sb.WriteString("### ✅ Verdict: SAFE\n\n")
			sb.WriteString("The simulated action passes all safety checks.\n")
		} else {
			sb.WriteString("### 🛑 Verdict: BLOCKED\n\n")
			sb.WriteString("The action would be blocked by safety rules.\n")
		}

		// Abort the simulation (don't apply changes)
		m.shadowMode.AbortSimulation("simulation complete - not applying")

		return responseMsg(sb.String())
	}
}

// runWhatIfQuery runs a quick counterfactual query
func (m chatModel) runWhatIfQuery(scenario string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// Parse the scenario to determine action type
		scenarioLower := strings.ToLower(scenario)
		var actionType core.SimActionType
		var target string

		switch {
		case strings.Contains(scenarioLower, "delete"):
			actionType = core.ActionTypeFileDelete
			target = extractTarget(scenario, "delete")
		case strings.Contains(scenarioLower, "refactor"):
			actionType = core.ActionTypeRefactor
			target = extractTarget(scenario, "refactor")
		case strings.Contains(scenarioLower, "modify") || strings.Contains(scenarioLower, "change") || strings.Contains(scenarioLower, "edit"):
			actionType = core.ActionTypeFileWrite
			target = extractTarget(scenario, "modify", "change", "edit")
		case strings.Contains(scenarioLower, "commit"):
			actionType = core.ActionTypeGitCommit
			target = "HEAD"
		case strings.Contains(scenarioLower, "test") && strings.Contains(scenarioLower, "fail"):
			// Simulate test failure scenario
			actionType = core.ActionTypeExec
			target = "test"
		default:
			actionType = core.ActionTypeFileWrite
			target = scenario
		}

		// Create action
		action := core.SimulatedAction{
			ID:          fmt.Sprintf("whatif_%d", time.Now().UnixNano()),
			Type:        actionType,
			Target:      target,
			Description: scenario,
		}

		// Run what-if query
		result, err := m.shadowMode.WhatIf(ctx, action)
		if err != nil {
			return errorMsg(fmt.Errorf("what-if query failed: %w", err))
		}

		// Build response
		var sb strings.Builder
		sb.WriteString("## 🔮 What-If Analysis Results\n\n")
		sb.WriteString(fmt.Sprintf("**Scenario**: _%s_\n\n", scenario))
		sb.WriteString(fmt.Sprintf("**Interpreted as**: `%s` on `%s`\n\n", actionType, target))

		// Effects
		sb.WriteString("### If this happens, then:\n\n")
		if len(result.Effects) == 0 {
			sb.WriteString("- No immediate effects detected\n")
		} else {
			for _, effect := range result.Effects {
				sb.WriteString(fmt.Sprintf("- `%s(%v)` would be asserted\n", effect.Predicate, effect.Args))
			}
		}
		sb.WriteString("\n")

		// Consequences
		sb.WriteString("### Potential Consequences:\n\n")
		if len(result.Violations) == 0 {
			sb.WriteString("✅ No safety violations predicted.\n\n")
		} else {
			for _, v := range result.Violations {
				icon := "⚠️"
				if v.Blocking {
					icon = "🛑"
				}
				sb.WriteString(fmt.Sprintf("%s %s\n", icon, v.Description))
			}
			sb.WriteString("\n")
		}

		// Recommendation
		sb.WriteString("### Recommendation:\n\n")
		if result.IsSafe {
			sb.WriteString("👍 This action appears safe to proceed with.\n")
		} else {
			sb.WriteString("⚠️ Consider addressing the violations before proceeding.\n")
		}

		return responseMsg(sb.String())
	}
}

// extractTarget extracts the target from a scenario description
func extractTarget(scenario string, keywords ...string) string {
	words := strings.Fields(scenario)
	for i, word := range words {
		for _, kw := range keywords {
			if strings.EqualFold(word, kw) && i+1 < len(words) {
				// Return everything after the keyword
				return strings.Join(words[i+1:], " ")
			}
		}
	}
	// Return the whole scenario if no keyword found
	return scenario
}

// buildDerivationTrace constructs a derivation trace from kernel facts
func (m chatModel) buildDerivationTrace(predicate string, facts []core.Fact) *ui.DerivationTrace {
	trace := &ui.DerivationTrace{
		Query:       predicate,
		TotalFacts:  len(facts),
		DerivedTime: 10 * time.Millisecond, // Placeholder, could track actual time
		RootNodes:   make([]*ui.DerivationNode, 0),
	}

	// Build nodes from facts
	for _, fact := range facts {
		args := make([]string, len(fact.Args))
		for i, arg := range fact.Args {
			args[i] = fmt.Sprintf("%v", arg)
		}

		node := &ui.DerivationNode{
			Predicate:  fact.Predicate,
			Args:       args,
			Source:     "idb", // Assume derived unless we can determine otherwise
			Rule:       m.getRuleForPredicate(fact.Predicate),
			Expanded:   true,
			Activation: 0.8 + float64(len(facts)-1)*0.05, // Simulated activation
			Children:   m.getChildNodes(fact),
		}
		trace.RootNodes = append(trace.RootNodes, node)
	}

	// If no facts, create a placeholder
	if len(trace.RootNodes) == 0 {
		trace.RootNodes = append(trace.RootNodes, &ui.DerivationNode{
			Predicate:  predicate,
			Args:       []string{"(no facts derived)"},
			Source:     "edb",
			Expanded:   false,
			Activation: 0.0,
		})
	}

	return trace
}

// getRuleForPredicate returns the Mangle rule that derives a predicate
func (m chatModel) getRuleForPredicate(predicate string) string {
	// Map of predicates to their derivation rules
	ruleMap := map[string]string{
		"next_action":          "next_action(A) :- user_intent(_, V, T, _), action_mapping(V, A).",
		"block_commit":         "block_commit(R) :- diagnostic(/error, _, _, _, _).",
		"permitted":            "permitted(A) :- safe_action(A).",
		"impacted":             "impacted(X) :- dependency_link(X, Y, _), modified(Y).",
		"clarification_needed": "clarification_needed(R) :- focus_resolution(R, _, _, S), S < 0.85.",
		"unsafe_to_refactor":   "unsafe_to_refactor(T) :- impacted(D), not test_coverage(D).",
		"needs_research":       "needs_research(A) :- shard_profile(A, _, T, _), not knowledge_ingested(A).",
	}

	if rule, ok := ruleMap[predicate]; ok {
		return rule
	}
	return ""
}

// getChildNodes finds the supporting facts for a derived fact
func (m chatModel) getChildNodes(fact core.Fact) []*ui.DerivationNode {
	children := make([]*ui.DerivationNode, 0)

	// Based on the predicate, find related facts
	switch fact.Predicate {
	case "next_action":
		// next_action depends on user_intent and action_mapping
		intents, _ := m.kernel.Query("user_intent")
		for _, intent := range intents {
			args := make([]string, len(intent.Args))
			for i, arg := range intent.Args {
				args[i] = fmt.Sprintf("%v", arg)
			}
			children = append(children, &ui.DerivationNode{
				Predicate:  intent.Predicate,
				Args:       args,
				Source:     "edb",
				Expanded:   false,
				Activation: 0.7,
			})
		}

	case "permitted":
		// permitted depends on safe_action or admin_override
		safeActions, _ := m.kernel.Query("safe_action")
		for _, sa := range safeActions {
			args := make([]string, len(sa.Args))
			for i, arg := range sa.Args {
				args[i] = fmt.Sprintf("%v", arg)
			}
			children = append(children, &ui.DerivationNode{
				Predicate:  sa.Predicate,
				Args:       args,
				Source:     "edb",
				Expanded:   false,
				Activation: 0.6,
			})
		}

	case "impacted":
		// impacted depends on dependency_link and modified
		deps, _ := m.kernel.Query("dependency_link")
		for _, dep := range deps {
			args := make([]string, len(dep.Args))
			for i, arg := range dep.Args {
				args[i] = fmt.Sprintf("%v", arg)
			}
			children = append(children, &ui.DerivationNode{
				Predicate:  dep.Predicate,
				Args:       args,
				Source:     "edb",
				Expanded:   false,
				Activation: 0.5,
			})
		}

	case "block_commit":
		// block_commit depends on diagnostic or test_state
		diagnostics, _ := m.kernel.Query("diagnostic")
		for _, diag := range diagnostics {
			args := make([]string, len(diag.Args))
			for i, arg := range diag.Args {
				args[i] = fmt.Sprintf("%v", arg)
			}
			children = append(children, &ui.DerivationNode{
				Predicate:  diag.Predicate,
				Args:       args,
				Source:     "edb",
				Expanded:   false,
				Activation: 0.9, // High activation for blockers
			})
		}
	}

	return children
}

// runInit performs comprehensive workspace initialization
