package chat

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"codenerd/internal/northstar"
)

const northstarTestSHA = "4444444444444444444444444444444444444444"

// richNorthstar is a vision the wizard cannot display in full.
func richNorthstar() *northstar.Vision {
	return &northstar.Vision{
		Mission: "Keep the kernel the executive", Problem: "Improvised control flow", VisionStmt: "Facts decide",
		Personas: []northstar.Persona{{Name: "Engineer", PainPoints: []string{"drift"}, Needs: []string{"evidence"}}},
		Capabilities: []northstar.Capability{
			{ID: "cap_policy", Description: "Policy in Mangle", Timeline: "now", Priority: "critical", Serves: []string{"Engineer"}},
			{ID: "cap_audit", Description: "Audit trail", Timeline: "6mo", Priority: "high"},
		},
		Risks: []northstar.Risk{
			{ID: "risk_drift", Description: "Policy drift", Likelihood: "high", Impact: "high", Mitigation: "golden gate",
				MitigationStatus: northstar.MitigationVerified,
				MitigationEvidence: []northstar.MitigationEvidence{
					{Kind: "test", Ref: "go test ./internal/core", Outcome: northstar.EvidencePassed, Revision: northstarTestSHA}}},
			{ID: "risk_cost", Description: "Cost", Likelihood: "low", Impact: "medium", Mitigation: "accepted for now",
				MitigationStatus: northstar.MitigationWaived,
				Waiver:           &northstar.RiskWaiver{Authority: "owner", Scope: "q4", Rationale: "cheap", Expires: time.Date(2031, 1, 2, 3, 4, 5, 0, time.UTC)}},
		},
		Requirements: []northstar.Requirement{
			{ID: "req-001", Type: "functional", Description: "Derive actions in Mangle", Priority: "must-have", Source: "user",
				Supports: []string{"cap_policy"}, Addresses: []string{"risk_drift"}},
			{ID: "req-002", Type: "non-functional", Description: "Keep a trail", Priority: "should-have", Source: "auto-generated",
				Supports: []string{"cap_audit"}},
		},
		Constraints: []string{"offline"},
	}
}

// seedNorthstar writes v as the authority of a fresh workspace, the way any
// canonical writer would, and returns the workspace.
func seedNorthstar(t *testing.T, v *northstar.Vision) string {
	t.Helper()
	t.Cleanup(northstar.ResetGuardianRegistry)
	workspace := t.TempDir()
	g := acquireNorthstar(t, workspace)
	if err := g.UpdateVision(v); err != nil {
		t.Fatal(err)
	}
	return workspace
}

func acquireNorthstar(t *testing.T, workspace string) *northstar.Guardian {
	t.Helper()
	g, err := northstar.AcquireGuardian(filepath.Join(workspace, ".nerd"), northstar.DefaultGuardianConfig())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = northstar.ReleaseGuardian(g) })
	return g
}

func openNorthstar(t *testing.T, workspace string) *NorthstarWizardState {
	t.Helper()
	w, ok, err := loadExistingNorthstar(workspace)
	if err != nil || !ok {
		t.Fatalf("loadExistingNorthstar: ok=%v err=%v", ok, err)
	}
	return w
}

func storedNorthstar(t *testing.T, workspace string) *northstar.Vision {
	t.Helper()
	v, _, err := acquireNorthstar(t, workspace).SnapshotVision()
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestLoadExistingNorthstar_ReadsTheAuthorityNotTheJSONExport(t *testing.T) {
	workspace := seedNorthstar(t, richNorthstar())
	// A stale export that disagrees with the store must not be what the wizard shows.
	stale := richNorthstar()
	stale.Mission = "stale export"
	if _, err := northstar.WriteVisionJSON(filepath.Join(workspace, ".nerd"), stale); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(workspace, ".nerd", northstar.VisionJSONFileName), old, old); err != nil {
		t.Fatal(err)
	}

	w := openNorthstar(t, workspace)
	if w.Mission != "Keep the kernel the executive" {
		t.Fatalf("wizard opened on the stale export: %q", w.Mission)
	}
	if w.Capabilities[0].ID != "cap_policy" || w.Risks[0].ID != "risk_drift" || w.Requirements[0].Supports[0] != "cap_policy" || w.Requirements[0].Source != "user" {
		t.Fatalf("identity and links lost on open: %+v", w)
	}
	if w.edit.vision == nil {
		t.Fatal("the canonical base was not retained")
	}
}

