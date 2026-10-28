package autopoiesis

import (
	"context"
	"fmt"
	"strings"
	"time"

	internalconfig "codenerd/internal/config"
	"codenerd/internal/learnjudge"
	"codenerd/internal/logging"
	"codenerd/internal/types"
	"codenerd/internal/typesafe"
)

// Jev's autopoiesis checks (internal/learnjudge/autopoiesis.go): Jev reads a
// chat request, and the kernel decides whether it warrants a campaign or a
// persistent agent (policy/jev_autopoiesis.mg); Jev reads a generated tool's
// output, and the quality evaluator's truth table names its issues
// (quality.go). Off, failed or timed out, Jev is no evidence: the keyword
// heuristics decide, and the log says so.

// jevUse is the Jev client and the typesafe section that switches it.
type jevUse struct {
	asker learnjudge.Asker
	cfg   *internalconfig.TypeSafeConfig
}

// on reports whether the autopoiesis checks may ask Jev.
func (j jevUse) on() bool {
	return j.asker != nil && j.cfg.Uses().AutopoiesisChecks
}

// jevUseFrom builds the client from the operator's typesafe section. An absent
// section is Jev off; one that cannot resolve is Jev off with a warning.
func jevUseFrom(cfg *internalconfig.UserConfig) jevUse {
	if cfg == nil || cfg.TypeSafe == nil {
		return jevUse{}
	}
	client, err := typesafe.FromConfig(cfg.TypeSafe)
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("jev: autopoiesis checks off, the keyword heuristics decide: %v", err)
		return jevUse{}
	}
	if client == nil {
		return jevUse{}
	}
	return jevUse{asker: client, cfg: cfg.TypeSafe}
}

// SetJev replaces the Jev the autopoiesis checks ask (tests, and a session
// whose typesafe section changed). A nil asker turns the checks off.
func (o *Orchestrator) SetJev(asker learnjudge.Asker, cfg *internalconfig.TypeSafeConfig) {
	o.mu.Lock()
	o.jev = jevUse{asker: asker, cfg: cfg}
	evaluator := o.evaluator
	o.mu.Unlock()
	if evaluator != nil {
		evaluator.SetOutputJudge(o.toolOutputJudge())
	}
}

// toolOutputJudge is the quality evaluator's reading of a tool's output, or
// nil when the checks are off.
func (o *Orchestrator) toolOutputJudge() OutputJudge {
	o.mu.RLock()
	j := o.jev
	o.mu.RUnlock()
	if !j.on() {
		return nil
	}
	timeout := j.cfg.Uses().TurnJudgmentTimeout
	yes := yesProbability(j.cfg)
	return func(ctx context.Context, f *ExecutionFeedback) (OutputJudgments, error) {
		jctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		p, err := learnjudge.JudgeToolOutput(jctx, j.asker, f.ToolName, f.Input, f.Output)
		if err != nil {
			return OutputJudgments{}, err
		}
		// The same answer carries the per-call tool-value observation
		// (tool_value.go).
		o.recordCallHit(f, p[learnjudge.OutputGivesWhatWasAsked])
		return OutputJudgments{Probabilities: p, Yes: yes}, nil
	}
}

// yesProbability is judgment_yes_percent as a probability.
func yesProbability(cfg *internalconfig.TypeSafeConfig) float64 {
	for _, p := range cfg.Params() {
		if p.Key == "/jev_yes_percent" {
			return float64(p.Value) / 100
		}
	}
	return float64(internalconfig.DefaultJudgmentYesPercent) / 100
}

// RequestSuggestion is what the kernel derives for a chat request: whether to
// suggest a campaign and whether to suggest a persistent agent, each with the
// evidence it rests on ("jev" or "heuristic").
type RequestSuggestion struct {
	Campaign         bool
	CampaignSource   string
	CampaignReason   string
	Persistent       *PersistenceNeed
	PersistentSource string
}

// Kernel predicates (policy/jev_autopoiesis.mg).
const (
	heuristicPredicate           = "autopoiesis_heuristic"
	campaignSuggestedPredicate   = "autopoiesis_campaign_suggested"
	persistentSuggestedPredicate = "persistent_agent_suggested"
)

