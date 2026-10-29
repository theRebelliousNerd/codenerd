package session

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"codenerd/internal/broker"
	"codenerd/internal/config"
	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/learnjudge"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// The /unwired round (policy/turn_unwired.mg). A turn could add a function
// no production path calls and still read verified: on 2026-10-28 a coder
// added RepairExistingPromptAtoms, ran it once from a temporary test, deleted
// the test and reported done. The round measures the functions the turn added
// and their uses (unwired_measure.go), and while the kernel derives
// turn_unwired_pushback it hands the model the unreached ones: wire each into
// the production path meant to run it, delete it, or say why it is meant to
// have no caller here. Jev reads each reason; the kernel decides whether it
// settles the function. An attempt that writes nothing ends the round
// (turn_unwired_idle), and what is still open withholds the verdict. Writes
// re-arm the gate rounds, so the wiring is built and tested like the rest of
// the turn.

// wireUnreachedCode is the /unwired round's driver. current is the turn's
// answer so far; a push-back's own words are appended to it, so the report
// still says what the turn did and now also what the wiring did, or why a
// function has no caller.
func (e *Executor) wireUnreachedCode(
	ctx context.Context,
	trp types.ToolResultsProvider,
	systemPrompt string,
	history []types.Message,
	current *types.LLMToolResponse,
	toolDefs []types.ToolDefinition,
	cfg *jitconfig.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) (*types.LLMToolResponse, []string, error) {
	if result == nil || e.kernel == nil {
		return nil, nil, nil
	}
	turn := result.turnAtom()
	// A re-run (a /finish write re-armed the round) judges the code as it is
	// now; the previous run's idle mark says nothing about it.
	e.retractTurnPredicates(turn, "turn_unwired_idle")
	task := ""
	if loop := activeWorkingLoop(ctx); loop != nil {
		task = loop.task
	}
	var said []string
	var errs []string
	wroteAny := false
	for {
		funcs, err := e.measureNewFuncs(ctx, result)
		if err != nil {
			// The measurement is the round's subject: without it nothing
			// says the code is reached, so the round fails rather than
			// passing the turn on an unasked question.
			return nil, errs, fmt.Errorf("measure the functions this turn added: %w", err)
		}
		if !e.turnHolds("turn_unwired_pushback", result) {
			break
		}
		open := e.openUnwired(result, funcs)
		logging.Get(logging.CategorySession).Warn("this turn added %d function(s) no production path reaches: %s", len(open), unwiredRefs(open))
		if trp == nil || activeWorkingLoop(ctx) == nil {
			// No channel back to the model: the open functions stand, and
			// the verdict says so (turn_missing_evidence /code_unwired).
			break
		}
		resp, _, _, attemptErrs, _, wrote, err := e.workingAttempt(broker.WithPhase(ctx, broker.PhaseRepair), trp, systemPrompt, &history, toolDefs, cfg, result, unwiredPrompt(open), false)
		errs = append(errs, attemptErrs...)
		if err != nil {
			return nil, errs, fmt.Errorf("wiring the unreached code: %w", err)
		}
		if resp != nil && strings.TrimSpace(resp.Text) != "" {
			said = append(said, strings.TrimSpace(resp.Text))
			e.judgeUnusedReasons(ctx, result, task, open, resp.Text)
		}
		if !wrote {
			// No write, no progress: the kernel stops pushing. The reasons
			// this attempt gave still settle what they settle.
			e.assertTurnFact(types.Fact{Predicate: "turn_unwired_idle", Args: []any{turn}})
			if _, err := e.measureNewFuncs(ctx, result); err != nil {
				return nil, errs, fmt.Errorf("measure the functions this turn added: %w", err)
			}
			break
		}
		wroteAny = true
	}
	if wroteAny {
		e.rearmGateRounds(result)
	}
	if len(said) == 0 || current == nil {
		return nil, errs, nil
	}
	answer := *current
	answer.Text = strings.TrimSpace(current.Text + "\n\n" + strings.Join(said, "\n\n"))
	return &answer, errs, nil
}

// openUnwired is the functions the kernel derives turn_func_unwired_open for,
// with what the measurement knows about each, in ref order.
func (e *Executor) openUnwired(result *ExecutionResult, funcs map[string]newFunc) []newFunc {
	turn := result.turnAtom()
	testOnly := map[string]bool{}
	if rows, err := e.turnRows("turn_func_test_only", turn); err == nil {
		for _, row := range rows {
			if ref, ok := rowRef(row); ok {
				testOnly[ref] = true
			}
		}
	}
	rows, err := e.turnRows("turn_func_unwired_open", turn)
	if err != nil {
		logging.Get(logging.CategorySession).Warn("query turn_func_unwired_open: %v", err)
		return nil
	}
	var out []newFunc
	for _, row := range rows {
		ref, ok := rowRef(row)
		if !ok {
			continue
		}
		fn, ok := funcs[ref]
		if !ok {
			fn = newFunc{ref: ref}
		}
		fn.testOnly = testOnly[ref]
		out = append(out, fn)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ref < out[j].ref })
	return out
}

