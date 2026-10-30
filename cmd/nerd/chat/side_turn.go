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
//
// Every foreground execution is accepted (accept) when its command is built
// on the Update thread, before it runs, and every later operation names it:
// begin registers its cancel, setWorking its label, end releases the slot.
// An operation for an execution that no longer owns the slot does nothing,
// so a canceled turn still unwinding cannot clear its successor's cancel or
// label, and a late stop cannot reach a newer turn. A stop that arrives
// before the execution registered its cancel is latched and applied at
// begin. The slot is busy from accept to end; queued messages start only
// when it is free (startQueuedTurn), so a stopped turn settles before the
// next one runs.
type turnState struct {
	mu            sync.Mutex
	seq           uint64 // last execution id handed out
	owner         uint64 // the accepted execution that has not ended; 0 when none
	cancel        context.CancelFunc
	stopRequested bool
	working       string
	queued        []queuedMessage
	msgSeq        uint64
	// requestSeq is the last request id handed out (newRequest).
	requestSeq uint64

	// decide serializes a side turn's kernel section (its decision facts,
	// the query, the brief publication). It is never held across a model
	// call.
	decide sync.Mutex
}

// queuedMessage is a message waiting for the running turn. Its place is
// taken when it is received; a side turn's message is pending until the
// kernel decides it is work (/later), so classification order cannot reorder
// the queue. Its kind is fixed when it is received: a slash command waits as
// a command and runs through the command dispatcher, never as a sentence for
// perception (audit queued-slash-command-kind-conservation).
//
// request is the id of the user message it is (Message.Request): its turn
// answers that request (request_answers.go). 0 is a step nobody typed.
type queuedMessage struct {
	id      uint64
	input   string
	request uint64
	command bool
	pending bool
}

func newTurnState() *turnState { return &turnState{} }

// accept takes the slot for a new foreground execution and returns its id.
// A previous owner that has not ended loses the slot; it keeps running
// unowned (its end and setWorking no longer touch the slot).
func (t *turnState) accept() uint64 {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.owner != 0 {
		logging.Get(logging.CategorySession).Warn("turn %d accepted while turn %d had not ended", t.seq+1, t.owner)
	}
	t.seq++
	t.owner, t.cancel, t.stopRequested, t.working = t.seq, nil, false, ""
	return t.seq
}

// begin records the execution's cancel. An execution that lost the slot
// before it started, or was stopped before it got here, is canceled at once.
func (t *turnState) begin(id uint64, cancel context.CancelFunc) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if id != t.owner || t.stopRequested {
		cancel()
		return
	}
	t.cancel = cancel
}

// end releases the slot if id still owns it.
func (t *turnState) end(id uint64) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if id != t.owner {
		return
	}
	t.owner, t.cancel, t.stopRequested, t.working = 0, nil, false, ""
}

// setWorking marks execution id as running work, described by label.
func (t *turnState) setWorking(id uint64, label string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if id == t.owner {
		t.working = label
	}
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

// current is the execution that owns the slot; 0 when none does.
func (t *turnState) current() uint64 {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.owner
}

// stopping reports an execution asked to stop that has not ended yet.
func (t *turnState) stopping() bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.owner != 0 && t.stopRequested
}

// stop cancels the execution that owns the slot (Ctrl+X); false when none
// does.
func (t *turnState) stop() bool {
	return t.stopExecution(t.current())
}

// stopExecution cancels execution id, and only it: false when id is not
// the slot's owner (it ended, or never ran).
func (t *turnState) stopExecution(id uint64) bool {
	if t == nil || id == 0 {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if id != t.owner {
		return false
	}
	t.stopRequested = true
	if t.cancel != nil {
		t.cancel()
	}
	return true
}

// enqueue queues a message that is already known to be work; its place is
// assigned here.
func (t *turnState) enqueue(q queuedMessage) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.msgSeq++
	q.id, q.pending = t.msgSeq, false
	t.queued = append(t.queued, q)
}

// reserve takes a queue place for a message whose disposition the kernel
// has not decided yet; settle resolves it.
func (t *turnState) reserve(input string, request uint64) uint64 {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.msgSeq++
	t.queued = append(t.queued, queuedMessage{id: t.msgSeq, input: input, request: request, pending: true})
	return t.msgSeq
}

// newRequest hands out the next request id (request_answers.go).
func (t *turnState) newRequest() uint64 {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.requestSeq++
	return t.requestSeq
}

// seenRequests keeps later ids above the ones a loaded session already uses.
func (t *turnState) seenRequests(top uint64) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if top > t.requestSeq {
		t.requestSeq = top
	}
}

// settle resolves a reserved place: kept as work, or released.
func (t *turnState) settle(id uint64, keep bool) {
	if t == nil || id == 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for i := range t.queued {
		if t.queued[i].id != id {
			continue
		}
		if keep {
			t.queued[i].pending = false
		} else {
			t.queued = append(t.queued[:i], t.queued[i+1:]...)
		}
		return
	}
}

