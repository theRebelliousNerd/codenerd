package campaign

import (
	"codenerd/internal/core"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TODO: [Type Coercion] Test getCurrentPhase when Mangle fact arguments (phase ID) are returned as unexpected types (int, float, boolean) instead of string.
// TODO: [User Request Extremes] Test getCurrentPhase with phase IDs that are extremely long strings (1MB+) to check for memory exhaustion in the linear search array.
// TODO: [User Request Extremes] Test getCurrentPhase when the campaign has 10,000+ phases, testing the performance of the O(N) array search.
// TODO: [State Conflicts] Test startNextPhase deadlocks when holding o.mu.Lock() while calling external northstarObserver.OnPhaseStart which may block.
// TODO: [State Conflicts] Test completePhase deadlocks when holding o.mu.Lock() while calling external northstarObserver.OnPhaseComplete which may block.
// TODO: [State Conflicts] Test completePhase 'ghost facts' when RetractFact("campaign_phase") fails but the function returns normally without rolling back status.
func TestOrchestrator_GetCurrentPhase(t *testing.T) {
	mockKernel := &MockKernel{}
	c := &Campaign{
		ID: "/campaign_1",
		Phases: []Phase{
			{ID: "/phase_1", Name: "Phase 1"},
			{ID: "/phase_2", Name: "Phase 2"},
		},
	}

	// 1. Success case
	_ = mockKernel.Assert(core.Fact{
		Predicate: "current_phase",
		Args:      []any{"/phase_1"},
	})

	orch := &Orchestrator{
		kernel:   mockKernel,
		campaign: c,
	}

	phase := orch.getCurrentPhase()
	if phase == nil {
		t.Fatal("Expected phase, got nil")
	}
	if phase.ID != "/phase_1" {
		t.Errorf("Expected /phase_1, got %s", phase.ID)
	}

	// 2. Not found in campaign
	mockKernel.Facts = nil
	_ = mockKernel.Assert(core.Fact{
		Predicate: "current_phase",
		Args:      []any{"/phase_99"},
	})
	phase = orch.getCurrentPhase()
	if phase != nil {
		t.Errorf("Expected nil for non-existent phase, got %s", phase.ID)
	}

	// 3. Malformed fact (no arguments)
	mockKernel.Facts = nil
	_ = mockKernel.Assert(core.Fact{
		Predicate: "current_phase",
		Args:      []any{}, // No arguments
	})
	phase = orch.getCurrentPhase()
	if phase != nil {
		t.Errorf("Expected nil when fact has no arguments, got %s", phase.ID)
	}

	// 4. No fact
	mockKernel.Facts = nil
	phase = orch.getCurrentPhase()
	if phase != nil {
		t.Errorf("Expected nil when no fact exists, got %s", phase.ID)
	}

	// 5. Empty string argument
	mockKernel.Facts = nil
	_ = mockKernel.Assert(core.Fact{
		Predicate: "current_phase",
		Args:      []any{""},
	})
	phase = orch.getCurrentPhase()
	if phase != nil {
		t.Errorf("Expected nil when fact has empty string argument, got %s", phase.ID)
	}

	// 6. Nil argument
	mockKernel.Facts = nil
	_ = mockKernel.Assert(core.Fact{
		Predicate: "current_phase",
		Args:      []any{nil},
	})
	phase = orch.getCurrentPhase()
	if phase != nil {
		t.Errorf("Expected nil when fact has nil argument, got %s", phase.ID)
	}
}

// TODO: [Null/Undefined/Empty] Test getEligibleTasks with a nil Phase argument.
// TODO: [Null/Undefined/Empty] Test getEligibleTasks when 'eligible_task' fact returns missing or empty string arguments.
// TODO: [Type Coercion] Test getEligibleTasks when 'eligible_task' fact argument is coerced from non-string Atom/types.
// TODO: [State Conflicts] Test getEligibleTasks with concurrent modifications to Phase.Tasks.
func TestOrchestrator_GetEligibleTasks_ExtremeScaling(t *testing.T) {
	mockKernel := &MockKernel{}
	numTasks := 10000

	tasks := make([]Task, numTasks)
	for i := 0; i < numTasks; i++ {
		tasks[i] = Task{ID: fmt.Sprintf("/task_%d", i)}
	}

	c := &Campaign{
		ID: "/campaign_scaling",
		Phases: []Phase{
			{
				ID:    "/phase_1",
				Tasks: tasks,
			},
		},
	}

	facts := make([]core.Fact, numTasks)
	for i := 0; i < numTasks; i++ {
		facts[i] = core.Fact{Predicate: "eligible_task", Args: []any{fmt.Sprintf("/task_%d", i)}}
	}

	// Directly set mockKernel.Facts instead of AssertBatch or looped Asserts
	// since it's just a MockKernel for unit tests.
	mockKernel.Facts = facts

	orch := &Orchestrator{
		kernel:   mockKernel,
		campaign: c,
	}

	phase := &c.Phases[0]

	start := time.Now()
	eligibleTasks := orch.getEligibleTasks(phase)
	duration := time.Since(start)

	if len(eligibleTasks) != numTasks {
		t.Fatalf("Expected %d tasks, got %d", numTasks, len(eligibleTasks))
	}

	if duration > 100*time.Millisecond {
		t.Errorf("Performance test failed: O(N*M) nested loop took %v, expected < 100ms", duration)
	}
}

// TODO: [Null/Undefined/Empty] Test getNextTask with a nil Phase argument.
// TODO: [Type Coercion] Test getNextTask when 'next_campaign_task' fact argument is not a string.
// TODO: [State Conflicts] Test getNextTask when concurrent tasks are modifying the Phase structure.
func TestOrchestrator_GetNextTask(t *testing.T) {
	mockKernel := &MockKernel{}
	c := &Campaign{
		ID: "/campaign_1",
		Phases: []Phase{
			{
				ID: "/phase_1",
				Tasks: []Task{
					{ID: "/task_1"},
					{ID: "/task_2"},
				},
			},
		},
	}

	// 1. Success
	_ = mockKernel.Assert(core.Fact{
		Predicate: "next_campaign_task",
		Args:      []any{"/task_2"},
	})

	orch := &Orchestrator{
		kernel:   mockKernel,
		campaign: c,
	}

	phase := &c.Phases[0]
	task := orch.getNextTask(phase)
	if task == nil {
		t.Fatal("Expected task, got nil")
	}
	if task.ID != "/task_2" {
		t.Errorf("Expected /task_2, got %s", task.ID)
	}

	// 2. Not in phase
	mockKernel.Facts = nil
	_ = mockKernel.Assert(core.Fact{
		Predicate: "next_campaign_task",
		Args:      []any{"/task_99"},
	})
	task = orch.getNextTask(phase)
	if task != nil {
		t.Errorf("Expected nil for task not in phase, got %s", task.ID)
	}

	// 3. Type Coercion (Argument is not a string but something that gets coerced safely)
	mockKernel.Facts = nil
	_ = mockKernel.Assert(core.Fact{
		Predicate: "next_campaign_task",
		Args:      []any{123}, // Coerced to "123", which doesn't match any task ID
	})
	task = orch.getNextTask(phase)
	if task != nil {
		t.Errorf("Expected nil when coerced ID doesn't match, got %s", task.ID)
	}

}

// Completion is the kernel's: campaign_phases_done and all_phase_tasks_complete,
// derived from the campaign's own rows. The in-memory scans that decided it
// beside those rules (isCampaignComplete, isPhaseComplete) are deleted (sweep
// finding F3).
func completionOrchestrator(t *testing.T, c *Campaign) *Orchestrator {
	t.Helper()
	kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := kernel.LoadFacts(c.ToFacts()); err != nil {
		t.Fatalf("load campaign facts: %v", err)
	}
	return &Orchestrator{kernel: kernel, campaign: c}
}

func TestOrchestrator_CampaignPhasesDoneIsTheKernels(t *testing.T) {
	for _, tc := range []struct {
		name     string
		statuses []PhaseStatus
		want     bool
	}{
		{"no phases", nil, true},
		{"completed and skipped", []PhaseStatus{PhaseCompleted, PhaseSkipped}, true},
		{"one in progress", []PhaseStatus{PhaseCompleted, PhaseInProgress}, false},
		{"one pending", []PhaseStatus{PhaseCompleted, PhasePending}, false},
		{"one closed unverified", []PhaseStatus{PhaseCompleted, PhaseUnverified}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &Campaign{ID: "/campaign_done", Title: "done", Status: StatusActive}
			for i, s := range tc.statuses {
				c.Phases = append(c.Phases, Phase{ID: fmt.Sprintf("/phase_done_%d", i), CampaignID: c.ID, Name: "p", Order: i, Status: s})
			}
			if got := completionOrchestrator(t, c).campaignPhasesDone(); got != tc.want {
				t.Fatalf("campaignPhasesDone = %v, want %v", got, tc.want)
			}
		})
	}
	// A campaign the kernel holds no row for is not done: no finished
	// campaign is declared on a kernel that does not know it.
	orch := completionOrchestrator(t, &Campaign{ID: "/campaign_done", Title: "done", Status: StatusActive})
	orch.campaign = &Campaign{ID: "/campaign_unknown"}
	if orch.campaignPhasesDone() {
		t.Fatal("a campaign the kernel holds no row for was declared done")
	}
}

