package learnjudge

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/typesafe"
)

// Shared cause: whether a group of failures that prompt evolution would buy a
// generated atom for shares one cause that one piece of guidance could
// prevent. Until 2026-10-28 any three failures with the same regex-classified
// problem type bought a generation, so three unrelated failures (a timeout, a
// wrong file, a provider outage) taught one atom. The decision is
// prompt_evolution's (jev_evolution.go); this asks the observation.

// GroupFailure is one failure in a group, as its verdict describes it.
type GroupFailure struct {
	Request     string
	Explanation string
	Category    string
}

// SharesOneCause is the question's name.
const SharesOneCause = "shares_one_cause"

// groupFailureLimit bounds each failure's text in the state; groupMaxFailures
// bounds how many go in one request. The state says what was left out.
const (
	groupFailureLimit = 2000
	groupMaxFailures  = 12
)

// SharedCauseRequest builds the state and the question.
func SharedCauseRequest(failures []GroupFailure) (map[string]any, map[string]typesafe.Question) {
	state := map[string]any{"agent": "codeNERD, a coding agent"}
	list := failures
	if len(list) > groupMaxFailures {
		state["more_failures_omitted"] = len(list) - groupMaxFailures
		list = list[len(list)-groupMaxFailures:]
	}
	items := make([]map[string]any, 0, len(list))
	for _, f := range list {
		item := map[string]any{}
		putBounded(item, "task", strings.TrimSpace(f.Request), groupFailureLimit)
		putBounded(item, "what_went_wrong", strings.TrimSpace(f.Explanation), groupFailureLimit)
		if c := strings.TrimSpace(f.Category); c != "" {
			item["kind"] = strings.ToLower(c)
		}
		items = append(items, item)
	}
	state["failures"] = items
	return state, map[string]typesafe.Question{
		SharesOneCause: typesafe.Noul{
			Instructions: "Do the failures in `failures` share one cause that a single instruction to the agent could prevent?",
			Criteria: &typesafe.NoulCriteria{
				True:  "The same mistake recurs across the failures, so one instruction (always do X, never do Y) would have prevented most of them.",
				False: "The failures have different causes, or causes outside the agent's control such as timeouts, outages or missing access, so no one instruction would prevent them.",
			},
		},
	}
}

// JudgeSharedCause asks whether the group shares one preventable cause. A
// failed call is an error, never a "no".
func JudgeSharedCause(ctx context.Context, a Asker, failures []GroupFailure) (float64, error) {
	if a == nil {
		return 0, fmt.Errorf("learnjudge: no Jev client")
	}
	if len(failures) == 0 {
		return 0, fmt.Errorf("learnjudge: empty failure group")
	}
	state, qs := SharedCauseRequest(failures)
	resp, err := a.Ask(typesafe.WithOperation(ctx, OpTaskVerdict), state, qs)
	if err != nil {
		return 0, err
	}
	return resp.Noul(SharesOneCause)
}
