package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/cmd/nerd/ui"
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/observation"
	"codenerd/internal/perception"
	"codenerd/internal/session"
	"codenerd/internal/tactile"
	"codenerd/internal/types"
)

// campaignControlExecutor lets the test hold each campaign turn at the
// execution boundary so the slash command can be exercised while Run is live.
type campaignControlExecutor struct {
	calls    atomic.Int32
	started  chan int
	releases []chan struct{}
}

func (e *campaignControlExecutor) ExecuteObserved(ctx context.Context, _ session.TaskRequest) (observation.Return, error) {
	call := int(e.calls.Add(1))
	e.started <- call
	if call > len(e.releases) {
		return observation.Return{}, fmt.Errorf("unexpected task execution %d", call)
	}
	select {
	case <-ctx.Done():
		return observation.Return{}, ctx.Err()
	case <-e.releases[call-1]:
		return observation.Return{Output: fmt.Sprintf("completed task %d", call), Outcome: "/done"}, nil
	}
}

func (e *campaignControlExecutor) ExecuteObservedWithContext(ctx context.Context, req session.TaskRequest, _ *types.SessionContext, _ types.SpawnPriority) (observation.Return, error) {
	return e.ExecuteObserved(ctx, req)
}

func (e *campaignControlExecutor) Execute(ctx context.Context, req session.TaskRequest) (string, error) {
	ret, err := e.ExecuteObserved(ctx, req)
	return ret.Output, err
}

func (e *campaignControlExecutor) ExecuteWithContext(ctx context.Context, req session.TaskRequest, _ *types.SessionContext, _ types.SpawnPriority) (string, error) {
	return e.Execute(ctx, req)
}

func (*campaignControlExecutor) ExecuteAsync(context.Context, session.TaskRequest) (string, error) {
	return "", errors.New("campaign control test executor is synchronous")
}

func (*campaignControlExecutor) GetResult(string) (string, bool, error) {
	return "", false, errors.New("campaign control test executor is synchronous")
}

func (*campaignControlExecutor) WaitForResult(context.Context, string) (string, error) {
	return "", errors.New("campaign control test executor is synchronous")
}

