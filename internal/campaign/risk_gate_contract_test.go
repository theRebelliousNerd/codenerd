package campaign

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/tactile"
)

// The hard/soft contract lives in campaign_rules.mg Section 13. These tests
// drive it two ways: through a real kernel (the authority) and through the Go
// mirror (the fallback when the rules are not loaded), and assert the two agree.
// If they ever diverge, a deployment without campaign_rules.mg would enforce a
// different policy than one with it — the worst kind of safety difference,
// because nothing about the campaign looks different.

func riskContractCampaign(id string, paths ...string) *Campaign {
	tasks := make([]Task, 0, len(paths))
	for i, p := range paths {
		tasks = append(tasks, Task{
			ID:       id + "_task",
			PhaseID:  id + "_phase",
			Status:   TaskPending,
			Type:     TaskTypeFileModify,
			Order:    i,
			WriteSet: []string{p},
		})
	}
	return &Campaign{
		ID:          id,
		Title:       "risk contract",
		Status:      StatusActive,
		TotalPhases: 1,
		TotalTasks:  len(tasks),
		Phases: []Phase{{
			ID:         id + "_phase",
			CampaignID: id,
			Name:       "work",
			Status:     PhasePending,
			Tasks:      tasks,
		}},
	}
}

func riskContractOrchestrator(t *testing.T, kernel core.Kernel, c *Campaign) *Orchestrator {
	t.Helper()
	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace:    t.TempDir(),
		Kernel:       kernel,
		LLMClient:    &MockLLMClient{},
		TaskExecutor: &MockTaskExecutor{},
		Executor:     tactile.NewDirectExecutor(),
		VirtualStore: &core.VirtualStore{},
		EventChan:    make(chan OrchestratorEvent, 64),
	})
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	orch.campaign = c
	return orch
}

func newRiskContractKernel(t *testing.T) core.Kernel {
	t.Helper()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Skipf("real kernel unavailable in this environment: %v", err)
	}
	return kernel
}

// A blocked gate on an ordinary surface is advice: the campaign runs.
func TestRiskClassification_WhenGateBlockedOnOrdinarySurface_ShouldBeSoft(t *testing.T) {
	kernel := newRiskContractKernel(t)
	c := riskContractCampaign("/campaign_soft", "internal/widgets/thing.go")
	orch := riskContractOrchestrator(t, kernel, c)

	measured := riskContractFacts{
		campaignID: c.ID,
		gateOutcomes: []RiskGateResult{{
			Name: RiskGateEdge, Enabled: true, Outcome: RiskGateOutcomeBlocked,
			Reason: "edge analysis detected blocking pre-work (3 files)",
		}},
		decision: &CampaignRiskDecision{Score: 40, Threshold: 70, Gated: false},
	}

	findings, kernelDecided := orch.classifyRiskGateResults(measured)
	if !kernelDecided {
		t.Fatal("campaign_rules.mg Section 13 did not derive; the kernel is not the authority here")
	}
	if got := len(hardRiskFindings(findings)); got != 0 {
		t.Fatalf("edge prework on an ordinary surface must not be a hard stop; got %d hard findings: %+v", got, findings)
	}
	soft := softRiskFindings(findings)
	if len(soft) != 1 || soft[0].Gate != RiskGateEdge {
		t.Fatalf("expected one soft edge finding, got %+v", findings)
	}
	if soft[0].Detail == "" {
		t.Error("soft finding lost the gate's own explanation; the operator would see only an atom")
	}
}

// The same blocked gate on a protected surface is a hard stop.
func TestRiskClassification_WhenGateBlockedOnProtectedSurface_ShouldBeHard(t *testing.T) {
	kernel := newRiskContractKernel(t)
	c := riskContractCampaign("/campaign_protected", "internal/core/kernel_init.go")
	orch := riskContractOrchestrator(t, kernel, c)

	measured := riskContractFacts{
		campaignID:     c.ID,
		protectedRoots: []string{"internal/core"},
		gateOutcomes: []RiskGateResult{{
			Name: RiskGateEdge, Enabled: true, Outcome: RiskGateOutcomeBlocked,
			Reason: "edge analysis detected blocking pre-work (3 files)",
		}},
		decision: &CampaignRiskDecision{Score: 90, Threshold: 70, Gated: true},
	}

	findings, kernelDecided := orch.classifyRiskGateResults(measured)
	if !kernelDecided {
		t.Fatal("campaign_rules.mg Section 13 did not derive")
	}
	hard := hardRiskFindings(findings)
	if len(hard) == 0 {
		t.Fatalf("a blocked gate on internal/core must be a hard stop; got %+v", findings)
	}
	if hard[0].Reason != "/protected_surface" {
		t.Fatalf("expected /protected_surface classification, got %q", hard[0].Reason)
	}
}