// dequeue takes the oldest message when it is ready; a pending one at the
// head holds the queue, since a later message may depend on it.
func (t *turnState) dequeue() (queuedMessage, bool) {
	if t == nil {
		return queuedMessage{}, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.queued) == 0 || t.queued[0].pending {
		return queuedMessage{}, false
	}
	next := t.queued[0]
	t.queued = t.queued[1:]
	return next, true
}

// queuedCount counts the messages waiting as work.
func (t *turnState) queuedCount() int {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	n := 0
	for _, q := range t.queued {
		if !q.pending {
			n++
		}
	}
	return n
}

// sideTurnMsg is a side turn's outcome. Owner is the execution the message
// was typed under; Slot its queue place.
type sideTurnMsg struct {
	Input  string
	Reply  string
	Action string // /stop, /later or /converse (side_turn_action)
	Err    error
	Owner  uint64
	Slot   uint64
	// BriefNote says why the brief the message returned was not applied.
	BriefNote string
}

// submitWhileBusy handles Enter while a turn is in flight: a side turn once
// the turn runs work, a queued message before then or while a stopped turn
// winds down. A slash command is queued as a command whatever the turn is
// doing: it is not a sentence for a side turn to read.
func (m Model) submitWhileBusy() (Model, tea.Cmd) {
	raw := m.textarea.Value()
	input := strings.TrimSpace(raw)
	if input == "" {
		return m, nil
	}
	m.textarea.Reset()
	m = m.addMessage(Message{Role: "user", Content: input})
	request := m.lastRequest()
	if strings.HasPrefix(input, "/") {
		// The untrimmed tail, as idle dispatch gives it: /write and /append
		// carry its bytes as file content.
		m.turn.enqueue(queuedMessage{input: strings.TrimLeft(raw, " \t\r\n"), request: request, command: true})
		name := strings.Fields(input)[0]
		return m.pushAssistantMsg("_Queued: `" + name + "` runs as a command as soon as the current turn finishes._"), nil
	}
	if m.turn.stopping() {
		m.turn.enqueue(queuedMessage{input: input, request: request})
		return m.pushAssistantMsg("_Queued: the stopped turn is still winding down; I'll take this up as soon as it has._"), nil
	}
	if m.turn.workingOn() == "" {
		m.turn.enqueue(queuedMessage{input: input, request: request})
		return m.pushAssistantMsg("_Queued: I'll take this up as soon as the current turn finishes._"), nil
	}
	return m, m.sideTurn(input, request)
}

// sideTurn reads a message typed while work runs (see the file comment). The
// running execution and the message's queue place are captured here, on the
// Update thread, when the message is received.
func (m Model) sideTurn(input string, request uint64) tea.Cmd {
	history := make([]perception.ConversationTurn, 0, len(m.history))
	for _, h := range m.history {
		history = append(history, perception.ConversationTurn{Role: h.Role, Content: h.Content})
	}
	owner := m.turn.current()
	slot := m.turn.reserve(input, request)
	return func() tea.Msg {
		ctx := m.shutdownCtx
		if ctx == nil {
			ctx = context.Background()
		}
		var judged <-chan judgedTurn
		judged = m.startTurnJudgments(ctx, input, history)
		base := m.currentBrief().Render()
		briefCtx := types.WithSessionContext(ctx, &types.SessionContext{Brief: base})
		intent, err := m.transducer.ParseIntentWithContext(briefCtx, input, history)
		if err != nil {
			return sideTurnMsg{Input: input, Err: err, Owner: owner, Slot: slot}
		}
		action, briefNote := m.decideSideTurn(ctx, intent, judged, owner, base)
		reply := strings.TrimSpace(intent.Response)
		// A message no model read is answered with the kernel's notice for the
		// cause, not the compatibility response (perception_outcome.mg).
		if stop, _ := perception.ClassificationStopFor(m.kernel, sideTurnIntentID, intent); stop != nil {
			reply = stop.Notice
		}
		return sideTurnMsg{Input: input, Reply: reply, Action: action, Owner: owner, Slot: slot, BriefNote: briefNote}
	}
}

// sideTurnIntentID names the side turn's classification outcome
// (classification_outcome, policy/perception_outcome.mg). The running turn's
// outcome is under /current_intent and is left alone.
const sideTurnIntentID = "/side_turn"

// sideTurnPredicates are the side turn's own facts, replaced each side turn.
var sideTurnPredicates = []string{"side_turn_intent", "side_turn_judgment"}

