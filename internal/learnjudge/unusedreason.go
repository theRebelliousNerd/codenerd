package learnjudge

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/types"
	"codenerd/internal/typesafe"
)

// Unused-symbol reason judgment: the /unwired round (session/unwired_round.go,
// policy/turn_unwired.mg) pushes back a turn whose new production function no
// production path reaches, until it is wired or the model says why it is meant
// to have no caller. Jev reads that reason: does it name a caller that exists
// today outside the code the index reads, and does the user's request ask for
// the function without asking for it to run? The kernel accepts the reason on
// either; "for later", "kept just in case" and "I ran it once from a test" are
// neither.

// OpUnusedReason is the usage tracker's operation for this use.
const OpUnusedReason = "jev_unused_reason"

// UnusedReasonInput is one unwired function and the model's reason for it.
type UnusedReasonInput struct {
	Task      string
	Ref       string
	Signature string
	Doc       string
	Reason    string
}

// UnusedReasonReading is Jev's two probabilities over one reason.
type UnusedReasonReading struct {
	CurrentCaller float64
	TaskAsks      float64
}

// Question names.
const (
	UnusedNamesCurrentCaller = "names_current_caller"
	UnusedTaskAsksUncalled   = "task_asks_uncalled"
)

// UnusedReasonRequest builds the state and both questions.
func UnusedReasonRequest(in UnusedReasonInput) (map[string]any, map[string]typesafe.Question) {
	state := map[string]any{"assistant": "codeNERD, a coding agent"}
	putBounded(state, "task", strings.TrimSpace(in.Task), reportLimit)
	fn := map[string]any{"ref": in.Ref}
	if s := strings.TrimSpace(in.Signature); s != "" {
		fn["signature"] = s
	}
	if d := strings.TrimSpace(in.Doc); d != "" {
		fn["doc"] = d
	}
	state["function"] = fn
	state["facts"] = "No Go code in the repository outside tests calls or references `function`. `reason` is the agent's explanation of why that is intended."
	putBounded(state, "reason", strings.TrimSpace(in.Reason), reportLimit)
	return state, map[string]typesafe.Question{
		UnusedNamesCurrentCaller: typesafe.Noul{
			Instructions: fmt.Sprintf("Does `reason` name something that calls or loads the function %s today?", in.Ref),
			Criteria: &typesafe.NoulCriteria{
				True:  "The reason names a caller that exists now outside this repository's Go code, such as another module that imports it, a plugin loader, a lookup by name through reflection, or a code generator.",
				False: "The reason names no caller that exists now: the function is for later work, kept in case it is needed, was run once by hand or from a test, or waits for someone else to call it.",
			},
		},
		UnusedTaskAsksUncalled: typesafe.Noul{
			Instructions: fmt.Sprintf("Does `task` ask for the function %s to be written without asking for the program to call it?", in.Ref),
			Criteria: &typesafe.NoulCriteria{
				True:  "The request asks only for the function or a library helper, or says not to wire it in yet.",
				False: "The request asks the program to do something, or to fix something, which happens only if the function runs.",
			},
		},
	}
}

// JudgeUnusedReason asks both questions in one request. A failed call is an
// error, never a reading.
func JudgeUnusedReason(ctx context.Context, a Asker, in UnusedReasonInput) (UnusedReasonReading, error) {
	if a == nil {
		return UnusedReasonReading{}, fmt.Errorf("learnjudge: no Jev client")
	}
	if strings.TrimSpace(in.Reason) == "" {
		return UnusedReasonReading{}, fmt.Errorf("learnjudge: no reason to read")
	}
	state, questions := UnusedReasonRequest(in)
	resp, err := a.Ask(typesafe.WithOperation(ctx, OpUnusedReason), state, questions)
	if err != nil {
		return UnusedReasonReading{}, err
	}
	caller, err := resp.Noul(UnusedNamesCurrentCaller)
	if err != nil {
		return UnusedReasonReading{}, err
	}
	asks, err := resp.Noul(UnusedTaskAsksUncalled)
	if err != nil {
		return UnusedReasonReading{}, err
	}
	return UnusedReasonReading{CurrentCaller: caller, TaskAsks: asks}, nil
}

// UnusedReasonPredicate is turn_unused_reason_reading(Turn, Ref, Caller,
// TaskAsks), both as whole percents (policy/turn_unwired.mg).
const UnusedReasonPredicate = "turn_unused_reason_reading"

// UnusedReasonFact projects a reading for one function of one turn.
func UnusedReasonFact(turn any, ref string, r UnusedReasonReading) types.Fact {
	return types.Fact{Predicate: UnusedReasonPredicate, Args: []any{turn, types.MangleString(ref), types.PercentFromRatio(r.CurrentCaller), types.PercentFromRatio(r.TaskAsks)}}
}
