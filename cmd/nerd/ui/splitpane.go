// Package ui provides the split-pane TUI component for logic visualization.
// The Glass Box Interface shows live Mangle derivations alongside the chat.
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

// PaneMode represents the current display mode of the split pane
type PaneMode int

const (
	ModeSinglePane PaneMode = iota // Chat only
	ModeSplitPane                  // Chat + Logic visualization
	ModeFullLogic                  // Logic visualization only
)

// DerivationNode represents a node in the derivation tree
type DerivationNode struct {
	Predicate  string
	Args       []string
	Source     string // "edb" (base fact) or "idb" (derived)
	Rule       string // The rule that derived this
	Children   []*DerivationNode
	Depth      int
	Expanded   bool
	Timestamp  time.Time
	Activation float64 // Spreading activation score
}

// DerivationTrace represents the full derivation trace
type DerivationTrace struct {
	Query       string
	RootNodes   []*DerivationNode
	TotalFacts  int
	DerivedTime time.Duration
}

// LogicPane represents the logic visualization pane
type LogicPane struct {
	Viewport       viewport.Model
	Styles         Styles
	CurrentTrace   *DerivationTrace
	Mode           PaneMode
	Width          int
	Height         int
	ShowActivation bool
	SelectedNode   int
	Nodes          []*DerivationNode // Flattened list for navigation
	ScrollOffset   int
}

// NewLogicPane creates a new logic visualization pane
func NewLogicPane(styles Styles, width, height int) LogicPane {
	vp := viewport.New(width, height)
	vp.SetContent("")

	return LogicPane{
		Viewport:       vp,
		Styles:         styles,
		Mode:           ModeSinglePane,
		Width:          width,
		Height:         height,
		ShowActivation: true,
		SelectedNode:   0,
		Nodes:          make([]*DerivationNode, 0),
	}
}

// SetSize updates the pane dimensions
func (p *LogicPane) SetSize(width, height int) {
	p.Width = width
	p.Height = height
	p.Viewport.Width = width
	p.Viewport.Height = height
}

// SetTrace updates the current derivation trace
func (p *LogicPane) SetTrace(trace *DerivationTrace) {
	p.CurrentTrace = trace
	if trace != nil {
		p.Nodes = p.flattenNodes(trace.RootNodes, 0)
	} else {
		p.Nodes = nil
	}
	p.Viewport.SetContent(p.renderContent())
}

// ToggleMode cycles through display modes
func (p *LogicPane) ToggleMode() {
	switch p.Mode {
	case ModeSinglePane:
		p.Mode = ModeSplitPane
	case ModeSplitPane:
		p.Mode = ModeFullLogic
	case ModeFullLogic:
		p.Mode = ModeSinglePane
	}
}

// ToggleActivation toggles the activation score display
func (p *LogicPane) ToggleActivation() {
	p.ShowActivation = !p.ShowActivation
	p.Viewport.SetContent(p.renderContent())
}

// SelectNext selects the next node
func (p *LogicPane) SelectNext() {
	if len(p.Nodes) > 0 && p.SelectedNode < len(p.Nodes)-1 {
		p.SelectedNode++
		p.Viewport.SetContent(p.renderContent())
	}
}

// SelectPrev selects the previous node
func (p *LogicPane) SelectPrev() {
	if p.SelectedNode > 0 {
		p.SelectedNode--
		p.Viewport.SetContent(p.renderContent())
	}
}

// ToggleExpand toggles expansion of the selected node
func (p *LogicPane) ToggleExpand() {
	if len(p.Nodes) > 0 && p.SelectedNode < len(p.Nodes) {
		p.Nodes[p.SelectedNode].Expanded = !p.Nodes[p.SelectedNode].Expanded
		p.Viewport.SetContent(p.renderContent())
	}
}

// flattenNodes creates a navigable flat list from the tree
func (p *LogicPane) flattenNodes(nodes []*DerivationNode, depth int) []*DerivationNode {
	result := make([]*DerivationNode, 0)
	for _, node := range nodes {
		node.Depth = depth
		result = append(result, node)
		if node.Expanded && len(node.Children) > 0 {
			result = append(result, p.flattenNodes(node.Children, depth+1)...)
		}
	}
	return result
}

