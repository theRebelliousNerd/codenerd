package ui

import (
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/types"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
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

// ShardPageModel defines the state of the Shard Console.
type ShardPageModel struct {
	width  int
	height int
	table  table.Model

	// Data
	activeShards   []types.ShardAgent
	filteredShards []types.ShardAgent // Shards after filtering
	backpressure   *coreshards.BackpressureStatus

	// Filter state
	filterInput   textinput.Model
	filterMode    ShardFilterMode
	filterFocused bool // Whether filter input is focused

	// Styles
	styles Styles
}

// NewShardPageModel creates a new shard console.
// TODO: Allow dynamic column configuration
func NewShardPageModel() ShardPageModel {
	t := table.New(
		table.WithColumns([]table.Column{
			{Title: "ID", Width: 30},
			{Title: "Type", Width: 15},
			{Title: "Status", Width: 15},
		}),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	// Initialize filter input
	fi := textinput.New()
	fi.Placeholder = "Filter by ID or type..."
	fi.CharLimit = 50
	fi.Width = 40

	return ShardPageModel{
		table:          t,
		filterInput:    fi,
		filterMode:     FilterModeAll,
		filterFocused:  false,
		filteredShards: make([]types.ShardAgent, 0),
		styles:         DefaultStyles(),
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
			// Cycle through filter modes
			if !m.filterFocused {
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
	} else {
		// Update table when not filtering
		m.table, cmd = m.table.Update(msg)
		cmds = append(cmds, cmd)
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
			if state != types.ShardStateActive && state != types.ShardStateExecuting {
				continue
			}
		case FilterModeIdle:
			if state != types.ShardStateIdle && state != types.ShardStateReady {
				continue
			}
		case FilterModeFailed:
			if state != types.ShardStateFailed && state != types.ShardStateError {
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
		cfg := s.GetConfig()
		state := s.GetState()
		id := s.GetID()

		rows = append(rows, table.Row{
			id,
			string(cfg.Type),
			string(state),
		})
	}
	m.table.SetRows(rows)
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

// View renders the page.
// TODO: Add detailed view for selected shard
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

	// Table
	sb.WriteString(m.styles.Content.Render(m.table.View()))

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
		BorderForeground(m.styles.Theme.Border).
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
	hint := m.styles.Muted.Render("[/] Filter  [Tab] Mode")
	sb.WriteString("  ")
	sb.WriteString(hint)

	return sb.String()
}

// SetSize updates the size.
func (m *ShardPageModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.table.SetWidth(w - 4)
	m.table.SetHeight(h - 6)
}

// UpdateContent updates the data.
func (m *ShardPageModel) UpdateContent(shards []types.ShardAgent, bp *coreshards.BackpressureStatus) {
	m.activeShards = shards
	m.backpressure = bp

	// Apply current filter to new data
	m.applyFilter()
}