func TestLoadExistingNorthstar_NoVision(t *testing.T) {
	t.Cleanup(northstar.ResetGuardianRegistry)
	w, ok, err := loadExistingNorthstar(t.TempDir())
	if err != nil || ok || w != nil {
		t.Fatalf("empty workspace: %v %v %v", w, ok, err)
	}
}

func TestLoadExistingNorthstar_ImportsAHandWrittenJSONSurface(t *testing.T) {
	t.Cleanup(northstar.ResetGuardianRegistry)
	workspace := t.TempDir()
	if _, err := northstar.WriteVisionJSON(filepath.Join(workspace, ".nerd"), richNorthstar()); err != nil {
		t.Fatal(err)
	}
	w := openNorthstar(t, workspace)
	if w.edit.vision == nil || w.Capabilities[0].ID != "cap_policy" {
		t.Fatalf("JSON-only vision was not reconciled into the authority: %+v", w)
	}
}

// The audit's central defect: open a rich vision, change nothing, save.
func TestNorthstarEdit_UnchangedSaveIsANoOp(t *testing.T) {
	workspace := seedNorthstar(t, richNorthstar())
	before := storedNorthstar(t, workspace)
	exportStat, err := os.Stat(filepath.Join(workspace, ".nerd", northstar.VisionJSONFileName))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)

	w := openNorthstar(t, workspace)
	update, err := commitNorthstarVision(workspace, w, nil)
	if err != nil {
		t.Fatal(err)
	}
	if update.Outcome != northstar.VisionUnchanged {
		t.Fatalf("outcome = %s, changes %v", update.Outcome, update.Changes)
	}
	after := storedNorthstar(t, workspace)
	if !after.UpdatedAt.Equal(before.UpdatedAt) || !northstar.VisionsEquivalent(before, after) {
		t.Fatal("an unchanged save rewrote the authority")
	}
	again, _ := os.Stat(filepath.Join(workspace, ".nerd", northstar.VisionJSONFileName))
	if !again.ModTime().Equal(exportStat.ModTime()) {
		t.Fatal("an unchanged save rewrote the JSON export")
	}
}

func TestNorthstarEdit_OneFieldEditKeepsLinksEvidenceWaiverAndIDs(t *testing.T) {
	workspace := seedNorthstar(t, richNorthstar())
	w := openNorthstar(t, workspace)
	w.Capabilities[1].Description = "Audit trail with receipts"

	update, err := commitNorthstarVision(workspace, w, nil)
	if err != nil {
		t.Fatal(err)
	}
	if update.Outcome != northstar.VisionCommitted || !reflect.DeepEqual(update.Changes, []string{"capability cap_audit changed"}) {
		t.Fatalf("update = %+v", update)
	}
	for name, v := range map[string]*northstar.Vision{
		"store":  storedNorthstar(t, workspace),
		"export": mustLoadExport(t, workspace),
	} {
		if v.Capabilities[1].Description != "Audit trail with receipts" ||
			v.Capabilities[0].ID != "cap_policy" || v.Capabilities[0].Serves[0] != "Engineer" ||
			v.Requirements[0].Supports[0] != "cap_policy" || v.Requirements[0].Addresses[0] != "risk_drift" || v.Requirements[0].Source != "user" ||
			len(v.Risks[0].MitigationEvidence) != 1 || v.Risks[0].MitigationStatus != northstar.MitigationVerified ||
			v.Risks[1].Waiver == nil || v.Risks[1].Waiver.Authority != "owner" {
			t.Fatalf("%s lost strategic data on a one-field edit: %+v", name, v)
		}
	}
}

