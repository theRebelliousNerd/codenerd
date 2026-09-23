package campaign

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/observation"
	"codenerd/internal/tactile"
)

// Ladder C2: a /file_modify whose planned target does not exist is a
// modification whose target the plan guessed. It was retyped to /file_create
// at plan time, and R1-2's task then wrote a new file at the guessed path
// instead of changing the command where it lives. It stays a modification,
// and what satisfies it is a change to existing code -- wherever that code is.
func TestValidateFileModifyOutcome_AnAbsentTargetIsSatisfiedByChangingExistingCode(t *testing.T) {
	dir := t.TempDir()
	guessed := filepath.Join(dir, "internal", "cli", "check-mangle.go")
	existing := filepath.Join(dir, "cmd", "nerd", "cmd_mangle_check.go")
	declared := filepath.Join(dir, "internal", "checker.go")
	for _, p := range []string{guessed, existing, declared} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, content string) {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	task := &Task{ID: "/task_fix", Type: TaskTypeFileModify}
	nothingDeclaredExisted := taskExecutionSnapshot{fileMutations: []fileMutationSnapshot{{Path: guessed}}}

	write(existing, "package main // fixed\n")
	changedExisting := []observation.FileWrite{{Path: existing, Before: known("package main\n"), After: known("package main // fixed\n")}}
	if err := validateFileModifyOutcome(task, nothingDeclaredExisted, changedExisting); err != nil {
		t.Errorf("a change to existing code, the planned target absent: %v", err)
	}

	write(guessed, "package cli\n")
	createdTheGuess := []observation.FileWrite{{Path: guessed, Before: absent, After: known("package cli\n")}}
	err := validateFileModifyOutcome(task, nothingDeclaredExisted, createdTheGuess)
	if err == nil || !strings.Contains(err.Error(), "creating the planned file does not satisfy a modification") {
		t.Errorf("creating the guessed target = %v, want it refused as a modification that changed nothing", err)
	}

	write(existing, "package main\n")
	putBack := []observation.FileWrite{
		{Path: existing, Before: known("package main\n"), After: known("package main // tried\n")},
		{Path: existing, Before: known("package main // tried\n"), After: known("package main\n")},
	}
	if err := validateFileModifyOutcome(task, nothingDeclaredExisted, putBack); err == nil {
		t.Errorf("a file written and put back as it was counted as a modification")
	}

	// A write set that named existing code keeps its contract: the change
	// lands there, not elsewhere.
	write(declared, "package internal\n")
	declaredExisting := taskExecutionSnapshot{fileMutations: []fileMutationSnapshot{{Path: declared, Exists: true, Content: []byte("package internal\n")}}}
	write(existing, "package main // fixed\n")
	err = validateFileModifyOutcome(task, declaredExisting, changedExisting)
	if err == nil || !strings.Contains(err.Error(), "in its declared write set") {
		t.Errorf("a change outside a write set of existing files = %v, want it refused", err)
	}
}

// Through the scheduler: the attempt is told its planned target does not
// exist; an attempt that creates it anyway is refused and undone, the retry
// reads why, and a retry that changes the existing code completes the task.
func TestRunPhase_AModificationWithAnAbsentTargetLandsInTheExistingCode(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("the shipped corpus must load: %v", err)
	}
	workspace := t.TempDir()
	const planned = "internal/cli/check-mangle.go"
	guessed := filepath.Join(workspace, filepath.FromSlash(planned))
	existing := filepath.Join(workspace, "cmd", "nerd", "cmd_mangle_check.go")
	const before = "package main\n\nfunc checkFile() {}\n"
	const after = "package main\n\nfunc checkFile() { preloadSharedSchemas() }\n"
	if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}

	turns := &scriptedTurns{turn: func(n int, _ string) (observation.Return, error) {
		if n == 1 {
			const guess = "package cli\n"
			if err := os.MkdirAll(filepath.Dir(guessed), 0o755); err != nil {
				t.Error(err)
			}
			if err := os.WriteFile(guessed, []byte(guess), 0o644); err != nil {
				t.Error(err)
			}
			return observation.Return{Output: "Created the file.", Outcome: "/done",
				Writes: []observation.FileWrite{{Path: guessed, Before: absent, After: known(guess)}}}, nil
		}
		if err := os.WriteFile(existing, []byte(after), 0o644); err != nil {
			t.Error(err)
		}
		return observation.Return{Output: "Changed checkFile.", Outcome: "/done",
			Writes: []observation.FileWrite{{Path: existing, Before: known(before), After: known(after)}}}, nil
	}}
	c := &Campaign{
		ID: "/campaign_absent", Type: CampaignTypeFeature, Title: "absent target", Goal: "fix check-mangle",
		Status: StatusActive, CreatedAt: time.Now().UTC(), TotalPhases: 1, TotalTasks: 1,
		Phases: []Phase{{
			ID: "/phase_fix", CampaignID: "/campaign_absent", Name: "fix", Status: PhaseInProgress,
			Category: "/implementation", EstimatedComplexity: "/low",
			Objectives: []PhaseObjective{{Type: ObjectiveModify, Description: "check-mangle", VerificationMethod: VerifyNone}},
			Tasks: []Task{{
				ID: "/task_fix", PhaseID: "/phase_fix", Description: "fix check-mangle's shared schemas",
				Status: TaskPending, Type: TaskTypeFileModify, Priority: PriorityNormal,
				WriteSet: []string{planned},
			}},
		}},
	}
	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace: workspace, Kernel: kernel, LLMClient: &MockLLMClient{}, TaskExecutor: turns,
		Executor: tactile.NewDirectExecutor(), VirtualStore: &core.VirtualStore{},
		Campaign: testCampaignConfig(fastRetries(3)),
	})
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	orch.replanner = nil
	if err := orch.SetCampaign(c); err != nil {
		t.Fatalf("SetCampaign: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := orch.runPhase(ctx, &orch.campaign.Phases[0]); err != nil && ctx.Err() == nil {
		t.Logf("runPhase returned: %v", err)
	}

	calls := turns.calls()
	if len(calls) != 2 {
		t.Fatalf("the coder ran %d time(s), want 2 (the guess refused, then the fix)", len(calls))
	}
	if !strings.Contains(calls[0], "PLANNED TARGET DOES NOT EXIST") || !strings.Contains(calls[0], planned) {
		t.Errorf("the first attempt was not told its planned target is absent:\n%s", calls[0])
	}
	if !strings.Contains(calls[1], "creating the planned file does not satisfy a modification") {
		t.Errorf("the retry was not told why the first attempt was refused:\n%s", calls[1])
	}
	if _, err := os.Stat(guessed); !os.IsNotExist(err) {
		t.Errorf("the refused attempt's guessed file survived: %v", err)
	}
	if data, _ := os.ReadFile(existing); string(data) != after {
		t.Errorf("%s = %q, want the retry's change kept", existing, data)
	}
	if got := orch.campaign.Phases[0].Tasks[0]; got.Status != TaskCompleted || got.Type != TaskTypeFileModify {
		t.Errorf("task = %s %s, want a completed modification", got.Type, got.Status)
	}
}
