package autopoiesis

import (
	"context"
	"fmt"
	"sort"
	"strings"

	internalconfig "codenerd/internal/config"
	"codenerd/internal/learnjudge"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// Tool value: Jev's observations about generated tools, as kernel facts the
// tool-value rules read (policy/jev_learning.mg, tool_value_evidence; the
// measured ledger's tool_value_class reads that beside cost and condensation).
// A tool earns its place by adding a capability the harness lacks, condensing
// the search space, or doing far more work than it costs to describe, call and
// verify. Jev observes three things code cannot measure:
//
//   - per call, whether the output gave the caller what its input asked for
//     (tool_call_hit, asked with the output judgments, no extra call);
//   - per tool, whether another registered tool already does the same thing
//     (tool_duplicate_judgment, the generation-time duplicate check reused);
//   - per tool, whether the harness or one shell command already does it
//     (tool_harness_judgment).
//
// No answer (Jev off, tool_checks off, a failed call) is no fact: the rules
// then see only the ledger's measurements.

const (
	toolCallHitPred       = "tool_call_hit"
	toolDuplicatePred     = "tool_duplicate_judgment"
	toolHarnessPred       = "tool_harness_judgment"
	toolValueEvidencePred = "tool_value_evidence"
)

// recordCallHit settles one call's observation and counts it in the ledger. The
// kernel classifies the call (tool_call_landed / tool_call_missed, against the
// Jev thresholds); Go reads that back, adds it to the tool's persisted tally
// (tool_ledger_jev.go), drops the per-call fact, and replays the tallies into
// the kernel, which derives the evidence (tool_value_evidence) from the tally
// across sessions. With no ledger to keep it in there is nothing to count.
func (o *Orchestrator) recordCallHit(f *ExecutionFeedback, p float64) {
	o.mu.RLock()
	kernel, j, ledger := o.kernel, o.jev, o.profiles
	o.mu.RUnlock()
	if kernel == nil || ledger == nil || f == nil || f.ToolName == "" {
		return
	}
	log := logging.Get(logging.CategoryAutopoiesis)
	call := f.ExecutionID
	if call == "" {
		call = fmt.Sprintf("%s@%d", f.ToolName, f.Timestamp.UnixNano())
	}
	if j.cfg != nil {
		if err := internalconfig.EnsureParams(kernel, j.cfg.Params()); err != nil {
			log.Warn("tool value: thresholds not asserted: %v", err)
			return
		}
	}
	hit := types.Fact{Predicate: toolCallHitPred, Args: []any{f.ToolName, call, types.PercentFromRatio(p)}}
	if err := kernel.Assert(hit); err != nil {
		log.Warn("tool value: assert call observation for %s: %v", f.ToolName, err)
		return
	}
	outcome := callOutcome(kernel, f.ToolName, call)
	if err := kernel.RetractFact(hit); err != nil {
		log.Warn("tool value: retract settled observation for %s: %v", f.ToolName, err)
	}
	ledger.NoteJevCall(f.ToolName, outcome)
	o.replayJevEvidence(kernel, ledger)
}

// callOutcome reads back how the policy classified one call.
func callOutcome(kernel types.Kernel, tool, call string) JevCallOutcome {
	landed, err := kernel.Query("tool_call_landed")
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("tool value: query tool_call_landed: %v", err)
	}
	missed, err := kernel.Query("tool_call_missed")
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("tool value: query tool_call_missed: %v", err)
	}
	switch {
	case isCall(landed, tool, call):
		return JevCallLanded
	case isCall(missed, tool, call):
		return JevCallMissed
	}
	return JevCallUnsettled
}

func isCall(rows []types.Fact, tool, call string) bool {
	for _, r := range rows {
		if len(r.Args) == 2 && types.ExtractString(r.Args[0]) == tool && types.ExtractString(r.Args[1]) == call {
			return true
		}
	}
	return false
}

// replayJevEvidence makes the kernel's Jev facts the ledger's.
func (o *Orchestrator) replayJevEvidence(kernel types.Kernel, ledger *ProfileStore) {
	if err := ReplaceJevFacts(kernel, ledger.ToolLedgers()); err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("tool value: replay Jev evidence: %v", err)
	}
}

// loadJevEvidence reloads what earlier processes persisted into a kernel just
// attached, so the evidence, and the classes derived from it, survive restarts.
func (o *Orchestrator) loadJevEvidence() {
	o.mu.RLock()
	kernel, ledger := o.kernel, o.profiles
	o.mu.RUnlock()
	if kernel == nil || ledger == nil {
		return
	}
	o.replayJevEvidence(kernel, ledger)
}

// ToolValueEvidence is one tool's Jev-derived value evidence, as the kernel
// derives it.
type ToolValueEvidence struct {
	Tool     string
	Evidence []string // /duplicate, /harness_covers, /hits_mark, /misses_mark
}

