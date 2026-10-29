// Package chat provides the interactive TUI chat interface for codeNERD.
// This file binds the Northstar wizard to the canonical vision it edits.
package chat

import (
	"fmt"
	"path/filepath"
	"strings"

	"codenerd/internal/northstar"
)

// northstarEdit is what the wizard is a view of: the canonical vision it was
// opened on, read from the authority. The wizard shows and edits only part of
// a vision (no capability links, no mitigation evidence or waivers), so saving
// the view alone would erase the rest; the save merges the view's edits into
// this base instead, and the authority refuses the save if it has moved since.
type northstarEdit struct {
	// vision is the authority as read, detached; nil when no vision existed.
	vision *northstar.Vision
	// replace marks a deliberate "start fresh": the view is the whole new
	// vision. The base still guards the write, so a start-fresh over a vision
	// that changed in the meantime is refused like any other stale save.
	replace bool
}

// loadExistingNorthstar opens the wizard on the authoritative vision. The
// authority is the Northstar store (after the JSON import surface is
// reconciled into it), not .nerd/northstar.json: the JSON is an export a human
// may have edited, and reading it here is what let an unvalidated, possibly
// stale file stand in for the vision.
//
// It reports (nil, false, nil) when no vision exists.
func loadExistingNorthstar(workspace string) (*NorthstarWizardState, bool, error) {
	guardian, err := northstar.AcquireGuardian(filepath.Join(workspace, ".nerd"), northstar.GuardianConfigFor(workspace))
	if err != nil {
		return nil, false, fmt.Errorf("open northstar: %w", err)
	}
	defer func() { _ = northstar.ReleaseGuardian(guardian) }()

	vision, _, err := guardian.SnapshotVision()
	if err != nil {
		return nil, false, fmt.Errorf("read northstar: %w", err)
	}
	if vision == nil {
		return nil, false, nil
	}
	return wizardFromVision(vision), true, nil
}

// wizardFromVision builds the wizard's view of a canonical vision and keeps the
// vision as the edit base. It takes ownership of v; callers pass a detached copy.
func wizardFromVision(v *northstar.Vision) *NorthstarWizardState {
	w := NewNorthstarWizard()
	w.Mission, w.Problem, w.Vision = v.Mission, v.Problem, v.VisionStmt
	w.Constraints = append(w.Constraints, v.Constraints...)
	for _, p := range v.Personas {
		w.Personas = append(w.Personas, UserPersona{
			Name:       p.Name,
			PainPoints: append([]string{}, p.PainPoints...),
			Needs:      append([]string{}, p.Needs...),
		})
	}
	for _, c := range v.Capabilities {
		w.Capabilities = append(w.Capabilities, Capability{ID: c.ID, Description: c.Description, Timeline: c.Timeline, Priority: c.Priority})
	}
	for _, r := range v.Risks {
		w.Risks = append(w.Risks, Risk{ID: r.ID, Description: r.Description, Likelihood: r.Likelihood, Impact: r.Impact, Mitigation: r.Mitigation})
	}
	for _, r := range v.Requirements {
		w.Requirements = append(w.Requirements, NorthstarRequirement{
			ID: r.ID, Type: r.Type, Description: r.Description, Priority: r.Priority, Source: r.Source,
			Supports:  append([]string(nil), r.Supports...),
			Addresses: append([]string(nil), r.Addresses...),
		})
	}
	w.edit = northstarEdit{vision: v}
	return w
}

// startReplacement is the wizard for "start fresh": empty, but still bound to
// the vision it will replace.
func (w *NorthstarWizardState) startReplacement() *NorthstarWizardState {
	fresh := NewNorthstarWizard()
	fresh.edit = northstarEdit{vision: w.edit.vision, replace: true}
	return fresh
}

// ensureIDs gives every capability and risk created in the wizard its stable
// ID. Existing IDs never change; a new one is the lowest free number, so it
// cannot collide with an entity the view holds or the base retains.
func (w *NorthstarWizardState) ensureIDs() {
	var capIDs, riskIDs []string
	for _, c := range w.Capabilities {
		capIDs = append(capIDs, c.ID)
	}
	for _, r := range w.Risks {
		riskIDs = append(riskIDs, r.ID)
	}
	if base := w.baseForMerge(); base != nil {
		for _, c := range base.Capabilities {
			capIDs = append(capIDs, c.ID)
		}
		for _, r := range base.Risks {
			riskIDs = append(riskIDs, r.ID)
		}
	}
	for i := range w.Capabilities {
		if strings.TrimSpace(w.Capabilities[i].ID) == "" {
			id := northstar.MintID("cap_", capIDs)
			w.Capabilities[i].ID = id
			capIDs = append(capIDs, id)
		}
	}
	for i := range w.Risks {
		if strings.TrimSpace(w.Risks[i].ID) == "" {
			id := northstar.MintID("risk_", riskIDs)
			w.Risks[i].ID = id
			riskIDs = append(riskIDs, id)
		}
	}
}