// renderContent renders the logic pane content
func (p *LogicPane) renderContent() string {
	if p.CurrentTrace == nil {
		return p.renderEmptyState()
	}

	var sb strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Styles.Theme.Primary).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(p.Styles.Theme.Border).
		Width(p.Width - 4).
		Padding(0, 1)

	sb.WriteString(headerStyle.Render("🔬 Derivation Trace"))
	sb.WriteString("\n\n")

	// Query info
	queryStyle := lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Accent).
		Italic(true)

	sb.WriteString(queryStyle.Render(fmt.Sprintf("Query: %s", p.CurrentTrace.Query)))
	sb.WriteString("\n")

	infoStyle := lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Muted)

	sb.WriteString(infoStyle.Render(fmt.Sprintf("Facts: %d │ Time: %v",
		p.CurrentTrace.TotalFacts,
		p.CurrentTrace.DerivedTime.Round(time.Millisecond))))
	sb.WriteString("\n\n")

	// Derivation tree
	sb.WriteString(p.renderTree())

	// Legend
	sb.WriteString("\n\n")
	sb.WriteString(p.renderLegend())

	return sb.String()
}

// renderEmptyState renders the empty state message
func (p *LogicPane) renderEmptyState() string {
	emptyStyle := lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Muted).
		Italic(true).
		Padding(2).
		Width(p.Width - 4).
		Align(lipgloss.Center)

	msg := `┌─────────────────────────────┐
│     🔬 Glass Box View       │
│                             │
│  No derivation trace yet.   │
│                             │
│  Execute a query to see     │
│  the logic derivation tree. │
│                             │
│  Use /query <predicate>     │
│  or /why <predicate>        │
└─────────────────────────────┘`

	return emptyStyle.Render(msg)
}

// renderTree renders the derivation tree
func (p *LogicPane) renderTree() string {
	if len(p.Nodes) == 0 {
		return ""
	}

	var sb strings.Builder

	for i, node := range p.Nodes {
		sb.WriteString(p.renderNode(node, i == p.SelectedNode))
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderNode renders a single derivation node
func (p *LogicPane) renderNode(node *DerivationNode, selected bool) string {
	var sb strings.Builder

	// Indentation
	indent := strings.Repeat("  ", node.Depth)

	// Tree connector
	connector := "├─"
	if node.Depth == 0 {
		connector = "●"
	}

	// Expand/collapse indicator
	expandIndicator := " "
	if len(node.Children) > 0 {
		if node.Expanded {
			expandIndicator = "▼"
		} else {
			expandIndicator = "▶"
		}
	}

	// Source indicator
	sourceIndicator := "📊" // EDB (base fact)
	if node.Source == "idb" {
		sourceIndicator = "⚡" // IDB (derived)
	}

	// Activation bar
	activationBar := ""
	if p.ShowActivation && node.Activation > 0 {
		barWidth := int(node.Activation * 10)
		if barWidth > 10 {
			barWidth = 10
		}
		activationBar = fmt.Sprintf(" [%s%s]",
			strings.Repeat("█", barWidth),
			strings.Repeat("░", 10-barWidth))
	}

	// Build the line
	predStyle := lipgloss.NewStyle().Foreground(p.Styles.Theme.Primary).Bold(true)
	argsStyle := lipgloss.NewStyle().Foreground(p.Styles.Theme.Foreground)
	ruleStyle := lipgloss.NewStyle().Foreground(p.Styles.Theme.Muted).Italic(true)
	activStyle := lipgloss.NewStyle().Foreground(Success)

	// Selection highlight
	if selected {
		predStyle = predStyle.Background(p.Styles.Theme.Secondary)
		argsStyle = argsStyle.Background(p.Styles.Theme.Secondary)
	}

	// Format predicate with args
	argsStr := ""
	if len(node.Args) > 0 {
		argsStr = "(" + strings.Join(node.Args, ", ") + ")"
	}

	sb.WriteString(indent)
	sb.WriteString(connector)
	sb.WriteString(" ")
	sb.WriteString(expandIndicator)
	sb.WriteString(" ")
	sb.WriteString(sourceIndicator)
	sb.WriteString(" ")
	sb.WriteString(predStyle.Render(node.Predicate))
	sb.WriteString(argsStyle.Render(argsStr))

	if p.ShowActivation && activationBar != "" {
		sb.WriteString(activStyle.Render(activationBar))
	}

	// Show rule on expanded nodes
	if node.Expanded && node.Rule != "" {
		sb.WriteString("\n")
		sb.WriteString(indent)
		sb.WriteString("   ")
		sb.WriteString(ruleStyle.Render("← " + node.Rule))
	}

	return sb.String()
}

// renderLegend renders the legend explaining the symbols
func (p *LogicPane) renderLegend() string {
	legendStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Styles.Theme.Border).
		Padding(0, 1).
		Width(p.Width - 4)

	legend := "📊 Base Fact (EDB)  │  ⚡ Derived (IDB)  │  ▶ Expand  │  ▼ Collapse"

	if p.ShowActivation {
		legend += "\n█ Activation Score (Spreading Activation)"
	}

	return legendStyle.Render(legend)
}

// View returns the rendered view
func (p *LogicPane) View() string {
	return p.Viewport.View()
}

// SplitPaneView renders a split-pane view with chat and logic
type SplitPaneView struct {
	Styles      Styles
	LeftPane    string // Chat content
	RightPane   *LogicPane
	Mode        PaneMode
	Width       int
	Height      int
	SplitRatio  float64 // 0.0-1.0, left pane percentage
	FocusRight  bool
}

// NewSplitPaneView creates a new split pane view
func NewSplitPaneView(styles Styles, width, height int) SplitPaneView {
	rightWidth := width / 3
	logicPane := NewLogicPane(styles, rightWidth, height-4)

	return SplitPaneView{
		Styles:     styles,
		RightPane:  &logicPane,
		Mode:       ModeSinglePane,
		Width:      width,
		Height:     height,
		SplitRatio: 0.65,
		FocusRight: false,
	}
}

// SetSize updates dimensions
func (s *SplitPaneView) SetSize(width, height int) {
	s.Width = width
	s.Height = height

	rightWidth := int(float64(width) * (1 - s.SplitRatio))
	s.RightPane.SetSize(rightWidth-4, height-4)
}

// SetMode sets the display mode
func (s *SplitPaneView) SetMode(mode PaneMode) {
	s.Mode = mode
	s.RightPane.Mode = mode
}

// ToggleFocus switches focus between panes
func (s *SplitPaneView) ToggleFocus() {
	s.FocusRight = !s.FocusRight
}

// Render renders the complete split pane view
func (s *SplitPaneView) Render(leftContent string) string {
	switch s.Mode {
	case ModeSinglePane:
		return leftContent

	case ModeFullLogic:
		s.RightPane.SetSize(s.Width-4, s.Height-4)
		return s.RightPane.renderContent()

	case ModeSplitPane:
		return s.renderSplit(leftContent)

	default:
		return leftContent
	}
}

// renderSplit renders the split view
func (s *SplitPaneView) renderSplit(leftContent string) string {
	leftWidth := int(float64(s.Width) * s.SplitRatio)
	rightWidth := s.Width - leftWidth - 1 // -1 for divider

	// Style for left pane
	leftStyle := lipgloss.NewStyle().
		Width(leftWidth).
		Height(s.Height).
		MaxHeight(s.Height)

	// Style for divider
	dividerStyle := lipgloss.NewStyle().
		Width(1).
		Height(s.Height).
		Background(s.Styles.Theme.Border).
		Foreground(s.Styles.Theme.Muted)

	// Style for right pane
	rightBorder := lipgloss.NormalBorder()
	if s.FocusRight {
		rightBorder = lipgloss.ThickBorder()
	}
	rightStyle := lipgloss.NewStyle().
		Width(rightWidth - 2).
		Height(s.Height - 2).
		MaxHeight(s.Height - 2).
		Border(rightBorder).
		BorderForeground(func() lipgloss.Color {
			if s.FocusRight {
				return s.Styles.Theme.Accent
			}
			return s.Styles.Theme.Border
		}())

	// Build divider
	var divider strings.Builder
	for i := 0; i < s.Height; i++ {
		divider.WriteString("│\n")
	}

	// Update right pane size and render
	s.RightPane.SetSize(rightWidth-4, s.Height-4)

	// Join horizontally
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftStyle.Render(leftContent),
		dividerStyle.Render(divider.String()),
		rightStyle.Render(s.RightPane.renderContent()),
	)
}

