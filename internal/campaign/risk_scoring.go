package campaign

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"codenerd/internal/logging"
)

const (
	defaultRiskGateThreshold       = 70
	defaultRiskIntelligenceTimeout = 45 * time.Second
)

var protectedCampaignRiskRoots = []string{
	"internal/core",
	"internal/mangle",
	"internal/campaign",
	"internal/perception",
	"internal/articulation",
}

// RiskGateMode controls override behavior for campaign risk gating.
type RiskGateMode string

const (
	RiskGateModeAuto       RiskGateMode = "/auto"
	RiskGateModeForceAllow RiskGateMode = "/force_allow"
	RiskGateModeForceBlock RiskGateMode = "/force_block"
)

// RiskGateToggle controls per-gate wiring.
type RiskGateToggle string

const (
	RiskGateToggleAuto     RiskGateToggle = "/auto"
	RiskGateToggleEnabled  RiskGateToggle = "/enabled"
	RiskGateToggleDisabled RiskGateToggle = "/disabled"
)

// RiskGateName identifies one strict gate.
type RiskGateName string

const (
	RiskGateNorthstar RiskGateName = "/northstar"
	RiskGateEdge      RiskGateName = "/edge"
	RiskGateAdvisory  RiskGateName = "/advisory"
)

// RiskGateOutcome describes a single gate result.
type RiskGateOutcome string

const (
	RiskGateOutcomePassed  RiskGateOutcome = "/passed"
	RiskGateOutcomeBlocked RiskGateOutcome = "/blocked"
	RiskGateOutcomeSkipped RiskGateOutcome = "/skipped"
)

type riskGateResolved struct {
	Advisory  bool
	Edge      bool
	Northstar bool
}

// RiskInputSnapshot captures pinned inputs used by deterministic risk scoring.
type RiskInputSnapshot struct {
	CapturedAt time.Time `json:"captured_at"`
	Source     string    `json:"source"`

	TargetPathCount int `json:"target_path_count"`
	TotalPhases     int `json:"total_phases"`
	TotalTasks      int `json:"total_tasks"`

	MaxComplexity string `json:"max_complexity"`

	HighChurnFiles      int `json:"high_churn_files"`
	SafetyWarnings      int `json:"safety_warnings"`
	BlockedActions      int `json:"blocked_actions"`
	ToolGaps            int `json:"tool_gaps"`
	MissingCapabilities int `json:"missing_capabilities"`
	UncoveredPaths      int `json:"uncovered_paths"`
	GatheringErrors     int `json:"gathering_errors"`
	AdvisorySignals     int `json:"advisory_signals"`
}

// CampaignRiskDecision captures deterministic risk scoring and gate resolution.
type CampaignRiskDecision struct {
	Score         int
	Threshold     int
	Gated         bool
	TieBreak      string
	SnapshotID    string
	OverrideLevel string

	Criticality int
	Churn       int
	CoverageGap int
	Centrality  int

	Inputs RiskInputSnapshot

	AdvisoryGateEnabled  bool
	EdgeGateEnabled      bool
	NorthstarGateEnabled bool
}

// RiskGateResult captures one gate execution result.
type RiskGateResult struct {
	Name    RiskGateName    `json:"name"`
	Enabled bool            `json:"enabled"`
	Outcome RiskGateOutcome `json:"outcome"`
	Reason  string          `json:"reason"`
	Data    map[string]any  `json:"data,omitzero"`
}

// RiskGateEvaluation captures full preflight risk gate execution.
type RiskGateEvaluation struct {
	Decision    *CampaignRiskDecision `json:"decision,omitzero"`
	Results     []RiskGateResult      `json:"results,omitzero"`
	Allowed     bool                  `json:"allowed"`
	BlockedBy   RiskGateName          `json:"blocked_by,omitzero"`
	BlockReason string                `json:"block_reason,omitzero"`

	// ProtectedRoots are the protected surfaces this campaign targets. They are
	// what turns advice into a hard stop (campaign_rules.mg Section 13).
	ProtectedRoots []string `json:"protected_roots,omitzero"`

	// Findings are the kernel's grading of the gate results: which are hard
	// stops and which are advisory. See risk_gate_contract.go.
	Findings []RiskFinding `json:"findings,omitzero"`

	// KernelDecided is false when campaign_rules.mg was not loaded and the Go
	// mirror graded the findings instead. Operators should see that difference.
	KernelDecided bool `json:"kernel_decided"`
}

// HardFindings returns the findings that stop a campaign.
func (e *RiskGateEvaluation) HardFindings() []RiskFinding {
	if e == nil {
		return nil
	}
	return hardRiskFindings(e.Findings)
}

// SoftFindings returns the advisory findings that were surfaced but did not
// stop the campaign.
func (e *RiskGateEvaluation) SoftFindings() []RiskFinding {
	if e == nil {
		return nil
	}
	return softRiskFindings(e.Findings)
}

