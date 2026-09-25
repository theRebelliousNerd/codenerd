package chat

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"codenerd/internal/articulation"
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	"codenerd/internal/northstar"
	"codenerd/internal/world"

	tea "github.com/charmbracelet/bubbletea"
)

// Chat surface for /recurse: a self-improvement sweep over the subsystem DAG,
// wave after wave. Wave zero starts like any campaign (campaignStartedMsg);
// each following wave chains off the previous wave's completion message, so
// the TUI stays live, pauses work between waves, and no goroutine blocks for
// the whole run.

// recurseState lives on the Model while a sweep runs. The loop is the shared
// wave policy from the campaign package, so bounds and the stall fuse mean
// the same thing here as on the CLI.
type recurseState struct {
	loop           *campaign.RecurseLoop
	cfg            campaign.RecurseConfig
	promptProvider campaign.PromptProvider
	lastWave       *campaign.Campaign
	held           bool
}

// parseRecurseArgs parses `/recurse` / `/campaign recurse` arguments. Bare
// tokens name subsystems to focus; unknown flags fail closed. known is the set
// of node IDs in the workspace's derived DAG (recurseNodeIDs); a subsystem
// outside it is refused here rather than after the first wave is planned.
func parseRecurseArgs(args []string, known map[string]bool) (campaign.RecurseConfig, error) {
	cfg := campaign.RecurseConfig{MaxWaves: 1}
	for i := 0; i < len(args); i++ {
		a := strings.TrimSpace(args[i])
		if a == "" {
			continue
		}
		value := func() (string, error) {
			if v, ok := strings.CutPrefix(a, "--waves="); ok {
				return v, nil
			}
			if a == "--waves" && i+1 < len(args) {
				i++
				return strings.TrimSpace(args[i]), nil
			}
			return "", fmt.Errorf("invalid --waves")
		}
		switch {
		case a == "--waves" || strings.HasPrefix(a, "--waves="):
			v, err := value()
			if err != nil {
				return cfg, err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return cfg, fmt.Errorf("invalid --waves %q: %w", v, err)
			}
			cfg.MaxWaves = n
		case a == "--angles" || strings.HasPrefix(a, "--angles="):
			var v string
			if rest, ok := strings.CutPrefix(a, "--angles="); ok {
				v = rest
			} else if i+1 < len(args) {
				i++
				v = strings.TrimSpace(args[i])
			} else {
				return cfg, fmt.Errorf("invalid --angles")
			}
			for _, name := range strings.Split(v, ",") {
				angle, err := campaign.ParseRecurseAngle(name)
				if err != nil {
					return cfg, err
				}
				cfg.Angles = append(cfg.Angles, angle)
			}
		case a == "--subsystem" || strings.HasPrefix(a, "--subsystem="):
			var v string
			if rest, ok := strings.CutPrefix(a, "--subsystem="); ok {
				v = rest
			} else if i+1 < len(args) {
				i++
				v = strings.TrimSpace(args[i])
			} else {
				return cfg, fmt.Errorf("invalid --subsystem")
			}
			if !known[v] {
				return cfg, fmt.Errorf("unknown subsystem %q", v)
			}
			cfg.Subsystems = append(cfg.Subsystems, v)
		case a == "--stall-waves" || strings.HasPrefix(a, "--stall-waves="):
			var v string
			if rest, ok := strings.CutPrefix(a, "--stall-waves="); ok {
				v = rest
			} else if i+1 < len(args) {
				i++
				v = strings.TrimSpace(args[i])
			} else {
				return cfg, fmt.Errorf("invalid --stall-waves")
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return cfg, fmt.Errorf("invalid --stall-waves %q: %w", v, err)
			}
			cfg.StallWaveLimit = n
		case strings.HasPrefix(a, "--"):
			return cfg, fmt.Errorf("unknown flag %q (want --waves, --angles, --subsystem, --stall-waves)", a)
		default:
			if !known[a] {
				return cfg, fmt.Errorf("unknown subsystem %q", a)
			}
			cfg.Subsystems = append(cfg.Subsystems, a)
		}
	}
	return cfg.Normalize()
}