func TestOrchestrator_PhaseTasksDoneIsTheKernels(t *testing.T) {
	for _, tc := range []struct {
		name     string
		statuses []TaskStatus
		want     bool
	}{
		{"no tasks", nil, true},
		{"completed and skipped", []TaskStatus{TaskCompleted, TaskSkipped}, true},
		{"one in progress", []TaskStatus{TaskCompleted, TaskInProgress}, false},
		{"one pending", []TaskStatus{TaskCompleted, TaskPending}, false},
		{"one failed", []TaskStatus{TaskCompleted, TaskFailed}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			phase := Phase{ID: "/phase_tasks", CampaignID: "/campaign_tasks", Name: "p", Status: PhaseInProgress}
			for i, s := range tc.statuses {
				phase.Tasks = append(phase.Tasks, Task{ID: fmt.Sprintf("/task_done_%d", i), PhaseID: phase.ID, Description: "t", Status: s, Type: TaskTypeFileModify})
			}
			c := &Campaign{ID: "/campaign_tasks", Title: "tasks", Status: StatusActive, Phases: []Phase{phase}}
			orch := completionOrchestrator(t, c)
			if got := orch.phaseTasksDone(&c.Phases[0]); got != tc.want {
				t.Fatalf("phaseTasksDone = %v, want %v", got, tc.want)
			}
		})
	}
	if (&Orchestrator{}).phaseTasksDone(nil) {
		t.Fatal("a nil phase is done")
	}
}