func normalizeRiskGateMode(mode RiskGateMode) RiskGateMode {
	switch mode {
	case RiskGateModeForceAllow, RiskGateModeForceBlock:
		return mode
	default:
		return RiskGateModeAuto
	}
}

func normalizeRiskGateToggle(toggle RiskGateToggle) RiskGateToggle {
	switch toggle {
	case RiskGateToggleEnabled, RiskGateToggleDisabled:
		return toggle
	default:
		return RiskGateToggleAuto
	}
}

func resolveRiskGateEnabled(toggle RiskGateToggle, available bool, autoWiring bool) bool {
	switch normalizeRiskGateToggle(toggle) {
	case RiskGateToggleEnabled:
		return true
	case RiskGateToggleDisabled:
		return false
	default:
		return autoWiring && available
	}
}

func clampRiskThreshold(threshold int) int {
	if threshold < defaultRiskGateThreshold {
		return defaultRiskGateThreshold
	}
	if threshold > 100 {
		return 100
	}
	return threshold
}

func (o *Orchestrator) recomputeRiskGateStateLocked() {
	autoWiring := o.config.EnableRiskAutoWiring
	o.riskGateState = riskGateResolved{
		Advisory:  resolveRiskGateEnabled(o.config.AdvisoryGateToggle, o.advisoryBoard != nil, autoWiring),
		Edge:      resolveRiskGateEnabled(o.config.EdgeGateToggle, o.edgeCaseDetector != nil, autoWiring),
		Northstar: resolveRiskGateEnabled(o.config.NorthstarGateToggle, o.configuredNorthstarObserver != nil, autoWiring),
	}
}

func (o *Orchestrator) refreshRiskGateState() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.recomputeRiskGateStateLocked()
}

func deriveRiskInputSnapshotFromReport(report *IntelligenceReport) RiskInputSnapshot {
	if report == nil {
		return RiskInputSnapshot{
			CapturedAt: time.Now().UTC(),
			Source:     "none",
		}
	}
	return RiskInputSnapshot{
		CapturedAt:          time.Now().UTC(),
		Source:              "intelligence",
		HighChurnFiles:      len(report.HighChurnFiles),
		SafetyWarnings:      len(report.SafetyWarnings),
		BlockedActions:      len(report.BlockedActions),
		ToolGaps:            len(report.ToolGaps),
		MissingCapabilities: len(report.MissingCapabilities),
		UncoveredPaths:      len(report.UncoveredPaths),
		GatheringErrors:     len(report.GatheringErrors),
		AdvisorySignals:     len(report.ShardAdvice),
	}
}

