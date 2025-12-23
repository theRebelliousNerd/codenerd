// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains view rendering functions for the TUI.
package chat

import (
	"codenerd/cmd/nerd/ui"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// =============================================================================
// VIEW RENDERING
// =============================================================================
// These functions render the TUI components: history, header, footer, etc.

func (m Model) renderHistory() string {
	var sb strings.Builder

	for _, msg := range m.history {
		switch msg.Role {
		case "user":
			// Render user message
			userStyle := m.styles.Bold.
				Foreground(m.styles.Theme.Primary).
				MarginTop(1)
			sb.WriteString(userStyle.Render("You") + "\n")
			sb.WriteString(m.styles.UserInput.Render(msg.Content))
			sb.WriteString("\n\n")

		case "system":
			// Render Glass Box system event (only when enabled)
			if !m.glassBoxEnabled {
				continue
			}
			sb.WriteString(m.renderGlassBoxMessage(msg))

		default: // "assistant"
			// Render assistant message with markdown
			assistantStyle := m.styles.Bold.
				Foreground(m.styles.Theme.Accent).
				MarginTop(1)
			sb.WriteString(assistantStyle.Render("codeNERD") + "\n")

			// Render markdown with panic recovery
			rendered := m.safeRenderMarkdown(msg.Content)
			sb.WriteString(rendered)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// renderGlassBoxMessage formats a Glass Box system event for display.
func (m Model) renderGlassBoxMessage(msg Message) string {
	// Category prefix with color
	prefix := m.styles.Muted.Render(msg.GlassBoxCategory.DisplayPrefix())

	// Content with dimmed styling
	content := m.styles.Muted.Render(msg.Content)

	// Collapsible indicator if message has details (check for newline)
	indicator := ""
	if strings.Contains(msg.Content, "\n") {
		if msg.IsCollapsed {
			indicator = m.styles.Muted.Render(" [+]")
		} else {
			indicator = m.styles.Muted.Render(" [-]")
		}
	}

	return fmt.Sprintf("  %s%s %s\n", prefix, indicator, content)
}

// safeRenderMarkdown renders markdown with panic recovery
func (m Model) safeRenderMarkdown(content string) (result string) {
	defer func() {
		if r := recover(); r != nil {
			// If glamour panics, return plain text
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

	// Handle List View Mode
	if m.viewMode == ListView {
		return m.styles.Content.Render(m.list.View())
	}

	// Handle File Picker Mode
	if m.viewMode == FilePickerView {
		title := m.styles.Header.Render(" Select a file ")
		content := m.styles.Content.Render(m.filepicker.View())
		return lipgloss.JoinVertical(lipgloss.Left, title, content)
	}

	// Handle Usage View Mode
	if m.viewMode == UsageView {
		return m.styles.Content.Render(m.usagePage.View())
	}

	// Handle Campaign Page Mode
	if m.viewMode == CampaignPage {
		return m.styles.Content.Render(m.campaignPage.View())
	}

	// Handle JIT Inspector Mode
	if m.viewMode == PromptInspector {
		return m.styles.Content.Render(m.jitPage.View())
	}

	// Handle Autopoiesis Page Mode
	if m.viewMode == AutopoiesisPage {
		return m.styles.Content.Render(m.autoPage.View())
	}

	// Handle Shard Console Mode
	if m.viewMode == ShardPage {
		return m.styles.Content.Render(m.shardPage.View())
	}

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
		BorderForeground(m.styles.Theme.Accent).
		Padding(0, 1)

	inputArea := inputStyle.Render(m.textarea.View())

	// Footer (with mode indicator)
	footer := m.renderFooter()

	// Compose full view
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		chatView,
		inputArea,
		footer,
	)
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
		Foreground(ui.Destructive).
		Render("Error") +
		m.styles.Muted.Render("  Alt+E: scroll  Alt+Shift+E: hide")

	panelStyle := lipgloss.NewStyle().
		Border(border).
		BorderForeground(ui.Destructive).
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

	return lipgloss.JoinVertical(
		lipgloss.Left,
		headerLine,
		workspace,
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
	return lipgloss.NewStyle().
		MarginTop(1).
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