func TestCampaignSlashPauseResumeControlsLiveOrchestrator(t *testing.T) {
	workspace := t.TempDir()
	realKernel, err := core.NewRealKernelWithWorkspace(workspace)
	if err != nil {
		t.Fatalf("create real kernel: %v", err)
	}
	executor := &campaignControlExecutor{
		started:  make(chan int, 2),
		releases: []chan struct{}{make(chan struct{}), make(chan struct{})},
	}
	policy := config.DefaultCampaignConfig()
	policy.MaxParallelTasks = 1
	orch, err := campaign.NewOrchestrator(campaign.OrchestratorConfig{
		Workspace:         workspace,
		Kernel:            campaignControlKernel{Kernel: realKernel},
		LLMClient:         perception.NewGeminiClientWithConfig(perception.GeminiConfig{APIKey: "unused-test-key", Model: "gemini-test"}),
		PlanningLLMClient: perception.NewGeminiClientWithConfig(perception.GeminiConfig{APIKey: "unused-test-key", Model: "gemini-test"}),
		TaskExecutor:      executor,
		Executor:          tactile.NewDirectExecutor(),
		VirtualStore:      &core.VirtualStore{},
		Campaign:          policy,
	})
	if err != nil {
		t.Fatalf("create orchestrator: %v", err)
	}
	t.Cleanup(func() { _ = orch.Close() })

	c := &campaign.Campaign{
		ID: "/campaign_chat_control", Type: campaign.CampaignTypeCustom,
		Title: "chat control", Goal: "prove pause and resume control the live scheduler",
		Status: campaign.StatusActive, TotalPhases: 1, TotalTasks: 2,
		Phases: []campaign.Phase{{
			ID: "/phase_chat_control", CampaignID: "/campaign_chat_control",
			Name: "controlled work", Order: 0, Status: campaign.PhasePending,
			Objectives: []campaign.PhaseObjective{{Type: campaign.ObjectiveResearch, Description: "complete the two turns", VerificationMethod: campaign.VerifyNone}},
			Tasks: []campaign.Task{
				{ID: "/task_chat_control_1", PhaseID: "/phase_chat_control", Description: "first controlled task", Type: campaign.TaskTypeResearch, Status: campaign.TaskPending, Priority: campaign.PriorityNormal, Order: 0},
				{ID: "/task_chat_control_2", PhaseID: "/phase_chat_control", Description: "second controlled task", Type: campaign.TaskTypeResearch, Status: campaign.TaskPending, Priority: campaign.PriorityNormal, Order: 1},
			},
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	runDone := make(chan error, 1)
	go func() { runDone <- orch.Run(ctx) }()

	if got := awaitTaskStart(t, executor.started); got != 1 {
		t.Fatalf("first task start = %d, want 1", got)
	}
	updated, _ := m.handleCommand("/campaign pause")
	m = updated.(Model)
	if got := orch.GetProgress().CampaignStatus; got != string(campaign.StatusPaused) {
		t.Fatalf("live orchestrator status after /campaign pause = %s, want %s", got, campaign.StatusPaused)
	}
	if got := m.campaignProgress.CampaignStatus; got != string(campaign.StatusPaused) {
		t.Fatalf("chat progress after /campaign pause = %s, want %s", got, campaign.StatusPaused)
	}
	if !strings.Contains(lastChatMessage(m), "Campaign paused.") {
		t.Fatalf("pause response did not confirm the live transition: %q", lastChatMessage(m))
	}

	close(executor.releases[0])
	select {
	case got := <-executor.started:
		t.Fatalf("task %d started while the live campaign was paused", got)
	case <-time.After(350 * time.Millisecond):
	}
	if got := orch.GetProgress().CampaignStatus; got != string(campaign.StatusPaused) {
		t.Fatalf("live orchestrator left paused state before resume: %s", got)
	}

	updated, _ = m.handleCommand("/campaign resume")
	m = updated.(Model)
	if got := orch.GetProgress().CampaignStatus; got != string(campaign.StatusActive) {
		t.Fatalf("live orchestrator status after /campaign resume = %s, want %s", got, campaign.StatusActive)
	}
	if got := m.campaignProgress.CampaignStatus; got != string(campaign.StatusActive) {
		t.Fatalf("chat progress after /campaign resume = %s, want %s", got, campaign.StatusActive)
	}
	if !strings.Contains(lastChatMessage(m), "Campaign resumed.") {
		t.Fatalf("resume response did not confirm the live transition: %q", lastChatMessage(m))
	}
	if got := awaitTaskStart(t, executor.started); got != 2 {
		t.Fatalf("second task start = %d, want 2 after resume", got)
	}
	close(executor.releases[1])

	select {
	case err := <-runDone:
		if err != nil {
			t.Fatalf("campaign run: %v", err)
		}
	case <-ctx.Done():
		t.Fatalf("campaign did not finish after resume: %v", ctx.Err())
	}
	if got := executor.calls.Load(); got != 2 {
		t.Fatalf("executor ran %d tasks, want exactly 2", got)
	}
}

func TestCampaignSlashPauseResumeDoNotClaimMissingTransitions(t *testing.T) {
	t.Run("no active campaign", func(t *testing.T) {
		m := NewTestModel()
		for _, command := range []string{"/campaign pause", "/campaign resume"} {
			updated, _ := m.handleCommand(command)
			m = updated.(Model)
			assertNoCampaignTransitionClaim(t, lastChatMessage(m))
		}
	})

	t.Run("active snapshot but missing orchestrator", func(t *testing.T) {
		m := NewTestModel()
		m.activeCampaign = &campaign.Campaign{ID: "/campaign_missing_orchestrator", Status: campaign.StatusPaused}
		for _, command := range []string{"/campaign pause", "/campaign resume"} {
			updated, _ := m.handleCommand(command)
			m = updated.(Model)
			assertNoCampaignTransitionClaim(t, lastChatMessage(m))
		}
	})

	t.Run("orchestrator without a loaded campaign", func(t *testing.T) {
		workspace := t.TempDir()
		kernel, err := core.NewRealKernelWithWorkspace(workspace)
		if err != nil {
			t.Fatalf("create real kernel: %v", err)
		}
		orch, err := campaign.NewOrchestrator(campaign.OrchestratorConfig{
			Workspace: workspace, Kernel: campaignControlKernel{Kernel: kernel},
			LLMClient:         perception.NewGeminiClientWithConfig(perception.GeminiConfig{APIKey: "unused-test-key", Model: "gemini-test"}),
			PlanningLLMClient: perception.NewGeminiClientWithConfig(perception.GeminiConfig{APIKey: "unused-test-key", Model: "gemini-test"}),
			TaskExecutor:      &campaignControlExecutor{started: make(chan int, 1), releases: []chan struct{}{make(chan struct{})}},
			Executor:          tactile.NewDirectExecutor(), VirtualStore: &core.VirtualStore{},
		})
		if err != nil {
			t.Fatalf("create orchestrator: %v", err)
		}
		t.Cleanup(func() { _ = orch.Close() })
		m := NewTestModel()
		m.workspace = workspace
		m.activeCampaign = &campaign.Campaign{ID: "/campaign_missing_state", Status: campaign.StatusPaused}
		m.campaignOrch = orch
		for _, command := range []string{"/campaign pause", "/campaign resume"} {
			updated, _ := m.handleCommand(command)
			m = updated.(Model)
			assertNoCampaignTransitionClaim(t, lastChatMessage(m))
		}
	})
}

// campaignControlKernel keeps the shipped Mangle program while making this
// fixture's optional intelligence wiring explicit and offline.
type campaignControlKernel struct{ core.Kernel }

func (k campaignControlKernel) Transaction() types.KernelTransaction {
	return k.Kernel.(types.KernelTransactor).Transaction()
}

func awaitTaskStart(t *testing.T, started <-chan int) int {
	t.Helper()
	select {
	case call := <-started:
		return call
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the orchestrator to start a task")
		return 0
	}
}

func lastChatMessage(m Model) string {
	if len(m.history) == 0 {
		return ""
	}
	return m.history[len(m.history)-1].Content
}

func assertNoCampaignTransitionClaim(t *testing.T, message string) {
	t.Helper()
	if message == "Campaign paused." || message == "Campaign resumed." || strings.HasPrefix(message, "Resuming campaign") {
		t.Fatalf("command claimed a transition without a matching live campaign: %q", message)
	}
}