func (o *Orchestrator) runRiskPreflight(ctx context.Context) (*RiskGateEvaluation, error) {
	if o.campaign == nil {
		return nil, nil
	}

	o.recomputeRiskGateStateLocked()
	targetPaths := collectCampaignRiskPaths(o.campaign)
	protectedRoots := detectProtectedCampaignRoots(targetPaths)

	// Everything Go observes goes here; the kernel grades it at the end.
	measured := riskContractFacts{
		campaignID:     o.campaign.ID,
		protectedRoots: protectedRoots,
	}

	if len(protectedRoots) > 0 {
		if o.advisoryBoard == nil {
			o.riskDecision = nil
			o.northstarObserver = nil
			reason := fmt.Sprintf("advisory board not configured for protected campaign surfaces: %s", strings.Join(protectedRoots, ", "))
			measured.gateOutcomes = append(measured.gateOutcomes, RiskGateResult{
				Name:    RiskGateAdvisory,
				Enabled: false,
				Outcome: RiskGateOutcomeBlocked,
				Reason:  reason,
				Data:    map[string]any{"protected_roots": protectedRoots},
			})
			// A protected surface with no reviewer at all is a rejection, not a
			// note: there is no opinion to weigh, so nothing can downgrade it.
			measured.concerns = append(measured.concerns, riskConcern{
				gate: RiskGateAdvisory, severity: riskConcernBlocking, detail: reason,
			})
			return o.finalizeRiskPreflight(nil, measured, len(targetPaths))
		}
		if o.configuredNorthstarObserver == nil {
			o.riskDecision = nil
			o.northstarObserver = nil
			reason := fmt.Sprintf("northstar observer not configured for protected campaign surfaces: %s", strings.Join(protectedRoots, ", "))
			measured.gateOutcomes = append(measured.gateOutcomes, RiskGateResult{
				Name:    RiskGateNorthstar,
				Enabled: false,
				Outcome: RiskGateOutcomeBlocked,
				Reason:  reason,
				Data:    map[string]any{"protected_roots": protectedRoots},
			})
			measured.concerns = append(measured.concerns, riskConcern{
				gate: RiskGateNorthstar, severity: riskConcernBlocking, detail: reason,
			})
			return o.finalizeRiskPreflight(nil, measured, len(targetPaths))
		}
	}

	mode := normalizeRiskGateMode(o.config.RiskGateMode)
	if !o.config.EnableRiskAutoWiring &&
		mode == RiskGateModeAuto &&
		o.config.CampaignRiskOverride == nil {
		o.riskDecision = nil
		o.northstarObserver = nil
		o.emitRiskAudit(EventRiskGateSkipped, "Risk auto-wiring disabled", map[string]any{
			"mode":                 string(mode),
			"enable_auto_wiring":   false,
			"campaign_override":    false,
			"task_overrides_count": len(o.config.TaskRiskOverrides),
		})
		eval := &RiskGateEvaluation{
			Allowed:        true,
			Results:        []RiskGateResult{},
			ProtectedRoots: protectedRoots,
		}
		o.setLastRiskEvaluation(eval)
		return eval, nil
	}

	intel := o.gatherRiskIntelligence(ctx, targetPaths)
	decision := buildCampaignRiskDecision(o.campaign, o.config, o.riskGateState, targetPaths, intel)
	o.riskDecision = decision

	o.emitRiskAudit(EventRiskSnapshotPinned, "Pinned deterministic risk inputs", map[string]any{
		"snapshot_id": decision.SnapshotID,
		"inputs":      decision.Inputs,
	})
	o.emitRiskAudit(EventRiskScoreComputed, "Computed deterministic risk score", map[string]any{
		"score":          decision.Score,
		"threshold":      decision.Threshold,
		"gated":          decision.Gated,
		"tie_break":      decision.TieBreak,
		"override_level": decision.OverrideLevel,
		"criticality":    decision.Criticality,
		"churn":          decision.Churn,
		"coverage_gap":   decision.CoverageGap,
		"centrality":     decision.Centrality,
		"snapshot_id":    decision.SnapshotID,
	})

	if mode == RiskGateModeForceBlock {
		o.northstarObserver = nil
		measured.override = string(RiskGateModeForceBlock)
		measured.gateOutcomes = append(measured.gateOutcomes, RiskGateResult{
			Name:    riskGateOverride,
			Enabled: true,
			Outcome: RiskGateOutcomeBlocked,
			Reason:  "force_block override",
		})
		return o.finalizeRiskPreflight(decision, measured, len(targetPaths))
	}
	if mode == RiskGateModeForceAllow {
		measured.override = string(RiskGateModeForceAllow)
	}

	// When strict gating is off, keep runtime observer disabled so northstar checks
	// don't implicitly run via phase/task hooks.
	if !decision.Gated {
		o.northstarObserver = nil
		o.emitRiskAudit(EventRiskGateSkipped, "Risk below threshold; strict gates disabled", map[string]any{
			"score":     decision.Score,
			"threshold": decision.Threshold,
		})
		eval := &RiskGateEvaluation{
			Decision:       decision,
			Allowed:        true,
			ProtectedRoots: protectedRoots,
		}
		o.setLastRiskEvaluation(eval)
		return eval, nil
	}

	if decision.NorthstarGateEnabled {
		o.northstarObserver = o.configuredNorthstarObserver
		res := o.runNorthstarRiskGate(ctx)
		measured.gateOutcomes = append(measured.gateOutcomes, res)
		o.emitRiskAudit(EventRiskGateResult, "Northstar gate evaluated", map[string]any{
			"gate":    string(res.Name),
			"outcome": string(res.Outcome),
			"reason":  res.Reason,
			"data":    res.Data,
		})
	} else {
		o.northstarObserver = nil
	}

	if decision.EdgeGateEnabled {
		res, concerns := o.runEdgeRiskGate(ctx, targetPaths, intel)
		measured.gateOutcomes = append(measured.gateOutcomes, res)
		measured.concerns = append(measured.concerns, concerns...)
		o.emitRiskAudit(EventRiskGateResult, "Edge gate evaluated", map[string]any{
			"gate":    string(res.Name),
			"outcome": string(res.Outcome),
			"reason":  res.Reason,
			"data":    res.Data,
		})
	}

	if decision.AdvisoryGateEnabled {
		res, concerns := o.runAdvisoryRiskGate(ctx, targetPaths, intel)
		measured.gateOutcomes = append(measured.gateOutcomes, res)
		measured.concerns = append(measured.concerns, concerns...)
		o.emitRiskAudit(EventRiskGateResult, "Advisory gate evaluated", map[string]any{
			"gate":    string(res.Name),
			"outcome": string(res.Outcome),
			"reason":  res.Reason,
			"data":    res.Data,
		})
	}

	return o.finalizeRiskPreflight(decision, measured, len(targetPaths))
}

