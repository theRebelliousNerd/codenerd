package campaign

import (
	"context"
	"testing"
)

// TestReplanForNewRequirement_DuplicateSuppression verifies the per-phase
// duplicate guard for the first replanner construction site
// (ReplanForNewRequirement). A replan proposing a task whose description
// already exists in the phase must not add a second copy and must not inflate
// TotalTasks. It also verifies case/punctuation insensitive matching via
// normalizedTaskKey.
func TestReplanForNewRequirement_DuplicateSuppression(t *testing.T) {
	tests := []struct {
		name         string
		existingDesc string
		newDesc      string
		wantLen      int
		wantTotal    int
	}{
		{
			name:         "exact duplicate suppressed",
			existingDesc: "Implement user login",
			newDesc:      "Implement user login",
			wantLen:      1,
			wantTotal:    1,
		},
		{
			name:         "normalized duplicate suppressed case punctuation",
			existingDesc: "Implement user login",
			newDesc:      "  implement  USER   login!  ",
			wantLen:      1,
			wantTotal:    1,
		},
		{
			name:         "genuinely new description added",
			existingDesc: "Implement user login",
			newDesc:      "Implement user logout",
			wantLen:      2,
			wantTotal:    2,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			campaign := &Campaign{
				ID:    "test-replan-dup-1",
				Title: "Test",
				Goal:  "test",
				Phases: []Phase{
					{
						ID:       "/phase_test_0",
						Order:    0,
						Category: "/scaffold",
						Tasks: []Task{
							{ID: "/task_test_0_0", PhaseID: "/phase_test_0", Description: tc.existingDesc, Status: TaskPending, Type: TaskTypeFileCreate},
						},
					},
				},
				TotalPhases: 1,
				TotalTasks:  1,
			}
			r := NewReplanner(&MockKernel{}, &MockLLMClient{
				CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
					return `{"new_tasks": [{"phase_order": 0, "description": "` + tc.newDesc + `", "type": "/file_create", "priority": "/high"}], "modified_tasks": [], "summary": "ok"}`, nil
				},
			})
			if err := r.ReplanForNewRequirement(context.Background(), campaign, "new requirement"); err != nil {
				t.Fatalf("ReplanForNewRequirement failed: %v", err)
			}
			if got := len(campaign.Phases[0].Tasks); got != tc.wantLen {
				t.Fatalf("task count = %d, want %d; tasks=%v", got, tc.wantLen, campaign.Phases[0].Tasks)
			}
			if campaign.TotalTasks != tc.wantTotal {
				t.Fatalf("TotalTasks = %d, want %d", campaign.TotalTasks, tc.wantTotal)
			}
			// Ensure duplicate case did not create a second ID with same description
			if tc.wantLen == 1 {
				if campaign.Phases[0].Tasks[0].Description != tc.existingDesc {
					t.Fatalf("original task description mutated: got %q want %q", campaign.Phases[0].Tasks[0].Description, tc.existingDesc)
				}
			}
		})
	}
}

