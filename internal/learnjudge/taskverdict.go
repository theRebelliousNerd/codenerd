package learnjudge

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/typesafe"
)

// Task verdict: how one past task went, for prompt evolution. It replaces the
// LLM judge's prose verdict with observations; the verdict itself is a truth
// table in prompt_evolution (jev_verdict.go). Facts code already knows (the
// success flag, build errors, test counts, which files were changed) are given
// to the questions as state, not asked.

// TaskAction is one recorded action of the agent.
type TaskAction struct {
	Type, Description, Target string
	Failed                    bool
}

// TaskRecord is one execution as a person reviewing it would read it.
type TaskRecord struct {
	Request      string
	Shard        string
	Actions      []TaskAction
	Succeeded    bool
	TestsPassed  int
	TestsFailed  int
	BuildErrors  []string
	Output       string
	ChangedFiles []string
}

// TaskJudgments are Jev's answers for one record. FileNeeded holds, per
// changed file in order, the probability that the request calls for changing
// it. Category is the failure kind Jev picks; CategoryFits is the companion:
// the probability that any listed kind describes what went wrong.
type TaskJudgments struct {
	DidWhatWasAsked float64
	StoppedEarly    float64
	FileNeeded      []float64
	Category        string
	CategoryProbs   map[string]float64
	CategoryFits    float64
	Model           string
}

// Question names.
const (
	DidWhatWasAsked = "did_what_was_asked"
	StoppedEarly    = "stopped_early"
	FailureKind     = "failure_kind"
	FailureKindFits = "failure_kind_fits"
)

// FailureKinds are the Choice options, named for what they mean; prompt
// evolution maps each to its ErrorCategory.
var FailureKinds = map[string]any{
	"logic_error":      "The approach or algorithm is wrong: the code runs but computes or does the wrong thing.",
	"syntax_error":     "The code does not parse or compile.",
	"api_misuse":       "A library, function, command or tool was called the wrong way or with the wrong arguments.",
	"edge_case":        "The main case works, but an empty, missing, boundary or error case is not handled.",
	"context_miss":     "The agent did not read or use code, files or facts that the task depended on.",
	"instruction_miss": "The agent did not do what the request said: the wrong target, the wrong scope, or a step it was told to do skipped.",
	"hallucination":    "The agent relied on a file, function, API or fact that does not exist.",
}

// Bounds that keep one record within Jev's per-question context; the state
// says what was left out.
const (
	taskOutputLimit  = 12000
	taskActionsLimit = 200
)

// TaskVerdictRequest builds the state and the questions for one record.
func TaskVerdictRequest(rec TaskRecord) (map[string]any, map[string]typesafe.Question) {
	state := map[string]any{
		"request": rec.Request,
		"result": map[string]any{
			"reported_success": rec.Succeeded,
		},
	}
	if s := strings.TrimSpace(rec.Shard); s != "" {
		state["agent"] = strings.TrimPrefix(s, "/")
	}
	result := state["result"].(map[string]any)
	if rec.TestsPassed > 0 || rec.TestsFailed > 0 {
		result["tests_passed"], result["tests_failed"] = rec.TestsPassed, rec.TestsFailed
	}
	if len(rec.BuildErrors) > 0 {
		result["build_errors"] = rec.BuildErrors
	}
	if out := strings.TrimSpace(rec.Output); out != "" {
		putBounded(result, "final_output", out, taskOutputLimit)
	}
	if len(rec.ChangedFiles) > 0 {
		result["changed_files"] = rec.ChangedFiles
	}
	actions := rec.Actions
	if len(actions) > taskActionsLimit {
		state["earlier_actions_omitted"] = len(actions) - taskActionsLimit
		actions = actions[len(actions)-taskActionsLimit:]
	}
	if len(actions) > 0 {
		list := make([]map[string]any, 0, len(actions))
		for _, a := range actions {
			e := map[string]any{"type": a.Type, "description": a.Description}
			if a.Target != "" {
				e["target"] = a.Target
			}
			if a.Failed {
				e["failed"] = true
			}
			list = append(list, e)
		}
		state["actions"] = list
	}

	qs := map[string]typesafe.Question{
		DidWhatWasAsked: typesafe.Noul{
			Instructions: "Does `result` show the work that `request` asked for was done?",
			Criteria: &typesafe.NoulCriteria{
				True:  "Everything the request asked for is in the result: the change made, the question answered, the tests written.",
				False: "Part or all of what was asked is missing from the result, or something different was done instead.",
			},
		},
		StoppedEarly: typesafe.Noul{
			Instructions: "Did the agent stop before finishing the work in `request`?",
			Criteria: &typesafe.NoulCriteria{
				True:  "The record ends with asked-for work left undone: a step not taken, a plan never carried out, a failure the agent gave up on.",
				False: "The record ends with the asked-for work finished, or with nothing left that the request asked for.",
			},
		},
		FailureKind: typesafe.Choice{
			Instructions: "Which kind of mistake best describes what went wrong in this record?",
			Criteria:     FailureKinds,
		},
		FailureKindFits: typesafe.Noul{
			Instructions: map[string]any{
				"question":      "Does one of `failure_kinds` describe a mistake the agent made in this record?",
				"failure_kinds": FailureKinds,
			},
			Criteria: &typesafe.NoulCriteria{
				True:  "The record shows a mistake by the agent, and one of the kinds names it.",
				False: "The record shows no mistake by the agent, or its mistake is none of these kinds.",
			},
		},
	}
	for i, f := range rec.ChangedFiles {
		qs[fileQuestionName(i)] = typesafe.Noul{
			Instructions: map[string]any{
				"question": "Does `request` call for changing `file`?",
				"file":     f,
			},
			Criteria: &typesafe.NoulCriteria{
				True:  "The request names this file, or the change it asks for has to be made in this file.",
				False: "The request neither names this file nor needs it changed; changing it is outside what was asked.",
			},
		}
	}
	return state, qs
}

func fileQuestionName(i int) string { return fmt.Sprintf("file_needed_%d", i) }

// JudgeTask asks Jev about one record. A failed call or an unreadable answer is
// an error, never a verdict.
func JudgeTask(ctx context.Context, a Asker, rec TaskRecord) (*TaskJudgments, error) {
	if a == nil {
		return nil, fmt.Errorf("learnjudge: no Jev client")
	}
	state, qs := TaskVerdictRequest(rec)
	resp, err := a.Ask(typesafe.WithOperation(ctx, OpTaskVerdict), state, qs)
	if err != nil {
		return nil, err
	}
	j := &TaskJudgments{Model: resp.Model}
	if j.DidWhatWasAsked, err = resp.Noul(DidWhatWasAsked); err != nil {
		return nil, err
	}
	if j.StoppedEarly, err = resp.Noul(StoppedEarly); err != nil {
		return nil, err
	}
	if j.CategoryFits, err = resp.Noul(FailureKindFits); err != nil {
		return nil, err
	}
	kind, err := resp.Choice(FailureKind)
	if err != nil {
		return nil, err
	}
	j.Category, j.CategoryProbs = kind.Choice, kind.Probabilities
	for i := range rec.ChangedFiles {
		p, err := resp.Noul(fileQuestionName(i))
		if err != nil {
			return nil, err
		}
		j.FileNeeded = append(j.FileNeeded, p)
	}
	return j, nil
}
