package northstar

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

const editSHA = "3333333333333333333333333333333333333333"

// richVision is a vision the wizard cannot display in full: custom entity IDs,
// capability and requirement links, a verified risk with evidence, a waived
// risk, and requirement provenance.
func richVision() *Vision {
	return &Vision{
		Mission: "Keep the kernel the executive", Problem: "Improvised control flow", VisionStmt: "Facts decide",
		Personas: []Persona{{Name: "Engineer", PainPoints: []string{"drift"}, Needs: []string{"evidence"}}},
		Capabilities: []Capability{
			{ID: "cap_policy", Description: "Policy in Mangle", Timeline: "now", Priority: "critical", Serves: []string{"Engineer"}},
			{ID: "cap_audit", Description: "Audit trail", Timeline: "6mo", Priority: "high", Serves: []string{"persona_Engineer"}},
		},
		Risks: []Risk{
			{ID: "risk_drift", Description: "Policy drift", Likelihood: "high", Impact: "high", Mitigation: "golden gate",
				MitigationStatus:   MitigationVerified,
				MitigationEvidence: []MitigationEvidence{{Kind: "test", Ref: "go test ./internal/core", Outcome: EvidencePassed, Revision: editSHA}}},
			{ID: "risk_cost", Description: "Cost", Likelihood: "low", Impact: "medium", Mitigation: "accepted for now",
				MitigationStatus: MitigationWaived,
				Waiver:           &RiskWaiver{Authority: "owner", Scope: "q4", Rationale: "cheap", Expires: time.Date(2031, 1, 2, 3, 4, 5, 0, time.UTC)}},
		},
		Requirements: []Requirement{
			{ID: "req-001", Type: "functional", Description: "Derive actions in Mangle", Priority: "must-have", Source: "user",
				Supports: []string{"cap_policy"}, Addresses: []string{"risk_drift"}},
			{ID: "req-002", Type: "non-functional", Description: "Keep a trail", Priority: "should-have", Source: "auto-generated",
				Supports: []string{"cap_audit"}},
		},
		Constraints: []string{"offline"},
	}
}

// wizardView is what an editor that shows only text sends back: the displayed
// fields of every entity and nothing it does not hold.
func wizardView(v *Vision) *WizardDocument {
	d := &WizardDocument{Mission: v.Mission, Problem: v.Problem, Vision: v.VisionStmt, Constraints: append([]string{}, v.Constraints...), Personas: []WizardPersona{}}
	for _, p := range v.Personas {
		d.Personas = append(d.Personas, WizardPersona{Name: p.Name, PainPoints: p.PainPoints, Needs: p.Needs})
	}
	for _, c := range v.Capabilities {
		d.Capabilities = append(d.Capabilities, WizardCapability{ID: c.ID, Description: c.Description, Timeline: c.Timeline, Priority: c.Priority})
	}
	for _, r := range v.Risks {
		d.Risks = append(d.Risks, WizardRisk{ID: r.ID, Description: r.Description, Likelihood: r.Likelihood, Impact: r.Impact, Mitigation: r.Mitigation})
	}
	for _, r := range v.Requirements {
		d.Requirements = append(d.Requirements, WizardRequirement{ID: strings.ToUpper(r.ID), Type: r.Type, Description: r.Description, Priority: r.Priority})
	}
	return d
}

func newEditGuardian(t *testing.T) (*Guardian, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return NewGuardian(store, DefaultGuardianConfig()), dir
}

func seedRich(t *testing.T, g *Guardian) *Vision {
	t.Helper()
	if err := g.UpdateVision(richVision()); err != nil {
		t.Fatal(err)
	}
	v, _, err := g.SnapshotVision()
	if err != nil || v == nil {
		t.Fatalf("snapshot: %v %v", v, err)
	}
	return v
}

// The aliasing the audit named: cloneVision copied the outer capability and
// requirement slices but shared every nested link slice with the original.
func TestCloneVision_DetachesNestedLinkSlices(t *testing.T) {
	orig := richVision()
	clone := cloneVision(orig)
	clone.Capabilities[0].Serves[0] = "mutated"
	clone.Requirements[0].Supports[0] = "mutated"
	clone.Requirements[0].Addresses[0] = "mutated"
	clone.Risks[0].MitigationEvidence[0].Ref = "mutated"
	clone.Risks[1].Waiver.Scope = "mutated"
	if !reflect.DeepEqual(orig, richVision()) {
		t.Fatalf("mutating a clone changed the original:\n%+v", orig)
	}
}

