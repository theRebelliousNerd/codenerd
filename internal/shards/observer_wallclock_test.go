package shards

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/config"
)

type deadlineSpawner struct {
	started chan context.Context
}

func (s *deadlineSpawner) SpawnObserver(ctx context.Context, observerName, task string) (string, error) {
	s.started <- ctx
	<-ctx.Done()
	return "", ctx.Err()
}

// TestProcessEvent_SpawnedObserverHasNoWallClock pins that a spawned observer
// task inherits the manager run context and nothing else. Stop cancels that
// context; a two-minute clock does not.
func TestProcessEvent_SpawnedObserverHasNoWallClock(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	spawner := &deadlineSpawner{started: make(chan context.Context, 1)}
	m := NewBackgroundObserverManager(spawner)
	t.Cleanup(func() {
		cancel()
		m.taskWG.Wait()
	})
	if err := m.RegisterObserver("northstar"); err != nil {
		t.Fatal(err)
	}

	m.processEvent(parent, ObserverEvent{Type: EventAlignmentCheck, Source: "test"})
	var spawnCtx context.Context
	select {
	case spawnCtx = <-spawner.started:
	case <-time.After(2 * time.Second):
		t.Fatal("spawner was not called")
	}
	if _, ok := spawnCtx.Deadline(); ok {
		t.Fatal("spawned observer has a deadline")
	}
	cancel()
	select {
	case <-spawnCtx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("cancelling the manager context left the observer running")
	}
}

type deadlineNorthstar struct {
	got chan context.Context
}

func (h *deadlineNorthstar) HandleEvent(ctx context.Context, event ObserverEvent) (*ObserverAssessment, error) {
	h.got <- ctx
	return nil, nil
}

// TestProcessEvent_NorthstarCheckUsesPerCallTimeout pins the one alignment
// completion to llm_timeouts.per_call_timeout. The manager context stays the
// parent, so Stop still cancels the call.
func TestProcessEvent_NorthstarCheckUsesPerCallTimeout(t *testing.T) {
	prev := config.GetLLMTimeouts()
	t.Cleanup(func() { config.SetLLMTimeouts(prev) })
	configured := prev
	configured.PerCallTimeout = 37 * time.Second
	config.SetLLMTimeouts(configured)

	parent, cancel := context.WithCancel(context.Background())
	handler := &deadlineNorthstar{got: make(chan context.Context, 1)}
	m := NewBackgroundObserverManager(nil)
	m.SetNorthstarHandler(handler)
	t.Cleanup(func() {
		cancel()
		m.taskWG.Wait()
	})
	if err := m.RegisterObserver("northstar"); err != nil {
		t.Fatal(err)
	}

	m.processEvent(parent, ObserverEvent{Type: EventAlignmentCheck, Source: "test"})
	var got context.Context
	select {
	case got = <-handler.got:
	case <-time.After(2 * time.Second):
		t.Fatal("northstar handler was not called")
	}
	dl, ok := got.Deadline()
	if !ok {
		t.Fatal("northstar check has no deadline")
	}
	rem := time.Until(dl)
	if rem < 30*time.Second || rem > 37*time.Second {
		t.Fatalf("northstar deadline remaining %s, want per_call_timeout (37s)", rem)
	}
	cancel()
	select {
	case <-got.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("cancelling the manager context left the alignment check running")
	}
	m.taskWG.Wait()

	configured.PerCallTimeout = 0
	config.SetLLMTimeouts(configured)
	openParent, openCancel := context.WithCancel(context.Background())
	t.Cleanup(openCancel)
	open, openStop := northstarCheckContext(openParent)
	t.Cleanup(openStop)
	if _, ok := open.Deadline(); ok {
		t.Fatal("non-positive per_call_timeout added a deadline")
	}
	openCancel()
	if err := open.Err(); err == nil {
		t.Fatal("cancelling the parent left the alignment context running")
	}
}