func TestReplanForNewRequirement_ContextFromMapping(t *testing.T) {
	campaign := &Campaign{
		ID:    "test-replan-ctx-1",
		Title: "Test",
		Goal:  "test",
		Phases: []Phase{
			{
				ID:       "/phase_test_0",
				Order:    0,
				Category: "/scaffold",
				Tasks: []Task{
					{ID: "/task_test_0_0", PhaseID: "/phase_test_0", Description: "Research auth design", Status: TaskCompleted, Type: TaskTypeResearch},
					{ID: "/task_test_0_1", PhaseID: "/phase_test_0", Description: "Implement auth", Status: TaskPending, Type: TaskTypeFileCreate},
				},
			},
		},
		TotalPhases: 1,
		TotalTasks:  2,
	}
	// New task that names a dependency on the research task via description
	r := NewReplanner(&MockKernel{}, &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return `{"new_tasks": [{"phase_order": 0, "description": "Synthesize auth report", "type": "/document", "priority": "/high", "depends_on": ["Research auth design"]}], "modified_tasks": [], "summary": "ok"}`, nil
		},
	})
	if err := r.ReplanForNewRequirement(context.Background(), campaign, "need synthesis"); err != nil {
		t.Fatalf("ReplanForNewRequirement failed: %v", err)
	}
	if len(campaign.Phases[0].Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(campaign.Phases[0].Tasks))
	}
	newTask := campaign.Phases[0].Tasks[2]
	if newTask.Description != "Synthesize auth report" {
		t.Fatalf("new task description mismatch: %q", newTask.Description)
	}
	if len(newTask.ContextFrom) != 1 {
		t.Fatalf("expected ContextFrom length 1, got %v", newTask.ContextFrom)
	}
	if newTask.ContextFrom[0] != "/task_test_0_0" {
		t.Fatalf("expected ContextFrom %q, got %q", "/task_test_0_0", newTask.ContextFrom[0])
	}
	// Task without dependencies must stay empty rather than inventing edges
	campaign2 := &Campaign{
		ID:    "test-replan-ctx-2",
		Title: "Test",
		Goal:  "test",
		Phases: []Phase{
			{
				ID:       "/phase_test_0",
				Order:    0,
				Category: "/scaffold",
				Tasks: []Task{
					{ID: "/task_test_0_0", PhaseID: "/phase_test_0", Description: "Research auth design", Status: TaskCompleted, Type: TaskTypeResearch},
				},
			},
		},
		TotalPhases: 1,
		TotalTasks:  1,
	}
	r2 := NewReplanner(&MockKernel{}, &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return `{"new_tasks": [{"phase_order": 0, "description": "Implement auth", "type": "/file_create", "priority": "/high"}], "modified_tasks": [], "summary": "ok"}`, nil
		},
	})
	if err := r2.ReplanForNewRequirement(context.Background(), campaign2, "need impl"); err != nil {
		t.Fatalf("ReplanForNewRequirement failed: %v", err)
	}
	if len(campaign2.Phases[0].Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(campaign2.Phases[0].Tasks))
	}
	if got := campaign2.Phases[0].Tasks[1].ContextFrom; len(got) != 0 {
		t.Fatalf("expected empty ContextFrom for task without dependencies, got %v", got)
	}
}

func TestReplanForNewRequirement_ContextFromViaID(t *testing.T) {
	campaign := &Campaign{
		ID:    "test-replan-ctx-id",
		Title: "Test",
		Goal:  "test",
		Phases: []Phase{
			{
				ID:       "/phase_test_0",
				Order:    0,
				Category: "/scaffold",
				Tasks: []Task{
					{ID: "/task_test_0_0", PhaseID: "/phase_test_0", Description: "Research auth design", Status: TaskCompleted, Type: TaskTypeResearch},
				},
			},
		},
		TotalPhases: 1,
		TotalTasks:  1,
	}
	r := NewReplanner(&MockKernel{}, &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return `{"new_tasks": [{"phase_order": 0, "description": "Synthesize report", "type": "/document", "priority": "/high", "context_from": ["/task_test_0_0"]}], "modified_tasks": [], "summary": "ok"}`, nil
		},
	})
	if err := r.ReplanForNewRequirement(context.Background(), campaign, "need synthesis"); err != nil {
		t.Fatalf("ReplanForNewRequirement failed: %v", err)
	}
	if len(campaign.Phases[0].Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(campaign.Phases[0].Tasks))
	}
	if len(campaign.Phases[0].Tasks[1].ContextFrom) != 1 || campaign.Phases[0].Tasks[1].ContextFrom[0] != "/task_test_0_0" {
		t.Fatalf("expected ContextFrom [/task_test_0_0], got %v", campaign.Phases[0].Tasks[1].ContextFrom)
	}
}