func mustLoadExport(t *testing.T, workspace string) *northstar.Vision {
	t.Helper()
	v, err := northstar.LoadVisionJSON(filepath.Join(workspace, ".nerd"))
	if err != nil || v == nil {
		t.Fatalf("export: %v %v", v, err)
	}
	return v
}

func TestNorthstarEdit_ReorderedViewKeepsIdentity(t *testing.T) {
	workspace := seedNorthstar(t, richNorthstar())
	before := storedNorthstar(t, workspace)
	w := openNorthstar(t, workspace)
	w.Capabilities[0], w.Capabilities[1] = w.Capabilities[1], w.Capabilities[0]
	w.Risks[0], w.Risks[1] = w.Risks[1], w.Risks[0]

	update, err := commitNorthstarVision(workspace, w, nil)
	if err != nil || update.Outcome != northstar.VisionUnchanged {
		t.Fatalf("reordering retargeted entities: %+v %v", update, err)
	}
	if !northstar.VisionsEquivalent(before, storedNorthstar(t, workspace)) {
		t.Fatal("reorder changed the authority")
	}
}

func TestNorthstarEdit_AddedEntitiesGetStableIDsAndLinks(t *testing.T) {
	workspace := seedNorthstar(t, richNorthstar())
	w := openNorthstar(t, workspace)
	w.Capabilities = append(w.Capabilities, Capability{Description: "Replay", Timeline: "now", Priority: "high"})
	w.Risks = append(w.Risks, Risk{Description: "Flaky gate", Likelihood: "medium", Impact: "high", Mitigation: "retry"})
	w.Requirements = append(w.Requirements, NorthstarRequirement{ID: w.nextRequirementID(), Type: "functional", Description: "Replay runs", Priority: "must-have", Source: "user"})
	w.ensureIDs()
	w.Requirements[2].Supports = []string{w.Capabilities[2].ID}
	w.Requirements[2].Addresses = []string{w.Risks[2].ID}

	if w.Capabilities[2].ID != "cap_1" || w.Risks[2].ID != "risk_1" || w.Requirements[2].ID != "REQ-003" {
		t.Fatalf("minted IDs = %s %s %s", w.Capabilities[2].ID, w.Risks[2].ID, w.Requirements[2].ID)
	}
	if _, err := commitNorthstarVision(workspace, w, nil); err != nil {
		t.Fatal(err)
	}
	got := storedNorthstar(t, workspace)
	if got.Requirements[2].Supports[0] != "cap_1" || got.Requirements[2].Addresses[0] != "risk_1" || got.Requirements[0].Supports[0] != "cap_policy" {
		t.Fatalf("links after add: %+v", got.Requirements)
	}
}

// A requirement ID minted by position collided with an existing ID as soon as
// anything had been removed or generated elsewhere.
func TestNextRequirementID_NeverReusesAnExistingID(t *testing.T) {
	w := NewNorthstarWizard()
	w.Requirements = []NorthstarRequirement{{ID: "REQ-002"}, {ID: "req-001"}, {ID: "REQ-004"}}
	if got := w.nextRequirementID(); got != "REQ-003" {
		t.Fatalf("next = %s", got)
	}
	w.edit.vision = &northstar.Vision{Requirements: []northstar.Requirement{{ID: "req-003"}}}
	if got := w.nextRequirementID(); got != "REQ-005" {
		t.Fatalf("next with a retained base requirement = %s", got)
	}
}