// JudgeToolValue asks the per-tool questions for every registered tool,
// replaces their facts in the kernel, and returns what the kernel derives. It
// is asked on demand (`nerd autopoiesis tools`), not on every boot.
func (o *Orchestrator) JudgeToolValue(ctx context.Context) ([]ToolValueEvidence, error) {
	o.mu.RLock()
	kernel, j, loop := o.kernel, o.jev, o.ouroboros
	o.mu.RUnlock()
	if kernel == nil {
		return nil, fmt.Errorf("tool value: no kernel attached")
	}
	log := logging.Get(logging.CategoryAutopoiesis)
	switch {
	case j.asker == nil:
		log.Info("tool value: Jev is not configured; only measured evidence applies")
	case j.cfg == nil || !j.cfg.Uses().ToolChecks:
		log.Info("tool value: tool_checks is off; only measured evidence applies")
	case loop == nil:
	default:
		o.judgeEachTool(ctx, kernel, j, loop.ListTools())
	}
	return readToolValueEvidence(kernel)
}

func (o *Orchestrator) judgeEachTool(ctx context.Context, kernel types.Kernel, j jevUse, infos []types.ToolInfo) {
	log := logging.Get(logging.CategoryAutopoiesis)
	o.mu.RLock()
	ledger := o.profiles
	o.mu.RUnlock()
	if ledger != nil {
		// Whatever happens below, the kernel ends holding the ledger's evidence.
		defer o.replayJevEvidence(kernel, ledger)
	}
	// A fresh reading replaces the replayed verdicts too, or an old "yes"
	// would keep deriving the evidence the new answer is meant to withdraw.
	for _, pred := range []string{toolDuplicatePred, toolHarnessPred, jevVerdictPred} {
		_ = kernel.Retract(pred)
	}
	if err := internalconfig.EnsureParams(kernel, j.cfg.Params()); err != nil {
		log.Warn("tool value: thresholds not asserted: %v", err)
		return
	}
	tools := make([]learnjudge.ExistingTool, 0, len(infos))
	for _, t := range infos {
		tools = append(tools, learnjudge.ExistingTool{Name: t.Name, Description: t.Description})
	}
	sort.Slice(tools, func(a, b int) bool { return tools[a].Name < tools[b].Name })
	var facts []types.Fact
	var answered []JevVerdictWrite
	for i, t := range tools {
		if p, err := learnjudge.JudgeHarnessCovers(ctx, j.asker, t); err != nil {
			log.Warn("tool value: %s harness check unanswered: %v", t.Name, err)
		} else {
			facts = append(facts, types.Fact{Predicate: toolHarnessPred, Args: []any{t.Name, types.PercentFromRatio(p)}})
			answered = append(answered, JevVerdictWrite{Tool: t.Name, Kind: JevVerdictHarnessCovers,
				ToolJevVerdict: ToolJevVerdict{Percent: types.PercentFromRatio(p)}})
		}
		others := append(append([]learnjudge.ExistingTool{}, tools[:i]...), tools[i+1:]...)
		if len(others) == 0 || len(others) > learnjudge.MaxToolOptions {
			continue
		}
		c, err := learnjudge.CheckDuplicate(ctx, j.asker, learnjudge.ToolRequest{Purpose: t.Description}, others)
		if err != nil {
			log.Warn("tool value: %s duplicate check unanswered: %v", t.Name, err)
			continue
		}
		facts = append(facts, types.Fact{Predicate: toolDuplicatePred, Args: []any{t.Name, c.Tool, types.PercentFromRatio(c.AnyExists)}})
		answered = append(answered, JevVerdictWrite{Tool: t.Name, Kind: JevVerdictDuplicate,
			ToolJevVerdict: ToolJevVerdict{Percent: types.PercentFromRatio(c.AnyExists), Other: c.Tool}})
	}
	if len(facts) > 0 {
		if err := kernel.AssertBatch(facts); err != nil {
			log.Error("tool value: assert observations: %v", err)
		}
	}
	if ledger == nil {
		return
	}
	// The kernel decides which answers are a yes; the ledger keeps its decision.
	derived, err := readToolValueEvidence(kernel)
	if err != nil {
		log.Warn("tool value: read evidence to persist it: %v", err)
		return
	}
	yes := map[string]bool{}
	for _, e := range derived {
		for _, kind := range e.Evidence {
			yes[e.Tool+" "+kind] = true
		}
	}
	for i := range answered {
		answered[i].Yes = yes[answered[i].Tool+" /"+answered[i].Kind]
	}
	ledger.NoteJevVerdicts(answered)
}

func readToolValueEvidence(kernel types.Kernel) ([]ToolValueEvidence, error) {
	rows, err := kernel.Query(toolValueEvidencePred)
	if err != nil {
		return nil, err
	}
	by := map[string][]string{}
	for _, r := range rows {
		if len(r.Args) == 2 {
			tool := types.ExtractString(r.Args[0])
			by[tool] = append(by[tool], types.ExtractString(r.Args[1]))
		}
	}
	out := make([]ToolValueEvidence, 0, len(by))
	for tool, ev := range by {
		sort.Strings(ev)
		out = append(out, ToolValueEvidence{Tool: tool, Evidence: ev})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Tool < out[b].Tool })
	return out, nil
}

// String renders the evidence for the CLI.
func (e ToolValueEvidence) String() string {
	return e.Tool + ": " + strings.Join(e.Evidence, ", ")
}
