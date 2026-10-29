// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains campaign orchestration UI and rendering.
package chat

import (
	"codenerd/internal/broker"
	"codenerd/internal/campaign"
	"codenerd/internal/logging"
	"codenerd/internal/types"
	"codenerd/internal/usage"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/core"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// =============================================================================
// CAMPAIGN ORCHESTRATION UI
// =============================================================================
// Functions for managing long-running campaigns: starting, pausing, resuming,
// and rendering progress panels.

func (m Model) startCampaign(goal string) tea.Cmd {
	return func() tea.Msg {
		// Guard: ensure required components are initialized
		if m.kernel == nil {
			return campaignErrorMsg{err: fmt.Errorf("system not ready: kernel not initialized")}
		}
		if m.client == nil {
			return campaignErrorMsg{err: fmt.Errorf("system not ready: LLM client not initialized")}
		}
		if m.shardMgr == nil {
			return campaignErrorMsg{err: fmt.Errorf("system not ready: shard manager not initialized")}
		}

		// The session's context: quitting cancels the campaign; no phase clock.
		ctx, cancel := m.sessionOperationContext()
		if m.usageTracker != nil {
			ctx = usage.NewContext(ctx, m.usageTracker)
		}
		defer cancel()

		m.ReportStatus("Analyzing goal and docs...")

		// The orchestrator plans the campaign too: decomposition and every
		// replan run on the planning client, tasks on the task executor.
		// Larger buffers prevent backpressure during long campaigns (100+
		// phases).
		progressChan := make(chan campaign.Progress, 100)
		eventChan := make(chan campaign.OrchestratorEvent, 200)
		orch, err := m.newCampaignOrchestrator(progressChan, eventChan, m.campaignPromptProvider())
		if err != nil {
			return campaignErrorMsg{err: err}
		}

		m.ReportStatus("Decomposing into phases...")
		result, err := orch.Decompose(ctx, campaign.DecomposeRequest{
			Goal:         goal,
			CampaignType: campaign.CampaignTypeCustom,
			SourcePaths:  []string{}, // Will scan workspace
		})
		if err != nil {
			_ = orch.Close()
			return campaignErrorMsg{err: fmt.Errorf("failed to create campaign plan: %w", err)}
		}
		if err := orch.SetCampaign(result.Campaign); err != nil {
			_ = orch.Close()
			return campaignErrorMsg{err: fmt.Errorf("failed to set campaign: %w", err)}
		}

		m.ReportStatus("Campaign started")
		// Return orchestrator reference and channels - execution will be started by Update handler
		return campaignStartedMsg{
			campaign:     result.Campaign,
			orch:         orch,
			progressChan: progressChan,
			eventChan:    eventChan,
		}
	}
}

// runCampaignOrchestrator starts the orchestrator in a background goroutine and
// returns a tea.Cmd that listens for real-time channel updates.
func (m Model) runCampaignOrchestrator() tea.Cmd {
	if m.campaignOrch == nil {
		return nil
	}

	orch := m.campaignOrch

	// Use shutdown context for proper lifecycle management
	var ctx context.Context
	var cancel context.CancelFunc
	if m.shutdownCtx != nil {
		ctx, cancel = context.WithCancel(m.shutdownCtx)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}

	// This is where a campaign actually runs, and until now it was the one
	// LLM-spending path with no usage tracker attached. Two other functions
	// attached one to a context they then discarded, which is why the gap was
	// invisible: the wiring looked present everywhere except where it counted.
	//
	// The broker purpose rides along so campaign spend lands in its own account
	// rather than in "unattributed", which is where it has been going.
	if m.usageTracker != nil {
		ctx = usage.NewContext(ctx, m.usageTracker)
	}
	ctx = broker.WithPurpose(ctx, broker.PurposeCampaign)

	// Start orchestrator execution in background
	if m.goroutineWg != nil {
		m.goroutineWg.Add(1)
	}
	go func() {
		if m.goroutineWg != nil {
			defer m.goroutineWg.Done()
		}
		defer cancel()
		// The orchestrator owns the northstar observer it was built with;
		// once Run returns nothing else drives it, and the chat only drops
		// its reference.
		defer func() { _ = orch.Close() }()
		if err := orch.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			// The event channel and campaign status are the user-facing report.
			// This log is the one that survives when the orchestrator fails
			// before it can publish either.
			logging.Kernel("[campaign] orchestrator run ended with error: %v", err)
		}
	}()

	// Return a command that listens on the progress channel (real-time, not polling)
	return m.listenCampaignProgress()
}

