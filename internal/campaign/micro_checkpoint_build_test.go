package campaign

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/observation"
	"codenerd/internal/tactile"
)

// buildRecorder is the orchestrator's command executor for the test: it
// records every command and fails a build, as a 20-second limit or a missing
// build environment did.
type buildRecorder struct {
	mockTactileExecutor
	mu   sync.Mutex
	cmds []string
}

func (b *buildRecorder) Execute(_ context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	line := cmd.Binary + " " + strings.Join(cmd.Arguments, " ")
	b.cmds = append(b.cmds, line)
	if strings.HasPrefix(line, "go build") {
		return &tactile.ExecutionResult{ExitCode: 1, Stderr: "signal: killed (the command's 20s limit)"}, nil
	}
	return &tactile.ExecutionResult{Success: true}, nil
}

// Ladder L4: a task's turn built the tree under its own gate and ended /done;
// the campaign must not build it again and fail the task on that build. The
// micro-checkpoint's own `go build ./...` -- 20 seconds, the executor's
// environment, a tree that may hold a sibling's half-finished edits -- turned
// such a task into a failure and rolled its change back.
func TestRunPhase_ADoneTurnIsNotFailedByASecondBuild(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("the shipped corpus must load: %v", err)
	}
	workspace := t.TempDir()
	const target = "widget/widget.go"
	path := filepath.Join(workspace, filepath.FromSlash(target))
	const before = "package widget\n"
	const after = "package widget\n\nfunc Name() string { return \"widget\" }\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	for p, content := range map[string]string{
		filepath.Join(workspace, "go.mod"): "module example.com/app\n\ngo 1.22\n",
		path:                               before,
	} {
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	turns := &scriptedTurns{turn: func(int, string) (observation.Return, error) {
		if err := os.WriteFile(path, []byte(after), 0o644); err != nil {
			t.Error(err)
		}
		return observation.Return{Output: "Added Name.", Outcome: "/done",
			Writes: []observation.FileWrite{{Path: path, Before: known(before), After: known(after)}}}, nil
	}}
	builds := &buildRecorder{}
	c := &Campaign{
		ID: "/campaign_one_build", Type: CampaignTypeFeature, Title: "one build", Goal: "add Name",
		Status: StatusActive, CreatedAt: time.Now().UTC(), TotalPhases: 1, TotalTasks: 1,
		Phases: []Phase{{
			ID: "/phase_add", CampaignID: "/campaign_one_build", Name: "add", Status: PhaseInProgress,
			Category: "/implementation", EstimatedComplexity: "/low",
			Objectives: []PhaseObjective{{Type: ObjectiveModify, Description: "Name", VerificationMethod: VerifyNone}},
			Tasks: []Task{{
				ID: "/task_add", PhaseID: "/phase_add", Description: "add Name to the widget",
				Status: TaskPending, Type: TaskTypeFileModify, Priority: PriorityNormal,
				WriteSet: []string{target},
			}},
		}},
	}
	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace: workspace, Kernel: kernel, LLMClient: &MockLLMClient{}, TaskExecutor: turns,
		Executor: builds, VirtualStore: &core.VirtualStore{},
		MaxRetries: 2, RetryBackoffBase: time.Millisecond, RetryBackoffMax: time.Millisecond,
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

	if n := len(turns.calls()); n != 1 {
		t.Errorf("the coder ran %d time(s), want once", n)
	}
	if got := orch.campaign.Phases[0].Tasks[0]; got.Status != TaskCompleted {
		t.Errorf("task %s is %s (%s), want completed", got.ID, got.Status, got.LastError)
	}
	if data, _ := os.ReadFile(path); string(data) != after {
		t.Errorf("%s = %q, want the turn's change kept", target, data)
	}
	builds.mu.Lock()
	defer builds.mu.Unlock()
	for _, cmd := range builds.cmds {
		if strings.HasPrefix(cmd, "go build") {
			t.Errorf("the campaign built the tree again after a /done turn: %q", cmd)
		}
	}
}
