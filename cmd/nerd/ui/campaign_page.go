package ui

import (
	"codenerd/internal/campaign"
	"fmt"
	"strings"

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

// CampaignPageModel defines the state of the campaign dashboard.
type CampaignPageModel struct {
	width    int
	height   int
	viewport viewport.Model
	progress progress.Model

	// Data
	campaignData *campaign.Campaign
	progressData *campaign.Progress

	// Virtualization state
	visibleStartIdx int // First visible phase index
	visibleEndIdx   int // Last visible phase index (exclusive)
	totalPhases     int // Total number of phases

	// Styles
	styles Styles

	// Performance
	renderCache *CachedRender
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
		renderCache: NewCachedRender(nil), // Use default shared cache
	}
}

// Init initializes the model.
func (m CampaignPageModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
// TODO: IMPROVEMENT: Implement customizable key bindings using `bubbles/key` instead of hardcoded strings.
// TODO: IMPROVEMENT: Add debounce logic for rapid key presses if performance becomes an issue.
func (m CampaignPageModel) Update(msg tea.Msg) (CampaignPageModel, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "k", "up":
			m.viewport.LineUp(1)
		case "j", "down":
			m.viewport.LineDown(1)
		case "pgup":
			m.viewport.HalfViewUp()
		case "pgdown":
			m.viewport.HalfViewDown()
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the page.
// TODO: Improve the empty state with more helpful instructions or a starting guide.
// TODO: Add accessibility considerations (ARIA roles/attributes) if supported by the target TUI environment.
// TODO: IMPROVEMENT: Add a summary/dashboard view mode for high-level metrics.
// TODO: Add timeline view of campaign phases.
func (m CampaignPageModel) View() string {
	if m.campaignData == nil {
		return m.styles.Content.Render("No active campaign. Use '/campaign start' to begin.")
	}
	return m.viewport.View()
}

// SetSize updates the size of the viewport.
// TODO: Improve view resizing logic to avoid fragile calculations (e.g., `w - 4`). Use layout constants or dynamic measurement.
// TODO: IMPROVEMENT: Add logic to hide less important columns/info on small screens.
func (m *CampaignPageModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.viewport.Width = w
	m.viewport.Height = h
	m.progress.Width = w - 4 // Padding
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

	cacheKey := []interface{}{
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
	}

	render := func() string {
		var sb strings.Builder
		sb.WriteString(m.renderHeader(camp))

		if prog != nil {
			sb.WriteString(m.styles.Bold.Render("Overall Progress") + "\n")
			sb.WriteString(m.progress.ViewAs(prog.OverallProgress) + "\n\n")
		}

		hints := m.styles.Muted.Render("Controls: [Space] Pause/Resume  [r] Replan  [c] Checkpoint  [Esc] Back")
		sb.WriteString(hints + "\n\n")

		sb.WriteString(m.renderMetrics(camp))
		sb.WriteString(m.renderVirtualizedPhases(camp))
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
	availableHeight := m.height - 12 // Reserve space for header, progress, metrics, controls
	if availableHeight < 5 {
		availableHeight = 5
	}

	// Calculate how many phases can fit
	maxVisible := availableHeight / PhaseRowHeight
	if maxVisible > MaxVisiblePhases {
		maxVisible = MaxVisiblePhases
	}

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

	endIdx := startIdx + maxVisible + (VirtualBufferSize * 2)
	if endIdx > m.totalPhases {
		endIdx = m.totalPhases
	}

	m.visibleStartIdx = startIdx
	m.visibleEndIdx = endIdx
}

// renderHeader renders the campaign header and status
func (m *CampaignPageModel) renderHeader(camp *campaign.Campaign) string {
	var sb strings.Builder

	statusColor := m.styles.Info
	if camp.Status == campaign.StatusFailed {
		statusColor = m.styles.Error
	} else if camp.Status == campaign.StatusCompleted {
		statusColor = m.styles.Success
	} else if camp.Status == campaign.StatusPaused {
		statusColor = m.styles.Warning
	}

	title := m.styles.Header.Render(fmt.Sprintf(" %s ", camp.Title))
	status := statusColor.Render(strings.ToUpper(string(camp.Status)))
	header := lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", status)
	sb.WriteString(header + "\n\n")

	return sb.String()
}

// renderMetrics renders the campaign metrics grid
func (m *CampaignPageModel) renderMetrics(camp *campaign.Campaign) string {
	metrics := fmt.Sprintf(
		"Context Budget: %.1f%%  |  Learnings: %d  |  Replans: %d",
		camp.ContextUtilization*100,
		len(camp.Learnings),
		camp.RevisionNumber,
	)
	return m.styles.Info.Render(metrics) + "\n\n"
}

// renderVirtualizedPhases renders only the visible phases for performance
func (m *CampaignPageModel) renderVirtualizedPhases(camp *campaign.Campaign) string {
	var sb strings.Builder

	sb.WriteString(m.styles.Header.Render(" Phases ") + "\n")

	// Show indicator if we're not at the start
	if m.visibleStartIdx > 0 {
		sb.WriteString(m.styles.Muted.Render(fmt.Sprintf("  ... %d phases above ...\n", m.visibleStartIdx)))
	}

	// Render only visible phases
	for i := m.visibleStartIdx; i < m.visibleEndIdx && i < len(camp.Phases); i++ {
		sb.WriteString(m.renderPhase(&camp.Phases[i], i))
	}

	// Show indicator if there are more phases below
	if m.visibleEndIdx < m.totalPhases {
		remaining := m.totalPhases - m.visibleEndIdx
		sb.WriteString(m.styles.Muted.Render(fmt.Sprintf("  ... %d phases below ...\n", remaining)))
	}

	// Show total count
	sb.WriteString(m.styles.Muted.Render(fmt.Sprintf("\nTotal: %d phases", m.totalPhases)))

	return sb.String()
}

// renderPhase renders a single phase with its tasks
func (m *CampaignPageModel) renderPhase(p *campaign.Phase, index int) string {
	var sb strings.Builder

	icon := "○" // Pending
	style := m.styles.Muted
	if p.Status == campaign.PhaseInProgress {
		icon = "▶"
		style = m.styles.Info
	} else if p.Status == campaign.PhaseCompleted {
		icon = "✓"
		style = m.styles.Success
	} else if p.Status == campaign.PhaseFailed {
		icon = "✗"
		style = m.styles.Error
	}

	line := fmt.Sprintf(" %s %s", icon, p.Name)
	sb.WriteString(style.Render(line) + "\n")

	// If active phase, show tasks (with task count limit for very long task lists)
	if p.Status == campaign.PhaseInProgress {
		maxTasks := 20 // Limit tasks shown per phase
		for j := range p.Tasks {
			if j >= maxTasks {
				remaining := len(p.Tasks) - maxTasks
				sb.WriteString(m.styles.Muted.Render(fmt.Sprintf("     ... %d more tasks ...\n", remaining)))
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
	taskStyle := m.styles.Muted
	if t.Status == campaign.TaskInProgress {
		taskIcon = "  ➜"
		taskStyle = m.styles.Info
	} else if t.Status == campaign.TaskCompleted {
		taskIcon = "  ✓"
		taskStyle = m.styles.Success
	} else if t.Status == campaign.TaskFailed {
		taskIcon = "  ✗"
		taskStyle = m.styles.Error
	}

	// Truncate long descriptions
	desc := t.Description
	if len(desc) > 55 {
		desc = desc[:52] + "..."
	}

	taskLine := fmt.Sprintf("   %s %-55s [%s]", taskIcon, desc, t.Type)
	return taskStyle.Render(taskLine) + "\n"
}
