package chat

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/session"
	"codenerd/internal/types"
)

// heldExecutor is a task executor whose task runs until the test releases
// it, so the test controls when the task settles.
type heldExecutor struct {
	started chan context.Context
	release chan struct{}
	calls   atomic.Int64
}

func newHeldExecutor() *heldExecutor {
	return &heldExecutor{started: make(chan context.Context, 1), release: make(chan struct{})}
}

func (e *heldExecutor) Execute(ctx context.Context, _ session.TaskRequest) (string, error) {
	e.calls.Add(1)
	e.started <- ctx
	<-e.release
	return "reviewed", ctx.Err()
}
func (e *heldExecutor) ExecuteWithContext(ctx context.Context, req session.TaskRequest, _ *types.SessionContext, _ types.SpawnPriority) (string, error) {
	return e.Execute(ctx, req)
}
func (e *heldExecutor) ExecuteAsync(context.Context, session.TaskRequest) (string, error) {
	return "", nil
}
func (e *heldExecutor) GetResult(string) (string, bool, error) { return "", false, nil }
func (e *heldExecutor) WaitForResult(context.Context, string) (string, error) {
	return "", nil
}

// runChildren runs every command a batch holds, each on its own goroutine as
// the Bubble Tea runtime does, and delivers their messages on the channel.
func runChildren(cmd tea.Cmd, out chan<- tea.Msg) {
	if cmd == nil {
		return
	}
	go func() {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				runChildren(c, out)
			}
			return
		}
		out <- msg
	}()
}

func slotTestModel(t *testing.T, exec session.TaskExecutor) Model {
	t.Helper()
	m := NewTestModel()
	t.Cleanup(m.shutdownCancel)
	m.shardMgr = coreshards.NewShardManager()
	m.taskExecutor = exec
	return m
}

// /spawn set the loading flag but never took the foreground slot: Ctrl+X found
// no owner, cleared the spinner and said "Stopped" while the task ran on, and
// the next message took the apparently free slot (audit
// slash-operation-slot-admission). The task now holds the slot from Enter
// until it settles.
func TestSpawnSlashCommand_HoldsTheForegroundSlotUntilItSettles(t *testing.T) {
	t.Parallel()
	exec := newHeldExecutor()
	m := slotTestModel(t, exec)

	m, cmd := typeAndEnter(t, m, "/spawn reviewer review README.md")
	if m.turn.current() == 0 {
		t.Fatal("/spawn is running but holds no foreground slot")
	}
	msgs := make(chan tea.Msg, 4)
	runChildren(cmd, msgs)
	var taskCtx context.Context
	select {
	case taskCtx = <-exec.started:
	case <-time.After(10 * time.Second):
		t.Fatal("the /spawn task never reached the executor")
	}

	// Ctrl+X cancels the task itself, and the slot stays held while it winds
	// down.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = next.(Model)
	select {
	case <-taskCtx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("Ctrl+X said stopped, but the /spawn task's context was never canceled")
	}
	if !m.turn.stopping() {
		t.Fatal("the stopped /spawn task released the slot before it settled")
	}

	// A message typed meanwhile waits; it does not run beside the task.
	m, cmd = typeAndEnter(t, m, "what does the README say about installing?")
	if cmd != nil || m.turn.queuedCount() != 1 {
		t.Fatalf("a message typed while /spawn wound down was not queued (queued %d, command %v)", m.turn.queuedCount(), cmd != nil)
	}

	// Settling releases the slot, and the queued message starts once.
	close(exec.release)
	var settled tea.Msg
	for settled == nil {
		select {
		case msg := <-msgs:
			switch msg.(type) {
			case errorMsg, assistantMsg:
				settled = msg
			}
		case <-time.After(10 * time.Second):
			t.Fatal("the /spawn task never settled")
		}
	}
	if m.turn.current() != 0 {
		t.Fatal("the settled /spawn task still holds the slot")
	}
	next, cmd = m.Update(settled)
	m = next.(Model)
	if cmd == nil || m.turn.queuedCount() != 0 || !m.isLoading {
		t.Fatalf("the queued message did not start after /spawn settled (queued %d)", m.turn.queuedCount())
	}
}

// A stop that lands after the handler returned but before the task starts is
// latched: the task does no work, and its slot is released.
func TestSpawnSlashCommand_AStopBeforeStartIsLatched(t *testing.T) {
	t.Parallel()
	exec := newHeldExecutor()
	m := slotTestModel(t, exec)

	m, cmd := typeAndEnter(t, m, "/spawn reviewer review README.md")
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = next.(Model)

	msgs := make(chan tea.Msg, 4)
	runChildren(cmd, msgs)
	deadline := time.After(10 * time.Second)
	for {
		select {
		case msg := <-msgs:
			if _, ok := msg.(errorMsg); !ok {
				continue
			}
			if exec.calls.Load() != 0 {
				t.Fatal("a /spawn task stopped before it started still ran")
			}
			if m.turn.current() != 0 {
				t.Fatal("the latched /spawn task did not release its slot")
			}
			return
		case <-deadline:
			t.Fatalf("the latched /spawn task never settled (executor calls %d)", exec.calls.Load())
		}
	}
}

// A panic in the task is recovered and releases only its own slot.
func TestForegroundOperation_APanicReleasesItsOwnSlot(t *testing.T) {
	t.Parallel()
	m := slotTestModel(t, newHeldExecutor())
	cmd := m.foregroundOperation("reviewer", func(context.Context) tea.Msg { panic("boom") })
	if m.turn.current() == 0 {
		t.Fatal("foregroundOperation did not take the slot on the caller's thread")
	}
	if _, ok := cmd().(errorMsg); !ok {
		t.Fatal("a panicking foreground task did not come back as an error")
	}
	if m.turn.current() != 0 {
		t.Fatal("a panicking foreground task stranded the slot")
	}
}
