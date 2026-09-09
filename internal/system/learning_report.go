package system

import (
	"fmt"
	"strings"

	"codenerd/internal/types"
)

// LearningReport is the answer to the only question that matters about a
// self-improving system: is it actually improving?
//
// It is computed from the turn_cost facts the session executor has been
// asserting since long before anything read them. Those facts exist for
// exactly this — the schema calls turn_cost "the per-turn cost denominator for
// tokens-per-verified-work" — and nothing in the tree ever divided by it.
//
// Unlike every other per-turn fact, turn_cost is deliberately not retracted at
// the end of a turn. That is what makes the aggregate possible: the kernel
// holds the whole session's cost history.
type LearningReport struct {
	// Turns is every turn the kernel has cost facts for.
	Turns int

	// Verified, Hollow, Failed and Unverified partition Turns by the kernel's
	// verdict. Their meanings are the ones that matter for judging the system:
	// Hollow is work the agent CLAIMED to have done, which is the number that
	// should trend to zero, and Unverified is mostly questions answered.
	Verified   int
	Hollow     int
	Failed     int
	Unverified int

	// PromptTokens and CompletionTokens are the whole session's spend.
	PromptTokens     int64
	CompletionTokens int64

	// ToolCalls is the total across all turns.
	ToolCalls int64
}

// TokensPerVerifiedTurn is the headline number: what one piece of verified
// work costs.
//
// It returns 0 when nothing has been verified — which is itself the answer,
// and a more honest one than an infinity or a divide-by-zero panic. A session
// that spent a million tokens and verified nothing has no cost per unit of
// work because it produced no units of work.
func (r LearningReport) TokensPerVerifiedTurn() int64 {
	if r.Verified == 0 {
		return 0
	}
	return (r.PromptTokens + r.CompletionTokens) / int64(r.Verified)
}

// HollowRate is the share of turns that claimed success without evidence,
// 0.0-1.0. It is the single best measure of whether the agent is getting
// better or just getting more confident.
func (r LearningReport) HollowRate() float64 {
	if r.Turns == 0 {
		return 0
	}
	return float64(r.Hollow) / float64(r.Turns)
}

// String renders the report as one human-readable block.
func (r LearningReport) String() string {
	if r.Turns == 0 {
		return "no turns recorded yet"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d turns: %d verified, %d hollow, %d failed, %d unverified\n",
		r.Turns, r.Verified, r.Hollow, r.Failed, r.Unverified)
	fmt.Fprintf(&sb, "  spend:  %d prompt + %d completion tokens, %d tool calls\n",
		r.PromptTokens, r.CompletionTokens, r.ToolCalls)
	if r.Verified > 0 {
		fmt.Fprintf(&sb, "  cost:   %d tokens per verified turn\n", r.TokensPerVerifiedTurn())
	} else {
		fmt.Fprintf(&sb, "  cost:   no verified work yet, so no cost per unit of work\n")
	}
	fmt.Fprintf(&sb, "  hollow: %.1f%% of turns claimed success without evidence\n", r.HollowRate()*100)
	return sb.String()
}

// LearningReportFrom aggregates turn_cost facts into a report.
//
// Fact shape: turn_cost(SessionID, TurnNum, PromptTokens, CompletionTokens,
// ToolCalls, VerifiedOutcome). A fact with the wrong arity is skipped rather
// than guessed at — a miscounted denominator is worse than a missing one.
func LearningReportFrom(facts []types.Fact) LearningReport {
	var r LearningReport
	for _, fact := range facts {
		if len(fact.Args) != 6 {
			continue
		}
		r.Turns++
		r.PromptTokens += factInt64(fact.Args[2])
		r.CompletionTokens += factInt64(fact.Args[3])
		r.ToolCalls += factInt64(fact.Args[4])

		switch factAtom(fact.Args[5]) {
		case "/done":
			r.Verified++
		case "/hollow":
			r.Hollow++
		case "/failed":
			r.Failed++
		default:
			r.Unverified++
		}
	}
	return r
}

// LearningReport aggregates this Cortex's turn costs. An absent kernel or a
// failed query yields an empty report, which renders as "no turns recorded
// yet" rather than an error: this is a diagnostic, and a diagnostic that
// fails a command is a worse diagnostic.
func (c *Cortex) LearningReport() LearningReport {
	if c == nil || c.Kernel == nil {
		return LearningReport{}
	}
	facts, err := c.Kernel.Query("turn_cost")
	if err != nil {
		return LearningReport{}
	}
	return LearningReportFrom(facts)
}

// factInt64 coerces a Mangle numeric argument. Mangle numbers arrive as int64
// in this fork, but facts also reach the kernel from Go call sites that build
// them with int or float64, so all three are accepted.
func factInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	}
	return 0
}

// factAtom renders a Mangle name constant as a string, accepting both the
// typed atom and a bare string spelling.
func factAtom(v any) string {
	switch a := v.(type) {
	case types.MangleAtom:
		return string(a)
	case string:
		return a
	}
	return ""
}