// CreateMockTrace creates a mock derivation trace for demonstration
func CreateMockTrace(query string) *DerivationTrace {
	return &DerivationTrace{
		Query:       query,
		TotalFacts:  42,
		DerivedTime: 15 * time.Millisecond,
		RootNodes: []*DerivationNode{
			{
				Predicate:  "next_action",
				Args:       []string{"/execute"},
				Source:     "idb",
				Rule:       "next_action(A) :- user_intent(_, V, T, _), action_mapping(V, A).",
				Expanded:   true,
				Activation: 0.95,
				Children: []*DerivationNode{
					{
						Predicate:  "user_intent",
						Args:       []string{"/i001", "/query", "auth system", ""},
						Source:     "edb",
						Expanded:   false,
						Activation: 0.85,
					},
					{
						Predicate:  "action_mapping",
						Args:       []string{"/query", "/execute"},
						Source:     "edb",
						Expanded:   false,
						Activation: 0.75,
					},
				},
			},
			{
				Predicate:  "permitted",
				Args:       []string{"/execute"},
				Source:     "idb",
				Rule:       "permitted(A) :- safe_action(A).",
				Expanded:   true,
				Activation: 0.9,
				Children: []*DerivationNode{
					{
						Predicate:  "safe_action",
						Args:       []string{"/execute"},
						Source:     "edb",
						Expanded:   false,
						Activation: 0.8,
					},
				},
			},
		},
	}
}
