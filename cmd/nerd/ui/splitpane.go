// Package ui provides the split-pane TUI component for logic visualization.
// The Glass Box Interface shows live Mangle derivations alongside the chat.
package ui

import (
	"fmt"
	"strings"
	"time"

	"codenerd/internal/mangle"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

// Pre-computed indentation strings to avoid repeated allocation
var indentCache [50]string

func init() {
	for i := 0; i < len(indentCache); i++ {
		indentCache[i] = strings.Repeat("  ", i)
	}
}

func getIndent(depth int) string {
	if depth >= 0 && depth < len(indentCache) {
		return indentCache[depth]
	}
	return strings.Repeat("  ", depth)
}

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
	// TODO: Add a threshold control to filter nodes with low activation scores.
}

// DerivationTrace represents the full derivation trace
type DerivationTrace struct {
	Query       string
	RootNodes   []*DerivationNode
	TotalFacts  int
	DerivedTime time.Duration
}

// LogicPane represents the logic visualization pane
// TODO: IMPROVEMENT: Implement tea.Model interface for LogicPane to handle its own events
// TODO: Allow copying derivation trace to clipboard.
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

	// Pre-compiled styles for performance (avoid recreation per node)
	predStyle  lipgloss.Style
	argsStyle  lipgloss.Style
	ruleStyle  lipgloss.Style
	activStyle lipgloss.Style

	// Render cache for performance optimization
	renderCache *CachedRender
	traceVersion int // Incremented when trace changes
}

// SetTraceMangle adapts a backend trace to the UI model
func (p *LogicPane) SetTraceMangle(trace *mangle.DerivationTrace) {
	if trace == nil {
		p.SetTrace(nil)
		return
	}

	uiTrace := &DerivationTrace{
		Query:       trace.Query,
		TotalFacts:  len(trace.AllNodes),
		DerivedTime: trace.Duration,
		RootNodes:   make([]*DerivationNode, len(trace.RootNodes)),
	}

	for i, node := range trace.RootNodes {
		uiTrace.RootNodes[i] = convertMangleNodeToUI(node)
	}

	p.SetTrace(uiTrace)
}

func convertMangleNodeToUI(node *mangle.DerivationNode) *DerivationNode {
	// Convert args to strings
	args := make([]string, len(node.Fact.Args))
	for i, arg := range node.Fact.Args {
		args[i] = fmt.Sprintf("%v", arg)
	}

	uiNode := &DerivationNode{
		Predicate: node.Fact.Predicate,
		Args:      args,
		Source:    string(node.Source), // "EDB" or "IDB"
		Rule:      node.RuleName,
		Depth:     node.Depth,
		Children:  make([]*DerivationNode, len(node.Children)),
		// UI specific defaults
		Expanded: true, // Expand by default

		Timestamp:  node.Timestamp,
		Activation: 0.0, // Backend doesn't provide this yet
		// NOTE: Activation scores are not currently exposed by the Mangle backend engine.
		// When internal/mangle/engine.go exposes spreading activation metrics, connect them here.
	}

	for i, child := range node.Children {
		uiNode.Children[i] = convertMangleNodeToUI(child)
	}

	return uiNode
}

// NewLogicPane creates a new logic visualization pane with render caching
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
		// Pre-compile styles for performance
		predStyle:  lipgloss.NewStyle().Foreground(styles.Theme.Primary).Bold(true),
		argsStyle:  lipgloss.NewStyle().Foreground(styles.Theme.Foreground),
		ruleStyle:  lipgloss.NewStyle().Foreground(styles.Theme.Muted).Italic(true),
		activStyle: lipgloss.NewStyle().Foreground(Success),
		// Initialize render cache
		renderCache:  NewCachedRender(DefaultRenderCache),
		traceVersion: 0,
	}
}

// SetSize updates the pane dimensions and invalidates cache
func (p *LogicPane) SetSize(width, height int) {
	p.Width = width
	p.Height = height
	p.Viewport.Width = width
	p.Viewport.Height = height
	// Invalidate cache on resize
	if p.renderCache != nil {
		p.renderCache.Invalidate()
	}
}