// listenCampaignProgress returns a tea.Cmd that waits for real-time progress updates via channel.
// This is more efficient than polling - we only wake up when there's actual progress.
func (m Model) listenCampaignProgress() tea.Cmd {
	if m.campaignProgressChan == nil {
		return nil
	}

	progressChan := m.campaignProgressChan

	return func() tea.Msg {
		// Block until we receive a progress update from the orchestrator
		progress, ok := <-progressChan
		if !ok {
			// Channel closed - campaign finished or was cancelled
			return campaignCompletedMsg(m.activeCampaign)
		}

		// Check completion states
		if progress.CampaignStatus == string(campaign.StatusCompleted) {
			return campaignCompletedMsg(m.activeCampaign)
		}
		if progress.CampaignStatus == string(campaign.StatusFailed) {
			return campaignErrorMsg{err: fmt.Errorf("campaign failed")}
		}

		return campaignProgressMsg(&progress)
	}
}

// listenCampaignEvents returns a tea.Cmd that waits for real-time events via channel.
// Events include task_started, task_completed, phase_completed, learning, etc.
func (m Model) listenCampaignEvents() tea.Cmd {
	if m.campaignEventChan == nil {
		return nil
	}

	eventChan := m.campaignEventChan

	return func() tea.Msg {
		// Block until we receive an event from the orchestrator
		event, ok := <-eventChan
		if !ok {
			// Channel closed - stop listening
			return nil
		}

		return campaignEventMsg(event)
	}
}

// renderRiskGateEvent formats risk-gate orchestrator events for the chat
// transcript. The text is empty for non-risk events, which stay silent. The
// second return is true for a hard block, after which no further progress or
// events will arrive, so the caller stands the campaign UI down instead of
// re-arming the listener.
func renderRiskGateEvent(event campaign.OrchestratorEvent) (string, bool) {
	switch event.Type {
	case campaign.EventRiskGateBlocked:
		if eval := riskEvaluationFromEvent(event.Data); eval != nil {
			return "## Campaign Blocked by Risk Gate\n\n```\n" + campaign.FormatRiskEvaluation(eval) + "```", true
		}
		if msg := strings.TrimSpace(event.Message); msg != "" {
			return "## Campaign Blocked by Risk Gate\n\n" + msg, true
		}
		return "## Campaign Blocked by Risk Gate", true
	case campaign.EventRiskGateAdvisory:
		if msg := strings.TrimSpace(event.Message); msg != "" {
			return msg, false
		}
		return "", false
	default:
		return "", false
	}
}

// riskEvaluationFromEvent unwraps the evaluation the orchestrator attaches to
// a blocked event. Anything else (nil, a foreign payload) yields nil and the
// caller falls back to the event message.
func riskEvaluationFromEvent(data any) *campaign.RiskGateEvaluation {
	switch d := data.(type) {
	case *campaign.RiskGateEvaluation:
		return d
	case campaign.RiskGateEvaluation:
		return &d
	default:
		return nil
	}
}

// runLaunchCampaign runs clarifier then auto-starts a campaign using the goal plus clarifier answers (if provided).
func (m *Model) runLaunchCampaign(goal string) tea.Cmd {
	return func() tea.Msg {
		finalGoal := goal
		clarifier := strings.TrimSpace(m.launchClarifyAnswers)
		if clarifier != "" {
			finalGoal = fmt.Sprintf("%s\n\nClarifier responses:\n%s", goal, clarifier)
		}
		// Persist intent capture
		m.captureCampaignIntent(finalGoal, clarifier)
		// Reset clarifier state
		m.launchClarifyPending = false
		m.launchClarifyGoal = ""
		m.launchClarifyAnswers = ""

		return m.startCampaign(finalGoal)()
	}
}

// captureCampaignIntent stores clarifier answers into kernel facts for downstream logic.
func (m *Model) captureCampaignIntent(goal, clarifierAnswers string) {
	if m.kernel == nil {
		return
	}
	campaignID := fmt.Sprintf("campaign_%d", time.Now().UnixNano())
	if err := m.kernel.Assert(core.Fact{
		Predicate: "campaign_intent_capture",
		Args: []any{
			campaignID,
			goal,
			clarifierAnswers,
			types.Atom("hands_free"),
			"{}",
		},
	}); err != nil {
		logging.Kernel("[campaign] failed to assert campaign intent: %v", err)
	}
}

