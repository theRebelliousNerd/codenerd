package prompt_evolution

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"codenerd/internal/learnjudge"
	"codenerd/internal/logging"
)

// The task verdict as a cascade (ruled 2026-10-28): Jev answers observations
// about each record (learnjudge.JudgeTask) and verdictFromJudgments decides;
// the LLM judge (judge.go) is kept for the two cases Jev does not settle, a
// record whose observations land between the thresholds and a record Jev did
// not judge (off, switched off by task_verdicts, failed). That is TypeSafe's
// verify-and-escalate pattern: cheap typed judgments settle the clear records,
// a reasoning model takes the rest. The LLM no longer parses a verdict out of
// prose for a record Jev settled, and no record gets both.

// changedFileActions are the action types that change a file.
var changedFileActions = map[string]bool{"edit": true, "write": true, "create": true, "delete": true, "modify": true, "rename": true}

// taskRecordFor is the record as Jev's state carries it.
func taskRecordFor(exec *ExecutionRecord) learnjudge.TaskRecord {
	rec := learnjudge.TaskRecord{
		Request:     exec.TaskRequest,
		Shard:       exec.ShardType,
		Succeeded:   exec.ExecutionResult.Success,
		TestsPassed: exec.ExecutionResult.TestsPassed,
		TestsFailed: exec.ExecutionResult.TestsFailed,
		BuildErrors: exec.ExecutionResult.BuildErrors,
		Output:      exec.ExecutionResult.Output,
	}
	seen := map[string]bool{}
	for _, a := range exec.AgentActions {
		rec.Actions = append(rec.Actions, learnjudge.TaskAction{Type: a.Type, Description: a.Description, Target: a.Target, Failed: !a.Success})
		if a.Success && a.Target != "" && changedFileActions[strings.ToLower(a.Type)] && !seen[a.Target] {
			seen[a.Target] = true
			rec.ChangedFiles = append(rec.ChangedFiles, a.Target)
		}
	}
	return rec
}

// failureKindCategory maps a learnjudge.FailureKinds option to its category.
var failureKindCategory = map[string]ErrorCategory{
	"logic_error":      CategoryLogicError,
	"syntax_error":     CategorySyntaxError,
	"api_misuse":       CategoryAPIMisuse,
	"edge_case":        CategoryEdgeCase,
	"context_miss":     CategoryContextMiss,
	"instruction_miss": CategoryInstructionMiss,
	"hallucination":    CategoryHallucination,
}

// verdictFromJudgments is the verdict truth table. failed is what code already
// knows: the kernel saw the turn fail, the build broke, or tests failed.
//
//	failed  did_what_was_asked  stopped_early  a changed file not asked for  verdict
//	yes     any                 any            any                           FAIL
//	no      no                  any            any                           FAIL
//	no      any                 yes            any                           FAIL
//	no      any                 any            yes                           FAIL
//	no      yes                 no             none, none unsettled          PASS
//	no      otherwise (an unsettled reading and nothing failing)             unsettled
//
// A FAIL's category is the failure_kind Choice when the companion says a kind
// fits (or is unsettled: the Choice still names the likeliest kind), and
// NO_AGENT_MISTAKE when it says none does. ok is false for an unsettled record.
func verdictFromJudgments(exec *ExecutionRecord, j *learnjudge.TaskJudgments, u *jevUse, changed []string) (*JudgeVerdict, bool) {
	res := exec.ExecutionResult
	failed := !res.Success || len(res.BuildErrors) > 0 || res.TestsFailed > 0
	did, early := u.read(j.DidWhatWasAsked), u.read(j.StoppedEarly)

	var reasons []string
	if failed {
		reasons = append(reasons, "the run failed (kernel outcome, build errors or failing tests)")
	}
	if did == readNo {
		reasons = append(reasons, fmt.Sprintf("the result does not show the asked-for work done (Jev %.2f)", j.DidWhatWasAsked))
	}
	if early == readYes {
		reasons = append(reasons, fmt.Sprintf("the agent stopped before finishing (Jev %.2f)", j.StoppedEarly))
	}
	fileUnsettled := false
	for i, p := range j.FileNeeded {
		if i >= len(changed) {
			break
		}
		switch u.read(p) {
		case readNo:
			reasons = append(reasons, fmt.Sprintf("it changed %s, which the request did not call for (Jev %.2f)", changed[i], p))
		case readUnsettled:
			fileUnsettled = true
		}
	}

	v := &JudgeVerdict{
		TaskID: exec.TaskID, ShardType: exec.ShardType, AtomIDs: exec.AtomIDs,
		Provider: exec.Provider, Model: exec.Model,
		EvaluatedBy: "jev:" + j.Model, Timestamp: time.Now(),
	}
	switch {
	case len(reasons) > 0:
		v.Verdict = "FAIL"
		v.Category = failureCategory(j, u)
		v.Confidence = failConfidence(failed, j, u)
		v.Explanation = "Failed: " + strings.Join(reasons, "; ") + "."
		if failed && did == readYes {
			v.Explanation += fmt.Sprintf(" Jev reads the output as complete (%.2f), so the reported work is not backed by evidence.", j.DidWhatWasAsked)
		}
		v.Explanation += fmt.Sprintf(" Kind: %s (fits %.2f).", strings.ToLower(string(v.Category)), j.CategoryFits)
		return v, true
	case did == readYes && early == readNo && !fileUnsettled:
		v.Verdict = "PASS"
		v.Category = CategoryCorrect
		v.Confidence = min(j.DidWhatWasAsked, 1-j.StoppedEarly)
		for _, p := range j.FileNeeded {
			v.Confidence = min(v.Confidence, p)
		}
		v.Explanation = fmt.Sprintf("Passed: the result shows the asked-for work done (Jev %.2f), finished (stopped early %.2f), and only files the request called for changed.", j.DidWhatWasAsked, j.StoppedEarly)
		return v, true
	default:
		return nil, false
	}
}

