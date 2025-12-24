// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains navigation and phase prompts for the Northstar wizard.
package chat

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// NORTHSTAR NAVIGATION
// =============================================================================

// advanceNorthstarPhase moves to the next wizard phase
func (m Model) advanceNorthstarPhase() (tea.Model, tea.Cmd) {
	w := m.northstarWizard
	w.SubStep = 0

	switch w.Phase {
	case NorthstarWelcome:
		w.Phase = NorthstarProblemStatement
	case NorthstarDocIngestion:
		w.Phase = NorthstarProblemStatement
	case NorthstarProblemStatement:
		w.Phase = NorthstarVisionStatement
	case NorthstarVisionStatement:
		w.Phase = NorthstarTargetUsers
	case NorthstarTargetUsers:
		w.Phase = NorthstarCapabilities
	case NorthstarCapabilities:
		w.Phase = NorthstarRedTeaming
	case NorthstarRedTeaming:
		w.Phase = NorthstarRequirements
	case NorthstarRequirements:
		w.Phase = NorthstarConstraints
	case NorthstarConstraints:
		w.Phase = NorthstarSummary
		return m.showNorthstarSummary()
	}

	// Recursively handle the new phase with empty input to show prompt
	return m.showNorthstarPhasePrompt()
}

// previousNorthstarPhase moves to the previous wizard phase
func (m Model) previousNorthstarPhase() (tea.Model, tea.Cmd) {
	w := m.northstarWizard
	w.SubStep = 0

	switch w.Phase {
	case NorthstarProblemStatement:
		w.Phase = NorthstarWelcome
	case NorthstarVisionStatement:
		w.Phase = NorthstarProblemStatement
	case NorthstarTargetUsers:
		w.Phase = NorthstarVisionStatement
	case NorthstarCapabilities:
		w.Phase = NorthstarTargetUsers
	case NorthstarRedTeaming:
		w.Phase = NorthstarCapabilities
	case NorthstarRequirements:
		w.Phase = NorthstarRedTeaming
	case NorthstarConstraints:
		w.Phase = NorthstarRequirements
	case NorthstarSummary:
		w.Phase = NorthstarConstraints
	}

	return m.showNorthstarPhasePrompt()
}

