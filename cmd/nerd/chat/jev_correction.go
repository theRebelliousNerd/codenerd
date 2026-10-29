package chat

import (
	"context"
	"strings"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/learnjudge"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// The learning-loop trigger (policy/jev_learning.mg). Jev is asked, beside
// perception, whether the latest message tells codeNERD its last reply or
// action was wrong (learnjudge.JudgeCorrection); the rebuke-phrase reading
// (perception.IsRebuke) is asserted beside it as fallback evidence. The kernel
// derives learning_correction, and only then is the correction learned from,
// beside the turn (learning_auto.go). Until 2026-10-28 handleSubmit matched keywords before
// perception, so "stop" or "wrong" in a short message was a correction.

// judgedCorrection is the outcome of one turn's correction question. asked is
// false when there was no last reply to correct.
type judgedCorrection struct {
	p     float64
	asked bool
	err   error
	took  time.Duration
}

// startCorrectionJudgment asks Jev about the turn in the background and
// returns the channel its outcome arrives on; nil when Jev or the
// correction_judgments switch is off. Routing waits for it under the same
// bound as the turn judgments (turn_judgment_timeout).
func (m Model) startCorrectionJudgment(ctx context.Context, input string) <-chan judgedCorrection {
	uses := m.Config.TypeSafe.Uses()
	if m.jev == nil || !uses.CorrectionJudgments {
		return nil
	}
	in := lastExchange(m.history, input)
	out := make(chan judgedCorrection, 1)
	jctx, cancel := context.WithTimeout(m.jevContext(ctx), uses.TurnJudgmentTimeout)
	go func() {
		defer cancel()
		began := time.Now()
		p, asked, err := learnjudge.JudgeCorrection(jctx, m.jev, in)
		out <- judgedCorrection{p: p, asked: asked, err: err, took: time.Since(began)}
	}()
	return out
}

// lastExchange is codeNERD's last reply and the request it answered, from the
// history the current message has already joined.
func lastExchange(history []Message, input string) learnjudge.CorrectionInput {
	in := learnjudge.CorrectionInput{Message: input}
	end := len(history)
	if end > 0 && history[end-1].Role == "user" && history[end-1].Content == input {
		end--
	}
	reply := -1
	for i := end - 1; i >= 0; i-- {
		if history[i].Role == "assistant" && strings.TrimSpace(history[i].Content) != "" {
			reply = i
			break
		}
	}
	if reply < 0 {
		return in
	}
	in.LastReply = history[reply].Content
	for i := reply - 1; i >= 0; i-- {
		if history[i].Role == "user" {
			in.PreviousRequest = history[i].Content
			break
		}
	}
	return in
}

// recordCorrection replaces last turn's correction evidence with this turn's:
// the rebuke-phrase reading, then Jev's judgment when it arrives in time. It
// returns the kernel's decision (learning_correction) and its source.
func (m Model) recordCorrection(ctx context.Context, input string, judged <-chan judgedCorrection) (string, bool) {
	if m.kernel == nil {
		return "", false
	}
	for _, pred := range learnjudge.CorrectionPredicates {
		_ = m.kernel.Retract(pred)
	}
	log := logging.Get(logging.CategoryPerception)
	if strings.TrimSpace(lastExchange(m.history, input).LastReply) == "" {
		// Nothing has been said yet, so there is nothing to correct.
		return "", false
	}
	if perception.IsRebuke(input) {
		if err := m.kernel.Assert(core.Fact{Predicate: learnjudge.RebukePhrasePredicate}); err != nil {
			log.Error("correction: assert rebuke phrase: %v", err)
		}
	}
	switch {
	case judged == nil:
		if m.jev != nil {
			log.Info("correction: correction_judgments is off; the rebuke-phrase reading stands")
		}
	default:
		select {
		case j := <-judged:
			m.assertCorrection(j)
		case <-ctx.Done():
			log.Warn("correction: turn ended before Jev's correction judgment arrived: %v", ctx.Err())
		}
	}
	return m.learningCorrection()
}

// assertCorrection records Jev's judgment, or logs why there is none; with
// none, the kernel falls back to the rebuke-phrase reading.
func (m Model) assertCorrection(j judgedCorrection) {
	log := logging.Get(logging.CategoryPerception)
	switch {
	case j.err != nil:
		log.Warn("correction: no Jev judgment (call %s), the rebuke-phrase reading stands: %v", j.took.Round(time.Millisecond), j.err)
		return
	case !j.asked:
		return
	}
	if err := config.EnsureParams(m.kernel, m.Config.TypeSafe.Params()); err != nil {
		log.Warn("correction: thresholds not asserted, judgment dropped: %v", err)
		return
	}
	if err := m.kernel.Assert(learnjudge.CorrectionFact(j.p)); err != nil {
		log.Error("correction: assert judgment: %v", err)
		return
	}
	log.Info("correction: Jev %s=%d%% in %s", learnjudge.CorrectsLastTurn, types.PercentFromRatio(j.p), j.took.Round(time.Millisecond))
}

// learningCorrection is the kernel's decision for this turn.
func (m Model) learningCorrection() (string, bool) {
	rows, err := m.kernel.Query("learning_correction")
	if err != nil {
		logging.Get(logging.CategoryPerception).Warn("correction: query learning_correction: %v", err)
		return "", false
	}
	for _, r := range rows {
		if len(r.Args) == 1 {
			return strings.TrimPrefix(types.ExtractString(r.Args[0]), "/"), true
		}
	}
	return "", false
}