// finalizeRiskPreflight hands the measurements to the kernel, then enforces
// what came back. Go does not decide here; it only acts on campaign_risk_block.
func (o *Orchestrator) finalizeRiskPreflight(decision *CampaignRiskDecision, measured riskContractFacts, targetPathCount int) (*RiskGateEvaluation, error) {
	measured.decision = decision
	findings, kernelDecided := o.classifyRiskGateResults(measured)

	eval := &RiskGateEvaluation{
		Decision:       decision,
		Results:        measured.gateOutcomes,
		Allowed:        true,
		ProtectedRoots: measured.protectedRoots,
		Findings:       findings,
		KernelDecided:  kernelDecided,
	}

	for _, soft := range softRiskFindings(findings) {
		// Soft findings are the whole point of the contract: they must be
		// visible, or the operator only ever learns about risk when work stops.
		o.emitRiskAudit(EventRiskGateAdvisory, "Advisory risk finding (campaign continues)", map[string]any{
			"gate":           string(soft.Gate),
			"reason":         soft.Reason,
			"detail":         soft.Detail,
			"kernel_decided": kernelDecided,
		})
	}

	hard := hardRiskFindings(findings)
	if len(hard) == 0 {
		audit := map[string]any{
			"kernel_decided":  kernelDecided,
			"soft_findings":   len(findings) - len(hard),
			"protected_roots": measured.protectedRoots,
		}
		if decision != nil {
			audit["score"] = decision.Score
			audit["threshold"] = decision.Threshold
			audit["snapshot_id"] = decision.SnapshotID
		}
		o.emitRiskAudit(EventRiskGatePassed, "Strict risk gates passed", audit)
		o.setLastRiskEvaluation(eval)
		return eval, nil
	}

	blocker := hard[0]
	eval.Allowed = false
	eval.BlockedBy = blocker.Gate
	eval.BlockReason = blocker.Detail
	if eval.BlockReason == "" {
		eval.BlockReason = strings.TrimPrefix(blocker.Reason, "/")
	}

	audit := map[string]any{
		"blocked_by":      string(blocker.Gate),
		"reason":          eval.BlockReason,
		"classification":  blocker.Reason,
		"kernel_decided":  kernelDecided,
		"protected_roots": measured.protectedRoots,
		"hard_findings":   len(hard),
	}
	if decision != nil {
		audit["score"] = decision.Score
		audit["threshold"] = decision.Threshold
		audit["snapshot_id"] = decision.SnapshotID
	}
	if targetPathCount > 0 {
		audit["target_path_count"] = targetPathCount
	}
	o.emitRiskAudit(EventRiskGateBlocked, "Campaign blocked by hard risk finding", audit)

	o.setLastRiskEvaluation(eval)
	return eval, &RiskBlockedError{CampaignID: measured.campaignID, Evaluation: eval}
}

// setLastRiskEvaluation runs with o.mu already held by Run.
func (o *Orchestrator) setLastRiskEvaluation(eval *RiskGateEvaluation) {
	o.lastRiskEvaluation = eval
	campaignID := ""
	if o.campaign != nil {
		campaignID = o.campaign.ID
	}
	o.observeRiskPreflight(campaignID, eval)
}

// LastRiskEvaluation returns the most recent preflight evaluation, including
// advisory findings that did not stop the run. Operator surfaces use it to show
// what the gates saw even on a campaign that started successfully.
func (o *Orchestrator) LastRiskEvaluation() *RiskGateEvaluation {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.lastRiskEvaluation
}

func (o *Orchestrator) gatherRiskIntelligence(ctx context.Context, targetPaths []string) *IntelligenceReport {
	if o.intelligenceGatherer == nil || o.campaign == nil {
		return nil
	}

	timeout := o.config.RiskIntelligenceTimeout
	if timeout <= 0 {
		timeout = defaultRiskIntelligenceTimeout
	}

	sampleCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	report, err := o.intelligenceGatherer.Gather(sampleCtx, o.campaign.Goal, targetPaths)
	if err != nil {
		o.emitRiskAudit(EventRiskIntelligenceError, "Failed to gather intelligence for risk scoring", map[string]any{
			"error": err.Error(),
		})
		return &IntelligenceReport{
			GatheringErrors: []string{err.Error()},
			RiskInputs: RiskInputSnapshot{
				CapturedAt:      time.Now().UTC(),
				Source:          "intelligence_error",
				GatheringErrors: 1,
			},
		}
	}

	return report
}

func buildCampaignRiskDecision(c *Campaign, cfg OrchestratorConfig, gates riskGateResolved, paths []string, intel *IntelligenceReport) *CampaignRiskDecision {
	if c == nil {
		return nil
	}
	if paths == nil {
		paths = collectCampaignRiskPaths(c)
	}
	paths = dedupeSortedStrings(paths)
	inputs := buildRiskInputSnapshot(c, paths, intel)
	metrics := calculateRiskMetrics(c, paths, inputs)

	score := weightedRiskScore(
		metrics.criticality,
		metrics.churn,
		metrics.coverageGap,
		metrics.centrality,
		metrics.complexityNorm,
		metrics.safetyNorm,
		metrics.capabilityNorm,
		metrics.errorNorm,
	)

	threshold := clampRiskThreshold(cfg.RiskGateThreshold)
	gated, tieBreak := applyRiskThreshold(score, threshold, inputs, gates)

	overrideGated, overrideLevel := determineRiskOverrideLevel(cfg)
	gated = applyRiskOverride(gated, overrideGated)

	return &CampaignRiskDecision{
		Score:         score,
		Threshold:     threshold,
		Gated:         gated,
		TieBreak:      tieBreak,
		SnapshotID:    riskSnapshotID(c, paths),
		OverrideLevel: overrideLevel,

		Criticality: metrics.criticality,
		Churn:       metrics.churn,
		CoverageGap: metrics.coverageGap,
		Centrality:  metrics.centrality,
		Inputs:      inputs,

		AdvisoryGateEnabled:  gates.Advisory,
		EdgeGateEnabled:      gates.Edge,
		NorthstarGateEnabled: gates.Northstar,
	}
}

