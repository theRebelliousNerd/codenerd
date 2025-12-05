// Package main provides the codeNERD CLI entry point.
// This file contains view rendering functions for the TUI.
package main

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

func (m chatModel) renderHistory() string {
	var sb strings.Builder

	for _, msg := range m.history {
		if msg.role == "user" {
			// Render user message
			userStyle := m.styles.Bold.
				Foreground(m.styles.Theme.Primary).
				MarginTop(1)
			sb.WriteString(userStyle.Render("You") + "\n")
			sb.WriteString(m.styles.UserInput.Render(msg.content))
			sb.WriteString("\n\n")
		} else {
			// Render assistant message with markdown
			assistantStyle := m.styles.Bold.
				Foreground(m.styles.Theme.Accent).
				MarginTop(1)
			sb.WriteString(assistantStyle.Render("🧠 codeNERD") + "\n")

			// Render markdown with panic recovery
			rendered := m.safeRenderMarkdown(msg.content)
			sb.WriteString(rendered)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// safeRenderMarkdown renders markdown with panic recovery
func (m chatModel) safeRenderMarkdown(content string) (result string) {
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

func (m chatModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Header
	header := m.renderHeader()

	// Chat viewport
	chatView := m.styles.Content.Render(m.viewport.View())

	// Loading indicator
	if m.isLoading {
		chatView += "\n" + m.styles.Spinner.Render(m.spinner.View()) + " Thinking..."
	}

	// Error display
	if m.err != nil {
		chatView += "\n" + m.styles.Error.Render("Error: "+m.err.Error())
	}

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

	inputArea := inputStyle.Render(m.textinput.View())

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

func (m chatModel) renderHeader() string {
	// Logo and title
	title := m.styles.Header.Render(" 🧠 codeNERD ")
	version := m.styles.Badge.Render("v1.0")
	workspace := m.styles.Muted.Render(fmt.Sprintf(" 📁 %s", m.workspace))

	// Status indicators
	var status string
	if m.isLoading {
		status = m.styles.Warning.Render("● Processing")
	} else {
		status = m.styles.Success.Render("● Ready")
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

func (m chatModel) renderFooter() string {
	// Build mode indicator
	modeIndicator := ""
	switch m.paneMode {
	case ui.ModeSinglePane:
		modeIndicator = "📝 Chat"
	case ui.ModeSplitPane:
		modeIndicator = "🔬 Split (Chat + Logic)"
	case ui.ModeFullLogic:
		modeIndicator = "🔬 Logic View"
	}

	// Add campaign indicator if active
	campaignIndicator := ""
	if m.activeCampaign != nil {
		progress := 0.0
		if m.activeCampaign.TotalTasks > 0 {
			progress = float64(m.activeCampaign.CompletedTasks) / float64(m.activeCampaign.TotalTasks) * 100
		}
		campaignIndicator = fmt.Sprintf(" • 🎯 Campaign: %.0f%%", progress)
	}

	help := m.styles.Muted.Render(fmt.Sprintf("%s%s • Enter: send • Ctrl+L: logic • Ctrl+P: campaign • /help • Ctrl+C: exit", modeIndicator, campaignIndicator))
	return lipgloss.NewStyle().
		MarginTop(1).
		Render(help)
}

// runShadowSimulation runs a full Shadow Mode simulation
