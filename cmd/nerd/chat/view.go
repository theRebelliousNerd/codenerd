// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains view rendering functions for the TUI.
package chat

// Accessibility note: To check for screen readers, we might use os.Getenv("CLICOLOR") in the future.

import (
	"codenerd/cmd/nerd/ui"
	"codenerd/internal/transparency"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// =============================================================================
// UI STRINGS AND CONSTANTS
// =============================================================================
// These constants define user-visible strings for easy localization/customization.

const (
	// Role labels displayed in chat
	LabelUser          = "You"
	LabelAssistant     = "codeNERD"
	LabelToolExecution = "Tool Execution"

	// Role identifiers (internal)
	RoleUser      = "user"
	RoleSystem    = "system"
	RoleTool      = "tool"
	RoleAssistant = "assistant"
)

// =============================================================================
// VIEW RENDERING
// =============================================================================
// These functions render the TUI components: history, header, footer, etc.

func (m *Model) renderHistory() string {
	var sb strings.Builder

	// Optimization: Only render messages visible in viewport (with some buffer)
	// For very long sessions, this prevents O(N) rendering on every frame
	const historyRenderLimit = 100
	startIdx := 0
	if len(m.history) > historyRenderLimit {
		// Keep last 100 messages + some buffer for smooth scrolling
		startIdx = len(m.history) - historyRenderLimit
	}

	// Performance: Use message-level caching with invalidation
	// Bubbletea's View() is called frequently, so we avoid re-rendering unchanged messages.
	// The cache is populated in Update() via updateMessageCache() helper since View() uses a value receiver.
	// Future: Consider viewport-based rendering via bubbles/viewport for very long histories.
	for idx := startIdx; idx < len(m.history); idx++ {
		msg := m.history[idx]

		// Check cache first - use cached render if available and valid
		if m.renderedCache != nil && m.cacheInvalidFrom >= 0 && idx < m.cacheInvalidFrom {
			if cached, exists := m.renderedCache[idx]; exists {
				sb.WriteString(cached)
				continue
			}
		}

		// Message not cached or cache invalidated - render it now
		rendered := m.renderSingleMessage(msg)

		// Note: Can't update cache here (View() is value receiver, updates won't persist)
		// Cache is populated in Update() via updateMessageCache() helper
		sb.WriteString(rendered)
	}

	return sb.String()
}

// renderSingleMessage renders a single message without caching
func (m *Model) renderSingleMessage(msg Message) string {
	var rendered strings.Builder

	switch msg.Role {
	case RoleUser:
		// Render user message
		userStyle := m.styles.Text.Bold.
			Foreground(m.styles.Theme.Primary()).
			MarginTop(1)
		rendered.WriteString(userStyle.Render(LabelUser) + "\n")
		rendered.WriteString(m.styles.Interactive.UserInput.Render(msg.Content))
		rendered.WriteString("\n\n")

	case RoleSystem:
		// Render Glass Box system event (only when enabled)
		if !m.glassBoxEnabled {
			return ""
		}
		rendered.WriteString(m.renderGlassBoxMessage(msg))

	case RoleTool:
		// Render tool execution notification (ALWAYS shown, not gated by Glass Box)
		// Inline styles used here for specific tool highlight
		toolStyle := m.styles.Text.Bold.
			Foreground(lipgloss.Color("214")). // Orange for tool execution
			MarginTop(1)
		rendered.WriteString(toolStyle.Render(LabelToolExecution) + "\n")
		// Render tool output with markdown (result/error)
		markdownRendered := m.safeRenderMarkdown(msg.Content)
		rendered.WriteString(markdownRendered)
		rendered.WriteString("\n")

	default: // RoleAssistant
		// Render assistant message with markdown
		assistantStyle := m.styles.Text.Bold.
			Foreground(m.styles.Theme.Secondary()).
			MarginTop(1)
		rendered.WriteString(assistantStyle.Render(LabelAssistant) + "\n")

		// Render reasoning trace if present
		if msg.ThoughtSummary != "" {
			thoughtStyle := m.styles.Text.Muted.Italic(true)
			// Prefix with a thinking indicator, e.g., 🤔
			rendered.WriteString(thoughtStyle.Render("🤔 "+msg.ThoughtSummary) + "\n\n")
		}

		// Render markdown with panic recovery
		markdownRendered := m.safeRenderMarkdown(msg.Content)
		rendered.WriteString(markdownRendered)
		rendered.WriteString("\n")
	}

	// Render active stream if present. Two layers:
	//   1. Thought trace (dim+italic, prefixed with 🤔) streams in first
	//      and stays visible as the model reasons. This matches the
	//      pattern used for completed messages' ThoughtSummary, so the
	//      look stays consistent before/after stream end.
	//   2. Visible surface response streams below it, with the standard
	//      bold "nerd:" label and a cursor block █ to signal liveness.
	//
	// Either layer alone (or both together) is rendered when present.
	if m.isStreaming {
		if strings.TrimSpace(m.currentThought) != "" {
			thoughtStyle := m.styles.Text.Muted.Italic(true)
			// While streaming, append the same █ cursor we use on
			// surface output, so users can see thoughts are still
			// arriving even before any visible answer text begins.
			cursor := ""
			if m.currentStream == "" {
				cursor = " █"
			}
			rendered.WriteString(thoughtStyle.Render("🤔 "+m.currentThought+cursor) + "\n\n")
		}
		if m.currentStream != "" {
			rendered.WriteString("\n**nerd:**\n")
			rendered.WriteString("\n")
			markdownRendered := m.safeRenderMarkdown(m.currentStream + " █") // Add cursor block
			rendered.WriteString(markdownRendered)
			rendered.WriteString("\n")
		}
	}

	return rendered.String()
}

// glassBoxIcon returns a small unicode icon for a Glass Box category.
// Kept ASCII-friendly so Windows terminals without a Nerd Font still render.
func glassBoxIcon(c transparency.GlassBoxCategory) string {
	switch c {
	case transparency.CategoryPerception:
		return "◎"
	case transparency.CategoryKernel:
		return "◈"
	case transparency.CategoryJIT:
		return "⧉"
	case transparency.CategoryShard:
		return "⚡"
	case transparency.CategoryControl:
		return "▸"
	case transparency.CategoryRouting:
		return "⤷"
	default:
		return "•"
	}
}

// glassBoxLabelStyle returns the color style for a category pill.
func (m Model) glassBoxLabelStyle(c transparency.GlassBoxCategory) lipgloss.Style {
	switch c {
	case transparency.CategoryPerception:
		return m.styles.Status.Success
	case transparency.CategoryKernel:
		return m.styles.Status.Warning
	case transparency.CategoryShard:
		return m.styles.Text.Title
	case transparency.CategoryJIT:
		return m.styles.Status.Info
	case transparency.CategoryRouting:
		return m.styles.Status.Success
	case transparency.CategoryControl:
		return m.styles.Status.Info
	default:
		return m.styles.Text.Muted
	}
}

// renderGlassBoxMessage formats a Glass Box system event for display.
// Timeline-style chrome: "  │ ◎ PERCEPTION  summary" so the stream reads
// as a living log rather than flat system spam.
func (m Model) renderGlassBoxMessage(msg Message) string {
	icon := glassBoxIcon(msg.GlassBoxCategory)
	label := strings.ToUpper(string(msg.GlassBoxCategory))
	if label == "" {
		label = "SYS"
	}
	labelStyle := m.glassBoxLabelStyle(msg.GlassBoxCategory)

	// Split summary vs optional multi-line details.
	lines := strings.SplitN(msg.Content, "\n", 2)
	summary := strings.TrimSpace(lines[0])
	details := ""
	if len(lines) > 1 {
		details = strings.TrimSpace(lines[1])
	}

	// Timestamp chip for temporal feel.
	ts := ""
	if !msg.Time.IsZero() {
		ts = m.styles.Text.Muted.Render(msg.Time.Format("15:04:05"))
	}

	rail := m.styles.Text.Muted.Render("│")
	pill := labelStyle.Bold(true).Render(fmt.Sprintf("%s %s", icon, label))
	body := m.styles.Text.Muted.Render(summary)

	var b strings.Builder
	if ts != "" {
		b.WriteString(fmt.Sprintf("  %s %s  %s  %s\n", rail, pill, body, ts))
	} else {
		b.WriteString(fmt.Sprintf("  %s %s  %s\n", rail, pill, body))
	}

	// Expanded details hang under the rail.
	if details != "" && !msg.IsCollapsed {
		for _, dl := range strings.Split(details, "\n") {
			dl = strings.TrimSpace(dl)
			if dl == "" {
				continue
			}
			b.WriteString(fmt.Sprintf("  %s   %s\n",
				m.styles.Text.Muted.Render("│"),
				m.styles.Text.Muted.Italic(true).Render(dl),
			))
		}
	} else if details != "" && msg.IsCollapsed {
		b.WriteString(fmt.Sprintf("  %s   %s\n",
			m.styles.Text.Muted.Render("│"),
			m.styles.Text.Muted.Render("··· details collapsed"),
		))
	}

	return b.String()
}

// safeRenderMarkdown renders markdown with panic recovery
func (m Model) safeRenderMarkdown(content string) (result string) {
	defer func() {
		if r := recover(); r != nil {
			// Log panic and recover
			fmt.Printf("Error rendering markdown: %v\n", r)
			result = content
		}
	}()

	if m.renderer != nil && content != "" {
		rendered, err := m.renderer.Render(content)
		if err == nil {
			return rendered
		}
	}
	return content
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Handle Booting State
	if m.isBooting {
		return m.renderBootScreen()
	}

	// Delegate to specific view renderers
	switch m.viewMode {
	case ListView:
		return m.renderListView()
	case FilePickerView:
		return m.renderFilePickerView()
	case UsageView:
		return m.renderUsageView()
	case CampaignPage:
		return m.renderCampaignView()
	case PromptInspector:
		return m.renderJITView()
	case AutopoiesisPage:
		return m.renderAutopoiesisView()
	case ShardPage:
		return m.renderShardView()
	}

	return m.renderChatView()
}

func (m Model) renderErrorPanel() string {
	if m.err == nil {
		return ""
	}

	border := lipgloss.RoundedBorder()
	if m.focusError {
		border = lipgloss.ThickBorder()
	}

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(m.styles.Theme.Destructive()).
		Render("Error") +
		m.styles.Text.Muted.Render("  Alt+E: scroll  Alt+Shift+E: hide")

	panelStyle := lipgloss.NewStyle().
		Border(border).
		BorderForeground(m.styles.Theme.Destructive()).
		Padding(0, 1).
		Width(m.viewport.Width).
		MaxWidth(m.viewport.Width)

	return panelStyle.Render(lipgloss.JoinVertical(lipgloss.Left, header, m.errorVP.View()))
}

func (m Model) renderHeader() string {
	// Logo and title
	title := m.styles.Layout.Header.Render(" codeNERD ")
	version := m.styles.Components.Badge.Render("v1.0")
	workspace := m.styles.Text.Muted.Render(fmt.Sprintf(" %s", m.workspace))

	// Status indicators — live elapsed + latest beat so the header never
	// freezes on a static "Thinking..." while the system works.
	var status string
	if m.isLoading {
		spin := m.spinner.View()
		msg := m.statusMessage
		if msg == "" {
			msg = m.activityLine
		}
		if msg == "" {
			msg = "Working..."
		}
		// Prefer the freshest activity beat when status is generic.
		if m.activityLine != "" && (msg == "Thinking..." || msg == "Working...") {
			msg = m.activityLine
		}
		if len(msg) > 48 {
			msg = msg[:45] + "..."
		}
		elapsed := ""
		if !m.turnStartedAt.IsZero() {
			elapsed = fmt.Sprintf(" %s", formatElapsedShort(time.Since(m.turnStartedAt)))
		}
		status = lipgloss.JoinHorizontal(lipgloss.Center,
			spin, " ",
			m.styles.Components.Badge.Render(msg+elapsed),
		)
	} else {
		status = m.styles.Status.Success.Render("● Ready")
	}

	headerLine := lipgloss.JoinHorizontal(
		lipgloss.Center,
		title,
		" ",
		version,
		"  ",
		status,
	)

	// Clamp header and workspace to terminal width to prevent overflow
	maxW := max(m.width, 1)
	headerStyle := lipgloss.NewStyle().MaxWidth(maxW)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		headerStyle.Render(headerLine),
		headerStyle.Render(workspace),
		m.styles.RenderDivider(m.width),
	)
}

