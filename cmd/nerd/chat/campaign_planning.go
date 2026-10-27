package chat

import (
	"fmt"

	"codenerd/internal/articulation"
	"codenerd/internal/broker"
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	"codenerd/internal/northstar"
	"codenerd/internal/perception"
	"codenerd/internal/usage"
	"codenerd/internal/world"

	tea "github.com/charmbracelet/bubbletea"
)

// campaignPlanningClient is the client chat campaigns plan on: the Cortex's
// planning tier (the planner slot config.json configures, else main), or the
// chat's own main client when no Cortex was booted.
func (m Model) campaignPlanningClient() perception.LLMClient {
	if planning := m.cortex.PlanningLLM(); planning != nil {
		return planning
	}
	return m.client
}

// campaignPromptProvider compiles the campaign roles' prompts from the JIT
// corpus with this session's JIT budgets. Nil without a compiler, which
// leaves each role on its static fallback.
func (m Model) campaignPromptProvider() campaign.PromptProvider {
	if m.jitCompiler == nil {
		return nil
	}
	pa, err := articulation.NewPromptAssemblerWithJIT(m.kernel, m.jitCompiler)
	if err != nil {
		return nil
	}
	jitCfg := config.DefaultJITConfig()
	if m.Config != nil {
		jitCfg = m.Config.GetEffectiveJITConfig()
	}
	pa.SetJITBudgets(jitCfg.TokenBudget, jitCfg.ReservedTokens, jitCfg.SemanticTopK, jitCfg.ReservedTokensFallbackRatio)
	pa.EnableJIT(jitCfg.Enabled)
	return campaign.NewJITPromptProvider(pa)
}

// campaignRequirementMsg reports a requirement the running campaign's plan
// took, or refused.
type campaignRequirementMsg struct {
	requirement string
	err         error
}

// addCampaignRequirement integrates requirement into the running campaign's
// plan off the UI goroutine: the replanner adds or revises tasks on the
// planning tier and the orchestrator persists the revision.
func (m Model) addCampaignRequirement(requirement string) tea.Cmd {
	orch := m.campaignOrch
	return func() tea.Msg {
		ctx, cancel := m.sessionOperationContext()
		defer cancel()
		if m.usageTracker != nil {
			ctx = usage.NewContext(ctx, m.usageTracker)
		}
		ctx = broker.WithPurpose(ctx, broker.PurposeCampaign)
		return campaignRequirementMsg{requirement: requirement, err: orch.HandleNewRequirement(ctx, requirement)}
	}
}

// handleCampaignRequirementMsg reports the outcome of /campaign add.
func (m Model) handleCampaignRequirementMsg(msg campaignRequirementMsg) Model {
	if msg.err != nil {
		return m.pushAssistantMsg(fmt.Sprintf("Requirement not added to the campaign plan: %v", msg.err))
	}
	return m.pushAssistantMsg(fmt.Sprintf("Requirement added to the campaign plan: %s", msg.requirement))
}

// newCampaignOrchestrator builds the orchestrator a chat campaign plans and
// runs on, over its own channels: planning on campaignPlanningClient, tasks on
// the session's task executor (the worker tier), the risk preflight's sensors
// and the user's campaign policy. Start, assault and recurse all build through
// here. The caller owns the orchestrator and closes it.
func (m Model) newCampaignOrchestrator(progressChan chan campaign.Progress, eventChan chan campaign.OrchestratorEvent, promptProvider campaign.PromptProvider) (*campaign.Orchestrator, error) {
	consultationProvider := newCampaignConsultationProvider(m.consultationMgr)
	// Holographic provider: rich codebase context for intelligence gathering.
	var holographic *world.HolographicProvider
	if m.kernel != nil {
		holographic = world.NewHolographicProvider(m.kernel, m.workspace)
	}
	intelligenceGatherer := campaign.NewIntelligenceGatherer(
		m.workspace,
		m.kernel,
		m.scanner,
		holographic,
		m.learningStore,
		m.localDB,
		nil,
		nil,
		consultationProvider,
	)
	var advisoryBoard *campaign.ShardAdvisoryBoard
	if consultationProvider != nil {
		advisoryBoard = campaign.NewShardAdvisoryBoard(consultationProvider)
	}

	orch, err := campaign.NewOrchestrator(campaign.OrchestratorConfig{
		Workspace:         m.workspace,
		Kernel:            m.kernel,
		LLMClient:         m.client,
		PlanningLLMClient: m.campaignPlanningClient(),
		ShardManager:      m.shardMgr,
		// Required: without it both verification checkpoints have nothing to
		// run on (internal/campaign/checkpoint.go), and an in-chat campaign
		// reported every phase verified while running no verification.
		TaskExecutor: m.taskExecutor,
		Executor:     m.executor,
		VirtualStore: m.virtualStore,
		ProgressChan: progressChan,
		EventChan:    eventChan,
		// The policy: the campaign section of the user's config, the same on
		// every door a campaign starts through.
		Campaign:             m.Config.GetCampaignConfig(),
		IntelligenceGatherer: intelligenceGatherer,
		AdvisoryBoard:        advisoryBoard,
		EdgeCaseDetector:     campaign.NewEdgeCaseDetector(m.kernel, m.scanner),
		NorthstarObserver:    northstar.BuildCampaignObserver(m.workspace, m.client, m.kernel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize campaign orchestrator: %w", err)
	}
	if promptProvider != nil {
		orch.SetPromptProvider(promptProvider)
	}
	return orch, nil
}
