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
	table    table.Model

	// State
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

// Update handles messages.
func (m AutopoiesisPageModel) Update(msg tea.Msg) (AutopoiesisPageModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.activeTab = (m.activeTab + 1) % 2
			m.refreshTable()
			// TODO: Add keyboard navigation for tab switching (Left/Right arrows)
		}
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// refreshTable updates the table rows based on active tab
// TODO: Add sorting capabilities to the table columns.
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
// TODO: Replace magic number '60' with a defined breakpoint constant.
func (m *AutopoiesisPageModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.table.SetWidth(w - 4)
	m.table.SetHeight(h - 10)

	// Responsive column visibility
	cols := m.table.Columns()
	// Simple heuristic for small screens
	if w < 60 && len(cols) > 2 {
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
		// Restore full table via refresh if needed
		// For now we rely on next update cycle or explicit refresh but this handles the visual constraint
		m.refreshTable()
	}
}

// UpdateContent updates the data.
func (m *AutopoiesisPageModel) UpdateContent(patterns []*autopoiesis.DetectedPattern, learnings []*autopoiesis.ToolLearning) {
	m.patterns = patterns
	m.learnings = learnings
	m.refreshTable()
}