// TODO: [Null/Undefined/Empty] Test getCampaignBlockReason when 'campaign_blocked' has < 2 arguments.
// TODO: [Type Coercion] Test getCampaignBlockReason when the reason argument is not a string (e.g. integer or boolean).

func TestOrchestrator_StartNextPhase_DoubleInvocation(t *testing.T) {
	mockKernel := &MockKernel{}
	c := &Campaign{
		ID: "/campaign_1",
		Phases: []Phase{
			{
				ID:     "/phase_1",
				Name:   "Phase 1",
				Status: PhasePending,
			},
		},
	}

	orch := &Orchestrator{
		kernel:   mockKernel,
		campaign: c,
	}

	_ = mockKernel.Assert(core.Fact{
		Predicate: "phase_eligible",
		Args:      []any{"/phase_1"},
	})

	err1 := orch.startNextPhase(context.Background())
	if err1 != nil {
		t.Fatalf("First invocation failed: %v", err1)
	}

	if orch.campaign.Phases[0].Status != PhaseInProgress {
		t.Fatalf("Phase should be InProgress, got %v", orch.campaign.Phases[0].Status)
	}

	err2 := orch.startNextPhase(context.Background())
	if err2 != nil {
		t.Fatalf("Second invocation failed: %v", err2)
	}

	if orch.campaign.Phases[0].Status != PhaseInProgress {
		t.Fatalf("Phase should still be InProgress, got %v", orch.campaign.Phases[0].Status)
	}
}

