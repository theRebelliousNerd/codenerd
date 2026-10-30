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
	Request        string
	Explanation    string
	Category       string
	TaskID         string
	VerdictVersion string
	SuggestedRule  string
}

// OpSharedCause identifies screening separately from task verdicts.
const OpSharedCause = "jev_shared_cause"

// SharesOneCause is the question's name.
const SharesOneCause = "shares_one_cause"

// groupFailureLimit bounds each failure's text in the state; groupMaxFailures
// bounds how many go in one request. The state says what was left out.
const (
	groupFailureLimit = 2000
	groupMaxFailures  = SharedCauseMaxFailures
	// SharedCauseMaxFailures is the request protocol ceiling. Evolution selects
	// a cohort within it before either model sees evidence.
	SharedCauseMaxFailures = 12
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
		if f.TaskID != "" {
			item["task_id"] = f.TaskID
		}
		if f.VerdictVersion != "" {
			item["verdict_version"] = f.VerdictVersion
		}
		putBounded(item, "suggested_rule", strings.TrimSpace(f.SuggestedRule), groupFailureLimit)
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

// SharedCauseJudgment keeps the answering model beside its observation.
type SharedCauseJudgment struct {
	Probability float64
	Model       string
}

// JudgeSharedCause asks about an already constructed projection. Evolution
// passes the same projection to the generator; this function never reselects
// or rerenders its evidence. A failed call is an error, never a "no".
func JudgeSharedCause(ctx context.Context, a Asker, state map[string]any, qs map[string]typesafe.Question) (*SharedCauseJudgment, error) {
	if a == nil {
		return nil, fmt.Errorf("learnjudge: no Jev client")
	}
	items, ok := state["failures"].([]map[string]any)
	if !ok || len(items) == 0 {
		return nil, fmt.Errorf("learnjudge: empty failure group")
	}
	if _, ok := qs[SharesOneCause]; !ok {
		return nil, fmt.Errorf("learnjudge: missing shared-cause question")
	}
	resp, err := a.Ask(typesafe.WithOperation(ctx, OpSharedCause), state, qs)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("learnjudge: no shared-cause response")
	}
	p, err := resp.Noul(SharesOneCause)
	if err != nil {
		return nil, err
	}
	return &SharedCauseJudgment{Probability: p, Model: resp.Model}, nil
}
