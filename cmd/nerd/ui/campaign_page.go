package ui

import (
	"codenerd/internal/campaign"
	"fmt"
	"strings"

	"time"

	"github.com/charmbracelet/bubbles/key"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Virtualization constants for campaign phases
const (
	PhaseRowHeight    = 2  // Approximate lines per phase (name + status)
	TaskRowHeight     = 1  // Lines per task
	VirtualBufferSize = 5  // Extra phases to render above/below viewport
	MaxVisiblePhases  = 50 // Maximum phases to render at once for performance
)

// CampaignKeyMap defines the key bindings for the campaign dashboard.
type CampaignKeyMap struct {
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
}

// DefaultCampaignKeyMap returns the default key bindings.
func DefaultCampaignKeyMap() CampaignKeyMap {
	return CampaignKeyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("↓/j", "down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("pgdown", "page down"),
		),
	}
}

// CampaignPageModel defines the state of the campaign dashboard.
type CampaignPageModel struct {
	width    int
	height   int
	layout   LayoutConfig
	viewport viewport.Model
	progress progress.Model

	// Data
	campaignData *campaign.Campaign
	progressData *campaign.Progress

	// Virtualization state
	visibleStartIdx int // First visible phase index
	visibleEndIdx   int // Last visible phase index (exclusive)
	totalPhases     int // Total number of phases

	// View Mode
	viewMode int

	// Styles
	styles Styles

	// Performance
	renderCache *CachedRender

	// Navigation
	keys         CampaignKeyMap
	lastKeyPress time.Time
}

// NewCampaignPageModel creates a new campaign page.
func NewCampaignPageModel() CampaignPageModel {
	p := progress.New(progress.WithDefaultGradient())
	vp := viewport.New(80, 20) // Initialize with reasonable default size
	vp.SetContent("")
	return CampaignPageModel{
		viewport:    vp,
		progress:    p,
		styles:      DefaultStyles(),
		width:       80,
		height:      20,
		layout:      NewLayoutConfig(80, 20),
		keys:        DefaultCampaignKeyMap(),
		renderCache: NewCachedRender(nil), // Use default shared cache
	}
}

