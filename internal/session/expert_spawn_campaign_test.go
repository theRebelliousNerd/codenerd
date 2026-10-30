package session

import (
	"context"
	"testing"
	"time"
)

// countingConsultant counts consultations per agent.
type countingConsultant struct{ recordingConsultant }

func (c *countingConsultant) count() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[string]int{}
	for agent := range c.asked {
		out[agent]++
	}
	return out
}

// countingSpawns wraps the consultant so repeated calls are counted, not
// overwritten.
type countingSpawns struct {
	inner ExpertConsultant
	calls map[string]int
}

func (c *countingSpawns) SpawnConsultation(ctx context.Context, agent, task string) (string, error) {
	c.calls[agent]++
	return c.inner.SpawnConsultation(ctx, agent, task)
}

// A campaign task reaches the executor the way spawnTask sends it: the task's
// verb (/fix), its primary file as Target, through JITExecutor.ExecuteObserved. The
// shard agent the kernel spawns for that file is consulted exactly once per
// task -- here, by the executor. The campaign itself consults no one
// (internal/campaign/orchestrator_experts.go records the selection and
// nothing more; its campaign-level consultation was the second path, removed
// 2026-10-29).
func TestCampaignShapedTaskConsultsItsExpertOncePerTask(t *testing.T) {
	k := spawnRosterKernel(t)
	client := &systemCapturingProvider{MockLLMClient: &MockLLMClient{}}
	executor := NewExecutor(k, &testExecutiveStore{}, client, &MockJITCompiler{}, journeyConfigFactory(), journeyIntent("/explain", "/query"))
	cfg := journeyConfig(t)
	cfg.WorkspaceRoot = t.TempDir()
	executor.SetConfig(cfg)
	spawns := &countingSpawns{inner: &countingConsultant{}, calls: map[string]int{}}
	executor.SetExpertConsultant(spawns)
	jit := NewJITExecutor(executor, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	// Two tasks on the same file with different requests: each is its own
	// question (consult_route reuses a card only for the same request).
	tasks := []string{"fix the chat view so the status line renders", "fix the chat view so the footer wraps"}
	for i := range 2 {
		if _, err := jit.ExecuteObserved(ctx, TaskRequest{
			IntentVerb: "/fix",
			Task:       tasks[i],
			Target:     "cmd/nerd/chat/view.go",
		}); err != nil {
			t.Logf("task %d error (not under test): %v", i, err)
		}
		if got := spawns.calls["bubbleteaexpert"]; got != i+1 {
			t.Fatalf("after task %d the Bubble Tea expert was consulted %d time(s), want %d (once per task)", i+1, got, i+1)
		}
	}
}
