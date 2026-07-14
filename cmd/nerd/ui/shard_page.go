package ui

import (
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/types"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ShardFilterMode represents the current filter mode
type ShardFilterMode int

const (
	FilterModeAll ShardFilterMode = iota
	FilterModeActive
	FilterModeIdle
	FilterModeFailed
)

// ShardColumnDef defines a dynamic column for the shard table.
type ShardColumnDef struct {
	Title string
	Width int
	Value func(types.ShardAgent) string
}

// DefaultShardColumns returns the default columns for the shard table.
func DefaultShardColumns() []ShardColumnDef {
	return []ShardColumnDef{
		{Title: "ID", Width: 30, Value: func(s types.ShardAgent) string { return s.GetID() }},
		{Title: "Type", Width: 15, Value: func(s types.ShardAgent) string { return string(s.GetConfig().Type) }},
		{Title: "Status", Width: 15, Value: func(s types.ShardAgent) string { return string(s.GetState()) }},
	}
}

// ShardPageModel defines the state of the Shard Console.
type ShardPageModel struct {
	width           int
	height          int
	table           table.Model
	detailsViewport viewport.Model

	// Data
	activeShards   []types.ShardAgent
	filteredShards []types.ShardAgent // Shards after filtering
	backpressure   *coreshards.BackpressureStatus

	// Filter state
	filterInput    textinput.Model
	filterMode     ShardFilterMode
	filterFocused  bool // Whether filter input is focused
	detailsFocused bool // Whether details viewport is focused
	columns        []ShardColumnDef

	// Styles
	styles Styles
}

// NewShardPageModel creates a new shard console.
func NewShardPageModel(columns ...ShardColumnDef) ShardPageModel {
	cols := columns
	if len(cols) == 0 {
		cols = DefaultShardColumns()
	}

	tableCols := make([]table.Column, len(cols))
	for i, c := range cols {
		tableCols[i] = table.Column{Title: c.Title, Width: c.Width}
	}

	t := table.New(
		table.WithColumns(tableCols),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	// Initialize filter input
	fi := textinput.New()
	fi.Placeholder = "Filter by ID or type..."
	fi.CharLimit = 50
	fi.Width = 40

	// Initialize details viewport
	vp := viewport.New(0, 0)
	vp.SetContent("Select a shard to see details")

	return ShardPageModel{
		table:           t,
		detailsViewport: vp,
		filterInput:     fi,
		filterMode:      FilterModeAll,
		filterFocused:   false,
		filteredShards:  make([]types.ShardAgent, 0),
		styles:          DefaultStyles(),
		columns:         cols,
	}
}

// Init initializes the model.
func (m ShardPageModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m ShardPageModel) Update(msg tea.Msg) (ShardPageModel, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "/":
			// Toggle filter input focus
			m.filterFocused = !m.filterFocused
			if m.filterFocused {
				m.filterInput.Focus()
			} else {
				m.filterInput.Blur()
			}
			return m, nil
		case "tab":
			// Cycle through filter modes OR focus
			if !m.filterFocused {
				m.detailsFocused = !m.detailsFocused
				if m.detailsFocused {
					m.table.Blur()
				} else {
					m.table.Focus()
				}
			}
		case "m":
			// Cycle through filter modes
			if !m.filterFocused && !m.detailsFocused {
				m.filterMode = (m.filterMode + 1) % 4
				m.applyFilter()
			}
		case "esc":
			// Clear filter and unfocus
			if m.filterFocused {
				m.filterFocused = false
				m.filterInput.Blur()
				return m, nil
			}
		case "enter":
			// Apply filter and unfocus
			if m.filterFocused {
				m.filterFocused = false
				m.filterInput.Blur()
				m.applyFilter()
				return m, nil
			}
		}
	}

	// Update filter input if focused
	if m.filterFocused {
		m.filterInput, cmd = m.filterInput.Update(msg)
		cmds = append(cmds, cmd)
		// Apply filter on each keystroke for live filtering
		m.applyFilter()
	} else if m.detailsFocused {
		m.detailsViewport, cmd = m.detailsViewport.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		// Update table when not filtering or in details
		oldCursor := m.table.Cursor()
		m.table, cmd = m.table.Update(msg)
		cmds = append(cmds, cmd)

		// Update details if cursor moved
		if m.table.Cursor() != oldCursor {
			m.updateDetails()
		}
	}

	return m, tea.Batch(cmds...)
}

