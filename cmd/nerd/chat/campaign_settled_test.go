package chat

import (
	"strings"
	"testing"
	"time"

	"codenerd/cmd/nerd/ui"
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/tactile"

	tea "github.com/charmbracelet/bubbletea"
)

// settledRun builds a real orchestrator over a one-task campaign that
// succeeds, installed in a model the way campaignStartedMsg installs it.
func settledRun(t *testing.T) (Model, *campaign.Orchestrator) {
	t.Helper()
	workspace := t.TempDir()
	kernel, err := core.NewRealKernelWithWorkspace(workspace)
	if err != nil {
		t.Fatalf("create real kernel: %v", err)
	}
	release := make(chan struct{})
	close(release)
	executor := &campaignControlExecutor{started: make(chan int, 4), releases: []chan struct{}{release}}
	progress := make(chan campaign.Progress, 100)
	events := make(chan campaign.OrchestratorEvent, 200)
	orch, err := campaign.NewOrchestrator(campaign.OrchestratorConfig{
		Workspace:         workspace,
		Kernel:            campaignControlKernel{Kernel: kernel},
		LLMClient:         perception.NewGeminiClientWithConfig(perception.GeminiConfig{APIKey: "unused-test-key", Model: "gemini-test"}),
		PlanningLLMClient: perception.NewGeminiClientWithConfig(perception.GeminiConfig{APIKey: "unused-test-key", Model: "gemini-test"}),
		TaskExecutor:      executor,
		Executor:          tactile.NewDirectExecutor(),
		VirtualStore:      &core.VirtualStore{},
		Campaign:          config.DefaultCampaignConfig(),
		ProgressChan:      progress,
		EventChan:         events,
	})
	if err != nil {
		t.Fatalf("create orchestrator: %v", err)
	}
	c := &campaign.Campaign{
		ID: "/campaign_settled", Type: campaign.CampaignTypeCustom, Title: "settled receipt",
		Goal: "finish one task", Status: campaign.StatusActive, TotalPhases: 1, TotalTasks: 1,
		Phases: []campaign.Phase{{
			ID: "/phase_settled", CampaignID: "/campaign_settled", Name: "work", Status: campaign.PhasePending,
			Objectives: []campaign.PhaseObjective{{Type: campaign.ObjectiveResearch, Description: "finish", VerificationMethod: campaign.VerifyNone}},
			Tasks: []campaign.Task{{ID: "/task_settled", PhaseID: "/phase_settled", Description: "the one task",
				Type: campaign.TaskTypeResearch, Status: campaign.TaskPending, Priority: campaign.PriorityNormal}},
		}},
	}
	if err := orch.SetCampaign(c); err != nil {
		t.Fatalf("set campaign: %v", err)
	}
	m := NewTestModel()
	m.workspace = workspace
	m.campaignPage = ui.NewCampaignPageModel()
	m.activeCampaign = c
	m.campaignOrch = orch
	m.campaignProgressChan = progress
	m.campaignEventChan = events
	m.showCampaignPanel = true
	return m, orch
}

// awaitSettled runs the commands runCampaignOrchestrator returned and
// returns the run's terminal receipt. The progress listener's messages are
// discarded: the receipt must not depend on any of them.
func awaitSettled(t *testing.T, cmd tea.Cmd) campaignSettledMsg {
	t.Helper()
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatal("runCampaignOrchestrator did not return a batch of the progress listener and the receipt")
	}
	got := make(chan tea.Msg, len(batch))
	for _, c := range batch {
		go func() { got <- c() }()
	}
	deadline := time.After(20 * time.Second)
	for {
		select {
		case msg := <-got:
			if s, ok := msg.(campaignSettledMsg); ok {
				return s
			}
		case <-deadline:
			t.Fatal("no terminal receipt arrived after the run")
		}
	}
}

// A successful run is retired by its own terminal receipt, whatever the
// progress stream last said: the completion transcript appears and the pane
// and handles are released. A second receipt for the same run (a duplicate,
// or one arriving after a completed heartbeat already retired it) changes
// nothing. Before the receipt existed the bridge returned only the progress
// listener, so a run whose last snapshot was still active left the pane up
// for good (audit campaign-terminal-publication).
func TestCampaignRunIsRetiredByItsTerminalReceipt(t *testing.T) {
	m, orch := settledRun(t)
	receipt := awaitSettled(t, m.runCampaignOrchestrator())
	if receipt.orch != orch || receipt.status != campaign.StatusCompleted || receipt.err != nil {
		t.Fatalf("receipt = %+v, want this run, completed, no error", receipt)
	}
	updated, _ := m.Update(receipt)
	m = updated.(Model)
	if m.campaignOrch != nil || m.activeCampaign != nil || m.campaignProgressChan != nil || m.showCampaignPanel {
		t.Fatal("the receipt did not retire the campaign pane and handles")
	}
	if !strings.Contains(lastChatMessage(m), "Campaign Completed!") {
		t.Fatalf("no completion transcript: %q", lastChatMessage(m))
	}
	before := len(m.history)
	updated, _ = m.Update(receipt)
	if len(updated.(Model).history) != before {
		t.Fatal("a duplicate receipt rendered again")
	}
}

// Releasing a settled run keeps its outcome: Stop on a completed run (the
// chat quitting before it showed the completion) used to rewrite /completed
// to /paused.
func TestStopKeepsACompletedRunCompleted(t *testing.T) {
	m, orch := settledRun(t)
	receipt := awaitSettled(t, m.runCampaignOrchestrator())
	if receipt.status != campaign.StatusCompleted {
		t.Fatalf("run settled %s, want completed", receipt.status)
	}
	orch.Stop()
	if got := orch.GetProgress().CampaignStatus; got != string(campaign.StatusCompleted) {
		t.Fatalf("status after Stop = %s, want %s", got, campaign.StatusCompleted)
	}
}
