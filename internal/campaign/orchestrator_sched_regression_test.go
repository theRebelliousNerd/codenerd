package campaign

import (
	"errors"
	"testing"
)

// TestRollback_NonStructuralTask_PreservesSiblingCompletions is the F-SCHED-2
// regression guard. A failing non-structural mutating task (/document) must NOT
// swap o.campaign nor revert the concurrently-committed completion of a sibling
// task. Before the fix, the whole-campaign snapshot/restore clobbered sibling
// completions and orphaned runPhase's phase pointer, driving an infinite
// completion→re-dispatch loop.
func TestRollback_NonStructuralTask_PreservesSiblingCompletions(t *testing.T) {
	orch := newSnapshotTestOrchestrator()

	// Add a sibling task in the same phase that will complete concurrently.
	orch.campaign.Phases[0].Tasks = append(orch.campaign.Phases[0].Tasks, Task{
		ID:          "/task_sibling",
		PhaseID:     "/phase_0",
		Description: "sibling",
		Status:      TaskInProgress,
		Type:        TaskTypeResearch,
		Priority:    PriorityNormal,
		Order:       1,
	})
	orch.campaign.TotalTasks = 2

	campaignBefore := orch.campaign // identity guard

	failing := &Task{ID: "/task_seed", PhaseID: "/phase_0", Type: TaskTypeDocument}
	_, err := orch.withTaskExecutionSnapshot(failing, func() (any, error) {
		// Simulate a concurrent sibling committing its completion to the live
		// campaign while the failing task is mid-flight.
		for i := range orch.campaign.Phases[0].Tasks {
			if orch.campaign.Phases[0].Tasks[i].ID == "/task_sibling" {
				orch.campaign.Phases[0].Tasks[i].Status = TaskCompleted
			}
		}
		return nil, errors.New("document task failed")
	})
	if err == nil {
		t.Fatalf("expected failure from non-structural task")
	}

	// 1. The campaign pointer must NOT be swapped — otherwise runPhase's cached
	//    phase pointer is orphaned.
	if orch.campaign != campaignBefore {
		t.Fatalf("non-structural rollback swapped o.campaign pointer (orphans phase pointer)")
	}

	// 2. The sibling's concurrent completion must survive the rollback.
	if got := taskStatusByID(orch.campaign, "/task_sibling"); got != TaskCompleted {
		t.Fatalf("sibling completion clobbered by rollback: got %s, want %s", got, TaskCompleted)
	}
}

// TestRollback_StructuralTask_StillFullyRollsBack documents that assault
// (plan-restructuring) task types retain whole-campaign transactional rollback.
func TestRollback_StructuralTask_StillFullyRollsBack(t *testing.T) {
	orch := newSnapshotTestOrchestrator()
	beforeTasks := len(orch.campaign.Phases[0].Tasks)

	_, err := orch.withTaskExecutionSnapshot(&Task{ID: "/task_seed", Type: TaskTypeAssaultDiscover}, func() (any, error) {
		orch.campaign.Phases[0].Tasks = append(orch.campaign.Phases[0].Tasks, Task{
			ID:      "/task_discovered",
			PhaseID: "/phase_0",
			Status:  TaskPending,
			Type:    TaskTypeFileModify,
		})
		return nil, errors.New("assault discover failed mid-plan")
	})
	if err == nil {
		t.Fatalf("expected failure from structural task")
	}
	if got := len(orch.campaign.Phases[0].Tasks); got != beforeTasks {
		t.Fatalf("structural rollback did not revert plan edit: got %d tasks, want %d", got, beforeTasks)
	}
}

// TestIncrementCheckpointFailures_BoundsAndIsolation guards the F-CKPT-2 counter:
// it must increment monotonically per phase, keep phases independent, and be safe
// for an unknown phase ID. The runPhase exhaustion→advance behavior is verified
// end-to-end by the live audit run.
func TestIncrementCheckpointFailures_BoundsAndIsolation(t *testing.T) {
	orch := newSnapshotTestOrchestrator()
	orch.campaign.Phases = append(orch.campaign.Phases, Phase{
		ID:         "/phase_1",
		CampaignID: "/campaign_txn_test",
		Name:       "phase 1",
		Order:      1,
		Status:     PhasePending,
	})

	// Monotonic increment on phase 0 up to the cap.
	for want := 1; want <= maxPhaseCheckpointAttempts; want++ {
		if got := orch.incrementCheckpointFailures("/phase_0"); got != want {
			t.Fatalf("increment #%d for /phase_0 = %d, want %d", want, got, want)
		}
	}
	if got := orch.campaign.Phases[0].CheckpointFailures; got != maxPhaseCheckpointAttempts {
		t.Fatalf("persisted /phase_0 CheckpointFailures = %d, want %d", got, maxPhaseCheckpointAttempts)
	}

	// Phases are independent.
	if got := orch.incrementCheckpointFailures("/phase_1"); got != 1 {
		t.Fatalf("first increment for /phase_1 = %d, want 1", got)
	}
	if got := orch.campaign.Phases[0].CheckpointFailures; got != maxPhaseCheckpointAttempts {
		t.Fatalf("/phase_0 counter mutated by /phase_1 increment: %d", got)
	}

	// Unknown phase ID is safe (returns 0, no panic).
	if got := orch.incrementCheckpointFailures("/nonexistent"); got != 0 {
		t.Fatalf("increment for unknown phase = %d, want 0", got)
	}
}

func taskStatusByID(c *Campaign, id string) TaskStatus {
	for i := range c.Phases {
		for j := range c.Phases[i].Tasks {
			if c.Phases[i].Tasks[j].ID == id {
				return c.Phases[i].Tasks[j].Status
			}
		}
	}
	return ""
}
