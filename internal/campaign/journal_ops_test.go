package campaign

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/tactile"
)

func journalOpsOrchestrator(t *testing.T, workspace string) *Orchestrator {
	t.Helper()
	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace:    workspace,
		Kernel:       &MockKernel{},
		LLMClient:    &MockLLMClient{},
		TaskExecutor: &MockTaskExecutor{},
		Executor:     tactile.NewDirectExecutor(),
		VirtualStore: &core.VirtualStore{},
	})
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	return orch
}

func journalOpsCampaign(id string) *Campaign {
	return &Campaign{
		ID:          id,
		Title:       "journal ops",
		Status:      StatusActive,
		TotalPhases: 1,
		TotalTasks:  2,
		Phases: []Phase{{
			ID:         id + "_phase",
			CampaignID: id,
			Name:       "work",
			Status:     PhaseInProgress,
			Tasks: []Task{
				{ID: id + "_t0", PhaseID: id + "_phase", Status: TaskCompleted, Type: TaskTypeFileCreate},
				{ID: id + "_t1", PhaseID: id + "_phase", Status: TaskPending, Type: TaskTypeFileCreate},
			},
		}},
	}
}

func TestVerifyCampaignJournal_WhenIntact_ShouldReportHealthy(t *testing.T) {
	ws := t.TempDir()
	orch := journalOpsOrchestrator(t, ws)
	c := journalOpsCampaign("campaign_journal_ok")

	if err := orch.SetCampaign(c); err != nil {
		t.Fatalf("SetCampaign: %v", err)
	}
	orch.mu.Lock()
	c.CompletedTasks = 1
	err := orch.saveCampaign()
	orch.mu.Unlock()
	if err != nil {
		t.Fatalf("saveCampaign: %v", err)
	}

	v, err := VerifyCampaignJournal(ws, c.ID)
	if err != nil {
		t.Fatalf("VerifyCampaignJournal: %v", err)
	}
	if !v.Healthy {
		t.Fatalf("healthy journal reported defects: %s", RenderJournalVerification(v))
	}
	if !v.SnapshotMatches {
		t.Errorf("snapshot checksum did not match the last committed event:\n%s", RenderJournalVerification(v))
	}
	if v.UncommittedWrites != 0 {
		t.Errorf("UncommittedWrites = %d on a clean save, want 0", v.UncommittedWrites)
	}
	if v.ValidEvents < 4 {
		t.Errorf("expected at least 4 events (two request/commit pairs), got %d", v.ValidEvents)
	}
}

// Chaos: the process dies between the verified temp write and the rename.
//
// This is the one moment where the snapshot protocol can leave inconsistent
// state on disk, and the whole event-before-ack design exists for it. The
// guarantees under test: the previous snapshot is untouched, the journal shows
// a write request with no commit, and verification names that condition rather
// than reporting healthy.
func TestSaveCampaign_WhenKilledDuringSnapshotRename_ShouldLeavePreviousSnapshotIntact(t *testing.T) {
	ws := t.TempDir()
	orch := journalOpsOrchestrator(t, ws)
	c := journalOpsCampaign("campaign_journal_crash")

	if err := orch.SetCampaign(c); err != nil {
		t.Fatalf("SetCampaign: %v", err)
	}

	snapshotPath := campaignSnapshotPath(ws, c.ID)
	before, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("reading committed snapshot: %v", err)
	}

	// Simulate the kill: every rename attempt fails, exactly as if the process
	// vanished before the kernel completed it.
	original := osRenameFile
	osRenameFile = func(string, string) error { return errors.New("simulated kill during rename") }
	t.Cleanup(func() { osRenameFile = original })

	orch.mu.Lock()
	c.Status = StatusCompleted
	c.CompletedTasks = 2
	saveErr := orch.saveCampaign()
	orch.mu.Unlock()
	if saveErr == nil {
		t.Fatal("saveCampaign reported success while the rename never happened")
	}

	osRenameFile = original

	after, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("previous snapshot disappeared after a failed rename: %v", err)
	}
	if string(after) != string(before) {
		t.Fatal("the committed snapshot was modified by a save that never completed")
	}
	var onDisk Campaign
	if uerr := json.Unmarshal(after, &onDisk); uerr != nil {
		t.Fatalf("surviving snapshot is not valid JSON: %v", uerr)
	}
	if onDisk.Status == StatusCompleted {
		t.Fatal("the uncommitted status leaked into the on-disk snapshot")
	}

	// No temp file may survive: a stale <id>.json.tmp-* would be mistaken for
	// a snapshot by anything globbing the directory.
	leftovers, _ := filepath.Glob(filepath.Join(ws, ".nerd", "campaigns", "*.tmp-*"))
	if len(leftovers) != 0 {
		t.Errorf("temp snapshots left behind after a failed rename: %v", leftovers)
	}

	v, err := VerifyCampaignJournal(ws, c.ID)
	if err != nil {
		t.Fatalf("VerifyCampaignJournal: %v", err)
	}
	if v.UncommittedWrites != 1 {
		t.Fatalf("expected exactly one uncommitted write after the simulated kill, got %d:\n%s",
			v.UncommittedWrites, RenderJournalVerification(v))
	}
	if !v.SnapshotMatches {
		t.Errorf("the surviving snapshot no longer matches its last commit record:\n%s",
			RenderJournalVerification(v))
	}

	replay, err := ReplayCampaignJournal(ws, c.ID, 0)
	if err != nil {
		t.Fatalf("ReplayCampaignJournal: %v", err)
	}
	if replay.FinalState == nil {
		t.Fatal("replay produced no final state")
	}
	if replay.FinalState.Committed {
		t.Error("replay claims the final write committed; it did not, and resume would trust the wrong snapshot")
	}
}

