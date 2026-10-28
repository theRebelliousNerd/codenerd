package turnjudge

import (
	"context"
	"fmt"

	"codenerd/internal/problemlog"
	"codenerd/internal/types"
	"codenerd/internal/typesafe"
)

// TriageProblems asks Jev, for each of codeNERD's own logged problems,
// whether a bug in codeNERD's code caused it. The answers reach the kernel as
// self_problem_triage (TriageFacts), which decides what is a failure around
// codeNERD; the self-diagnosis chain works the defects first and lists the
// rest apart. Problems go batchSize to a request, each with its whole message.
// A failed triage is an error and no facts: the problems stay in the log's
// order.
func TriageProblems(ctx context.Context, a Asker, problems []problemlog.Problem, batchSize int) ([]float64, error) {
	parts, err := inBatches(problems, batchSize, func(batch []problemlog.Problem) ([]float64, error) {
		questions := make(map[string]typesafe.Question, len(batch))
		for i, p := range batch {
			questions[fmt.Sprintf("problem_%d", i)] = triageQuestion(p)
		}
		resp, err := a.Ask(typesafe.WithOperation(ctx, OpDiagnosisTriage), triageState, questions)
		if err != nil {
			return nil, err
		}
		out := make([]float64, len(batch))
		for i := range batch {
			if out[i], err = resp.Noul(fmt.Sprintf("problem_%d", i)); err != nil {
				return nil, err
			}
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	var out []float64
	for _, p := range parts {
		out = append(out, p...)
	}
	return out, nil
}

// triageState is shared by every question of a request.
var triageState = map[string]any{
	"context": "codeNERD is a coding agent. Each problem is a warning or error from its own log.",
}

// triageQuestion asks whether a bug in codeNERD's code caused the problem.
// The count is left out: how often it repeated does not say what caused it.
func triageQuestion(p problemlog.Problem) typesafe.Question {
	return typesafe.Noul{
		Instructions: map[string]any{
			"question": "Was `problem` caused by a bug in codeNERD's own code?",
			"problem": map[string]any{
				"level":    p.Level,
				"category": p.Category,
				"message":  p.Sample,
			},
		},
		Criteria: &typesafe.NoulCriteria{
			True:  "codeNERD's code is wrong: a nil dereference, a broken invariant, a parse or logic error, a rule that misfires, a missing case.",
			False: "Something around codeNERD failed: the network, a rate limit or outage at a model provider, the user's configuration, a missing tool, the disk or the machine.",
		},
	}
}

// Triage facts (policy/jev_evidence.mg): self_problem_triage(Signature,
// Category, Percent) is Jev's probability that a problem is a defect in
// codeNERD's own code; the kernel derives self_problem_around(Signature,
// Category) for one judged at or below judgment_no_percent.
const (
	TriagePredicate = "self_problem_triage"
	AroundPredicate = "self_problem_around"
)

// TriageFacts projects the judgments, one per problem, keyed as self_problem
// is (signature and category).
func TriageFacts(problems []problemlog.Problem, probs []float64) []types.Fact {
	out := make([]types.Fact, 0, len(problems))
	for i, p := range problems {
		if i >= len(probs) {
			break
		}
		out = append(out, types.Fact{Predicate: TriagePredicate, Args: []any{
			types.MangleString(p.Signature), types.MangleString(p.Category), types.PercentFromRatio(probs[i]),
		}})
	}
	return out
}

// SplitDefects separates the problems the kernel derives as failures around
// codeNERD (self_problem_around) from the rest, keeping each group in the
// log's order. An unsettled or unjudged problem stays with the defects: it
// still needs a look.
func SplitDefects(k Querier, problems []problemlog.Problem) (defects, around []problemlog.Problem, err error) {
	rows, err := k.Query(AroundPredicate)
	if err != nil {
		return nil, nil, err
	}
	type key struct{ sig, cat string }
	out := make(map[key]bool, len(rows))
	for _, r := range rows {
		if len(r.Args) == 2 {
			out[key{types.ExtractString(r.Args[0]), types.ExtractString(r.Args[1])}] = true
		}
	}
	for _, p := range problems {
		if out[key{p.Signature, p.Category}] {
			around = append(around, p)
			continue
		}
		defects = append(defects, p)
	}
	return defects, around, nil
}
