package learnjudge

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/types"
	"codenerd/internal/typesafe"
)

// Autopoiesis checks: what a chat request asks for, read before the turn
// decides whether to suggest a campaign or a persistent agent
// (policy/jev_autopoiesis.mg); and what a generated tool's output reports,
// read when the tool's quality is assessed (autopoiesis/quality.go). These
// replace the keyword heuristics in autopoiesis/complexity.go,
// persistence.go and quality.go wherever the heuristic stood in for reading
// language. Counts and sizes stay in code.

// Request judgments, the names the kernel reads as
// autopoiesis_judgment(Name, Percent).
const (
	ScopeSeveralDeliverables = "several_deliverables"
	ScopeOrderedStages       = "ordered_stages"
	ScopeManyFiles           = "many_files"
	ScopeContinuesAfter      = "continues_after_request"
)

// RequestJudgmentPredicate carries each request judgment into the kernel.
const RequestJudgmentPredicate = "autopoiesis_judgment"

// requestState is the request as a person would read it: the message and, when
// perception named one, the code it is about.
func requestState(request, target string) map[string]any {
	state := map[string]any{
		"assistant": "codeNERD, a coding agent working in the user's repository",
		"request":   request,
	}
	if t := strings.TrimSpace(target); t != "" && t != "none" {
		state["target"] = t
	}
	return state
}

// requestQuestions are the four request judgments.
func requestQuestions() map[string]typesafe.Question {
	return map[string]typesafe.Question{
		ScopeSeveralDeliverables: typesafe.Noul{
			Instructions: "Does `request` ask for several separate pieces of work, each with its own result?",
			Criteria: &typesafe.NoulCriteria{
				True:  "The request names distinct things to deliver, such as a new endpoint, its tests and its documentation, or a schema migration and the code that uses it.",
				False: "The request asks for one change, one fix, one answer or one explanation, even when that one change touches several files.",
			},
		},
		ScopeOrderedStages: typesafe.Noul{
			Instructions: "Does part of the work in `request` have to be finished before another part of it can start?",
			Criteria: &typesafe.NoulCriteria{
				True:  "One step depends on another: design the schema, then migrate the data, then switch the readers over.",
				False: "The work is one step, or its parts can be done in any order.",
			},
		},
		ScopeManyFiles: typesafe.Noul{
			Instructions: "Will carrying out `request` take changes to many files?",
			Criteria: &typesafe.NoulCriteria{
				True:  "A new feature or subsystem, a migration, a rewrite, or a change made throughout the codebase.",
				False: "A change to one file, one function or a few related lines; or no change at all, such as a question, an explanation or a review.",
			},
		},
		ScopeContinuesAfter: typesafe.Noul{
			Instructions: "Does `request` ask for work that keeps going after this request has been answered?",
			Criteria: &typesafe.NoulCriteria{
				True:  "Watching for changes, monitoring, alerting, a check on every commit or pull request, a daily or nightly job, work left running in the background.",
				False: "A task that is done once it is answered, however large it is.",
			},
		},
	}
}

// JudgeRequestScope asks the four request judgments in one call and returns
// the probability that each is true. A failed call is an error and no
// judgments: the caller falls back, it never reads a "no".
func JudgeRequestScope(ctx context.Context, a Asker, request, target string) (map[string]float64, error) {
	if strings.TrimSpace(request) == "" {
		return nil, fmt.Errorf("learnjudge: empty request")
	}
	questions := requestQuestions()
	resp, err := a.Ask(typesafe.WithOperation(ctx, OpAutopoiesisChecks), requestState(request, target), questions)
	if err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(questions))
	for name := range questions {
		p, err := resp.Noul(name)
		if err != nil {
			return nil, err
		}
		out[name] = p
	}
	return out, nil
}

// RequestScopeFacts projects the judgments as autopoiesis_judgment(Name,
// Percent), sorted by name.
func RequestScopeFacts(judgments map[string]float64) []types.Fact {
	out := make([]types.Fact, 0, len(judgments))
	for _, name := range []string{ScopeContinuesAfter, ScopeManyFiles, ScopeOrderedStages, ScopeSeveralDeliverables} {
		p, ok := judgments[name]
		if !ok {
			continue
		}
		out = append(out, types.Fact{Predicate: RequestJudgmentPredicate, Args: []any{
			types.MangleString(name), types.PercentFromRatio(p),
		}})
	}
	return out
}

// Tool-output judgments: what a generated tool's output says about itself.
const (
	OutputReportsError     = "reports_error"
	OutputReportsPartial   = "reports_partial_results"
	OutputReportsMorePages = "reports_more_pages"
	OutputReportsRateLimit = "reports_rate_limit"
)

// toolOutputLimit bounds the output Jev reads, within its per-question context
// (32k tokens); the state says how much was left out.
const toolOutputLimit = 24000

// toolOutputQuestions are the four output judgments.
func toolOutputQuestions() map[string]typesafe.Question {
	return map[string]typesafe.Question{
		OutputReportsError: typesafe.Noul{
			Instructions: "Does `output` report that the tool's work failed with an error?",
			Criteria: &typesafe.NoulCriteria{
				True:  "The output is an error message, an exception, a timeout or a refused connection instead of the result.",
				False: "The output is the result. Data that merely mentions errors, such as a list of log lines or a search hit for the word, is still a result.",
			},
		},
		OutputReportsPartial: typesafe.Noul{
			Instructions: "Does `output` say that it holds only part of the results the tool was asked for?",
			Criteria: &typesafe.NoulCriteria{
				True:  "The output says it was cut off, limited, capped at a maximum, or that some items could not be fetched.",
				False: "The output gives its results with no sign that any are missing.",
			},
		},
		OutputReportsMorePages: typesafe.Noul{
			Instructions: "Does `output` say that more pages of results exist beyond the ones it shows?",
			Criteria: &typesafe.NoulCriteria{
				True:  "A page marker such as page 1 of 5, a next-page token or link, or a has_more flag that is true.",
				False: "The output is all there is, or it states it is the last page.",
			},
		},
		OutputReportsRateLimit: typesafe.Noul{
			Instructions: "Does `output` say that a rate limit stopped or slowed the tool?",
			Criteria: &typesafe.NoulCriteria{
				True:  "An HTTP 429, a too-many-requests message, or a note that requests were throttled.",
				False: "No limit on the number of requests is mentioned.",
			},
		},
	}
}

// JudgeToolOutput asks the four output judgments about one execution of a
// generated tool. A failed call is an error and no judgments.
func JudgeToolOutput(ctx context.Context, a Asker, toolName, input, output string) (map[string]float64, error) {
	state := map[string]any{"tool": toolName}
	if strings.TrimSpace(input) != "" {
		putBounded(state, "input", input, toolOutputLimit/6)
	}
	putBounded(state, "output", output, toolOutputLimit)
	questions := toolOutputQuestions()
	resp, err := a.Ask(typesafe.WithOperation(ctx, OpAutopoiesisChecks), state, questions)
	if err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(questions))
	for name := range questions {
		p, err := resp.Noul(name)
		if err != nil {
			return nil, err
		}
		out[name] = p
	}
	return out, nil
}
