package campaign

import "testing"

// Phase completion is idempotent: a phase completed once is not counted
// again when something delivers it a second time (a stale current_phase used
// to re-select a completed phase and complete it again; audit
// campaign-context-seed-scheduler-ownership).
func TestCompletePhase_CountsAPhaseOnce(t *testing.T) {
	t.Parallel()
	o := newSnapshotTestOrchestrator()
	phase := &o.campaign.Phases[0]
	o.completePhase(phase)
	o.completePhase(phase)
	if got := o.campaign.CompletedPhases; got != 1 {
		t.Fatalf("CompletedPhases = %d after completing one phase twice, want 1", got)
	}
}