func (m Model) renderFooter() string {
	// Build continuation mode indicator
	modeChar := 'A' + rune(m.continuationMode)
	modeName := m.continuationMode.String()
	continuationModeStr := fmt.Sprintf("[%c] %s", modeChar, modeName)

	// Build pane mode indicator
	paneModeStr := ""
	switch m.paneMode {
	case ui.ModeSinglePane:
		paneModeStr = "Chat"
	case ui.ModeSplitPane:
		paneModeStr = "Split"
	case ui.ModeFullLogic:
		paneModeStr = "Logic"
	}

	// Add campaign indicator if active
	campaignIndicator := ""
	if m.activeCampaign != nil {
		progress := 0.0
		if m.activeCampaign.TotalTasks > 0 {
			progress = float64(m.activeCampaign.CompletedTasks) / float64(m.activeCampaign.TotalTasks) * 100
		}
		campaignIndicator = fmt.Sprintf(" | Campaign: %.0f%%", progress)
	}

	// Continuation progress indicator
	continuationIndicator := ""
	if m.continuationTotal > 0 {
		continuationIndicator = fmt.Sprintf(" | Step %d/%d", m.continuationStep, m.continuationTotal)
	}

	// Context window utilization indicator
	contextIndicator := ""
	if m.compressor != nil {
		used, total := m.compressor.GetBudgetUsage()
		if total > 0 {
			pct := float64(used) / float64(total) * 100
			contextIndicator = fmt.Sprintf(" | Ctx: %.0f%%", pct)
		}
	}

	// Memory usage indicator (process RAM)
	memoryIndicator := ""
	if m.memSysBytes > 0 {
		mb := float64(m.memSysBytes) / (1024 * 1024)
		memoryIndicator = fmt.Sprintf(" | RAM: %.0fMB", mb)
	}

	// Mouse mode indicator
	mouseIndicator := ""
	if !m.mouseEnabled {
		mouseIndicator = " | [SELECT]"
	}

	// Glass Box indicator
	glassIndicator := ""
	if m.glassBoxEnabled {
		if m.isLoading {
			glassIndicator = " | [GLASS LIVE]"
		} else {
			glassIndicator = " | [GLASS]"
		}
	}

	// Build hotkeys section - show Ctrl+X prominently when loading
	hotkeys := ""
	if m.isLoading {
		hotkeys = "Ctrl+X: STOP | "
	}
	hotkeys += "Shift+Tab: mode | Alt+L: logic | Alt+G: glass | Alt+D: debug | Alt+P: jit | Alt+A: auto | Alt+S: shards | /help"

	timestamp := time.Now().Format("15:04")
	help := m.styles.Text.Muted.Render(fmt.Sprintf("%s | %s%s%s%s%s%s%s | %s | %s",
		continuationModeStr, paneModeStr, campaignIndicator, continuationIndicator, contextIndicator, memoryIndicator, mouseIndicator, glassIndicator, timestamp, hotkeys))
	maxW := max(m.width, 1)
	return lipgloss.NewStyle().
		MarginTop(1).
		MaxWidth(maxW).
		Render(help)
}