func buildRiskInputSnapshot(c *Campaign, paths []string, intel *IntelligenceReport) RiskInputSnapshot {
	inputs := deriveRiskInputSnapshotFromReport(intel)
	inputs.CapturedAt = time.Now().UTC()
	inputs.TargetPathCount = len(paths)
	inputs.TotalPhases = len(c.Phases)
	inputs.TotalTasks = c.TotalTasks
	inputs.MaxComplexity = campaignMaxComplexity(c)
	inputs.Source = "campaign+intelligence"
	if intel == nil {
		inputs.Source = "campaign_only"
	}
	return inputs
}

type riskMetrics struct {
	criticality    int
	churn          int
	coverageGap    int
	centrality     int
	complexityNorm int
	safetyNorm     int
	capabilityNorm int
	errorNorm      int
}

func calculateRiskMetrics(c *Campaign, paths []string, inputs RiskInputSnapshot) riskMetrics {
	criticality := criticalityNorm(paths)
	churnBase := percentileNorm(len(paths), []int{1, 3, 5, 8, 13, 21, 34, 55, 89})
	churnIntel := clampInt(inputs.HighChurnFiles*10, 0, 100)
	churn := clampInt(int(math.Round(0.7*float64(churnBase)+0.3*float64(churnIntel))), 0, 100)

	coverageBase := clampInt(100-coverageFromPlan(c), 0, 100)
	coverageGap := clampInt(coverageBase+clampInt(inputs.UncoveredPaths*4, 0, 24), 0, 100)

	centrality := percentileNorm(len(c.Phases)+len(paths), []int{1, 2, 3, 5, 8, 13, 21, 34, 55})
	complexityNorm := complexityToNorm(inputs.MaxComplexity)
	safetyNorm := clampInt(inputs.SafetyWarnings*18+inputs.BlockedActions*22, 0, 100)
	capabilityNorm := clampInt(inputs.ToolGaps*12+inputs.MissingCapabilities*10, 0, 100)
	errorNorm := clampInt(inputs.GatheringErrors*20, 0, 100)

	return riskMetrics{criticality, churn, coverageGap, centrality, complexityNorm, safetyNorm, capabilityNorm, errorNorm}
}

func determineRiskOverrideLevel(cfg OrchestratorConfig) (*bool, string) {
	mode := normalizeRiskGateMode(cfg.RiskGateMode)
	switch mode {
	case RiskGateModeForceBlock:
		b := true
		return &b, "mode_force_block"
	case RiskGateModeForceAllow:
		b := false
		return &b, "mode_force_allow"
	default:
		if cfg.CampaignRiskOverride != nil {
			return cfg.CampaignRiskOverride, "campaign_override"
		} else if !cfg.GlobalRiskGate {
			b := false
			return &b, "global_override_disabled"
		}
	}
	return nil, "score_threshold"
}

func applyRiskOverride(gated bool, overrideGated *bool) bool {
	if overrideGated != nil {
		return *overrideGated
	}
	return gated
}
func applyRiskThreshold(score, threshold int, inputs RiskInputSnapshot, gates riskGateResolved) (bool, string) {
	if score > threshold {
		return true, "above_threshold"
	}
	if score < threshold {
		return false, "below_threshold"
	}

	criticalSignals := inputs.SafetyWarnings + inputs.BlockedActions + inputs.GatheringErrors
	if strings.EqualFold(inputs.MaxComplexity, "/critical") {
		criticalSignals++
	}
	if criticalSignals > 0 {
		return true, "equal_threshold_critical_signals"
	}

	// Deterministic tie-break precedence: northstar > edge > advisory.
	if gates.Northstar {
		return true, "equal_threshold_northstar_tiebreak"
	}
	if gates.Edge {
		return true, "equal_threshold_edge_tiebreak"
	}
	if gates.Advisory {
		return true, "equal_threshold_advisory_tiebreak"
	}
	return false, "equal_threshold_no_gate_enabled"
}

// selectBlockingRiskGate used to pick the blocker straight from the gate
// results, which made every blocked gate equally fatal. Gate precedence now
// lives in riskGatePrecedence (risk_gate_contract.go) and applies only AFTER
// the kernel has separated hard findings from advisory ones.