// recurseNodeIDs is the set of node IDs recurse would sweep in workspace: the
// workspace's own package directories plus the cross-cutting close.
func recurseNodeIDs(workspace string) (map[string]bool, error) {
	nodes, err := campaign.DeriveWorkspaceDAG(context.Background(), workspace)
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		known[n.ID] = true
	}
	return known, nil
}

func (m Model) startRecurseCampaign(args []string) tea.Cmd {
	return func() tea.Msg {
		if m.kernel == nil {
			return campaignErrorMsg{err: fmt.Errorf("system not ready: kernel not initialized")}
		}
		if m.client == nil {
			return campaignErrorMsg{err: fmt.Errorf("system not ready: LLM client not initialized")}
		}
		if m.shardMgr == nil {
			return campaignErrorMsg{err: fmt.Errorf("system not ready: shard manager not initialized")}
		}
		if m.taskExecutor == nil {
			return campaignErrorMsg{err: fmt.Errorf("system not ready: task executor not initialized")}
		}

		known, err := recurseNodeIDs(m.workspace)
		if err != nil {
			return campaignErrorMsg{err: err}
		}
		cfg, err := parseRecurseArgs(args, known)
		if err != nil {
			return campaignErrorMsg{err: err}
		}
		yolo := m.Config.YoloMode()
		loop := &campaign.RecurseLoop{
			MaxWaves:       cfg.MaxWaves,
			AllowUnbounded: yolo,
			StallWaveLimit: cfg.StallWaveLimit,
		}
		if err := loop.Validate(); err != nil {
			return campaignErrorMsg{err: err}
		}

		var promptProvider campaign.PromptProvider
		if m.jitCompiler != nil {
			if pa, err := articulation.NewPromptAssemblerWithJIT(m.kernel, m.jitCompiler); err == nil {
				jitCfg := config.DefaultJITConfig()
				if m.Config != nil {
					jitCfg = m.Config.GetEffectiveJITConfig()
				}
				pa.SetJITBudgets(jitCfg.TokenBudget, jitCfg.ReservedTokens, jitCfg.SemanticTopK, jitCfg.ReservedTokensFallbackRatio)
				pa.EnableJIT(jitCfg.Enabled)
				promptProvider = &campaignJITProvider{assembler: pa}
			}
		}

		camp, err := campaign.NewRecurseCampaign(m.workspace, cfg)
		if err != nil {
			return campaignErrorMsg{err: fmt.Errorf("failed to plan recurse wave 0: %w", err)}
		}

		progressChan := make(chan campaign.Progress, 100)
		eventChan := make(chan campaign.OrchestratorEvent, 200)
		orch, err := m.buildRecurseOrchestrator(camp, progressChan, eventChan, promptProvider)
		if err != nil {
			return campaignErrorMsg{err: err}
		}

		m.ReportStatus("Recurse sweep started")
		return campaignStartedMsg{
			campaign:     camp,
			orch:         orch,
			progressChan: progressChan,
			eventChan:    eventChan,
			recurse:      &recurseState{loop: loop, cfg: cfg, promptProvider: promptProvider},
		}
	}
}

