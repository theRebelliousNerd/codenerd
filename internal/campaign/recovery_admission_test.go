package campaign

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/tactile"
	internaltypes "codenerd/internal/types"
)

const admissionCampaignID = "/campaign_admission"

// newAdmissionOrchestrator is a production-configured orchestrator over
// workspace with its own real kernel (campaign_rules.mg loaded) and no
// campaign: what a resume door builds before admission.
func newAdmissionOrchestrator(t *testing.T, workspace string) (*Orchestrator, core.Kernel) {
	t.Helper()
	kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace:         workspace,
		Kernel:            kernel,
		LLMClient:         &MockLLMClient{},
		PlanningLLMClient: &MockLLMClient{},
		Executor:          tactile.NewDirectExecutor(),
		VirtualStore:      &core.VirtualStore{},
		TaskExecutor:      &MockTaskExecutor{},
		EventChan:         make(chan OrchestratorEvent, 64),
		Campaign:          testCampaignConfig(nil),
	})
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	t.Cleanup(func() { _ = orch.Close() })
	return orch, kernel
}

func admissionCampaign(status CampaignStatus) *Campaign {
	now := time.Now()
	return &Campaign{
		ID: admissionCampaignID, Type: CampaignTypeCustom, Title: "admission", Goal: "g",
		Status: status, CreatedAt: now, UpdatedAt: now, TotalPhases: 1, TotalTasks: 1,
		Phases: []Phase{{
			ID: "/phase_admission", CampaignID: admissionCampaignID, Name: "p", Status: PhasePending,
			Tasks: []Task{{ID: "/task_admission", PhaseID: "/phase_admission", Description: "work", Type: TaskTypeResearch, Status: TaskPending}},
		}},
	}
}

