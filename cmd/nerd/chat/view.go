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

func (m Model) renderHistory() string {
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
func (m Model) renderSingleMessage(msg Message) string {
	var rendered strings.Builder

	switch msg.Role {
	case RoleUser:
		// Render user message
		userStyle := m.styles.Bold.
			Foreground(m.styles.Theme.Primary).
			MarginTop(1)
		rendered.WriteString(userStyle.Render(LabelUser) + "\n")
		rendered.WriteString(m.styles.UserInput.Render(msg.Content))
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
		toolStyle := m.styles.Bold.
			Foreground(lipgloss.Color("214")). // Orange for tool execution
			MarginTop(1)
		rendered.WriteString(toolStyle.Render(LabelToolExecution) + "\n")
		// Render tool output with markdown (result/error)
		markdownRendered := m.safeRenderMarkdown(msg.Content)
		rendered.WriteString(markdownRendered)
		rendered.WriteString("\n")

	default: // RoleAssistant
		// Render assistant message with markdown
		assistantStyle := m.styles.Bold.
			Foreground(m.styles.Theme.Secondary).
			MarginTop(1)
		rendered.WriteString(assistantStyle.Render(LabelAssistant) + "\n")

		// Render reasoning trace if present
		if msg.ThoughtSummary != "" {
			thoughtStyle := m.styles.Muted.Italic(true)
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
			thoughtStyle := m.styles.Muted.Italic(true)
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
		return "🎯"
	case transparency.CategoryKernel:
		return "🧠"
	case transparency.CategoryJIT:
		return "🧩"
	case transparency.CategoryShard:
		return "⚡"
	case transparency.CategoryControl:
		return "📦"
	case transparency.CategoryRouting:
		return "🛠"
	default:
		return "•"
	}
}

// renderGlassBoxMessage formats a Glass Box system event for display.
// Format: "  <icon> <CATEGORY> summary  (durationms)"
// Icon + category are color-keyed, body is dimmed so the event reads as
// chrome rather than chat content.
func (m Model) renderGlassBoxMessage(msg Message) string {
	icon := glassBoxIcon(msg.GlassBoxCategory)
	label := strings.ToUpper(string(msg.GlassBoxCategory))

	// Pick a color per category. Falls back to Muted if a category lacks a
	// dedicated style. We intentionally keep the palette narrow so the
	// status line doesn't strobe.
	var labelStyle lipgloss.Style
	switch msg.GlassBoxCategory {
	case transparency.CategoryPerception:
		labelStyle = m.styles.Success
	case transparency.CategoryKernel:
		labelStyle = m.styles.Warning
	case transparency.CategoryShard:
		labelStyle = m.styles.Title
	case transparency.CategoryJIT:
		labelStyle = m.styles.Info
	case transparency.CategoryRouting:
		labelStyle = m.styles.Success
	case transparency.CategoryControl:
		labelStyle = m.styles.Info
	default:
		labelStyle = m.styles.Muted
	}

	renderedLabel := labelStyle.Render(fmt.Sprintf("%s %s", icon, label))
	body := m.styles.Muted.Render(msg.Content)

	indicator := ""
	if strings.Contains(msg.Content, "\n") {
		if msg.IsCollapsed {
			indicator = m.styles.Muted.Render(" [+]")
		} else {
			indicator = m.styles.Muted.Render(" [-]")
		}
	}

	return fmt.Sprintf("  %s%s  %s\n", renderedLabel, indicator, body)
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
		Foreground(m.styles.Theme.Destructive).
		Render("Error") +
		m.styles.Muted.Render("  Alt+E: scroll  Alt+Shift+E: hide")

	panelStyle := lipgloss.NewStyle().
		Border(border).
		BorderForeground(m.styles.Theme.Destructive).
		Padding(0, 1).
		Width(m.viewport.Width).
		MaxWidth(m.viewport.Width)

	return panelStyle.Render(lipgloss.JoinVertical(lipgloss.Left, header, m.errorVP.View()))
}

func (m Model) renderHeader() string {
	// Logo and title
	title := m.styles.Header.Render(" codeNERD ")
	version := m.styles.Badge.Render("v1.0")
	workspace := m.styles.Muted.Render(fmt.Sprintf(" %s", m.workspace))

	// Status indicators
	var status string
	if m.isLoading {
		// Show spinner and detailed status message
		spin := m.spinner.View()
		msg := m.statusMessage
		if msg == "" {
			msg = "Thinking..."
		}
		status = lipgloss.JoinHorizontal(lipgloss.Center, spin, " ", m.styles.Badge.Render(msg))
	} else {
		status = m.styles.Success.Render("Ready")
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
		glassIndicator = " | [GLASS]"
	}

	// Build hotkeys section - show Ctrl+X prominently when loading
	hotkeys := ""
	if m.isLoading {
		hotkeys = "Ctrl+X: STOP | "
	}
	hotkeys += "Shift+Tab: mode | Alt+L: logic | Alt+D: debug | Alt+P: jit | Alt+A: auto | Alt+S: shards | /help"

	timestamp := time.Now().Format("15:04")
	help := m.styles.Muted.Render(fmt.Sprintf("%s | %s%s%s%s%s%s%s | %s | %s",
		continuationModeStr, paneModeStr, campaignIndicator, continuationIndicator, contextIndicator, memoryIndicator, mouseIndicator, glassIndicator, timestamp, hotkeys))
	maxW := max(m.width, 1)
	return lipgloss.NewStyle().
		MarginTop(1).
		MaxWidth(maxW).
		Render(help)
}

func (m Model) renderBootScreen() string {
	spin := m.spinner.View()
	title := m.styles.Header.Render(" codeNERD ")

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
	subtitle := m.styles.Badge.Render(subtitleText)

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		"\n",
		spin,
		"\n",
		subtitle,
		m.styles.Muted.Render(detailText),
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
	return m.styles.Content.Render(m.list.View())
}

func (m Model) renderFilePickerView() string {
	title := m.styles.Header.Render(" Select a file ")
	content := m.styles.Content.Render(m.filepicker.View())
	return lipgloss.JoinVertical(lipgloss.Left, title, content)
}

func (m Model) renderUsageView() string {
	return m.styles.Content.Render(m.usagePage.View())
}

func (m Model) renderCampaignView() string {
	return m.styles.Content.Render(m.campaignPage.View())
}

func (m Model) renderJITView() string {
	return m.styles.Content.Render(m.jitPage.View())
}

func (m Model) renderAutopoiesisView() string {
	return m.styles.Content.Render(m.autoPage.View())
}

func (m Model) renderShardView() string {
	return m.styles.Content.Render(m.shardPage.View())
}

func (m Model) renderChatView() string {
	// Header
	header := m.renderHeader()

	// Content area (chat viewport + optional error panel)
	content := m.viewport.View()
	if m.err != nil && m.showError {
		content = lipgloss.JoinVertical(lipgloss.Left, content, m.renderErrorPanel())
	}
	chatView := m.styles.Content.Render(content)

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
		BorderForeground(m.styles.Theme.Secondary).
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

// renderActivityLine returns a single dimmed line showing the most
// recent Glass Box event, or "" if there's nothing to show or Glass
// Box is disabled. Designed to live just above the input area as
// transient chrome — replaced on every new event, cleared on turn
// end.
func (m Model) renderActivityLine() string {
	if !m.glassBoxEnabled || strings.TrimSpace(m.activityLine) == "" {
		return ""
	}
	icon := glassBoxIcon(transparency.GlassBoxCategory(m.activityIconCh))
	age := ""
	if !m.activityAt.IsZero() {
		d := time.Since(m.activityAt)
		switch {
		case d < time.Second:
			age = "now"
		case d < time.Minute:
			age = fmt.Sprintf("%ds", int(d.Seconds()))
		default:
			age = fmt.Sprintf("%dm", int(d.Minutes()))
		}
	}
	text := fmt.Sprintf("  %s  %s", icon, m.activityLine)
	if age != "" {
		text = fmt.Sprintf("%s  · %s ago", text, age)
	}
	return m.styles.Muted.Italic(true).Render(text)
}
