// Package ui layout constants for consistent spacing and dimensions
package ui

// Layout constants for viewport and panel sizing
const (
	// Viewport padding and margins
	ViewportHorizontalPadding = 4
	ViewportVerticalPadding   = 8
	ViewportTopMargin         = 2
	ViewportBottomMargin      = 2

	// Split pane dimensions
	SplitPaneLeftRatio  = 0.6
	SplitPaneRightRatio = 0.4
	SplitPaneDivider    = 1

	// Panel borders and spacing
	PanelBorderWidth  = 2
	PanelPaddingH     = 1
	PanelPaddingV     = 0
	PanelMargin       = 1
	ContentIndent     = 2
	ListIndent        = 4
	NestedIndent      = 2

	// Table dimensions
	TablePadding      = 2
	TableBorderWidth  = 2
	TableHeaderHeight = 3
	TableRowHeight    = 1

	// Control areas
	ControlsHeight      = 3
	HeaderHeight        = 4
	FooterHeight        = 2
	StatusBarHeight     = 1
	TabBarHeight        = 2
	HelpPaneHeight      = 3

	// Responsive breakpoints
	MinimumTerminalWidth  = 80
	MinimumTerminalHeight = 24
	CompactModeWidth      = 100
	FullFeaturesWidth     = 120

	// Content widths
	WarningBoxPadding = 4
	DetailPaneWidth   = 40
	MinContentWidth   = 60
)

// LayoutConfig provides computed layout dimensions based on terminal size
type LayoutConfig struct {
	TerminalWidth  int
	TerminalHeight int
	IsCompact      bool
	IsFullWidth    bool
}

// NewLayoutConfig creates a layout configuration for the given terminal size
func NewLayoutConfig(width, height int) LayoutConfig {
	return LayoutConfig{
		TerminalWidth:  width,
		TerminalHeight: height,
		IsCompact:      width < CompactModeWidth,
		IsFullWidth:    width >= FullFeaturesWidth,
	}
}

// ContentWidth returns the usable content width for a viewport
func (l LayoutConfig) ContentWidth() int {
	return l.TerminalWidth - ViewportHorizontalPadding
}

// ContentHeight returns the usable content height for a viewport
func (l LayoutConfig) ContentHeight() int {
	return l.TerminalHeight - ViewportVerticalPadding
}

// ViewportWidth returns the viewport width for a given container width
func ViewportWidth(containerWidth int) int {
	return containerWidth - ViewportHorizontalPadding
}

// ViewportHeight returns the viewport height for a given container height
func ViewportHeight(containerHeight int) int {
	return containerHeight - ViewportVerticalPadding
}

// SplitPaneWidths calculates left and right pane widths for a split view
func SplitPaneWidths(totalWidth int) (leftWidth, rightWidth int) {
	leftWidth = int(float64(totalWidth) * SplitPaneLeftRatio)
	rightWidth = totalWidth - leftWidth - SplitPaneDivider
	return
}

// PanelContentWidth returns the content width inside a bordered panel
func PanelContentWidth(panelWidth int) int {
	return panelWidth - (PanelBorderWidth * 2) - (PanelPaddingH * 2)
}

// PanelContentHeight returns the content height inside a bordered panel
func PanelContentHeight(panelHeight int) int {
	return panelHeight - (PanelBorderWidth * 2) - (PanelPaddingV * 2)
}

// TableContentHeight calculates available height for table rows
func TableContentHeight(totalHeight int) int {
	return totalHeight - TableHeaderHeight - ControlsHeight - TablePadding
}

// WarningBoxWidth calculates the width for warning boxes
func WarningBoxWidth(containerWidth int) int {
	return containerWidth - WarningBoxPadding*2
}