func (m Model) renderBootScreen() string {
	spin := m.spinner.View()
	title := m.styles.Layout.Header.Render(" codeNERD ")

	subtitleText := "System Booting"
	detailText := "Initializing Kernel, Shards, and Knowledge Base..."
	if m.bootStage == BootStageScanning {
		subtitleText = "Indexing Workspace"
		if strings.TrimSpace(m.statusMessage) != "" {
			detailText = m.statusMessage
		} else {
			detailText = "Scanning workspace for fresh facts..."
		}
	}
	subtitle := m.styles.Components.Badge.Render(subtitleText)

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		"\n",
		spin,
		"\n",
		subtitle,
		m.styles.Text.Muted.Render(detailText),
	)

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}

func (m Model) renderListView() string {
	return m.styles.Layout.Content.Render(m.list.View())
}

func (m Model) renderFilePickerView() string {
	title := m.styles.Layout.Header.Render(" Select a file ")
	content := m.styles.Layout.Content.Render(m.filepicker.View())
	return lipgloss.JoinVertical(lipgloss.Left, title, content)
}

func (m Model) renderUsageView() string {
	return m.styles.Layout.Content.Render(m.usagePage.View())
}

func (m Model) renderCampaignView() string {
	return m.styles.Layout.Content.Render(m.campaignPage.View())
}

