package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/types"
	"testing"
	"time"
)

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
		Args:      []interface{}{"/phase_1"},
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
		Args:      []interface{}{"/phase_99"},
	})
	phase = orch.getCurrentPhase()
	if phase != nil {
		t.Errorf("Expected nil for non-existent phase, got %s", phase.ID)
	}

	// 3. No fact
	mockKernel.Facts = nil
	phase = orch.getCurrentPhase()
	if phase != nil {
		t.Errorf("Expected nil when no fact exists, got %s", phase.ID)
	}
}

func TestOrchestrator_GetEligibleTasks(t *testing.T) {
	mockKernel := &MockKernel{}
	c := &Campaign{
		ID: "/campaign_1",
		Phases: []Phase{
			{
				ID: "/phase_1",
				Tasks: []Task{
					{ID: "/task_1"},
					{ID: "/task_2"},
					{ID: "/task_3", NextRetryAt: time.Now().Add(1 * time.Hour)}, // Future backoff
					{ID: "/task_4", NextRetryAt: time.Now().Add(-1 * time.Hour)}, // Past backoff
				},
			},
		},
	}

	// Inject eligible_task facts
	_ = mockKernel.Assert(core.Fact{Predicate: "eligible_task", Args: []interface{}{"/task_1"}})
	_ = mockKernel.Assert(core.Fact{Predicate: "eligible_task", Args: []interface{}{"/task_3"}})
	_ = mockKernel.Assert(core.Fact{Predicate: "eligible_task", Args: []interface{}{"/task_4"}})

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
		Args:      []interface{}{"/task_2"},
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
		Args:      []interface{}{"/task_99"},
	})
	task = orch.getNextTask(phase)
	if task != nil {
		t.Errorf("Expected nil for task not in phase, got %s", task.ID)
	}
}

func TestOrchestrator_IsCampaignComplete(t *testing.T) {
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

func TestOrchestrator_IsPhaseComplete(t *testing.T) {
	// Case 1: All tasks completed or skipped
	p1 := &Phase{
		Tasks: []Task{
			{ID: "t1", Status: TaskCompleted},
			{ID: "t2", Status: TaskSkipped},
		},
	}
	orch := &Orchestrator{}
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

// Additional test for getCampaignBlockReason
func TestOrchestrator_GetCampaignBlockReason(t *testing.T) {
	mockKernel := &MockKernel{}
	orch := &Orchestrator{kernel: mockKernel}

	// 1. No block
	if reason := orch.getCampaignBlockReason(); reason != "" {
		t.Errorf("Expected empty reason, got %s", reason)
	}

	// 2. Blocked
	_ = mockKernel.Assert(core.Fact{
		Predicate: "campaign_blocked",
		Args:      []interface{}{"some_id", "/security_violation"},
	})

	if reason := orch.getCampaignBlockReason(); reason != "/security_violation" {
		t.Errorf("Expected /security_violation, got %s", reason)
	}
}