func (o *Orchestrator) runNorthstarRiskGate(ctx context.Context) RiskGateResult {
	if o.northstarObserver == nil || o.campaign == nil {
		return RiskGateResult{
			Name:    RiskGateNorthstar,
			Enabled: false,
			Outcome: RiskGateOutcomeSkipped,
			Reason:  "northstar observer not configured",
		}
	}
	if err := o.northstarObserver.StartCampaign(ctx, o.campaign.ID, o.campaign.Goal); err != nil {
		return RiskGateResult{
			Name:    RiskGateNorthstar,
			Enabled: true,
			Outcome: RiskGateOutcomeBlocked,
			Reason:  err.Error(),
		}
	}
	return RiskGateResult{
		Name:    RiskGateNorthstar,
		Enabled: true,
		Outcome: RiskGateOutcomePassed,
		Reason:  "northstar campaign start alignment passed",
	}
}

// runEdgeRiskGate returns the gate outcome plus the graded concerns behind it.
// "The detector wants prework" carries no concern of its own, so on an ordinary
// surface the kernel grades it /advisory_only — it is a recommendation about
// how to sequence work, not a safety verdict. A detector that could not run at
// all is /unapproved, which is hard on protected surfaces and soft elsewhere.
func (o *Orchestrator) runEdgeRiskGate(ctx context.Context, targetPaths []string, intel *IntelligenceReport) (RiskGateResult, []riskConcern) {
	if o.edgeCaseDetector == nil {
		return RiskGateResult{
			Name:    RiskGateEdge,
			Enabled: false,
			Outcome: RiskGateOutcomeSkipped,
			Reason:  "edge case detector not configured",
		}, nil
	}

	analysis, err := o.edgeCaseDetector.AnalyzeForCampaign(ctx, targetPaths, intel)
	if err != nil {
		reason := fmt.Sprintf("edge analysis failed: %v", err)
		return RiskGateResult{
				Name:    RiskGateEdge,
				Enabled: true,
				Outcome: RiskGateOutcomeBlocked,
				Reason:  reason,
			}, []riskConcern{{
				gate: RiskGateEdge, severity: riskConcernUnapproved, detail: reason,
			}}
	}
	if analysis == nil {
		return RiskGateResult{
			Name:    RiskGateEdge,
			Enabled: true,
			Outcome: RiskGateOutcomeSkipped,
			Reason:  "edge analysis returned nil",
		}, nil
	}
	if analysis.HasBlockingIssues() {
		return RiskGateResult{
			Name:    RiskGateEdge,
			Enabled: true,
			Outcome: RiskGateOutcomeBlocked,
			Reason:  fmt.Sprintf("edge analysis detected blocking pre-work (%d files)", analysis.RequiresPrework),
			Data: map[string]any{
				"requires_prework": analysis.RequiresPrework,
				"modularize_files": len(analysis.ModularizeFiles),
				"refactor_files":   len(analysis.RefactorFiles),
			},
		}, nil
	}
	return RiskGateResult{
		Name:    RiskGateEdge,
		Enabled: true,
		Outcome: RiskGateOutcomePassed,
		Reason:  "edge analysis passed",
		Data: map[string]any{
			"requires_prework": analysis.RequiresPrework,
			"total_files":      analysis.TotalFiles,
		},
	}, nil
}

// runAdvisoryRiskGate returns the gate outcome and the graded advisory
// concerns. The grading is what makes the hard path credible:
//
//	/blocking          a critical advisor voted REJECT — hard everywhere
//	/requires_changes  a critical advisor wants changes — advice off protected surfaces
//	/unapproved        no consensus, or the consultation failed — advice off
//	                   protected surfaces, hard on them (you cannot ship an
//	                   unreviewed change to the kernel because the reviewer
//	                   timed out)
func (o *Orchestrator) runAdvisoryRiskGate(ctx context.Context, targetPaths []string, intel *IntelligenceReport) (RiskGateResult, []riskConcern) {
	if o.advisoryBoard == nil || o.campaign == nil {
		return RiskGateResult{
			Name:    RiskGateAdvisory,
			Enabled: false,
			Outcome: RiskGateOutcomeSkipped,
			Reason:  "advisory board not configured",
		}, nil
	}

	advisoryPhases := make([]AdvisoryPhase, 0, len(o.campaign.Phases))
	for _, phase := range o.campaign.Phases {
		desc := phase.Name
		if len(phase.Objectives) > 0 && strings.TrimSpace(phase.Objectives[0].Description) != "" {
			desc = phase.Objectives[0].Description
		}
		advisoryPhases = append(advisoryPhases, AdvisoryPhase{
			ID:          phase.ID,
			Name:        phase.Name,
			Description: desc,
			TaskCount:   len(phase.Tasks),
		})
	}

	req := AdvisoryRequest{
		CampaignID:   o.campaign.ID,
		Goal:         o.campaign.Goal,
		RawPlan:      o.campaign.Title,
		Phases:       advisoryPhases,
		TaskCount:    o.campaign.TotalTasks,
		TargetPaths:  targetPaths,
		Intelligence: intel,
	}
	responses, err := o.advisoryBoard.ConsultAdvisors(ctx, req)
	if err != nil {
		reason := fmt.Sprintf("advisory consultation failed: %v", err)
		return RiskGateResult{
				Name:    RiskGateAdvisory,
				Enabled: true,
				Outcome: RiskGateOutcomeBlocked,
				Reason:  reason,
			}, []riskConcern{{
				gate: RiskGateAdvisory, severity: riskConcernUnapproved, detail: reason,
			}}
	}

	synthesis := o.advisoryBoard.SynthesizeVotes(responses)
	if !synthesis.Approved || len(synthesis.BlockingConcerns) > 0 {
		return RiskGateResult{
			Name:    RiskGateAdvisory,
			Enabled: true,
			Outcome: RiskGateOutcomeBlocked,
			Reason:  synthesis.Summary,
			Data: map[string]any{
				"approval_ratio":     synthesis.ApprovalRatio,
				"blocking_concerns":  len(synthesis.BlockingConcerns),
				"overall_confidence": synthesis.OverallConfidence,
			},
		}, gradeAdvisoryConcerns(synthesis)
	}
	return RiskGateResult{
		Name:    RiskGateAdvisory,
		Enabled: true,
		Outcome: RiskGateOutcomePassed,
		Reason:  "advisory synthesis approved",
		Data: map[string]any{
			"approval_ratio":     synthesis.ApprovalRatio,
			"overall_confidence": synthesis.OverallConfidence,
		},
	}, nil
}

