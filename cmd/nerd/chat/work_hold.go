package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// pendingWork is work the kernel held (route_decision(/hold)) until the user
// says go: the request as typed, the intent perception read from it, and the
// lane the kernel would have run. A brief-goal run has no saved intent; the
// goal goes through perception like any request.
type pendingWork struct {
	Deliberated    bool
	ResolutionNote string
	Input          string
	Intent         perception.Intent
	Route          RouteDecision
	At             time.Time
}

// holdMsg carries held work from the turn back to the model, which keeps it
// as pendingWork and shows the surface. The conversation stays open.
type holdMsg struct {
	Work    *pendingWork
	Surface string
}

// approveWorkMsg is the kernel's /approve_pending lane: run the held work.
type approveWorkMsg struct{}

// holdSurface is the main agent's reply, then what is held. In propose mode
// with a ready brief it is a proposal; otherwise it says the work is held and
// why, so the user knows nothing has started.
func holdSurface(intent perception.Intent, route RouteDecision, mode config.WorkMode, brief *perception.Brief) string {
	var b strings.Builder
	if r := strings.TrimSpace(intent.Response); r != "" {
		b.WriteString(r)
		b.WriteString("\n\n")
	}
	open := brief != nil && len(brief.OpenQuestions) > 0
	switch {
	case mode == config.WorkModeExplicit:
		b.WriteString("_Held:_ ")
	case open:
		b.WriteString("**Waiting on:** " + strings.Join(brief.OpenQuestions, "; ") + "\n\n**Held:** ")
	default:
		b.WriteString("**Proposed:** ")
	}
	switch {
	case len(route.Steps) > 0:
		fmt.Fprintf(&b, "%d steps\n", len(route.Steps))
		for i, s := range route.Steps {
			shard := strings.TrimPrefix(s.ShardType, "/")
			if shard == "" {
				shard = "?"
			}
			fmt.Fprintf(&b, "%d. %s (%s)\n", i+1, strings.TrimSpace(s.Task), shard)
		}
	case route.Shard != "":
		fmt.Fprintf(&b, "the %s shard: %s %s\n", route.Shard, strings.TrimPrefix(intent.Verb, "/"), strings.TrimSpace(intent.Target))
	default:
		fmt.Fprintf(&b, "%s %s\n", strings.TrimPrefix(intent.Verb, "/"), strings.TrimSpace(intent.Target))
	}
	b.WriteString("\nNothing has started. Say **go** (or `/go`) to start it, or keep talking to change it.")
	return b.String()
}

// approvePendingWork starts the held work with the user's go-ahead on the
// turn. With nothing held, a brief with a goal is the work: big runs start
// from the brief.
func (m Model) approvePendingWork() (Model, tea.Cmd) {
	// "/go" typed into the box is spent once it is read; left there it sat in
	// the input for the whole run.
	m.textarea.Reset()
	work := m.pendingWork
	if work == nil {
		if brief := m.currentBrief(); brief != nil && strings.TrimSpace(brief.Goal) != "" {
			work = &pendingWork{Input: brief.Goal, At: time.Now()}
		}
	}
	if work == nil {
		return m.pushAssistantMsg("Nothing is waiting for a go-ahead, and the brief has no goal yet."), nil
	}
	m.pendingWork = nil
	m.goAheadWork = work
	// The held work answers the request that approved it ("go"); /go typed
	// as a command is no request, and m.answering is 0.
	cmd := m.processInput(work.Input, m.answering)
	m.goAheadWork = nil
	m.isLoading = true
	return m, cmd
}

// briefContext carries the running brief to the perception call only: the
// rest of the turn keeps its own context, since the executor treats any
// session context as already built.
func (m Model) briefContext(ctx context.Context) context.Context {
	return types.WithSessionContext(ctx, &types.SessionContext{Brief: m.currentBrief().Render()})
}

// currentBrief reads the brief back from the kernel; nil when there is none.
func (m Model) currentBrief() *perception.Brief {
	if m.kernel == nil {
		return nil
	}
	var facts []types.Fact
	for _, pred := range perception.BriefPredicates {
		rows, err := m.kernel.Query(pred)
		if err != nil {
			logging.Get(logging.CategoryKernel).Warn("brief: query %s: %v", pred, err)
			continue
		}
		facts = append(facts, rows...)
	}
	return perception.BriefFromFacts(facts)
}

// recordBrief replaces the kernel's brief with the one perception returned.
// Nil (perception returned none) keeps the brief as it was. It serializes
// with side turns' kernel sections (turnState.decide).
func (m Model) recordBrief(brief *perception.Brief) {
	if m.kernel == nil || brief == nil {
		return
	}
	if m.turn != nil {
		m.turn.decide.Lock()
		defer m.turn.decide.Unlock()
	}
	m.writeBrief(brief)
}

// writeBrief swaps the brief in one transaction, so a reader sees the old
// brief or the new one, never a brief half retracted. The caller holds
// turnState.decide.
func (m Model) writeBrief(brief *perception.Brief) {
	tx := m.kernel.Transaction()
	for _, pred := range perception.BriefPredicates {
		tx.Retract(pred)
	}
	for _, f := range brief.Facts() {
		tx.Assert(f)
	}
	if err := tx.Commit(); err != nil {
		logging.Get(logging.CategoryKernel).Error("brief: replace: %v", err)
	}
}

// syncWorkModeFacts mirrors the work mode into the kernel: work_mode/1, the
// admission gate's input (routing_arbitration.mg), and yolo_mode(), which the
// clarification and approval rules consult. Config is the source of truth;
// the facts are derived session state. A nil kernel (pre-boot toggle) is
// fine: the boot hook re-syncs once the kernel exists.
func (m Model) syncWorkModeFacts() {
	// Yolo approves its own approval prompts (approval.Broker.SetAutonomous).
	m.approvals.SetAutonomous(m.yoloEnabled())
	if m.kernel == nil {
		return
	}
	log := logging.Get(logging.CategoryKernel)
	if err := m.kernel.Retract("work_mode"); err != nil {
		log.Error("work mode: retract work_mode: %v", err)
	}
	if err := m.kernel.Assert(core.Fact{Predicate: "work_mode", Args: []any{types.MangleAtom("/" + string(m.workMode()))}}); err != nil {
		log.Error("work mode: assert work_mode: %v", err)
	}
	if m.yoloEnabled() {
		if err := m.kernel.Assert(core.Fact{Predicate: "yolo_mode"}); err != nil {
			log.Error("work mode: assert yolo_mode: %v", err)
		}
		return
	}
	if err := m.kernel.Retract("yolo_mode"); err != nil {
		log.Error("work mode: retract yolo_mode: %v", err)
	}
}
