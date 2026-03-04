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
	Data    map[string]any  `json:"data,omitempty"`
}

// RiskGateEvaluation captures full preflight risk gate execution.
type RiskGateEvaluation struct {
	Decision    *CampaignRiskDecision `json:"decision,omitempty"`
	Results     []RiskGateResult      `json:"results,omitempty"`
	Allowed     bool                  `json:"allowed"`
	BlockedBy   RiskGateName          `json:"blocked_by,omitempty"`
	BlockReason string                `json:"block_reason,omitempty"`
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
	if len(protectedRoots) > 0 {
		if o.advisoryBoard == nil {
			o.riskDecision = nil
			o.northstarObserver = nil
			reason := fmt.Sprintf("advisory board not configured for protected campaign surfaces: %s", strings.Join(protectedRoots, ", "))
			o.emitRiskAudit("risk_gate_blocked", "Campaign blocked: mandatory advisory safety review missing", map[string]any{
				"blocked_by":        string(RiskGateAdvisory),
				"reason":            reason,
				"protected_roots":   protectedRoots,
				"target_path_count": len(targetPaths),
			})
			return &RiskGateEvaluation{
				Allowed:     false,
				BlockedBy:   RiskGateAdvisory,
				BlockReason: reason,
				Results: []RiskGateResult{{
					Name:    RiskGateAdvisory,
					Enabled: false,
					Outcome: RiskGateOutcomeBlocked,
					Reason:  reason,
					Data: map[string]any{
						"protected_roots": protectedRoots,
					},
				}},
			}, fmt.Errorf("risk gate blocked campaign start (%s): %s", RiskGateAdvisory, reason)
		}
		if o.configuredNorthstarObserver == nil {
			o.riskDecision = nil
			o.northstarObserver = nil
			reason := fmt.Sprintf("northstar observer not configured for protected campaign surfaces: %s", strings.Join(protectedRoots, ", "))
			o.emitRiskAudit("risk_gate_blocked", "Campaign blocked: mandatory northstar safety review missing", map[string]any{
				"blocked_by":        string(RiskGateNorthstar),
				"reason":            reason,
				"protected_roots":   protectedRoots,
				"target_path_count": len(targetPaths),
			})
			return &RiskGateEvaluation{
				Allowed:     false,
				BlockedBy:   RiskGateNorthstar,
				BlockReason: reason,
				Results: []RiskGateResult{{
					Name:    RiskGateNorthstar,
					Enabled: false,
					Outcome: RiskGateOutcomeBlocked,
					Reason:  reason,
					Data: map[string]any{
						"protected_roots": protectedRoots,
					},
				}},
			}, fmt.Errorf("risk gate blocked campaign start (%s): %s", RiskGateNorthstar, reason)
		}
	}

	mode := normalizeRiskGateMode(o.config.RiskGateMode)
	if !o.config.EnableRiskAutoWiring &&
		mode == RiskGateModeAuto &&
		o.config.CampaignRiskOverride == nil {
		o.riskDecision = nil
		o.northstarObserver = nil
		o.emitRiskAudit("risk_gate_skipped", "Risk auto-wiring disabled", map[string]any{
			"mode":                 string(mode),
			"enable_auto_wiring":   false,
			"campaign_override":    false,
			"task_overrides_count": len(o.config.TaskRiskOverrides),
		})
		return &RiskGateEvaluation{
			Allowed: true,
			Results: []RiskGateResult{},
		}, nil
	}

	intel := o.gatherRiskIntelligence(ctx, targetPaths)
	decision := buildCampaignRiskDecision(o.campaign, o.config, o.riskGateState, targetPaths, intel)
	o.riskDecision = decision

	o.emitRiskAudit("risk_snapshot_pinned", "Pinned deterministic risk inputs", map[string]any{
		"snapshot_id": decision.SnapshotID,
		"inputs":      decision.Inputs,
	})
	o.emitRiskAudit("risk_score_computed", "Computed deterministic risk score", map[string]any{
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
		o.emitRiskAudit("risk_gate_blocked", "Campaign blocked by force-block override", map[string]any{
			"score":       decision.Score,
			"threshold":   decision.Threshold,
			"snapshot_id": decision.SnapshotID,
		})
		return &RiskGateEvaluation{
			Decision:    decision,
			Allowed:     false,
			BlockedBy:   "/override",
			BlockReason: "force_block override",
		}, fmt.Errorf("risk gate blocked campaign start (/override): force_block override")
	}

	// When strict gating is off, keep runtime observer disabled so northstar checks
	// don't implicitly run via phase/task hooks.
	if !decision.Gated {
		o.northstarObserver = nil
		o.emitRiskAudit("risk_gate_skipped", "Risk below threshold; strict gates disabled", map[string]any{
			"score":     decision.Score,
			"threshold": decision.Threshold,
		})
		return &RiskGateEvaluation{
			Decision: decision,
			Allowed:  true,
		}, nil
	}

	eval := &RiskGateEvaluation{
		Decision: decision,
		Allowed:  true,
		Results:  make([]RiskGateResult, 0, 3),
	}

	if decision.NorthstarGateEnabled {
		o.northstarObserver = o.configuredNorthstarObserver
		res := o.runNorthstarRiskGate(ctx)
		eval.Results = append(eval.Results, res)
		o.emitRiskAudit("risk_gate_result", "Northstar gate evaluated", map[string]any{
			"gate":    string(res.Name),
			"outcome": string(res.Outcome),
			"reason":  res.Reason,
			"data":    res.Data,
		})
	} else {
		o.northstarObserver = nil
	}

	if decision.EdgeGateEnabled {
		res := o.runEdgeRiskGate(ctx, targetPaths, intel)
		eval.Results = append(eval.Results, res)
		o.emitRiskAudit("risk_gate_result", "Edge gate evaluated", map[string]any{
			"gate":    string(res.Name),
			"outcome": string(res.Outcome),
			"reason":  res.Reason,
			"data":    res.Data,
		})
	}

	if decision.AdvisoryGateEnabled {
		res := o.runAdvisoryRiskGate(ctx, targetPaths, intel)
		eval.Results = append(eval.Results, res)
		o.emitRiskAudit("risk_gate_result", "Advisory gate evaluated", map[string]any{
			"gate":    string(res.Name),
			"outcome": string(res.Outcome),
			"reason":  res.Reason,
			"data":    res.Data,
		})
	}

	if blocker, ok := selectBlockingRiskGate(eval.Results); ok {
		eval.Allowed = false
		eval.BlockedBy = blocker.Name
		eval.BlockReason = blocker.Reason
		o.emitRiskAudit("risk_gate_blocked", "Campaign blocked by strict risk gate", map[string]any{
			"blocked_by":  string(blocker.Name),
			"reason":      blocker.Reason,
			"score":       decision.Score,
			"threshold":   decision.Threshold,
			"snapshot_id": decision.SnapshotID,
		})
		return eval, fmt.Errorf("risk gate blocked campaign start (%s): %s", blocker.Name, blocker.Reason)
	}

	o.emitRiskAudit("risk_gate_passed", "Strict risk gates passed", map[string]any{
		"score":       decision.Score,
		"threshold":   decision.Threshold,
		"snapshot_id": decision.SnapshotID,
	})

	return eval, nil
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
		o.emitRiskAudit("risk_intelligence_error", "Failed to gather intelligence for risk scoring", map[string]any{
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

	reportInputs := deriveRiskInputSnapshotFromReport(intel)
	inputs := reportInputs
	inputs.CapturedAt = time.Now().UTC()
	inputs.TargetPathCount = len(paths)
	inputs.TotalPhases = len(c.Phases)
	inputs.TotalTasks = c.TotalTasks
	inputs.MaxComplexity = campaignMaxComplexity(c)
	inputs.Source = "campaign+intelligence"
	if intel == nil {
		inputs.Source = "campaign_only"
	}

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

	score := weightedRiskScore(
		criticality,
		churn,
		coverageGap,
		centrality,
		complexityNorm,
		safetyNorm,
		capabilityNorm,
		errorNorm,
	)

	threshold := clampRiskThreshold(cfg.RiskGateThreshold)
	gated, tieBreak := applyRiskThreshold(score, threshold, inputs, gates)
	overrideLevel := "score_threshold"

	mode := normalizeRiskGateMode(cfg.RiskGateMode)
	switch mode {
	case RiskGateModeForceBlock:
		gated = true
		overrideLevel = "mode_force_block"
	case RiskGateModeForceAllow:
		gated = false
		overrideLevel = "mode_force_allow"
	default:
		if cfg.CampaignRiskOverride != nil {
			gated = *cfg.CampaignRiskOverride
			overrideLevel = "campaign_override"
		} else if !cfg.GlobalRiskGate {
			gated = false
			overrideLevel = "global_override_disabled"
		}
	}

	return &CampaignRiskDecision{
		Score:         score,
		Threshold:     threshold,
		Gated:         gated,
		TieBreak:      tieBreak,
		SnapshotID:    riskSnapshotID(c, paths),
		OverrideLevel: overrideLevel,

		Criticality: criticality,
		Churn:       churn,
		CoverageGap: coverageGap,
		Centrality:  centrality,
		Inputs:      inputs,

		AdvisoryGateEnabled:  gates.Advisory,
		EdgeGateEnabled:      gates.Edge,
		NorthstarGateEnabled: gates.Northstar,
	}
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

func selectBlockingRiskGate(results []RiskGateResult) (RiskGateResult, bool) {
	if len(results) == 0 {
		return RiskGateResult{}, false
	}
	precedence := map[RiskGateName]int{
		RiskGateNorthstar: 1,
		RiskGateEdge:      2,
		RiskGateAdvisory:  3,
	}

	found := false
	best := RiskGateResult{}
	bestRank := math.MaxInt
	for _, r := range results {
		if r.Outcome != RiskGateOutcomeBlocked {
			continue
		}
		rank, ok := precedence[r.Name]
		if !ok {
			rank = math.MaxInt - 1
		}
		if !found || rank < bestRank {
			best = r
			bestRank = rank
			found = true
		}
	}
	return best, found
}

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

func (o *Orchestrator) runEdgeRiskGate(ctx context.Context, targetPaths []string, intel *IntelligenceReport) RiskGateResult {
	if o.edgeCaseDetector == nil {
		return RiskGateResult{
			Name:    RiskGateEdge,
			Enabled: false,
			Outcome: RiskGateOutcomeSkipped,
			Reason:  "edge case detector not configured",
		}
	}

	analysis, err := o.edgeCaseDetector.AnalyzeForCampaign(ctx, targetPaths, intel)
	if err != nil {
		return RiskGateResult{
			Name:    RiskGateEdge,
			Enabled: true,
			Outcome: RiskGateOutcomeBlocked,
			Reason:  fmt.Sprintf("edge analysis failed: %v", err),
		}
	}
	if analysis == nil {
		return RiskGateResult{
			Name:    RiskGateEdge,
			Enabled: true,
			Outcome: RiskGateOutcomeSkipped,
			Reason:  "edge analysis returned nil",
		}
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
		}
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
	}
}

func (o *Orchestrator) runAdvisoryRiskGate(ctx context.Context, targetPaths []string, intel *IntelligenceReport) RiskGateResult {
	if o.advisoryBoard == nil || o.campaign == nil {
		return RiskGateResult{
			Name:    RiskGateAdvisory,
			Enabled: false,
			Outcome: RiskGateOutcomeSkipped,
			Reason:  "advisory board not configured",
		}
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
		return RiskGateResult{
			Name:    RiskGateAdvisory,
			Enabled: true,
			Outcome: RiskGateOutcomeBlocked,
			Reason:  fmt.Sprintf("advisory consultation failed: %v", err),
		}
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
		}
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
	}
}

func (o *Orchestrator) emitRiskAudit(eventType, message string, data map[string]any) {
	o.emitEvent(eventType, "", "", message, data)
	logging.Campaign("RISK_AUDIT %s: %s", eventType, message)
}

func (o *Orchestrator) computeCampaignRiskDecision() *CampaignRiskDecision {
	o.mu.RLock()
	c := o.campaign
	cfg := o.config
	gates := o.riskGateState
	o.mu.RUnlock()
	if c == nil {
		return nil
	}
	paths := collectCampaignRiskPaths(c)
	return buildCampaignRiskDecision(c, cfg, gates, paths, nil)
}

func (o *Orchestrator) shouldGateTask(taskID string) bool {
	o.mu.RLock()
	decision := o.riskDecision
	cfg := o.config
	o.mu.RUnlock()

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
