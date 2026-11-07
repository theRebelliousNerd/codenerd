package campaign

import (
	"context"
	"slices"
	"testing"
)

func refineWith(t *testing.T, campaign *Campaign, response string) {
	t.Helper()
	r := NewReplanner(&MockKernel{}, &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) { return response, nil },
	}, "")
	r.SetPromptProvider(testPromptProvider{})
	if err := r.RefineNextPhase(context.Background(), campaign, &campaign.Phases[0], nil); err != nil {
		t.Fatalf("RefineNextPhase failed: %v", err)
	}
}

func refineEdgesCampaign() *Campaign {
	return &Campaign{
		ID: "test-refine-edges", Title: "Test", Goal: "test",
		Phases: []Phase{
			{ID: "/phase_test_0", Order: 0, Tasks: []Task{{ID: "/t0", PhaseID: "/phase_test_0", Description: "prior", Status: TaskCompleted}}},
			{ID: "/phase_test_1", Order: 1, Category: "/scaffold", Tasks: []Task{
				{ID: "/t1", PhaseID: "/phase_test_1", Description: "Select the defects to fix", Status: TaskPending, Type: TaskTypeResearch},
			}},
		},
		TotalPhases: 2, TotalTasks: 2,
	}
}

// campaign-refinement-dependency-kind #1: a refined task's depends_on names a
// pending predecessor. It must wait for it (DependsOn), not only read its
// return (ContextFrom); context_from alone stays context.
func TestRefineNextPhase_DependsOnBecomesAnOrderingEdge(t *testing.T) {
	for _, action := range []string{"add", "update"} {
		t.Run(action, func(t *testing.T) {
			c := refineEdgesCampaign()
			refineWith(t, c, `{"tasks": [{"task_id": "/t_new", "description": "Fix the selected defects", "type": "/research", "action": "`+action+`", "depends_on": ["/t1"]},
				{"task_id": "/t_ctx", "description": "Summarize the defect selection", "type": "/research", "action": "`+action+`", "context_from": ["/t1"]}], "summary": "ok"}`)
			byID := map[string]Task{}
			for _, task := range c.Phases[1].Tasks {
				byID[task.ID] = task
			}
			fix, ok := byID["/t_new"]
			if !ok {
				t.Fatalf("refined task missing: %+v", c.Phases[1].Tasks)
			}
			if !slices.Contains(fix.DependsOn, "/t1") || !slices.Contains(fix.ContextFrom, "/t1") {
				t.Fatalf("depends_on did not become an ordering edge: DependsOn=%v ContextFrom=%v", fix.DependsOn, fix.ContextFrom)
			}
			sum := byID["/t_ctx"]
			if len(sum.DependsOn) != 0 || !slices.Contains(sum.ContextFrom, "/t1") {
				t.Fatalf("context_from must stay context only: DependsOn=%v ContextFrom=%v", sum.DependsOn, sum.ContextFrom)
			}
		})
	}
}

// campaign-refinement-dependency-kind #2: the matched-update path applied
// description, targets, type and priority, and dropped the dependency fields.
func TestRefineNextPhase_MatchedUpdateAppliesDependencies(t *testing.T) {
	c := refineEdgesCampaign()
	c.Phases[1].Tasks = append(c.Phases[1].Tasks, Task{ID: "/t2", PhaseID: "/phase_test_1", Description: "Apply the fixes", Status: TaskPending, Type: TaskTypeResearch})
	refineWith(t, c, `{"tasks": [{"task_id": "/t2", "action": "update", "depends_on": ["/t1"], "depends_on_self": [], "dependencies": ["/t2"]}], "summary": "ok"}`)
	got := c.Phases[1].Tasks[1]
	if !slices.Contains(got.DependsOn, "/t1") {
		t.Fatalf("matched update dropped depends_on: %+v", got)
	}
	if slices.Contains(got.DependsOn, "/t2") {
		t.Fatalf("a task must not depend on itself: %v", got.DependsOn)
	}
}

// campaign-refinement-scope-deduplication #1: a restated description over
// files disjoint from the matched task's targets is a second piece of work.
func TestRefineNextPhase_NearDuplicateWithDisjointTargetsIsKept(t *testing.T) {
	c := refineEdgesCampaign()
	c.Phases[1].Tasks = []Task{{ID: "/t1", PhaseID: "/phase_test_1", Description: "Fix the nil guard", Status: TaskPending, Type: TaskTypeFileModify, WriteSet: []string{"internal/a/a.go"}}}
	refineWith(t, c, `{"tasks": [
		{"task_id": "/t_b", "description": "Fix the nil guard in internal/b/b.go", "type": "/file_modify", "action": "add", "write_set": ["internal/b/b.go"]},
		{"task_id": "/t_a", "description": "Fix the nil guard in internal/a/a.go", "type": "/file_modify", "action": "add", "write_set": ["internal/a/a.go"]}
	], "summary": "ok"}`)
	var ids []string
	for _, task := range c.Phases[1].Tasks {
		ids = append(ids, task.ID)
	}
	if !slices.Contains(ids, "/t_b") {
		t.Fatalf("restatement over disjoint targets was dropped: %v", ids)
	}
	if slices.Contains(ids, "/t_a") {
		t.Fatalf("restatement over the same target must still be dropped: %v", ids)
	}
}
