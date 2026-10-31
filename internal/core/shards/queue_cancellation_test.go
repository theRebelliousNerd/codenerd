package shards

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"codenerd/internal/types"
)

type queuedCancellationAgent struct {
	*BaseShardAgent
	started chan string
	release <-chan struct{}
	err     error
}

func (a *queuedCancellationAgent) Execute(context.Context, string) (string, error) {
	a.started <- a.GetID()
	<-a.release
	return "late execution result", a.err
}

func waitQueueDisposition(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ready() {
		if time.Now().After(deadline) {
			t.Fatal("queue disposition barrier did not complete")
		}
		runtime.Gosched()
	}
}

func receiveQueueDisposition[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("queue disposition event did not arrive")
		var zero T
		return zero
	}
}

func TestSpawnQueue_CanceledSynchronousWaiterDisposesLateResult(t *testing.T) {
	for _, executionErr := range []error{nil, errors.New("late execution failure")} {
		name := "success"
		if executionErr != nil {
			name = "execution_error"
		}
		t.Run(name, func(t *testing.T) {
			sm := NewShardManager()
			started := make(chan string, 1)
			release := make(chan struct{})
			var once sync.Once
			finish := func() { once.Do(func() { close(release) }) }
			t.Cleanup(finish)
			sm.RegisterShard("queued_cancel", func(id string, cfg types.ShardConfig) types.ShardAgent {
				return &queuedCancellationAgent{NewBaseShardAgent(id, cfg), started, release, executionErr}
			})
			sm.DefineProfile("queued_cancel", types.ShardConfig{Name: "queued_cancel", Type: types.ShardTypePersistent})
			nerdDir := t.TempDir()
			dbPath := types.AgentKnowledgeDBPath(nerdDir, "queued_cancel")
			if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dbPath, []byte("test"), 0o600); err != nil {
				t.Fatal(err)
			}
			registered, unregistered := make(chan string, 1), make(chan string, 1)
			sm.SetNerdDir(nerdDir)
			sm.SetJITRegistrar(func(name, path string) (func(), error) {
				if path != dbPath {
					t.Errorf("registered database = %q, want %q", path, dbPath)
				}
				registered <- name
				return func() { unregistered <- name }, nil
			})
			// Cancellation of this request must not consume another attached result.
			sm.recordResult("unrelated-result", "keep this result", nil)
			otherStarted, otherRelease := make(chan string, 1), make(chan struct{})
			var otherOnce sync.Once
			finishOther := func() { otherOnce.Do(func() { close(otherRelease) }) }
			t.Cleanup(finishOther)
			sm.RegisterShard("unrelated", func(id string, cfg types.ShardConfig) types.ShardAgent {
				return &queuedCancellationAgent{NewBaseShardAgent(id, cfg), otherStarted, otherRelease, nil}
			})
			sm.DefineProfile("unrelated", types.ShardConfig{Name: "unrelated", Type: types.ShardTypeEphemeral})
			otherID, err := sm.SpawnAsync(context.Background(), "unrelated", "independent task")
			if err != nil {
				t.Fatal(err)
			}
			if got := receiveQueueDisposition(t, otherStarted); got != otherID {
				t.Fatalf("unrelated started shard = %q, want %q", got, otherID)
			}
			sq := NewSpawnQueue(sm, nil, SpawnQueueConfig{WorkerCount: 1})
			sm.SetSpawnQueue(sq)
			if err := sq.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				finish()
				if err := sq.Stop(); err != nil {
					t.Error(err)
				}
			})
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			waited := make(chan error, 1)
			go func() {
				_, err := sm.SpawnWithPriority(ctx, "queued_cancel", "task", nil, types.PriorityNormal)
				waited <- err
			}()
			id := receiveQueueDisposition(t, started)
			if got := receiveQueueDisposition(t, registered); got != "queued_cancel" {
				t.Fatalf("registered shard = %q", got)
			}
			cancel()
			if err := receiveQueueDisposition(t, waited); err != context.Canceled {
				t.Fatalf("wait error = %v, want caller cancellation", err)
			}
			// The outer SubmitAndWait select may return before its worker. Observe
			// the worker's completed wait before releasing the still-running agent.
			waitQueueDisposition(t, func() bool { return sq.GetMetrics().TotalSpawned == 1 })
			sm.mu.RLock()
			_, active := sm.shards[id]
			_, detached := sm.detached[id]
			_, retained := sm.results[id]
			_, lease := sm.activeJITDBs[id]
			sm.mu.RUnlock()
			if !active || !detached || retained || !lease {
				t.Errorf("after abandoned wait, before completion: active=%v detached=%v retained=%v JITlease=%v; want true, true, false, true", active, detached, retained, lease)
			}
			finish()
			if got := receiveQueueDisposition(t, unregistered); got != "queued_cancel" {
				t.Fatalf("unregistered shard = %q", got)
			}
			// recordResult releases the lease and publishes disposition under one
			// mutex; acquiring that mutex after the callback joins publication.
			sm.mu.RLock()
			_, active = sm.shards[id]
			_, detached = sm.detached[id]
			_, retained = sm.results[id]
			_, lease = sm.activeJITDBs[id]
			_, otherActive := sm.shards[otherID]
			_, otherDetached := sm.detached[otherID]
			_, otherRetained := sm.results[otherID]
			sm.mu.RUnlock()
			if active || detached || retained || lease {
				t.Errorf("after late completion: active=%v detached=%v retained=%v JITlease=%v; want all false", active, detached, retained, lease)
			}
			if result, ok := sm.GetResult("unrelated-result"); !ok || result.Result != "keep this result" {
				t.Fatalf("unrelated attached result changed: %+v, present=%v", result, ok)
			}
			if !otherActive || otherDetached || otherRetained {
				t.Errorf("unrelated running shard changed: active=%v detached=%v retained=%v", otherActive, otherDetached, otherRetained)
			}
			finishOther()
			waitQueueDisposition(t, func() bool { return sm.GetActiveShardCount() == 0 })
			if result, ok := sm.GetResult(otherID); !ok || result.Error != nil || result.Result != "late execution result" {
				t.Errorf("unrelated execution result changed: %+v, present=%v", result, ok)
			}
		})
	}
}

