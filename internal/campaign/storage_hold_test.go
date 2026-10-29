package campaign

import (
	"os"
	"path/filepath"
	"testing"
)

// A campaign whose snapshot write failed runs from memory only. The policy
// holds new dispatch (eligible_task) and keeps the pending work alive
// (has_next_campaign_task: a wait, not /all_tasks_blocked); the next write that
// lands releases it. The kernel decides; Go only reports the storage.
func TestStorageHold_AFailedSnapshotHoldsDispatchUntilAWriteLands(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	c := eligibilityCampaign(Task{ID: "/task_ready", Status: TaskPending})
	c.Phases[0].Status = PhaseInProgress
	k := realKernelFor(t, c, true)
	o := &Orchestrator{workspace: dir, nerdDir: dir, kernel: k, campaign: c,
		eventChan: make(chan OrchestratorEvent, 8)}

	if got := eligibleIDs(o.getEligibleTasks(&c.Phases[0])); len(got) != 1 {
		t.Fatalf("precondition: want /task_ready eligible before any failure, got %v", got)
	}

	// A regular file where the campaigns directory belongs: every save fails.
	blocker := filepath.Join(dir, "campaigns")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	o.persistCampaign("autosave")

	if got := eligibleIDs(o.getEligibleTasks(&c.Phases[0])); len(got) != 0 {
		t.Fatalf("scheduled %v while the campaign's storage is uncertain", got)
	}
	if reason := o.getCampaignBlockReason(); reason != "" {
		t.Fatalf("a storage hold must be a wait, not a block; campaign blocked on %q", reason)
	}

	// The disk recovers; the next autosave lands and releases the hold.
	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	o.persistCampaign("autosave")

	if got := eligibleIDs(o.getEligibleTasks(&c.Phases[0])); len(got) != 1 {
		t.Fatalf("a successful write must release the hold; eligible = %v", got)
	}
	if o.storageHeld {
		t.Fatal("storageHeld still set after a write landed")
	}
}
