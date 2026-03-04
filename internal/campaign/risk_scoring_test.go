package campaign

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/autopoiesis"
	"codenerd/internal/northstar"
)

func TestBuildCampaignRiskDecision_Deterministic(t *testing.T) {
	c := testRiskCampaign()
	cfg := OrchestratorConfig{
		RiskGateThreshold: 70,
		GlobalRiskGate:    true,
		RiskGateMode:      RiskGateModeAuto,
	}
	gates := riskGateResolved{Advisory: true, Edge: true, Northstar: true}
	paths := []string{
		"internal/core/kernel.go",
		"internal/campaign/orchestrator_execution.go",
	}
	intel := &IntelligenceReport{
		HighChurnFiles:      []string{"internal/core/kernel.go"},
		SafetyWarnings:      []SafetyWarning{{Action: "delete", Severity: "high"}},
		BlockedActions:      []string{"rm -rf"},
		ToolGaps:            []autopoiesis.ToolNeed{{Name: "x"}},
		MissingCapabilities: []string{"security-review"},
		UncoveredPaths:      []string{"internal/campaign/orchestrator_execution.go"},
		GatheringErrors:     []string{},
	}

	d1 := buildCampaignRiskDecision(c, cfg, gates, paths, intel)
	d2 := buildCampaignRiskDecision(c, cfg, gates, paths, intel)

	if d1 == nil || d2 == nil {
		t.Fatal("expected non-nil risk decisions")
	}
	if d1.Score != d2.Score {
		t.Fatalf("expected deterministic score, got %d vs %d", d1.Score, d2.Score)
	}
	if d1.SnapshotID != d2.SnapshotID {
		t.Fatalf("expected deterministic snapshot id, got %q vs %q", d1.SnapshotID, d2.SnapshotID)
	}
	if d1.TieBreak != d2.TieBreak {
		t.Fatalf("expected deterministic tie-break, got %q vs %q", d1.TieBreak, d2.TieBreak)
	}
}

func TestBuildCampaignRiskDecision_ThresholdClamp(t *testing.T) {
	c := testRiskCampaign()
	cfg := OrchestratorConfig{
		RiskGateThreshold: 12,
		GlobalRiskGate:    true,
	}
	d := buildCampaignRiskDecision(c, cfg, riskGateResolved{}, nil, nil)
	if d == nil {
		t.Fatal("expected non-nil decision")
	}
	if d.Threshold != defaultRiskGateThreshold {
		t.Fatalf("expected threshold clamp to %d, got %d", defaultRiskGateThreshold, d.Threshold)
	}
}

func TestApplyRiskThreshold_TieBreak(t *testing.T) {
	score := 70
	threshold := 70

	// No critical signals and no gates => not gated on tie.
	gated, reason := applyRiskThreshold(score, threshold, RiskInputSnapshot{}, riskGateResolved{})
	if gated {
		t.Fatalf("expected tie without critical signals to be ungated, reason=%s", reason)
	}

	// Northstar tie-break should gate deterministically.
	gated, reason = applyRiskThreshold(score, threshold, RiskInputSnapshot{}, riskGateResolved{Northstar: true})
	if !gated {
		t.Fatalf("expected northstar tie-break to gate, reason=%s", reason)
	}
	if reason != "equal_threshold_northstar_tiebreak" {
		t.Fatalf("unexpected tie-break reason: %s", reason)
	}
}

func TestBuildCampaignRiskDecision_OverridePrecedence(t *testing.T) {
	c := testRiskCampaign()
	overrideTrue := true
	overrideFalse := false

	d := buildCampaignRiskDecision(c, OrchestratorConfig{
		GlobalRiskGate:       false,
		CampaignRiskOverride: &overrideTrue,
		RiskGateMode:         RiskGateModeAuto,
	}, riskGateResolved{}, nil, nil)
	if !d.Gated || d.OverrideLevel != "campaign_override" {
		t.Fatalf("expected campaign override precedence, got gated=%v level=%s", d.Gated, d.OverrideLevel)
	}

	d = buildCampaignRiskDecision(c, OrchestratorConfig{
		GlobalRiskGate:       true,
		CampaignRiskOverride: &overrideFalse,
		RiskGateMode:         RiskGateModeForceBlock,
	}, riskGateResolved{}, nil, nil)
	if !d.Gated || d.OverrideLevel != "mode_force_block" {
		t.Fatalf("expected force_block precedence, got gated=%v level=%s", d.Gated, d.OverrideLevel)
	}

	d = buildCampaignRiskDecision(c, OrchestratorConfig{
		GlobalRiskGate:       true,
		CampaignRiskOverride: &overrideTrue,
		RiskGateMode:         RiskGateModeForceAllow,
	}, riskGateResolved{}, nil, nil)
	if d.Gated || d.OverrideLevel != "mode_force_allow" {
		t.Fatalf("expected force_allow precedence, got gated=%v level=%s", d.Gated, d.OverrideLevel)
	}
}