// SuggestForRequest asks Jev about the request, asserts its judgments and the
// keyword heuristic's matches as evidence, and returns what the kernel
// derives. Jev's judgments outrank the heuristic; the heuristic decides only a
// request Jev did not judge. Without a kernel nothing is derived and nothing
// is suggested.
func (o *Orchestrator) SuggestForRequest(ctx context.Context, input, target string) RequestSuggestion {
	o.mu.RLock()
	kernel, j := o.kernel, o.jev
	o.mu.RUnlock()
	log := logging.Get(logging.CategoryAutopoiesis)
	if kernel == nil {
		log.Debug("autopoiesis checks: no kernel, nothing suggested")
		return RequestSuggestion{}
	}
	for _, pred := range []string{learnjudge.RequestJudgmentPredicate, heuristicPredicate} {
		_ = kernel.Retract(pred)
	}

	complexity := o.complexity.Analyze(ctx, input, target)
	persistence := o.persistence.Analyze(ctx, input)
	var evidence []types.Fact
	if complexity.NeedsCampaign {
		evidence = append(evidence, types.Fact{Predicate: heuristicPredicate, Args: []any{types.MangleAtom("/campaign")}})
	}
	if persistence.NeedsPersistent && len(persistence.Needs) > 0 {
		evidence = append(evidence, types.Fact{Predicate: heuristicPredicate, Args: []any{types.MangleAtom("/persistent")}})
	}

	var judgments map[string]float64
	switch {
	case j.on():
		jctx, cancel := context.WithTimeout(ctx, j.cfg.Uses().TurnJudgmentTimeout)
		began := time.Now()
		var err error
		judgments, err = learnjudge.JudgeRequestScope(jctx, j.asker, input, target)
		cancel()
		if err != nil {
			log.Warn("jev: autopoiesis checks unanswered after %s, the keyword heuristic decides: %v", time.Since(began).Round(time.Millisecond), err)
			judgments = nil
			break
		}
		if err := internalconfig.EnsureParams(kernel, j.cfg.Params()); err != nil {
			log.Warn("jev: thresholds not asserted, the keyword heuristic decides: %v", err)
			judgments = nil
			break
		}
		evidence = append(evidence, learnjudge.RequestScopeFacts(judgments)...)
		log.Info("jev: autopoiesis checks %s in %s", judgmentSummary(judgments), time.Since(began).Round(time.Millisecond))
	case j.asker != nil:
		log.Debug("jev: autopoiesis_checks is off, the keyword heuristic decides")
	default:
		log.Debug("jev: not configured, the keyword heuristic decides the autopoiesis checks")
	}
	if len(evidence) > 0 {
		if err := kernel.AssertBatch(evidence); err != nil {
			log.Error("autopoiesis checks: assert evidence: %v", err)
			return RequestSuggestion{}
		}
	}

	var out RequestSuggestion
	if rows, err := kernel.Query(campaignSuggestedPredicate); err == nil && len(rows) > 0 {
		out.Campaign = true
		out.CampaignSource = sourceOf(rows[0].Args)
		out.CampaignReason = campaignReason(out.CampaignSource, judgments, complexity)
	}
	if rows, err := kernel.Query(persistentSuggestedPredicate); err == nil && len(rows) > 0 && len(rows[0].Args) == 2 {
		out.PersistentSource = strings.TrimPrefix(types.ExtractString(rows[0].Args[1]), "/")
		out.Persistent = persistentNeed(out.PersistentSource, input, judgments, persistence)
	}
	return out
}

func sourceOf(args []any) string {
	if len(args) == 0 {
		return ""
	}
	return strings.TrimPrefix(types.ExtractString(args[0]), "/")
}

// campaignReason says why, in the evidence's own terms.
func campaignReason(source string, judgments map[string]float64, c ComplexityResult) string {
	if source == "jev" {
		return fmt.Sprintf("Jev reads several deliverables (%d%%), staged work (%d%%), many files (%d%%)",
			types.PercentFromRatio(judgments[learnjudge.ScopeSeveralDeliverables]),
			types.PercentFromRatio(judgments[learnjudge.ScopeOrderedStages]),
			types.PercentFromRatio(judgments[learnjudge.ScopeManyFiles]))
	}
	reason := fmt.Sprintf("keyword heuristic: %s (score %.2f)", complexityLevelString(c.Level), c.Score)
	if len(c.Reasons) > 0 {
		reason += fmt.Sprintf("; %v", c.Reasons)
	}
	return reason
}

// persistentNeed describes the suggested agent. Its name and schedule are read
// off the request's words; the decision that one is needed is the kernel's.
func persistentNeed(source, input string, judgments map[string]float64, p PersistenceResult) *PersistenceNeed {
	if source == "jev" {
		lower := strings.ToLower(input)
		prob := judgments[learnjudge.ScopeContinuesAfter]
		return &PersistenceNeed{
			AgentType:  determineAgentType(lower),
			Purpose:    "work that keeps going after this request",
			Schedule:   determineSchedule(lower),
			Confidence: prob,
			Reasoning:  fmt.Sprintf("Jev reads work that continues after the request (%d%%)", types.PercentFromRatio(prob)),
		}
	}
	var best *PersistenceNeed
	for i := range p.Needs {
		if best == nil || p.Needs[i].Confidence > best.Confidence {
			best = &p.Needs[i]
		}
	}
	return best
}

// judgmentSummary is name=percent for the log, in a fixed order.
func judgmentSummary(j map[string]float64) string {
	var parts []string
	for _, name := range []string{learnjudge.ScopeSeveralDeliverables, learnjudge.ScopeOrderedStages, learnjudge.ScopeManyFiles, learnjudge.ScopeContinuesAfter} {
		if p, ok := j[name]; ok {
			parts = append(parts, fmt.Sprintf("%s=%d%%", name, types.PercentFromRatio(p)))
		}
	}
	return strings.Join(parts, " ")
}
