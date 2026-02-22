package ui

import (
	"codenerd/internal/autopoiesis"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

const (
	listWidthPadding  = 4
	listHeightPadding = 10
	minTermWidth      = 20
)

// TabComponent defines the interface for autopoiesis dashboard tabs.
type TabComponent interface {
	Update(msg tea.Msg) (TabComponent, tea.Cmd)
	View() string
	SetSize(width, height int)
	UpdateData(patterns []*autopoiesis.DetectedPattern, learnings []*autopoiesis.ToolLearning)
	Title() string
	IsFiltering() bool
}

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

// PatternsTab implements the TabComponent for detected patterns.
type PatternsTab struct {
	list     list.Model
	patterns []*autopoiesis.DetectedPattern
	styles   Styles
	renderer *glamour.TermRenderer
	width    int
	height   int
}

func NewPatternsTab(styles Styles) *PatternsTab {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Detected Patterns"
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	return &PatternsTab{
		list:     l,
		styles:   styles,
		renderer: renderer,
	}
}

func (p *PatternsTab) Update(msg tea.Msg) (TabComponent, tea.Cmd) {
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

func (p *PatternsTab) View() string {
	var sb strings.Builder
	sb.WriteString(p.styles.Content.Render(p.list.View()))

	// Detail View (if selected)
	sb.WriteString("\n\n")
	if len(p.patterns) > 0 {
		item := p.list.SelectedItem()
		if pItem, ok := item.(patternItem); ok {
			pat := pItem.DetectedPattern
			if len(pat.Examples) > 0 {
				sb.WriteString(p.styles.Bold.Render("Example Trace:") + "\n")

				example := pat.Examples[0]
				formatted, isJSON := formatJSON(example)

				if isJSON && p.renderer != nil {
					md := fmt.Sprintf("```json\n%s\n```", formatted)
					rendered, err := p.renderer.Render(md)
					if err == nil {
						sb.WriteString(rendered)
					} else {
						sb.WriteString(formatted + "\n")
					}
				} else {
					sb.WriteString(example + "\n")
				}
			} else {
				sb.WriteString(p.styles.Muted.Render("No examples available.") + "\n")
			}
		}
	}
	return sb.String()
}

func (p *PatternsTab) SetSize(w, h int) {
	p.width = w
	p.height = h
	p.list.SetWidth(w - listWidthPadding)
	p.list.SetHeight(h - listHeightPadding)

	if w > minTermWidth {
		p.renderer, _ = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(w-listHeightPadding),
		)
	}
}

func (p *PatternsTab) UpdateData(patterns []*autopoiesis.DetectedPattern, _ []*autopoiesis.ToolLearning) {
	p.patterns = patterns
	items := make([]list.Item, 0, len(patterns))
	for _, pat := range patterns {
		items = append(items, patternItem{pat})
	}
	p.list.SetItems(items)
}

func (p *PatternsTab) Title() string {
	return "Patterns"
}

func (p *PatternsTab) IsFiltering() bool {
	return p.list.FilterState() == list.Filtering
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

// LearningsTab implements the TabComponent for tool learnings.
type LearningsTab struct {
	list      list.Model
	learnings []*autopoiesis.ToolLearning
	styles    Styles
	width     int
	height    int
}

func NewLearningsTab(styles Styles) *LearningsTab {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Tool Learnings"
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	return &LearningsTab{
		list:   l,
		styles: styles,
	}
}

func (l *LearningsTab) Update(msg tea.Msg) (TabComponent, tea.Cmd) {
	var cmd tea.Cmd
	l.list, cmd = l.list.Update(msg)
	return l, cmd
}

func (l *LearningsTab) View() string {
	return l.styles.Content.Render(l.list.View())
}

func (l *LearningsTab) SetSize(w, h int) {
	l.width = w
	l.height = h
	l.list.SetWidth(w - listWidthPadding)
	l.list.SetHeight(h - listHeightPadding)
}

func (l *LearningsTab) UpdateData(_ []*autopoiesis.DetectedPattern, learnings []*autopoiesis.ToolLearning) {
	l.learnings = learnings
	items := make([]list.Item, 0, len(learnings))
	for _, learn := range learnings {
		items = append(items, learningItem{learn})
	}
	l.list.SetItems(items)
}

func (l *LearningsTab) Title() string {
	return "Tool Learnings"
}

func (l *LearningsTab) IsFiltering() bool {
	return l.list.FilterState() == list.Filtering
}

// AutopoiesisPageModel defines the state of the Autopoiesis Dashboard.
type AutopoiesisPageModel struct {
	width    int
	height   int
	viewport viewport.Model

	// State
	tabs           []TabComponent
	activeTabIndex int

	// Styles
	styles Styles
}

// NewAutopoiesisPageModel creates a new dashboard.
func NewAutopoiesisPageModel() AutopoiesisPageModel {
	vp := viewport.New(0, 0)
	styles := DefaultStyles()

	tabs := []TabComponent{
		NewPatternsTab(styles),
		NewLearningsTab(styles),
	}

	return AutopoiesisPageModel{
		viewport:       vp,
		styles:         styles,
		tabs:           tabs,
		activeTabIndex: 0,
	}
}

// Init initializes the model.
func (m AutopoiesisPageModel) Init() tea.Cmd {
	return nil
}

// Update handles messages with comprehensive keyboard navigation.
func (m AutopoiesisPageModel) Update(msg tea.Msg) (AutopoiesisPageModel, tea.Cmd) {
	var cmd tea.Cmd

	// Get active tab
	var activeTab TabComponent
	if len(m.tabs) > 0 {
		activeTab = m.tabs[m.activeTabIndex]
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't intercept global navigation keys if the active tab is filtering
		if activeTab != nil && activeTab.IsFiltering() {
			// Let the tab handle it
			break
		}

		switch msg.String() {
		// Tab switching (Tab key)
		case "tab":
			m.activeTabIndex = (m.activeTabIndex + 1) % len(m.tabs)
			return m, nil
		case "shift+tab":
			m.activeTabIndex--
			if m.activeTabIndex < 0 {
				m.activeTabIndex = len(m.tabs) - 1
			}
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
	}

	// Delegate to active tab
	if activeTab != nil {
		var newTab TabComponent
		newTab, cmd = activeTab.Update(msg)
		m.tabs[m.activeTabIndex] = newTab
	}

	return m, cmd
}

// formatJSON attempts to format a string as indented JSON.
func formatJSON(input string) (string, bool) {
	var obj interface{}
	if err := json.Unmarshal([]byte(input), &obj); err != nil {
		return input, false
	}
	bytes, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return input, false
	}
	return string(bytes), true
}

// View renders the page.
func (m AutopoiesisPageModel) View() string {
	var sb strings.Builder

	// Header / Tabs
	var tabViews []string
	for i, tab := range m.tabs {
		style := m.styles.Muted
		label := fmt.Sprintf("[ %s ]", tab.Title())
		if i == m.activeTabIndex {
			style = m.styles.Info.Copy().Bold(true)
		}
		tabViews = append(tabViews, style.Render(label))
		tabViews = append(tabViews, "  ")
	}

	// Join tabs
	tabsHeader := lipgloss.JoinHorizontal(lipgloss.Top, tabViews...)
	tabsHeader = lipgloss.JoinHorizontal(lipgloss.Top, tabsHeader, m.styles.Muted.Render("(Press Tab to switch)"))

	sb.WriteString(tabsHeader + "\n\n")

	if len(m.tabs) > 0 {
		sb.WriteString(m.tabs[m.activeTabIndex].View())
	}

	return sb.String()
}

// SetSize updates the size.
func (m *AutopoiesisPageModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	for _, tab := range m.tabs {
		tab.SetSize(w, h)
	}
}

// UpdateContent updates the data.
func (m *AutopoiesisPageModel) UpdateContent(patterns []*autopoiesis.DetectedPattern, learnings []*autopoiesis.ToolLearning) {
	for _, tab := range m.tabs {
		tab.UpdateData(patterns, learnings)
	}
}
