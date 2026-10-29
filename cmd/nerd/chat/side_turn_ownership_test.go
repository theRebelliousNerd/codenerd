package chat

import (
	"context"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/turnjudge"
)

// side_turn_ownership_test.go: a side turn, a stop and the queue act on the
// execution and the message they belong to (turnState), whatever order
// their asynchronous results arrive in.

// hookTransducer runs during reads before answering, standing in for a model
// call during which the chat moves on.
type hookTransducer struct {
	scriptedTransducer
	during func()
}

func (h *hookTransducer) ParseIntentWithContext(ctx context.Context, in string, hist []perception.ConversationTurn) (perception.Intent, error) {
	if h.during != nil {
		h.during()
	}
	return h.scriptedTransducer.ParseIntentWithContext(ctx, in, hist)
}

// startExecution accepts and registers a foreground execution the way
// processInput does, returning its id and context.
func startExecution(t *testing.T, m Model, label string) (uint64, context.Context) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	id := m.turn.accept()
	m.turn.begin(id, cancel)
	m.turn.setWorking(id, label)
	return id, ctx
}

// A stop decided for turn A arrives after A ended and B began: B runs on.
func TestSideTurn_ADelayedStopCannotReachTheNextTurn(t *testing.T) {
	t.Parallel()
	chat := perception.Intent{Category: "/query", Verb: "/converse", Target: "none", Confidence: 0.9, Response: "Okay."}
	m, _ := busyModel(t, chat)
	m = withJev(t, m, &fakeJevServer{probs: map[string]float64{"wants_to_stop": 0.97, "changes_files": 0.03}})
	a := m.turn.current()
	m, cmd := typeAndEnter(t, m, "stop that")
	msg := cmd().(sideTurnMsg)
	if msg.Action != "/stop" || msg.Owner != a {
		t.Fatalf("side turn = %+v, want a stop for turn %d", msg, a)
	}

	m.turn.end(a)
	_, bCtx := startExecution(t, m, "coder: next task")
	next, _ := m.Update(msg)
	m = next.(Model)
	if bCtx.Err() != nil {
		t.Fatal("a stop meant for the finished turn canceled the next one")
	}
	if !strings.Contains(lastAssistant(m), "already finished") {
		t.Fatalf("reply = %q", lastAssistant(m))
	}
}

// A side turn of turn A whose read finishes after A ended does not replace
// the brief the next turn works from.
func TestSideTurn_AStaleSideTurnDoesNotReplaceTheBrief(t *testing.T) {
	t.Parallel()
	m, _ := busyModel(t)
	current := &perception.Brief{Goal: "the next task", Ready: true}
	stale := &perception.Brief{Goal: "the old task", Constraints: []string{"old rule"}}
	ht := &hookTransducer{scriptedTransducer: scriptedTransducer{intents: []perception.Intent{
		{Category: "/query", Verb: "/converse", Target: "none", Confidence: 0.9, Response: "Sure.", Brief: stale},
	}}}
	ht.during = func() {
		m.turn.end(m.turn.current())
		startExecution(t, m, "coder: the next task")
		m.recordBrief(current)
	}
	m.transducer = ht
	m, cmd := typeAndEnter(t, m, "keep the old rule")
	m = runSideTurn(t, m, cmd)
	if got := m.currentBrief(); got == nil || got.Goal != "the next task" || len(got.Constraints) != 0 {
		t.Fatalf("brief = %+v, the stale side turn replaced it", got)
	}
	if !strings.Contains(lastAssistant(m), "already ended") {
		t.Fatalf("reply = %q", lastAssistant(m))
	}
}

// Two reads of the same brief: the one that finishes second does not erase
// the change the first one made; it says so instead.
func TestSideTurn_AConcurrentBriefChangeIsNotLost(t *testing.T) {
	t.Parallel()
	m, _ := busyModel(t)
	m.recordBrief(&perception.Brief{Goal: "fix the README"})
	newer := &perception.Brief{Goal: "fix the README", Constraints: []string{"keep the badge"}}
	mine := &perception.Brief{Goal: "fix the README", Constraints: []string{"use British spelling"}}
	ht := &hookTransducer{scriptedTransducer: scriptedTransducer{intents: []perception.Intent{
		{Category: "/query", Verb: "/converse", Target: "none", Confidence: 0.9, Response: "Will do.", Brief: mine},
	}}}
	ht.during = func() { m.recordBrief(newer) }
	m.transducer = ht
	m, cmd := typeAndEnter(t, m, "use British spelling")
	m = runSideTurn(t, m, cmd)
	got := m.currentBrief()
	if got == nil || len(got.Constraints) != 1 || got.Constraints[0] != "keep the badge" {
		t.Fatalf("brief = %+v, the newer change was lost", got)
	}
	if !strings.Contains(lastAssistant(m), "brief changed while I read this") {
		t.Fatalf("reply = %q", lastAssistant(m))
	}
}

