package session

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/broker"
	"codenerd/internal/config"
	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/learnjudge"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// The /finish round (policy/turn_finish.mg). A turn whose own report says
// part of the task is not done used to close "not verified" and stop; the
// admission withheld /done (admission_audit.go) and nothing sent it back. The
// round reads the report with Jev and, while the kernel derives
// turn_finish_pushback, hands the model the unfinished work and lets it go on
// with its tools. It stops when the report no longer admits unfinished work,
// names an obstacle codeNERD cannot clear, or an attempt writes nothing
// (turn_finish_idle). Writes re-arm the gate rounds, so what the model
// finishes is built and tested like the rest of the turn.

// ReportJudge reads final reports with Jev (learnjudge.JudgeReport). Config
// supplies the thresholds the kernel reads them with.
type ReportJudge struct {
	Asker  learnjudge.Asker
	Config *config.TypeSafeConfig
}

// SetReportJudge installs the final-report reader. Nil leaves the /finish
// round with nothing to read, and the closure audit asks the main model.
func (e *Executor) SetReportJudge(j *ReportJudge) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.reportJudge = j
}

func (e *Executor) currentReportJudge() *ReportJudge {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.reportJudge == nil || e.reportJudge.Asker == nil {
		return nil
	}
	return e.reportJudge
}

// gateRounds are the rounds a /finish write re-arms: the ones that measure the
// workspace, and /unwired, which asks whether the code written since is
// reached. The critic and the survivors round advise and are not repeated.
var gateRounds = []string{"/build", "/test", "/coverage", "/pinned", "/vet", "/removed_tests", "/unwired", "/test_run"}

// readReport asks Jev about report and asserts the reading for the turn,
// replacing the previous one. It reports whether a reading was asserted: no
// judge, or a failed call (logged), leaves the kernel without one.
func (e *Executor) readReport(ctx context.Context, result *ExecutionResult, task, report string) bool {
	j := e.currentReportJudge()
	if j == nil || e.kernel == nil || strings.TrimSpace(report) == "" {
		return false
	}
	reading, err := learnjudge.JudgeReport(ctx, j.Asker, learnjudge.ReportInput{Task: task, Report: report})
	if err != nil {
		logging.Get(logging.CategorySession).Warn("final report not read (Jev: %v); the turn closes without the reading", err)
		return false
	}
	if err := config.EnsureParams(e.kernel, j.Config.Params()); err != nil {
		logging.Get(logging.CategorySession).Warn("final report reading dropped: thresholds not asserted: %v", err)
		return false
	}
	turn := result.turnAtom()
	if result.reportReading != nil {
		if err := e.kernel.RetractExactFactsBatch([]types.Fact{*result.reportReading}); err != nil {
			logging.Get(logging.CategorySession).Warn("replace the final report reading: %v", err)
		}
	}
	fact := learnjudge.ReportReadingFact(turn, reading)
	if !e.assertTurnFact(fact) {
		return false
	}
	result.reportReading = &fact
	logging.Get(logging.CategorySession).Info("final report: unfinished %.2f, blocker %.2f", reading.Unfinished, reading.Blocked)
	return true
}

// turnHolds reports whether the kernel derives predicate(Turn) for this turn.
func (e *Executor) turnHolds(predicate string, result *ExecutionResult) bool {
	rows, err := e.turnRows(predicate, result.turnAtom())
	if err != nil {
		logging.Get(logging.CategorySession).Warn("query %s: %v", predicate, err)
		return false
	}
	return len(rows) > 0
}

// finishUnfinishedWork is the /finish round's driver. current is the turn's
// answer so far; the round returns the answer it ends with.
func (e *Executor) finishUnfinishedWork(
	ctx context.Context,
	trp types.ToolResultsProvider,
	systemPrompt string,
	history []types.Message,
	current *types.LLMToolResponse,
	toolDefs []types.ToolDefinition,
	cfg *jitconfig.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) (*types.LLMToolResponse, []string, error) {
	if result == nil || current == nil || trp == nil || e.currentReportJudge() == nil {
		return nil, nil, nil
	}
	task := ""
	if loop := activeWorkingLoop(ctx); loop != nil {
		task = loop.task
	}
	ctx = broker.WithPhase(ctx, broker.PhaseFinish)
	report := current.Text
	var answer *types.LLMToolResponse
	var errs []string
	wroteAny := false
	prompt := finishPrompt(report)
	for {
		if !e.readReport(ctx, result, task, report) || !e.turnHolds("turn_finish_pushback", result) {
			break
		}
		logging.Get(logging.CategorySession).Warn("the turn's report says part of the task is not done; pushing it back to the model")
		resp, _, _, attemptErrs, _, wrote, err := e.workingAttempt(ctx, trp, systemPrompt, &history, toolDefs, cfg, result, prompt, false)
		errs = append(errs, attemptErrs...)
		if err != nil {
			return answer, errs, fmt.Errorf("finishing the unfinished work: %w", err)
		}
		if resp != nil && strings.TrimSpace(resp.Text) != "" {
			answer, report = resp, resp.Text
		}
		if !wrote {
			// No write, no progress: the kernel stops pushing. The reading of
			// what the model said this time still stands for the verdict.
			e.assertTurnFact(types.Fact{Predicate: "turn_finish_idle", Args: []any{result.turnAtom()}})
			e.readReport(ctx, result, task, report)
			break
		}
		wroteAny = true
		prompt = finishContinuePrompt
	}
	if wroteAny {
		e.rearmGateRounds(result)
	}
	return answer, errs, nil
}

// rearmGateRounds lets the gate rounds run again over what /finish wrote. The
// retraction is exact: RetractFact matches on the first argument only, which
// would drop every round this turn ran.
func (e *Executor) rearmGateRounds(result *ExecutionResult) {
	turn := result.turnAtom()
	var ran []types.Fact
	var rounds []string
	for _, round := range gateRounds {
		if result.roundsRan[round] {
			ran = append(ran, types.Fact{Predicate: "turn_round_ran", Args: []any{turn, types.MangleAtom(round)}})
			rounds = append(rounds, round)
		}
	}
	if len(ran) == 0 {
		return
	}
	if err := e.kernel.RetractExactFactsBatch(ran); err != nil {
		logging.Get(logging.CategorySession).Warn("re-arm the gate rounds after /finish: %v", err)
		return
	}
	for _, round := range rounds {
		delete(result.roundsRan, round)
	}
	logging.Get(logging.CategorySession).Info("/finish wrote: the gate rounds run again (%s)", strings.Join(rounds, " "))
}

// finishPrompt hands the model its own admission, whole: the model wrote it,
// and a cut admission could drop the part that names what is left.
func finishPrompt(report string) string {
	return "Your report says part of the task is not done. Your report:\n\n" + strings.TrimSpace(report) +
		"\n\nFinish it now: do the remaining work with your tools. If an obstacle you cannot clear yourself stops you " +
		"(a permission or policy denial, a missing credential or input, a choice only the user can make), " +
		"say exactly what it is and what you need, instead of doing more."
}

// finishContinuePrompt follows an attempt that wrote.
const finishContinuePrompt = "Keep going until the whole task is done, then say what you did. " +
	"If an obstacle you cannot clear yourself stops you, say exactly what it is."
