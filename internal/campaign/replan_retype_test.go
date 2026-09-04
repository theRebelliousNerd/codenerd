package campaign

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestRetypePathlessDocumentToResearch pins the shared pathless-file-task
// defense directly: a /document task with no artifact path, no write-set
// entry, and no extractable description path (campaign 5a2f4c8d's
// "/task_5a2f4c8d_4_2" shape) becomes /research with the recorded reason.
func TestRetypePathlessDocumentToResearch(t *testing.T) {
	workspace := t.TempDir()
	task := &Task{
		ID:          "/task_5a2f4c8d_4_2",
		Description: "Assemble short ranked risk report from Phase 3 correctness and safety findings",
		Type:        TaskTypeDocument,
	}
	changed, reason := applyTaskTypeDefenses(workspace, task)
	if !changed {
		t.Fatalf("expected pathless /document to be retyped, type now %s", task.Type)
	}
	if task.Type != TaskTypeResearch {
		t.Fatalf("type = %s, want %s", task.Type, TaskTypeResearch)
	}
	if reason != "no artifact, write set, or extractable path" {
		t.Fatalf("reason = %q, want %q", reason, "no artifact, write set, or extractable path")
	}
}

// TestRetypeFileCreateFollowsWriteSetDecision pins that the shared defense
// keeps reconcileTaskTypeWithWriteSet's verdict: a /file_create whose
// write-set path already exists on disk becomes /file_modify, and the
// pathless retype (which needs an empty write set) does not override it.
func TestRetypeFileCreateFollowsWriteSetDecision(t *testing.T) {
	workspace := t.TempDir()
	existing := filepath.Join(workspace, "docs", "report.md")
	if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
		t.Fatalf("setup: mkdir %s: %v", filepath.Dir(existing), err)
	}
	if err := os.WriteFile(existing, []byte("# report\n"), 0o644); err != nil {
		t.Fatalf("setup: write %s: %v", existing, err)
	}
	task := &Task{
		ID:          "/task_retype_keep",
		Description: "Write the risk report",
		Type:        TaskTypeFileCreate,
		Artifacts:   []TaskArtifact{{Type: "/source_file", Path: "docs/report.md"}},
		WriteSet:    []string{existing},
	}
	changed, reason := applyTaskTypeDefenses(workspace, task)
	if !changed {
		t.Fatalf("expected /file_create with existing path to be retyped, type now %s", task.Type)
	}
	if task.Type != TaskTypeFileModify {
		t.Fatalf("type = %s, want %s", task.Type, TaskTypeFileModify)
	}
	if reason != "every write-set path already exists" {
		t.Fatalf("reason = %q, want %q", reason, "every write-set path already exists")
	}
}

// TestReplanRefineRetypesPathlessDocument drives the rolling-wave path
// end to end: a refinement that adds a pathless /document task must land
// it as /research instead of a file task with nowhere to write.
func TestReplanRefineRetypesPathlessDocument(t *testing.T) {
	campaign := &Campaign{
		ID:    "test-replan-retype",
		Title: "Test",
		Goal:  "test",
		Phases: []Phase{
			{ID: "/phase_test_0", Order: 0, Tasks: []Task{{ID: "/t0", PhaseID: "/phase_test_0", Description: "prior work", Status: TaskCompleted}}},
			{ID: "/phase_test_1", Order: 1, Category: "/scaffold", Tasks: []Task{}},
		},
		TotalPhases: 2,
		TotalTasks:  1,
	}
	r := NewReplanner(&MockKernel{}, &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return `{"tasks": [{"task_id": "", "description": "Assemble short ranked risk report from Phase 3 correctness and safety findings", "type": "/document", "priority": "/high", "action": "add"}], "summary": "refine"}`, nil
		},
	}, t.TempDir())
	if err := r.RefineNextPhase(context.Background(), campaign, &campaign.Phases[0]); err != nil {
		t.Fatalf("RefineNextPhase failed: %v", err)
	}
	if got := len(campaign.Phases[1].Tasks); got != 1 {
		t.Fatalf("next phase task count = %d, want 1; tasks=%v", got, campaign.Phases[1].Tasks)
	}
	if got := campaign.Phases[1].Tasks[0].Type; got != TaskTypeResearch {
		t.Fatalf("re-plan /document type = %s, want %s", got, TaskTypeResearch)
	}
}

