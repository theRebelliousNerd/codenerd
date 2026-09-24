package campaign

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/session"
	"codenerd/internal/tactile"
	"codenerd/internal/types"
)

// A quiet loop wakes only when a withheld retry comes due; every other state
// keeps polling.
func TestEligibilityWake_AQuietLoopWaitsForAResultOrADueRetry(t *testing.T) {
	o := &Orchestrator{}
	phase := &Phase{Tasks: []Task{
		{ID: "/t_done", Status: TaskCompleted, NextRetryAt: time.Now().Add(50 * time.Millisecond)},
		{ID: "/t_overdue", Status: TaskPending, NextRetryAt: time.Now().Add(-time.Minute)},
	}}

	if o.eligibilityWake(phase, true) != nil {
		t.Fatal("a quiet loop with no retry ahead of it woke on a timer: nothing but a result can change what runs")
	}
	select {
	case <-o.eligibilityWake(phase, false):
	case <-time.After(5 * time.Second):
		t.Fatal("a loop that is not quiet stopped polling")
	}

	phase.Tasks = append(phase.Tasks, Task{ID: "/t_backoff", Status: TaskPending, NextRetryAt: time.Now().Add(300 * time.Millisecond)})
	start := time.Now()
	wake := o.eligibilityWake(phase, true)
	if wake == nil {
		t.Fatal("a quiet loop with a retry coming due never wakes for it")
	}
	select {
	case <-wake:
		if waited := time.Since(start); waited < 200*time.Millisecond {
			t.Fatalf("woke after %v, before the retry came due", waited)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the wake for a due retry never fired")
	}
}

// countingKernel counts the loop's eligibility questions.
type countingKernel struct {
	core.Kernel
	eligible atomic.Int32
}

func (k *countingKernel) Query(predicate string) ([]types.Fact, error) {
	if predicate == "eligible_task" {
		k.eligible.Add(1)
	}
	return k.Kernel.Query(predicate)
}

// While one task runs and the next depends on it, only its result can make
// anything eligible, so the loop asks once per event rather than five times a
// second. Measured 2026-09-24: 2 eligible_task queries across this phase with
// the quiet wait, 9-10 with the 200 ms poll it replaced.
func TestRunPhase_WhileTheOnlyRunnableTaskRunsTheLoopDoesNotPoll(t *testing.T) {
	workspace := t.TempDir()
	real, err := core.NewRealKernelWithWorkspace(workspace)
	if err != nil {
		t.Fatal(err)
	}
	kernel := &countingKernel{Kernel: real}
	write := func(name string) error {
		return os.WriteFile(filepath.Join(workspace, name), []byte("# "+name+"\n"), 0o644)
	}
	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace:    workspace,
		Kernel:       kernel,
		LLMClient:    &MockLLMClient{},
		Executor:     tactile.NewDirectExecutor(),
		VirtualStore: &core.VirtualStore{},
		TaskExecutor: &MockTaskExecutor{ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
			if strings.Contains(req.Task, "a.md") {
				time.Sleep(1500 * time.Millisecond)
				return "wrote a.md", write("a.md")
			}
			return "wrote b.md", write("b.md")
		}},
		Campaign: testCampaignConfig(nil),
	})
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	orch.publishPolicyParams()
	orch.campaign = &Campaign{
		ID:          "/campaign_quiet",
		Title:       "quiet loop",
		Status:      StatusActive,
		TotalPhases: 1,
		TotalTasks:  2,
		Phases: []Phase{{
			ID:         "/phase_quiet_0",
			CampaignID: "/campaign_quiet",
			Name:       "two tasks in a row",
			Status:     PhaseInProgress,
			Tasks: []Task{
				{ID: "/task_quiet_0_0", PhaseID: "/phase_quiet_0", Description: "Write a.md", Status: TaskPending, Type: TaskTypeFileCreate, Priority: PriorityNormal, WriteSet: []string{"a.md"}},
				{ID: "/task_quiet_0_1", PhaseID: "/phase_quiet_0", Description: "Write b.md", Status: TaskPending, Type: TaskTypeFileCreate, Priority: PriorityNormal, WriteSet: []string{"b.md"}, DependsOn: []string{"/task_quiet_0_0"}},
			},
		}},
	}
	if err := real.LoadFacts(orch.campaign.ToFacts()); err != nil {
		t.Fatalf("load campaign facts: %v", err)
	}

	if err := orch.runPhase(context.Background(), &orch.campaign.Phases[0]); err != nil {
		t.Fatalf("runPhase: %v", err)
	}
	for _, task := range orch.campaign.Phases[0].Tasks {
		if task.Status != TaskCompleted {
			t.Fatalf("task %s ended %s; both tasks must still run in order", task.ID, task.Status)
		}
	}
	t.Logf("eligible_task queries across the phase: %d", kernel.eligible.Load())
	if got := kernel.eligible.Load(); got > 4 {
		t.Fatalf("the loop asked eligible_task %d times across one 1.5 s task and its successor: it is polling while quiet", got)
	}
}
