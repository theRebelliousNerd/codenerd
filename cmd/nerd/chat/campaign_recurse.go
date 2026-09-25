package chat

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"codenerd/internal/articulation"
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	"codenerd/internal/northstar"
	"codenerd/internal/world"

	tea "github.com/charmbracelet/bubbletea"
)

// Chat surface for /recurse: the improve-everything loop
// (Docs/journeys/10-forever-loop.md), the same campaign.RunRecurseCycles the
// CLI runs, in the background. Each attempt is a one-task campaign on its own
// orchestrator; the loop's progress lines stream into the conversation.
// `/recurse stop` or `/campaign pause` cancels it, reverting an attempt in
// flight; `/recurse` again resumes from .nerd/recurse/journal.jsonl.

// recurseState lives on the Model while the loop runs.
type recurseState struct {
	cancel context.CancelFunc
	lines  chan string
}

type (
	// recurseStartedMsg carries the running loop's state and the command
	// that runs it.
	recurseStartedMsg struct {
		state *recurseState
		run   tea.Cmd
	}
	// recurseLineMsg is one progress line from the loop.
	recurseLineMsg struct {
		line  string
		lines chan string
	}
	// recurseFinishedMsg ends the loop.
	recurseFinishedMsg struct {
		result *campaign.RecurseCycleResult
		err    error
	}
)

// parseRecurseArgs parses `/recurse` / `/campaign recurse` arguments. Bare
// tokens name subsystems to focus; unknown flags fail closed. known is the set
// of node IDs in the workspace's derived DAG (recurseNodeIDs); a subsystem
// outside it is refused here rather than after the loop starts.
func parseRecurseArgs(args []string, known map[string]bool) (campaign.RecurseConfig, error) {
	cfg := campaign.RecurseConfig{}
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
		case strings.HasPrefix(a, "--"):
			return cfg, fmt.Errorf("unknown flag %q (want --waves, --subsystem)", a)
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
		if m.recurse != nil {
			return campaignErrorMsg{err: fmt.Errorf("a recurse loop is already running; `/recurse stop` ends it")}
		}

		known, err := recurseNodeIDs(m.workspace)
		if err != nil {
			return campaignErrorMsg{err: err}
		}
		cfg, err := parseRecurseArgs(args, known)
		if err != nil {
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

		ctx, cancel := context.WithCancel(context.Background())
		st := &recurseState{cancel: cancel, lines: make(chan string, 64)}
		return recurseStartedMsg{state: st, run: m.runRecurseLoop(ctx, st, cfg, promptProvider)}
	}
}

// runRecurseLoop runs the loop to its end and reports it. Every attempt gets
// a fresh orchestrator over its own channels, drained here: the chat's
// campaign panel follows one campaign, and the loop is a series of them.
func (m Model) runRecurseLoop(ctx context.Context, st *recurseState, cfg campaign.RecurseConfig, promptProvider campaign.PromptProvider) tea.Cmd {
	return func() tea.Msg {
		defer st.cancel()
		out := &recurseLineWriter{lines: st.lines}
		res, err := campaign.RunRecurseCycles(ctx, campaign.RecurseCycleConfig{
			Workspace:  m.workspace,
			Kernel:     m.kernel,
			Passes:     cfg.MaxWaves,
			Subsystems: cfg.Subsystems,
			Progress:   out,
			Execute: func(ctx context.Context, a campaign.RecurseAttempt) error {
				camp := campaign.RecurseAttemptCampaign(m.workspace, a)
				camp.ContextBudget = cfg.ContextBudget
				defer func() {
					if err := campaign.ReleaseRecurseAttempt(m.workspace, m.kernel, camp); err != nil {
						fmt.Fprintf(out, "recurse: %v\n", err)
					}
				}()
				progress := make(chan campaign.Progress, 100)
				events := make(chan campaign.OrchestratorEvent, 200)
				done := make(chan struct{})
				drained := make(chan struct{})
				go func() {
					defer close(drained)
					for {
						select {
						case <-progress:
						case ev := <-events:
							if ev.Type == "task_completed" || ev.Type == "task_failed" {
								fmt.Fprintf(out, "  %s: %s\n", ev.Type, ev.Message)
							}
						case <-done:
							return
						}
					}
				}()
				defer func() { close(done); <-drained }()
				orch, err := m.buildRecurseOrchestrator(camp, progress, events, promptProvider)
				if err != nil {
					return err
				}
				defer func() { _ = orch.Close() }()
				return orch.Run(ctx)
			},
		})
		out.flush()
		close(st.lines)
		return recurseFinishedMsg{result: res, err: err}
	}
}