func TestNorthstarEdit_StaleBaseIsRefusedWithoutAnyWrite(t *testing.T) {
	workspace := seedNorthstar(t, richNorthstar())
	w := openNorthstar(t, workspace)
	w.Mission = "Editor's mission"

	// The CLI import lands while the wizard is open.
	cli := richNorthstar()
	cli.Constraints = append(cli.Constraints, "added by the CLI")
	if err := acquireNorthstar(t, workspace).UpdateVision(cli); err != nil {
		t.Fatal(err)
	}
	exportBefore, _ := os.ReadFile(filepath.Join(workspace, ".nerd", northstar.VisionJSONFileName))

	_, err := commitNorthstarVision(workspace, w, nil)
	var stale *northstar.BaseStaleError
	if !errors.As(err, &stale) {
		t.Fatalf("stale edit accepted: %v", err)
	}
	if !reflect.DeepEqual(stale.Changes, []string{"constraints changed"}) {
		t.Fatalf("comparison = %v", stale.Changes)
	}
	now := storedNorthstar(t, workspace)
	if now.Mission != cli.Mission || len(now.Constraints) != 2 {
		t.Fatalf("authority lost the concurrent write: %+v", now)
	}
	exportAfter, _ := os.ReadFile(filepath.Join(workspace, ".nerd", northstar.VisionJSONFileName))
	if string(exportBefore) != string(exportAfter) {
		t.Fatal("a refused edit rewrote the export")
	}
}

