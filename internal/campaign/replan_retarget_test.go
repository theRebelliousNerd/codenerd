package campaign

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Ladder C1 (R1-2's campaign): phase 2 aimed at internal/cli/check-mangle.go,
// a path that does not exist; phase 0's research had found the command in
// cmd/nerd/cmd_mangle_check.go. The rolling-wave refinement saw neither --
// no result, no task ID, no write set -- so it could not retarget the task,
// and an update to an ID it had never been shown was added as a new task and
// dropped as a near-duplicate. It is shown all three now, and a retarget by ID
// lands on the task.
func TestRefineNextPhase_RetargetsATaskFromWhatTheResearchFound(t *testing.T) {
	ws := t.TempDir()
	located := filepath.Join(ws, "cmd", "nerd", "cmd_mangle_check.go")
	if err := os.MkdirAll(filepath.Dir(located), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(located, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	const finding = "The check-mangle command is implemented in cmd/nerd/cmd_mangle_check.go (checkFile, preloadSharedSchemas)."
	campaign := &Campaign{
		ID: "c-retarget", Title: "retarget", Goal: "fix check-mangle",
		Phases: []Phase{
			{ID: "/p0", Order: 0, Tasks: []Task{{ID: "/task_research", PhaseID: "/p0", Description: "find the check-mangle command", Type: TaskTypeResearch, Status: TaskCompleted}}},
			{ID: "/p1", Order: 1, Category: "/implementation", Tasks: []Task{{
				ID: "/task_fix", PhaseID: "/p1", Description: "fix check-mangle's context", Type: TaskTypeFileModify,
				Status: TaskPending, WriteSet: []string{"internal/cli/check-mangle.go"},
			}}},
		},
		TotalPhases: 2, TotalTasks: 2,
	}
	var prompt string
	r := NewReplanner(&MockKernel{}, &MockLLMClient{
		CompleteFunc: func(_ context.Context, p string) (string, error) {
			prompt = p
			return `{"tasks": [{"task_id": "/task_fix", "action": "update", "write_set": ["cmd/nerd/cmd_mangle_check.go"]}], "summary": "retargeted"}`, nil
		},
	}, ws)
	results := func(id string) (string, bool) {
		if id == "/task_research" {
			return finding, true
		}
		return "", false
	}

	if err := r.RefineNextPhase(context.Background(), campaign, &campaign.Phases[0], results); err != nil {
		t.Fatalf("RefineNextPhase: %v", err)
	}

	for _, want := range []string{finding, "[/task_fix]", "internal/cli/check-mangle.go (ABSENT)", `"write_set"`} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the refinement was not shown %q", want)
		}
	}
	tasks := campaign.Phases[1].Tasks
	if len(tasks) != 1 {
		t.Fatalf("phase 1 has %d tasks, want the one task retargeted: %+v", len(tasks), tasks)
	}
	// Refined write sets are stored as the lock manager's absolute keys.
	names := func(suffix string) func(string) bool {
		return func(p string) bool {
			return strings.HasSuffix(strings.ToLower(filepath.ToSlash(p)), suffix)
		}
	}
	if got := tasks[0].DeterministicWriteSet(); !slices.ContainsFunc(got, names("cmd/nerd/cmd_mangle_check.go")) || slices.ContainsFunc(got, names("internal/cli/check-mangle.go")) {
		t.Fatalf("write set = %v, want the located path in place of the guessed one", got)
	}
	if tasks[0].Type != TaskTypeFileModify {
		t.Fatalf("type = %s, want the modification kept", tasks[0].Type)
	}
}
