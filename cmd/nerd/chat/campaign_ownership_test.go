package chat

import (
	"errors"
	"strings"
	"testing"

	"codenerd/cmd/nerd/ui"
	"codenerd/internal/campaign"
)

func installRun(m Model, orch *campaign.Orchestrator, title string) Model {
	m.campaignPage = ui.NewCampaignPageModel()
	m.activeCampaign = &campaign.Campaign{ID: "/" + title, Title: title, Status: campaign.StatusActive}
	m.campaignOrch = orch
	m.campaignProgressChan = make(chan campaign.Progress, 1)
	m.campaignEventChan = make(chan campaign.OrchestratorEvent, 1)
	m.showCampaignPanel = true
	return m
}

// A message from an earlier run never mutates the current run's slot:
// progress, failure and completion of run A arrive after run B is current
// and change nothing of B's. Before, every campaign message was unowned, and
// A's progress overwrote B's (audit campaign-ui-run-ownership).
func TestCampaignMessagesFromAnEarlierRunDoNotTouchTheCurrentOne(t *testing.T) {
	runA, runB := &campaign.Orchestrator{}, &campaign.Orchestrator{}
	m := installRun(NewTestModel(), runB, "B")
	m.isLoading = true
	bProgress := &campaign.Progress{CampaignID: "/B", CompletedTasks: 1, CampaignStatus: string(campaign.StatusActive)}
	m.campaignProgress = bProgress

	steps := []any{
		campaignProgressMsg{orch: runA, progress: &campaign.Progress{CampaignID: "/A", CompletedTasks: 7, CampaignStatus: string(campaign.StatusActive)}},
		campaignErrorMsg{orch: runA, err: errors.New("campaign failed")},
		campaignCompletedMsg{orch: runA, campaign: &campaign.Campaign{Title: "A"}},
		campaignEventMsg{orch: runA, event: campaign.OrchestratorEvent{Type: campaign.EventRiskGateBlocked, Message: "A blocked"}},
	}
	for _, step := range steps {
		updated, cmd := m.Update(step)
		m = updated.(Model)
		if cmd != nil {
			t.Errorf("%T from run A re-armed a listener on run B's slot", step)
		}
	}
	if m.campaignOrch != runB || m.activeCampaign == nil || m.activeCampaign.Title != "B" || m.campaignProgress != bProgress || !m.isLoading || !m.showCampaignPanel {
		t.Fatalf("run A's messages changed run B's slot: orch=%p campaign=%+v progress=%+v loading=%v", m.campaignOrch, m.activeCampaign, m.campaignProgress, m.isLoading)
	}

	// B's own progress still applies.
	updated, _ := m.Update(campaignProgressMsg{orch: runB, progress: &campaign.Progress{CampaignID: "/B", CompletedTasks: 2, CampaignStatus: string(campaign.StatusActive)}})
	if got := updated.(Model).campaignProgress.CompletedTasks; got != 2 {
		t.Fatalf("run B's own progress was not applied: %d", got)
	}
}

// One managed campaign: a second run accepted while one is live is not
// installed over it, and /campaign start refuses up front.
func TestASecondCampaignDoesNotReplaceALiveOne(t *testing.T) {
	runA := &campaign.Orchestrator{}
	m := installRun(NewTestModel(), runA, "A")
	updated, cmd := m.Update(campaignStartedMsg{campaign: &campaign.Campaign{Title: "B"}, orch: &campaign.Orchestrator{}})
	m = updated.(Model)
	if cmd != nil || m.campaignOrch != runA || m.activeCampaign.Title != "A" {
		t.Fatalf("run B replaced the live run A: orch=%p campaign=%+v", m.campaignOrch, m.activeCampaign)
	}
	if !strings.Contains(lastChatMessage(m), "Campaign Not Started") {
		t.Fatalf("the refusal is not said: %q", lastChatMessage(m))
	}
	updated, cmd = m.handleCommand("/campaign start another goal")
	m = updated.(Model)
	if cmd != nil || m.isLoading || !strings.Contains(lastChatMessage(m), "already running") {
		t.Fatalf("/campaign start did not refuse while A runs: loading=%v last=%q", m.isLoading, lastChatMessage(m))
	}
}
