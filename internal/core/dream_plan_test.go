package core

import (
	"testing"
)

func TestDreamPlan_New(t *testing.T) {
	plan := NewDreamPlan("", "Fix all bugs")

	if plan == nil {
		t.Fatal("NewDreamPlan returned nil")
	}

	if plan.ID == "" {
		t.Error("Plan missing auto-generated ID")
	}

	if plan.Hypothetical != "Fix all bugs" {
		t.Errorf("Expected hypothetical 'Fix all bugs', got %q", plan.Hypothetical)
	}

	if plan.Status != DreamPlanStatusPending {
		t.Errorf("Expected status pending, got %s", plan.Status)
	}
}

func TestDreamPlan_AddSubtask(t *testing.T) {
	plan := NewDreamPlan("plan-1", "Test plan")

	subtask := DreamSubtask{
		ID:        "task-1",
		Order:     1,
		ShardName: "coder",
		Task:      "Implement feature",
		Status:    SubtaskStatusPending,
	}

	plan.AddSubtask(subtask)

	if len(plan.Subtasks) != 1 {
		t.Errorf("Expected 1 subtask, got %d", len(plan.Subtasks))
	}
}

func TestDreamPlan_GetNextPendingSubtask(t *testing.T) {
	plan := NewDreamPlan("plan-1", "Test")

	plan.AddSubtask(DreamSubtask{ID: "t1", Order: 1, Status: SubtaskStatusPending})
	plan.AddSubtask(DreamSubtask{ID: "t2", Order: 2, Status: SubtaskStatusPending, DependsOn: []int{0}})

	// First pending should be t1
	next := plan.GetNextPendingSubtask()
	if next == nil {
		t.Fatal("Expected next subtask, got nil")
	}
	if next.ID != "t1" {
		t.Errorf("Expected t1, got %s", next.ID)
	}

	// Mark t1 complete
	plan.MarkSubtaskCompleted("t1", "done")

	// Now t2 should be next
	next = plan.GetNextPendingSubtask()
	if next == nil {
		t.Fatal("Expected t2, got nil")
	}
	if next.ID != "t2" {
		t.Errorf("Expected t2, got %s", next.ID)
	}
}

func TestDreamPlan_MarkSubtaskRunning(t *testing.T) {
	plan := NewDreamPlan("plan-1", "Test")
	plan.AddSubtask(DreamSubtask{ID: "t1", Status: SubtaskStatusPending})

	plan.MarkSubtaskRunning("t1")

	if plan.Subtasks[0].Status != SubtaskStatusRunning {
		t.Errorf("Expected running, got %s", plan.Subtasks[0].Status)
	}
}

func TestDreamPlan_MarkSubtaskFailed(t *testing.T) {
	plan := NewDreamPlan("plan-1", "Test")
	plan.AddSubtask(DreamSubtask{ID: "t1", Status: SubtaskStatusRunning})

	plan.MarkSubtaskFailed("t1", "compile error")

	if plan.Subtasks[0].Status != SubtaskStatusFailed {
		t.Errorf("Expected failed, got %s", plan.Subtasks[0].Status)
	}
	if plan.Subtasks[0].Error != "compile error" {
		t.Errorf("Expected error 'compile error', got %q", plan.Subtasks[0].Error)
	}
}

func TestDreamPlan_IsComplete(t *testing.T) {
	plan := NewDreamPlan("plan-1", "Test")
	plan.AddSubtask(DreamSubtask{ID: "t1", Status: SubtaskStatusCompleted})
	plan.AddSubtask(DreamSubtask{ID: "t2", Status: SubtaskStatusSkipped})

	if !plan.IsComplete() {
		t.Error("Expected plan to be complete")
	}

	plan.AddSubtask(DreamSubtask{ID: "t3", Status: SubtaskStatusPending})
	if plan.IsComplete() {
		t.Error("Expected plan to not be complete with pending task")
	}
}

func TestDreamPlan_AllSucceeded(t *testing.T) {
	plan := NewDreamPlan("plan-1", "Test")
	plan.AddSubtask(DreamSubtask{ID: "t1", Status: SubtaskStatusCompleted})
	plan.AddSubtask(DreamSubtask{ID: "t2", Status: SubtaskStatusCompleted})

	if !plan.AllSucceeded() {
		t.Error("Expected all succeeded")
	}

	plan.Subtasks[1].Status = SubtaskStatusFailed
	if plan.AllSucceeded() {
		t.Error("Expected not all succeeded with failed task")
	}
}

func TestDreamPlan_Progress(t *testing.T) {
	plan := NewDreamPlan("plan-1", "Test")

	// Empty plan
	if plan.Progress() != 0 {
		t.Errorf("Expected 0 progress for empty plan, got %f", plan.Progress())
	}

	plan.AddSubtask(DreamSubtask{ID: "t1", Status: SubtaskStatusCompleted})
	plan.AddSubtask(DreamSubtask{ID: "t2", Status: SubtaskStatusPending})

	progress := plan.Progress()
	if progress != 0.5 {
		t.Errorf("Expected 0.5 progress, got %f", progress)
	}
}