// nextRequirementID is the lowest free REQ-nnn. It looks at the IDs in the
// wizard and in the base, so a requirement created after others were removed
// or generated elsewhere never reuses an ID.
func (w *NorthstarWizardState) nextRequirementID() string {
	var used []string
	for _, r := range w.Requirements {
		used = append(used, r.ID)
	}
	if base := w.baseForMerge(); base != nil {
		for _, r := range base.Requirements {
			used = append(used, r.ID)
		}
	}
	taken := make(map[string]struct{}, len(used))
	for _, id := range used {
		taken[strings.ToLower(strings.TrimSpace(id))] = struct{}{}
	}
	for n := 1; ; n++ {
		id := fmt.Sprintf("REQ-%03d", n)
		if _, clash := taken[strings.ToLower(id)]; !clash {
			return id
		}
	}
}

// baseForMerge is the base a save merges into: none when starting fresh.
func (w *NorthstarWizardState) baseForMerge() *northstar.Vision {
	if w.edit.replace {
		return nil
	}
	return w.edit.vision
}

// view renders the wizard as the document the canonical merge understands.
// Fields the wizard does not hold (capability serves, mitigation status,
// evidence, waivers) stay unset, which the merge reads as "not edited".
func (w *NorthstarWizardState) view() *northstar.WizardDocument {
	doc := &northstar.WizardDocument{
		Mission:      w.Mission,
		Problem:      w.Problem,
		Vision:       w.Vision,
		Personas:     []northstar.WizardPersona{},
		Constraints:  append([]string{}, w.Constraints...),
		Capabilities: make([]northstar.WizardCapability, 0, len(w.Capabilities)),
		Risks:        make([]northstar.WizardRisk, 0, len(w.Risks)),
		Requirements: make([]northstar.WizardRequirement, 0, len(w.Requirements)),
	}
	for _, p := range w.Personas {
		doc.Personas = append(doc.Personas, northstar.WizardPersona{Name: p.Name, PainPoints: p.PainPoints, Needs: p.Needs})
	}
	for _, c := range w.Capabilities {
		doc.Capabilities = append(doc.Capabilities, northstar.WizardCapability{ID: c.ID, Description: c.Description, Timeline: c.Timeline, Priority: c.Priority})
	}
	for _, r := range w.Risks {
		doc.Risks = append(doc.Risks, northstar.WizardRisk{ID: r.ID, Description: r.Description, Likelihood: r.Likelihood, Impact: r.Impact, Mitigation: r.Mitigation})
	}
	for _, r := range w.Requirements {
		doc.Requirements = append(doc.Requirements, northstar.WizardRequirement{
			ID: r.ID, Type: r.Type, Description: r.Description, Priority: r.Priority, Source: r.Source,
			Supports: r.Supports, Addresses: r.Addresses,
		})
	}
	return doc
}

// candidate is the vision this wizard proposes to save: its view merged into
// the base it was opened on.
func (w *NorthstarWizardState) candidate() (*northstar.Vision, error) {
	w.ensureIDs()
	return northstar.MergeViewEdit(w.baseForMerge(), w.view())
}

// commitNorthstarVision saves the wizard's edit through the Guardian. It is
// the only writer of the wizard's result: the Guardian owns the JSON and .mg
// exports and the kernel projection, all derived from the committed vision,
// so the wizard no longer encodes any of them itself.
//
// On success the wizard is rebased onto the committed vision, so saving again
// is an edit of what was just saved rather than a stale one.
func commitNorthstarVision(workspace string, w *NorthstarWizardState, kernel northstar.KernelClient) (northstar.VisionUpdate, error) {
	candidate, err := w.candidate()
	if err != nil {
		return northstar.VisionUpdate{}, err
	}
	guardian, err := northstar.AcquireGuardian(filepath.Join(workspace, ".nerd"), northstar.GuardianConfigFor(workspace))
	if err != nil {
		return northstar.VisionUpdate{}, fmt.Errorf("open guardian: %w", err)
	}
	defer func() { _ = northstar.ReleaseGuardian(guardian) }()

	// The kernel is wired so the Guardian's retract-then-assert replaces the
	// northstar_* facts with the canonical projection of the committed vision.
	if kernel != nil {
		guardian.SetParentKernel(kernel)
	}

	update, err := guardian.UpdateVisionFromBase(candidate, w.edit.vision)
	if err != nil {
		return northstar.VisionUpdate{}, err
	}
	w.edit = northstarEdit{vision: update.Vision}
	return update, nil
}
