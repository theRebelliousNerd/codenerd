package campaign

import (
	"context"
	"fmt"
	"slices"
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
// The bounded escape hatch is not an exception to that: after
// maxPhaseCheckpointAttempts the phase stops spinning failure -> replan ->
// re-checkpoint and closes /unverified -- not completed -- so the phases built
// on it stay blocked and a resume re-arms the checkpoint.
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

func drainEventTypes(ch chan OrchestratorEvent) map[OrchestratorEventType]int {
	seen := make(map[OrchestratorEventType]int)
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
	orch, _ := newCheckpointRegressionOrchestrator(t, `{"control_packet": {"mangle_updates": ["checkpoint_verdict(\"verified phase\", /pass, \"everything verified\", 95)"]}, "surface_response": "done"}`)

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

// exhaustCheckpoints runs the phase until its checkpoint attempts are spent.
func exhaustCheckpoints(t *testing.T, orch *Orchestrator) {
	t.Helper()
	for attempt := 1; attempt <= maxPhaseCheckpointAttempts; attempt++ {
		if err := orch.runPhase(context.Background(), &orch.campaign.Phases[0]); err != nil {
			t.Fatalf("runPhase attempt %d returned error: %v", attempt, err)
		}
		if orch.campaign.Phases[0].Status == PhaseCompleted {
			t.Fatalf("phase completed on attempt %d of %d with every checkpoint failing", attempt, maxPhaseCheckpointAttempts)
		}
	}
}

// External audit N03 (2026-09-19): the cap on failed checkpoints used to
// complete the phase -- /completed in the kernel, a completed-phase count, a
// success to the Northstar observer -- so a known failed checkpoint unlocked
// the phases built on it. The phase closes /unverified now: announced, and not
// completed anywhere.
func TestRunPhase_WhenCheckpointExhausted_ThePhaseClosesUnverified(t *testing.T) {
	orch, events := newCheckpointRegressionOrchestrator(t, "FAIL: still broken")
	exhaustCheckpoints(t, orch)

	phase := orch.campaign.Phases[0]
	if phase.Status != PhaseUnverified {
		t.Fatalf("after %d failed checkpoints status = %s, want %s", maxPhaseCheckpointAttempts, phase.Status, PhaseUnverified)
	}
	if orch.campaign.CompletedPhases != 0 {
		t.Fatalf("CompletedPhases = %d, want 0: an unverified phase is not a completed one", orch.campaign.CompletedPhases)
	}

	seen := drainEventTypes(events)
	if seen[EventCheckpointExhausted] == 0 {
		t.Fatalf("closing a phase unverified must be announced with %s; got %v", EventCheckpointExhausted, seen)
	}
	if seen[EventPhaseCompleted] != 0 {
		t.Fatalf("a phase_completed event was emitted for a phase whose checkpoint never passed; got %v", seen)
	}

	var rows []string
	for _, f := range orch.kernel.(*MockKernel).Facts {
		if f.Predicate == "campaign_phase" && len(f.Args) > 4 {
			rows = append(rows, fmt.Sprint(f.Args[4]))
		}
	}
	if !slices.Contains(rows, "/unverified") || slices.Contains(rows, "/completed") {
		t.Fatalf("kernel campaign_phase statuses = %v, want /unverified and never /completed", rows)
	}

	// Every checkpoint on record failed, and that record is what the report
	// and the operator see.
	if len(phase.Checkpoints) == 0 {
		t.Fatal("no checkpoint records survived; a phase closed unverified with no evidence of why")
	}
	for _, cp := range phase.Checkpoints {
		if cp.Passed {
			t.Fatalf("a passing checkpoint appeared on a phase whose verification never passed: %+v", cp)
		}
	}
}

// A resume re-arms the checkpoint: the phase is back in progress with a fresh
// attempt budget, still not completed -- the debt survives -- and only a
// passing checkpoint completes it.
func TestPrepareResume_ReArmsAnUnverifiedPhase(t *testing.T) {
	orch, _ := newCheckpointRegressionOrchestrator(t, "FAIL: still broken")
	exhaustCheckpoints(t, orch)
	// What the loop does when campaign_blocked(/phase_unverified) derives.
	orch.campaign.Status = StatusFailed

	if err := orch.PrepareResume(); err != nil {
		t.Fatalf("PrepareResume: %v", err)
	}
	phase := orch.campaign.Phases[0]
	if phase.Status != PhaseInProgress || phase.CheckpointFailures != 0 {
		t.Fatalf("after resume status = %s, CheckpointFailures = %d; want %s with a fresh budget", phase.Status, phase.CheckpointFailures, PhaseInProgress)
	}
	if orch.campaign.CompletedPhases != 0 {
		t.Fatalf("CompletedPhases = %d after resume, want 0", orch.campaign.CompletedPhases)
	}

	orch.checkpoint = NewCheckpointRunner(nil, &MockTaskExecutor{
		ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
			return `{"control_packet": {"mangle_updates": ["checkpoint_verdict(\"verified phase\", /pass, \"fixed\", 95)"]}, "surface_response": "done"}`, nil
		},
	}, orch.workspace)
	if err := orch.runPhase(context.Background(), &orch.campaign.Phases[0]); err != nil {
		t.Fatalf("runPhase after resume: %v", err)
	}
	if orch.campaign.Phases[0].Status != PhaseCompleted || orch.campaign.CompletedPhases != 1 {
		t.Fatalf("a passing checkpoint after resume must complete the phase: status = %s, CompletedPhases = %d",
			orch.campaign.Phases[0].Status, orch.campaign.CompletedPhases)
	}
}
