package campaign

import (
	"context"
	"testing"
)

// Campaign 440585a6: the refinement restated all three tasks of the next phase
// with reworded descriptions and returned their IDs without the leading slash.
// The exact-string ID match missed every one, each "update" fell through to
// "add", and a finished three-task phase ran a second time as six.
func TestRefineNextPhase_UpdateMatchesAnIDReturnedWithoutItsSlash(t *testing.T) {
	campaign := &Campaign{
		ID:    "test-refine-slashless-id",
		Title: "Test",
		Goal:  "test",
		Phases: []Phase{
			{ID: "/phase_x_0", Order: 0, Tasks: []Task{{ID: "/task_x_0_0", PhaseID: "/phase_x_0", Description: "inventory", Status: TaskCompleted}}},
			{ID: "/phase_x_1", Order: 1, Category: "/scaffold", Tasks: []Task{
				{ID: "/task_x_1_0", PhaseID: "/phase_x_1", Description: "Create 02-CURRENT-STATE.md shipped file-by-file", Status: TaskPending},
				{ID: "/task_x_1_1", PhaseID: "/phase_x_1", Description: "Create IMPLEMENTED_SPEC.md shipped authoritative record", Status: TaskPending},
			}},
		},
		TotalPhases: 2,
		TotalTasks:  3,
	}
	r := NewReplanner(&MockKernel{}, &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return `{"tasks": [
				{"task_id": "task_x_1_0", "description": "Write the current-state layer from the keepable inventory, every claim cited", "action": "update"},
				{"task_id": "/task_x_1_1", "description": "Write the authoritative shipped record; it wins on disagreement", "action": "update"}
			], "summary": "sharpened"}`, nil
		},
	}, "")
	if err := r.RefineNextPhase(context.Background(), campaign, &campaign.Phases[0], nil); err != nil {
		t.Fatalf("RefineNextPhase: %v", err)
	}
	tasks := campaign.Phases[1].Tasks
	if len(tasks) != 2 {
		t.Fatalf("the phase held 2 tasks and holds %d after a refinement that only restated them: %+v", len(tasks), tasks)
	}
	if tasks[0].ID != "/task_x_1_0" || tasks[0].Description != "Write the current-state layer from the keepable inventory, every claim cited" {
		t.Fatalf("the slashless update did not reach its task: %+v", tasks[0])
	}
	if campaign.TotalTasks != 3 {
		t.Fatalf("TotalTasks = %d, want 3: a restatement is not new work", campaign.TotalTasks)
	}
}

// A remove must find its task however the ID is spelled, and a genuinely new
// task whose ID arrives without a slash is stored with one.
func TestRefineNextPhase_RemoveAndAddSpellTaskIDsCanonically(t *testing.T) {
	campaign := &Campaign{
		ID:    "test-refine-canonical-id",
		Title: "Test",
		Goal:  "test",
		Phases: []Phase{
			{ID: "/phase_y_0", Order: 0, Tasks: []Task{{ID: "/task_y_0_0", PhaseID: "/phase_y_0", Description: "inventory", Status: TaskCompleted}}},
			{ID: "/phase_y_1", Order: 1, Category: "/scaffold", Tasks: []Task{
				{ID: "/task_y_1_0", PhaseID: "/phase_y_1", Description: "Draft the vision document", Status: TaskPending},
			}},
		},
		TotalPhases: 2,
		TotalTasks:  2,
	}
	r := NewReplanner(&MockKernel{}, &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return `{"tasks": [
				{"task_id": "task_y_1_0", "action": "remove"},
				{"task_id": "task_y_1_9", "description": "Record the risk register", "type": "/document", "action": "add", "write_set": ["docs/RISK-REGISTER.md"]}
			], "summary": "swapped"}`, nil
		},
	}, "")
	if err := r.RefineNextPhase(context.Background(), campaign, &campaign.Phases[0], nil); err != nil {
		t.Fatalf("RefineNextPhase: %v", err)
	}
	tasks := campaign.Phases[1].Tasks
	if len(tasks) != 1 || tasks[0].ID != "/task_y_1_9" {
		t.Fatalf("want exactly the added task, stored as /task_y_1_9; got %+v", tasks)
	}
}
