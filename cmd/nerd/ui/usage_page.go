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
// TODO: IMPROVEMENT: Implement responsive layout for small screens (e.g., stack tables vertically).
func (m *UsagePageModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.viewport.Width = w
	m.viewport.Height = h - 4 // Reserve space for header/footer
	m.UpdateContent()
}

// UpdateContent refreshes the viewport content from the tracker data.
// TODO: IMPROVEMENT: Add visual charts (bar/pie) using termui or similar for better data visualization.
func (m *UsagePageModel) UpdateContent() {
	if m.tracker == nil {
		m.viewport.SetContent("Usage tracking not available.")
		return
	}

	stats := m.tracker.Stats()

	var sb strings.Builder

	// Title
	sb.WriteString(m.styles.Layout.Header.Render("Token Usage Statistics"))
	sb.WriteString("\n\n")

	// Total Project Usage
	total := stats.TotalProject
	sb.WriteString(fmt.Sprintf("Total Input:  %d\n", total.Input))
	sb.WriteString(fmt.Sprintf("Total Output: %d\n", total.Output))
	sb.WriteString(fmt.Sprintf("Grand Total:  %d\n", total.Total))
	sb.WriteString(fmt.Sprintf("Est. Cost:    %s\n", formatCost(total.Cost)))
	// A small cost total is ambiguous unless we say how much spend was on
	// models we have no price for.
	if stats.UnpricedTokens > 0 {
		sb.WriteString(fmt.Sprintf("              (%d tokens on unpriced models are excluded)\n", stats.UnpricedTokens))
	}
	sb.WriteString("\n")

	// Helper to render map tables, ordered by spend so the expensive rows are
	// visible without scrolling.
	renderTable := func(title string, data map[string]usage.TokenCounts) {
		if len(data) == 0 {
			return
		}

		keys := make([]string, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if data[keys[i]].Total != data[keys[j]].Total {
				return data[keys[i]].Total > data[keys[j]].Total
			}
			return keys[i] < keys[j]
		})

		t := NewSimpleTable(title, []string{"Name", "Input", "Output", "Total", "Cost"})
		for _, k := range keys {
			c := data[k]
			t.AddRow(
				truncate(k, 20),
				fmt.Sprintf("%d", c.Input),
				fmt.Sprintf("%d", c.Output),
				fmt.Sprintf("%d", c.Total),
				formatCost(c.Cost),
			)
		}
		sb.WriteString(t.View(m.styles))
	}

	renderTable("By Provider", stats.ByProvider)
	renderTable("By Model", stats.ByModel)
	renderTable("By Shard Type", stats.ByShardType)
	renderTable("By Shard Name", stats.ByShardName)
	renderTable("By Operation", stats.ByOperation)
	renderTable("By Session", stats.BySession)

	m.viewport.SetContent(sb.String())
}

// formatCost renders an estimated USD cost. Sub-cent amounts get four decimals
// so a cheap-but-nonzero row does not read as free.
func formatCost(cost float64) string {
	switch {
	case cost == 0:
		return "—"
	case cost < 0.01:
		return fmt.Sprintf("$%.4f", cost)
	default:
		return fmt.Sprintf("$%.2f", cost)
	}
}

func truncate(s string, l int) string {
	if len(s) > l {
		return s[:l-3] + "..."
	}
	return s
}

// Update handles messages.
// TODO: IMPROVEMENT: Add real-time updates via a subscription model if the tracker supports it.
// TODO: Add date range filter for usage stats.
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
// TODO: IMPROVEMENT: Support exporting usage stats (CSV/JSON).
func (m UsagePageModel) View() string {
	return m.viewport.View()
}
