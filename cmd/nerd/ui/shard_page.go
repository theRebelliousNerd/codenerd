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
// TODO: Add filter input to search/filter shards by ID or status.
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

	return ShardPageModel{
		table:  t,
		styles: DefaultStyles(),
	}
}

// Init initializes the model.
func (m ShardPageModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
// TODO: IMPROVEMENT: Implement auto-refresh mechanism for shard status.
// TODO: Add command to manually restart a shard.
func (m ShardPageModel) Update(msg tea.Msg) (ShardPageModel, tea.Cmd) {
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// View renders the page.
// TODO: Add detailed view showing last N log lines for the selected shard.
// TODO: IMPROVEMENT: Add visual indicators (colors/icons) for different shard statuses.
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

	sb.WriteString(m.styles.Content.Render(m.table.View()))
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

	var rows []table.Row
	for _, s := range shards {
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
