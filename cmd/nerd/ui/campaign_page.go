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

// CampaignPageModel defines the state of the campaign dashboard.
type CampaignPageModel struct {
	width    int
	height   int
	viewport viewport.Model
	progress progress.Model

	// Data
	campaignData *campaign.Campaign
	progressData *campaign.Progress

	// Styles
	styles Styles
}

// NewCampaignPageModel creates a new campaign page.
func NewCampaignPageModel() CampaignPageModel {
	p := progress.New(progress.WithDefaultGradient())
	vp := viewport.New(80, 20) // Initialize with reasonable default size
	vp.SetContent("")
	return CampaignPageModel{
		viewport: vp,
		progress: p,
		styles:   DefaultStyles(),
		width:    80,
		height:   20,
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
func (m CampaignPageModel) View() string {
	if m.campaignData == nil {
		return m.styles.Content.Render("No active campaign. Use '/campaign start' to begin.")
	}
	return m.viewport.View()
}

// SetSize updates the size of the viewport.
// TODO: Improve view resizing logic to avoid fragile calculations (e.g., `w - 4`). Use layout constants or dynamic measurement.
func (m *CampaignPageModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.viewport.Width = w
	m.viewport.Height = h
	m.progress.Width = w - 4 // Padding
}

// UpdateContent updates the viewport content based on campaign data.
// TODO: IMPROVEMENT: Refactor to avoid rebuilding large strings; use `lipgloss.Join` or component composition for better performance and maintainability.
// TODO: IMPROVEMENT: Decouple rendering logic from UpdateContent (move to View or a helper).
func (m *CampaignPageModel) UpdateContent(prog *campaign.Progress, camp *campaign.Campaign) {
	m.campaignData = camp
	m.progressData = prog

	if camp == nil {
		m.viewport.SetContent("No active campaign.")
		return
	}

	// TODO: IMPROVEMENT: Replace `strings.Builder` with `lipgloss.Join` to compose vertical layouts more idiomatically.
	var sb strings.Builder

	// 1. Header & Status
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

	// 2. Global Progress
	if prog != nil {
		sb.WriteString(m.styles.Bold.Render("Overall Progress") + "\n")
		sb.WriteString(m.progress.ViewAs(prog.OverallProgress) + "\n\n")
	}

	// 3. Control Hints (The Direct Control Plane)
	// TODO: Use bubbles/help component for standardized keybinding display.
	hints := m.styles.Muted.Render("Controls: [Space] Pause/Resume  [r] Replan  [c] Checkpoint  [Esc] Back")
	sb.WriteString(hints + "\n\n")

	// 4. Metrics Grid
	metrics := fmt.Sprintf(
		"Context Budget: %.1f%%  |  Learnings: %d  |  Replans: %d",
		camp.ContextUtilization*100,
		len(camp.Learnings),
		camp.RevisionNumber,
	)
	sb.WriteString(m.styles.Info.Render(metrics) + "\n\n")

	// 5. Phases List
	// TODO: IMPROVEMENT: Virtualize the phases list if it grows too large.
	// TODO: IMPROVEMENT: Refactor Phases list to use bubbles/list for better interactivity and scrolling.
	// TODO: IMPROVEMENT: Use `bubbles/list` delegates to render tasks, allowing for better key navigation and selection.
	// TODO: IMPROVEMENT: Break down phase rendering into smaller helper functions.
	sb.WriteString(m.styles.Header.Render(" Phases ") + "\n")
	for _, p := range camp.Phases {
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

		// If active phase, show tasks
		if p.Status == campaign.PhaseInProgress {
			for _, t := range p.Tasks {
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

				// Show active tasks and failed tasks, generally hide pending/completed to save space unless focused
				// For now, listing all for visibility
				// TODO: Implement proper task filtering (show active/failed, toggle completed) to reduce visual clutter.
				taskLine := fmt.Sprintf("   %s %-60s [%s]", taskIcon, t.Description, t.Type)
				sb.WriteString(taskStyle.Render(taskLine) + "\n")
			}
			sb.WriteString("\n")
		}
	}

	m.viewport.SetContent(sb.String())
}
