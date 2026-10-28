package autopoiesis

import (
	"context"
	"strings"

	internalconfig "codenerd/internal/config"
	"codenerd/internal/learnjudge"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// The tool-need trigger: whether a request (and a failed attempt at it) is a
// capability gap a generated tool should fill. Until 2026-10-28 the
// ToolGenerator decided in Go: missingCapabilityPatterns over the input, and
// any non-empty failed attempt, made a tool need, so every failure looked like
// a missing tool.
//
// Now Jev observes (learnjudge.JudgeToolNeed: asks_for_tool,
// needs_missing_operation, failed_for_missing_tool), the pattern match is
// asserted beside it as fallback evidence, and the kernel decides
// (policy/jev_learning.mg, tool_gap_observed):
//
//	asks_for_tool  failed_for_missing_tool  needs_missing_operation  gap
//	yes            any                      any                      yes (/jev)
//	not yes        yes                      yes                      yes (/jev)
//	otherwise                                                        no
//	no judgment (Jev off, tool_checks off, failed): a pattern match   yes (/pattern)
//
// A failed attempt alone is never a gap. Only after the kernel derives one does
// the LLM write the tool's spec (refineToolNeedWithLLM): generation is the only
// part that needs it. The duplicate check (tool_jev.go) still runs before the
// tool is built.

// toolGapOutcome is the kernel's decision for one request.
type toolGapOutcome struct {
	gap    bool
	source string // "jev", "pattern", or "pattern-no-kernel"
	kind   string // a ToolKinds hint for the spec ("" when Jev gave none)
}

// toolGapDecider decides one request; the Orchestrator installs it with the
// kernel (SetKernel).
type toolGapDecider func(ctx context.Context, input, failedAttempt string) toolGapOutcome

func (tg *ToolGenerator) setGapDecider(d toolGapDecider) {
	tg.gapMu.Lock()
	tg.gapDecider = d
	tg.gapMu.Unlock()
}

// decideToolGap asks the installed decider; without one (no kernel attached)
// the pattern match alone decides, and the log says so.
func (tg *ToolGenerator) decideToolGap(ctx context.Context, input, failedAttempt string) toolGapOutcome {
	tg.gapMu.RLock()
	d := tg.gapDecider
	tg.gapMu.RUnlock()
	if d != nil {
		return d(ctx, input, failedAttempt)
	}
	matched := shouldCheckToolNeed(input)
	if matched {
		logging.Get(logging.CategoryAutopoiesis).Info("Tool need: no kernel attached, the capability pattern decides")
	}
	return toolGapOutcome{gap: matched, source: "pattern-no-kernel"}
}

// Kernel predicates for the trigger.
const (
	toolNeedJudgmentPred = "tool_need_judgment"
	toolNeedPatternPred  = "tool_need_pattern"
)

// decideToolGap is the Orchestrator's decider: Jev's judgments and the pattern
// match become facts keyed by the request, the kernel derives
// tool_gap_observed, and the facts are retracted once read.
func (o *Orchestrator) decideToolGap(ctx context.Context, input, failedAttempt string) toolGapOutcome {
	o.mu.RLock()
	kernel, j, tg := o.kernel, o.jev, o.toolGen
	o.mu.RUnlock()
	log := logging.Get(logging.CategoryAutopoiesis)
	req := hashString(input + "\x00" + failedAttempt)
	var facts []types.Fact
	if shouldCheckToolNeed(input) {
		facts = append(facts, types.Fact{Predicate: toolNeedPatternPred, Args: []any{req}})
	}
	out := toolGapOutcome{}
	switch {
	case j.asker == nil:
	case j.cfg == nil || !j.cfg.Uses().ToolChecks:
		log.Info("Tool need: tool_checks is off; the capability pattern decides")
	default:
		in := learnjudge.ToolNeedInput{Request: input, FailedAttempt: failedAttempt}
		if tg != nil {
			for name, schema := range tg.snapshotTools() {
				in.Tools = append(in.Tools, learnjudge.ExistingTool{Name: name, Description: schema.Description})
			}
		}
		jctx, cancel := context.WithTimeout(ctx, j.cfg.Uses().TurnJudgmentTimeout)
		judged, err := learnjudge.JudgeToolNeed(jctx, j.asker, in)
		cancel()
		if err != nil {
			log.Warn("Tool need: no Jev judgment, the capability pattern decides: %v", err)
			break
		}
		out.kind = judged.Kind
		add := func(name string, p float64) {
			facts = append(facts, types.Fact{Predicate: toolNeedJudgmentPred, Args: []any{req, name, types.PercentFromRatio(p)}})
		}
		add(learnjudge.AsksForTool, judged.AsksForTool)
		add(learnjudge.NeedsMissingOp, judged.NeedsMissingOp)
		if judged.AskedAboutFailure {
			add(learnjudge.FailedForMissingTool, judged.FailedForMissingTool)
		}
		log.Info("Tool need: Jev asks_for_tool=%.2f needs_missing_operation=%.2f failed_for_missing_tool=%.2f (asked=%t)",
			judged.AsksForTool, judged.NeedsMissingOp, judged.FailedForMissingTool, judged.AskedAboutFailure)
	}
	if kernel == nil {
		out.gap, out.source = shouldCheckToolNeed(input), "pattern-no-kernel"
		return out
	}
	if j.cfg != nil {
		if err := internalconfig.EnsureParams(kernel, j.cfg.Params()); err != nil {
			log.Warn("Tool need: thresholds not asserted: %v", err)
		}
	}
	if len(facts) == 0 {
		return out
	}
	if err := kernel.AssertBatch(facts); err != nil {
		log.Error("Tool need: assert evidence: %v", err)
		return out
	}
	defer func() {
		if err := kernel.RetractExactFactsBatch(facts); err != nil {
			log.Warn("Tool need: retract evidence: %v", err)
		}
	}()
	rows, err := kernel.Query("tool_gap_observed")
	if err != nil {
		log.Warn("Tool need: query tool_gap_observed: %v", err)
		return out
	}
	for _, r := range rows {
		if len(r.Args) == 2 && types.ExtractString(r.Args[0]) == req {
			out.gap = true
			out.source = strings.TrimPrefix(types.ExtractString(r.Args[1]), "/")
			break
		}
	}
	return out
}