func (m Model) renderJITView() string {
	return m.styles.Layout.Content.Render(m.jitPage.View())
}

func (m Model) renderAutopoiesisView() string {
	return m.styles.Layout.Content.Render(m.autoPage.View())
}

func (m Model) renderShardView() string {
	return m.styles.Layout.Content.Render(m.shardPage.View())
}

func (m Model) renderChatView() string {
	// Header
	header := m.renderHeader()

	// Content area (chat viewport + optional error panel)
	content := m.viewport.View()
	if m.err != nil && m.showError {
		content = lipgloss.JoinVertical(lipgloss.Left, content, m.renderErrorPanel())
	}
	chatView := m.styles.Layout.Content.Render(content)

	// Apply split-pane view if enabled (Glass Box Interface)
	if m.showLogic && m.splitPane != nil {
		chatView = m.splitPane.Render(chatView)
	}

	// Show campaign panel if active
	if m.showCampaignPanel && m.activeCampaign != nil {
		campaignPanel := m.renderCampaignPanel()
		chatView = lipgloss.JoinHorizontal(lipgloss.Top, chatView, "  ", campaignPanel)
	}

	// Input area
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.Theme.Secondary()).
		Padding(0, 1)

	inputArea := inputStyle.Render(m.textarea.View())

	// Activity line: transient single-line ping above the input box.
	// Shows the most recent Glass Box event. Empty when nothing has
	// happened yet this session, so it disappears entirely.
	activity := m.renderActivityLine()

	// Footer (with mode indicator)
	footer := m.renderFooter()

	// Compose full view
	parts := []string{header, chatView}
	if activity != "" {
		parts = append(parts, activity)
	}
	parts = append(parts, inputArea, footer)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// renderActivityLine draws the live pulse panel above the input box.