// A critical advisor voting REJECT is hard everywhere; requesting changes is not.
func TestRiskClassification_AdvisorySeverities_ShouldGradeIndependently(t *testing.T) {
	kernel := newRiskContractKernel(t)

	cases := []struct {
		name     string
		severity string
		wantHard bool
	}{
		{"reject is hard", riskConcernBlocking, true},
		{"requires_changes is advice", riskConcernRequiresChanges, false},
		{"no consensus is advice", riskConcernUnapproved, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := riskContractCampaign("/campaign_adv_"+strings.TrimPrefix(tc.severity, "/"), "cmd/app/main.go")
			orch := riskContractOrchestrator(t, kernel, c)

			measured := riskContractFacts{
				campaignID: c.ID,
				gateOutcomes: []RiskGateResult{{
					Name: RiskGateAdvisory, Enabled: true, Outcome: RiskGateOutcomeBlocked,
					Reason: "advisory synthesis did not approve",
				}},
				concerns: []riskConcern{{
					gate: RiskGateAdvisory, severity: tc.severity, detail: "advisor said so",
				}},
				decision: &CampaignRiskDecision{Score: 50, Threshold: 70, Gated: false},
			}

			findings, kernelDecided := orch.classifyRiskGateResults(measured)
			if !kernelDecided {
				t.Fatal("campaign_rules.mg Section 13 did not derive")
			}
			gotHard := len(hardRiskFindings(findings)) > 0
			if gotHard != tc.wantHard {
				t.Fatalf("severity %s: hard=%v, want %v (findings %+v)", tc.severity, gotHard, tc.wantHard, findings)
			}
		})
	}
}

// Safety signals escalate an ordinary blocked gate once the deterministic score
// is already over the threshold.
func TestRiskClassification_WhenGatedWithCriticalSignals_ShouldBeHard(t *testing.T) {
	kernel := newRiskContractKernel(t)
	c := riskContractCampaign("/campaign_signals", "internal/widgets/thing.go")
	orch := riskContractOrchestrator(t, kernel, c)

	measured := riskContractFacts{
		campaignID: c.ID,
		gateOutcomes: []RiskGateResult{{
			Name: RiskGateEdge, Enabled: true, Outcome: RiskGateOutcomeBlocked, Reason: "prework",
		}},
		decision: &CampaignRiskDecision{
			Score: 88, Threshold: 70, Gated: true,
			Inputs: RiskInputSnapshot{SafetyWarnings: 2},
		},
	}

	findings, _ := orch.classifyRiskGateResults(measured)
	hard := hardRiskFindings(findings)
	if len(hard) == 0 {
		t.Fatalf("a gated campaign with safety warnings must escalate; got %+v", findings)
	}
	if hard[0].Reason != "/gated_with_critical_signals" {
		t.Fatalf("expected /gated_with_critical_signals, got %q", hard[0].Reason)
	}
}

