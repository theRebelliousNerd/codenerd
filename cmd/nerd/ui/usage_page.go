package ui

import (
	"codenerd/internal/usage"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// UsagePageModel handles the rendering of the token usage statistics.
type UsagePageModel struct {
	viewport viewport.Model
	tracker  *usage.Tracker
	styles   Styles
	width    int
	height   int
}

// NewUsagePageModel creates a new usage page component.
func NewUsagePageModel(tracker *usage.Tracker, styles Styles) UsagePageModel {
	vp := viewport.New(80, 20)
	return UsagePageModel{
		viewport: vp,
		tracker:  tracker,
		styles:   styles,
	}
}

// SetSize updates the size of the viewport.
func (m *UsagePageModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.viewport.Width = w
	m.viewport.Height = h - 4 // Reserve space for header/footer
	m.UpdateContent()
}

// UpdateContent refreshes the viewport content from the tracker data.
func (m *UsagePageModel) UpdateContent() {
	if m.tracker == nil {
		m.viewport.SetContent("Usage tracking not available.")
		return
	}

	stats := m.tracker.Stats()

	var sb strings.Builder

	// Title
	sb.WriteString(m.styles.Header.Render("Token Usage Statistics"))
	sb.WriteString("\n\n")

	// Total Project Usage
	total := stats.TotalProject
	sb.WriteString(fmt.Sprintf("Total Input:  %d\n", total.Input))
	sb.WriteString(fmt.Sprintf("Total Output: %d\n", total.Output))
	sb.WriteString(fmt.Sprintf("Grand Total:  %d\n", total.Total))
	sb.WriteString("\n")

	// Helper to render map tables
	// TODO: Move table rendering to a reusable component or bubble
	renderTable := func(title string, data map[string]usage.TokenCounts) {
		if len(data) == 0 {
			return
		}
		sb.WriteString(m.styles.Title.Render(title))
		sb.WriteString("\n")

		// Sort keys
		keys := make([]string, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// Header
		// Name | Input | Output | Total
		sb.WriteString(fmt.Sprintf("%-20s | %-10s | %-10s | %-10s\n", "Name", "Input", "Output", "Total"))
		sb.WriteString(strings.Repeat("-", 60) + "\n")

		for _, k := range keys {
			c := data[k]
			sb.WriteString(fmt.Sprintf("%-20s | %-10d | %-10d | %-10d\n", truncate(k, 20), c.Input, c.Output, c.Total))
		}
		sb.WriteString("\n")
	}

	renderTable("By Provider", stats.ByProvider)
	renderTable("By Model", stats.ByModel)
	renderTable("By Shard Type", stats.ByShardType)
	renderTable("By Operation", stats.ByOperation)

	m.viewport.SetContent(sb.String())
}

func truncate(s string, l int) string {
	if len(s) > l {
		return s[:l-3] + "..."
	}
	return s
}

// Update handles messages.
func (m UsagePageModel) Update(msg tea.Msg) (UsagePageModel, tea.Cmd) {
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)

	// Refresh content on tick if needed, or if triggered specifically
	// For now, let's refresh on every keypress just in case, or add a specific message event
	// But actually, we probably only need to refresh when entering view or periodic tick.
	// Let's assume UpdateContent is called manually when entering.

	return m, cmd
}

// View renders the page.
// TODO: Add support for exporting usage stats (CSV/JSON)
func (m UsagePageModel) View() string {
	return m.viewport.View()
}