func TestSpawnQueue_CanceledWaitDisposesAlreadyRecordedResult(t *testing.T) {
	for _, tc := range []struct {
		name string
		ctx  func() (context.Context, context.CancelFunc)
		err  error
	}{
		{"canceled", func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) }, context.Canceled},
		{"deadline", func() (context.Context, context.CancelFunc) {
			return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		}, context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sm := NewShardManager()
			sm.recordResult("completed", "completed before waiter left", nil)
			ctx, cancel := tc.ctx()
			cancel()
			result := NewSpawnQueue(sm, nil, SpawnQueueConfig{}).waitForShardCompletion(ctx, "completed")
			if result.Error != tc.err || result.ShardID != "completed" {
				t.Fatalf("abandoned wait = %+v", result)
			}
			sm.mu.RLock()
			defer sm.mu.RUnlock()
			if len(sm.results) != 0 || len(sm.detached) != 0 {
				t.Fatalf("completed abandoned result: retained=%d detached=%d, want 0, 0", len(sm.results), len(sm.detached))
			}
		})
	}
}

func TestSpawnQueue_ConsumedExecutionErrorsDoNotDetach(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"success", nil},
		{"execution_error", errors.New("execution failed")},
		{"execution_canceled", context.Canceled},
		{"execution_deadline", context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sm := NewShardManager()
			release := make(chan struct{})
			close(release)
			sm.RegisterShard("queue_result", func(id string, cfg types.ShardConfig) types.ShardAgent {
				return &queuedCancellationAgent{NewBaseShardAgent(id, cfg), make(chan string, 1), release, tc.err}
			})
			sm.DefineProfile("queue_result", types.ShardConfig{Name: "queue_result", Type: types.ShardTypeEphemeral})
			sq := NewSpawnQueue(sm, nil, SpawnQueueConfig{WorkerCount: 1})
			if err := sq.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := sq.Stop(); err != nil {
					t.Error(err)
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			result, err := sq.SubmitAndWait(ctx, "queue_result", "task", nil, types.PriorityNormal)
			if ctx.Err() != nil || err != tc.err || result.ShardID == "" || result.Result != "late execution result" || result.Error != tc.err {
				t.Fatalf("consumed result changed: %+v", result)
			}
			sm.mu.RLock()
			defer sm.mu.RUnlock()
			if len(sm.results) != 0 || len(sm.detached) != 0 {
				t.Fatalf("consumed execution left retained=%d detached=%d, want 0, 0", len(sm.results), len(sm.detached))
			}
		})
	}
}

func TestSpawnQueue_CancellationCompletionRaceIsIsolated(t *testing.T) {
	sm := NewShardManager()
	sq := NewSpawnQueue(sm, nil, SpawnQueueConfig{})
	sm.recordResult("other-result", "keep", nil)
	sm.detach("other-running")
	const attempts = 64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range attempts {
		id := "racing-" + strconv.Itoa(i)
		ctx, cancel := context.WithCancel(context.Background())
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			cancel()
			result := sq.waitForShardCompletion(ctx, id)
			if result.Error != context.Canceled && (result.Error != nil || result.Result != "completion racing with cancellation") {
				t.Errorf("racing wait returned neither cancellation nor the consumed result: %+v", result)
			}
		}()
		go func() {
			defer wg.Done()
			<-start
			sm.recordResult(id, "completion racing with cancellation", nil)
		}()
	}
	close(start)
	wg.Wait()
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if len(sm.results) != 1 || sm.results["other-result"].Result != "keep" {
		t.Errorf("racing completions retained %d results, want only unrelated result", len(sm.results))
	}
	if _, ok := sm.detached["other-running"]; !ok || len(sm.detached) != 1 {
		t.Errorf("racing completions left %d detached marks, want only unrelated running mark", len(sm.detached))
	}
}
