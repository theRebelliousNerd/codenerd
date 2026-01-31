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

// Activation threshold constants
const (
	MinActivationThreshold     = 0.0 // Show all nodes
	MaxActivationThreshold     = 1.0 // Hide all nodes with activation < 1.0
	DefaultActivationThreshold = 0.0 // Default: show all
	ActivationThresholdStep    = 0.1 // Step size for keyboard adjustment
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
// TODO: IMPROVEMENT: Implement tea.Model interface for LogicPane to handle its own events
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

	// Activation threshold filter - nodes below this threshold are hidden
	ActivationThreshold float64

	// Render cache for performance optimization
	cachedContent  string
	cacheValid     bool
	lastCacheWidth int // Track width changes that invalidate cache

	// Pre-compiled styles for performance (avoid recreation per node)
	predStyle  lipgloss.Style
	argsStyle  lipgloss.Style
	ruleStyle  lipgloss.Style
	activStyle lipgloss.Style
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

// NewLogicPane creates a new logic visualization pane
func NewLogicPane(styles Styles, width, height int) LogicPane {
	vp := viewport.New(width, height)
	vp.SetContent("")

	return LogicPane{
		Viewport:            vp,
		Styles:              styles,
		Mode:                ModeSinglePane,
		Width:               width,
		Height:              height,
		ShowActivation:      true,
		SelectedNode:        0,
		Nodes:               make([]*DerivationNode, 0),
		ActivationThreshold: DefaultActivationThreshold,
		// Pre-compile styles for performance
		predStyle:  lipgloss.NewStyle().Foreground(styles.Theme.Primary).Bold(true),
		argsStyle:  lipgloss.NewStyle().Foreground(styles.Theme.Foreground),
		ruleStyle:  lipgloss.NewStyle().Foreground(styles.Theme.Muted).Italic(true),
		activStyle: lipgloss.NewStyle().Foreground(Success),
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
	p.invalidateCache()
	if trace != nil {
		// Use filtered flattening if threshold is set
		if p.ActivationThreshold > 0 {
			p.Nodes = p.flattenNodesFiltered(trace.RootNodes, 0)
		} else {
			p.Nodes = p.flattenNodes(trace.RootNodes, 0)
		}
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
	p.invalidateCache()
	p.Viewport.SetContent(p.renderContent())
}

// IncreaseActivationThreshold increases the threshold (hides more low-activation nodes)
// Keyboard shortcut: typically bound to + or =
func (p *LogicPane) IncreaseActivationThreshold() {
	p.ActivationThreshold += ActivationThresholdStep
	if p.ActivationThreshold > MaxActivationThreshold {
		p.ActivationThreshold = MaxActivationThreshold
	}
	p.refreshNodes()
}

// DecreaseActivationThreshold decreases the threshold (shows more nodes)
// Keyboard shortcut: typically bound to - or _
func (p *LogicPane) DecreaseActivationThreshold() {
	p.ActivationThreshold -= ActivationThresholdStep
	if p.ActivationThreshold < MinActivationThreshold {
		p.ActivationThreshold = MinActivationThreshold
	}
	p.refreshNodes()
}

// SetActivationThreshold sets the threshold to a specific value
func (p *LogicPane) SetActivationThreshold(threshold float64) {
	if threshold < MinActivationThreshold {
		threshold = MinActivationThreshold
	}
	if threshold > MaxActivationThreshold {
		threshold = MaxActivationThreshold
	}
	p.ActivationThreshold = threshold
	p.refreshNodes()
}

// ResetActivationThreshold resets the threshold to show all nodes
func (p *LogicPane) ResetActivationThreshold() {
	p.ActivationThreshold = DefaultActivationThreshold
	p.refreshNodes()
}

// GetActivationThreshold returns the current threshold value
func (p *LogicPane) GetActivationThreshold() float64 {
	return p.ActivationThreshold
}

// refreshNodes re-flattens and re-renders the node list with the current threshold
func (p *LogicPane) refreshNodes() {
	p.invalidateCache()
	if p.CurrentTrace != nil {
		p.Nodes = p.flattenNodesFiltered(p.CurrentTrace.RootNodes, 0)
		// Reset selection if it's now out of bounds
		if p.SelectedNode >= len(p.Nodes) {
			p.SelectedNode = len(p.Nodes) - 1
			if p.SelectedNode < 0 {
				p.SelectedNode = 0
			}
		}
	}
	p.Viewport.SetContent(p.renderContent())
}

// SelectNext selects the next node
// TODO: Implement circular navigation (wrap around to top).
func (p *LogicPane) SelectNext() {
	if len(p.Nodes) == 0 {
		return
	}
	if p.SelectedNode < len(p.Nodes)-1 {
		p.SelectedNode++
		p.invalidateCache()
		p.Viewport.SetContent(p.renderContent())
	}
}

// SelectPrev selects the previous node
func (p *LogicPane) SelectPrev() {
	if len(p.Nodes) == 0 {
		return
	}
	if p.SelectedNode > 0 {
		p.SelectedNode--
		p.invalidateCache()
		p.Viewport.SetContent(p.renderContent())
	}
}

// ToggleExpand toggles expansion of the selected node
func (p *LogicPane) ToggleExpand() {
	if len(p.Nodes) == 0 || p.SelectedNode < 0 || p.SelectedNode >= len(p.Nodes) {
		return
	}
	p.Nodes[p.SelectedNode].Expanded = !p.Nodes[p.SelectedNode].Expanded
	p.invalidateCache()
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

// flattenNodesFiltered creates a navigable flat list from the tree, filtering by activation threshold
func (p *LogicPane) flattenNodesFiltered(nodes []*DerivationNode, depth int) []*DerivationNode {
	result := make([]*DerivationNode, 0)
	for _, node := range nodes {
		// Filter out nodes below activation threshold (unless threshold is 0, show all)
		if p.ActivationThreshold > 0 && node.Activation < p.ActivationThreshold {
			continue
		}
		node.Depth = depth
		result = append(result, node)
		if node.Expanded && len(node.Children) > 0 {
			result = append(result, p.flattenNodesFiltered(node.Children, depth+1)...)
		}
	}
	return result
}

// invalidateCache marks the render cache as invalid, forcing a re-render on next call
func (p *LogicPane) invalidateCache() {
	p.cacheValid = false
}

// renderContent renders the logic pane content with caching for performance
func (p *LogicPane) renderContent() string {
	// Return cached content if valid and dimensions haven't changed
	if p.cacheValid && p.lastCacheWidth == p.Width {
		return p.cachedContent
	}

	if p.CurrentTrace == nil {
		content := p.renderEmptyState()
		p.cachedContent = content
		p.cacheValid = true
		p.lastCacheWidth = p.Width
		return content
	}

	var sb strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Styles.Theme.Primary).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(p.Styles.Theme.Border).
		Width(p.Width-4).
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

	// Cache the rendered content
	content := sb.String()
	p.cachedContent = content
	p.cacheValid = true
	p.lastCacheWidth = p.Width

	return content
}

// renderEmptyState renders the empty state message
// TODO: IMPROVEMENT: Replace magic number `Width - 4` with a named constant or calculated value from style margins.
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
// TODO: IMPROVEMENT: Improve tree visualization accessibility (e.g., consider screen reader friendly alternatives to ASCII art).
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
		Width(p.Width - 4)

	legend := "📊 Base Fact (EDB)  │  ⚡ Derived (IDB)  │  ▶ Expand  │  ▼ Collapse"

	if p.ShowActivation {
		legend += "\n█ Activation Score (Spreading Activation)"
	}

	// Show activation threshold if filtering is active
	if p.ActivationThreshold > 0 {
		legend += fmt.Sprintf("\n🔍 Threshold: %.1f (Press +/- to adjust, 0 to reset)", p.ActivationThreshold)
	} else {
		legend += "\n🔍 Threshold: OFF (Press + to filter low-activation nodes)"
	}

	return legendStyle.Render(legend)
}

// View returns the rendered view
func (p *LogicPane) View() string {
	return p.Viewport.View()
}

// Split ratio adjustment constants
const (
	MinSplitRatio     = 0.2  // Minimum left pane percentage
	MaxSplitRatio     = 0.9  // Maximum left pane percentage
	SplitRatioStep    = 0.05 // Step size for keyboard resize
	DefaultSplitRatio = 0.67 // Default left pane percentage (2/3 chat, 1/3 logic)
)

// SplitPaneView renders a split-pane view with chat and logic
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

// NewSplitPaneView creates a new split pane view with default ratio
func NewSplitPaneView(styles Styles, width, height int) SplitPaneView {
	return NewSplitPaneViewWithRatio(styles, width, height, DefaultSplitRatio)
}

// NewSplitPaneViewWithRatio creates a new split pane view with a configurable ratio
func NewSplitPaneViewWithRatio(styles Styles, width, height int, splitRatio float64) SplitPaneView {
	// Clamp ratio to valid range
	if splitRatio < MinSplitRatio {
		splitRatio = MinSplitRatio
	}
	if splitRatio > MaxSplitRatio {
		splitRatio = MaxSplitRatio
	}

	rightWidth := int(float64(width) * (1 - splitRatio))
	logicPane := NewLogicPane(styles, rightWidth, height-4)

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

// IncreaseSplitRatio increases the left pane size (moves divider right)
// Keyboard shortcut: typically bound to Ctrl+Right or ]
func (s *SplitPaneView) IncreaseSplitRatio() {
	s.SplitRatio += SplitRatioStep
	if s.SplitRatio > MaxSplitRatio {
		s.SplitRatio = MaxSplitRatio
	}
	s.updatePaneSizes()
}

// DecreaseSplitRatio decreases the left pane size (moves divider left)
// Keyboard shortcut: typically bound to Ctrl+Left or [
func (s *SplitPaneView) DecreaseSplitRatio() {
	s.SplitRatio -= SplitRatioStep
	if s.SplitRatio < MinSplitRatio {
		s.SplitRatio = MinSplitRatio
	}
	s.updatePaneSizes()
}

// SetSplitRatio sets the split ratio to a specific value (clamped to valid range)
func (s *SplitPaneView) SetSplitRatio(ratio float64) {
	if ratio < MinSplitRatio {
		ratio = MinSplitRatio
	}
	if ratio > MaxSplitRatio {
		ratio = MaxSplitRatio
	}
	s.SplitRatio = ratio
	s.updatePaneSizes()
}

// ResetSplitRatio resets the split ratio to the default value
func (s *SplitPaneView) ResetSplitRatio() {
	s.SplitRatio = DefaultSplitRatio
	s.updatePaneSizes()
}

// updatePaneSizes recalculates pane sizes after ratio change
func (s *SplitPaneView) updatePaneSizes() {
	rightWidth := int(float64(s.Width) * (1 - s.SplitRatio))
	s.RightPane.SetSize(rightWidth-4, s.Height-4)
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
		s.RightPane.SetSize(s.Width-4, s.Height-4)
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

// renderSplit renders the split view
// TODO: IMPROVEMENT: Use `bubbles/layout` or a similar robust layout library instead of manual width/height calculations.
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