func TestVerifyCampaignJournal_WhenTailCorrupt_ShouldReportChecksumMismatch(t *testing.T) {
	ws := t.TempDir()
	orch := journalOpsOrchestrator(t, ws)
	c := journalOpsCampaign("campaign_journal_corrupt")
	if err := orch.SetCampaign(c); err != nil {
		t.Fatalf("SetCampaign: %v", err)
	}

	path := campaignJournalPath(ws, c.ID)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	// Flip a payload byte on the last event so its checksum no longer matches.
	corrupted := strings.Replace(string(data), `"status":"`, `"status":"x`, 1)
	if corrupted == string(data) {
		t.Fatal("could not corrupt the journal; the payload shape changed")
	}
	if err := os.WriteFile(path, []byte(corrupted), 0o644); err != nil {
		t.Fatalf("write corrupted journal: %v", err)
	}

	v, err := VerifyCampaignJournal(ws, c.ID)
	if err != nil {
		t.Fatalf("VerifyCampaignJournal: %v", err)
	}
	if v.Healthy {
		t.Fatal("a tampered journal was reported healthy")
	}
	found := false
	for _, p := range v.Problems {
		if p.Kind == "checksum_mismatch" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a checksum_mismatch problem, got %+v", v.Problems)
	}
}

func TestListCampaignJournals_ShouldFindWrittenCampaigns(t *testing.T) {
	ws := t.TempDir()
	orch := journalOpsOrchestrator(t, ws)
	for _, id := range []string{"campaign_list_a", "campaign_list_b"} {
		if err := orch.SetCampaign(journalOpsCampaign(id)); err != nil {
			t.Fatalf("SetCampaign(%s): %v", id, err)
		}
	}

	ids, err := ListCampaignJournals(ws)
	if err != nil {
		t.Fatalf("ListCampaignJournals: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 journals, got %v", ids)
	}
}

func TestVerifyCampaignJournal_WhenSnapshotEditedOutsideOrchestrator_ShouldReportMismatch(t *testing.T) {
	ws := t.TempDir()
	orch := journalOpsOrchestrator(t, ws)
	c := journalOpsCampaign("campaign_journal_edited")
	if err := orch.SetCampaign(c); err != nil {
		t.Fatalf("SetCampaign: %v", err)
	}

	snapshotPath := campaignSnapshotPath(ws, c.ID)
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if err := os.WriteFile(snapshotPath, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}

	v, err := VerifyCampaignJournal(ws, c.ID)
	if err != nil {
		t.Fatalf("VerifyCampaignJournal: %v", err)
	}
	if v.SnapshotMatches {
		t.Fatal("an externally edited snapshot still matched its recorded checksum")
	}
	if v.Healthy {
		t.Fatal("an externally edited snapshot was reported healthy")
	}
}
