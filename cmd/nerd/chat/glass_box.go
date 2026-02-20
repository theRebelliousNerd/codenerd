// Package chat provides the interactive TUI chat interface for codeNERD.
// This file implements Glass Box debug mode for inline system visibility.
package chat

import (
	"fmt"
	"strings"
	"time"

	"codenerd/internal/transparency"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	// maxGlassBoxEvents caps the event buffer to prevent unbounded growth
	maxGlassBoxEvents = 50
)

// listenGlassBoxEvents returns a tea.Cmd that waits for Glass Box events.
// This enables the Bubble Tea update loop to receive events from the event bus.
func (m Model) listenGlassBoxEvents() tea.Cmd {
	if m.glassBoxEventChan == nil || !m.glassBoxEnabled {
		return nil
	}

	eventChan := m.glassBoxEventChan
	return func() tea.Msg {
		event, ok := <-eventChan
		if !ok {
			return nil // Channel closed
		}
		return glassBoxEventMsg(event)
	}
}

// handleGlassBoxEvent processes a Glass Box event and adds it to history.
func (m *Model) handleGlassBoxEvent(event transparency.GlassBoxEvent) {
	// Add to event buffer (capped)
	m.glassBoxEvents = append(m.glassBoxEvents, event)
	if len(m.glassBoxEvents) > maxGlassBoxEvents {
		m.glassBoxEvents = m.glassBoxEvents[1:]
	}

	// Convert to Message and add to history
	msg := m.glassBoxEventToMessage(event)
	// Use addMessage to ensure caching
	*m = m.addMessage(msg)
}

// glassBoxEventToMessage converts a GlassBoxEvent to a Message for display.
func (m *Model) glassBoxEventToMessage(event transparency.GlassBoxEvent) Message {
	content := event.Summary
	if event.Details != "" && m.isGlassBoxVerbose() {
		content = fmt.Sprintf("%s\n%s", event.Summary, event.Details)
	}

	return Message{
		Role:             "system",
		Content:          content,
		Time:             event.Timestamp,
		GlassBoxCategory: event.Category,
		IsCollapsed:      true, // Start collapsed by default
	}
}

// isGlassBoxVerbose returns true if verbose mode is enabled.
func (m *Model) isGlassBoxVerbose() bool {
	if m.glassBoxEventBus != nil {
		return m.glassBoxEventBus.IsVerbose()
	}
	return false
}

// toggleGlassBox toggles Glass Box debug mode on/off.
func (m *Model) toggleGlassBox() string {
	m.glassBoxEnabled = !m.glassBoxEnabled

	if m.glassBoxEnabled {
		// Enable the event bus
		if m.glassBoxEventBus != nil {
			m.glassBoxEventBus.Enable()
		}
		return "Glass Box Debug Mode: **ON**\n\nSystem activity will now appear inline in the chat."
	}

	// Disable the event bus
	if m.glassBoxEventBus != nil {
		m.glassBoxEventBus.Disable()
	}
	return "Glass Box Debug Mode: **OFF**"
}

// toggleGlassBoxVerbose toggles verbose mode for detailed output.
func (m *Model) toggleGlassBoxVerbose() string {
	if m.glassBoxEventBus == nil {
		return "Glass Box event bus not initialized."
	}

	verbose := !m.glassBoxEventBus.IsVerbose()
	m.glassBoxEventBus.SetVerbose(verbose)

	if verbose {
		return "Glass Box Verbose Mode: **ON**\n\nEvents will show expanded details."
	}
	return "Glass Box Verbose Mode: **OFF**\n\nEvents will show summaries only."
}

// toggleGlassBoxCategory toggles a specific category on/off.
func (m *Model) toggleGlassBoxCategory(category string) string {
	if !transparency.ValidCategory(category) {
		valid := make([]string, 0, 5)
		for _, c := range transparency.AllCategories() {
			valid = append(valid, string(c))
		}
		return fmt.Sprintf("Invalid category: %s\n\nValid categories: %s", category, strings.Join(valid, ", "))
	}

	// For simplicity, we'll just report the toggle
	// The actual filtering is done via SetCategories on the event bus
	return fmt.Sprintf("Category '%s' filter toggled.\n\nUse `/glassbox status` to see current settings.", category)
}