// applyFilter filters the shards based on current filter text and mode
func (m *ShardPageModel) applyFilter() {
	filterText := strings.ToLower(m.filterInput.Value())

	m.filteredShards = make([]types.ShardAgent, 0, len(m.activeShards))

	for _, s := range m.activeShards {
		// Apply status filter
		state := s.GetState()
		switch m.filterMode {
		case FilterModeActive:
			if state != types.ShardStateRunning {
				continue
			}
		case FilterModeIdle:
			// Completed shards are effectively idle from a scheduler POV.
			if state != types.ShardStateIdle && state != types.ShardStateCompleted {
				continue
			}
		case FilterModeFailed:
			if state != types.ShardStateFailed {
				continue
			}
		}

		// Apply text filter
		if filterText != "" {
			id := strings.ToLower(s.GetID())
			cfg := s.GetConfig()
			shardType := strings.ToLower(string(cfg.Type))

			if !strings.Contains(id, filterText) && !strings.Contains(shardType, filterText) {
				continue
			}
		}

		m.filteredShards = append(m.filteredShards, s)
	}

	// Update table rows
	m.updateTableRows()
}

// updateTableRows updates the table with filtered shards
func (m *ShardPageModel) updateTableRows() {
	var rows []table.Row
	for _, s := range m.filteredShards {
		row := make(table.Row, len(m.columns))
		for i, col := range m.columns {
			row[i] = col.Value(s)
		}
		rows = append(rows, row)
	}
	m.table.SetRows(rows)
	m.updateDetails()
}

// ClearFilter clears the filter text and resets to show all shards
func (m *ShardPageModel) ClearFilter() {
	m.filterInput.SetValue("")
	m.filterMode = FilterModeAll
	m.applyFilter()
}

// SetFilterMode sets the filter mode directly
func (m *ShardPageModel) SetFilterMode(mode ShardFilterMode) {
	m.filterMode = mode
	m.applyFilter()
}

// updateDetails updates the details viewport based on selection
func (m *ShardPageModel) updateDetails() {
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.filteredShards) {
		m.detailsViewport.SetContent("No shard selected")
		return
	}

	shard := m.filteredShards[idx]
	m.detailsViewport.SetContent(m.renderShardDetails(shard))
}

// renderShardDetails renders detailed information for a shard
func (m *ShardPageModel) renderShardDetails(s types.ShardAgent) string {
	cfg := s.GetConfig()
	state := s.GetState()

	var sb strings.Builder

	// Header
	sb.WriteString(m.styles.Bold.Render("Shard ID: ") + s.GetID() + "\n")
	sb.WriteString(m.styles.Bold.Render("Type:     ") + string(cfg.Type) + "\n")
	sb.WriteString(m.styles.Bold.Render("Status:   ") + string(state) + "\n\n")

	// Model Info
	sb.WriteString(m.styles.Header.Render(" Model Config ") + "\n")
	sb.WriteString(fmt.Sprintf("Name:       %s\n", cfg.Model.Name))
	sb.WriteString(fmt.Sprintf("Capability: %s\n\n", cfg.Model.Capability))

	// Permissions
	sb.WriteString(m.styles.Header.Render(" Permissions ") + "\n")
	if len(cfg.Permissions) == 0 {
		sb.WriteString("None\n")
	} else {
		for _, p := range cfg.Permissions {
			sb.WriteString(fmt.Sprintf("• %s\n", p))
		}
	}
	sb.WriteString("\n")

	// Tools
	sb.WriteString(m.styles.Header.Render(" Tools ") + "\n")
	if len(cfg.Tools) == 0 {
		sb.WriteString("None\n")
	} else {
		for _, t := range cfg.Tools {
			sb.WriteString(fmt.Sprintf("• %s\n", t))
		}
	}

	return sb.String()
}