// Deferred work does not steer the running work's brief.
func TestSideTurn_LaterWorkDoesNotReplaceTheActiveBrief(t *testing.T) {
	t.Parallel()
	more := perception.Intent{Category: "/mutation", Verb: "/create", Target: "CHANGELOG.md", Confidence: 0.9, Response: "Sure.",
		Brief: &perception.Brief{Goal: "add a CHANGELOG entry"}}
	m, _ := busyModel(t, more)
	m.recordBrief(&perception.Brief{Goal: "fix the README"})
	m, cmd := typeAndEnter(t, m, "afterwards add a CHANGELOG entry")
	m = runSideTurn(t, m, cmd)
	if m.turn.queuedCount() != 1 {
		t.Fatalf("queued = %d", m.turn.queuedCount())
	}
	if got := m.currentBrief(); got == nil || got.Goal != "fix the README" {
		t.Fatalf("brief = %+v, deferred work replaced it", got)
	}
}

// Two side turns decided at once read only their own facts: B's stop
// judgment is not in force when A, which judged no stop, is decided.
func TestSideTurn_ConcurrentDecisionsDoNotMixJudgments(t *testing.T) {
	t.Parallel()
	m, _ := busyModel(t)
	m.Config.TypeSafe = &config.TypeSafeConfig{}
	chat := perception.Intent{Category: "/query", Verb: "/converse", Target: "none", Confidence: 0.9}
	judged := func(stop float64) chan judgedTurn {
		ch := make(chan judgedTurn, 1)
		ch <- judgedTurn{result: &turnjudge.Result{Judgments: map[string]float64{"wants_to_stop": stop, "changes_files": 0.02}}}
		return ch
	}
	aJudged := make(chan judgedTurn, 1)
	aDone := make(chan string, 1)
	aCtx := &doneProbe{Context: context.Background(), reached: make(chan struct{})}
	go func() {
		action, _ := m.decideSideTurn(aCtx, chat, aJudged, m.turn.current(), m.currentBrief().Render())
		aDone <- action
	}()
	<-aCtx.reached // A is waiting for its judgments
	if action, _ := m.decideSideTurn(context.Background(), chat, judged(0.97), m.turn.current(), m.currentBrief().Render()); action != "/stop" {
		t.Fatalf("B = %s, want /stop", action)
	}
	aJudged <- (<-judged(0.01))
	if action := <-aDone; action != "/converse" {
		t.Fatalf("A = %s: it was decided on B's stop judgment", action)
	}
}

// doneProbe signals when a caller first waits on Done.
type doneProbe struct {
	context.Context
	reached chan struct{}
	once    sync.Once
}

func (d *doneProbe) Done() <-chan struct{} {
	d.once.Do(func() { close(d.reached) })
	return d.Context.Done()
}

// Ctrl+X: the stopped turn's late cleanup cannot clear the next turn, and
// the queued message waits until the stopped turn has wound down.
func TestCtrlX_QueuedWorkWaitsForTheStoppedTurnToSettle(t *testing.T) {
	t.Parallel()
	m, aCtx := busyModel(t)
	a := m.turn.current()
	m.turn.enqueue("then add the section")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = next.(Model)
	if aCtx.Err() == nil {
		t.Fatal("Ctrl+X did not cancel the turn")
	}
	if cmd != nil || m.turn.queuedCount() != 1 {
		t.Fatalf("the queued message started before the stopped turn ended (queued=%d)", m.turn.queuedCount())
	}
	if !strings.Contains(lastAssistant(m), "1 queued message") {
		t.Fatalf("stop notice = %q", lastAssistant(m))
	}
	// Typing now queues behind it rather than starting a second turn.
	m, cmd = typeAndEnter(t, m, "and update the docs")
	if cmd != nil || m.turn.queuedCount() != 2 {
		t.Fatalf("a message during wind-down started a turn (queued=%d)", m.turn.queuedCount())
	}

	m.turn.end(a)
	next, cmd = m.Update(errorMsg(context.Canceled))
	m = next.(Model)
	b := m.turn.current()
	if cmd == nil || b == 0 || b == a || m.turn.queuedCount() != 1 {
		t.Fatalf("the queued message did not start after the stop settled (owner=%d queued=%d)", b, m.turn.queuedCount())
	}
	m.turn.end(a) // a duplicate late cleanup of the stopped turn
	if m.turn.current() != b {
		t.Fatal("the stopped turn's cleanup released its successor's slot")
	}
}

// A stop before the accepted execution registered its cancel is latched.
func TestTurnState_StopBeforeStartIsNotLost(t *testing.T) {
	ts := newTurnState()
	id := ts.accept()
	if !ts.stop() {
		t.Fatal("stop of an accepted execution reported nothing to stop")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ts.begin(id, cancel)
	if ctx.Err() == nil {
		t.Fatal("the latched stop did not cancel the execution at begin")
	}
	ts.setWorking(id+1, "not mine")
	if ts.workingOn() != "" {
		t.Fatal("a non-owner set the working label")
	}
}

// Messages leave the queue in the order they were received, whichever
// classification finishes first.
func TestTurnState_QueueKeepsReceiptOrder(t *testing.T) {
	ts := newTurnState()
	b := ts.reserve("create the file")
	c := ts.reserve("then add the section")
	ts.settle(c, true)
	if _, ok := ts.dequeue(); ok {
		t.Fatal("a later message overtook one still being read")
	}
	ts.settle(b, true)
	first, _ := ts.dequeue()
	second, _ := ts.dequeue()
	if first != "create the file" || second != "then add the section" {
		t.Fatalf("order = %q, %q", first, second)
	}
	d := ts.reserve("how is it going?")
	ts.settle(d, false)
	if _, ok := ts.dequeue(); ok || ts.queuedCount() != 0 {
		t.Fatal("conversation stayed in the queue")
	}
}