func (m Model) renderCampaignStarted(c *campaign.Campaign) string {
	var sb strings.Builder

	sb.WriteString("## Campaign Created\n\n")
	sb.WriteString(fmt.Sprintf("**Title**: %s\n", c.Title))
	sb.WriteString(fmt.Sprintf("**Type**: %s\n", c.Type))
	sb.WriteString(fmt.Sprintf("**Goal**: %s\n\n", c.Goal))

	sb.WriteString("### Execution Plan\n\n")
	sb.WriteString(fmt.Sprintf("**Phases**: %d\n", c.TotalPhases))
	sb.WriteString(fmt.Sprintf("**Tasks**: %d\n\n", c.TotalTasks))

	// Show phase overview
	sb.WriteString("| # | Phase | Tasks | Status |\n")
	sb.WriteString("|---|-------|-------|--------|\n")
	for i, phase := range c.Phases {
		sb.WriteString(fmt.Sprintf("| %d | %s | %d | %s |\n",
			i+1, phase.Name, len(phase.Tasks), phase.Status))
	}

	sb.WriteString("\n_Campaign execution started. Use `/campaign status` to monitor progress._\n")
	sb.WriteString("_Toggle campaign panel with **Ctrl+P**_\n")

	return sb.String()
}

// renderCampaignCompleted generates the display for a completed campaign
func (m Model) renderCampaignCompleted(c *campaign.Campaign) string {
	var sb strings.Builder

	sb.WriteString("## Campaign Completed!\n\n")
	sb.WriteString(fmt.Sprintf("**Title**: %s\n", c.Title))
	sb.WriteString(fmt.Sprintf("**Status**: %s\n\n", c.Status))

	// Summary
	sb.WriteString("### Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Phases Completed**: %d/%d\n", c.CompletedPhases, c.TotalPhases))
	sb.WriteString(fmt.Sprintf("- **Tasks Completed**: %d/%d\n", c.CompletedTasks, c.TotalTasks))
	sb.WriteString(fmt.Sprintf("- **Revisions**: %d\n", c.RevisionNumber))

	// Show artifacts created
	artifactCount := 0
	for _, phase := range c.Phases {
		for _, task := range phase.Tasks {
			artifactCount += len(task.Artifacts)
		}
	}
	if artifactCount > 0 {
		sb.WriteString(fmt.Sprintf("- **Artifacts Created**: %d\n", artifactCount))
	}

	sb.WriteString("\n### Goal Achieved\n\n")
	sb.WriteString(fmt.Sprintf("_%s_\n", c.Goal))

	return sb.String()
}

// renderCampaignStatus generates the current campaign status display
func (m Model) renderCampaignStatus() string {
	if m.activeCampaign == nil {
		return "No active campaign."
	}

	c := m.activeCampaign
	var sb strings.Builder

	sb.WriteString("## Campaign Status\n\n")
	sb.WriteString(fmt.Sprintf("**Title**: %s\n", c.Title))
	sb.WriteString(fmt.Sprintf("**Status**: %s\n", c.Status))
	sb.WriteString(fmt.Sprintf("**Progress**: %d/%d phases, %d/%d tasks\n\n",
		c.CompletedPhases, c.TotalPhases, c.CompletedTasks, c.TotalTasks))

	// Progress bar
	progress := 0.0
	if c.TotalTasks > 0 {
		progress = float64(c.CompletedTasks) / float64(c.TotalTasks)
	}
	progressBar := renderProgressBar(progress, 30)
	sb.WriteString(fmt.Sprintf("**Overall**: %s %.1f%%\n\n", progressBar, progress*100))

	// Phase details
	sb.WriteString("### Phases\n\n")
	sb.WriteString("| # | Phase | Tasks | Status |\n")
	sb.WriteString("|---|-------|-------|--------|\n")
	for i, phase := range c.Phases {
		completedInPhase := 0
		for _, task := range phase.Tasks {
			if task.Status == campaign.TaskCompleted {
				completedInPhase++
			}
		}
		statusIcon := getStatusIcon(string(phase.Status))
		sb.WriteString(fmt.Sprintf("| %d | %s | %d/%d | %s %s |\n",
			i+1, phase.Name, completedInPhase, len(phase.Tasks), statusIcon, phase.Status))
	}

	// Current task
	if m.campaignProgress != nil && m.campaignProgress.CurrentTask != "" {
		sb.WriteString(fmt.Sprintf("\n**Current Task**: %s\n", m.campaignProgress.CurrentTask))
	}

	// Errors if any
	if m.campaignProgress != nil && len(m.campaignProgress.Errors) > 0 {
		sb.WriteString("\n### Errors\n\n")
		for _, err := range m.campaignProgress.Errors {
			sb.WriteString(fmt.Sprintf("- %s\n", err))
		}
	}

	return sb.String()
}