// The Go mirror only runs when campaign_rules.mg is absent, so it must reach the
// same verdict the kernel would. A silent divergence would make safety policy
// depend on whether a policy file happened to load.
func TestRiskClassification_KernelAndMirror_ShouldAgree(t *testing.T) {
	kernel := newRiskContractKernel(t)

	scenarios := []struct {
		name     string
		measured riskContractFacts
	}{
		{"soft edge on ordinary surface", riskContractFacts{
			gateOutcomes: []RiskGateResult{{Name: RiskGateEdge, Outcome: RiskGateOutcomeBlocked, Reason: "prework"}},
			decision:     &CampaignRiskDecision{Score: 30, Threshold: 70},
		}},
		{"hard on protected surface", riskContractFacts{
			protectedRoots: []string{"internal/mangle"},
			gateOutcomes:   []RiskGateResult{{Name: RiskGateAdvisory, Outcome: RiskGateOutcomeBlocked, Reason: "no"}},
			decision:       &CampaignRiskDecision{Score: 95, Threshold: 70, Gated: true},
		}},
		{"northstar always hard", riskContractFacts{
			gateOutcomes: []RiskGateResult{{Name: RiskGateNorthstar, Outcome: RiskGateOutcomeBlocked, Reason: "drift"}},
			decision:     &CampaignRiskDecision{Score: 10, Threshold: 70},
		}},
		{"critical advisor rejection", riskContractFacts{
			gateOutcomes: []RiskGateResult{{Name: RiskGateAdvisory, Outcome: RiskGateOutcomeBlocked, Reason: "no"}},
			concerns:     []riskConcern{{gate: RiskGateAdvisory, severity: riskConcernBlocking, detail: "no"}},
			decision:     &CampaignRiskDecision{Score: 20, Threshold: 70},
		}},
		{"requires changes only", riskContractFacts{
			gateOutcomes: []RiskGateResult{{Name: RiskGateAdvisory, Outcome: RiskGateOutcomeBlocked, Reason: "meh"}},
			concerns:     []riskConcern{{gate: RiskGateAdvisory, severity: riskConcernRequiresChanges, detail: "meh"}},
			decision:     &CampaignRiskDecision{Score: 20, Threshold: 70},
		}},
		{"force block", riskContractFacts{
			override:     string(RiskGateModeForceBlock),
			gateOutcomes: []RiskGateResult{{Name: riskGateOverride, Outcome: RiskGateOutcomeBlocked, Reason: "force_block override"}},
			decision:     &CampaignRiskDecision{Score: 5, Threshold: 70},
		}},
		{"gated with safety warnings", riskContractFacts{
			gateOutcomes: []RiskGateResult{{Name: RiskGateEdge, Outcome: RiskGateOutcomeBlocked, Reason: "prework"}},
			decision: &CampaignRiskDecision{
				Score: 80, Threshold: 70, Gated: true,
				Inputs: RiskInputSnapshot{BlockedActions: 1},
			},
		}},
		{"everything passed", riskContractFacts{
			gateOutcomes: []RiskGateResult{{Name: RiskGateEdge, Outcome: RiskGateOutcomePassed, Reason: "ok"}},
			decision:     &CampaignRiskDecision{Score: 10, Threshold: 70},
		}},
	}

	for i, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			id := "/campaign_agree_" + string(rune('a'+i))
			c := riskContractCampaign(id, "internal/widgets/thing.go")

			kernelMeasured := sc.measured
			kernelMeasured.campaignID = id
			kernelOrch := riskContractOrchestrator(t, kernel, c)
			kernelFindings, kernelDecided := kernelOrch.classifyRiskGateResults(kernelMeasured)
			if len(kernelMeasured.gateOutcomes) > 0 && !kernelDecided {
				t.Fatal("kernel classification unavailable; the comparison would be meaningless")
			}

			mirrorMeasured := sc.measured
			mirrorMeasured.campaignID = id
			mirrorFindings := mirrorRiskClassification(mirrorMeasured)
			sortRiskFindings(mirrorFindings)

			if !sameFindingSet(kernelFindings, mirrorFindings) {
				t.Errorf("kernel and Go mirror disagree.\n  kernel: %s\n  mirror: %s",
					describeFindings(kernelFindings), describeFindings(mirrorFindings))
			}
		})
	}
}

func sameFindingSet(a, b []RiskFinding) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, f := range a {
		seen[string(f.Gate)+"|"+f.Reason+"|"+string(f.Severity)]++
	}
	for _, f := range b {
		key := string(f.Gate) + "|" + f.Reason + "|" + string(f.Severity)
		seen[key]--
		if seen[key] < 0 {
			return false
		}
	}
	return true
}

func describeFindings(findings []RiskFinding) string {
	if len(findings) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(findings))
	for _, f := range findings {
		parts = append(parts, string(f.Severity)+" "+string(f.Gate)+" "+f.Reason)
	}
	return strings.Join(parts, "; ")
}