// Init initializes the model.
func (m CampaignPageModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m CampaignPageModel) Update(msg tea.Msg) (CampaignPageModel, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if time.Since(m.lastKeyPress) < 15*time.Millisecond {
			return m, nil
		}
		m.lastKeyPress = time.Now()

		switch {
		case key.Matches(msg, m.keys.Up):
			m.viewport.LineUp(1)
		case key.Matches(msg, m.keys.Down):
			m.viewport.LineDown(1)
		case key.Matches(msg, m.keys.PageUp):
			m.viewport.HalfViewUp()
		case key.Matches(msg, m.keys.PageDown):
			m.viewport.HalfViewDown()
		case msg.String() == "v":
			// Cycle detail/summary/dashboard view.
			m.viewMode = (m.viewMode + 1) % 3
			if m.renderCache != nil {
				m.renderCache.Invalidate()
			}
			// Trigger re-render with new mode
			m.UpdateContent(m.progressData, m.campaignData)
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the page.
// TODO: Add timeline view of campaign phases.
func (m CampaignPageModel) View() string {
	if m.campaignData == nil {
		var sb strings.Builder
		sb.WriteString(m.styles.Layout.Header.Render(" No Active Campaign ") + "\n\n")
		sb.WriteString(m.styles.Text.Body.Render("Campaigns allow you to orchestrate multi-phase goals and automated tasks.") + "\n\n")
		sb.WriteString(m.styles.Text.Bold.Render("Getting Started:") + "\n")
		sb.WriteString("  " + m.styles.Code.InlineCode.Render("/campaign start <goal>") + m.styles.Text.Muted.Render("   Begin a new multi-phase campaign") + "\n")
		sb.WriteString("  " + m.styles.Code.InlineCode.Render("/campaign assault <target>") + m.styles.Text.Muted.Render(" Run an adversarial soak/stress test") + "\n")
		sb.WriteString("  " + m.styles.Code.InlineCode.Render("/campaign resume") + m.styles.Text.Muted.Render("           Resume a paused campaign") + "\n\n")
		sb.WriteString(m.styles.Text.Bold.Render("Available Controls:") + "\n")
		sb.WriteString(m.styles.Text.Muted.Render("  [Space] Pause/Resume  [r] Replan  [c] Checkpoint  [Esc] Back"))
		return m.styles.Layout.Content.Render(sb.String())
	}
	return m.viewport.View()
}

// SetSize updates the size of the viewport.
func (m *CampaignPageModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.layout = NewLayoutConfig(w, h)
	m.viewport.Width = w
	m.viewport.Height = h
	m.progress.Width = ViewportWidth(w)
	// Invalidate cache on resize
	if m.renderCache != nil {
		m.renderCache.Invalidate()
	}
}

// UpdateContent updates the viewport content based on campaign data.
func (m *CampaignPageModel) UpdateContent(prog *campaign.Progress, camp *campaign.Campaign) {
	m.campaignData = camp
	m.progressData = prog

	if camp == nil {
		m.viewport.SetContent("No active campaign.")
		return
	}

	// Update virtualization state.
	m.totalPhases = len(camp.Phases)
	m.calculateVisibleRange()

	// Cache key components. Include scroll + visible range so we refresh on navigation.
	var overallProgress float64
	if prog != nil {
		overallProgress = prog.OverallProgress
	}

	cacheKey := []any{
		camp.RevisionNumber,
		camp.Status,
		overallProgress,
		m.totalPhases,
		len(camp.Learnings),
		m.visibleStartIdx,
		m.visibleEndIdx,
		m.viewport.YOffset,
		m.width,
		m.height,
		m.layout.IsCompact,
		m.viewMode,
	}

	render := func() string {
		var sb strings.Builder
		sb.WriteString(m.renderHeader(camp))

		if prog != nil {
			sb.WriteString(m.styles.Text.Bold.Render("Overall Progress") + "\n")
			sb.WriteString(m.progress.ViewAs(prog.OverallProgress) + "\n\n")
		}

		hints := m.styles.Text.Muted.Render("Controls: [Space] Pause/Resume  [r] Replan  [c] Checkpoint  [v] Cycle View  [Esc] Back")
		sb.WriteString(hints + "\n\n")

		sb.WriteString(m.renderMetrics(camp))
		if m.viewMode == 1 {
			sb.WriteString(m.renderSummary(camp, prog))
		} else if m.viewMode == 2 {
			sb.WriteString(m.renderDashboard(camp, prog))
		} else {
			sb.WriteString(m.renderVirtualizedPhases(camp))
		}
		return sb.String()
	}

	if m.renderCache != nil {
		m.viewport.SetContent(m.renderCache.Render(cacheKey, render))
		return
	}

	m.viewport.SetContent(render())
}

// calculateVisibleRange determines which phases should be rendered based on viewport
func (m *CampaignPageModel) calculateVisibleRange() {
	if m.totalPhases == 0 {
		m.visibleStartIdx = 0
		m.visibleEndIdx = 0
		return
	}

	// Calculate available height for phases (accounting for header, progress, etc.)
	availableHeight := max(
		// Reserve space for header, progress, metrics, controls
		m.height-12, 5)

	// Calculate how many phases can fit
	maxVisible := min(availableHeight/PhaseRowHeight, MaxVisiblePhases)

	// Start from viewport scroll position (approximate)
	scrollRatio := 0.0
	if m.viewport.TotalLineCount() > 0 {
		scrollRatio = float64(m.viewport.YOffset) / float64(m.viewport.TotalLineCount())
	}

	startIdx := int(scrollRatio * float64(m.totalPhases))
	startIdx -= VirtualBufferSize // Add buffer above
	if startIdx < 0 {
		startIdx = 0
	}

	endIdx := min(startIdx+maxVisible+(VirtualBufferSize*2), m.totalPhases)

	m.visibleStartIdx = startIdx
	m.visibleEndIdx = endIdx
}

// renderHeader renders the campaign header and status
func (m *CampaignPageModel) renderHeader(camp *campaign.Campaign) string {
	var sb strings.Builder

	statusColor := m.styles.Status.Info
	if camp.Status == campaign.StatusFailed {
		statusColor = m.styles.Status.Error
	} else if camp.Status == campaign.StatusCompleted {
		statusColor = m.styles.Status.Success
	} else if camp.Status == campaign.StatusPaused {
		statusColor = m.styles.Status.Warning
	}

	title := m.styles.Layout.Header.Render(fmt.Sprintf(" %s ", camp.Title))
	status := statusColor.Render(strings.ToUpper(string(camp.Status)))
	header := lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", status)
	sb.WriteString(header + "\n\n")

	return sb.String()
}

// renderMetrics renders the campaign metrics grid
func (m *CampaignPageModel) renderMetrics(camp *campaign.Campaign) string {
	var metrics string
	if m.layout.IsCompact {
		metrics = fmt.Sprintf(
			"Learnings: %d  |  Replans: %d",
			len(camp.Learnings),
			camp.RevisionNumber,
		)
	} else {
		metrics = fmt.Sprintf(
			"Context Budget: %.1f%%  |  Learnings: %d  |  Replans: %d",
			camp.ContextUtilization*100,
			len(camp.Learnings),
			camp.RevisionNumber,
		)
	}
	return m.styles.Status.Info.Render(metrics) + "\n\n"
}

// renderVirtualizedPhases renders only the visible phases for performance
func (m *CampaignPageModel) renderVirtualizedPhases(camp *campaign.Campaign) string {
	var sb strings.Builder

	sb.WriteString(m.styles.Layout.Header.Render(" Phases ") + "\n")

	// Show indicator if we're not at the start
	if m.visibleStartIdx > 0 {
		sb.WriteString(m.styles.Text.Muted.Render(fmt.Sprintf("  ... %d phases above ...\n", m.visibleStartIdx)))
	}

	// Render only visible phases
	for i := m.visibleStartIdx; i < m.visibleEndIdx && i < len(camp.Phases); i++ {
		sb.WriteString(m.renderPhase(&camp.Phases[i], i))
	}

	// Show indicator if there are more phases below
	if m.visibleEndIdx < m.totalPhases {
		remaining := m.totalPhases - m.visibleEndIdx
		sb.WriteString(m.styles.Text.Muted.Render(fmt.Sprintf("  ... %d phases below ...\n", remaining)))
	}

	// Show total count
	sb.WriteString(m.styles.Text.Muted.Render(fmt.Sprintf("\nTotal: %d phases", m.totalPhases)))

	return sb.String()
}

// renderPhase renders a single phase with its tasks
func (m *CampaignPageModel) renderPhase(p *campaign.Phase, index int) string {
	var sb strings.Builder

	icon := "○" // Pending
	style := m.styles.Text.Muted
	if p.Status == campaign.PhaseInProgress {
		icon = "▶"
		style = m.styles.Status.Info
	} else if p.Status == campaign.PhaseCompleted {
		icon = "✓"
		style = m.styles.Status.Success
	} else if p.Status == campaign.PhaseFailed {
		icon = "✗"
		style = m.styles.Status.Error
	}

	line := fmt.Sprintf(" %s %s", icon, p.Name)
	sb.WriteString(style.Render(line) + "\n")

	// If active phase, show tasks (with task count limit for very long task lists)
	if p.Status == campaign.PhaseInProgress {
		maxTasks := 20 // Limit tasks shown per phase
		for j := range p.Tasks {
			if j >= maxTasks {
				remaining := len(p.Tasks) - maxTasks
				sb.WriteString(m.styles.Text.Muted.Render(fmt.Sprintf("     ... %d more tasks ...\n", remaining)))
				break
			}
			sb.WriteString(m.renderTask(&p.Tasks[j]))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderTask renders a single task line
func (m *CampaignPageModel) renderTask(t *campaign.Task) string {
	taskIcon := "  •"
	taskStyle := m.styles.Text.Muted
	if t.Status == campaign.TaskInProgress {
		taskIcon = "  ➜"
		taskStyle = m.styles.Status.Info
	} else if t.Status == campaign.TaskCompleted {
		taskIcon = "  ✓"
		taskStyle = m.styles.Status.Success
	} else if t.Status == campaign.TaskFailed {
		taskIcon = "  ✗"
		taskStyle = m.styles.Status.Error
	}

	// Truncate long descriptions
	desc := t.Description

	var taskLine string
	if m.layout.IsCompact {
		if len(desc) > 35 {
			desc = desc[:32] + "..."
		}
		taskLine = fmt.Sprintf("   %s %-35s", taskIcon, desc)
	} else {
		if len(desc) > 55 {
			desc = desc[:52] + "..."
		}
		taskLine = fmt.Sprintf("   %s %-55s [%s]", taskIcon, desc, t.Type)
	}

	return taskStyle.Render(taskLine) + "\n"
}

// renderSummary renders a high-level overview of the campaign
func (m *CampaignPageModel) renderSummary(camp *campaign.Campaign, prog *campaign.Progress) string {
	var sb strings.Builder

	sb.WriteString(m.styles.Layout.Header.Render(" Campaign Summary ") + "\n\n")

	// Goal
	if camp.Goal != "" {
		sb.WriteString(m.styles.Text.Bold.Render("Goal:") + "\n")
		// Simple word wrap for goal
		words := strings.Fields(camp.Goal)
		lineLen := 0
		maxLen := m.layout.ContentWidth() - 4
		if maxLen < 20 {
			maxLen = 60
		}
		sb.WriteString("  ")
		for _, w := range words {
			if lineLen+len(w)+1 > maxLen {
				sb.WriteString("\n  ")
				lineLen = 0
			}
			sb.WriteString(w + " ")
			lineLen += len(w) + 1
		}
		sb.WriteString("\n\n")
	}

	// Active Phase Details
	if prog != nil && prog.CurrentPhase != "" {
		sb.WriteString(m.styles.Text.Bold.Render("Current Phase:") + "\n")
		sb.WriteString(fmt.Sprintf("  %s (%d/%d)\n", prog.CurrentPhase, prog.CompletedPhases, prog.TotalPhases))

		if prog.CurrentTask != "" {
			sb.WriteString(m.styles.Text.Bold.Render("Current Task:") + "\n")
			sb.WriteString(fmt.Sprintf("  %s\n", prog.CurrentTask))
		}
		sb.WriteString("\n")
	}

	// Stats Grid
	sb.WriteString(m.styles.Text.Bold.Render("Statistics:") + "\n")
	stats := []string{
		fmt.Sprintf("Phases:     %d/%d Completed", camp.CompletedPhases, camp.TotalPhases),
		fmt.Sprintf("Tasks:      %d/%d Completed", camp.CompletedTasks, camp.TotalTasks),
		fmt.Sprintf("Budget:     %.1f%% Utilized", camp.ContextUtilization*100),
		fmt.Sprintf("Confidence: %.1f%%", camp.Confidence*100),
	}
	for _, stat := range stats {
		sb.WriteString(fmt.Sprintf("  • %s\n", stat))
	}
	sb.WriteString("\n")

	// Knowledge/Learnings
	if len(camp.Learnings) > 0 {
		sb.WriteString(m.styles.Text.Bold.Render(fmt.Sprintf("Learnings (%d):", len(camp.Learnings))) + "\n")
		limit := 3
		if len(camp.Learnings) < limit {
			limit = len(camp.Learnings)
		}
		for i := 0; i < limit; i++ {
			sb.WriteString(fmt.Sprintf("  • %s\n", camp.Learnings[i].Type))
		}
		if len(camp.Learnings) > limit {
			sb.WriteString(fmt.Sprintf("  ... and %d more\n", len(camp.Learnings)-limit))
		}
	}

	return sb.String()
}

// renderDashboard renders a high-level metrics dashboard
func (m *CampaignPageModel) renderDashboard(camp *campaign.Campaign, prog *campaign.Progress) string {
	var sb strings.Builder

	sb.WriteString(m.styles.Header.Render(" Metrics Dashboard ") + "\n\n")

	// 1. Progress Metrics
	sb.WriteString(m.styles.Bold.Render("Progress & Execution:") + "\n")
	sb.WriteString(fmt.Sprintf("  • Phases Completed: %d / %d\n", camp.CompletedPhases, camp.TotalPhases))
	sb.WriteString(fmt.Sprintf("  • Tasks Completed:  %d / %d\n", camp.CompletedTasks, camp.TotalTasks))
	if prog != nil {
		sb.WriteString(fmt.Sprintf("  • Overall Progress: %.1f%%\n", prog.OverallProgress*100))
	}
	sb.WriteString("\n")

	// 2. Resource Metrics
	sb.WriteString(m.styles.Bold.Render("Resources & Context:") + "\n")
	sb.WriteString(fmt.Sprintf("  • Context Budget: %d tokens\n", camp.ContextBudget))
	sb.WriteString(fmt.Sprintf("  • Context Used:   %d tokens\n", camp.ContextUsed))
	sb.WriteString(fmt.Sprintf("  • Utilization:    %.1f%%\n", camp.ContextUtilization*100))
	sb.WriteString("\n")

	// 3. Quality Metrics
	sb.WriteString(m.styles.Bold.Render("Quality & Stability:") + "\n")
	sb.WriteString(fmt.Sprintf("  • Confidence:     %.1f%%\n", camp.Confidence*100))
	sb.WriteString(fmt.Sprintf("  • Replans:        %d\n", camp.RevisionNumber))
	if camp.LastRevision != "" {
		sb.WriteString(fmt.Sprintf("  • Last Replan:    %s\n", camp.LastRevision))
	}
	sb.WriteString(fmt.Sprintf("  • Learnings:      %d acquired\n", len(camp.Learnings)))
	sb.WriteString("\n")

	return sb.String()
}