// decideSideTurn asserts the side turn's reading and asks the kernel what to
// do with it (side_turn_action, policy/side_turns.mg). No kernel, or no
// derivation, is a reply only.
//
// Judgments are awaited first; the kernel section after that runs under
// turnState.decide, so two side turns never read each other's facts. The
// brief the message returned is published in the same section, and only
// when it still applies: the execution it was typed under still runs, the
// brief is the one the message was read against (base), and the message is
// not deferred work (/later is read again as a full turn when it runs). The
// returned note says why a brief was not applied.
func (m Model) decideSideTurn(ctx context.Context, intent perception.Intent, judged <-chan judgedTurn, owner uint64, base string) (action, briefNote string) {
	const converse = "/converse"
	if m.kernel == nil {
		return converse, ""
	}
	log := logging.Get(logging.CategoryKernel)
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

	if m.turn != nil {
		m.turn.decide.Lock()
		defer m.turn.decide.Unlock()
	}
	// What happened when a model read the message, under the side turn's own id
	// so the running turn's row stands.
	if err := perception.PublishClassificationOutcome(m.kernel, sideTurnIntentID, intent.Outcome); err != nil {
		log.Error("side turn: publish classification outcome: %v", err)
		return converse, ""
	}
	if err := config.EnsureParams(m.kernel, m.Config.TypeSafe.Params()); err != nil {
		log.Warn("side turn: thresholds not asserted: %v", err)
	}
	tx := m.kernel.Transaction()
	for _, p := range sideTurnPredicates {
		tx.Retract(p)
	}
	for _, f := range facts {
		tx.Assert(f)
	}
	if err := tx.Commit(); err != nil {
		log.Error("side turn: assert: %v", err)
		return converse, ""
	}
	action = converse
	if rows, err := m.kernel.Query("side_turn_action"); err == nil && len(rows) == 1 && len(rows[0].Args) == 1 {
		action = types.ExtractString(rows[0].Args[0])
	}

	switch {
	case intent.Brief == nil:
	case action == "/later":
		// Deferred work is read again, brief and all, when it runs.
	case owner != m.turn.current():
		briefNote = "The work this was about had already ended, so its change to the brief was not applied."
	case m.currentBrief().Render() != base:
		briefNote = "The brief changed while I read this, so this message's change to it was not applied; say it again if it still holds."
	default:
		m.writeBrief(intent.Brief)
	}
	return action, briefNote
}

// applySideTurn shows the side turn's reply and acts on the kernel's verdict,
// for the execution the message was typed under.
func (m Model) applySideTurn(msg sideTurnMsg) Model {
	if msg.Err != nil {
		// A transducer that errors instead of degrading: say why without the
		// provider's body, and keep the promise the message makes. The input
		// is queued, in the place it was received, unless the read was
		// canceled -- a shutdown is not a request to retry.
		outcome := perception.FailedOutcome(msg.Err, "", 0)
		note := "I couldn't read that while the work runs (" + outcome.SafeMessage() + ")."
		if outcome.Disposition == perception.ClassificationCanceled {
			m.turn.settle(msg.Slot, false)
			return m.pushAssistantMsg(note)
		}
		m.turn.settle(msg.Slot, true)
		return m.pushAssistantMsg(note + " It is queued for when the work finishes.")
	}
	reply := msg.Reply
	m.turn.settle(msg.Slot, msg.Action == "/later")
	switch msg.Action {
	case "/stop":
		working := m.turn.workingOn()
		if m.turn.stopExecution(msg.Owner) {
			reply = strings.TrimSpace(reply + "\n\nStopping the current work: " + working)
		} else {
			reply = strings.TrimSpace(reply + "\n\nThe work you asked to stop had already finished; nothing else was stopped.")
		}
	case "/later":
		reply = strings.TrimSpace(reply + "\n\n_I'll take this up as soon as the current work finishes._")
	}
	if msg.BriefNote != "" {
		reply = strings.TrimSpace(reply + "\n\n_" + msg.BriefNote + "_")
	}
	if reply == "" {
		reply = "Noted."
	}
	return m.pushAssistantMsg(reply)
}

// startQueuedTurn runs queued messages once nothing is in flight -- the
// loading flag clear and the slot released by the previous execution's end
// -- and nil when nothing waits or a turn still runs. A message runs as a
// full turn. A command runs through the dispatcher an idle submission uses:
// one that finishes on the spot (/help) leaves nothing in flight, so the
// next message runs after it; one that starts work holds the slot, and the
// rest wait for it.
func (m Model) startQueuedTurn() (Model, tea.Cmd) {
	var cmds []tea.Cmd
	for !m.isLoading && m.turn.current() == 0 {
		next, ok := m.turn.dequeue()
		if !ok {
			break
		}
		if next.command {
			// What the command prints on the spot answers its request.
			prev := m.answering
			m.answering = next.request
			model, cmd := m.handleCommand(next.input)
			cmds = append(cmds, cmd)
			nm, ok := model.(Model)
			if !ok {
				logging.Get(logging.CategorySession).Error("queued command %q returned %T, not the chat model", next.input, model)
				break
			}
			m = nm
			m.answering = prev
			continue
		}
		m.isLoading = true
		cmds = append(cmds, m.processInput(next.input, next.request))
	}
	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
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
