package ui

import (
	"codenerd/internal/autopoiesis"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type AutoTab int

const (
	TabPatterns AutoTab = iota
	TabLearnings
)

// AutopoiesisPageModel defines the state of the Autopoiesis Dashboard.
type AutopoiesisPageModel struct {
	width    int
	height   int
	viewport viewport.Model
	// TODO: Use bubbles/list instead of table for better list management if items grow.
	table    table.Model

	// State
	// TODO: IMPROVEMENT: Consider using a state machine for managing tab transitions and view states if complexity increases.
	activeTab AutoTab

	// Data
	patterns  []*autopoiesis.DetectedPattern
	learnings []*autopoiesis.ToolLearning

	// Styles
	styles Styles
}

// NewAutopoiesisPageModel creates a new dashboard.
func NewAutopoiesisPageModel() AutopoiesisPageModel {
	vp := viewport.New(0, 0)

	// Default table
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "Pattern", Width: 40},
			{Title: "Confidence", Width: 10},
			{Title: "Count", Width: 10},
		}),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	return AutopoiesisPageModel{
		viewport:  vp,
		table:     t,
		styles:    DefaultStyles(),
		activeTab: TabPatterns,
	}
}

// Init initializes the model.
func (m AutopoiesisPageModel) Init() tea.Cmd {
	return nil
}

// Update handles messages with comprehensive keyboard navigation.
func (m AutopoiesisPageModel) Update(msg tea.Msg) (AutopoiesisPageModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		// Tab switching (Tab key and Left/Right arrows)
		case "tab", "right":
			m.activeTab = (m.activeTab + 1) % 2
			m.refreshTable()
			return m, nil
		case "shift+tab", "left":
			if m.activeTab == 0 {
				m.activeTab = 1
			} else {
				m.activeTab = 0
			}
			m.refreshTable()
			return m, nil

		// Table navigation (Up/Down arrows handled by table itself)
		case "up", "k":
			// Handled by table.Update below
		case "down", "j":
			// Handled by table.Update below
		case "pgup":
			// Page up in table
		case "pgdown":
			// Page down in table
		case "home":
			// Go to first row
		case "end":
			// Go to last row
		}
	}

	// Let table handle its own navigation events
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// refreshTable updates the table rows based on active tab
// TODO: IMPROVEMENT: Add sorting capabilities to the table columns.
// TODO: IMPROVEMENT: Optimize refreshTable for large datasets (consider virtualization or diffing).
func (m *AutopoiesisPageModel) refreshTable() {
	var rows []table.Row
	var cols []table.Column

	if m.activeTab == TabPatterns {
		cols = []table.Column{
			{Title: "Category", Width: 20},
			{Title: "Pattern Rule", Width: 50},
			{Title: "Confidence", Width: 10},
		}
		for _, p := range m.patterns {
			rows = append(rows, table.Row{
				string(p.IssueType),
				p.PatternID,
				fmt.Sprintf("%.2f", p.Confidence),
			})
		}
	} else {
		cols = []table.Column{
			{Title: "Tool", Width: 30},
			{Title: "Uses", Width: 10},
			{Title: "Success", Width: 10},
			{Title: "Fail", Width: 10},
			{Title: "Rate", Width: 10},
		}
		for _, l := range m.learnings {
			successCount := int(float64(l.TotalExecutions) * l.SuccessRate)
			failureCount := l.TotalExecutions - successCount

			rows = append(rows, table.Row{
				l.ToolName,
				fmt.Sprintf("%d", l.TotalExecutions),
				fmt.Sprintf("%d", successCount),
				fmt.Sprintf("%d", failureCount),
				fmt.Sprintf("%.1f%%", l.SuccessRate*100),
			})
		}
	}

	m.table.SetColumns(cols)
	m.table.SetRows(rows)
}

// View renders the page.
// TODO: IMPROVEMENT: Refactor the tab system to use a dedicated component or better state management for scalability.
// TODO: IMPROVEMENT: Add a help/legend component to explain the table columns and status icons.
func (m AutopoiesisPageModel) View() string {
	var sb strings.Builder

	// Header / Tabs
	patStyle := m.styles.Muted
	learnStyle := m.styles.Muted

	if m.activeTab == TabPatterns {
		patStyle = m.styles.Info.Copy().Bold(true)
	} else {
		learnStyle = m.styles.Info.Copy().Bold(true)
	}

	tabs := lipgloss.JoinHorizontal(lipgloss.Top,
		patStyle.Render("[ Patterns ]"),
		"  ",
		learnStyle.Render("[ Tool Learnings ]"),
		"  ",
		m.styles.Muted.Render("(Press Tab to switch)"),
	)

	sb.WriteString(tabs + "\n\n")
	sb.WriteString(m.styles.Content.Render(m.table.View()))

	// Detail View (if selected)
	// TODO: Enhance detail view with full JSON structure and syntax highlighting.
	sb.WriteString("\n\n")
	if m.activeTab == TabPatterns && len(m.patterns) > 0 {
		sel := m.table.Cursor()
		if sel < len(m.patterns) {
			p := m.patterns[sel]
			if len(p.Examples) > 0 {
				sb.WriteString(m.styles.Bold.Render("Example Trace:") + "\n")
				sb.WriteString(p.Examples[0] + "\n")
			} else {
				sb.WriteString(m.styles.Muted.Render("No examples available.") + "\n")
			}
		}
	}

	return sb.String()
}

// SetSize updates the size.
// TODO: IMPROVEMENT: Replace magic number '60' with a defined breakpoint constant.
// TODO: IMPROVEMENT: Implement a generic responsive layout manager to handle visibility of components based on available width.
func (m *AutopoiesisPageModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.table.SetWidth(w - 4)
	m.table.SetHeight(h - 10)

	// Responsive column visibility
	cols := m.table.Columns()
	isCompact := len(cols) <= 2
	shouldBeCompact := w < 60

	if shouldBeCompact {
		// In compact mode, we always update columns because width is dynamic (w - X)
		// This handles both the switch to compact AND resizing within compact mode.
		if m.activeTab == TabPatterns {
			// Compact view for small terminals
			m.table.SetColumns([]table.Column{
				{Title: "Pattern Rule", Width: w - 10},
			})
		} else {
			// Compact view for tool learning
			m.table.SetColumns([]table.Column{
				{Title: "Tool", Width: w - 20},
				{Title: "Rate", Width: 10},
			})
		}
	} else {
		// In full mode, widths are fixed constants (defined in refreshTable).
		// We only need to refresh (rebuild table) if we are switching from compact to full.
		// If we are already in full mode, resizing doesn't change column widths, so we skip refresh.
		if isCompact {
			m.refreshTable()
		}
	}
}

// UpdateContent updates the data.
func (m *AutopoiesisPageModel) UpdateContent(patterns []*autopoiesis.DetectedPattern, learnings []*autopoiesis.ToolLearning) {
	m.patterns = patterns
	m.learnings = learnings
	m.refreshTable()
}
