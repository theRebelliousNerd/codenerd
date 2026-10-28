package chat

import (
	"context"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// Talking while work runs. A turn that has started work (a delegation, a
// decomposition) keeps the kernel's per-turn facts (user_intent, the routing
// signals) that its gates read, so a message typed meanwhile must not become
// a full turn: it would rewrite them under the running coder. It becomes a
// side turn instead (policy/side_turns.mg):
//
//   - perception reads it with the brief (direct transducer: no user_intent
//     is asserted) and its reply is shown;
//   - the brief it returns replaces the kernel's, which is how a mid-work
//     clarification steers the coder: its next compile reads the brief;
//   - Jev's judgments, when on, are asserted as side_turn_judgment, never as
//     the turn's own jev_judgment;
//   - the kernel derives the action: /stop cancels the running turn, /later
//     queues the message as a full turn for when the work ends, /converse is
//     just the reply.
//
// A message typed before the turn has started work (still perceiving or
// routing) is queued whole.

// turnState is the in-flight turn, shared across Model copies.
type turnState struct {
	mu      sync.Mutex
	cancel  context.CancelFunc
	working string
	queued  []string
}

func newTurnState() *turnState { return &turnState{} }

// begin records the running turn's cancel.
func (t *turnState) begin(cancel context.CancelFunc) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cancel, t.working = cancel, ""
}

// end clears the running turn.
func (t *turnState) end() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cancel, t.working = nil, ""
}

// setWorking marks the turn as running work, described by label.
func (t *turnState) setWorking(label string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.working = label
}

// workingOn is what the running turn works on; "" before work or between turns.
func (t *turnState) workingOn() string {
	if t == nil {
		return ""
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.working
}

// stop cancels the running turn; false when none runs.
func (t *turnState) stop() bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cancel == nil {
		return false
	}
	t.cancel()
	return true
}

func (t *turnState) enqueue(input string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.queued = append(t.queued, input)
}

func (t *turnState) dequeue() (string, bool) {
	if t == nil {
		return "", false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.queued) == 0 {
		return "", false
	}
	next := t.queued[0]
	t.queued = t.queued[1:]
	return next, true
}

func (t *turnState) queuedCount() int {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.queued)
}

// sideTurnMsg is a side turn's outcome.
type sideTurnMsg struct {
	Input  string
	Reply  string
	Action string // /stop, /later or /converse (side_turn_action)
	Err    error
}

// submitWhileBusy handles Enter while a turn is in flight: a side turn once
// the turn runs work, a queued message before then.
func (m Model) submitWhileBusy() (Model, tea.Cmd) {
	input := strings.TrimSpace(m.textarea.Value())
	if input == "" {
		return m, nil
	}
	m.textarea.Reset()
	m = m.addMessage(Message{Role: "user", Content: input})
	if m.turn.workingOn() == "" {
		m.turn.enqueue(input)
		return m.pushAssistantMsg("_Queued: I'll take this up as soon as the current turn finishes._"), nil
	}
	return m, m.sideTurn(input)
}

// sideTurn reads a message typed while work runs (see the file comment).
func (m Model) sideTurn(input string) tea.Cmd {
	history := make([]perception.ConversationTurn, 0, len(m.history))
	for _, h := range m.history {
		history = append(history, perception.ConversationTurn{Role: h.Role, Content: h.Content})
	}
	return func() tea.Msg {
		ctx := m.shutdownCtx
		if ctx == nil {
			ctx = context.Background()
		}
		var judged <-chan judgedTurn
		judged = m.startTurnJudgments(ctx, input, history)
		intent, err := m.transducer.ParseIntentWithContext(m.briefContext(ctx), input, history)
		if err != nil {
			return sideTurnMsg{Input: input, Err: err}
		}
		m.recordBrief(intent.Brief)
		action := m.decideSideTurn(ctx, intent, judged)
		return sideTurnMsg{Input: input, Reply: strings.TrimSpace(intent.Response), Action: action}
	}
}

// sideTurnPredicates are the side turn's own facts, replaced each side turn.
var sideTurnPredicates = []string{"side_turn_intent", "side_turn_judgment"}

// decideSideTurn asserts the side turn's reading and asks the kernel what to
// do with it (side_turn_action, policy/side_turns.mg). No kernel, or no
// derivation, is a reply only.
func (m Model) decideSideTurn(ctx context.Context, intent perception.Intent, judged <-chan judgedTurn) string {
	const converse = "/converse"
	if m.kernel == nil {
		return converse
	}
	log := logging.Get(logging.CategoryKernel)
	for _, p := range sideTurnPredicates {
		_ = m.kernel.Retract(p)
	}
	facts := []core.Fact{{Predicate: "side_turn_intent", Args: []any{types.MangleAtom(intent.Category), types.MangleAtom(intent.Verb)}}}
	if judged != nil {
		select {
		case j := <-judged:
			if j.err == nil {
				for _, f := range j.result.Facts() {
					if f.Predicate == "jev_judgment" {
						facts = append(facts, core.Fact{Predicate: "side_turn_judgment", Args: f.Args})
					}
				}
			} else {
				log.Warn("jev: side turn without judgments: %v", j.err)
			}
		case <-ctx.Done():
		}
	}
	if err := config.EnsureParams(m.kernel, m.Config.TypeSafe.Params()); err != nil {
		log.Warn("side turn: thresholds not asserted: %v", err)
	}
	if err := m.kernel.AssertBatch(facts); err != nil {
		log.Error("side turn: assert: %v", err)
		return converse
	}
	rows, err := m.kernel.Query("side_turn_action")
	if err != nil || len(rows) != 1 || len(rows[0].Args) != 1 {
		return converse
	}
	return types.ExtractString(rows[0].Args[0])
}

// applySideTurn shows the side turn's reply and acts on the kernel's verdict.
func (m Model) applySideTurn(msg sideTurnMsg) Model {
	if msg.Err != nil {
		return m.pushAssistantMsg("I couldn't read that while the work runs (" + msg.Err.Error() + "); it is queued for when the work finishes.")
	}
	reply := msg.Reply
	switch msg.Action {
	case "/stop":
		if m.turn.stop() {
			reply = strings.TrimSpace(reply + "\n\nStopping the current work: " + m.turn.workingOn())
		}
	case "/later":
		m.turn.enqueue(msg.Input)
		reply = strings.TrimSpace(reply + "\n\n_I'll take this up as soon as the current work finishes._")
	}
	if reply == "" {
		reply = "Noted."
	}
	return m.pushAssistantMsg(reply)
}

// startQueuedTurn runs the next queued message as a full turn once nothing is
// in flight; nil when nothing waits or a turn still runs.
func (m Model) startQueuedTurn() (Model, tea.Cmd) {
	if m.isLoading {
		return m, nil
	}
	next, ok := m.turn.dequeue()
	if !ok {
		return m, nil
	}
	m.isLoading = true
	return m, m.processInput(next)
}

// turnFooter names running work and queued messages; "" when there is none.
func (m Model) turnFooter() string {
	var parts []string
	if w := m.turn.workingOn(); w != "" {
		parts = append(parts, "working: "+w)
	}
	if n := m.turn.queuedCount(); n > 0 {
		parts = append(parts, "queued: "+strings.Repeat("•", n))
	}
	return strings.Join(parts, " ")
}
