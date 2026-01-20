package campaign

import (
	"codenerd/internal/core"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestReplan_NoFailures(t *testing.T) {
	kernel := &MockKernel{}
	llm := &MockLLMClient{}
	replanner := NewReplanner(kernel, llm)
	ctx := context.Background()

	campaign := &Campaign{
		ID:     "camp1",
		Status: StatusActive,
		Phases: []Phase{
			{
				Tasks: []Task{{Status: TaskCompleted}},
			},
		},
	}

	err := replanner.Replan(ctx, campaign, "")
	if err != nil {
		t.Fatalf("Replan failed: %v", err)
	}
	// Should do nothing if no failures/blocks
}

func TestReplan_WithFailures(t *testing.T) {
	kernel := &MockKernel{}
	llm := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			if !strings.Contains(prompt, "Scope: failed_task") {
				return "", fmt.Errorf("unexpected prompt")
			}
			return `{
				"success": true,
				"change_summary": "Retrying with new approach",
				"retry_tasks": [
					{"task_id": "failed_task", "new_approach": "Use simpler logic"}
				],
				"skip_tasks": [],
				"add_tasks": []
			}`, nil
		},
	}
	replanner := NewReplanner(kernel, llm)
	ctx := context.Background()

	campaign := &Campaign{
		ID:     "camp1",
		Status: StatusActive,
		Phases: []Phase{
			{
				ID: "phase1",
				Tasks: []Task{
					{
						ID:        "failed_task",
						Status:    TaskFailed,
						LastError: "Some error",
					},
				},
			},
		},
	}

	// We pass "failed_task" as scope
	err := replanner.Replan(ctx, campaign, "failed_task")
	if err != nil {
		t.Fatalf("Replan failed: %v", err)
	}

	// Verify changes
	task := campaign.Phases[0].Tasks[0]
	if task.Description != "Use simpler logic" {
		t.Errorf("Task description not updated, got: %s", task.Description)
	}
	if task.Status != TaskPending {
		t.Errorf("Task status not reset to pending, got: %s", task.Status)
	}
	if campaign.RevisionNumber != 1 {
		t.Errorf("Revision number not incremented")
	}
}

func TestReplanForNewRequirement(t *testing.T) {
	kernel := &MockKernel{}
	llm := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return `{
				"new_tasks": [
					{"phase_order": 0, "description": "Implement new req", "type": "/file_create", "priority": "/high"}
				],
				"modified_tasks": [],
				"summary": "Added requirement task"
			}`, nil
		},
	}
	replanner := NewReplanner(kernel, llm)
	ctx := context.Background()

	campaign := &Campaign{
		ID:     "camp1",
		Status: StatusActive,
		Phases: []Phase{
			{
				ID: "phase1",
				Tasks: []Task{
					{ID: "task1", Description: "Old task"},
				},
			},
		},
	}

	err := replanner.ReplanForNewRequirement(ctx, campaign, "Need feature X")
	if err != nil {
		t.Fatalf("ReplanForNewRequirement failed: %v", err)
	}

	// Verify new task
	if len(campaign.Phases[0].Tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(campaign.Phases[0].Tasks))
	}
	newTask := campaign.Phases[0].Tasks[1]
	if newTask.Description != "Implement new req" {
		t.Errorf("Unexpected new task description: %s", newTask.Description)
	}
	if newTask.Priority != PriorityHigh {
		t.Errorf("Unexpected priority: %s", newTask.Priority)
	}

	// Verify replan trigger asserted
	triggerFound := false
	for _, f := range kernel.Facts {
		if f.Predicate == "replan_trigger" && f.Args[1] == "/new_requirement" {
			triggerFound = true
			break
		}
	}
	if !triggerFound {
		t.Error("Expected replan_trigger fact")
	}
}

func TestRefineNextPhase(t *testing.T) {
	kernel := &MockKernel{}
	llm := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return `{
				"tasks": [
					{"task_id": "next_task", "description": "Refined next task", "type": "/file_modify", "priority": "/normal", "action": "update"}
				],
				"summary": "Refined next phase"
			}`, nil
		},
	}
	replanner := NewReplanner(kernel, llm)
	ctx := context.Background()

	campaign := &Campaign{
		ID:     "camp1",
		Status: StatusActive,
		Phases: []Phase{
			{
				ID:    "phase1",
				Name:  "Phase 1",
				Order: 0,
				Tasks: []Task{{Status: TaskCompleted}},
			},
			{
				ID:    "phase2",
				Name:  "Phase 2",
				Order: 1,
				Tasks: []Task{
					{ID: "next_task", Description: "Original next task"},
				},
			},
		},
	}

	completedPhase := &campaign.Phases[0]
	err := replanner.RefineNextPhase(ctx, campaign, completedPhase)
	if err != nil {
		t.Fatalf("RefineNextPhase failed: %v", err)
	}

	// Verify update
	nextTask := campaign.Phases[1].Tasks[0]
	if nextTask.Description != "Refined next task" {
		t.Errorf("Task description not updated: %s", nextTask.Description)
	}
	if campaign.LastRevision != "Refined next phase" {
		t.Errorf("LastRevision not updated: %s", campaign.LastRevision)
	}
}

// Test GetReplanTriggers logic using MockKernel
func TestGetReplanTriggers(t *testing.T) {
	kernel := &MockKernel{}
	replanner := NewReplanner(kernel, &MockLLMClient{})

	// Inject trigger fact
	now := time.Now().Unix()
	kernel.Assert(core.Fact{
		Predicate: "replan_trigger",
		Args:      []interface{}{"camp1", "/task_failed", now},
	})

	triggers := replanner.getReplanTriggers("camp1")
	if len(triggers) != 1 {
		t.Errorf("Expected 1 trigger, got %d", len(triggers))
	}
	if triggers[0].Reason != "/task_failed" {
		t.Errorf("Unexpected trigger reason: %s", triggers[0].Reason)
	}
}
