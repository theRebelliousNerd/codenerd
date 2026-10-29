package campaign

import (
	"context"
	"testing"

	"codenerd/internal/northstar"
)

// alignmentLLM answers every alignment completion with a fixed response.
type alignmentLLM struct{ response string }

func (a alignmentLLM) CompleteWithSystem(context.Context, string, string) (string, error) {
	return a.response, nil
}

func northstarGateObserver(t *testing.T, withVision bool, llm northstar.LLMClient) *northstar.CampaignObserver {
	t.Helper()
	store, err := northstar.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	guardian := northstar.NewGuardian(store, northstar.DefaultGuardianConfig())
	if err := guardian.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if withVision {
		if err := guardian.UpdateVision(&northstar.Vision{Mission: "Ship trustworthy changes"}); err != nil {
			t.Fatalf("UpdateVision: %v", err)
		}
	}
	if llm != nil {
		guardian.SetLLMClient(llm)
	}
	return northstar.NewCampaignObserver(guardian)
}

// NS-VISION-04: the campaign risk gate grades the alignment check instead of
// treating everything but an explicit block as passed.
func TestRunNorthstarRiskGate_MapsEveryAlignmentResult(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		withVision  bool
		llm         northstar.LLMClient
		wantOutcome RiskGateOutcome
		wantConcern string
	}{
		{"no vision is skipped", false, alignmentLLM{"SCORE: 0.9\nRESULT: passed"}, RiskGateOutcomeSkipped, ""},
		{"no llm is inconclusive", true, nil, RiskGateOutcomeInconclusive, ""},
		{"malformed answer is inconclusive", true, alignmentLLM{"I think it's fine"}, RiskGateOutcomeInconclusive, ""},
		{"bare passed claim is inconclusive", true, alignmentLLM{"RESULT: passed"}, RiskGateOutcomeInconclusive, ""},
		{"blocked blocks", true, alignmentLLM{"SCORE: 0.1\nRESULT: blocked"}, RiskGateOutcomeBlocked, ""},
		{"failed passes with a requires-changes concern", true, alignmentLLM{"SCORE: 0.4\nRESULT: failed"}, RiskGateOutcomePassed, riskConcernRequiresChanges},
		{"passed passes", true, alignmentLLM{"SCORE: 0.9\nRESULT: passed"}, RiskGateOutcomePassed, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := &Orchestrator{
				campaign:          riskContractCampaign("/campaign_ns_gate", "internal/widgets/thing.go"),
				northstarObserver: northstarGateObserver(t, tc.withVision, tc.llm),
			}
			res, concerns := o.runNorthstarRiskGate(context.Background())
			if res.Outcome != tc.wantOutcome {
				t.Fatalf("outcome = %s, want %s (reason %q)", res.Outcome, tc.wantOutcome, res.Reason)
			}
			if tc.wantConcern == "" && len(concerns) != 0 {
				t.Fatalf("unexpected concerns %+v", concerns)
			}
			if tc.wantConcern != "" && (len(concerns) != 1 || concerns[0].severity != tc.wantConcern) {
				t.Fatalf("concerns = %+v, want one %s", concerns, tc.wantConcern)
			}
		})
	}
}

// An unassessed northstar gate is a hard stop on a protected surface and a
// soft, visible warning elsewhere -- never a silent pass.
func TestRiskClassification_InconclusiveNorthstar_HardOnProtectedSoftElsewhere(t *testing.T) {
	t.Parallel()
	kernel := newRiskContractKernel(t)
	for _, tc := range []struct {
		name      string
		id        string
		protected []string
		wantHard  bool
		reason    string
	}{
		{"protected", "/campaign_ns_unassessed_protected", []string{"internal/core"}, true, "/protected_surface_unassessed"},
		{"ordinary", "/campaign_ns_unassessed_ordinary", nil, false, "/inconclusive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			orch := riskContractOrchestrator(t, kernel, riskContractCampaign(tc.id, "internal/core/kernel.go"))
			findings, decided := orch.classifyRiskGateResults(riskContractFacts{
				campaignID:     tc.id,
				protectedRoots: tc.protected,
				gateOutcomes:   []RiskGateResult{{Name: RiskGateNorthstar, Enabled: true, Outcome: RiskGateOutcomeInconclusive, Reason: "not assessed"}},
				decision:       &CampaignRiskDecision{Score: 80, Threshold: 70, Gated: true},
			})
			if !decided {
				t.Fatal("kernel classification unavailable")
			}
			hard, soft := hardRiskFindings(findings), softRiskFindings(findings)
			if tc.wantHard {
				if len(hard) != 1 || hard[0].Reason != tc.reason || hard[0].Detail != "not assessed" {
					t.Fatalf("want one hard %s finding, got %s", tc.reason, describeFindings(findings))
				}
				return
			}
			if len(hard) != 0 || len(soft) != 1 || soft[0].Reason != tc.reason {
				t.Fatalf("want one soft %s finding, got %s", tc.reason, describeFindings(findings))
			}
		})
	}
}