// gradeAdvisoryConcerns maps AdvisorySynthesis onto the severities the kernel
// grades. Only the strongest severity present is emitted, so one REJECT is not
// diluted by three requests-for-changes.
func gradeAdvisoryConcerns(synthesis AdvisorySynthesis) []riskConcern {
	rejected := false
	changes := false
	detail := strings.TrimSpace(synthesis.Summary)
	for _, bc := range synthesis.BlockingConcerns {
		switch bc.Severity {
		case "blocking":
			rejected = true
			if strings.TrimSpace(bc.Concern) != "" {
				detail = fmt.Sprintf("%s (%s): %s", bc.Advisor, bc.Severity, bc.Concern)
			}
		case "requires_changes":
			changes = true
		}
	}

	switch {
	case rejected:
		return []riskConcern{{gate: RiskGateAdvisory, severity: riskConcernBlocking, detail: detail}}
	case changes:
		return []riskConcern{{gate: RiskGateAdvisory, severity: riskConcernRequiresChanges, detail: detail}}
	default:
		return []riskConcern{{gate: RiskGateAdvisory, severity: riskConcernUnapproved, detail: detail}}
	}
}

func (o *Orchestrator) emitRiskAudit(eventType OrchestratorEventType, message string, data map[string]any) {
	o.emitEvent(eventType, "", "", message, data)
	logging.Campaign("RISK_AUDIT %s: %s", eventType, message)
}

func (o *Orchestrator) computeCampaignRiskDecision() *CampaignRiskDecision {
	o.mu.RLock()
	defer o.mu.RUnlock()
	c := o.campaign
	cfg := o.config
	gates := o.riskGateState
	if c == nil {
		return nil
	}
	paths := collectCampaignRiskPaths(c)
	return buildCampaignRiskDecision(c, cfg, gates, paths, nil)
}

func (o *Orchestrator) shouldGateTask(taskID string) bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	decision := o.riskDecision
	cfg := o.config

	// Task-level overrides have highest precedence.
	if cfg.TaskRiskOverrides != nil {
		if v, ok := cfg.TaskRiskOverrides[taskID]; ok {
			return v
		}
	}

	// Explicit mode overrides are next.
	mode := normalizeRiskGateMode(cfg.RiskGateMode)
	switch mode {
	case RiskGateModeForceBlock:
		return true
	case RiskGateModeForceAllow:
		return false
	}

	// Campaign-level override beats global defaults in auto mode.
	if cfg.CampaignRiskOverride != nil {
		return *cfg.CampaignRiskOverride
	}

	// Auto-wiring risk gates are enforced in preflight. After a successful preflight
	// we do not hard-block mutating tasks unless explicitly overridden above.
	if !cfg.EnableRiskAutoWiring || decision == nil {
		return false
	}
	return false
}

func weightedRiskScore(
	criticality, churn, coverageGap, centrality,
	complexityNorm, safetyNorm, capabilityNorm, errorNorm int,
) int {
	score := 0.20*float64(criticality) +
		0.14*float64(churn) +
		0.13*float64(coverageGap) +
		0.10*float64(centrality) +
		0.12*float64(complexityNorm) +
		0.17*float64(safetyNorm) +
		0.09*float64(capabilityNorm) +
		0.05*float64(errorNorm)
	return clampInt(int(math.Round(score)), 0, 100)
}