// Additional test for getCampaignBlockReason
func TestOrchestrator_CompletePhase_NilPhase(t *testing.T) {
	mockKernel := &MockKernel{}
	orch := &Orchestrator{kernel: mockKernel}

	// This should not panic
	orch.completePhase(nil)
}

func TestOrchestrator_GetCampaignBlockReason(t *testing.T) {
	// TODO: TestOrchestrator_Concurrency_ReadWritePhases

	mockKernel := &MockKernel{}
	orch := &Orchestrator{kernel: mockKernel}

	// 1. No block
	if reason := orch.getCampaignBlockReason(); reason != "" {
		t.Errorf("Expected empty reason, got %s", reason)
	}

	// 2. Blocked
	_ = mockKernel.Assert(core.Fact{
		Predicate: "campaign_blocked",
		Args:      []any{"some_id", "/security_violation"},
	})

	if reason := orch.getCampaignBlockReason(); reason != "/security_violation" {
		t.Errorf("Expected /security_violation, got %s", reason)
	}
}

func TestOrchestrator_CompletePhase_KernelAssertFailure(t *testing.T) {
	mockKernel := &MockKernel{
		AssertErr: errors.New("kernel assert failed"),
	}

	phase := &Phase{
		ID:     "phase-1",
		Name:   "Test Phase",
		Status: PhaseInProgress,
	}

	camp := &Campaign{
		ID:     "campaign-1",
		Phases: []Phase{*phase},
	}

	orch := &Orchestrator{
		kernel:   mockKernel,
		campaign: camp,
		nerdDir:  t.TempDir(),
	}

	orch.completePhase(phase)

	// Verify phase was still completed despite the kernel error
	if orch.campaign.Phases[0].Status != PhaseCompleted {
		t.Errorf("expected phase status to be PhaseCompleted, got %v", orch.campaign.Phases[0].Status)
	}
	if orch.campaign.CompletedPhases != 1 {
		t.Errorf("expected CompletedPhases to be 1, got %d", orch.campaign.CompletedPhases)
	}
}

func TestOrchestrator_StartNextPhase_NilContext(t *testing.T) {
	orch := &Orchestrator{}
	//lint:ignore SA1012 this test exists to prove the callee rejects a nil Context; passing context.Background() would make it assert nothing.
	err := orch.startNextPhase(nil)
	if err == nil {
		t.Errorf("Expected error when calling startNextPhase with nil context, got nil")
	} else if err.Error() != "context cannot be nil" {
		t.Errorf("Expected 'context cannot be nil' error, got %v", err)
	}
}

func TestOrchestrator_StartNextPhase_RaceCondition(t *testing.T) {
	mockKernel := &MockKernel{
		Facts: []core.Fact{
			{
				Predicate: "phase_eligible",
				Args:      []any{"phase-1"},
			},
		},
	}

	campaignState := &Campaign{
		ID: "camp-1",
		Phases: []Phase{
			{
				ID:     "phase-1",
				Name:   "Phase 1",
				Status: PhasePending,
			},
		},
	}

	orch := &Orchestrator{
		kernel:   mockKernel,
		campaign: campaignState,
	}

	ctx := context.Background()
	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = orch.startNextPhase(ctx)
		}()
	}

	wg.Wait()

	// Verify phase status was updated.
	orch.mu.RLock()
	defer orch.mu.RUnlock()
	if orch.campaign.Phases[0].Status != PhaseInProgress {
		t.Errorf("Expected phase status to be PhaseInProgress, got %s", orch.campaign.Phases[0].Status)
	}
}