// A stale evaluation must not leak into the next run's grading.
func TestRiskClassification_WhenRerun_ShouldNotGradeAgainstPreviousMeasurements(t *testing.T) {
	kernel := newRiskContractKernel(t)
	c := riskContractCampaign("/campaign_rerun", "internal/core/kernel_init.go")
	orch := riskContractOrchestrator(t, kernel, c)

	blocked := riskContractFacts{
		campaignID:     c.ID,
		protectedRoots: []string{"internal/core"},
		gateOutcomes: []RiskGateResult{{
			Name: RiskGateAdvisory, Outcome: RiskGateOutcomeBlocked, Reason: "no",
		}},
		decision: &CampaignRiskDecision{Score: 90, Threshold: 70, Gated: true},
	}
	if findings, _ := orch.classifyRiskGateResults(blocked); len(hardRiskFindings(findings)) == 0 {
		t.Fatal("setup failed: expected the first pass to be a hard block")
	}

	cleared := riskContractFacts{
		campaignID:     c.ID,
		protectedRoots: []string{"internal/core"},
		gateOutcomes: []RiskGateResult{{
			Name: RiskGateAdvisory, Outcome: RiskGateOutcomePassed, Reason: "approved",
		}},
		decision: &CampaignRiskDecision{Score: 90, Threshold: 70, Gated: true},
	}
	findings, _ := orch.classifyRiskGateResults(cleared)
	if hard := hardRiskFindings(findings); len(hard) != 0 {
		t.Fatalf("the previous run's blocked gate still blocks after the advisor approved: %s",
			describeFindings(hard))
	}
}

// Run must refuse to start on a hard finding, and the error must carry the
// evaluation so operator surfaces can render more than a sentence.
func TestRun_WhenRiskPreflightHardBlocks_ShouldReturnRenderableError(t *testing.T) {
	kernel := newRiskContractKernel(t)
	c := riskContractCampaign("/campaign_run_block", "internal/core/kernel_init.go")
	orch := riskContractOrchestrator(t, kernel, c)
	orch.config.EnableRiskAutoWiring = true
	orch.config.GlobalRiskGate = true

	err := orch.Run(context.Background())
	if err == nil {
		t.Fatal("a campaign targeting internal/core with no advisory board must not start")
	}
	if !errors.Is(err, ErrRiskGateBlocked) {
		t.Fatalf("callers branch on ErrRiskGateBlocked; got %v", err)
	}

	eval := RiskEvaluation(err)
	if eval == nil {
		t.Fatal("the error carried no evaluation; the UI would have nothing but a string to show")
	}
	if eval.Allowed {
		t.Fatal("evaluation says allowed but Run refused")
	}
	if len(eval.ProtectedRoots) == 0 {
		t.Error("evaluation lost the protected roots that made this a hard stop")
	}

	rendered, ok := FormatRiskBlock(err)
	if !ok {
		t.Fatal("FormatRiskBlock did not recognise a risk block")
	}
	for _, want := range []string{"Blocked by", "internal/core", "Gates:"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered block is missing %q:\n%s", want, rendered)
		}
	}

	if orch.LastRiskEvaluation() == nil {
		t.Error("LastRiskEvaluation should expose the evaluation after a refused start")
	}
}

// A campaign that starts must still surface its advisory findings.
func TestRun_WhenOnlySoftFindings_ShouldStartAndEmitAdvisories(t *testing.T) {
	kernel := newRiskContractKernel(t)
	c := riskContractCampaign("/campaign_run_soft", "internal/widgets/thing.go")
	orch := riskContractOrchestrator(t, kernel, c)

	measured := riskContractFacts{
		campaignID: c.ID,
		gateOutcomes: []RiskGateResult{{
			Name: RiskGateEdge, Outcome: RiskGateOutcomeBlocked, Reason: "prework wanted",
		}},
		decision: &CampaignRiskDecision{Score: 30, Threshold: 70},
	}

	eval, err := orch.finalizeRiskPreflight(measured.decision, measured, 1)
	if err != nil {
		t.Fatalf("soft findings must not block the campaign: %v", err)
	}
	if !eval.Allowed {
		t.Fatal("evaluation refused a campaign with only advisory findings")
	}
	if len(eval.SoftFindings()) == 0 {
		t.Fatal("advisory finding vanished; an unsurfaced advisory is the same as no gate at all")
	}

	seen := drainEventTypes(orch.eventChan)
	if seen[EventRiskGateAdvisory] == 0 {
		t.Fatalf("expected %s events so the operator sees the advice; got %v", EventRiskGateAdvisory, seen)
	}
}