func TestGuardian_GetVisionIsDetachedFromLiveState(t *testing.T) {
	g, _ := newEditGuardian(t)
	seedRich(t, g)
	got := g.GetVision()
	got.Capabilities[0].Serves[0] = "mutated"
	got.Requirements[0].Supports[0] = "mutated"
	again := g.GetVision()
	if again.Capabilities[0].Serves[0] != "Engineer" || again.Requirements[0].Supports[0] != "cap_policy" {
		t.Fatal("a caller's mutation of GetVision reached the guardian's live vision")
	}
}

func TestVisionDigest_IsContentNotClock(t *testing.T) {
	a, b := richVision(), richVision()
	b.CreatedAt, b.UpdatedAt = time.Now(), time.Now()
	b.Capabilities[0].Serves = append([]string(nil), b.Capabilities[0].Serves...)
	if VisionDigest(a) != VisionDigest(b) {
		t.Fatal("timestamps changed the digest")
	}
	b.Risks[0].MitigationEvidence[0].Outcome = EvidenceFailed
	if VisionDigest(a) == VisionDigest(b) {
		t.Fatal("evidence change did not change the digest")
	}
	if VisionDigest(nil) != NoVisionDigest {
		t.Fatal("nil vision must have the no-vision digest")
	}
}

// Foster et al's no-op law: writing an unchanged view back leaves the source
// exactly as it was, including everything the view never showed.
func TestMergeViewEdit_UnchangedViewIsTheBase(t *testing.T) {
	base := richVision()
	got, err := MergeViewEdit(base, wizardView(base))
	if err != nil {
		t.Fatal(err)
	}
	if VisionDigest(got) != VisionDigest(base) || !VisionsEquivalent(got, base) {
		t.Fatalf("no-op view edit changed the vision: %v", DiffVisions(base, got))
	}
	if got.Requirements[0].Supports[0] != "cap_policy" || got.Risks[0].MitigationEvidence[0].Revision != editSHA ||
		got.Risks[1].Waiver == nil || got.Capabilities[0].Serves[0] != "Engineer" || got.Requirements[1].Source != "auto-generated" {
		t.Fatalf("the view-invisible fields were lost: %+v", got)
	}
	if !reflect.DeepEqual(base, richVision()) {
		t.Fatal("merge mutated its base")
	}
}

func TestMergeViewEdit_SingleFieldEditIsTheOnlyDelta(t *testing.T) {
	base := richVision()
	view := wizardView(base)
	view.Capabilities[1].Description = "Audit trail with receipts"
	got, err := MergeViewEdit(base, view)
	if err != nil {
		t.Fatal(err)
	}
	if delta := DiffVisions(base, got); !reflect.DeepEqual(delta, []string{"capability cap_audit changed"}) {
		t.Fatalf("delta = %v", delta)
	}
	if got.Capabilities[1].Serves[0] != "persona_Engineer" {
		t.Fatal("the edited capability lost its serves link")
	}
}

func TestMergeViewEdit_ReorderKeepsIdentity(t *testing.T) {
	base := richVision()
	view := wizardView(base)
	view.Capabilities[0], view.Capabilities[1] = view.Capabilities[1], view.Capabilities[0]
	view.Requirements[0], view.Requirements[1] = view.Requirements[1], view.Requirements[0]
	got, err := MergeViewEdit(base, view)
	if err != nil {
		t.Fatal(err)
	}
	if !VisionsEquivalent(got, base) {
		t.Fatalf("reordering the view retargeted entities: %v", DiffVisions(base, got))
	}
}

func TestMergeViewEdit_NewEntitiesGetFreeIDsAndNeverOverwrite(t *testing.T) {
	base := richVision()
	base.Capabilities = append(base.Capabilities, Capability{ID: "cap_1", Description: "taken"})
	view := wizardView(base)
	view.Capabilities = append(view.Capabilities, WizardCapability{Description: "new one", Timeline: "now", Priority: "low"})
	view.Requirements = append(view.Requirements, WizardRequirement{Description: "new req", Type: "functional", Priority: "must-have", Supports: []string{"cap_policy"}})
	got, err := MergeViewEdit(base, view)
	if err != nil {
		t.Fatal(err)
	}
	last := got.Capabilities[len(got.Capabilities)-1]
	if last.ID != "cap_2" || last.Description != "new one" {
		t.Fatalf("new capability = %+v", last)
	}
	if got.Capabilities[2].Description != "taken" {
		t.Fatal("a minted ID overwrote an existing capability")
	}
	req := got.Requirements[len(got.Requirements)-1]
	if req.ID != "req_1" || req.Supports[0] != "cap_policy" {
		t.Fatalf("new requirement = %+v", req)
	}
	if err := ValidateVision(got); err != nil {
		t.Fatal(err)
	}
}