// TestReplanDedupeDropsSuffixedRestatement pins the rolling-wave
// near-duplicate guard: "Research correctness in internal/retrieval" is a
// restatement of the existing "Research correctness" once the trailing
// " in <path>" suffix is stripped, so it must be skipped without inflating
// the phase or TotalTasks.
func TestReplanDedupeDropsSuffixedRestatement(t *testing.T) {
	campaign := &Campaign{
		ID:    "test-replan-neardup",
		Title: "Test",
		Goal:  "test",
		Phases: []Phase{
			{ID: "/phase_test_0", Order: 0, Tasks: []Task{{ID: "/t0", PhaseID: "/phase_test_0", Description: "prior work", Status: TaskCompleted}}},
			{ID: "/phase_test_1", Order: 1, Category: "/scaffold", Tasks: []Task{
				{ID: "/t1", PhaseID: "/phase_test_1", Description: "Research correctness", Status: TaskPending, Type: TaskTypeResearch},
			}},
		},
		TotalPhases: 2,
		TotalTasks:  2,
	}
	r := NewReplanner(&MockKernel{}, &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return `{"tasks": [{"task_id": "", "description": "Research correctness in internal/retrieval", "type": "/research", "priority": "/high", "action": "add"}], "summary": "refine"}`, nil
		},
	}, t.TempDir())
	if err := r.RefineNextPhase(context.Background(), campaign, &campaign.Phases[0]); err != nil {
		t.Fatalf("RefineNextPhase failed: %v", err)
	}
	if got := len(campaign.Phases[1].Tasks); got != 1 {
		t.Fatalf("next phase task count = %d, want 1 (restatement skipped); tasks=%v", got, campaign.Phases[1].Tasks)
	}
	if campaign.TotalTasks != 2 {
		t.Fatalf("TotalTasks = %d, want 2 (restatement must not inflate totals)", campaign.TotalTasks)
	}
	if got := campaign.Phases[1].Tasks[0].Description; got != "Research correctness" {
		t.Fatalf("original task mutated: got %q", got)
	}
}

// TestReplanDedupeKeepsDistinctTasks guards the other direction: stripping
// the " in <path>" suffix must not swallow a genuinely new task whose base
// description differs from everything already in the phase.
func TestReplanDedupeKeepsDistinctTasks(t *testing.T) {
	campaign := &Campaign{
		ID:    "test-replan-neardup-keep",
		Title: "Test",
		Goal:  "test",
		Phases: []Phase{
			{ID: "/phase_test_0", Order: 0, Tasks: []Task{{ID: "/t0", PhaseID: "/phase_test_0", Description: "prior work", Status: TaskCompleted}}},
			{ID: "/phase_test_1", Order: 1, Category: "/scaffold", Tasks: []Task{
				{ID: "/t1", PhaseID: "/phase_test_1", Description: "Research correctness", Status: TaskPending, Type: TaskTypeResearch},
			}},
		},
		TotalPhases: 2,
		TotalTasks:  2,
	}
	r := NewReplanner(&MockKernel{}, &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return `{"tasks": [{"task_id": "", "description": "Draft release notes in docs/notes.md", "type": "/document", "priority": "/high", "action": "add"}], "summary": "refine"}`, nil
		},
	}, t.TempDir())
	if err := r.RefineNextPhase(context.Background(), campaign, &campaign.Phases[0]); err != nil {
		t.Fatalf("RefineNextPhase failed: %v", err)
	}
	if got := len(campaign.Phases[1].Tasks); got != 2 {
		t.Fatalf("next phase task count = %d, want 2 (distinct task kept); tasks=%v", got, campaign.Phases[1].Tasks)
	}
	if campaign.TotalTasks != 3 {
		t.Fatalf("TotalTasks = %d, want 3", campaign.TotalTasks)
	}
}

// TestDefenseDecomposerPlanTimeRetypeStillApplies pins that the extraction
// did not change plan-time behavior: buildCampaign still retypes a pathless
// /document task to /research, while a /document task carrying an artifact
// path keeps its type.
func TestDefenseDecomposerPlanTimeRetypeStillApplies(t *testing.T) {
	workspace := t.TempDir()
	d := NewDecomposer(&MockKernel{}, &MockLLMClient{}, workspace)
	campaign := d.buildCampaign("/campaign_retype_defense", DecomposeRequest{
		Goal:          "Stabilize campaign planning",
		CampaignType:  CampaignTypeCustom,
		ContextBudget: 1000,
	}, &RawPlan{
		Title:      "Plan",
		Confidence: 0.9,
		Phases: []RawPhase{{
			Name:        "Phase 1",
			Category:    "/scaffold",
			Description: "Create baseline",
			Tasks: []RawTask{
				{Description: "Assemble short ranked risk report from Phase 3 correctness and safety findings", Type: "/document"},
				{Description: "Write docs/report.md with the risk findings", Type: "/document", Artifacts: []string{"docs/report.md"}},
			},
		}},
	})
	tasks := campaign.Phases[0].Tasks
	if len(tasks) != 2 {
		t.Fatalf("task count = %d, want 2; tasks=%v", len(tasks), tasks)
	}
	if tasks[0].Type != TaskTypeResearch {
		t.Fatalf("pathless /document type = %s, want %s", tasks[0].Type, TaskTypeResearch)
	}
	if tasks[1].Type != TaskTypeDocument {
		t.Fatalf("pathed /document type = %s, want %s", tasks[1].Type, TaskTypeDocument)
	}
}
