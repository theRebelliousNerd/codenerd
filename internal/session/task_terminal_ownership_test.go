package session

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"codenerd/internal/observation"
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

// TASK-OWNER-03: a result the retention bound evicted reads as expired, not as
// a task that never existed, so a late reader does not retry or give up wrongly.
func TestJITExecutor_ALateReaderIsToldTheResultExpired(t *testing.T) {
	spawner := NewSpawner(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{}, DefaultSpawnerConfig())
	j := NewJITExecutor(createTestExecutor(t), spawner, &MockTransducer{})
	for i := 0; i <= maxCachedResults; i++ {
		j.cacheCompletedResult(fmt.Sprintf("t-%d", i), observation.Return{Output: "x"}, nil)
	}
	if _, _, err := j.getObserved("t-0"); !errors.Is(err, ErrTaskResultExpired) {
		t.Fatalf("evicted task read = %v; want ErrTaskResultExpired", err)
	}
	if _, done, err := j.getObserved("t-1"); err != nil || !done {
		t.Fatalf("retained task read = done %v, %v", done, err)
	}
	if _, _, err := j.getObserved("never-existed"); err == nil || errors.Is(err, ErrTaskResultExpired) {
		t.Fatalf("an unknown task read = %v; want plain not found", err)
	}
}

// TASK-OWNER-04: Shutdown closes admission, stops what is running, and reports
// what did not stop instead of calling a stop request termination.
func TestSpawner_ShutdownJoinsWhatStopsAndNamesWhatDoesNot(t *testing.T) {
	cooperative := make(chan struct{})
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			close(cooperative)
			<-ctx.Done()
			return nil, ctx.Err()
		},
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		},
	}
	spawner := NewSpawner(&MockKernel{}, &MockVirtualStore{}, mockLLM, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{}, DefaultSpawnerConfig())
	jitExec := NewJITExecutor(createTestExecutor(t), spawner, &MockTransducer{})
	taskID, err := jitExec.ExecuteAsync(context.Background(), TaskRequest{IntentVerb: "/test", Task: "hang"})
	if err != nil {
		t.Fatal(err)
	}
	agent, _ := spawner.Get(taskID)
	if agent == nil {
		t.Fatal("agent not registered")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	unjoined, err := spawner.Shutdown(ctx)
	if err != nil || len(unjoined) != 0 {
		t.Fatalf("Shutdown = %v, %v; a cooperative worker should be joined", unjoined, err)
	}
	select {
	case <-agent.Done():
	default:
		t.Fatal("Shutdown returned before the worker reached a terminal state")
	}
	if _, err := spawner.Spawn(context.Background(), SpawnRequest{Name: "late", Task: "x", Type: SubAgentTypeEphemeral}); !errors.Is(err, ErrSpawnerClosed) {
		t.Fatalf("Spawn after Shutdown = %v; want ErrSpawnerClosed", err)
	}
}

func TestSpawner_ShutdownNamesAWorkerThatIgnoresTheStop(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	started := make(chan struct{}, 2)
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			started <- struct{}{}
			<-release // ignores ctx
			return &types.LLMToolResponse{Text: "late"}, nil
		},
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			started <- struct{}{}
			<-release
			return "late", nil
		},
	}
	spawner := NewSpawner(&MockKernel{}, &MockVirtualStore{}, mockLLM, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{}, DefaultSpawnerConfig())
	jitExec := NewJITExecutor(createTestExecutor(t), spawner, &MockTransducer{})
	taskID, err := jitExec.ExecuteAsync(context.Background(), TaskRequest{IntentVerb: "/test", Task: "deaf"})
	if err != nil {
		t.Fatal(err)
	}
	<-started

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	unjoined, err := spawner.Shutdown(ctx)
	if err == nil || len(unjoined) != 1 || unjoined[0] != taskID {
		t.Fatalf("Shutdown = %v, %v; want the deaf worker %s named, not reported joined", unjoined, err, taskID)
	}
}
