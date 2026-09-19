package campaign

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/observation"
	"codenerd/internal/session"
	"codenerd/internal/tactile"
	"codenerd/internal/types"
)

// scriptedTurns is a task executor whose every call is one turn of the
// script: the turn's number, the input it was given, and what it returned.
// The string route returns the same turn's output, as JITExecutor's does --
// the verdict beside it is what that route drops.
type scriptedTurns struct {
	mu     sync.Mutex
	inputs []string
	turn   func(n int, input string) (observation.Return, error)
}

func (s *scriptedTurns) ExecuteObserved(_ context.Context, req session.TaskRequest) (observation.Return, error) {
	s.mu.Lock()
	s.inputs = append(s.inputs, req.Task)
	n := len(s.inputs)
	s.mu.Unlock()
	return s.turn(n, req.Task)
}

func (s *scriptedTurns) ExecuteObservedWithContext(ctx context.Context, req session.TaskRequest, _ *types.SessionContext, _ types.SpawnPriority) (observation.Return, error) {
	return s.ExecuteObserved(ctx, req)
}

func (s *scriptedTurns) Execute(ctx context.Context, req session.TaskRequest) (string, error) {
	ret, err := s.ExecuteObserved(ctx, req)
	return ret.Output, err
}

func (s *scriptedTurns) ExecuteWithContext(ctx context.Context, req session.TaskRequest, _ *types.SessionContext, _ types.SpawnPriority) (string, error) {
	return s.Execute(ctx, req)
}

func (s *scriptedTurns) ExecuteAsync(context.Context, session.TaskRequest) (string, error) {
	return "", errors.New("scriptedTurns runs synchronously")
}

func (s *scriptedTurns) GetResult(string) (string, bool, error) {
	return "", false, errors.New("scriptedTurns runs synchronously")
}

func (s *scriptedTurns) WaitForResult(context.Context, string) (string, error) {
	return "", errors.New("scriptedTurns runs synchronously")
}

func (s *scriptedTurns) calls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.inputs...)
}

// External audit F2 with ladder C3, through the scheduler on the shipped
// corpus: a /file_create whose coder turn wrote the file but ended
// /unverified -- no test beside the new code -- used to complete on its
// first attempt, because the file existed and the string route carried no
// verdict. The attempt fails now, and the retry is told why.
func TestRunPhase_AnUnverifiedTurnFailsItsAttemptAndTheRetryIsToldWhy(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("the shipped corpus must load: %v", err)
	}
	const target = "internal/widget/widget.go"
	c := &Campaign{
		ID: "/campaign_verdict", Type: CampaignTypeFeature, Title: "verdict", Goal: "read the verdict",
		Status: StatusActive, CreatedAt: time.Now().UTC(), TotalPhases: 1, TotalTasks: 1,
		Phases: []Phase{{
			ID: "/phase_verdict", CampaignID: "/campaign_verdict", Name: "build", Status: PhaseInProgress,
			Category: "/implementation", EstimatedComplexity: "/low",
			Objectives: []PhaseObjective{{Type: ObjectiveCreate, Description: "the widget", VerificationMethod: VerifyNone}},
			Tasks: []Task{{
				ID: "/task_widget", PhaseID: "/phase_verdict", Description: "create the widget",
				Status: TaskPending, Type: TaskTypeFileCreate, Priority: PriorityNormal,
				Artifacts: []TaskArtifact{{Type: "/source_file", Path: target}},
			}},
		}},
	}
	workspace := t.TempDir()
	write := func() {
		full := filepath.Join(workspace, filepath.FromSlash(target))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Error(err)
		}
		if err := os.WriteFile(full, []byte("package widget\n\nfunc Widget() int { return 1 }\n"), 0o644); err != nil {
			t.Error(err)
		}
	}
	turns := &scriptedTurns{turn: func(n int, _ string) (observation.Return, error) {
		write() // the coder wrote the file both times
		if n == 1 {
			return observation.Return{Output: "Wrote 1 file(s).", Outcome: "/unverified", Missing: []string{"/tests_not_written"}}, nil
		}
		return observation.Return{Output: "Wrote the widget and its test.", Outcome: "/done"}, nil
	}}
	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace:        workspace,
		Kernel:           kernel,
		LLMClient:        &MockLLMClient{},
		TaskExecutor:     turns,
		Executor:         tactile.NewDirectExecutor(),
		VirtualStore:     &core.VirtualStore{},
		MaxRetries:       2,
		RetryBackoffBase: time.Millisecond,
		RetryBackoffMax:  time.Millisecond,
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
		t.Fatalf("the coder ran %d time(s), want 2: an /unverified turn must not complete the task", len(calls))
	}
	if !strings.Contains(calls[1], "production code was written with no test beside it") {
		t.Fatalf("the retry's input does not say why the first attempt failed:\n%s", calls[1])
	}
	if got := orch.campaign.Phases[0].Tasks[0].Status; got != TaskCompleted {
		t.Fatalf("task status = %s after a /done retry, want %s", got, TaskCompleted)
	}
}