// SetTrace updates the current derivation trace and invalidates cache
func (p *LogicPane) SetTrace(trace *DerivationTrace) {
	p.CurrentTrace = trace
	if trace != nil {
		p.Nodes = p.flattenNodes(trace.RootNodes, 0)
	} else {
		p.Nodes = nil
	}
	// Invalidate cache and increment version on trace change
	p.traceVersion++
	if p.renderCache != nil {
		p.renderCache.Invalidate()
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

// SelectNext selects the next node with circular navigation (wraps to top)
func (p *LogicPane) SelectNext() {
	if len(p.Nodes) == 0 {
		return
	}
	p.SelectedNode = (p.SelectedNode + 1) % len(p.Nodes)
	if p.renderCache != nil {
		p.renderCache.Invalidate()
	}
	p.Viewport.SetContent(p.renderContent())
}

// SelectPrev selects the previous node with circular navigation (wraps to bottom)
func (p *LogicPane) SelectPrev() {
	if len(p.Nodes) == 0 {
		return
	}
	if p.SelectedNode <= 0 {
		p.SelectedNode = len(p.Nodes) - 1
	} else {
		p.SelectedNode--
	}
	if p.renderCache != nil {
		p.renderCache.Invalidate()
	}
	p.Viewport.SetContent(p.renderContent())
}

// ToggleExpand toggles expansion of the selected node
func (p *LogicPane) ToggleExpand() {
	if len(p.Nodes) == 0 || p.SelectedNode < 0 || p.SelectedNode >= len(p.Nodes) {
		return
	}
	p.Nodes[p.SelectedNode].Expanded = !p.Nodes[p.SelectedNode].Expanded
	p.Viewport.SetContent(p.renderContent())
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

// renderContent renders the logic pane content with hash-based caching
func (p *LogicPane) renderContent() string {
	if p.CurrentTrace == nil {
		return p.renderEmptyState()
	}

	// Use cache if available
	if p.renderCache != nil {
		cacheKey := []interface{}{
			p.traceVersion,
			p.Width,
			p.Height,
			p.ShowActivation,
			p.SelectedNode,
			p.ScrollOffset,
		}

		return p.renderCache.Render(cacheKey, func() string {
			return p.renderContentUncached()
		})
	}

	return p.renderContentUncached()
}

// renderContentUncached performs the actual rendering without caching using lipgloss.Join
func (p *LogicPane) renderContentUncached() string {
	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Styles.Theme.Primary).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(p.Styles.Theme.Border).
		Width(ViewportWidth(p.Width)).
		Padding(0, 1)

	header := headerStyle.Render("🔬 Derivation Trace")

	// Query info
	queryStyle := lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Accent).
		Italic(true)

	query := queryStyle.Render(fmt.Sprintf("Query: %s", p.CurrentTrace.Query))

	infoStyle := lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Muted)

	info := infoStyle.Render(fmt.Sprintf("Facts: %d │ Time: %v",
		p.CurrentTrace.TotalFacts,
		p.CurrentTrace.DerivedTime.Round(time.Millisecond)))

	// Compose vertically using lipgloss.Join
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		query,
		info,
		"",
		p.renderTree(),
		"",
		p.renderLegend(),
	)
}

// renderEmptyState renders the empty state message
func (p *LogicPane) renderEmptyState() string {
	emptyStyle := lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Muted).
		Italic(true).
		Padding(2).
		Width(ViewportWidth(p.Width)).
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

// renderTree renders the derivation tree using lipgloss.JoinVertical
// TODO: IMPROVEMENT: Add search/filter functionality for derivation nodes.
// TODO: Add minimap for large derivation trees.
func (p *LogicPane) renderTree() string {
	if len(p.Nodes) == 0 {
		return ""
	}

	// Render all nodes
	nodeStrings := make([]string, len(p.Nodes))
	for i, node := range p.Nodes {
		nodeStrings[i] = p.renderNode(node, i == p.SelectedNode)
	}

	return lipgloss.JoinVertical(lipgloss.Left, nodeStrings...)
}

// renderNode renders a single derivation node
// TODO: IMPROVEMENT: Improve tree visualization accessibility (e.g., consider screen reader friendly alternatives to ASCII art).
// TODO: IMPROVEMENT: Implement custom rendering for specific predicates (e.g., clickable links).
func (p *LogicPane) renderNode(node *DerivationNode, selected bool) string {
	var sb strings.Builder

	// Indentation
	// Indentation
	indent := getIndent(node.Depth)

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

	// Use pre-compiled styles, apply selection background if needed
	predStyle := p.predStyle
	argsStyle := p.argsStyle

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
		sb.WriteString(p.activStyle.Render(activationBar))
	}

	// Show rule on expanded nodes
	if node.Expanded && node.Rule != "" {
		sb.WriteString("\n")
		sb.WriteString(indent)
		sb.WriteString("   ")
		sb.WriteString(p.ruleStyle.Render("← " + node.Rule))
	}

	return sb.String()
}

