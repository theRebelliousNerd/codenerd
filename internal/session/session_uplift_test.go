package session

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/observation"
	"codenerd/internal/types"
)

// A nil budget controller must deny extension, not panic: every other method
// on the controller is nil-safe, and the deny path is the fail-closed answer.
func TestToolBudgetController_NilDeniesWithoutPanic(t *testing.T) {
	var c *toolBudgetController
	decision := c.maybeExtend(true)
	if decision.Granted {
		t.Error("nil controller must not grant an extension")
	}
	if decision.Reason == "" {
		t.Error("nil controller must explain the denial")
	}
}

// A second Run while the agent is started must not start a second execution:
// two Run goroutines would share result, error and history with no
// coordination. The mock counts model invocations; an agent Run twice must
// cost exactly what an agent Run once costs.
func TestSubAgent_DoubleRunExecutesOnce(t *testing.T) {
	runAndCount := func(t *testing.T, runs int) (int64, string) {
		t.Helper()
		var calls atomic.Int64
		mockLLM := &MockLLMClient{
			CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
				calls.Add(1)
				return &types.LLMToolResponse{Text: "done"}, nil
			},
			CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
				calls.Add(1)
				return "done", nil
			},
		}
		agent := NewSubAgent(
			DefaultSubAgentConfig("double-run"),
			&MockKernel{},
			&MockVirtualStore{},
			mockLLM,
			&MockJITCompiler{},
			&MockConfigFactory{},
			&MockTransducer{},
		)
		for i := 0; i < runs; i++ {
			agent.Run(context.Background(), "task")
		}
		result, err := agent.Wait()
		if err != nil {
			t.Fatalf("agent failed: %v", err)
		}
		return calls.Load(), result
	}

	once, resultOnce := runAndCount(t, 1)
	twice, resultTwice := runAndCount(t, 2)
	if twice != once {
		t.Errorf("Run twice cost %d model calls, Run once cost %d; second start must be a no-op", twice, once)
	}
	if resultTwice != resultOnce {
		t.Errorf("double-run result %q differs from single-run %q", resultTwice, resultOnce)
	}
}

// SpawnConsultation implements shards.ConsultationSpawner, whose contract
// returns the specialist's ANSWER text: the ConsultationManager parses the
// return as the reply. Returning the async task ID matches the signature but
// corrupts every consultation, so the blocking answer is pinned here.
func TestJITExecutor_SpawnConsultationReturnsAnswer(t *testing.T) {
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return &types.LLMToolResponse{Text: "specialist says yes"}, nil
		},
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			return "specialist says yes", nil
		},
	}
	spawner := NewSpawner(
		&MockKernel{},
		&MockVirtualStore{},
		mockLLM,
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
		DefaultSpawnerConfig(),
	)
	jitExec := NewJITExecutor(createTestExecutor(t), spawner, &MockTransducer{})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	answer, err := jitExec.SpawnConsultation(ctx, "Reviewer", "is this sound?")
	if err != nil {
		t.Fatalf("SpawnConsultation failed: %v", err)
	}
	if answer != "specialist says yes" {
		t.Errorf("SpawnConsultation returned %q; want the specialist's answer text, not a task ID", answer)
	}
}

// Completed results are bounded FIFO: the cache must not retain every
// delegated response for the process lifetime, in-flight entries must never
// be evicted, and re-caching one task must not consume two slots.
func TestJITExecutor_CompletedResultsBounded(t *testing.T) {
	j := NewJITExecutor(nil, nil, nil)
	j.results["in-flight"] = &TaskResult{TaskID: "in-flight", Completed: false}
	for i := 0; i < maxCachedResults+50; i++ {
		j.cacheCompletedResult(fmt.Sprintf("task-%d", i), observation.Return{Output: "r"}, nil)
	}
	if got := len(j.results); got != maxCachedResults+1 {
		t.Fatalf("cache holds %d entries; want %d completed + 1 in-flight", got, maxCachedResults)
	}
	if _, ok := j.results["in-flight"]; !ok {
		t.Error("in-flight entry evicted; only completed entries may go")
	}
	if _, ok := j.results["task-0"]; ok {
		t.Error("oldest completed entry survived past the bound")
	}
	if _, ok := j.results[fmt.Sprintf("task-%d", maxCachedResults+49)]; !ok {
		t.Error("newest completed entry missing")
	}
	j.cacheCompletedResult("task-300", observation.Return{Output: "r2"}, nil)
	j.cacheCompletedResult("task-300", observation.Return{Output: "r3"}, nil)
	if got := len(j.completedOrder); got != maxCachedResults {
		t.Errorf("re-caching duplicated eviction slots: order len %d, want %d", got, maxCachedResults)
	}
}

// Spawner.Remove releases terminal agents and keeps live ones: dropping a
// running agent would orphan its execution under a still-awaited ID.
func TestSpawner_RemoveOnlyTerminal(t *testing.T) {
	s := NewSpawner(nil, nil, nil, nil, nil, nil, DefaultSpawnerConfig())
	mkAgent := func(id string, st SubAgentState) *SubAgent {
		a := NewSubAgent(SubAgentConfig{ID: id, Name: id}, nil, nil, nil, nil, nil, nil)
		atomic.StoreInt32(&a.state, int32(st))
		s.subagents[id] = a
		return a
	}
	mkAgent("done", SubAgentStateCompleted)
	mkAgent("failed", SubAgentStateFailed)
	mkAgent("running", SubAgentStateRunning)

	if !s.Remove("done") || !s.Remove("failed") {
		t.Error("terminal agents must be removable")
	}
	if s.Remove("running") {
		t.Error("running agent must be kept")
	}
	if s.Remove("missing") {
		t.Error("missing ID must report false")
	}
	if _, ok := s.Get("running"); !ok {
		t.Error("running agent lost from registry")
	}
}

// End to end: once GetResult has cached a completion, the spawner entry is
// released but the cached answer survives — the registry holds an agent
// exactly until its result has a durable home.
func TestJITExecutor_GetResultReapsSpawner(t *testing.T) {
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return &types.LLMToolResponse{Text: "reaped"}, nil
		},
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			return "reaped", nil
		},
	}
	spawner := NewSpawner(
		&MockKernel{}, &MockVirtualStore{}, mockLLM,
		&MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{},
		DefaultSpawnerConfig(),
	)
	j := NewJITExecutor(createTestExecutor(t), spawner, &MockTransducer{})
	taskID, err := j.ExecuteAsync(context.Background(), TaskRequest{IntentVerb: "/test", Task: "t"})
	if err != nil {
		t.Fatalf("ExecuteAsync: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := j.WaitForResult(ctx, taskID); err != nil {
		t.Fatalf("WaitForResult: %v", err)
	}
	if _, ok := spawner.Get(taskID); ok {
		t.Error("completed agent still in spawner after its result was cached")
	}
	res, done, err := j.GetResult(taskID)
	if err != nil || !done || res != "reaped" {
		t.Errorf("cached result lost with the spawner entry: res=%q done=%v err=%v", res, done, err)
	}
}
