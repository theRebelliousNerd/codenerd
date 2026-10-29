package learnjudge

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/types"
	"codenerd/internal/typesafe"
)

// Final-report judgment: does the agent's own report say part of the task is
// not done, and does it name an obstacle codeNERD cannot clear by itself? The
// /finish round (session/finish_round.go, policy/turn_finish.mg) pushes an
// unfinished report back to the model unless it names such a blocker, so a
// turn no longer ends "not verified" on an admission nobody read back. It
// replaces a main-model call (24.6s in the dogfood run) with Jev (~150ms).
// Live on Jev 1.14, the unfinished question read 8 of 8 reports right; the
// blocker question read 6 of 8 right and left 2 unsettled, which push back.

// ReportInput is the task the agent was given and its final report.
type ReportInput struct {
	Task   string
	Report string
}

// ReportReading is Jev's two probabilities over one report.
type ReportReading struct {
	Unfinished float64
	Blocked    float64
}

// Question names.
const (
	ReportAdmitsUnfinished = "admits_unfinished"
	ReportNamesBlocker     = "names_blocker"
)

// reportLimit bounds the task and the report in the state; the state says how
// much was left out. bounded keeps a text's head and tail, and an admission
// sits in the closing paragraphs.
const reportLimit = 24000

// ReportRequest builds the state and both questions.
func ReportRequest(in ReportInput) (map[string]any, map[string]typesafe.Question) {
	state := map[string]any{"assistant": "codeNERD, a coding agent"}
	putBounded(state, "task", strings.TrimSpace(in.Task), reportLimit)
	putBounded(state, "report", strings.TrimSpace(in.Report), reportLimit)
	return state, map[string]typesafe.Question{
		ReportAdmitsUnfinished: typesafe.Noul{
			Instructions: "Does `report` say that part of the work `task` asks for was not done?",
			Criteria: &typesafe.NoulCriteria{
				True:  "The report says something the task asks for was skipped, left for later, not finished, or not done because of an obstacle.",
				False: "The report says the work is done, even with caveats, suggested follow-ups, or a note that some extra check could not run.",
			},
		},
		ReportNamesBlocker: blockerQuestion("report"),
	}
}

// blockerQuestion asks whether the text at the state path field names an
// obstacle codeNERD cannot clear by itself. The final-report reading and the
// request-delivery reading ask it in the same words.
func blockerQuestion(field string) typesafe.Question {
	return typesafe.Noul{
		Instructions: fmt.Sprintf("Does `%s` name an obstacle to finishing that codeNERD cannot clear by itself?", field),
		Criteria: &typesafe.NoulCriteria{
			True:  "A permission or policy denial, a missing credential or input, or a choice only the user can make.",
			False: "No such obstacle is named: the work is done, or what is left is work codeNERD could still do, even if it believes otherwise.",
		},
	}
}

// JudgeReport asks both questions in one request. A failed call is an error,
// never a reading.
func JudgeReport(ctx context.Context, a Asker, in ReportInput) (ReportReading, error) {
	if a == nil {
		return ReportReading{}, fmt.Errorf("learnjudge: no Jev client")
	}
	if strings.TrimSpace(in.Report) == "" {
		return ReportReading{}, fmt.Errorf("learnjudge: no report to read")
	}
	state, questions := ReportRequest(in)
	resp, err := a.Ask(typesafe.WithOperation(ctx, OpFinalReport), state, questions)
	if err != nil {
		return ReportReading{}, err
	}
	unfinished, err := resp.Noul(ReportAdmitsUnfinished)
	if err != nil {
		return ReportReading{}, err
	}
	blocked, err := resp.Noul(ReportNamesBlocker)
	if err != nil {
		return ReportReading{}, err
	}
	return ReportReading{Unfinished: unfinished, Blocked: blocked}, nil
}

// ReportReadingPredicate is turn_report_reading(Turn, Unfinished, Blocked),
// both as whole percents (policy/turn_finish.mg).
const ReportReadingPredicate = "turn_report_reading"

// ReportReadingFact projects a reading for one turn.
func ReportReadingFact(turn any, r ReportReading) types.Fact {
	return types.Fact{Predicate: ReportReadingPredicate, Args: []any{turn, types.PercentFromRatio(r.Unfinished), types.PercentFromRatio(r.Blocked)}}
}
