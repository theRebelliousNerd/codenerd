// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains persistence and storage functions for the Northstar wizard.
package chat

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/northstar"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// NORTHSTAR PERSISTENCE
// =============================================================================

// saveNorthstar commits the wizard's edit and optionally starts a campaign.
//
// The order is the contract: the edit is validated and committed to the
// authority first (Guardian.UpdateVisionFromBase), and only a committed vision
// is projected anywhere. A rejected or conflicted edit therefore leaves the
// store, the .mg and JSON exports, the knowledge atoms and the kernel exactly
// as they were, and an edit equal to the stored vision rewrites nothing.
func (m Model) saveNorthstar(startCampaign bool) (tea.Model, tea.Cmd) {
	w := m.northstarWizard

	// m.kernel is a typed pointer: pass a nil INTERFACE when it is unset, or
	// the Guardian's refreshKernelFacts would call Retract on a nil *core.RealKernel.
	var visionKernel northstar.KernelClient
	if m.kernel != nil {
		visionKernel = m.kernel
	}

	update, err := commitNorthstarVision(m.workspace, w, visionKernel)
	if err != nil {
		return m.northstarSaveRefused(err), nil
	}

	var warnings []string
	if update.Outcome != northstar.VisionUnchanged {
		if m.localDB != nil {
			if errs := saveNorthstarToKnowledgeBase(m.localDB, update.Vision); len(errs) > 0 {
				warnings = append(warnings, fmt.Sprintf("Knowledge base: %d item(s) failed to save", len(errs)))
			}
		} else {
			warnings = append(warnings, "Knowledge database not available")
		}
		if m.kernel == nil {
			warnings = append(warnings, "Kernel not available - facts not loaded")
		}
	}
	for _, perr := range update.ProjectionErrors {
		warnings = append(warnings, fmt.Sprintf("Projection pending: %v", perr))
	}

	// Exit wizard mode
	m.setInputMode(InputModeNormal)
	m.textarea.Placeholder = "Ask me anything... (Enter to send, Alt+Enter for newline, Ctrl+C to exit)"

	nerdDir := filepath.Join(m.workspace, ".nerd")
	northstarPath := filepath.Join(nerdDir, northstar.VisionMangleFileName)
	jsonPath := filepath.Join(nerdDir, northstar.VisionJSONFileName)

	var msgBuilder strings.Builder
	switch update.Outcome {
	case northstar.VisionUnchanged:
		msgBuilder.WriteString("## Northstar Unchanged\n\nNothing differs from the stored vision, so nothing was rewritten.\n\n")
	case northstar.VisionProjectionPending:
		msgBuilder.WriteString("## Northstar Saved - Projection Pending\n\nThe vision is committed, but not every copy of it was updated:\n")
	default:
		msgBuilder.WriteString("## Northstar Saved!\n\nYour vision has been stored to:\n")
		msgBuilder.WriteString(fmt.Sprintf("- `%s` (Mangle facts)\n", northstarPath))
		msgBuilder.WriteString(fmt.Sprintf("- `%s` (JSON export)\n", jsonPath))
		msgBuilder.WriteString("- Knowledge database (semantic search)\n\n")
	}
	if len(update.Changes) > 0 {
		msgBuilder.WriteString("**Changed:** " + strings.Join(update.Changes, "; ") + "\n\n")
	}
	if len(warnings) > 0 {
		msgBuilder.WriteString("**Warnings:**\n")
		for _, warning := range warnings {
			msgBuilder.WriteString(fmt.Sprintf("- %s\n", warning))
		}
		msgBuilder.WriteString("\n")
	}

	deferCampaign := startCampaign && update.Outcome == northstar.VisionProjectionPending
	switch {
	case startCampaign && !deferCampaign:
		msgBuilder.WriteString("**Starting campaign based on this vision...**\n\n_The Campaign Planner will decompose your vision into actionable phases._")
	case deferCampaign:
		msgBuilder.WriteString("**Campaign not started:** the vision this campaign would read is not fully projected yet. Run `/northstar` and save again, then `/campaign start <goal>`.")
	default:
		msgBuilder.WriteString("The kernel now has access to your vision for reasoning.\n\n")
		msgBuilder.WriteString("**Next steps:**\n")
		msgBuilder.WriteString("- Run `/campaign start <goal>` to start building\n")
		msgBuilder.WriteString("- Run `/query northstar_mission` to query the kernel\n")
		msgBuilder.WriteString("- The vision will inform all future planning")
	}

	m = m.addMessage(Message{
		Role:    "assistant",
		Content: msgBuilder.String(),
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()

	if startCampaign && !deferCampaign {
		goal := fmt.Sprintf("Implement the Northstar vision: %s", update.Vision.Mission)
		m.isLoading = true
		return m, tea.Batch(m.spinner.Tick, m.startCampaign(goal))
	}
	return m, nil
}

// northstarSaveRefused reports a save that changed nothing. Rejected (the
// candidate is invalid) keeps the wizard open so the user can fix it;
// conflicted (the stored vision moved) closes it, because the user's edits were
// made against a vision that no longer exists and re-opening reads the current one.
func (m Model) northstarSaveRefused(err error) Model {
	var msg string
	switch {
	case errors.Is(err, northstar.ErrVisionBaseStale):
		m.setInputMode(InputModeNormal)
		m.textarea.Placeholder = "Ask me anything... (Enter to send, Alt+Enter for newline, Ctrl+C to exit)"
		msg = fmt.Sprintf("## Northstar Not Saved\n\nThe stored vision changed after you opened it, so your edit was not applied and nothing was written.\n\n%v\n\nRun `/northstar` to edit the current version.", err)
	case errors.Is(err, northstar.ErrVisionImportRejected), errors.Is(err, northstar.ErrReassessmentRequired):
		msg = fmt.Sprintf("## Northstar Not Saved\n\nThis edit is not a valid vision, so nothing was written:\n\n%v\n\nType **edit** to change it, or **save** to try again.", err)
	default:
		msg = fmt.Sprintf("## Northstar Not Saved\n\nNothing was written: %v", err)
	}
	m = m.addMessage(Message{Role: "assistant", Content: msg, Time: time.Now()})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m
}

// saveNorthstarToKnowledgeBase stores northstar data in the knowledge database
// Returns a slice of errors encountered during storage (empty if all succeeded)
func saveNorthstarToKnowledgeBase(db interface {
	StoreKnowledgeAtom(concept, content string, confidence float64, source string) error
}, v *northstar.Vision) []error {
	var errs []error

	// Store mission
	if err := db.StoreKnowledgeAtom("northstar:mission", v.Mission, 1.0, "northstar"); err != nil {
		errs = append(errs, err)
	}
	if err := db.StoreKnowledgeAtom("northstar:problem", v.Problem, 1.0, "northstar"); err != nil {
		errs = append(errs, err)
	}
	if err := db.StoreKnowledgeAtom("northstar:vision", v.VisionStmt, 1.0, "northstar"); err != nil {
		errs = append(errs, err)
	}

	// Store personas
	for _, p := range v.Personas {
		if err := db.StoreKnowledgeAtom("northstar:persona", fmt.Sprintf("%s: %s", p.Name, strings.Join(p.Needs, ", ")), 0.9, "northstar"); err != nil {
			errs = append(errs, err)
		}
	}

	// Store capabilities
	for _, c := range v.Capabilities {
		if err := db.StoreKnowledgeAtom("northstar:capability", fmt.Sprintf("[%s/%s] %s", c.Timeline, c.Priority, c.Description), 0.9, "northstar"); err != nil {
			errs = append(errs, err)
		}
	}

	// Store risks
	for _, r := range v.Risks {
		if err := db.StoreKnowledgeAtom("northstar:risk", fmt.Sprintf("[%s/%s] %s - Mitigation: %s", r.Likelihood, r.Impact, r.Description, r.Mitigation), 0.85, "northstar"); err != nil {
			errs = append(errs, err)
		}
	}

	// Store requirements
	for _, r := range v.Requirements {
		if err := db.StoreKnowledgeAtom("northstar:requirement", fmt.Sprintf("[%s] %s: %s (%s)", r.ID, r.Type, r.Description, r.Priority), 0.95, "northstar"); err != nil {
			errs = append(errs, err)
		}
	}

	// Store constraints
	for _, c := range v.Constraints {
		if err := db.StoreKnowledgeAtom("northstar:constraint", c, 0.9, "northstar"); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}