func TestMergeViewEdit_NilViewFieldsAreNotEdits(t *testing.T) {
	base := richVision()
	view := &WizardDocument{Mission: "Changed mission"}
	got, err := MergeViewEdit(base, view)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mission != "Changed mission" || got.Problem != base.Problem || len(got.Personas) != 1 ||
		len(got.Capabilities) != 2 || len(got.Risks) != 2 || len(got.Requirements) != 2 || len(got.Constraints) != 1 {
		t.Fatalf("an absent view field deleted data: %+v", got)
	}
}

func TestMergeViewEdit_RejectsDuplicateViewIDs(t *testing.T) {
	base := richVision()
	view := wizardView(base)
	view.Capabilities = append(view.Capabilities, view.Capabilities[0])
	if _, err := MergeViewEdit(base, view); err == nil {
		t.Fatal("two view entries for one capability were merged")
	}
}

func TestMergeViewEdit_SubjectChangeOfEvidencedRiskNeedsReassessment(t *testing.T) {
	base := richVision()
	view := wizardView(base)
	view.Risks[0].Description = "A different risk entirely"
	if _, err := MergeViewEdit(base, view); !errors.Is(err, ErrReassessmentRequired) {
		t.Fatalf("verified label carried onto a different risk: %v", err)
	}
	view.Risks[0].MitigationStatus = MitigationImplemented
	got, err := MergeViewEdit(base, view)
	if err != nil {
		t.Fatal(err)
	}
	if got.Risks[0].MitigationStatus != MitigationImplemented || len(got.Risks[0].MitigationEvidence) != 1 {
		t.Fatalf("reassessed risk = %+v (evidence must stay inspectable)", got.Risks[0])
	}
	// An unevidenced risk edits freely.
	view = wizardView(base)
	view.Risks[1].Description = "Cost, restated"
	if _, err := MergeViewEdit(base, view); err != nil {
		// risk_cost is waived with no evidence records.
		t.Fatalf("unevidenced risk refused an edit: %v", err)
	}
}

func TestSaveVisionIfBase_RefusesStaleBaseWithoutWriting(t *testing.T) {
	store := newTestStore(t)
	first := richVision()
	if err := store.SaveVisionIfBase(first, NoVisionDigest); err != nil {
		t.Fatalf("first save over an empty store: %v", err)
	}
	baseDigest := VisionDigest(first)

	// Another writer lands.
	other := richVision()
	other.Mission = "CLI import"
	if err := store.SaveVision(other); err != nil {
		t.Fatal(err)
	}

	mine := richVision()
	mine.Mission = "My edit"
	err := store.SaveVisionIfBase(mine, baseDigest)
	var stale *BaseStaleError
	if !errors.As(err, &stale) || !errors.Is(err, ErrVisionBaseStale) {
		t.Fatalf("stale base accepted: %v", err)
	}
	if got, _ := store.LoadVision(); got.Mission != "CLI import" {
		t.Fatalf("stale save wrote: %q", got.Mission)
	}

	// An editor that started from an empty authority is stale once one exists.
	if err := store.SaveVisionIfBase(mine, NoVisionDigest); !errors.Is(err, ErrVisionBaseStale) {
		t.Fatalf("a create over an existing vision was accepted: %v", err)
	}
}

func TestUpdateVisionFromBase_RejectedWritesNothing(t *testing.T) {
	g, dir := newEditGuardian(t)
	bad := richVision()
	bad.Requirements[0].Supports = []string{"cap_ghost"}
	_, err := g.UpdateVisionFromBase(bad, nil)
	if !errors.Is(err, ErrVisionImportRejected) {
		t.Fatalf("invalid candidate: %v", err)
	}
	if v, _ := g.store.LoadVision(); v != nil {
		t.Fatal("rejected edit reached the store")
	}
	for _, name := range []string{VisionJSONFileName, VisionMangleFileName} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Fatalf("rejected edit wrote %s", name)
		}
	}
	if g.HasVision() {
		t.Fatal("rejected edit reached the guardian cache")
	}
}

