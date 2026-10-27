package chat

import (
	"strings"

	"codenerd/cmd/nerd/ui"

	"charm.land/glamour/v2"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

// =============================================================================
// CHAT LAYOUT — the one place frame geometry is decided
// =============================================================================
//
// Every width the chat renders at derives from applyLayout: terminal size
// minus whichever sidebars (logic split pane, campaign panel) are open,
// minus the content padding those columns sit inside. Sidebar toggles,
// campaign start/finish, error panel show/hide and terminal resizes all
// cross it, so the wrap width can never disagree with the frame.
//
// Until 2026-10-27 the math lived only in the resize handler, and three
// paths bypassed it: opening the campaign sidebar joined a ~29-column
// panel beside a full-width chat (150-column lines at a 120-column
// terminal, hard-wrapped by the terminal), Alt+G pane cycling never
// refit, and the rendered-message cache was keyed by index alone, so a
// resize rebuilt the markdown renderer but kept serving text wrapped at
// the old width — the viewport then clipped it instead of wrapping it.

const (
	// contentPadX/contentPadY mirror ui.Layout.Content's Padding(1, 2):
	// two columns and one row on each side of the chat viewport.
	contentPadX = 4
	contentPadY = 2

	// campaignGap is the blank columns JoinHorizontal puts between the
	// chat column and the campaign sidebar ("  " in renderChatView).
	campaignGap = 2

	// minBesideChatWidth is the narrowest chat column worth keeping when
	// the campaign sidebar sits beside it; below it the panel stacks
	// underneath instead of squeezing the chat into a ribbon.
	minBesideChatWidth = 40

	// wrapSlack is left between the markdown wrap width and the viewport
	// width: glamour's own margins and table borders render outside the
	// word-wrap width it is given.
	wrapSlack = 4

	// minSplitFrameWidth is the narrowest row the logic split pane can
	// render into: below it the right pane's own labels overflow its box
	// (a 51-column frame at a 20-column terminal), so the split collapses
	// to single-pane until the frame widens again. The user's pane mode
	// is untouched; only the rendered mode collapses.
	minSplitFrameWidth = 60
)

// campaignPanelSize measures the campaign sidebar as rendered, or (0, 0)
// when it is not on screen.
func (m Model) campaignPanelSize() (width, height int) {
	if !m.showCampaignPanel || m.activeCampaign == nil {
		return 0, 0
	}
	panel := m.renderCampaignPanel()
	return lipgloss.Width(panel), lipgloss.Height(panel)
}

// campaignBeside reports whether the campaign sidebar fits beside a usable
// chat column at the current frame width. applyLayout and renderChatView
// both read m.campaignBeside, which applyLayout sets from this rule, so the
// reservation and the join can never disagree.
func (m Model) campaignFitsBeside(panelWidth int) bool {
	if panelWidth <= 0 {
		return false
	}
	return m.width-panelWidth-campaignGap >= minBesideChatWidth
}

// applyLayout recomputes every width-dependent piece of the model from the
// current frame size and sidebar state: viewport, error viewport, input,
// split pane, side pages, markdown wrap width and the rendered-message
// cache. It is idempotent; callers invoke it whenever anything it reads
// changes.
func (m Model) applyLayout() Model {
	if m.width < 1 || m.height < 1 {
		return m
	}
	frameW, frameH := m.width, m.height

	// Reserve the campaign sidebar first: it joins outside the split
	// frame, so everything to its left shrinks by its width.
	panelW, panelH := m.campaignPanelSize()
	m.campaignBeside = m.campaignFitsBeside(panelW)
	rowWidth := frameW
	stackedPanelH := 0
	if panelW > 0 {
		if m.campaignBeside {
			rowWidth -= panelW + campaignGap
		} else {
			stackedPanelH = panelH
		}
	}

	// The chat column inside the row: the split pane's left pane when the
	// logic sidebar is open, the whole row otherwise. The ratio is the
	// split pane's own, so the viewport is sized to the column it will
	// actually be joined into.
	splitActive := m.showLogic && m.splitPane != nil && m.paneMode == ui.ModeSplitPane && rowWidth >= minSplitFrameWidth
	chatOuter := rowWidth
	if splitActive {
		chatOuter = int(float64(rowWidth) * m.splitPane.SplitRatio)
	}
	viewportW := max(chatOuter-contentPadX, 1)
	wrapWidth := max(viewportW-wrapSlack, 1)

	// Vertical budget: header, footer (margin + line), the input box
	// (textarea + border), content padding, whichever of the error panel,
	// activity trail and a stacked campaign panel are on screen.
	const headerHeight = 4
	const footerHeight = 3
	inputHeight := m.textarea.Height() + 2 // rounded border top+bottom
	errorPanelHeight := 0
	if m.err != nil && m.showError {
		errorPanelHeight = 1 + errorPanelViewportHeight + 2
	}
	activityHeight := 0
	if activity := m.renderActivityLine(); activity != "" {
		activityHeight = strings.Count(activity, "\n") + 1
	}
	viewportH := max(frameH-headerHeight-footerHeight-inputHeight-contentPadY-
		errorPanelHeight-activityHeight-stackedPanelH, 1)

	if !m.ready {
		m.viewport = viewport.New(viewportW, viewportH)
		m.ready = true
	} else {
		m.viewport.Width = viewportW
		m.viewport.Height = viewportH
	}

	// The error panel sits inside the same padded content column as the
	// viewport; its bordered box must span exactly the viewport width, so
	// the text inside it gets that minus border (2) and padding (2).
	m.errorVP.Width = max(viewportW-4, 1)
	m.errorVP.Height = errorPanelViewportHeight
	if m.err != nil {
		m.refreshErrorViewport()
	}

	// The input box stacks below the chat row, across the full frame: its
	// width does not shrink when a sidebar opens beside the chat.
	m.textarea.SetWidth(max(frameW-4, 1))

	// Side pages and pickers are full-frame views; they follow the frame,
	// not the chat column. Guarded on splitPane as before: production
	// constructs that whole set together (session.go), bare test models
	// do not, and a zero-value bubbles list panics in SetSize.
	if m.splitPane != nil {
		m.list.SetSize(frameW, frameH)
		m.filepicker.SetHeight(frameH - 15)
		// Sync the rendered mode with the effective layout: a split the
		// frame is too narrow for renders single-pane until it widens.
		renderMode := m.paneMode
		if renderMode == ui.ModeSplitPane && !splitActive {
			renderMode = ui.ModeSinglePane
		}
		m.splitPane.SetMode(renderMode)
		m.splitPane.SetSize(rowWidth, viewportH+contentPadY)
		m.usagePage.SetSize(frameW, frameH-headerHeight)
		m.campaignPage.SetSize(frameW, frameH-headerHeight)
		m.autoPage.SetSize(frameW, frameH-headerHeight)
	}

	// A new wrap width invalidates every cached rendering: glamour wraps
	// at render time, so a message rendered for the old column would be
	// clipped by the narrower viewport, not rewrapped.
	if m.wrapWidth != wrapWidth {
		m.wrapWidth = wrapWidth
		if m.renderer != nil {
			m.renderer, _ = glamour.NewTermRenderer(
				glamour.WithStandardStyle("dark"),
				glamour.WithWordWrap(wrapWidth),
			)
		}
		m.renderedCache = make(map[int]string)
		m.cacheInvalidFrom = 0
		m.renderedCacheWidth = wrapWidth
	}
	m.viewport.SetContent(m.renderHistory())

	return m
}

// handleWindowSizeMsg processes the windowSizeMsg: record the new frame
// size and refit everything to it.
func (m Model) handleWindowSizeMsg(msg windowSizeMsg) Model {
	m.width = msg.Width
	m.height = msg.Height
	return m.applyLayout()
}
