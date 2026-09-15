package chat

import (
	"strings"
	"testing"

	"codenerd/internal/campaign"
)

// Risk-gate events are the campaign's only voice when preflight refuses to
// start: no progress will ever publish. These pin what the transcript shows.

func riskBlockedEvent() campaign.OrchestratorEvent {
	return campaign.OrchestratorEvent{
		Type:    campaign.EventRiskGateBlocked,
		Message: "risk gate blocked campaign start (/advisory): test reason",
		Data: &campaign.RiskGateEvaluation{
			BlockedBy:      campaign.RiskGateAdvisory,
			BlockReason:    "test reason",
			Decision:       &campaign.CampaignRiskDecision{Score: 90, Threshold: 70, TieBreak: "test-tiebreak"},
			ProtectedRoots: []string{"internal/core"},
		},
	}
}

func TestRenderRiskGateEvent_BlockedShowsFullReport(t *testing.T) {
	text, terminal := renderRiskGateEvent(riskBlockedEvent())
	if !terminal {
		t.Fatal("a hard block must be terminal: nothing further will arrive")
	}
	for _, want := range []string{
		"## Campaign Blocked by Risk Gate",
		"advisory",
		"test reason",
		"90",
		"internal/core",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("report is missing %q:\n%s", want, text)
		}
	}
}

func TestRenderRiskGateEvent_BlockedValueEvalAlsoRenders(t *testing.T) {
	event := riskBlockedEvent()
	eval := *(event.Data.(*campaign.RiskGateEvaluation))
	event.Data = eval
	text, terminal := renderRiskGateEvent(event)
	if !terminal || !strings.Contains(text, "test reason") {
		t.Fatalf("value-form eval must render too: terminal=%v text=%q", terminal, text)
	}
}

func TestRenderRiskGateEvent_BlockedWithoutEvalFallsBackToMessage(t *testing.T) {
	event := campaign.OrchestratorEvent{
		Type:    campaign.EventRiskGateBlocked,
		Message: "risk gate blocked campaign start (/edge): prework wanted",
	}
	text, terminal := renderRiskGateEvent(event)
	if !terminal {
		t.Fatal("a block with no eval payload is still terminal")
	}
	if !strings.Contains(text, "prework wanted") {
		t.Fatalf("must fall back to the event message: %q", text)
	}
}

func TestRenderRiskGateEvent_AdvisoryShowsMessageAndContinues(t *testing.T) {
	event := campaign.OrchestratorEvent{
		Type:    campaign.EventRiskGateAdvisory,
		Message: "Advisory: /edge prework wanted",
	}
	text, terminal := renderRiskGateEvent(event)
	if terminal {
		t.Fatal("an advisory must not stand the campaign down")
	}
	if !strings.Contains(text, "prework wanted") {
		t.Fatalf("advisory text lost: %q", text)
	}
}

func TestUpdate_CampaignRiskBlocked_RendersReportAndStandsDown(t *testing.T) {
	m := NewTestModel()
	m.isLoading = true
	m.activeCampaign = &campaign.Campaign{}
	m.campaignOrch = &campaign.Orchestrator{}
	m.campaignProgressChan = make(chan campaign.Progress, 1)
	m.campaignEventChan = make(chan campaign.OrchestratorEvent, 1)
	m.showCampaignPanel = true

	updated, cmd := m.Update(campaignEventMsg(riskBlockedEvent()))
	result, ok := updated.(Model)
	if !ok {
		t.Fatal("Update did not return a Model")
	}
	if len(result.history) == 0 {
		t.Fatal("expected the block report in history, got nothing")
	}
	last := result.history[len(result.history)-1]
	if last.Role != "assistant" || !strings.Contains(last.Content, "## Campaign Blocked by Risk Gate") {
		t.Fatalf("last message is not the block report: role=%q content=%q", last.Role, last.Content)
	}
	if !strings.Contains(last.Content, "test reason") {
		t.Errorf("report lost the block reason: %q", last.Content)
	}
	if result.isLoading {
		t.Error("isLoading must clear: the campaign already ended")
	}
	if result.activeCampaign != nil || result.campaignOrch != nil {
		t.Error("campaign handles must clear so the panel stops")
	}
	if result.campaignProgressChan != nil || result.campaignEventChan != nil {
		t.Error("channels must clear so the listeners are not re-armed")
	}
	if result.showCampaignPanel {
		t.Error("showCampaignPanel must clear")
	}
	if cmd != nil {
		t.Error("a terminal block must not re-arm the listener")
	}
}

func TestUpdate_CampaignRiskAdvisory_RendersAndKeepsListening(t *testing.T) {
	m := NewTestModel()
	m.isLoading = true
	m.activeCampaign = &campaign.Campaign{}
	m.campaignEventChan = make(chan campaign.OrchestratorEvent, 1)

	event := campaign.OrchestratorEvent{Type: campaign.EventRiskGateAdvisory, Message: "Advisory: /edge prework wanted"}
	updated, cmd := m.Update(campaignEventMsg(event))
	result, ok := updated.(Model)
	if !ok {
		t.Fatal("Update did not return a Model")
	}
	if len(result.history) == 0 || !strings.Contains(result.history[len(result.history)-1].Content, "prework wanted") {
		t.Fatalf("advisory missing from history: %+v", result.history)
	}
	if !result.isLoading {
		t.Error("an advisory must not stop a running campaign")
	}
	if cmd == nil {
		t.Error("the listener must re-arm after an advisory")
	}
}

func TestRenderRiskGateEvent_NonRiskEventStaysSilent(t *testing.T) {
	event := campaign.OrchestratorEvent{Type: campaign.EventCampaignCompleted, Message: "done"}
	if text, terminal := renderRiskGateEvent(event); text != "" || terminal {
		t.Fatalf("non-risk event must stay silent: terminal=%v text=%q", terminal, text)
	}
}