func TestRefineNextPhase_DuplicateSuppression(t *testing.T) {
	tests := []struct {
		name      string
		existing  string
		newDesc   string
		wantLen   int
		wantTotal int
	}{
		{
			name:      "exact duplicate suppressed in rolling wave",
			existing:  "Implement payment flow",
			newDesc:   "Implement payment flow",
			wantLen:   1,
			wantTotal: 2, // phase0 1 task + phase1 1 task (duplicate not added)
		},
		{
			name:      "normalized duplicate suppressed",
			existing:  "Implement payment flow",
			newDesc:   "implement PAYMENT flow!!!",
			wantLen:   1,
			wantTotal: 2,
		},
		{
			name:      "new task added",
			existing:  "Implement payment flow",
			newDesc:   "Implement refund flow",
			wantLen:   2,
			wantTotal: 3,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			campaign := &Campaign{
				ID:    "test-refine-dup",
				Title: "Test",
				Goal:  "test",
				Phases: []Phase{
					{ID: "/phase_test_0", Order: 0, Tasks: []Task{{ID: "/t0", PhaseID: "/phase_test_0", Description: "prior", Status: TaskCompleted}}},
					{ID: "/phase_test_1", Order: 1, Category: "/scaffold", Tasks: []Task{{ID: "/t1", PhaseID: "/phase_test_1", Description: tc.existing, Status: TaskPending}}},
				},
				TotalPhases: 2,
				TotalTasks:  2,
			}
			completed := &campaign.Phases[0]
			r := NewReplanner(&MockKernel{}, &MockLLMClient{
				CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
					return `{"tasks": [{"task_id": "", "description": "` + tc.newDesc + `", "type": "/file_create", "priority": "/high", "action": "add"}], "summary": "ok"}`, nil
				},
			})
			if err := r.RefineNextPhase(context.Background(), campaign, completed); err != nil {
				t.Fatalf("RefineNextPhase failed: %v", err)
			}
			if got := len(campaign.Phases[1].Tasks); got != tc.wantLen {
				t.Fatalf("next phase task count = %d, want %d; tasks=%v", got, tc.wantLen, campaign.Phases[1].Tasks)
			}
			if campaign.TotalTasks != tc.wantTotal {
				t.Fatalf("TotalTasks = %d, want %d", campaign.TotalTasks, tc.wantTotal)
			}
		})
	}
}

func TestRefineNextPhase_DuplicateViaUpdateFallback(t *testing.T) {
	// The fallback path where action is update but task not found triggers an add.
	// Duplicate guard must also apply there.
	campaign := &Campaign{
		ID:    "test-refine-fallback-dup",
		Title: "Test",
		Goal:  "test",
		Phases: []Phase{
			{ID: "/phase_test_0", Order: 0, Tasks: []Task{{ID: "/t0", PhaseID: "/phase_test_0", Description: "prior", Status: TaskCompleted}}},
			{ID: "/phase_test_1", Order: 1, Category: "/scaffold", Tasks: []Task{{ID: "/t1", PhaseID: "/phase_test_1", Description: "Implement payment flow", Status: TaskPending}}},
		},
		TotalPhases: 2,
		TotalTasks:  2,
	}
	r := NewReplanner(&MockKernel{}, &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			// No explicit action "add", so defaults to update path; t.TaskID does not match existing, so it falls through to add.
			return `{"tasks": [{"task_id": "nonexistent", "description": "Implement payment flow", "type": "/file_create", "priority": "/high", "action": "update"}], "summary": "ok"}`, nil
		},
	})
	if err := r.RefineNextPhase(context.Background(), campaign, &campaign.Phases[0]); err != nil {
		t.Fatalf("RefineNextPhase failed: %v", err)
	}
	if got := len(campaign.Phases[1].Tasks); got != 1 {
		t.Fatalf("expected duplicate suppressed via fallback path, got %d tasks: %v", got, campaign.Phases[1].Tasks)
	}
	if campaign.TotalTasks != 2 {
		t.Fatalf("TotalTasks should not inflate for duplicate via fallback, got %d", campaign.TotalTasks)
	}
}

