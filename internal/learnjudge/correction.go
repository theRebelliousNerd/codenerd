package learnjudge

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/types"
	"codenerd/internal/typesafe"
)

// Correction judgment: is the user correcting something codeNERD did or said
// in its last turn? It replaces the keyword match that sent any short message
// with "wrong" or "mistake" into the learning loop. The kernel decides
// (policy/jev_learning.mg, learning_correction): the learning loop runs only
// when it derives a correction.

// CorrectionInput is the last exchange and the new message. PreviousRequest is
// the user message codeNERD's last turn answered; LastReply is that answer.
type CorrectionInput struct {
	PreviousRequest string
	LastReply       string
	Message         string
}

// correctionReplyLimit bounds codeNERD's last reply in the state, to fit Jev's
// per-question context; the state says how much was left out.
const correctionReplyLimit = 24000

// CorrectsLastTurn is the question's name and the judgment's name in the
// kernel (correction_judgment).
const CorrectsLastTurn = "corrects_last_turn"

// CorrectionRequest builds the state and the question. Only fields with
// content go in the state.
func CorrectionRequest(in CorrectionInput) (map[string]any, map[string]typesafe.Question) {
	state := map[string]any{
		"assistant": "codeNERD, a coding agent working in the user's repository",
		"message":   in.Message,
	}
	if s := strings.TrimSpace(in.PreviousRequest); s != "" {
		state["previous_request"] = s
	}
	putBounded(state, "codenerd_reply", strings.TrimSpace(in.LastReply), correctionReplyLimit)
	return state, map[string]typesafe.Question{CorrectsLastTurn: correctionQuestion()}
}

func correctionQuestion() typesafe.Question {
	return typesafe.Noul{
		Instructions: "In `message`, is the user telling codeNERD that something in `codenerd_reply` was wrong?",
		Criteria: &typesafe.NoulCriteria{
			True:  "The user says codeNERD's last answer or action was mistaken: it misread the request, changed the wrong thing, gave a wrong answer, or did something other than what was asked.",
			False: "The user asks for new work, reports a bug in their own code or tests, asks codeNERD to stop running work, asks a question, or accepts the reply.",
		},
	}
}

// JudgeCorrection asks whether the message corrects codeNERD's last turn and
// returns the probability that it does. With no last reply there is nothing to
// correct, and it asks nothing: (0, false, nil). A failed call is an error,
// never a "no".
func JudgeCorrection(ctx context.Context, a Asker, in CorrectionInput) (p float64, asked bool, err error) {
	if a == nil {
		return 0, false, fmt.Errorf("learnjudge: no Jev client")
	}
	if strings.TrimSpace(in.LastReply) == "" || strings.TrimSpace(in.Message) == "" {
		return 0, false, nil
	}
	state, questions := CorrectionRequest(in)
	resp, err := a.Ask(typesafe.WithOperation(ctx, OpCorrection), state, questions)
	if err != nil {
		return 0, false, err
	}
	p, err = resp.Noul(CorrectsLastTurn)
	if err != nil {
		return 0, false, err
	}
	return p, true, nil
}

// Correction facts (policy/jev_learning.mg).
const (
	// CorrectionJudgmentPredicate is correction_judgment(Percent): Jev's
	// probability, as a whole percent, that the latest message corrects
	// codeNERD's last turn.
	CorrectionJudgmentPredicate = "correction_judgment"
	// RebukePhrasePredicate is correction_rebuke_phrase(): the message holds a
	// rebuke phrase (perception.IsRebuke). The kernel reads it only when Jev
	// gave no judgment this turn: Jev off, switched off, failed or timed out.
	RebukePhrasePredicate = "correction_rebuke_phrase"
	// LearningCorrectionPredicate is learning_correction(Source): the kernel's
	// decision that the turn is a correction to learn from, and what said so
	// (/jev or /rebuke_phrase).
	LearningCorrectionPredicate = "learning_correction"
)

// CorrectionPredicates are the per-turn facts, retracted before each turn's
// evidence is asserted.
var CorrectionPredicates = []string{CorrectionJudgmentPredicate, RebukePhrasePredicate}

// CorrectionFact projects a judgment.
func CorrectionFact(p float64) types.Fact {
	return types.Fact{Predicate: CorrectionJudgmentPredicate, Args: []any{types.PercentFromRatio(p)}}
}