// View renders the page.
func (m ShardPageModel) View() string {
	var sb strings.Builder

	// Header / Queue Status
	title := m.styles.Header.Render(" Active Shards ")
	sb.WriteString(title + "\n\n")

	if m.backpressure != nil {
		stats := fmt.Sprintf("Queue: %d pending | Slots Available: %d",
			m.backpressure.QueueDepth,
			m.backpressure.AvailableSlots,
		)
		sb.WriteString(m.styles.Info.Render(stats) + "\n\n")
	}

	// Filter bar
	sb.WriteString(m.renderFilterBar())
	sb.WriteString("\n\n")

	// Main content: Table + Details
	tableBorder := lipgloss.NormalBorder()
	if !m.detailsFocused && !m.filterFocused {
		tableBorder = lipgloss.ThickBorder()
	}
	tableView := lipgloss.NewStyle().
		Border(tableBorder).
		BorderForeground(func() lipgloss.Color {
			if !m.detailsFocused && !m.filterFocused {
				return m.styles.Theme.Primary
			}
			return m.styles.Theme.Outline
		}()).
		Render(m.table.View())

	detailsBorder := lipgloss.NormalBorder()
	if m.detailsFocused {
		detailsBorder = lipgloss.ThickBorder()
	}
	detailsView := lipgloss.NewStyle().
		Border(detailsBorder).
		BorderForeground(func() lipgloss.Color {
			if m.detailsFocused {
				return m.styles.Theme.Primary
			}
			return m.styles.Theme.Outline
		}()).
		Width(m.detailsViewport.Width).
		Height(m.detailsViewport.Height).
		Render(m.detailsViewport.View())

	content := lipgloss.JoinHorizontal(lipgloss.Top, tableView, " ", detailsView)
	sb.WriteString(content)

	// Filter count
	if len(m.filteredShards) != len(m.activeShards) {
		countInfo := fmt.Sprintf("\nShowing %d of %d shards", len(m.filteredShards), len(m.activeShards))
		sb.WriteString(m.styles.Muted.Render(countInfo))
	}

	return sb.String()
}

// renderFilterBar renders the filter input and mode selector
func (m ShardPageModel) renderFilterBar() string {
	var sb strings.Builder

	// Filter input
	filterStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.Theme.Outline).
		Padding(0, 1)

	if m.filterFocused {
		filterStyle = filterStyle.BorderForeground(m.styles.Theme.Primary)
	}

	sb.WriteString(filterStyle.Render(m.filterInput.View()))
	sb.WriteString("  ")

	// Filter mode tabs
	modes := []struct {
		mode  ShardFilterMode
		label string
	}{
		{FilterModeAll, "All"},
		{FilterModeActive, "Active"},
		{FilterModeIdle, "Idle"},
		{FilterModeFailed, "Failed"},
	}

	for _, mode := range modes {
		style := m.styles.Muted
		if m.filterMode == mode.mode {
			style = lipgloss.NewStyle().
				Foreground(m.styles.Theme.Primary).
				Bold(true).
				Underline(true)
		}
		sb.WriteString(style.Render(mode.label))
		sb.WriteString("  ")
	}

	// Help hint
	hint := m.styles.Muted.Render("[/] Filter  [Tab] Focus  [m] Mode")
	sb.WriteString("  ")
	sb.WriteString(hint)

	return sb.String()
}

// SetSize updates the size.
func (m *ShardPageModel) SetSize(w, h int) {
	m.width = w
	m.height = h

	// Calculate widths for split view
	leftWidth := int(float64(w) * SplitPaneLeftRatio)
	rightWidth := w - leftWidth - SplitPaneDivider

	m.table.SetWidth(leftWidth - 2)
	m.table.SetHeight(h - 12)

	m.detailsViewport.Width = rightWidth - 2
	m.detailsViewport.Height = h - 12
}

// UpdateContent updates the data.
func (m *ShardPageModel) UpdateContent(shards []types.ShardAgent, bp *coreshards.BackpressureStatus) {
	m.activeShards = shards
	m.backpressure = bp

	// Apply current filter to new data
	m.applyFilter()
}