// renderCampaignList shows all campaigns (active and stored)
func (m Model) renderCampaignList() string {
	var sb strings.Builder

	sb.WriteString("## Campaigns\n\n")

	// Active campaign
	if m.activeCampaign != nil {
		sb.WriteString("### Active Campaign\n\n")
		c := m.activeCampaign
		progress := 0.0
		if c.TotalTasks > 0 {
			progress = float64(c.CompletedTasks) / float64(c.TotalTasks) * 100
		}
		sb.WriteString(fmt.Sprintf("- **%s** (%s) - %.1f%% complete\n\n", c.Title, c.Status, progress))
	} else {
		sb.WriteString("_No active campaign._\n\n")
	}

	// Load stored campaigns from .nerd/campaigns/
	campaignsDir := m.workspace + "/.nerd/campaigns"
	if entries, err := os.ReadDir(campaignsDir); err == nil && len(entries) > 0 {
		sb.WriteString("### Stored Campaigns\n\n")
		sb.WriteString("| ID | Title | Status | Progress |\n")
		sb.WriteString("|----|-------|--------|----------|\n")

		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
				data, err := os.ReadFile(filepath.Join(campaignsDir, entry.Name()))
				if err != nil {
					continue
				}
				var c campaign.Campaign
				if err := json.Unmarshal(data, &c); err != nil {
					continue
				}
				progress := 0.0
				if c.TotalTasks > 0 {
					progress = float64(c.CompletedTasks) / float64(c.TotalTasks) * 100
				}
				sb.WriteString(fmt.Sprintf("| %s | %s | %s | %.0f%% |\n",
					c.ID[len(c.ID)-8:], c.Title, c.Status, progress))
			}
		}
	} else {
		sb.WriteString("_No stored campaigns._\n")
	}

	sb.WriteString("\n**Start a new campaign**: `/campaign start <goal>`\n")

	return sb.String()
}

// renderCampaignPanel generates the campaign progress panel for split-pane view
func (m Model) renderCampaignPanel() string {
	if m.activeCampaign == nil {
		return "No active campaign"
	}

	c := m.activeCampaign
	var sb strings.Builder

	// Header
	sb.WriteString("-- Campaign ---------------\n")
	sb.WriteString(fmt.Sprintf("| %s\n", types.Ellipsize(c.Title, 22)))
	sb.WriteString(fmt.Sprintf("| Status: %s\n", c.Status))

	// Progress
	progress := 0.0
	if c.TotalTasks > 0 {
		progress = float64(c.CompletedTasks) / float64(c.TotalTasks)
	}
	bar := renderProgressBar(progress, 20)
	sb.WriteString(fmt.Sprintf("| %s %.0f%%\n", bar, progress*100))
	sb.WriteString("|\n")

	// Phases
	sb.WriteString("| Phases:\n")
	for i, phase := range c.Phases {
		icon := getStatusIcon(string(phase.Status))
		sb.WriteString(fmt.Sprintf("| %s %d. %s\n", icon, i+1, types.Ellipsize(phase.Name, 18)))
	}

	// Current task
	if m.campaignProgress != nil && m.campaignProgress.CurrentTask != "" {
		sb.WriteString("|\n")
		sb.WriteString(fmt.Sprintf("| Task: %s\n", types.Ellipsize(m.campaignProgress.CurrentTask, 18)))
	}

	sb.WriteString("---------------------------\n")

	return sb.String()
}

// renderProgressBar creates a text-based progress bar
func renderProgressBar(progress float64, width int) string {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	filled := int(progress * float64(width))
	empty := width - filled

	barStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))    // Green
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Grey

	bar := barStyle.Render(strings.Repeat("#", filled)) + emptyStyle.Render(strings.Repeat(".", empty))
	return "[" + bar + "]"
}

// getStatusIcon returns an icon for campaign/phase/task status
func getStatusIcon(status string) string {
	switch status {
	case string(campaign.StatusActive), string(campaign.TaskInProgress):
		return "*"
	case string(campaign.StatusCompleted):
		return "+"
	case string(campaign.StatusPaused):
		return "="
	case string(campaign.StatusFailed):
		return "x"
	case string(campaign.TaskSkipped):
		return "-"
	case string(campaign.TaskBlocked):
		return "!"
	default:
		return "?"
	}
}