// seedCommittedCampaign publishes one committed generation through the real
// writer and releases ownership, as a process that exited would.
func seedCommittedCampaign(t *testing.T, workspace string, status CampaignStatus) {
	t.Helper()
	orch, _ := newAdmissionOrchestrator(t, workspace)
	if err := orch.SetCampaign(admissionCampaign(status)); err != nil {
		t.Fatalf("SetCampaign: %v", err)
	}
	if err := orch.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func admissionPaths(workspace string) (dir, snapshot, journal string) {
	dir = filepath.Join(workspace, ".nerd", "campaigns")
	return dir, filepath.Join(dir, admissionCampaignID+".json"), filepath.Join(dir, admissionCampaignID+".journal.jsonl")
}

// requestUncommittedWrite journals a write request for c, as saveCampaign
// does, and returns the bytes it would publish. The commit is never written.
func requestUncommittedWrite(t *testing.T, workspace string, c *Campaign) []byte {
	t.Helper()
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	o := &Orchestrator{nerdDir: filepath.Join(workspace, ".nerd"), campaign: c}
	if _, _, err := o.recoverJournalLocked(c.ID); err != nil {
		t.Fatal(err)
	}
	if err := o.appendJournalEventLocked(JournalEventSnapshotRequested, map[string]any{"status": c.Status}, checksumBytes(data)); err != nil {
		t.Fatal(err)
	}
	return data
}

func kernelHoldsCampaign(t *testing.T, kernel core.Kernel) bool {
	t.Helper()
	facts, err := kernel.Query("campaign")
	if err != nil {
		return false
	}
	for _, f := range facts {
		if len(f.Args) > 0 && internaltypes.ExtractString(f.Args[0]) == admissionCampaignID {
			return true
		}
	}
	return false
}

func TestResumeCampaign_CommittedMatchIsAdmitted(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	seedCommittedCampaign(t, ws, StatusPaused)

	orch, kernel := newAdmissionOrchestrator(t, ws)
	report, err := orch.ResumeCampaign(admissionCampaignID, AdmissionPolicy{})
	if err != nil {
		t.Fatalf("ResumeCampaign: %v", err)
	}
	if report.Class != RecoveryCommittedMatch || report.Provenance != ProvenanceCommitted || !report.Admitted {
		t.Fatalf("report = %s/%s admitted=%v, want committed", report.Class, report.Provenance, report.Admitted)
	}
	if !kernelHoldsCampaign(t, kernel) {
		t.Fatal("admitted campaign was not hydrated into the kernel")
	}
	// Admission does not publish status: Run does, when work starts.
	if orch.campaign.Status != StatusPaused {
		t.Fatalf("status after admission = %s, want unchanged paused", orch.campaign.Status)
	}
	v, err := VerifyCampaignJournal(ws, admissionCampaignID)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Healthy || v.Recovery != RecoveryCommittedMatch || v.UncommittedWrites != 0 {
		t.Fatalf("verify after admission: healthy=%v recovery=%s uncommitted=%d problems=%v", v.Healthy, v.Recovery, v.UncommittedWrites, v.Problems)
	}
}

func TestResumeCampaign_InterruptedPublicationIsReconciled(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	seedCommittedCampaign(t, ws, StatusActive)
	_, snapshot, _ := admissionPaths(ws)

	// The process died after the rename, before the commit record.
	next := admissionCampaign(StatusActive)
	next.CompletedTasks = 1
	data := requestUncommittedWrite(t, ws, next)
	if err := os.WriteFile(snapshot, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if got := InspectCampaignRecovery(ws, admissionCampaignID).Class; got != RecoveryPreparedSnapshot {
		t.Fatalf("class = %s, want prepared", got)
	}
	replay, err := ReplayCampaignJournal(ws, admissionCampaignID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if out := RenderJournalReplay(replay); !strings.Contains(out, "Its bytes landed") || strings.Contains(out, "fall back to the previous snapshot") {
		t.Fatalf("replay misstates the interrupted publication:\n%s", out)
	}

	orch, _ := newAdmissionOrchestrator(t, ws)
	report, err := orch.ResumeCampaign(admissionCampaignID, AdmissionPolicy{})
	if err != nil {
		t.Fatalf("ResumeCampaign: %v", err)
	}
	if report.Provenance != ProvenanceReconciledPublication {
		t.Fatalf("provenance = %s, want reconciled publication", report.Provenance)
	}
	if orch.campaign.CompletedTasks != 1 {
		t.Fatalf("admitted generation is not the newer one: completed=%d", orch.campaign.CompletedTasks)
	}
	if got := InspectCampaignRecovery(ws, admissionCampaignID).Class; got != RecoveryCommittedMatch {
		t.Fatalf("class after admission = %s, want committed", got)
	}
}

func TestResumeCampaign_UnpublishedWriteKeepsPreviousGeneration(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	seedCommittedCampaign(t, ws, StatusActive)
	next := admissionCampaign(StatusActive)
	next.CompletedTasks = 1
	requestUncommittedWrite(t, ws, next) // never published

	orch, _ := newAdmissionOrchestrator(t, ws)
	report, err := orch.ResumeCampaign(admissionCampaignID, AdmissionPolicy{})
	if err != nil {
		t.Fatalf("ResumeCampaign: %v", err)
	}
	if report.Class != RecoveryPreviousGeneration || report.Provenance != ProvenancePreviousGeneration {
		t.Fatalf("report = %s/%s, want previous generation", report.Class, report.Provenance)
	}
	if orch.campaign.CompletedTasks != 0 {
		t.Fatalf("the unpublished generation was admitted: completed=%d", orch.campaign.CompletedTasks)
	}
	v, err := VerifyCampaignJournal(ws, admissionCampaignID)
	if err != nil {
		t.Fatal(err)
	}
	if v.UncommittedWrites != 0 {
		t.Fatalf("admission left the dangling request unaccounted: uncommitted=%d", v.UncommittedWrites)
	}
}

// The old CLI resume rewrote the snapshot outside the journal before Cortex
// boot; a failed boot left exactly this. It must not hydrate or be overwritten.
func TestResumeCampaign_UnjournaledRewriteIsRefusedUntilAdopted(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	seedCommittedCampaign(t, ws, StatusPaused)
	_, snapshot, _ := admissionPaths(ws)
	rewritten := admissionCampaign(StatusActive)
	data, _ := json.MarshalIndent(rewritten, "", "  ")
	if err := os.WriteFile(snapshot, data, 0o644); err != nil {
		t.Fatal(err)
	}

	orch, kernel := newAdmissionOrchestrator(t, ws)
	report, err := orch.ResumeCampaign(admissionCampaignID, AdmissionPolicy{})
	var refused *RecoveryRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("ResumeCampaign error = %v, want RecoveryRefusedError", err)
	}
	if report.Class != RecoveryConflict || report.Admitted {
		t.Fatalf("report = %s admitted=%v, want refused conflict", report.Class, report.Admitted)
	}
	if kernelHoldsCampaign(t, kernel) || orch.campaign != nil {
		t.Fatal("a refused generation reached the kernel or the orchestrator")
	}
	after, _ := os.ReadFile(snapshot)
	if string(after) != string(data) {
		t.Fatal("a refused admission rewrote the snapshot")
	}
	if !strings.Contains(RenderRecoveryReport(report), "--adopt-snapshot") {
		t.Fatal("refusal report does not name the operator's way out")
	}

	report, err = orch.ResumeCampaign(admissionCampaignID, AdmissionPolicy{AdoptSnapshot: true})
	if err != nil {
		t.Fatalf("adopting resume: %v", err)
	}
	if report.Provenance != ProvenanceOperatorAdopted || !kernelHoldsCampaign(t, kernel) {
		t.Fatalf("adoption = %s hydrated=%v", report.Provenance, kernelHoldsCampaign(t, kernel))
	}
	if got := InspectCampaignRecovery(ws, admissionCampaignID).Class; got != RecoveryCommittedMatch {
		t.Fatalf("adopted generation was not republished as committed: %s", got)
	}
}

func TestResumeCampaign_UnreadableStorageIsNeverAdmitted(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	seedCommittedCampaign(t, ws, StatusPaused)
	_, snapshot, _ := admissionPaths(ws)
	if err := os.Remove(snapshot); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(snapshot, 0o755); err != nil { // reads fail, not "missing"
		t.Fatal(err)
	}
	orch, _ := newAdmissionOrchestrator(t, ws)
	report, err := orch.ResumeCampaign(admissionCampaignID, AdmissionPolicy{AdoptSnapshot: true})
	var refused *RecoveryRefusedError
	if !errors.As(err, &refused) || report.Class != RecoveryStorageUnreadable {
		t.Fatalf("err=%v class=%s, want refused storage_unreadable even with adoption", err, report.Class)
	}
}

func TestResumeCampaign_LegacySnapshotIsImportedWithProvenance(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	dir, snapshot, journal := admissionPaths(ws)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.MarshalIndent(admissionCampaign(StatusPaused), "", "  ")
	if err := os.WriteFile(snapshot, data, 0o644); err != nil {
		t.Fatal(err)
	}
	orch, _ := newAdmissionOrchestrator(t, ws)
	report, err := orch.ResumeCampaign(admissionCampaignID, AdmissionPolicy{})
	if err != nil {
		t.Fatalf("ResumeCampaign: %v", err)
	}
	if report.Provenance != ProvenanceLegacyImport {
		t.Fatalf("provenance = %s, want legacy import", report.Provenance)
	}
	raw, err := os.ReadFile(journal)
	if err != nil || !strings.Contains(string(raw), `"provenance":"/legacy_import"`) {
		t.Fatalf("the import boundary is not on the journal (err=%v)", err)
	}
}

// A death inside renameAtomicReplace's fallback: the old snapshot moved aside,
// the verified temp file not yet renamed in. Neither discovery nor admission
// may skip the campaign; the newer, complete generation wins and is restored.
func TestResumeCampaign_ReplacementLeftoversAreFoundAndRestored(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	seedCommittedCampaign(t, ws, StatusActive)
	dir, snapshot, _ := admissionPaths(ws)
	next := admissionCampaign(StatusActive)
	next.CompletedTasks = 1
	data := requestUncommittedWrite(t, ws, next)
	base := filepath.Base(snapshot)
	if err := os.Rename(snapshot, filepath.Join(dir, base+".prev-100")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, base+".tmp-200"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	orch, _ := newAdmissionOrchestrator(t, ws)
	report, err := orch.ResumeCampaign(admissionCampaignID, AdmissionPolicy{})
	if err != nil {
		t.Fatalf("ResumeCampaign: %v", err)
	}
	if report.Provenance != ProvenanceReconciledPublication || !strings.HasSuffix(report.CandidatePath, ".tmp-200") {
		t.Fatalf("admitted %s from %s, want the prepared temp file", report.Provenance, report.CandidatePath)
	}
	if _, err := os.Stat(snapshot); err != nil {
		t.Fatalf("snapshot was not restored under its name: %v", err)
	}
	if orch.campaign.CompletedTasks != 1 {
		t.Fatal("restored generation is not the prepared one")
	}
}

func TestResumeCampaign_DamagedJournalIsPreservedBeforeRepair(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	seedCommittedCampaign(t, ws, StatusPaused)
	_, _, journal := admissionPaths(ws)
	f, err := os.OpenFile(journal, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"seq":3,"event_type":"snapshot_wri`); err != nil { // torn
		t.Fatal(err)
	}
	f.Close()
	damaged, _ := os.ReadFile(journal)

	orch, _ := newAdmissionOrchestrator(t, ws)
	report, err := orch.ResumeCampaign(admissionCampaignID, AdmissionPolicy{})
	if err != nil {
		t.Fatalf("ResumeCampaign: %v", err)
	}
	if !report.JournalDamaged || report.PreservedJournal == "" {
		t.Fatalf("damage not reported: damaged=%v preserved=%q", report.JournalDamaged, report.PreservedJournal)
	}
	kept, err := os.ReadFile(report.PreservedJournal)
	if err != nil || string(kept) != string(damaged) {
		t.Fatalf("damaged journal not preserved byte for byte (err=%v)", err)
	}
	v, err := VerifyCampaignJournal(ws, admissionCampaignID)
	if err != nil || !v.Healthy {
		t.Fatalf("journal unhealthy after repair: err=%v problems=%v", err, v.Problems)
	}
}

func TestCampaignOwnership_SecondPublisherRefused(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	seedCommittedCampaign(t, ws, StatusPaused)

	first, _ := newAdmissionOrchestrator(t, ws)
	if _, err := first.ResumeCampaign(admissionCampaignID, AdmissionPolicy{}); err != nil {
		t.Fatalf("first owner: %v", err)
	}
	second, kernel := newAdmissionOrchestrator(t, ws)
	if _, err := second.ResumeCampaign(admissionCampaignID, AdmissionPolicy{}); !errors.Is(err, ErrCampaignOwned) {
		t.Fatalf("second owner err = %v, want ErrCampaignOwned", err)
	}
	if kernelHoldsCampaign(t, kernel) {
		t.Fatal("a refused owner hydrated the campaign")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := second.ResumeCampaign(admissionCampaignID, AdmissionPolicy{}); err != nil {
		t.Fatalf("after the first owner closed: %v", err)
	}
}

func TestSetCampaign_RefusesACampaignWithDurableHistory(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	seedCommittedCampaign(t, ws, StatusPaused)
	orch, _ := newAdmissionOrchestrator(t, ws)
	if err := orch.SetCampaign(admissionCampaign(StatusActive)); err == nil || !strings.Contains(err.Error(), "durable journal") {
		t.Fatalf("SetCampaign over existing history err = %v", err)
	}
}

func TestJournalAppend_OpenFailureConsumesNoSequence(t *testing.T) {
	o := &Orchestrator{nerdDir: t.TempDir(), campaign: &Campaign{ID: admissionCampaignID}}
	if err := o.appendJournalEventLocked("a", nil, ""); err != nil {
		t.Fatal(err)
	}
	orig := osOpenJournal
	osOpenJournal = func(string) (*os.File, error) { return nil, errors.New("EMFILE") }
	err := o.appendJournalEventLocked("b", nil, "")
	osOpenJournal = orig
	if err == nil {
		t.Fatal("open failure was not reported")
	}
	if o.journalUncertain {
		t.Fatal("a failure before any byte was written marked the journal uncertain")
	}
	if err := o.appendJournalEventLocked("c", nil, ""); err != nil {
		t.Fatal(err)
	}
	scan, err := scanJournal(o.journalPath(admissionCampaignID), admissionCampaignID)
	if err != nil || scan.damaged || scan.lastSeq() != 2 {
		t.Fatalf("journal after open failure: err=%v damaged=%v last=%d problems=%v", err, scan.damaged, scan.lastSeq(), scan.problems)
	}
}

func TestJournalAppend_TornWriteIsReconciledBeforeTheNextAppend(t *testing.T) {
	o := &Orchestrator{nerdDir: t.TempDir(), campaign: &Campaign{ID: admissionCampaignID}}
	if err := o.appendJournalEventLocked("a", nil, ""); err != nil {
		t.Fatal(err)
	}
	orig := osWriteJournal
	osWriteJournal = func(f *os.File, b []byte) (int, error) {
		n, _ := f.Write(b[:len(b)/2]) // half the record lands
		return n, errors.New("EIO")
	}
	err := o.appendJournalEventLocked("b", nil, "")
	osWriteJournal = orig
	if err == nil || !o.journalUncertain {
		t.Fatalf("torn write: err=%v uncertain=%v", err, o.journalUncertain)
	}
	if err := o.appendJournalEventLocked("c", nil, ""); err != nil {
		t.Fatalf("append after reconciliation: %v", err)
	}
	path := o.journalPath(admissionCampaignID)
	scan, err := scanJournal(path, admissionCampaignID)
	if err != nil || scan.damaged || scan.lastSeq() != 2 || scan.events[1].EventType != "c" {
		t.Fatalf("journal after reconciliation: err=%v damaged=%v last=%d", err, scan.damaged, scan.lastSeq())
	}
	preserved, _ := filepath.Glob(path + ".damaged-*")
	if len(preserved) != 1 {
		t.Fatalf("torn record not preserved: %v", preserved)
	}
}