// While work is in flight it shows spinner + elapsed + traveling bar +
// a short trail of recent beats so the screen never feels frozen.
// After a turn, a brief afterglow keeps the last few beats visible.
func (m Model) renderActivityLine() string {
	if !m.glassBoxEnabled {
		return ""
	}
	hasTrail := len(m.activityTrail) > 0 || strings.TrimSpace(m.activityLine) != ""
	if !hasTrail && !m.isLoading {
		return ""
	}

	// Afterglow: hide trail once the last beat is stale and we're idle.
	if !m.isLoading && !m.activityAt.IsZero() && time.Since(m.activityAt) > 20*time.Second {
		return ""
	}

	var lines []string
	lines = append(lines, m.renderLivePulseHeader())

	// Newest-first trail (cap already enforced on push).
	trail := m.activityTrail
	if len(trail) == 0 && m.activityLine != "" {
		trail = []activityPulse{{
			Summary:  m.activityLine,
			Category: transparency.GlassBoxCategory(m.activityIconCh),
			At:       m.activityAt,
		}}
	}
	for i, p := range trail {
		prefix := "  "
		if i == 0 {
			prefix = m.styles.Status.Success.Render("  ▸ ")
		} else {
			prefix = m.styles.Text.Muted.Render("  · ")
		}
		icon := glassBoxIcon(p.Category)
		catStyle := m.glassBoxLabelStyle(p.Category)
		summary := p.Summary
		if len(summary) > 72 {
			summary = summary[:69] + "..."
		}
		age := formatActivityAge(p.At)
		line := prefix + catStyle.Render(icon) + " " +
			m.styles.Text.Muted.Render(summary) +
			m.styles.Text.Muted.Italic(true).Render("  · "+age)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// renderLivePulseHeader is the animated top row of the activity panel.
func (m Model) renderLivePulseHeader() string {
	var parts []string

	if m.isLoading {
		spin := m.spinner.View()
		live := m.styles.Status.Success.Bold(true).Render("LIVE")
		parts = append(parts, "  "+spin+" "+live)

		if !m.turnStartedAt.IsZero() {
			elapsed := formatElapsedShort(time.Since(m.turnStartedAt))
			parts = append(parts, m.styles.Components.Badge.Render(elapsed))
		}

		// Traveling energy bar — pure visual "still working" cue.
		barW := 16
		if m.width > 0 && m.width < 60 {
			barW = 10
		}
		elapsed := time.Duration(0)
		if !m.turnStartedAt.IsZero() {
			elapsed = time.Since(m.turnStartedAt)
		} else if !m.activityAt.IsZero() {
			elapsed = time.Since(m.activityAt)
		}
		bar := m.styles.Status.Info.Render(livePulseBar(elapsed, barW))
		parts = append(parts, bar)

		if n := len(m.activityTrail); n > 0 {
			parts = append(parts, m.styles.Text.Muted.Render(fmt.Sprintf("%d beats", n)))
		}
	} else {
		// Afterglow header once the turn settles.
		parts = append(parts, m.styles.Text.Muted.Render("  ◈ recent"))
		if !m.activityAt.IsZero() {
			parts = append(parts, m.styles.Text.Muted.Italic(true).Render(formatActivityAge(m.activityAt)))
		}
	}

	return strings.Join(parts, "  ")
}

// livePulseBar draws a short traveling-brightness bar so idle waits still
// look alive. Pure function of elapsed time — no extra state needed.
func livePulseBar(elapsed time.Duration, width int) string {
	if width < 6 {
		width = 6
	}
	if width > 28 {
		width = 28
	}
	pos := int(elapsed.Milliseconds()/80) % width
	var b strings.Builder
	b.Grow(width)
	for i := 0; i < width; i++ {
		d := i - pos
		if d < 0 {
			d = -d
		}
		// Wrap distance for a circular sweep.
		wrap := width - d
		if wrap < d {
			d = wrap
		}
		switch {
		case d == 0:
			b.WriteRune('█')
		case d == 1:
			b.WriteRune('▓')
		case d == 2:
			b.WriteRune('▒')
		default:
			b.WriteRune('░')
		}
	}
	return b.String()
}

// formatActivityAge turns a timestamp into a short relative age label.
func formatActivityAge(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	d := time.Since(at)
	switch {
	case d < 400*time.Millisecond:
		return "now"
	case d < time.Second:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}

// formatElapsedShort formats a duration for the live header (e.g. 4.2s, 1m03s).
func formatElapsedShort(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	mins := int(d.Minutes())
	secs := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%02ds", mins, secs)
}