func TestRiskGateAutoWiringDeterministic(t *testing.T) {
	observer := newTestNorthstarObserver(t)

	orch := &Orchestrator{
		config: OrchestratorConfig{
			EnableRiskAutoWiring: true,
			AdvisoryGateToggle:   RiskGateToggleAuto,
			EdgeGateToggle:       RiskGateToggleAuto,
			NorthstarGateToggle:  RiskGateToggleAuto,
		},
		advisoryBoard:               &ShardAdvisoryBoard{},
		edgeCaseDetector:            &EdgeCaseDetector{},
		configuredNorthstarObserver: observer,
	}

	orch.recomputeRiskGateStateLocked()
	if !orch.riskGateState.Advisory || !orch.riskGateState.Edge || !orch.riskGateState.Northstar {
		t.Fatalf("expected auto-wiring to enable all configured gates, got %+v", orch.riskGateState)
	}

	orch.config.AdvisoryGateToggle = RiskGateToggleDisabled
	orch.config.EdgeGateToggle = RiskGateToggleDisabled
	orch.config.NorthstarGateToggle = RiskGateToggleEnabled
	orch.recomputeRiskGateStateLocked()
	if orch.riskGateState.Advisory || orch.riskGateState.Edge || !orch.riskGateState.Northstar {
		t.Fatalf("expected explicit toggles to win, got %+v", orch.riskGateState)
	}
}

func TestRunRiskPreflight_EmitsAuditEvents(t *testing.T) {
	eventCh := make(chan OrchestratorEvent, 16)
	orch := &Orchestrator{
		campaign: testRiskCampaign(),
		config: OrchestratorConfig{
			EnableRiskAutoWiring: true,
			GlobalRiskGate:       true,
			RiskGateMode:         RiskGateModeAuto,
			RiskGateThreshold:    100,
		},
		eventChan: eventCh,
	}

	eval, err := orch.runRiskPreflight(context.Background())
	if err != nil {
		t.Fatalf("runRiskPreflight returned error: %v", err)
	}
	if eval == nil || !eval.Allowed {
		t.Fatalf("expected allowed evaluation, got %+v", eval)
	}

	events := drainRiskEvents(eventCh)
	if !events["risk_snapshot_pinned"] {
		t.Fatalf("expected risk_snapshot_pinned event, got %v", events)
	}
	if !events["risk_score_computed"] {
		t.Fatalf("expected risk_score_computed event, got %v", events)
	}
	if !events["risk_gate_skipped"] && !events["risk_gate_passed"] {
		t.Fatalf("expected risk gate terminal event, got %v", events)
	}
}

func TestRunRiskPreflight_ForceBlockOverride(t *testing.T) {
	eventCh := make(chan OrchestratorEvent, 16)
	orch := &Orchestrator{
		campaign: testRiskCampaign(),
		config: OrchestratorConfig{
			EnableRiskAutoWiring: true,
			GlobalRiskGate:       true,
			RiskGateMode:         RiskGateModeForceBlock,
		},
		eventChan: eventCh,
	}

	eval, err := orch.runRiskPreflight(context.Background())
	if err == nil {
		t.Fatal("expected force_block override to block preflight")
	}
	if eval == nil || eval.Allowed {
		t.Fatalf("expected blocked evaluation, got %+v", eval)
	}
	events := drainRiskEvents(eventCh)
	if !events["risk_gate_blocked"] {
		t.Fatalf("expected risk_gate_blocked event, got %v", events)
	}
}

func testRiskCampaign() *Campaign {
	now := time.Now().UTC()
	return &Campaign{
		ID:          "/risk_test",
		Type:        CampaignTypeCustom,
		Title:       "Risk Test",
		Goal:        "Validate deterministic risk scoring",
		Status:      StatusPlanning,
		CreatedAt:   now,
		UpdatedAt:   now,
		TotalPhases: 2,
		TotalTasks:  5,
		Phases: []Phase{
			{
				ID:                  "/phase_1",
				Name:                "Phase 1",
				EstimatedComplexity: "/high",
				Tasks: []Task{
					{
						ID:          "/task_1",
						Type:        TaskTypeRefactor,
						Description: "Refactor internal/core/kernel.go",
						WriteSet:    []string{"internal/core/kernel.go"},
					},
					{
						ID:          "/task_2",
						Type:        TaskTypeTestWrite,
						Description: "Add tests",
						WriteSet:    []string{"internal/campaign/risk_scoring_test.go"},
					},
				},
			},
			{
				ID:                  "/phase_2",
				Name:                "Phase 2",
				EstimatedComplexity: "/medium",
				Tasks: []Task{
					{
						ID:          "/task_3",
						Type:        TaskTypeIntegrate,
						Description: "Integrate risk gates",
						Artifacts: []TaskArtifact{
							{Path: "internal/campaign/orchestrator_execution.go"},
						},
					},
				},
			},
		},
	}
}

func newTestNorthstarObserver(t *testing.T) *northstar.CampaignObserver {
	t.Helper()

	store, err := northstar.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create northstar store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	guardian := northstar.NewGuardian(store, northstar.DefaultGuardianConfig())
	if err := guardian.Initialize(); err != nil {
		t.Fatalf("failed to initialize northstar guardian: %v", err)
	}
	return northstar.NewCampaignObserver(guardian)
}

func drainRiskEvents(ch <-chan OrchestratorEvent) map[string]bool {
	out := map[string]bool{}
	for {
		select {
		case evt := <-ch:
			out[evt.Type] = true
		default:
			return out
		}
	}
}
