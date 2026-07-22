package campaign

import (
	"codenerd/internal/core"
	"fmt"
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

	// 4. Malformed fact (no arguments)
	mockKernel.Facts = nil
	_ = mockKernel.Assert(core.Fact{
		Predicate: "current_phase",
		Args:      []any{}, // No arguments
	})
	phase = orch.getCurrentPhase()
	if phase != nil {
		t.Errorf("Expected nil when fact has no arguments, got %s", phase.ID)
	}

	// 3. No fact
	mockKernel.Facts = nil
	phase = orch.getCurrentPhase()
	if phase != nil {
		t.Errorf("Expected nil when no fact exists, got %s", phase.ID)
	}

	// 4. Empty string argument
	mockKernel.Facts = nil
	_ = mockKernel.Assert(core.Fact{
		Predicate: "current_phase",
		Args:      []any{""},
	})
	phase = orch.getCurrentPhase()
	if phase != nil {
		t.Errorf("Expected nil when fact has empty string argument, got %s", phase.ID)
	}

	// 5. No arguments
	mockKernel.Facts = nil
	_ = mockKernel.Assert(core.Fact{
		Predicate: "current_phase",
		Args:      []any{},
	})
	phase = orch.getCurrentPhase()
	if phase != nil {
		t.Errorf("Expected nil when fact has no arguments, got %s", phase.ID)
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
func TestOrchestrator_GetEligibleTasks(t *testing.T) {
	// TODO: TestOrchestrator_GetEligibleTasks_ExtremeBackoff
	mockKernel := &MockKernel{}
	c := &Campaign{
		ID: "/campaign_1",
		Phases: []Phase{
			{
				ID: "/phase_1",
				Tasks: []Task{
					{ID: "/task_1"},
					{ID: "/task_2"},
					{ID: "/task_3", NextRetryAt: time.Now().Add(1 * time.Hour)},  // Future backoff
					{ID: "/task_4", NextRetryAt: time.Now().Add(-1 * time.Hour)}, // Past backoff
				},
			},
		},
	}

	// Inject eligible_task facts
	_ = mockKernel.Assert(core.Fact{Predicate: "eligible_task", Args: []any{"/task_1"}})
	_ = mockKernel.Assert(core.Fact{Predicate: "eligible_task", Args: []any{"/task_3"}})
	_ = mockKernel.Assert(core.Fact{Predicate: "eligible_task", Args: []any{"/task_4"}})

	orch := &Orchestrator{
		kernel:   mockKernel,
		campaign: c,
	}

	phase := &c.Phases[0]
	tasks := orch.getEligibleTasks(phase)

	// Expectations:
	// /task_1: Eligible and no backoff -> Included
	// /task_2: Not eligible -> Excluded
	// /task_3: Eligible but future backoff -> Excluded
	// /task_4: Eligible and past backoff -> Included

	if len(tasks) != 2 {
		t.Fatalf("Expected 2 tasks, got %d", len(tasks))
	}

	found1 := false
	found4 := false
	for _, task := range tasks {
		if task.ID == "/task_1" {
			found1 = true
		}
		if task.ID == "/task_4" {
			found4 = true
		}
	}

	if !found1 {
		t.Error("Expected /task_1 to be eligible")
	}
	if !found4 {
		t.Error("Expected /task_4 to be eligible (backoff expired)")
	}
}

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

// TODO: [Null/Undefined/Empty] Test isCampaignComplete when o.campaign is nil or o.campaign.Phases is empty/nil.
// TODO: [User Request Extremes] Test isCampaignComplete with 10,000+ phases to measure iterator overhead.
// TODO: [State Conflicts] Test isCampaignComplete when another goroutine is modifying Phase.Status concurrently.
func TestOrchestrator_IsCampaignComplete(t *testing.T) {
	// Case: Nil campaign
	orchNil := &Orchestrator{campaign: nil}
	if !orchNil.isCampaignComplete() {
		t.Error("Nil campaign should be complete")
	}

	// Case: Empty phases
	cEmpty := &Campaign{Phases: []Phase{}}
	orchEmpty := &Orchestrator{campaign: cEmpty}
	if !orchEmpty.isCampaignComplete() {
		t.Error("Empty campaign should be complete")
	}
	// Case 1: All completed or skipped
	c1 := &Campaign{
		Phases: []Phase{
			{ID: "p1", Status: PhaseCompleted},
			{ID: "p2", Status: PhaseSkipped},
		},
	}
	orch1 := &Orchestrator{campaign: c1}
	if !orch1.isCampaignComplete() {
		t.Error("Campaign should be complete")
	}

	// Case 2: One in progress
	c2 := &Campaign{
		Phases: []Phase{
			{ID: "p1", Status: PhaseCompleted},
			{ID: "p2", Status: PhaseInProgress},
		},
	}
	orch2 := &Orchestrator{campaign: c2}
	if orch2.isCampaignComplete() {
		t.Error("Campaign should not be complete (p2 in progress)")
	}

	// Case 3: One pending
	c3 := &Campaign{
		Phases: []Phase{
			{ID: "p1", Status: PhaseCompleted},
			{ID: "p2", Status: PhasePending},
		},
	}
	orch3 := &Orchestrator{campaign: c3}
	if orch3.isCampaignComplete() {
		t.Error("Campaign should not be complete (p2 pending)")
	}
}

// TODO: [Null/Undefined/Empty] Test isPhaseComplete when phase argument is nil or phase.Tasks is nil/empty.
func TestOrchestrator_IsPhaseComplete(t *testing.T) {
	orch := &Orchestrator{}

	// Case: Nil Phase
	if orch.isPhaseComplete(nil) {
		t.Error("Phase should not be complete when phase is nil")
	}

	// Case: Nil Tasks
	pNilTasks := &Phase{Tasks: nil}
	if !orch.isPhaseComplete(pNilTasks) {
		t.Error("Phase should be complete when Tasks is nil")
	}

	// Case: Empty Tasks
	pEmptyTasks := &Phase{Tasks: []Task{}}
	if !orch.isPhaseComplete(pEmptyTasks) {
		t.Error("Phase should be complete when Tasks is empty")
	}

	// Case 1: All tasks completed or skipped
	p1 := &Phase{
		Tasks: []Task{
			{ID: "t1", Status: TaskCompleted},
			{ID: "t2", Status: TaskSkipped},
		},
	}
	if !orch.isPhaseComplete(p1) {
		t.Error("Phase should be complete")
	}

	// Case 2: Task in progress
	p2 := &Phase{
		Tasks: []Task{
			{ID: "t1", Status: TaskCompleted},
			{ID: "t2", Status: TaskInProgress},
		},
	}
	if orch.isPhaseComplete(p2) {
		t.Error("Phase should not be complete (t2 in progress)")
	}

	// Case 3: Task pending
	p3 := &Phase{
		Tasks: []Task{
			{ID: "t1", Status: TaskCompleted},
			{ID: "t2", Status: TaskPending},
		},
	}
	if orch.isPhaseComplete(p3) {
		t.Error("Phase should not be complete (t2 pending)")
	}
}

// TODO: [Null/Undefined/Empty] Test getCampaignBlockReason when 'campaign_blocked' has < 2 arguments.
// TODO: [Type Coercion] Test getCampaignBlockReason when the reason argument is not a string (e.g. integer or boolean).
// Additional test for getCampaignBlockReason
func TestOrchestrator_GetCampaignBlockReason(t *testing.T) {
	// TODO: TestOrchestrator_StartNextPhase_NilContext
	// TODO: TestOrchestrator_StartNextPhase_RaceCondition
	// TODO: TestOrchestrator_StartNextPhase_DoubleInvocation
	// TODO: TestOrchestrator_CompletePhase_NilPhase
	// TODO: TestOrchestrator_CompletePhase_KernelAssertFailure
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
