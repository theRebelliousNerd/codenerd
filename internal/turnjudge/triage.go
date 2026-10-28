package turnjudge

import (
	"context"
	"fmt"

	"codenerd/internal/problemlog"
	"codenerd/internal/typesafe"
)

// TriageProblems asks Jev, for each of codeNERD's own logged problems,
// whether it is a defect in codeNERD's code rather than its surroundings. The
// self-diagnosis chain works the defects first and lists the rest apart; a
// failed triage leaves the problems in the log's order.
func TriageProblems(ctx context.Context, a Asker, problems []problemlog.Problem) ([]float64, error) {
	questions := make(map[string]typesafe.Question, len(problems))
	for i, p := range problems {
		questions[fmt.Sprintf("problem_%d", i)] = typesafe.Noul{
			Instructions: map[string]any{
				"question": "Is `problem` a defect in codeNERD's own code, rather than a failure of something around it?",
				"problem": map[string]any{
					"level":    p.Level,
					"category": p.Category,
					"count":    p.Count,
					"message":  p.Sample,
				},
			},
			Criteria: &typesafe.NoulCriteria{
				True:  "codeNERD's code is wrong: a nil dereference, a broken invariant, a parse or logic error, a rule that misfires, a missing case.",
				False: "Something around codeNERD failed: the network, a rate limit or outage at a model provider, the user's configuration, a missing tool, the disk or the machine.",
			},
		}
	}
	if len(questions) == 0 {
		return nil, nil
	}
	resp, err := a.Ask(ctx, map[string]any{"assistant": "codeNERD, a coding agent reading its own problems log"}, questions)
	if err != nil {
		return nil, err
	}
	out := make([]float64, len(problems))
	for i := range problems {
		p, err := resp.Noul(fmt.Sprintf("problem_%d", i))
		if err != nil {
			return nil, err
		}
		out[i] = p
	}
	return out, nil
}

// SplitDefects separates the problems Jev judged at or below no (not a defect
// in codeNERD's code) from the rest, keeping each group in the log's order. An
// unsettled judgment stays with the defects: it still needs a look.
func SplitDefects(problems []problemlog.Problem, probs []float64, no float64) (defects, around []problemlog.Problem) {
	for i, p := range problems {
		if i < len(probs) && probs[i] <= no {
			around = append(around, p)
			continue
		}
		defects = append(defects, p)
	}
	return defects, around
}
