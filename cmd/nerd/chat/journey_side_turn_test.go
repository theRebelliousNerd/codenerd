package chat

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/config"
	"codenerd/internal/perception"
)

// journey_side_turn_test.go: talking while work runs (side_turn.go,
// policy/side_turns.mg). The running turn is simulated by registering its
// cancel and marking it working, as processInput does at delegation.

// busyModel is a chat whose turn is running work; the returned context is the
// running turn's, so a test can see whether it was cancelled.
func busyModel(t *testing.T, intents ...perception.Intent) (Model, context.Context) {
	t.Helper()
	m, _ := journeyModel(t, config.WorkModePropose, intents...)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	m.turn.begin(cancel)
	m.turn.setWorking("coder: fix README.md")
	m.isLoading = true
	return m, ctx
}

// typeAndEnter types input and presses Enter, the way the TUI does.
func typeAndEnter(t *testing.T, m Model, input string) (Model, tea.Cmd) {
	t.Helper()
	m.textarea.SetValue(input)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return next.(Model), cmd
}

func runSideTurn(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		t.Fatal("Enter while working started no side turn")
	}
	msg, ok := cmd().(sideTurnMsg)
	if !ok {
		t.Fatalf("Enter while working ran %T, want a side turn", msg)
	}
	next, _ := m.Update(msg)
	return next.(Model)
}

// A question while the coder works is answered, the work keeps running, and
// the clarification lands in the brief the coder reads next.
func TestSideTurn_ConversationWhileWorkRunsSteersTheBrief(t *testing.T) {
	answer := perception.Intent{
		Category: "/query", Verb: "/converse", Target: "none", Confidence: 0.9,
		Response: "It's editing the second line now.",
		Brief:    &perception.Brief{Goal: "fix the README typo", Constraints: []string{"keep the badge line as is"}, Ready: true},
	}
	m, ctx := busyModel(t, answer)
	m, cmd := typeAndEnter(t, m, "how's it going? and keep the badge line as is")
	m = runSideTurn(t, m, cmd)

	if !strings.Contains(lastAssistant(m), "editing the second line") {
		t.Fatalf("the side turn was not answered:\n%s", lastAssistant(m))
	}
	if ctx.Err() != nil {
		t.Fatal("a conversational side turn stopped the work")
	}
	if b := m.currentBrief(); b == nil || len(b.Constraints) != 1 {
		t.Fatalf("the clarification did not reach the brief: %+v", b)
	}
	if m.turn.queuedCount() != 0 {
		t.Fatal("conversation was queued as work")
	}
	if rows, _ := m.kernel.Query("user_intent"); len(rows) != 0 {
		t.Fatalf("a side turn asserted user_intent under the running turn: %v", rows)
	}
}

// "stop" (Jev's wants_to_stop) cancels the running turn.
func TestSideTurn_StopCancelsTheRunningTurn(t *testing.T) {
	chat := perception.Intent{Category: "/query", Verb: "/converse", Target: "none", Confidence: 0.9, Response: "Okay."}
	m, ctx := busyModel(t, chat)
	m = withJev(t, m, &fakeJevServer{probs: map[string]float64{"wants_to_stop": 0.97, "changes_files": 0.03}})
	m, cmd := typeAndEnter(t, m, "stop, I changed my mind")
	m = runSideTurn(t, m, cmd)
	if ctx.Err() == nil {
		t.Fatal("stop did not cancel the running turn")
	}
	if !strings.Contains(lastAssistant(m), "Stopping the current work") {
		t.Fatalf("the stop was not acknowledged:\n%s", lastAssistant(m))
	}
}

// A request for other work waits for the running turn, then runs as a full
// turn of its own.
func TestSideTurn_NewWorkQueuesAndStartsWhenTheTurnEnds(t *testing.T) {
	more := perception.Intent{Category: "/mutation", Verb: "/create", Target: "CHANGELOG.md", Confidence: 0.9, Response: "Sure."}
	m, ctx := busyModel(t, more, more)
	m, cmd := typeAndEnter(t, m, "after that, add a CHANGELOG entry")
	m = runSideTurn(t, m, cmd)
	if ctx.Err() != nil || m.turn.queuedCount() != 1 {
		t.Fatalf("new work did not queue (cancelled=%v queued=%d)", ctx.Err() != nil, m.turn.queuedCount())
	}
	if !strings.Contains(m.renderFooter(), "queued") {
		t.Fatalf("the footer does not show the queued message:\n%s", m.renderFooter())
	}

	// The running turn ends: the queued message starts as a turn.
	m.turn.end()
	next, cmd := m.Update(responseMsg("README fixed."))
	m = next.(Model)
	if cmd == nil || !m.isLoading || m.turn.queuedCount() != 0 {
		t.Fatalf("the queued message did not start (cmd=%v loading=%v queued=%d)", cmd != nil, m.isLoading, m.turn.queuedCount())
	}
}

// A message sent before the turn has started work is queued whole.
func TestSideTurn_BeforeWorkStartsTheMessageQueues(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModePropose)
	m.isLoading = true
	m, cmd := typeAndEnter(t, m, "also check the tests")
	if cmd != nil || m.turn.queuedCount() != 1 {
		t.Fatalf("a message during perception was not queued (cmd=%v queued=%d)", cmd != nil, m.turn.queuedCount())
	}
}

// Ctrl+X cancels the running turn, not just the spinner.
func TestSideTurn_CtrlXCancelsTheRunningTurn(t *testing.T) {
	m, ctx := busyModel(t)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if ctx.Err() == nil {
		t.Fatal("Ctrl+X left the turn running")
	}
	if next.(Model).isLoading {
		t.Fatal("Ctrl+X left the chat busy")
	}
}