// buildRecurseOrchestrator constructs one wave's orchestrator. Every wave
// gets a fresh orchestrator over the run's shared channels; the wave
// campaign is set by the caller.
func (m Model) buildRecurseOrchestrator(camp *campaign.Campaign, progressChan chan campaign.Progress, eventChan chan campaign.OrchestratorEvent, promptProvider campaign.PromptProvider) (*campaign.Orchestrator, error) {
	consultationProvider := newCampaignConsultationProvider(m.consultationMgr)
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
	edgeCaseDetector := campaign.NewEdgeCaseDetector(m.kernel, m.scanner)
	var advisoryBoard *campaign.ShardAdvisoryBoard
	if consultationProvider != nil {
		advisoryBoard = campaign.NewShardAdvisoryBoard(consultationProvider)
	}

	orch, err := campaign.NewOrchestrator(campaign.OrchestratorConfig{
		Workspace:    m.workspace,
		Kernel:       m.kernel,
		LLMClient:    m.client,
		ShardManager: m.shardMgr,
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
		EdgeCaseDetector:     edgeCaseDetector,
		NorthstarObserver:    northstar.BuildCampaignObserver(m.workspace, m.client, m.kernel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize recurse orchestrator: %w", err)
	}
	if promptProvider != nil {
		orch.SetPromptProvider(promptProvider)
	}
	orch.SetSpecialistKnowledgeProvider(campaign.NewLocalSpecialistKnowledgeProvider(m.workspace))
	if err := orch.SetCampaign(camp); err != nil {
		return nil, fmt.Errorf("failed to set recurse campaign: %w", err)
	}
	return orch, nil
}

// maybeChainRecurseWave runs the wave boundary: record the finished wave,
// judge the loop, and either finish the sweep with a summary or start the
// next wave. ok=false means no sweep is running and the caller should take
// the normal completion path.
func (m Model) maybeChainRecurseWave(finished *campaign.Campaign) (Model, tea.Cmd, bool) {
	if m.recurse == nil {
		return m, nil, false
	}
	st := m.recurse
	decision, findings := st.loop.Observe(finished)
	st.lastWave = finished

	summary := fmt.Sprintf("Recurse wave %d finished: %d tasks completed, %d failed.",
		finished.RecurseWave, findings.Completed, findings.Failed)
	switch decision {
	case campaign.RecurseDoneBounds:
		m = m.pushAssistantMsg(summary + fmt.Sprintf(" Sweep complete after %d waves.", st.loop.WavesCompleted()))
		m.recurse = nil
		m = clearCampaignRun(m)
		return m, nil, true
	case campaign.RecurseDoneStalled:
		m = m.pushAssistantMsg(summary + " Stopped by the stall fuse: consecutive waves completed nothing — looping further would burn budget without progress.")
		m.recurse = nil
		m = clearCampaignRun(m)
		return m, nil, true
	}

	if st.held {
		m = m.pushAssistantMsg(summary + " Sweep held (paused between waves). `/campaign resume` continues.")
		m = clearCampaignRun(m)
		return m, nil, true
	}

	m, cmd := m.startNextRecurseWave(st, finished, summary)
	return m, cmd, true
}

// startNextRecurseWave plans and launches the wave after finished. The loop
// was already observed by the caller (boundary or resume-from-held).
func (m Model) startNextRecurseWave(st *recurseState, finished *campaign.Campaign, summary string) (Model, tea.Cmd) {
	next, err := campaign.PlanNextWave(m.workspace, st.cfg, finished)
	if err != nil {
		m = m.pushAssistantMsg(summary + fmt.Sprintf(" Could not plan the next wave: %v. Sweep stopped.", err))
		m.recurse = nil
		m = clearCampaignRun(m)
		return m, nil
	}
	// Resume-from-held arrives with cleared channels; recreate them so the new
	// wave has listeners to talk to.
	progressChan := m.campaignProgressChan
	eventChan := m.campaignEventChan
	if progressChan == nil {
		progressChan = make(chan campaign.Progress, 100)
	}
	if eventChan == nil {
		eventChan = make(chan campaign.OrchestratorEvent, 200)
	}
	orch, err := m.buildRecurseOrchestrator(next, progressChan, eventChan, st.promptProvider)
	if err != nil {
		m = m.pushAssistantMsg(summary + fmt.Sprintf(" Could not start the next wave: %v. Sweep stopped.", err))
		m.recurse = nil
		m = clearCampaignRun(m)
		return m, nil
	}
	m.activeCampaign = next
	m.campaignOrch = orch
	m.campaignProgress = nil
	m.campaignProgressChan = progressChan
	m.campaignEventChan = eventChan
	m.showCampaignPanel = true
	m.isLoading = true
	m = m.pushAssistantMsg(fmt.Sprintf("%s Next: wave %d (%s).", summary, next.RecurseWave, next.Title))
	// The previous wave's event listener may still be in flight; a second
	// one only load-balances (each event is still handled exactly once),
	// while a missing one would swallow risk blocks. Err toward the duplicate.
	return m, tea.Batch(
		m.runCampaignOrchestrator(),
		m.listenCampaignProgress(),
		m.listenCampaignEvents(),
	)
}

// clearCampaignRun stands the campaign UI down the way completion does. The
// recurse state itself is the caller's to keep or clear.
func clearCampaignRun(m Model) Model {
	m.isLoading = false
	m.activeCampaign = nil
	m.campaignOrch = nil
	m.campaignProgress = nil
	m.campaignProgressChan = nil
	m.campaignEventChan = nil
	m.showCampaignPanel = false
	return m
}
