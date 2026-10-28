// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains navigation and phase prompts for the Northstar wizard.
package chat

import (
	"fmt"
	"os"
	"sort"
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
	case NorthstarDocIngestion:
		w.Phase = NorthstarWelcome
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

	m = m.addMessage(Message{
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
		m = m.addMessage(Message{
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
// Agentic auto mode: it also explores the wizard's accumulated context
// (capabilities, risks, research facts), surfaces suggestions for uncovered
// areas, and asks targeted follow-up questions instead of silently finishing.
func (m Model) autoGenerateRequirementsHeuristic() (tea.Model, tea.Cmd) {
	w := m.northstarWizard
	w.ensureIDs()
	initialCount := len(w.Requirements)

	northstarHasRequirement := func(substr string) bool {
		for _, r := range w.Requirements {
			if strings.Contains(r.Description, substr) {
				return true
			}
		}
		return false
	}

	// Generate requirements from capabilities (idempotent: skip already-covered).
	for _, cap := range w.Capabilities {
		if cap.Priority == "critical" || cap.Priority == "high" {
			if northstarHasRequirement(cap.Description) {
				continue
			}
			w.Requirements = append(w.Requirements, NorthstarRequirement{
				ID:          w.nextRequirementID(),
				Type:        "functional",
				Description: fmt.Sprintf("Implement: %s", cap.Description),
				Priority:    "must-have",
				Source:      "capability",
				Supports:    []string{cap.ID},
			})
		}
	}

	// Generate requirements from risks (mitigations, idempotent).
	for _, risk := range w.Risks {
		if risk.Impact == "high" && risk.Mitigation != "" && risk.Mitigation != "none" {
			if northstarHasRequirement(risk.Mitigation) {
				continue
			}
			w.Requirements = append(w.Requirements, NorthstarRequirement{
				ID:          w.nextRequirementID(),
				Type:        "non-functional",
				Description: fmt.Sprintf("Risk mitigation: %s", risk.Mitigation),
				Priority:    "should-have",
				Source:      "risk-mitigation",
				Addresses:   []string{risk.ID},
			})
		}
	}

	// Generate requirements from research-extracted facts (idempotent).
	for _, fact := range w.ExtractedFacts {
		if northstarHasRequirement(fact) {
			continue
		}
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
			ID:          w.nextRequirementID(),
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
	if len(w.Personas) > 0 {
		sources = append(sources, fmt.Sprintf("%d persona(s)", len(w.Personas)))
	}

	sourceStr := "capabilities and risks"
	if len(sources) > 0 {
		sourceStr = strings.Join(sources, ", ")
	}

	// Agentic interview: explore coverage gaps, suggest concrete next steps,
	// and ask targeted questions. Auto mode proposes; the user disposes.
	var suggestions []string
	var questions []string

	coveredCaps := map[string]bool{}
	for _, r := range w.Requirements {
		for _, c := range w.Capabilities {
			if strings.Contains(r.Description, c.Description) {
				coveredCaps[c.Description] = true
			}
		}
	}
	for _, c := range w.Capabilities {
		if len(suggestions) >= 8 {
			break
		}
		if !coveredCaps[c.Description] && c.Priority != "critical" && c.Priority != "high" {
			suggestions = append(suggestions, fmt.Sprintf("Consider a requirement for capability %q [%s]", c.Description, c.Priority))
		}
	}
	unmitigated := 0
	for _, risk := range w.Risks {
		if strings.TrimSpace(risk.Mitigation) == "" || risk.Mitigation == "none" {
			unmitigated++
		}
	}
	if unmitigated > 0 && len(suggestions) < 8 {
		suggestions = append(suggestions, fmt.Sprintf("%d risk(s) have no mitigation yet \u2014 add one or type \"auto\" again after red-teaming", unmitigated))
	}
	if len(w.Personas) == 0 {
		questions = append(questions, "Who is the primary user? (go back with \"back\" to add a persona, or describe one now)")
	}
	if len(w.Capabilities) == 0 {
		questions = append(questions, "What is the one capability this project must have? (go back with \"back\" or describe it now)")
	}
	if len(w.ExtractedFacts) == 0 && len(w.ResearchDocs) == 0 {
		areas := northstarSurveyWorkspace(m.workspace)
		if len(areas) > 0 {
			display := strings.Join(areas, ", ")
			if len([]rune(display)) > 160 {
				display = string([]rune(display)[:157]) + "..."
			}
			suggestions = append(suggestions, fmt.Sprintf("Explore the codebase: detected %s — run /scan or add docs (README, nerd.md) in the Doc Ingestion step, then re-run \"auto\" to ground requirements", display))
			for _, area := range areas {
				if len(suggestions) >= 8 {
					break
				}
				suggestions = append(suggestions, fmt.Sprintf("Consider a requirement covering %q (detected in workspace)", area))
			}
			questions = append(questions, fmt.Sprintf("Which part of the codebase should requirements cover first (detected: %s)?", display))
		} else {
			suggestions = append(suggestions, "Explore the codebase: run /scan or add docs (README, nerd.md) in the Doc Ingestion step, then re-run \"auto\" to ground requirements")
			questions = append(questions, "Which part of the codebase should requirements cover first (e.g. CLI, kernel, chat)?")
		}
	}
	if w.Problem == "" {
		questions = append(questions, "What problem does this solve, in one sentence?")
	}
	if w.Vision == "" {
		questions = append(questions, "If this succeeds wildly, what changes?")
	}

	var extra strings.Builder
	if len(suggestions) > 0 {
		extra.WriteString("\n\n**Suggestions:**\n")
		for _, s := range suggestions {
			extra.WriteString("- " + s + "\n")
		}
	}
	if len(questions) > 0 {
		extra.WriteString("\n**To sharpen this (answer any, or type \"done\"):**\n")
		for i, q := range questions {
			extra.WriteString(fmt.Sprintf("%d. %s\n", i+1, q))
		}
	}

	w.LastUpdated = time.Now()

	m = m.addMessage(Message{
		Role: "assistant",
		Content: fmt.Sprintf(`Auto-generated **%d requirements** from %s.%s

  _Add more manually, answer a question above, or type "done" to continue._`, generated, sourceStr, extra.String()),
		Time: time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// northstarSurveyWorkspace explores the workspace directory to ground auto
// mode in real codebase structure. It lists top-level directories and key
// marker files (go.mod, README, nerd.md) so auto mode can ask targeted
// questions and suggest concrete areas instead of generic placeholders.
// Returns an empty slice when workspace is unset or unreadable.
func northstarSurveyWorkspace(workspace string) []string {
	if strings.TrimSpace(workspace) == "" {
		return nil
	}
	entries, err := os.ReadDir(workspace)
	if err != nil {
		return nil
	}
	var dirs []string
	var markers []string
	for _, e := range entries {
		name := e.Name()
		// Skip noise and hidden entries; visible dirs ground codebase questions.
		if name == ".git" || name == ".nerd" || name == "node_modules" || name == ".idea" || name == ".vscode" {
			continue
		}
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, name)
			continue
		}
		lower := strings.ToLower(name)
		if lower == "go.mod" || lower == "readme.md" || lower == "nerd.md" || lower == "package.json" || lower == "cargo.toml" {
			markers = append(markers, name)
		}
	}
	sort.Strings(dirs)
	sort.Strings(markers)
	// Prefer directories (codebase areas); append markers for grounding.
	areas := append([]string{}, dirs...)
	areas = append(areas, markers...)
	if len(areas) > 8 {
		areas = areas[:8]
	}
	return areas
}

// Agentic interview preserved: auto mode explores, asks, and suggests in a bounded state-driven way.
