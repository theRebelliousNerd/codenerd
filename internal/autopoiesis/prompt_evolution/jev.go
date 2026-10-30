package prompt_evolution

import (
	"context"
	"time"

	nerdconfig "codenerd/internal/config"
	"codenerd/internal/learnjudge"
	"codenerd/internal/logging"
)

// Jev (TypeSafe System One) in prompt evolution: the task verdict
// (jev_verdict.go), the problem type (classifier.go) and the shared-cause check
// on failure groups (jev_evolution.go). Jev supplies observations; the
// decisions are truth tables in this package. A failed, timed-out or
// switched-off Jev is never a "no": each use falls back as its file says and
// logs it (Docs/architecture/jev-turn-judgments.md, "Learning and
// autopoiesis").

// ProblemOther is a task Jev reads as none of the problem kinds (small talk, a
// question about something else). It groups with nothing a strategy serves.
const ProblemOther ProblemType = "other"

// CategoryNoAgentMistake is a failure Jev reads as no mistake of the agent's
// (a timeout, an outage, missing access): the kernel saw the turn fail, but no
// guidance to the agent would have prevented it, so evolution never learns
// from it.
const CategoryNoAgentMistake ErrorCategory = "NO_AGENT_MISTAKE"

// jevUse is Jev as the evolver uses it: the client, which uses are on, and the
// yes/no thresholds as probabilities.
type jevUse struct {
	asker learnjudge.Asker
	// A new attachment is a conservative model revision boundary, including
	// aliases whose backing model cannot be resolved without a purchase.
	screenEpoch string
	screenModel string
	uses        nerdconfig.TypeSafeUses
	yes, no     float64
	// problemTimeout bounds a problem-type call on the prompt compile path
	// (turn_judgment_timeout): a strategy lookup must not hold a turn longer
	// than the turn's own judgments may.
	problemTimeout time.Duration
}

// newJevUse resolves the typesafe section for the evolver; nil when Jev is not
// configured.
func newJevUse(a learnjudge.Asker, cfg *nerdconfig.TypeSafeConfig) *jevUse {
	if a == nil || cfg == nil {
		return nil
	}
	u := &jevUse{asker: a, uses: cfg.Uses(), screenEpoch: newOwnerToken(), screenModel: cfg.Model}
	for _, p := range cfg.Params() {
		switch p.Key {
		case "/jev_yes_percent":
			u.yes = float64(p.Value) / 100
		case "/jev_no_percent":
			u.no = float64(p.Value) / 100
		}
	}
	u.problemTimeout = u.uses.TurnJudgmentTimeout
	return u
}

// reading is one probability against the thresholds.
type reading int

const (
	readUnsettled reading = iota
	readYes
	readNo
)

func (u *jevUse) read(p float64) reading {
	switch {
	case p >= u.yes:
		return readYes
	case p <= u.no:
		return readNo
	default:
		return readUnsettled
	}
}

// SetJev gives the evolver Jev. Boot calls it once, before any cycle; a nil
// client or section leaves every use on its fallback.
func (pe *PromptEvolver) SetJev(a learnjudge.Asker, cfg *nerdconfig.TypeSafeConfig) {
	u := newJevUse(a, cfg)
	pe.mu.Lock()
	pe.jev = u
	pe.mu.Unlock()
	if pe.classifier != nil {
		pe.classifier.setJev(u)
	}
	if u != nil {
		logging.Autopoiesis("Prompt evolution: Jev task_verdicts=%t problem_types=%t", u.uses.TaskVerdicts, u.uses.ProblemTypes)
	}
}

func (pe *PromptEvolver) jevFor() *jevUse {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	return pe.jev
}

// withJevBound bounds one Jev call by the client's own request timeout; ctx
// already carries the cycle's.
func withJevBound(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, d)
}
