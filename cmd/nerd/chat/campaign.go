// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains campaign orchestration UI and rendering.
package chat

import (
	"codenerd/internal/campaign"
	"codenerd/internal/usage"
	"context"
	"encoding/json"
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
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		if m.usageTracker != nil {
			ctx = usage.NewContext(ctx, m.usageTracker)
		}
		defer cancel()

		m.ReportStatus("Analyzing goal and docs...")

		// Create decomposer to break down the goal
		decomposer := campaign.NewDecomposer(m.kernel, m.client, m.workspace)

		// Build request
		req := campaign.DecomposeRequest{
			Goal:         goal,
			CampaignType: campaign.CampaignTypeCustom,
			SourcePaths:  []string{}, // Will scan workspace
		}

		// Decompose the goal into a campaign
		m.ReportStatus("Decomposing into phases...")
		result, err := decomposer.Decompose(ctx, req)
		if err != nil {
			return campaignErrorMsg{err: fmt.Errorf("failed to create campaign plan: %w", err)}
		}

		// Create orchestrator
		orch := campaign.NewOrchestrator(campaign.OrchestratorConfig{
			Workspace:    m.workspace,
			Kernel:       m.kernel,
			LLMClient:    m.client,
			ShardManager: m.shardMgr,
			Executor:     m.executor,
		})

		if err := orch.SetCampaign(result.Campaign); err != nil {
			return campaignErrorMsg{err: fmt.Errorf("failed to set campaign: %w", err)}
		}

		// Start execution in background (non-blocking)
		go func() {
			if err := orch.Run(ctx); err != nil {
				// Error will be captured by progress updates
			}
		}()

		m.ReportStatus("Campaign started")
		return campaignStartedMsg(result.Campaign)
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
	_ = m.kernel.Assert(core.Fact{
		Predicate: "campaign_intent_capture",
		Args: []interface{}{
			campaignID,
			goal,
			clarifierAnswers,
			"hands_free",
			"{}",
		},
	})
}

// resumeCampaign continues execution of a paused campaign
func (m Model) resumeCampaign() tea.Cmd {
	return func() tea.Msg {
		if m.activeCampaign == nil || m.campaignOrch == nil {
			return campaignErrorMsg{err: fmt.Errorf("no campaign to resume")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		if m.usageTracker != nil {
			ctx = usage.NewContext(ctx, m.usageTracker)
		}
		defer cancel()

		// Resume execution
		go func() {
			if err := m.campaignOrch.Run(ctx); err != nil {
				// Error captured by progress
			}
		}()

		return campaignProgressMsg(&campaign.Progress{
			CampaignID:      m.activeCampaign.ID,
			CampaignStatus:  string(campaign.StatusActive),
			CurrentPhase:    fmt.Sprintf("%d", m.activeCampaign.CompletedPhases),
			CompletedPhases: m.activeCampaign.CompletedPhases,
			TotalPhases:     m.activeCampaign.TotalPhases,
			CompletedTasks:  m.activeCampaign.CompletedTasks,
			TotalTasks:      m.activeCampaign.TotalTasks,
			CampaignTitle:   m.activeCampaign.Title,
			OverallProgress: float64(m.activeCampaign.CompletedTasks) / float64(m.activeCampaign.TotalTasks),
		})
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
	sb.WriteString(fmt.Sprintf("| %s\n", truncateString(c.Title, 22)))
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
		sb.WriteString(fmt.Sprintf("| %s %d. %s\n", icon, i+1, truncateString(phase.Name, 18)))
	}

	// Current task
	if m.campaignProgress != nil && m.campaignProgress.CurrentTask != "" {
		sb.WriteString("|\n")
		sb.WriteString(fmt.Sprintf("| Task: %s\n", truncateString(m.campaignProgress.CurrentTask, 18)))
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

// truncateString truncates a string to maxLen and adds ellipsis if needed
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
