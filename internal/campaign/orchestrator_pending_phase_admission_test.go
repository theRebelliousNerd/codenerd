package campaign

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/session"
)

// campaign-pending-phase-admission #1: current_phase also names an eligible
// phase nothing has started. Run executed it without the phase start -- no
// admission check, no /in_progress fact, no phase clock, no phase-started
// event. A selected phase is started before its tasks run.
func TestRun_SelectedPhaseIsStartedBeforeItsTasksRun(t *testing.T) {
	t.Parallel()
	orch, _ := newPolicyFailureOrchestrator(t, nil, Task{
		ID: "/task_selected_phase", Description: "work in a selected phase", Type: TaskTypeResearch,
	})
	orch.campaign.Phases[0].Status = PhasePending
	orch.kernel = realKernelFor(t, orch.campaign, true)
	if p := orch.getCurrentPhase(); p == nil || p.ID != "/phase_failure_policy" {
		t.Fatalf("precondition: the pending phase must be current_phase, got %+v", p)
	}
	startedBeforeTask := false
	orch.taskExecutor = &MockTaskExecutor{ExecuteFunc: func(context.Context, session.TaskRequest) (string, error) {
		orch.mu.RLock()
		startedBeforeTask = orch.campaign.Phases[0].Status == PhaseInProgress
		orch.mu.RUnlock()
		return "ran", nil
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := orch.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	sawStart := false
	for {
		select {
		case e := <-orch.eventChan:
			if e.Type == EventPhaseStarted {
				sawStart = true
			}
			continue
		default:
		}
		break
	}
	if !sawStart || !startedBeforeTask {
		t.Fatalf("the selected phase ran without being started (phase-started event %v, started before its task %v)", sawStart, startedBeforeTask)
	}
}