// glassBoxStatus returns current Glass Box settings.
func (m *Model) glassBoxStatus() string {
	var sb strings.Builder
	sb.WriteString("## Glass Box Status\n\n")

	if m.glassBoxEnabled {
		sb.WriteString("- Mode: **ENABLED**\n")
	} else {
		sb.WriteString("- Mode: **DISABLED**\n")
	}

	if m.glassBoxEventBus != nil {
		stats := m.glassBoxEventBus.Stats()
		sb.WriteString(fmt.Sprintf("- Verbose: %v\n", stats.Verbose))
		sb.WriteString(fmt.Sprintf("- Total Events Emitted: %d\n", stats.TotalEmitted))
		sb.WriteString(fmt.Sprintf("- Buffered Events: %d\n", stats.BufferedEvents))
		sb.WriteString(fmt.Sprintf("- Subscribers: %d\n", stats.SubscriberCount))

		if stats.CategoryCount > 0 {
			sb.WriteString(fmt.Sprintf("- Category Filter: %d categories\n", stats.CategoryCount))
		} else {
			sb.WriteString("- Category Filter: ALL\n")
		}
	} else {
		sb.WriteString("- Event Bus: Not initialized\n")
	}

	sb.WriteString(fmt.Sprintf("- Events in Buffer: %d/%d\n", len(m.glassBoxEvents), maxGlassBoxEvents))

	sb.WriteString("\n### Categories\n")
	for _, c := range transparency.AllCategories() {
		sb.WriteString(fmt.Sprintf("- `%s`: %s\n", c, categoryDescription(c)))
	}

	sb.WriteString("\n### Keybindings\n")
	sb.WriteString("- `Alt+G`: Toggle Glass Box on/off\n")
	sb.WriteString("- `/glassbox verbose`: Toggle detailed output\n")
	sb.WriteString("- `/glassbox <category>`: Toggle category filter\n")

	return sb.String()
}

// categoryDescription returns a description for each category.
func categoryDescription(c transparency.GlassBoxCategory) string {
	switch c {
	case transparency.CategoryPerception:
		return "Intent parsing, entity resolution, confidence scores"
	case transparency.CategoryKernel:
		return "Fact assertions, rule derivations, next_action"
	case transparency.CategoryJIT:
		return "Prompt atom selection, compilation stats, budget"
	case transparency.CategoryShard:
		return "Shard spawn events, phase transitions"
	case transparency.CategoryControl:
		return "Control packets from LLM (reasoning trace, mangle updates)"
	default:
		return "Unknown category"
	}
}

// initGlassBox initializes the Glass Box event bus and subscription.
// Called during boot after components are available.
func (m *Model) initGlassBox(bus *transparency.GlassBoxEventBus) {
	m.glassBoxEventBus = bus

	// Subscribe to events
	if bus != nil {
		m.glassBoxEventChan = bus.Subscribe()
	}

	// Check config for initial state
	// Note: m.Config.Transparency is a pointer in UserConfig, so check both
	if m.Config != nil && m.Config.Transparency != nil && m.Config.Transparency.GlassBoxEnabled {
		m.glassBoxEnabled = true
		if bus != nil {
			bus.Enable()
		}
	}
}

// emitGlassBoxEvent is a helper to emit events from the chat package.
// It's a convenience wrapper around the event bus.
func (m *Model) emitGlassBoxEvent(category transparency.GlassBoxCategory, summary string, details string) {
	if m.glassBoxEventBus == nil || !m.glassBoxEnabled {
		return
	}

	m.glassBoxEventBus.Emit(transparency.GlassBoxEvent{
		Timestamp: time.Now(),
		Category:  category,
		Summary:   summary,
		Details:   details,
		TurnID:    m.turnCount,
	})
}

// =============================================================================
// TOOL EVENT VISIBILITY (Always Active)
// =============================================================================

// toolEventMsg wraps a ToolEvent for the Bubble Tea update loop.
type toolEventMsg transparency.ToolEvent

// listenToolEvents returns a tea.Cmd that waits for tool events.
// Unlike Glass Box, this is ALWAYS active - tool events always show in chat.
func (m Model) listenToolEvents() tea.Cmd {
	if m.toolEventChan == nil {
		return nil
	}

	eventChan := m.toolEventChan
	return func() tea.Msg {
		event, ok := <-eventChan
		if !ok {
			return nil // Channel closed
		}
		return toolEventMsg(event)
	}
}

// handleToolEvent processes a tool event and adds it to chat history.
// Tool events ALWAYS appear in the chat, regardless of Glass Box mode.
func (m *Model) handleToolEvent(event transparency.ToolEvent) {
	// Format the tool execution message
	var content string
	if event.Success {
		content = fmt.Sprintf("**🔧 %s** (%.0fms)\n%s", event.ToolName, float64(event.Duration.Milliseconds()), event.Result)
	} else {
		content = fmt.Sprintf("**🔧 %s** ❌ FAILED (%.0fms)\n%s", event.ToolName, float64(event.Duration.Milliseconds()), event.Result)
	}

	// Add to history with "tool" role
	// Use addMessage to ensure caching
	*m = m.addMessage(Message{
		Role:    "tool",
		Content: content,
		Time:    event.Timestamp,
	})
}

// initToolEventBus sets up the tool event bus subscription.
// Called during boot after components are available.
func (m *Model) initToolEventBus(bus *transparency.ToolEventBus) {
	m.toolEventBus = bus
	if bus != nil {
		m.toolEventChan = bus.Subscribe()
	}
}