// renderLegend renders the legend explaining the symbols
// TODO: IMPROVEMENT: Make legend responsive or collapsible on smaller screens.
func (p *LogicPane) renderLegend() string {
	legendStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Styles.Theme.Border).
		Padding(0, 1).
		Width(ViewportWidth(p.Width))

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
// TODO: IMPROVEMENT: Add support for resizing split ratio via mouse or keyboard
type SplitPaneView struct {
	Styles     Styles
	LeftPane   string // Chat content
	RightPane  *LogicPane
	Mode       PaneMode
	Width      int
	Height     int
	SplitRatio float64 // 0.0-1.0, left pane percentage
	FocusRight bool
}

// DefaultSplitRatio is the default left pane percentage (2/3 chat, 1/3 logic)
const DefaultSplitRatio = 0.67

// NewSplitPaneView creates a new split pane view with default ratio
func NewSplitPaneView(styles Styles, width, height int) SplitPaneView {
	return NewSplitPaneViewWithRatio(styles, width, height, DefaultSplitRatio)
}

// NewSplitPaneViewWithRatio creates a new split pane view with a configurable ratio
func NewSplitPaneViewWithRatio(styles Styles, width, height int, splitRatio float64) SplitPaneView {
	// Clamp ratio to valid range
	if splitRatio < 0.2 {
		splitRatio = 0.2
	}
	if splitRatio > 0.9 {
		splitRatio = 0.9
	}

	rightWidth := int(float64(width) * (1 - splitRatio))
	logicPane := NewLogicPane(styles, rightWidth, ViewportHeight(height))

	return SplitPaneView{
		Styles:     styles,
		RightPane:  &logicPane,
		Mode:       ModeSinglePane,
		Width:      width,
		Height:     height,
		SplitRatio: splitRatio,
		FocusRight: false,
	}
}

// SetSize updates dimensions
func (s *SplitPaneView) SetSize(width, height int) {
	s.Width = width
	s.Height = height

	rightWidth := int(float64(width) * (1 - s.SplitRatio))
	s.RightPane.SetSize(ViewportWidth(rightWidth), ViewportHeight(height))
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

// HandleKey processes keyboard input for split pane navigation
// Returns true if the key was handled, false otherwise
func (s *SplitPaneView) HandleKey(key string) bool {
	if s.Mode != ModeSplitPane || s.RightPane == nil {
		return false
	}

	switch key {
	case "ctrl+l", "ctrl+tab":
		// Toggle focus between left and right panes
		s.ToggleFocus()
		return true

	// Navigation in right pane (when focused)
	case "up", "k":
		if s.FocusRight {
			s.RightPane.SelectPrev()
			return true
		}
	case "down", "j":
		if s.FocusRight {
			s.RightPane.SelectNext()
			return true
		}
	case "enter", "space":
		if s.FocusRight {
			s.RightPane.ToggleExpand()
			return true
		}
	case "a":
		if s.FocusRight {
			s.RightPane.ToggleActivation()
			return true
		}
	}

	return false
}

// Render renders the complete split pane view
func (s *SplitPaneView) Render(leftContent string) string {
	switch s.Mode {
	case ModeSinglePane:
		return leftContent

	case ModeFullLogic:
		if s.RightPane == nil {
			return leftContent
		}
		s.RightPane.SetSize(ViewportWidth(s.Width), ViewportHeight(s.Height))
		return s.RightPane.renderContent()

	case ModeSplitPane:
		if s.RightPane == nil {
			return leftContent
		}
		return s.renderSplit(leftContent)

	default:
		return leftContent
	}
}

// renderSplit renders the split view using layout constants
func (s *SplitPaneView) renderSplit(leftContent string) string {
	leftWidth := int(float64(s.Width) * s.SplitRatio)
	rightWidth := s.Width - leftWidth - SplitPaneDivider

	// Style for left pane
	leftStyle := lipgloss.NewStyle().
		Width(leftWidth).
		Height(s.Height).
		MaxHeight(s.Height)

	// Style for divider
	dividerStyle := lipgloss.NewStyle().
		Width(SplitPaneDivider).
		Height(s.Height).
		Background(s.Styles.Theme.Border).
		Foreground(s.Styles.Theme.Muted)

	// Style for right pane
	rightBorder := lipgloss.NormalBorder()
	if s.FocusRight {
		rightBorder = lipgloss.ThickBorder()
	}
	rightStyle := lipgloss.NewStyle().
		Width(PanelContentWidth(rightWidth)).
		Height(PanelContentHeight(s.Height)).
		MaxHeight(PanelContentHeight(s.Height)).
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
	s.RightPane.SetSize(ViewportWidth(rightWidth), ViewportHeight(s.Height))

	// Join horizontally
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftStyle.Render(leftContent),
		dividerStyle.Render(divider.String()),
		rightStyle.Render(s.RightPane.renderContent()),
	)
}
