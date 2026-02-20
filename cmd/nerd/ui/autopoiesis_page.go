package ui

import (
	"codenerd/internal/autopoiesis"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type AutoTab int

const (
	TabPatterns AutoTab = iota
	TabLearnings
)

// Wrapper for DetectedPattern to implement list.Item
type patternItem struct {
	*autopoiesis.DetectedPattern
}

func (i patternItem) Title() string {
	return fmt.Sprintf("[%s] %s", i.IssueType, i.PatternID)
}

func (i patternItem) Description() string {
	return fmt.Sprintf("Confidence: %.2f | Occurrences: %d", i.Confidence, i.Occurrences)
}

func (i patternItem) FilterValue() string {
	return i.PatternID + " " + string(i.IssueType)
}

// Wrapper for ToolLearning to implement list.Item
type learningItem struct {
	*autopoiesis.ToolLearning
}

func (i learningItem) Title() string {
	return i.ToolName
}

func (i learningItem) Description() string {
	successCount := int(float64(i.TotalExecutions) * i.SuccessRate)
	failCount := i.TotalExecutions - successCount
	return fmt.Sprintf("Rate: %.1f%% | Uses: %d | Success: %d | Fail: %d", i.SuccessRate*100, i.TotalExecutions, successCount, failCount)
}

func (i learningItem) FilterValue() string {
	return i.ToolName
}

// AutopoiesisPageModel defines the state of the Autopoiesis Dashboard.
type AutopoiesisPageModel struct {
	width    int
	height   int
	viewport viewport.Model
	// TODO: Use bubbles/list instead of table for better list management if items grow.
	list     list.Model

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

	// Default list
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Detected Patterns"
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	return AutopoiesisPageModel{
		viewport:  vp,
		list:      l,
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
		// Don't intercept keys if filtering is active
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch msg.String() {
		// Tab switching (Tab key)
		case "tab":
			m.activeTab = (m.activeTab + 1) % 2
			m.refreshList()
			return m, nil
		case "shift+tab":
			if m.activeTab == 0 {
				m.activeTab = 1
			} else {
				m.activeTab = 0
			}
			m.refreshList()
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
	}

	// Let list handle its own navigation events
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// refreshList updates the list items based on active tab
// TODO: IMPROVEMENT: Optimize refreshList for large datasets (consider virtualization or diffing).
func (m *AutopoiesisPageModel) refreshList() {
	var items []list.Item

	if m.activeTab == TabPatterns {
		m.list.Title = "Detected Patterns"
		for _, p := range m.patterns {
			items = append(items, patternItem{p})
		}
	} else {
		m.list.Title = "Tool Learnings"
		for _, l := range m.learnings {
			items = append(items, learningItem{l})
		}
	}

	m.list.SetItems(items)
}

// View renders the page.
// TODO: IMPROVEMENT: Refactor the tab system to use a dedicated component or better state management for scalability.
// TODO: IMPROVEMENT: Add a help/legend component to explain the list columns and status icons.
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
	sb.WriteString(m.styles.Content.Render(m.list.View()))

	// Detail View (if selected)
	// TODO: Enhance detail view with full JSON structure and syntax highlighting.
	sb.WriteString("\n\n")
	if m.activeTab == TabPatterns && len(m.patterns) > 0 {
		item := m.list.SelectedItem()
		if pItem, ok := item.(patternItem); ok {
			p := pItem.DetectedPattern
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
	m.list.SetWidth(w - 4)
	m.list.SetHeight(h - 10)
}

// UpdateContent updates the data.
func (m *AutopoiesisPageModel) UpdateContent(patterns []*autopoiesis.DetectedPattern, learnings []*autopoiesis.ToolLearning) {
	m.patterns = patterns
	m.learnings = learnings
	m.refreshList()
}
