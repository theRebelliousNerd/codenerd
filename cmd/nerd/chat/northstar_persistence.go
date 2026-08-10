// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains persistence and storage functions for the Northstar wizard.
package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// NORTHSTAR PERSISTENCE
// =============================================================================

// saveNorthstar saves the northstar definition to disk and optionally starts a campaign
func (m Model) saveNorthstar(startCampaign bool) (tea.Model, tea.Cmd) {
	w := m.northstarWizard

	// Track any warnings during save
	var warnings []string

	// Generate Mangle facts
	mangleFacts := generateNorthstarMangle(w)

	// Save to .nerd/northstar.mg
	northstarPath := filepath.Join(m.workspace, ".nerd", "northstar.mg")
	if err := os.WriteFile(northstarPath, []byte(mangleFacts), 0644); err != nil {
		warnings = append(warnings, fmt.Sprintf("Failed to save northstar.mg: %v", err))
	}

	// Save to knowledge database
	if m.localDB != nil {
		if errs := saveNorthstarToKnowledgeBase(m.localDB, w); len(errs) > 0 {
			warnings = append(warnings, fmt.Sprintf("Knowledge base: %d item(s) failed to save", len(errs)))
		}
	} else {
		warnings = append(warnings, "Knowledge database not available")
	}

	// Assert facts into kernel
	if m.kernel != nil {
		if errs := assertNorthstarFacts(m.kernel, w); len(errs) > 0 {
			warnings = append(warnings, fmt.Sprintf("Kernel: %d fact(s) failed to assert", len(errs)))
		}
	} else {
		warnings = append(warnings, "Kernel not available - facts not loaded")
	}

	// Save JSON backup
	jsonPath := filepath.Join(m.workspace, ".nerd", "northstar.json")
	if jsonData, err := json.MarshalIndent(w, "", "  "); err == nil {
		if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
			warnings = append(warnings, fmt.Sprintf("Failed to save JSON backup: %v", err))
		}
	}

	// Exit wizard mode
	m.setInputMode(InputModeNormal)
	m.textarea.Placeholder = "Ask me anything... (Enter to send, Alt+Enter for newline, Ctrl+C to exit)"

	var msg string
	if startCampaign {
		msg = fmt.Sprintf(`## Northstar Saved!

Stored to:
- `+"`%s`"+`
- `+"`%s`"+`
- Knowledge database

**Starting campaign based on this vision...**

_The Campaign Planner will decompose your vision into actionable phases._`,
			northstarPath, jsonPath)

		m = m.addMessage(Message{
			Role:    "assistant",
			Content: msg,
			Time:    time.Now(),
		})
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		m.textarea.Reset()

		// Start campaign
		goal := fmt.Sprintf("Implement the Northstar vision: %s", w.Mission)
		m.isLoading = true
		return m, tea.Batch(m.spinner.Tick, m.startCampaign(goal))
	}

	var msgBuilder strings.Builder
	msgBuilder.WriteString("## Northstar Saved!\n\n")
	msgBuilder.WriteString("Your vision has been stored to:\n")
	msgBuilder.WriteString(fmt.Sprintf("- `%s` (Mangle facts)\n", northstarPath))
	msgBuilder.WriteString(fmt.Sprintf("- `%s` (JSON backup)\n", jsonPath))
	msgBuilder.WriteString("- Knowledge database (semantic search)\n\n")

	if len(warnings) > 0 {
		msgBuilder.WriteString("**Warnings:**\n")
		for _, w := range warnings {
			msgBuilder.WriteString(fmt.Sprintf("- %s\n", w))
		}
		msgBuilder.WriteString("\n")
	}

	msgBuilder.WriteString("The kernel now has access to your vision for reasoning.\n\n")
	msgBuilder.WriteString("**Next steps:**\n")
	msgBuilder.WriteString("- Run `/campaign start <goal>` to start building\n")
	msgBuilder.WriteString("- Run `/query northstar_mission` to query the kernel\n")
	msgBuilder.WriteString("- The vision will inform all future planning")

	m = m.addMessage(Message{
		Role:    "assistant",
		Content: msgBuilder.String(),
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}

// =============================================================================
// MANGLE GENERATION
// =============================================================================

// generateNorthstarMangle generates Mangle facts from the wizard state
func generateNorthstarMangle(w *NorthstarWizardState) string {
	var sb strings.Builder

	sb.WriteString("# Northstar Vision Facts\n")
	sb.WriteString(fmt.Sprintf("# Generated: %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString("# This file defines the project's north star and informs kernel reasoning.\n")
	sb.WriteString("# Schema declarations are in internal/core/defaults/schemas.mg\n\n")

	// Core Vision Facts - IDs are /string per Decl
	sb.WriteString("# Core Vision Facts\n")
	sb.WriteString(fmt.Sprintf("northstar_mission(%q, %q).\n", "global", w.Mission))
	sb.WriteString(fmt.Sprintf("northstar_problem(%q, %q).\n", "global", w.Problem))
	sb.WriteString(fmt.Sprintf("northstar_vision(%q, %q).\n", "global", w.Vision))
	sb.WriteString("\n")

	// Personas - Decl bound [/string, /string]
	if len(w.Personas) > 0 {
		sb.WriteString("# User Personas\n")
		for i, p := range w.Personas {
			personaID := fmt.Sprintf("persona_%d", i+1)
			sb.WriteString(fmt.Sprintf("northstar_persona(%q, %q).\n", personaID, p.Name))
			for _, pain := range p.PainPoints {
				sb.WriteString(fmt.Sprintf("northstar_pain_point(%q, %q).\n", personaID, pain))
			}
			for _, need := range p.Needs {
				sb.WriteString(fmt.Sprintf("northstar_need(%q, %q).\n", personaID, need))
			}
		}
		sb.WriteString("\n")
	}

	// Capabilities - Decl bound [/string, /string, /name, /number]
	if len(w.Capabilities) > 0 {
		sb.WriteString("# Capabilities\n")
		for i, c := range w.Capabilities {
			capID := fmt.Sprintf("cap_%d", i+1)
			timeline := strings.ToLower(strings.ReplaceAll(c.Timeline, " ", "_"))
			priority := capabilityPriorityToNumber(c.Priority)
			sb.WriteString(fmt.Sprintf("northstar_capability(%q, %q, /%s, %d).\n",
				capID, c.Description, timeline, priority))
		}
		sb.WriteString("\n")
	}

	// Risks - Decl bound [/string, /string, /name, /number]
	if len(w.Risks) > 0 {
		sb.WriteString("# Risks\n")
		for i, r := range w.Risks {
			riskID := fmt.Sprintf("risk_%d", i+1)
			likelihood := strings.ToLower(r.Likelihood)
			impact := riskImpactToNumber(r.Impact)
			sb.WriteString(fmt.Sprintf("northstar_risk(%q, %q, /%s, %d).\n",
				riskID, r.Description, likelihood, impact))
			if r.Mitigation != "" && r.Mitigation != "none" {
				sb.WriteString(fmt.Sprintf("northstar_mitigation(%q, /mitigation).\n", riskID))
			}
		}
		sb.WriteString("\n")
	}

	// Requirements - Decl bound [/string, /name, /string, /number]
	if len(w.Requirements) > 0 {
		sb.WriteString("# Requirements\n")
		for _, r := range w.Requirements {
			reqID := strings.ToLower(r.ID)
			reqType := strings.ToLower(strings.ReplaceAll(r.Type, " ", "_"))
			reqType = strings.ReplaceAll(reqType, "-", "_")
			priority := capabilityPriorityToNumber(r.Priority)
			sb.WriteString(fmt.Sprintf("northstar_requirement(%q, /%s, %q, %d).\n",
				reqID, reqType, r.Description, priority))
		}
		sb.WriteString("\n")
	}

	// Constraints - Decl bound [/string, /string]
	if len(w.Constraints) > 0 {
		sb.WriteString("# Constraints\n")
		for i, c := range w.Constraints {
			constraintID := fmt.Sprintf("constraint_%d", i+1)
			sb.WriteString(fmt.Sprintf("northstar_constraint(%q, %q).\n", constraintID, c))
		}
	}

	return sb.String()
}

func capabilityPriorityToNumber(p string) int {
	switch strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(p, "-", "_"), " ", "_")) {
	case "critical", "must_have":
		return 100
	case "high", "should_have":
		return 80
	case "medium":
		return 50
	case "low", "nice_to_have":
		return 20
	default:
		return 50
	}
}

func riskImpactToNumber(impact string) int {
	switch strings.ToLower(strings.TrimSpace(impact)) {
	case "high":
		return 100
	case "medium":
		return 50
	case "low":
		return 20
	default:
		return 50
	}
}
// saveNorthstarToKnowledgeBase stores northstar data in the knowledge database
// Returns a slice of errors encountered during storage (empty if all succeeded)
func saveNorthstarToKnowledgeBase(db interface {
	StoreKnowledgeAtom(concept, content string, confidence float64) error
}, w *NorthstarWizardState) []error {
	var errs []error

	// Store mission
	if err := db.StoreKnowledgeAtom("northstar:mission", w.Mission, 1.0); err != nil {
		errs = append(errs, err)
	}
	if err := db.StoreKnowledgeAtom("northstar:problem", w.Problem, 1.0); err != nil {
		errs = append(errs, err)
	}
	if err := db.StoreKnowledgeAtom("northstar:vision", w.Vision, 1.0); err != nil {
		errs = append(errs, err)
	}

	// Store personas
	for _, p := range w.Personas {
		if err := db.StoreKnowledgeAtom("northstar:persona", fmt.Sprintf("%s: %s", p.Name, strings.Join(p.Needs, ", ")), 0.9); err != nil {
			errs = append(errs, err)
		}
	}

	// Store capabilities
	for _, c := range w.Capabilities {
		if err := db.StoreKnowledgeAtom("northstar:capability", fmt.Sprintf("[%s/%s] %s", c.Timeline, c.Priority, c.Description), 0.9); err != nil {
			errs = append(errs, err)
		}
	}

	// Store risks
	for _, r := range w.Risks {
		if err := db.StoreKnowledgeAtom("northstar:risk", fmt.Sprintf("[%s/%s] %s - Mitigation: %s", r.Likelihood, r.Impact, r.Description, r.Mitigation), 0.85); err != nil {
			errs = append(errs, err)
		}
	}

	// Store requirements
	for _, r := range w.Requirements {
		if err := db.StoreKnowledgeAtom("northstar:requirement", fmt.Sprintf("[%s] %s: %s (%s)", r.ID, r.Type, r.Description, r.Priority), 0.95); err != nil {
			errs = append(errs, err)
		}
	}

	// Store constraints
	for _, c := range w.Constraints {
		if err := db.StoreKnowledgeAtom("northstar:constraint", c, 0.9); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

// =============================================================================
// KERNEL FACT ASSERTION
// =============================================================================

// assertNorthstarFacts injects ALL northstar facts into the kernel for reasoning
// Returns a slice of errors encountered during assertion (empty if all succeeded)
func assertNorthstarFacts(kernel interface{ AssertString(fact string) error }, w *NorthstarWizardState) []error {
	var errs []error

	// Helper to assert and collect errors
	assert := func(fact string) {
		if err := kernel.AssertString(fact); err != nil {
			errs = append(errs, err)
		}
	}

	// Mark that northstar is defined
	assert("northstar_defined().")

	// Core vision facts
	assert(fmt.Sprintf("northstar_mission(%q, %q).", "global", w.Mission))
	assert(fmt.Sprintf("northstar_problem(%q, %q).", "global", w.Problem))
	assert(fmt.Sprintf("northstar_vision(%q, %q).", "global", w.Vision))

	// Personas with pain points and needs
	for i, p := range w.Personas {
		personaID := fmt.Sprintf("persona_%d", i+1)
		assert(fmt.Sprintf("northstar_persona(%q, %q).", personaID, p.Name))
		for _, pain := range p.PainPoints {
			assert(fmt.Sprintf("northstar_pain_point(%q, %q).", personaID, pain))
		}
		for _, need := range p.Needs {
			assert(fmt.Sprintf("northstar_need(%q, %q).", personaID, need))
		}
	}

	// Capabilities - /string /string /name /number
	for i, c := range w.Capabilities {
		capID := fmt.Sprintf("cap_%d", i+1)
		timeline := strings.ToLower(strings.ReplaceAll(c.Timeline, " ", "_"))
		priority := capabilityPriorityToNumber(c.Priority)
		assert(fmt.Sprintf("northstar_capability(%q, %q, /%s, %d).", capID, c.Description, timeline, priority))
	}

	// Risks and mitigations
	for i, r := range w.Risks {
		riskID := fmt.Sprintf("risk_%d", i+1)
		likelihood := strings.ToLower(r.Likelihood)
		impact := riskImpactToNumber(r.Impact)
		assert(fmt.Sprintf("northstar_risk(%q, %q, /%s, %d).", riskID, r.Description, likelihood, impact))
		if r.Mitigation != "" && r.Mitigation != "none" {
			assert(fmt.Sprintf("northstar_mitigation(%q, /mitigation).", riskID))
		}
	}

	// Requirements - /string /name /string /number
	for _, r := range w.Requirements {
		reqID := strings.ToLower(r.ID)
		reqType := strings.ToLower(strings.ReplaceAll(r.Type, " ", "_"))
		reqType = strings.ReplaceAll(reqType, "-", "_")
		priority := capabilityPriorityToNumber(r.Priority)
		assert(fmt.Sprintf("northstar_requirement(%q, /%s, %q, %d).", reqID, reqType, r.Description, priority))
	}

	// Constraints
	for i, c := range w.Constraints {
		constraintID := fmt.Sprintf("constraint_%d", i+1)
		assert(fmt.Sprintf("northstar_constraint(%q, %q).", constraintID, c))
	}

	return errs
}