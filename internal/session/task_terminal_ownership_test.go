package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"codenerd/internal/types"
)

// T1 (Docs/audits/task-terminal-ownership.md): a waiter that gives up must
// not leave the finished task stranded. The worker settles its own result
// into the cache and releases the registry entry, with no later poll.
func TestJITExecutor_CanceledWaiterStillSettlesTheTask(t *testing.T) {
	release := make(chan struct{})
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			<-release
			return &types.LLMToolResponse{Text: "late result"}, nil
		},
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			<-release
			return "late result", nil
		},
	}
	spawner := NewSpawner(&MockKernel{}, &MockVirtualStore{}, mockLLM, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{}, DefaultSpawnerConfig())
	jitExec := NewJITExecutor(createTestExecutor(t), spawner, &MockTransducer{})

	// Detached: the task's own context stays live after the waiter leaves.
	taskID, err := jitExec.ExecuteAsync(context.Background(), TaskRequest{IntentVerb: "/test", Task: "slow"})
	if err != nil {
		t.Fatalf("ExecuteAsync: %v", err)
	}
	agent, ok := spawner.Get(taskID)
	if !ok {
		t.Fatal("agent not registered")
	}
	waitCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := jitExec.WaitForResult(waitCtx, taskID); !errors.Is(err, context.Canceled) {
		t.Fatalf("WaitForResult = %v; want the waiter's cancellation", err)
	}
	close(release)
	select {
	case <-agent.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the task never reached a terminal state")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		jitExec.mu.RLock()
		r := jitExec.results[taskID]
		jitExec.mu.RUnlock()
		_, registered := spawner.Get(taskID)
		if r != nil && r.Completed && !registered {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("not settled without a poll: cached=%+v registered=%v", r, registered)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// T2: a Stop that arrives before Run installs its cancel is latched; the
// task ends stopped instead of running with a live context.
func TestSubAgent_StopBeforeRunIsHonoured(t *testing.T) {
	called := false
	agent := NewSubAgent(DefaultSubAgentConfig("test"), &MockKernel{}, &MockVirtualStore{}, &MockLLMClient{
		CompleteWithSystemFunc: func(context.Context, string, string) (string, error) {
			called = true
			return "ran", nil
		},
	}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	settled := make(chan struct{})
	agent.SetOnTerminal(func() { close(settled) })
	if err := agent.Stop(); err != nil {
		t.Fatal(err)
	}
	agent.Run(context.Background(), "task")
	select {
	case <-settled:
	case <-time.After(5 * time.Second):
		t.Fatal("onTerminal never ran for a task stopped before it started")
	}
	if _, err := agent.GetResult(); !errors.Is(err, context.Canceled) || agent.GetState() != SubAgentStateFailed || called {
		t.Fatalf("state %v err %v model called %v; want stopped before start", agent.GetState(), err, called)
	}
}