func TestNorthstarEdit_StartFreshReplacesButStillRefusesAStaleBase(t *testing.T) {
	workspace := seedNorthstar(t, richNorthstar())
	old := openNorthstar(t, workspace)

	fresh := old.startReplacement()
	fresh.Mission, fresh.Problem, fresh.Vision = "A new mission", "A new problem", "A new vision"
	fresh.Capabilities = []Capability{{Description: "Only capability", Timeline: "now", Priority: "high"}}
	update, err := commitNorthstarVision(workspace, fresh, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := storedNorthstar(t, workspace)
	if update.Outcome != northstar.VisionCommitted || got.Mission != "A new mission" || len(got.Capabilities) != 1 ||
		got.Capabilities[0].ID != "cap_1" || len(got.Risks) != 0 || len(got.Requirements) != 0 {
		t.Fatalf("start-fresh did not replace: %+v", got)
	}

	// A second start-fresh built on the vision that was just replaced is stale.
	again := old.startReplacement()
	again.Mission = "Another"
	if _, err := commitNorthstarVision(workspace, again, nil); !errors.Is(err, northstar.ErrVisionBaseStale) {
		t.Fatalf("start-fresh over a changed vision: %v", err)
	}
}

func TestNorthstarEdit_CreateIsRefusedOnceAVisionExists(t *testing.T) {
	t.Cleanup(northstar.ResetGuardianRegistry)
	workspace := t.TempDir()
	w := NewNorthstarWizard()
	w.Mission = "First"

	if err := acquireNorthstar(t, workspace).UpdateVision(richNorthstar()); err != nil {
		t.Fatal(err)
	}
	if _, err := commitNorthstarVision(workspace, w, nil); !errors.Is(err, northstar.ErrVisionBaseStale) {
		t.Fatalf("a wizard opened on nothing overwrote a vision that appeared: %v", err)
	}
}

func TestNorthstarEdit_InvalidCandidateChangesNothing(t *testing.T) {
	workspace := seedNorthstar(t, richNorthstar())
	before := storedNorthstar(t, workspace)
	w := openNorthstar(t, workspace)
	w.Requirements[0].Supports = []string{"cap_ghost"}

	_, err := commitNorthstarVision(workspace, w, nil)
	if !errors.Is(err, northstar.ErrVisionImportRejected) {
		t.Fatalf("invalid candidate: %v", err)
	}
	if !northstar.VisionsEquivalent(before, storedNorthstar(t, workspace)) {
		t.Fatal("a rejected edit changed the authority")
	}
}

func TestNorthstarEdit_CommittedWizardRebasesSoSavingAgainIsNotStale(t *testing.T) {
	workspace := seedNorthstar(t, richNorthstar())
	w := openNorthstar(t, workspace)
	w.Problem = "First edit"
	if _, err := commitNorthstarVision(workspace, w, nil); err != nil {
		t.Fatal(err)
	}
	w.Problem = "Second edit"
	update, err := commitNorthstarVision(workspace, w, nil)
	if err != nil || update.Outcome != northstar.VisionCommitted {
		t.Fatalf("second save: %+v %v", update, err)
	}
}

func TestHeuristicRequirements_AreLinkedToTheirCapabilityAndRisk(t *testing.T) {
	w := NewNorthstarWizard()
	w.Capabilities = []Capability{{Description: "Replay", Timeline: "now", Priority: "critical"}}
	w.Risks = []Risk{{Description: "Drift", Likelihood: "high", Impact: "high", Mitigation: "gate"}}
	m := InitChat(Config{})
	m.northstarWizard = w
	m.autoGenerateRequirementsHeuristic()

	if len(w.Requirements) != 2 {
		t.Fatalf("requirements = %+v", w.Requirements)
	}
	if !reflect.DeepEqual(w.Requirements[0].Supports, []string{"cap_1"}) || !reflect.DeepEqual(w.Requirements[1].Addresses, []string{"risk_1"}) {
		t.Fatalf("heuristic requirements are unlinked: %+v", w.Requirements)
	}
	if w.Requirements[0].ID == w.Requirements[1].ID {
		t.Fatal("generated requirements share an ID")
	}
}

func TestSaveNorthstar_ReportsConflictAndKeepsTheStoredVision(t *testing.T) {
	workspace := seedNorthstar(t, richNorthstar())
	m := InitChat(Config{})
	m.workspace = workspace
	m.setInputMode(InputModeNorthstar)
	w := openNorthstar(t, workspace)
	w.Mission = "Editor's mission"
	m.northstarWizard = w

	if err := acquireNorthstar(t, workspace).UpdateVision(func() *northstar.Vision { v := richNorthstar(); v.Problem = "moved"; return v }()); err != nil {
		t.Fatal(err)
	}
	next, cmd := m.saveNorthstar(true)
	got := next.(Model)
	if cmd != nil {
		t.Fatal("a refused save started a campaign")
	}
	if got.inputMode == InputModeNorthstar {
		t.Fatal("a conflicted wizard stayed open on a vision that no longer exists")
	}
	last := got.history[len(got.history)-1].Content
	if !strings.Contains(last, "Not Saved") || !strings.Contains(last, "problem changed") {
		t.Fatalf("conflict message = %q", last)
	}
	if storedNorthstar(t, workspace).Mission != "Keep the kernel the executive" {
		t.Fatal("conflicted wizard overwrote the stored vision")
	}
}

func TestSaveNorthstar_InvalidEditKeepsTheWizardOpen(t *testing.T) {
	workspace := seedNorthstar(t, richNorthstar())
	m := InitChat(Config{})
	m.workspace = workspace
	m.setInputMode(InputModeNorthstar)
	w := openNorthstar(t, workspace)
	w.Requirements[0].Supports = []string{"cap_ghost"}
	m.northstarWizard = w

	next, cmd := m.saveNorthstar(false)
	got := next.(Model)
	if cmd != nil || got.inputMode != InputModeNorthstar || got.northstarWizard == nil {
		t.Fatal("an invalid edit closed the wizard")
	}
	if last := got.history[len(got.history)-1].Content; !strings.Contains(last, "Not Saved") || !strings.Contains(last, "cap_ghost") {
		t.Fatalf("message = %q", last)
	}
}

func TestSaveNorthstar_UnchangedAndCommittedMessages(t *testing.T) {
	workspace := seedNorthstar(t, richNorthstar())
	m := InitChat(Config{})
	m.workspace = workspace

	m.setInputMode(InputModeNorthstar)
	m.northstarWizard = openNorthstar(t, workspace)
	next, _ := m.saveNorthstar(false)
	if last := next.(Model).history[len(next.(Model).history)-1].Content; !strings.Contains(last, "Unchanged") {
		t.Fatalf("no-op message = %q", last)
	}

	m = InitChat(Config{})
	m.workspace = workspace
	m.setInputMode(InputModeNorthstar)
	w := openNorthstar(t, workspace)
	w.Constraints = append(w.Constraints, "must run offline")
	m.northstarWizard = w
	next, _ = m.saveNorthstar(false)
	last := next.(Model).history[len(next.(Model).history)-1].Content
	if !strings.Contains(last, "Saved") || !strings.Contains(last, "constraints changed") {
		t.Fatalf("commit message = %q", last)
	}
}