// showNorthstarPhasePrompt displays the prompt for the current phase
func (m Model) showNorthstarPhasePrompt() (tea.Model, tea.Cmd) {
	w := m.northstarWizard

	var prompt string
	var placeholder string

	switch w.Phase {
	case NorthstarWelcome:
		prompt = getNorthstarWelcomeMessage()
		placeholder = "yes / no..."
	case NorthstarDocIngestion:
		prompt = "Enter document paths or type \"done\":"
		placeholder = "Document paths..."
	case NorthstarProblemStatement:
		prompt = "## Problem Statement\n\n**What problem does this project solve?**"
		placeholder = "Describe the problem..."
	case NorthstarVisionStatement:
		if w.SubStep == 0 {
			prompt = "## Vision\n\n**If this succeeds wildly, what does the world look like?**"
			placeholder = "Describe the vision..."
		} else {
			prompt = "**Distill into a one-sentence mission:**"
			placeholder = "Mission statement..."
		}
	case NorthstarTargetUsers:
		prompt = "## Target Users\n\n**Describe a target user persona** (or \"done\"):"
		placeholder = "User persona..."
	case NorthstarCapabilities:
		prompt = "## Capabilities\n\n**What's one capability this project will have?** (or \"done\"):"
		placeholder = "Capability..."
	case NorthstarRedTeaming:
		prompt = "## Red Teaming\n\n**What could make this FAIL?** (or \"done\"):"
		placeholder = "Risk..."
	case NorthstarRequirements:
		prompt = "## Requirements\n\n**What's a must-have requirement?** (or \"done\", or \"auto\"):"
		placeholder = "Requirement..."
	case NorthstarConstraints:
		prompt = "## Constraints\n\n**What are the hard constraints?** (or \"done\"):"
		placeholder = "Constraints..."
	}

	m.history = append(m.history, Message{
		Role:    "assistant",
		Content: prompt,
		Time:    time.Now(),
	})
	m.textarea.Placeholder = placeholder
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// =============================================================================
// REQUIREMENT GENERATION
// =============================================================================

// autoGenerateRequirements triggers requirement generation from existing data
func (m Model) autoGenerateRequirements() (tea.Model, tea.Cmd) {
	w := m.northstarWizard

	// If LLM is available, use it for intelligent requirement generation
	if m.client != nil && (len(w.Capabilities) > 0 || len(w.Risks) > 0 || len(w.ExtractedFacts) > 0) {
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Analyzing vision, capabilities, and risks to generate requirements... This may take a moment.",
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()
		m.isLoading = true

		// Start async requirement generation
		return m, tea.Batch(m.spinner.Tick, m.generateRequirementsWithLLM())
	}

	// Fallback: Simple heuristic-based generation
	return m.autoGenerateRequirementsHeuristic()
}

// autoGenerateRequirementsHeuristic uses simple heuristics to generate requirements.
// This is the fallback when LLM is not available.
func (m Model) autoGenerateRequirementsHeuristic() (tea.Model, tea.Cmd) {
	w := m.northstarWizard
	initialCount := len(w.Requirements)

	// Generate requirements from capabilities
	for _, cap := range w.Capabilities {
		if cap.Priority == "critical" || cap.Priority == "high" {
			w.Requirements = append(w.Requirements, NorthstarRequirement{
				ID:          fmt.Sprintf("REQ-%03d", len(w.Requirements)+1),
				Type:        "functional",
				Description: fmt.Sprintf("Implement: %s", cap.Description),
				Priority:    "must-have",
				Source:      "capability",
			})
		}
	}

	// Generate requirements from risks (mitigations)
	for _, risk := range w.Risks {
		if risk.Impact == "high" && risk.Mitigation != "" && risk.Mitigation != "none" {
			w.Requirements = append(w.Requirements, NorthstarRequirement{
				ID:          fmt.Sprintf("REQ-%03d", len(w.Requirements)+1),
				Type:        "non-functional",
				Description: fmt.Sprintf("Risk mitigation: %s", risk.Mitigation),
				Priority:    "should-have",
				Source:      "risk-mitigation",
			})
		}
	}

	// Generate requirements from research-extracted facts
	for _, fact := range w.ExtractedFacts {
		// Classify the fact type based on keywords
		factLower := strings.ToLower(fact)
		var reqType, priority string

		if strings.Contains(factLower, "must") || strings.Contains(factLower, "require") ||
			strings.Contains(factLower, "need") || strings.Contains(factLower, "critical") {
			reqType = "functional"
			priority = "must-have"
		} else if strings.Contains(factLower, "performance") || strings.Contains(factLower, "security") ||
			strings.Contains(factLower, "scalab") || strings.Contains(factLower, "reliab") {
			reqType = "non-functional"
			priority = "should-have"
		} else if strings.Contains(factLower, "constraint") || strings.Contains(factLower, "limit") ||
			strings.Contains(factLower, "cannot") || strings.Contains(factLower, "must not") {
			reqType = "constraint"
			priority = "must-have"
		} else {
			// Skip facts that don't clearly map to requirements
			continue
		}

		w.Requirements = append(w.Requirements, NorthstarRequirement{
			ID:          fmt.Sprintf("REQ-%03d", len(w.Requirements)+1),
			Type:        reqType,
			Description: fact,
			Priority:    priority,
			Source:      "research",
		})
	}

	generated := len(w.Requirements) - initialCount
	var sources []string
	if len(w.Capabilities) > 0 {
		sources = append(sources, "capabilities")
	}
	if len(w.Risks) > 0 {
		sources = append(sources, "risks")
	}
	if len(w.ExtractedFacts) > 0 {
		sources = append(sources, "research documents")
	}

	sourceStr := "capabilities and risks"
	if len(sources) > 0 {
		sourceStr = strings.Join(sources, ", ")
	}

	m.history = append(m.history, Message{
		Role: "assistant",
		Content: fmt.Sprintf(`Auto-generated **%d requirements** from %s.

_Add more manually or type "done" to continue._`, generated, sourceStr),
		Time: time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}
