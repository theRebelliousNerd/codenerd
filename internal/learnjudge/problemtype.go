package learnjudge

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/typesafe"
)

// Problem type: what kind of coding problem a task is, for prompt evolution's
// failure groups and strategy selection. It replaces the regex patterns and
// keyword boosters of prompt_evolution's classifier, which stays only as the
// fallback when Jev gives no answer.

// ProblemKinds are the Choice options, named as prompt evolution's
// ProblemType values.
var ProblemKinds = map[string]any{
	"debugging":        "Find and fix a bug: something crashes, fails, or gives a wrong result.",
	"feature_creation": "Add new behavior or a new capability that does not exist yet.",
	"refactoring":      "Restructure existing code without changing what it does: rename, split, move, simplify.",
	"testing":          "Write, fix or extend tests.",
	"documentation":    "Write or update documentation, comments or explanations of the code.",
	"performance":      "Make something faster or use less memory.",
	"security":         "Fix or prevent a vulnerability, or harden access, secrets or input handling.",
	"api_integration":  "Connect to an external service or API, or change how one is called.",
	"data_migration":   "Change a stored data format or schema, or move data between stores.",
	"config_setup":     "Change configuration, build setup, environment or deployment settings.",
	"error_handling":   "Change how errors are reported, wrapped, recovered from or surfaced.",
	"concurrency":      "Fix or change goroutines, locks, channels, races or parallel work.",
	"type_system":      "Change types, interfaces, generics or type signatures.",
	"dependency_mgmt":  "Add, upgrade, remove or pin a dependency.",
	"code_review":      "Review code and report problems, without changing it.",
	"research":         "Find out how something works or answer a question about the code, without changing it.",
}

// Question names.
const (
	ProblemKind     = "problem_kind"
	ProblemKindFits = "problem_kind_fits"
)

// ProblemTypeJudgment is Jev's answer: the chosen kind with its distribution,
// and the companion probability that any kind fits the task.
type ProblemTypeJudgment struct {
	Kind  string
	Probs map[string]float64
	Fits  float64
	Model string
}

// problemRequestLimit bounds the task text in the state; the state says how
// much was left out.
const problemRequestLimit = 16000

// ProblemTypeRequest builds the state and the two questions.
func ProblemTypeRequest(request, agent string) (map[string]any, map[string]typesafe.Question) {
	state := map[string]any{}
	putBounded(state, "task", strings.TrimSpace(request), problemRequestLimit)
	if a := strings.TrimPrefix(strings.TrimSpace(agent), "/"); a != "" {
		state["agent"] = a
	}
	return state, map[string]typesafe.Question{
		ProblemKind: typesafe.Choice{
			Instructions: "Which kind of work does `task` ask for?",
			Criteria:     ProblemKinds,
		},
		ProblemKindFits: typesafe.Noul{
			Instructions: map[string]any{
				"question":      "Does `task` ask for one of the kinds of work in `problem_kinds`?",
				"problem_kinds": ProblemKinds,
			},
			Criteria: &typesafe.NoulCriteria{
				True:  "The task is coding work that one of the kinds describes.",
				False: "The task is not coding work (a greeting, small talk, a question about something else), or none of the kinds describes it.",
			},
		},
	}
}

// JudgeProblemType asks Jev what kind of work the task is. A failed call is an
// error, never a kind.
func JudgeProblemType(ctx context.Context, a Asker, request, agent string) (*ProblemTypeJudgment, error) {
	if a == nil {
		return nil, fmt.Errorf("learnjudge: no Jev client")
	}
	if strings.TrimSpace(request) == "" {
		return nil, fmt.Errorf("learnjudge: empty task")
	}
	state, qs := ProblemTypeRequest(request, agent)
	resp, err := a.Ask(typesafe.WithOperation(ctx, OpProblemType), state, qs)
	if err != nil {
		return nil, err
	}
	kind, err := resp.Choice(ProblemKind)
	if err != nil {
		return nil, err
	}
	fits, err := resp.Noul(ProblemKindFits)
	if err != nil {
		return nil, err
	}
	if _, ok := ProblemKinds[kind.Choice]; !ok {
		return nil, fmt.Errorf("learnjudge: problem kind %q is not an option", kind.Choice)
	}
	return &ProblemTypeJudgment{Kind: kind.Choice, Probs: kind.Probabilities, Fits: fits, Model: resp.Model}, nil
}