func failureCategory(j *learnjudge.TaskJudgments, u *jevUse) ErrorCategory {
	if u.read(j.CategoryFits) == readNo {
		return CategoryNoAgentMistake
	}
	if c, ok := failureKindCategory[j.Category]; ok {
		return c
	}
	return CategoryNoAgentMistake
}

// failConfidence is the strongest evidence for the failure: 1 when code saw
// it, else the most confident failing reading.
func failConfidence(failed bool, j *learnjudge.TaskJudgments, u *jevUse) float64 {
	if failed {
		return 1
	}
	c := 0.0
	if u.read(j.DidWhatWasAsked) == readNo {
		c = max(c, 1-j.DidWhatWasAsked)
	}
	if u.read(j.StoppedEarly) == readYes {
		c = max(c, j.StoppedEarly)
	}
	for _, p := range j.FileNeeded {
		if u.read(p) == readNo {
			c = max(c, 1-p)
		}
	}
	return c
}

// jevVerdictConcurrency bounds parallel Jev calls in one batch.
const jevVerdictConcurrency = 8

// evaluateBatch is the cascade: Jev first, the LLM judge for what Jev leaves.
func (pe *PromptEvolver) evaluateBatch(ctx context.Context, execs []*ExecutionRecord) ([]*JudgeVerdict, error) {
	u := pe.jevFor()
	log := logging.Get(logging.CategoryAutopoiesis)
	if u == nil || !u.uses.TaskVerdicts {
		if u != nil {
			log.Info("Task verdicts: task_verdicts is off; the LLM judge judges %d record(s)", len(execs))
		}
		return pe.judge.EvaluateBatch(ctx, execs)
	}
	verdicts := make([]*JudgeVerdict, len(execs))
	escalate := make([]bool, len(execs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, jevVerdictConcurrency)
	for i, exec := range execs {
		if exec == nil {
			continue
		}
		if exec.Verdict != nil {
			verdicts[i] = exec.Verdict
			continue
		}
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			rec := taskRecordFor(exec)
			j, err := learnjudge.JudgeTask(ctx, u.asker, rec)
			if err != nil {
				log.Warn("Task verdict for %s: no Jev judgment, the LLM judge decides: %v", exec.TaskID, err)
				escalate[i] = true
				return
			}
			v, ok := verdictFromJudgments(exec, j, u, rec.ChangedFiles)
			if !ok {
				log.Info("Task verdict for %s: Jev unsettled (did %.2f, stopped %.2f), the LLM judge decides", exec.TaskID, j.DidWhatWasAsked, j.StoppedEarly)
				escalate[i] = true
				return
			}
			verdicts[i] = v
			exec.Verdict = v
		})
	}
	wg.Wait()

	var rest []*ExecutionRecord
	var at []int
	for i, e := range escalate {
		if e {
			rest = append(rest, execs[i])
			at = append(at, i)
		}
	}
	settled := len(execs) - len(rest)
	logging.Autopoiesis("Task verdicts: Jev settled %d of %d record(s); %d go to the LLM judge", settled, len(execs), len(rest))
	if len(rest) == 0 {
		return verdicts, nil
	}
	llm, err := pe.judge.EvaluateBatch(ctx, rest)
	for k, v := range llm {
		if k < len(at) {
			verdicts[at[k]] = v
		}
	}
	return verdicts, err
}