// rowRef is the Ref column of a (Turn, Ref) row.
func rowRef(row types.Fact) (string, bool) {
	if len(row.Args) <= 1 {
		return "", false
	}
	return types.ExtractString(row.Args[1]), true
}

// judgeUnusedReasons asks Jev about each reason the model gave for an open
// function and asserts the reading. With no judge, or when the call fails,
// the stated reason is recorded unread: the kernel takes it as stated, and
// the log says it was not judged. A failed call is never a "no".
func (e *Executor) judgeUnusedReasons(ctx context.Context, result *ExecutionResult, task string, open []newFunc, text string) {
	reasons := parseUnusedReasons(text, open)
	if len(reasons) == 0 {
		return
	}
	turn := result.turnAtom()
	j := e.currentReportJudge()
	if j != nil {
		if err := config.EnsureParams(e.kernel, j.Config.Params()); err != nil {
			logging.Get(logging.CategorySession).Warn("unused reasons not judged: thresholds not asserted: %v", err)
			j = nil
		}
	}
	for _, fn := range open {
		reason, ok := reasons[fn.ref]
		if !ok {
			continue
		}
		if j == nil {
			logging.Get(logging.CategorySession).Warn("%s: the stated reason it has no caller is taken unjudged (no Jev): %s", fn.ref, reason)
			e.assertTurnFact(types.Fact{Predicate: "turn_unused_reason_unread", Args: []any{turn, types.MangleString(fn.ref)}})
			continue
		}
		reading, err := learnjudge.JudgeUnusedReason(ctx, j.Asker, learnjudge.UnusedReasonInput{
			Task: task, Ref: fn.ref, Signature: fn.signature, Doc: fn.doc, Reason: reason,
		})
		if err != nil {
			logging.Get(logging.CategorySession).Warn("%s: the stated reason it has no caller is taken unjudged (Jev: %v): %s", fn.ref, err, reason)
			e.assertTurnFact(types.Fact{Predicate: "turn_unused_reason_unread", Args: []any{turn, types.MangleString(fn.ref)}})
			continue
		}
		logging.Get(logging.CategorySession).Info("%s: unused reason read: names a current caller %.2f, task asks for it uncalled %.2f", fn.ref, reading.CurrentCaller, reading.TaskAsks)
		e.assertTurnFact(learnjudge.UnusedReasonFact(turn, fn.ref, reading))
	}
}

// unusedReasonLine is the answer format the push-back asks for:
// "unused <ref>: <reason>", one function per line, optionally bulleted or
// with the ref in backticks.
var unusedReasonLine = regexp.MustCompile("(?mi)^[\\s>*\\-]*unused\\s+`?([^\\s`:]+)`?\\s*:\\s*(\\S.*)$")

// parseUnusedReasons reads the reason lines out of the model's answer, for
// the functions the push-back named. A line naming a function the push-back
// did not name is not a reason for anything it asked about.
func parseUnusedReasons(text string, open []newFunc) map[string]string {
	asked := make(map[string]bool, len(open))
	for _, fn := range open {
		asked[fn.ref] = true
	}
	out := map[string]string{}
	for _, m := range unusedReasonLine.FindAllStringSubmatch(text, -1) {
		if asked[m[1]] {
			out[m[1]] = strings.TrimSpace(m[2])
		}
	}
	return out
}

func unwiredRefs(open []newFunc) string {
	refs := make([]string, 0, len(open))
	for _, fn := range open {
		refs = append(refs, fn.ref)
	}
	return strings.Join(refs, ", ")
}

// unwiredPrompt names each open function where it is and what calls it, and
// the three answers the round accepts.
func unwiredPrompt(open []newFunc) string {
	var b strings.Builder
	b.WriteString("This turn added production code that no production path calls:\n\n")
	for _, fn := range open {
		b.WriteString("- ")
		b.WriteString(fn.ref)
		if fn.file != "" {
			fmt.Fprintf(&b, " (%s:%d)", fn.file, fn.line)
		}
		if fn.testOnly {
			b.WriteString(": only tests call it")
		} else {
			b.WriteString(": nothing calls it")
		}
		b.WriteString("\n")
	}
	b.WriteString("\nWire each one into the production path that is meant to run it (the command, boot step, handler or " +
		"registration), with your tools. A function only a test calls is not wired: production code must call it, or it " +
		"belongs in the test file. Delete one that is not needed. If one is meant to have no caller in this repository, " +
		"leave it and answer with one line for it:\n\nunused <ref>: <what calls it, or why the task wants it uncalled>\n")
	return b.String()
}