func TestUpdateVisionFromBase_UnchangedRewritesNothing(t *testing.T) {
	g, dir := newEditGuardian(t)
	base := seedRich(t, g)
	before := stat(t, dir, VisionJSONFileName)
	time.Sleep(20 * time.Millisecond)

	candidate, err := MergeViewEdit(base, wizardView(base))
	if err != nil {
		t.Fatal(err)
	}
	update, err := g.UpdateVisionFromBase(candidate, base)
	if err != nil {
		t.Fatal(err)
	}
	if update.Outcome != VisionUnchanged || len(update.Changes) != 0 {
		t.Fatalf("outcome = %s %v", update.Outcome, update.Changes)
	}
	after, _ := g.store.LoadVision()
	if !after.UpdatedAt.Equal(base.UpdatedAt) {
		t.Fatal("a no-op edit rewrote the authority (updated_at moved)")
	}
	if !stat(t, dir, VisionJSONFileName).Equal(before) {
		t.Fatal("a no-op edit rewrote the JSON export")
	}
}

func stat(t *testing.T, dir, name string) time.Time {
	t.Helper()
	info, err := os.Stat(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return info.ModTime()
}

func TestUpdateVisionFromBase_SingleEditPreservesEverythingElseThroughStore(t *testing.T) {
	g, dir := newEditGuardian(t)
	base := seedRich(t, g)
	view := wizardView(base)
	view.Requirements[1].Description = "Keep a signed trail"
	candidate, err := MergeViewEdit(base, view)
	if err != nil {
		t.Fatal(err)
	}
	update, err := g.UpdateVisionFromBase(candidate, base)
	if err != nil {
		t.Fatal(err)
	}
	if update.Outcome != VisionCommitted || !reflect.DeepEqual(update.Changes, []string{"requirement req-002 changed"}) {
		t.Fatalf("update = %+v", update)
	}
	// Reload from the store and from the JSON export: both hold the full vision.
	stored, _ := g.store.LoadVision()
	exported, err := LoadVisionJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	for name, v := range map[string]*Vision{"store": stored, "export": exported} {
		if v.Requirements[1].Description != "Keep a signed trail" || v.Requirements[0].Supports[0] != "cap_policy" ||
			len(v.Risks[0].MitigationEvidence) != 1 || v.Risks[1].Waiver == nil || v.Requirements[1].Source != "auto-generated" {
			t.Fatalf("%s lost data: %+v", name, v)
		}
	}
	if VisionDigest(update.Vision) != update.Digest || VisionDigest(stored) != update.Digest {
		t.Fatal("receipt digest is not the stored content")
	}
}

func TestUpdateVisionFromBase_StaleBaseIsConflictWithSemanticComparison(t *testing.T) {
	g, dir := newEditGuardian(t)
	base := seedRich(t, g)

	// A concurrent canonical writer (the CLI import path uses Store.SaveVision).
	cli := cloneVision(base)
	cli.Capabilities[0].Description = "Policy in Mangle, per the CLI"
	if err := g.store.SaveVision(cli); err != nil {
		t.Fatal(err)
	}
	jsonBefore := stat(t, dir, VisionJSONFileName)

	view := wizardView(base)
	view.Mission = "Editor's mission"
	candidate, err := MergeViewEdit(base, view)
	if err != nil {
		t.Fatal(err)
	}
	_, err = g.UpdateVisionFromBase(candidate, base)
	var stale *BaseStaleError
	if !errors.As(err, &stale) {
		t.Fatalf("stale edit accepted: %v", err)
	}
	if !reflect.DeepEqual(stale.Changes, []string{"capability cap_policy changed"}) || !strings.Contains(err.Error(), "cap_policy") {
		t.Fatalf("comparison = %v", stale.Changes)
	}
	now, _ := g.store.LoadVision()
	if now.Mission != base.Mission || now.Capabilities[0].Description != "Policy in Mangle, per the CLI" {
		t.Fatalf("conflicted edit changed the authority: %+v", now)
	}
	if !stat(t, dir, VisionJSONFileName).Equal(jsonBefore) {
		t.Fatal("conflicted edit rewrote the export")
	}
}

func TestUpdateVisionFromBase_CandidateIsDetachedAfterAdmission(t *testing.T) {
	g, _ := newEditGuardian(t)
	base := seedRich(t, g)
	view := wizardView(base)
	view.Capabilities[0].Description = "edited"
	candidate, _ := MergeViewEdit(base, view)
	if _, err := g.UpdateVisionFromBase(candidate, base); err != nil {
		t.Fatal(err)
	}
	candidate.Capabilities[0].Serves[0] = "mutated after admission"
	candidate.Requirements[0].Supports[0] = "mutated after admission"
	live := g.GetVision()
	if live.Capabilities[0].Serves[0] != "Engineer" || live.Requirements[0].Supports[0] != "cap_policy" {
		t.Fatal("a retained candidate reference mutated the installed vision")
	}
}

type failingKernel struct{ err error }

func (k failingKernel) Assert(types.Fact) error { return k.err }
func (k failingKernel) Retract(string) error    { return nil }

func TestUpdateVisionFromBase_ProjectionFailureIsPendingNotRejected(t *testing.T) {
	g, _ := newEditGuardian(t)
	g.SetParentKernel(failingKernel{err: errors.New("kernel down")})
	update, err := g.UpdateVisionFromBase(richVision(), nil)
	if err != nil {
		t.Fatalf("a committed edit was reported as an error: %v", err)
	}
	if update.Outcome != VisionProjectionPending || len(update.ProjectionErrors) == 0 {
		t.Fatalf("outcome = %s %v", update.Outcome, update.ProjectionErrors)
	}
	if v, _ := g.store.LoadVision(); v == nil {
		t.Fatal("authority was not committed")
	}
}

func TestSnapshotVision_ReconcilesImportSurfaceFirst(t *testing.T) {
	g, dir := newEditGuardian(t)
	if v, digest, err := g.SnapshotVision(); err != nil || v != nil || digest != NoVisionDigest {
		t.Fatalf("empty authority: %v %q %v", v, digest, err)
	}
	if _, err := WriteVisionJSON(dir, richVision()); err != nil {
		t.Fatal(err)
	}
	v, digest, err := g.SnapshotVision()
	if err != nil || v == nil || digest != VisionDigest(v) {
		t.Fatalf("hand-written JSON was not imported before the snapshot: %v %v", v, err)
	}
	v.Capabilities[0].Serves[0] = "mutated"
	if again, _, _ := g.SnapshotVision(); again.Capabilities[0].Serves[0] != "Engineer" {
		t.Fatal("snapshots share storage")
	}
}

// Requirement provenance used to be dropped by the store round trip.
func TestRequirementSource_SurvivesStoreAndExport(t *testing.T) {
	g, dir := newEditGuardian(t)
	seedRich(t, g)
	stored, _ := g.store.LoadVision()
	exported, err := LoadVisionJSON(dir)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Requirements[0].Source != "user" || exported.Requirements[1].Source != "auto-generated" {
		t.Fatalf("source lost: %q %q", stored.Requirements[0].Source, exported.Requirements[1].Source)
	}
}

func TestParseRequirements_RefusesDanglingLinks(t *testing.T) {
	state := WizardRequirementsInput{
		Capabilities: []WizardCapability{{ID: "cap_policy"}},
		Risks:        []WizardRisk{{ID: "risk_drift"}},
	}
	ok := `{"requirements":[{"type":"functional","description":"d","priority":"must-have","source":"s","supports":["cap_policy"],"addresses":["risk_drift"]}]}`
	got, err := parseRequirements(ok, state)
	if err != nil || len(got) != 1 || got[0].Supports[0] != "cap_policy" || got[0].Addresses[0] != "risk_drift" {
		t.Fatalf("valid links: %+v %v", got, err)
	}
	for _, bad := range []string{
		`{"requirements":[{"type":"functional","description":"d","priority":"must-have","source":"s","supports":["cap_1"]}]}`,
		`{"requirements":[{"type":"functional","description":"d","priority":"must-have","source":"s","addresses":["risk_ghost"]}]}`,
	} {
		if _, err := parseRequirements(bad, state); err == nil || !strings.Contains(err.Error(), "unknown") {
			t.Fatalf("dangling link accepted: %v", err)
		}
	}
}