func TestRefineNextPhase_ContextFromMapping(t *testing.T) {
	campaign := &Campaign{
		ID:    "test-refine-ctx",
		Title: "Test",
		Goal:  "test",
		Phases: []Phase{
			{ID: "/phase_test_0", Order: 0, Tasks: []Task{{ID: "/t0", PhaseID: "/phase_test_0", Description: "prior", Status: TaskCompleted}}},
			{
				ID:       "/phase_test_1",
				Order:    1,
				Category: "/scaffold",
				Tasks: []Task{
					{ID: "/t1", PhaseID: "/phase_test_1", Description: "Research payment design", Status: TaskCompleted, Type: TaskTypeResearch},
				},
			},
		},
		TotalPhases: 2,
		TotalTasks:  2,
	}
	r := NewReplanner(&MockKernel{}, &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return `{"tasks": [{"task_id": "", "description": "Synthesize payment report", "type": "/document", "priority": "/high", "action": "add", "depends_on": ["Research payment design"]}], "summary": "ok"}`, nil
		},
	})
	if err := r.RefineNextPhase(context.Background(), campaign, &campaign.Phases[0]); err != nil {
		t.Fatalf("RefineNextPhase failed: %v", err)
	}
	if len(campaign.Phases[1].Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(campaign.Phases[1].Tasks))
	}
	newTask := campaign.Phases[1].Tasks[1]
	if len(newTask.ContextFrom) != 1 {
		t.Fatalf("expected ContextFrom 1, got %v", newTask.ContextFrom)
	}
	if newTask.ContextFrom[0] != "/t1" {
		t.Fatalf("expected ContextFrom [/t1], got %v", newTask.ContextFrom)
	}
	// Without dependencies, ContextFrom must stay empty
	campaign2 := &Campaign{
		ID:    "test-refine-ctx-empty",
		Title: "Test",
		Goal:  "test",
		Phases: []Phase{
			{ID: "/phase_test_0", Order: 0, Tasks: []Task{{ID: "/t0", PhaseID: "/phase_test_0", Description: "prior", Status: TaskCompleted}}},
			{ID: "/phase_test_1", Order: 1, Category: "/scaffold", Tasks: []Task{}},
		},
		TotalPhases: 2,
		TotalTasks:  1,
	}
	r2 := NewReplanner(&MockKernel{}, &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return `{"tasks": [{"task_id": "", "description": "Implement payment flow", "type": "/file_create", "priority": "/high", "action": "add"}], "summary": "ok"}`, nil
		},
	})
	if err := r2.RefineNextPhase(context.Background(), campaign2, &campaign2.Phases[0]); err != nil {
		t.Fatalf("RefineNextPhase failed: %v", err)
	}
	if len(campaign2.Phases[1].Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(campaign2.Phases[1].Tasks))
	}
	if got := campaign2.Phases[1].Tasks[0].ContextFrom; len(got) != 0 {
		t.Fatalf("expected empty ContextFrom for task without deps, got %v", got)
	}
}

func TestRefineNextPhase_PerPhaseDuplicateIsolation(t *testing.T) {
	// Same description in a different phase should NOT be suppressed (scoped per phase)
	campaign := &Campaign{
		ID:    "test-refine-isolation",
		Title: "Test",
		Goal:  "test",
		Phases: []Phase{
			{ID: "/phase_test_0", Order: 0, Tasks: []Task{{ID: "/t0", PhaseID: "/phase_test_0", Description: "Implement payment flow", Status: TaskCompleted}}},
			{ID: "/phase_test_1", Order: 1, Category: "/scaffold", Tasks: []Task{}},
		},
		TotalPhases: 2,
		TotalTasks:  1,
	}
	r := NewReplanner(&MockKernel{}, &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return `{"tasks": [{"task_id": "", "description": "Implement payment flow", "type": "/file_create", "priority": "/high", "action": "add"}], "summary": "ok"}`, nil
		},
	})
	if err := r.RefineNextPhase(context.Background(), campaign, &campaign.Phases[0]); err != nil {
		t.Fatalf("RefineNextPhase failed: %v", err)
	}
	if len(campaign.Phases[1].Tasks) != 1 {
		t.Fatalf("expected task added in different phase despite same description elsewhere, got %d", len(campaign.Phases[1].Tasks))
	}
}