// listenRecurseLines delivers the loop's next progress line.
func listenRecurseLines(lines chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-lines
		if !ok {
			return nil
		}
		return recurseLineMsg{line: line, lines: lines}
	}
}

// recurseLineWriter turns the loop's progress output into lines on a channel.
// The loop and an attempt's event drain both write, so it locks.
type recurseLineWriter struct {
	mu      sync.Mutex
	lines   chan<- string
	partial string
}

func (w *recurseLineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	text := w.partial + string(p)
	parts := strings.Split(text, "\n")
	w.partial = parts[len(parts)-1]
	for _, line := range parts[:len(parts)-1] {
		if strings.TrimSpace(line) != "" {
			w.lines <- line
		}
	}
	return len(p), nil
}

func (w *recurseLineWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if strings.TrimSpace(w.partial) != "" {
		w.lines <- w.partial
	}
	w.partial = ""
}

// stopRecurse cancels a loop this session runs. The loop reverts an attempt
// in flight before it reports back.
func (m Model) stopRecurse() (Model, bool) {
	if m.recurse == nil {
		return m, false
	}
	m.recurse.cancel()
	m = m.pushAssistantMsg("Recurse stopping: an attempt in flight is reverted. `/recurse` resumes from the journal.")
	return m, true
}

// stopRecurseElsewhere asks a loop another process runs on this workspace
// (`nerd campaign recurse` in a terminal) to stop after its attempt in
// flight is judged.
func (m Model) stopRecurseElsewhere() Model {
	st, err := campaign.ReadRecurseStatus(m.workspace)
	if err != nil {
		return m.pushAssistantMsg(fmt.Sprintf("Could not read the recurse status: %v", err))
	}
	if !st.Running {
		return m.pushAssistantMsg("No recurse loop is running.")
	}
	if err := campaign.RequestRecurseStop(m.workspace); err != nil {
		return m.pushAssistantMsg(fmt.Sprintf("Could not request a stop: %v", err))
	}
	return m.pushAssistantMsg("Stop requested: the loop running on this workspace ends once its attempt in flight is judged.")
}

// recurseStatus shows the workspace's recurse ledger.
func (m Model) recurseStatus() Model {
	st, err := campaign.ReadRecurseStatus(m.workspace)
	if err != nil {
		return m.pushAssistantMsg(fmt.Sprintf("Could not read the recurse status: %v", err))
	}
	return m.pushAssistantMsg("```\n" + st.String() + "\n```")
}

// handleRecurseMsg applies the loop's messages to the model.
func (m Model) handleRecurseMsg(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case recurseStartedMsg:
		m.recurse = msg.state
		m.isLoading = false
		m = m.pushAssistantMsg("Recurse started: improving the workspace node by node. `/recurse stop` ends it.")
		return m, tea.Batch(msg.run, listenRecurseLines(msg.state.lines)), true
	case recurseLineMsg:
		m = m.pushAssistantMsg(msg.line)
		return m, listenRecurseLines(msg.lines), true
	case recurseFinishedMsg:
		m.recurse = nil
		summary := msg.result.Summary()
		if msg.err != nil && !errors.Is(msg.err, context.Canceled) && !errors.Is(msg.err, campaign.ErrRecurseStopped) {
			summary = strings.TrimSpace(summary + fmt.Sprintf("\nStopped by an error: %v", msg.err))
		}
		if summary == "" {
			summary = "Recurse ended."
		}
		m = m.pushAssistantMsg(summary)
		return m, nil, true
	}
	return m, nil, false
}

// buildRecurseOrchestrator constructs one attempt's orchestrator over the
// attempt's own channels, with the attempt's campaign set.
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
