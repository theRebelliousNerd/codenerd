package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/turnjudge"
)

// Jev judgments for a chat turn (internal/turnjudge, policy/jev_evidence.mg).
// They are asked in parallel with perception, so they cost no latency, and
// land in the kernel before routing as evidence the rules may use to ask
// first, recognise a go-ahead, close an answered brief question, start the
// self-diagnosis chain, or stop background work. With Jev off, or its call
// failed, the turn routes exactly as it would have without it.

// judgedTurn is the outcome of one turn's judgments.
type judgedTurn struct {
	result *turnjudge.Result
	err    error
	took   time.Duration
}

// startTurnJudgments asks Jev about the turn in the background and returns
// the channel its outcome arrives on; nil when Jev is off.
func (m Model) startTurnJudgments(ctx context.Context, input string, history []perception.ConversationTurn) <-chan judgedTurn {
	if m.jev == nil {
		return nil
	}
	in := turnjudge.Input{Message: input, Brief: m.currentBrief(), HeldWork: describeHeldWork(m.pendingWork)}
	for _, h := range history {
		if strings.TrimSpace(h.Content) == "" {
			continue
		}
		in.Recent = append(in.Recent, turnjudge.Turn{From: h.Role, Text: h.Content})
	}
	out := make(chan judgedTurn, 1)
	go func() {
		began := time.Now()
		res, err := turnjudge.Judge(ctx, m.jev, in)
		out <- judgedTurn{result: res, err: err, took: time.Since(began)}
	}()
	return out
}

// recordTurnJudgments waits for the judgments and replaces last turn's
// evidence with them. A failure leaves no evidence, never a guess.
func (m Model) recordTurnJudgments(ctx context.Context, judged <-chan judgedTurn) {
	if m.kernel == nil {
		return
	}
	for _, pred := range turnjudge.Predicates {
		_ = m.kernel.Retract(pred)
	}
	if judged == nil {
		return
	}
	var j judgedTurn
	select {
	case j = <-judged:
	case <-ctx.Done():
		logging.Get(logging.CategoryPerception).Warn("jev: turn ended before the judgments arrived: %v", ctx.Err())
		return
	}
	log := logging.Get(logging.CategoryPerception)
	if j.err != nil {
		log.Warn("jev: no evidence this turn (%s): %v", j.took.Round(time.Millisecond), j.err)
		return
	}
	if err := config.EnsureParams(m.kernel, m.Config.TypeSafe.Params()); err != nil {
		log.Warn("jev: thresholds not asserted, evidence dropped: %v", err)
		return
	}
	if err := m.kernel.AssertBatch(j.result.Facts()); err != nil {
		log.Error("jev: assert judgments: %v", err)
		return
	}
	log.Info("jev: %s in %s", j.result.Summary(), j.took.Round(time.Millisecond))
}

// describeHeldWork is the held work as a person would read it.
func describeHeldWork(w *pendingWork) string {
	if w == nil {
		return ""
	}
	var b strings.Builder
	if verb := strings.TrimPrefix(w.Intent.Verb, "/"); verb != "" {
		fmt.Fprintf(&b, "%s %s", verb, strings.TrimSpace(w.Intent.Target))
		if r := strings.TrimSpace(w.Route.Shard); r != "" {
			fmt.Fprintf(&b, " (by the %s)", r)
		}
		b.WriteString(", requested as: ")
	}
	b.WriteString(strings.TrimSpace(w.Input))
	return b.String()
}

// stopRequestedBackground cancels running background work when the kernel
// derives that the user asked to stop it (stop_background_requested,
// jev_evidence.mg). It returns the jobs it stopped.
func (m Model) stopRequestedBackground() []backgroundJob {
	if m.kernel == nil {
		return nil
	}
	rows, err := m.kernel.Query("stop_background_requested")
	if err != nil || len(rows) == 0 {
		return nil
	}
	return m.background.cancelAll()
}
