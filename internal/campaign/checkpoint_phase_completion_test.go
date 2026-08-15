package campaign

import (
	"context"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/session"
	"codenerd/internal/tactile"
)

// A failing checkpoint must not complete the phase.
//
// The whole point of a phase checkpoint is that "all tasks finished" and "the
// phase is done" are different claims. If a failed verification still marked
// the phase completed, the campaign would advance to the next phase on top of
// unverified work, and the completed-phase count shown to the operator would
// mean nothing. runPhase does the right thing today; this pins it, because the
// failure path and the success path converge two statements apart in
// orchestrator_tasks.go and an early `completePhase` there would be silent.
//
// The one intended exception is the bounded escape hatch: after
// maxPhaseCheckpointAttempts the phase advances with an UNVERIFIED checkpoint
// rather than spinning failure -> replan -> re-checkpoint forever. That
// exception is tested too, including the requirement that the failure is
// recorded on the phase so "completed" is never mistaken for "verified".
func newCheckpointRegressionOrchestrator(t *testing.T, review string) (*Orchestrator, chan OrchestratorEvent) {
	t.Helper()

	kernel := &MockKernel{}
	events := make(chan OrchestratorEvent, 64)

	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace:    t.TempDir(),
		Kernel:       kernel,
		LLMClient:    &MockLLMClient{},
		TaskExecutor: &MockTaskExecutor{},
		Executor:     tactile.NewDirectExecutor(),
		VirtualStore: &core.VirtualStore{},
		EventChan:    events,
	})
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}

	// The checkpoint runs a /shard_validation review through the task executor.
	// Returning a verdict string makes pass/fail deterministic with no LLM.
	orch.checkpoint = NewCheckpointRunner(nil, &MockTaskExecutor{
		ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
			return review, nil
		},
	}, orch.workspace)

	// Replanning on checkpoint failure would need a live LLM; the invariant
	// under test is about phase status, not about what replan produces.
	orch.replanner = nil

	orch.campaign = &Campaign{
		ID:          "/campaign_ckpt",
		Title:       "checkpoint regression",
		Status:      StatusActive,
		TotalPhases: 1,
		TotalTasks:  1,
		Phases: []Phase{{
			ID:         "/phase_ckpt_0",
			CampaignID: "/campaign_ckpt",
			Name:       "verified phase",
			Status:     PhaseInProgress,
			Objectives: []PhaseObjective{{
				Type:               ObjectiveCreate,
				Description:        "produce the thing",
				VerificationMethod: VerifyShardValidate,
			}},
			Tasks: []Task{{
				ID:      "/task_ckpt_0",
				PhaseID: "/phase_ckpt_0",
				Status:  TaskCompleted,
				Type:    TaskTypeFileCreate,
			}},
		}},
	}
	return orch, events
}

func drainEventTypes(ch chan OrchestratorEvent) map[string]int {
	seen := make(map[string]int)
	for {
		select {
		case ev := <-ch:
			seen[ev.Type]++
		default:
			return seen
		}
	}
}

func TestRunPhase_WhenCheckpointFails_ShouldNotCompletePhase(t *testing.T) {
	orch, events := newCheckpointRegressionOrchestrator(t, "FAIL: the artifact was never written")

	if err := orch.runPhase(context.Background(), &orch.campaign.Phases[0]); err != nil {
		t.Fatalf("runPhase returned error: %v", err)
	}

	phase := orch.campaign.Phases[0]
	if phase.Status == PhaseCompleted {
		t.Fatal("phase was marked completed despite a FAILED checkpoint: unverified work would be treated as verified")
	}
	if orch.campaign.CompletedPhases != 0 {
		t.Fatalf("CompletedPhases = %d after a failed checkpoint, want 0", orch.campaign.CompletedPhases)
	}
	if phase.CheckpointFailures != 1 {
		t.Fatalf("CheckpointFailures = %d, want 1", phase.CheckpointFailures)
	}
	if len(phase.Checkpoints) != 1 || phase.Checkpoints[0].Passed {
		t.Fatalf("expected one recorded FAILED checkpoint, got %+v", phase.Checkpoints)
	}

	seen := drainEventTypes(events)
	if seen[EventCheckpointFailed] == 0 {
		t.Fatalf("expected a %s event so the operator learns why the phase stayed open; got %v", EventCheckpointFailed, seen)
	}
	if seen[EventPhaseCompleted] != 0 {
		t.Fatalf("a phase_completed event was emitted for a failed checkpoint; got %v", seen)
	}
}

func TestRunPhase_WhenCheckpointPasses_ShouldCompletePhase(t *testing.T) {
	orch, _ := newCheckpointRegressionOrchestrator(t, "PASS: everything verified")

	if err := orch.runPhase(context.Background(), &orch.campaign.Phases[0]); err != nil {
		t.Fatalf("runPhase returned error: %v", err)
	}

	if orch.campaign.Phases[0].Status != PhaseCompleted {
		t.Fatalf("phase status = %s after a PASSING checkpoint, want %s",
			orch.campaign.Phases[0].Status, PhaseCompleted)
	}
	if orch.campaign.CompletedPhases != 1 {
		t.Fatalf("CompletedPhases = %d, want 1", orch.campaign.CompletedPhases)
	}
}

func TestRunPhase_WhenCheckpointExhausted_ShouldAdvanceWithFailureRecorded(t *testing.T) {
	orch, events := newCheckpointRegressionOrchestrator(t, "FAIL: still broken")

	for attempt := 1; attempt < maxPhaseCheckpointAttempts; attempt++ {
		if err := orch.runPhase(context.Background(), &orch.campaign.Phases[0]); err != nil {
			t.Fatalf("runPhase attempt %d returned error: %v", attempt, err)
		}
		if orch.campaign.Phases[0].Status == PhaseCompleted {
			t.Fatalf("phase completed on attempt %d of %d; the escape hatch must only fire after the cap",
				attempt, maxPhaseCheckpointAttempts)
		}
	}

	if err := orch.runPhase(context.Background(), &orch.campaign.Phases[0]); err != nil {
		t.Fatalf("final runPhase returned error: %v", err)
	}

	phase := orch.campaign.Phases[0]
	if phase.Status != PhaseCompleted {
		t.Fatalf("after %d failed checkpoints the phase should advance UNVERIFIED rather than spin; status = %s",
			maxPhaseCheckpointAttempts, phase.Status)
	}

	seen := drainEventTypes(events)
	if seen[EventCheckpointExhausted] == 0 {
		t.Fatalf("advancing on an unverified phase must be announced with %s; got %v", EventCheckpointExhausted, seen)
	}

	// "Completed" must never be readable as "verified": every checkpoint on
	// record failed, and that record is what the report and the operator see.
	if len(phase.Checkpoints) == 0 {
		t.Fatal("no checkpoint records survived; a phase advanced unverified with no evidence of why")
	}
	for _, cp := range phase.Checkpoints {
		if cp.Passed {
			t.Fatalf("a passing checkpoint appeared on a phase whose verification never passed: %+v", cp)
		}
	}
}