func complexityToNorm(complexity string) int {
	switch strings.ToLower(strings.TrimSpace(complexity)) {
	case "/critical", "critical":
		return 100
	case "/high", "high":
		return 75
	case "/medium", "medium":
		return 50
	case "/low", "low":
		return 25
	default:
		return 40
	}
}

func campaignMaxComplexity(c *Campaign) string {
	if c == nil || len(c.Phases) == 0 {
		return "/medium"
	}
	best := 0
	label := "/medium"
	for _, phase := range c.Phases {
		norm := complexityToNorm(phase.EstimatedComplexity)
		if norm > best {
			best = norm
			label = strings.ToLower(strings.TrimSpace(phase.EstimatedComplexity))
			if label == "" {
				label = "/medium"
			}
		}
	}
	if !strings.HasPrefix(label, "/") {
		label = "/" + label
	}
	return label
}

func criticalityNorm(paths []string) int {
	protectedRoots := protectedCampaignRiskRoots
	apiRoots := []string{
		"internal/api",
		"internal/models",
		"internal/router",
	}
	sharedRoots := []string{
		"internal/world",
		"internal/store",
		"internal/tools",
	}

	for _, p := range paths {
		for _, root := range protectedRoots {
			if strings.Contains(strings.ToLower(p), strings.ToLower(root)) {
				return 100
			}
		}
	}
	for _, p := range paths {
		for _, root := range apiRoots {
			if strings.Contains(strings.ToLower(p), strings.ToLower(root)) {
				return 70
			}
		}
	}
	for _, p := range paths {
		for _, root := range sharedRoots {
			if strings.Contains(strings.ToLower(p), strings.ToLower(root)) {
				return 40
			}
		}
	}
	return 10
}

func detectProtectedCampaignRoots(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}

	matched := make(map[string]struct{}, len(protectedCampaignRiskRoots))
	for _, candidate := range paths {
		path := normalizeRiskPathForMatch(candidate)
		if path == "" {
			continue
		}
		for _, root := range protectedCampaignRiskRoots {
			if pathMatchesRiskRoot(path, root) {
				matched[root] = struct{}{}
			}
		}
	}

	if len(matched) == 0 {
		return nil
	}

	roots := make([]string, 0, len(matched))
	for root := range matched {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	return roots
}

func normalizeRiskPathForMatch(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	normalized := strings.ToLower(normalizePath(path))
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.Trim(normalized, "/")
	return normalized
}

func pathMatchesRiskRoot(path, root string) bool {
	if path == "" || root == "" {
		return false
	}
	root = normalizeRiskPathForMatch(root)
	if root == "" {
		return false
	}
	if path == root || strings.HasPrefix(path, root+"/") {
		return true
	}
	if strings.Contains(path, "/"+root+"/") {
		return true
	}
	return strings.HasSuffix(path, "/"+root)
}

func coverageFromPlan(c *Campaign) int {
	if c == nil || c.TotalTasks == 0 {
		return 50
	}
	testish := 0
	total := 0
	for _, phase := range c.Phases {
		for _, task := range phase.Tasks {
			total++
			if task.Type == TaskTypeTestWrite || task.Type == TaskTypeTestRun || task.Type == TaskTypeVerify {
				testish++
			}
		}
	}
	if total == 0 {
		return 50
	}
	return clampInt(int(math.Round(100*float64(testish)/float64(total))), 0, 100)
}

func percentileNorm(x int, distribution []int) int {
	if len(distribution) == 0 {
		return 50
	}
	sorted := append([]int(nil), distribution...)
	sort.Ints(sorted)
	less := 0
	equal := 0
	for _, v := range sorted {
		if v < x {
			less++
		} else if v == x {
			equal++
		}
	}
	p := (float64(less) + 0.5*float64(equal)) / float64(len(sorted))
	return clampInt(int(math.Round(100*p)), 0, 100)
}

func riskSnapshotID(c *Campaign, paths []string) string {
	id := c.ID + "|" + strings.Join(paths, "|") + "|" + string(c.Status)
	if len(id) > 128 {
		return id[:128]
	}
	return id
}

func collectCampaignRiskPaths(c *Campaign) []string {
	if c == nil {
		return nil
	}
	paths := make([]string, 0)
	paths = append(paths, c.SourceMaterial...)
	for _, phase := range c.Phases {
		for _, task := range phase.Tasks {
			paths = append(paths, task.DeterministicWriteSet()...)
			for _, ws := range task.WriteSet {
				if strings.TrimSpace(ws) != "" {
					paths = append(paths, normalizePath(ws))
				}
			}
			for _, a := range task.Artifacts {
				if strings.TrimSpace(a.Path) != "" {
					paths = append(paths, normalizePath(a.Path))
				}
			}
		}
	}
	return dedupeSortedStrings(paths)
}

func dedupeSortedStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	tmp := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		tmp = append(tmp, s)
	}
	sort.Strings(tmp)
	return tmp
}

func clampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}
