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

// LogicPane represents the logic visualization pane with search/filter support
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
	Nodes          []*DerivationNode // Flattened list for navigation (filtered)
	AllNodes       []*DerivationNode // Unfiltered node list
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

	// Render cache for performance optimization
	renderCache  *CachedRender
	traceVersion int // Incremented when trace changes

	// Search/filter functionality
	SearchQuery  string // Current search query
	FilterSource string // Filter by source: "" (all), "edb", "idb"
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
	p.invalidateCache()
	if trace != nil {
		p.AllNodes = p.flattenNodes(trace.RootNodes, 0)
		p.applyFilters() // Apply any active filters
	} else {
		p.AllNodes = nil
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

func (p *LogicPane) invalidateCache() {
	p.cachedContent = ""
	p.cacheValid = false
	p.lastCacheWidth = 0
	if p.renderCache != nil {
		p.renderCache.Invalidate()
	}
}

// GetActivationThreshold returns the current threshold value
func (p *LogicPane) GetActivationThreshold() float64 {
	return p.ActivationThreshold
}

// refreshNodes re-flattens and re-renders the node list with the current threshold
func (p *LogicPane) refreshNodes() {
	p.invalidateCache()
	if p.CurrentTrace != nil {
		var selected *DerivationNode
		if p.SelectedNode >= 0 && p.SelectedNode < len(p.Nodes) {
			selected = p.Nodes[p.SelectedNode]
		}

		// Expansion changes affect the flattening; rebuild and reapply all active filters.
		p.AllNodes = p.flattenNodes(p.CurrentTrace.RootNodes, 0)
		p.applyFilters()

		// Restore selection if the selected node is still present post-filter.
		if selected != nil {
			for i, n := range p.Nodes {
				if n == selected {
					p.SelectedNode = i
					break
				}
			}
		}
	}
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

	selected := p.Nodes[p.SelectedNode]
	selected.Expanded = !selected.Expanded

	// Expansion affects which nodes are visible; rebuild and reapply filters.
	if p.CurrentTrace != nil {
		p.AllNodes = p.flattenNodes(p.CurrentTrace.RootNodes, 0)
		p.applyFilters()

		// Keep selection anchored on the same node if possible.
		for i, n := range p.Nodes {
			if n == selected {
				p.SelectedNode = i
				break
			}
		}
	}

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

func (p *LogicPane) flattenNodesFiltered(nodes []*DerivationNode, depth int) []*DerivationNode {
	flat := p.flattenNodes(nodes, depth)
	if p.ActivationThreshold <= MinActivationThreshold {
		return flat
	}

	filtered := make([]*DerivationNode, 0, len(flat))
	for _, node := range flat {
		if node.Activation >= p.ActivationThreshold {
			filtered = append(filtered, node)
		}
	}
	return filtered
}

// renderContent renders the logic pane content with hash-based caching
func (p *LogicPane) renderContent() string {
	if p.CurrentTrace == nil {
		content := p.renderEmptyState()
		p.cachedContent = content
		p.cacheValid = true
		p.lastCacheWidth = p.Width
		return content
	}

	// Use cache if available
	if p.renderCache != nil {
		cacheKey := []interface{}{
			p.traceVersion,
			p.Width,
			p.Height,
			p.ShowActivation,
			p.ActivationThreshold,
			p.SelectedNode,
			p.ScrollOffset,
			p.SearchQuery, // Include filter parameters in cache key
			p.FilterSource,
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

	// Build content sections
	sections := []string{
		header,
		"",
		query,
		info,
	}

	// Add filter status if filters are active
	if p.HasActiveFilters() {
		filterStyle := lipgloss.NewStyle().
			Foreground(Warning).
			Bold(true)
		sections = append(sections, "", filterStyle.Render("🔍 "+p.GetFilterStatus()))
	}

	// Add tree and legend
	sections = append(sections,
		"",
		p.renderTree(),
		"",
		p.renderLegend(),
	)

	// Compose vertically using lipgloss.Join
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
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

// applyFilters filters AllNodes based on SearchQuery and FilterSource
func (p *LogicPane) applyFilters() {
	if p.AllNodes == nil {
		p.Nodes = nil
		return
	}

	// Start with all nodes
	filtered := make([]*DerivationNode, 0, len(p.AllNodes))

	for _, node := range p.AllNodes {
		// Apply activation threshold filter.
		if p.ActivationThreshold > MinActivationThreshold && node.Activation < p.ActivationThreshold {
			continue
		}

		// Apply source filter
		if p.FilterSource != "" && node.Source != p.FilterSource {
			continue
		}

		// Apply search query (case-insensitive)
		if p.SearchQuery != "" {
			query := strings.ToLower(p.SearchQuery)
			predicateMatch := strings.Contains(strings.ToLower(node.Predicate), query)
			argsMatch := false
			for _, arg := range node.Args {
				if strings.Contains(strings.ToLower(arg), query) {
					argsMatch = true
					break
				}
			}
			ruleMatch := strings.Contains(strings.ToLower(node.Rule), query)

			if !predicateMatch && !argsMatch && !ruleMatch {
				continue
			}
		}

		filtered = append(filtered, node)
	}

	p.Nodes = filtered

	// Reset selection if out of bounds
	if p.SelectedNode >= len(p.Nodes) {
		p.SelectedNode = 0
	}

	// Invalidate cache since nodes changed
	if p.renderCache != nil {
		p.renderCache.Invalidate()
	}
}

// SetSearchQuery updates the search query and reapplies filters
func (p *LogicPane) SetSearchQuery(query string) {
	p.SearchQuery = query
	p.applyFilters()
	p.Viewport.SetContent(p.renderContent())
}

// SetFilterSource sets the source filter ("", "edb", or "idb") and reapplies filters
func (p *LogicPane) SetFilterSource(source string) {
	// Normalize source to lowercase
	source = strings.ToLower(source)
	if source != "" && source != "edb" && source != "idb" {
		return // Invalid source
	}
	p.FilterSource = source
	p.applyFilters()
	p.Viewport.SetContent(p.renderContent())
}

// ClearFilters removes all active filters
func (p *LogicPane) ClearFilters() {
	p.SearchQuery = ""
	p.FilterSource = ""
	p.applyFilters()
	p.Viewport.SetContent(p.renderContent())
}

// HasActiveFilters returns true if any filters are active
func (p *LogicPane) HasActiveFilters() bool {
	return p.SearchQuery != "" || p.FilterSource != ""
}

// GetFilterStatus returns a human-readable string describing active filters
func (p *LogicPane) GetFilterStatus() string {
	if !p.HasActiveFilters() {
		return ""
	}

	var parts []string
	if p.SearchQuery != "" {
		parts = append(parts, fmt.Sprintf("Search: %q", p.SearchQuery))
	}
	if p.FilterSource != "" {
		parts = append(parts, fmt.Sprintf("Source: %s", strings.ToUpper(p.FilterSource)))
	}

	return fmt.Sprintf("Filters: %s (showing %d/%d nodes)",
		strings.Join(parts, ", "),
		len(p.Nodes),
		len(p.AllNodes))
}

// renderTree renders the derivation tree using a shared strings.Builder
// TODO: Add minimap for large derivation trees.
func (p *LogicPane) renderTree() string {
	if len(p.Nodes) == 0 {
		// Show different message if filters are active
		if p.HasActiveFilters() {
			return p.Styles.Muted.Render("No nodes match the current filters.")
		}
		return ""
	}

	var sb strings.Builder
	// Pre-allocate to avoid reallocations. Estimate 100 chars per node.
	sb.Grow(len(p.Nodes) * 100)

	for i, node := range p.Nodes {
		if i > 0 {
			sb.WriteString("\n")
		}
		p.writeNode(&sb, node, i == p.SelectedNode)
	}

	return sb.String()
}

// writeNode writes a single derivation node to the provided builder
// TODO: IMPROVEMENT: Improve tree visualization accessibility (e.g., consider screen reader friendly alternatives to ASCII art).
// TODO: IMPROVEMENT: Implement custom rendering for specific predicates (e.g., clickable links).
func (p *LogicPane) writeNode(sb *strings.Builder, node *DerivationNode, selected bool) {
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

// IncreaseSplitRatio increases the left pane size (moves divider right).
func (s *SplitPaneView) IncreaseSplitRatio() {
	s.SetSplitRatio(s.SplitRatio + SplitRatioStep)
}

// DecreaseSplitRatio decreases the left pane size (moves divider left).
func (s *SplitPaneView) DecreaseSplitRatio() {
	s.SetSplitRatio(s.SplitRatio - SplitRatioStep)
}

// SetSplitRatio sets the split ratio, clamped to valid range.
func (s *SplitPaneView) SetSplitRatio(ratio float64) {
	if ratio < MinSplitRatio {
		ratio = MinSplitRatio
	}
	if ratio > MaxSplitRatio {
		ratio = MaxSplitRatio
	}
	if ratio == s.SplitRatio {
		return
	}
	s.SplitRatio = ratio
	s.SetSize(s.Width, s.Height)
}

// ResetSplitRatio resets the split ratio to the default value.
func (s *SplitPaneView) ResetSplitRatio() {
	s.SetSplitRatio(DefaultSplitRatio)
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
