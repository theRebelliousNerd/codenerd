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
	// maxGlassBoxEvents caps the in-memory ring used by /glassbox status.
	// Chat scrollback keeps its own history; this is only the status buffer.
	maxGlassBoxEvents = 500

	// maxGlassBoxDrain is how many already-queued events we fold into a
	// single Bubble Tea frame so the viewport keeps up under burst load
	// (multi-shard spawn, tool storms) without dropping the listen loop.
	maxGlassBoxDrain = 64
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

// drainGlassBoxEvents folds any already-buffered events into the model so a
// busy turn renders as a stream of lines rather than one-per-frame lag.
// The first event is the one that woke the update loop; remaining are
// non-blocking receives capped at maxGlassBoxDrain.
func (m *Model) drainGlassBoxEvents(first transparency.GlassBoxEvent) {
	m.handleGlassBoxEvent(first)
	if m.glassBoxEventChan == nil {
		return
	}
	for i := 0; i < maxGlassBoxDrain; i++ {
		select {
		case event, ok := <-m.glassBoxEventChan:
			if !ok {
				return
			}
			m.handleGlassBoxEvent(event)
		default:
			return
		}
	}
}

// handleGlassBoxEvent processes a Glass Box event and adds it to history.
//
// Debug mode streams EVERYTHING into chat scrollback — perception, kernel
// routing, JIT stats, shard lifecycle, control packets, tool/routing
// results, and status pings. The live activity pulse (trail above input)
// also updates so the user always sees motion even when scrolled up.
func (m *Model) handleGlassBoxEvent(event transparency.GlassBoxEvent) {
	// Add to event buffer (capped) — used by /glassbox status etc.
	m.glassBoxEvents = append(m.glassBoxEvents, event)
	if len(m.glassBoxEvents) > maxGlassBoxEvents {
		m.glassBoxEvents = m.glassBoxEvents[1:]
	}

	// Live pulse: latest beat + short trail so the chrome never looks frozen.
	at := event.Timestamp
	if at.IsZero() {
		at = time.Now()
	}
	m.activityLine = event.Summary
	m.activityIconCh = string(event.Category)
	m.activityAt = at
	m.pushActivityPulse(activityPulse{
		Summary:  event.Summary,
		Category: event.Category,
		At:       at,
	})

	// Full stream: every event is a permanent chat line when Glass Box is on.
	if m.glassBoxEnabled {
		msg := m.glassBoxEventToMessage(event)
		*m = m.addMessage(msg)
	}
}

// pushActivityPulse prepends a beat to the live trail (newest first).
func (m *Model) pushActivityPulse(p activityPulse) {
	// Dedup identical consecutive summaries so the trail doesn't stutter.
	if len(m.activityTrail) > 0 && m.activityTrail[0].Summary == p.Summary {
		m.activityTrail[0].At = p.At
		m.activityTrail[0].Category = p.Category
		return
	}
	m.activityTrail = append([]activityPulse{p}, m.activityTrail...)
	if len(m.activityTrail) > maxActivityTrail {
		m.activityTrail = m.activityTrail[:maxActivityTrail]
	}
}

// beginLiveTurn marks the start of a working turn for the elapsed timer
// and seeds the activity trail with an immediate "working" beat.
func (m *Model) beginLiveTurn(label string) {
	m.turnStartedAt = time.Now()
	if strings.TrimSpace(label) == "" {
		label = "Working..."
	}
	m.statusMessage = label
	m.activityLine = label
	m.activityIconCh = string(transparency.CategoryControl)
	m.activityAt = m.turnStartedAt
	m.pushActivityPulse(activityPulse{
		Summary:  label,
		Category: transparency.CategoryControl,
		At:       m.turnStartedAt,
	})
}

// isMilestoneEvent is retained for tests/callers that want a "big events only"
// heuristic. Glass Box debug mode no longer uses it for scrollback gating —
// everything streams.
func isMilestoneEvent(e transparency.GlassBoxEvent) bool {
	switch e.Category {
	case transparency.CategoryShard, transparency.CategoryRouting:
		return true
	case transparency.CategoryKernel:
		// Only completed decisions, not raw fact pings.
		return e.Duration > 0 || strings.HasPrefix(e.Summary, "next_action") || strings.Contains(e.Summary, "denied")
	case transparency.CategoryControl:
		return true
	default:
		return e.Duration > 0
	}
}

// glassBoxEventToMessage converts a GlassBoxEvent to a Message for display.
func (m *Model) glassBoxEventToMessage(event transparency.GlassBoxEvent) Message {
	content := event.Summary
	if event.Source != "" && !strings.Contains(content, event.Source) {
		content = fmt.Sprintf("%s  · %s", content, event.Source)
	}
	if event.Duration > 0 {
		content = fmt.Sprintf("%s  (%.0fms)", content, float64(event.Duration.Milliseconds()))
	}
	verbose := m.isGlassBoxVerbose()
	if event.Details != "" && verbose {
		content = fmt.Sprintf("%s\n%s", content, event.Details)
	}

	return Message{
		Role:             "system",
		Content:          content,
		Time:             event.Timestamp,
		GlassBoxCategory: event.Category,
		// Expanded when verbose so details are readable without a keypress.
		IsCollapsed: !verbose || event.Details == "",
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
		return "Glass Box Debug Mode: **ON**\n\n**Full stream:** every shard, kernel, JIT, perception, routing, and status event streams into chat scrollback."
	}

	// Disable the event bus
	if m.glassBoxEventBus != nil {
		m.glassBoxEventBus.Disable()
	}
	return "Glass Box Debug Mode: **OFF**\n\nSystem events stop appearing in chat (tool executions still show)."
}

// toggleGlassBoxVerbose toggles verbose mode for detailed output.
func (m *Model) toggleGlassBoxVerbose() string {
	if m.glassBoxEventBus == nil {
		return "Glass Box event bus not initialized."
	}

	verbose := !m.glassBoxEventBus.IsVerbose()
	m.glassBoxEventBus.SetVerbose(verbose)

	if verbose {
		return "Glass Box Verbose Mode: **ON**\n\nEvents show expanded details and emit immediately (no batching delay)."
	}
	return "Glass Box Verbose Mode: **OFF**\n\nEvents show summaries only (batching re-enabled)."
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

	// This used to validate the name and then return "Category '%s' filter
	// toggled" without touching anything — the command reported a state change
	// it had not made, which is worse than not having the command at all when
	// you are using Glass Box to debug something.
	if m.glassBoxEventBus == nil {
		return "Glass Box event bus is not running, so category filters cannot be changed.\n\nEnable Glass Box first (`Alt+D` or `/glassbox`)."
	}

	active := m.glassBoxEventBus.ToggleCategory(transparency.GlassBoxCategory(category))
	if len(active) == 0 {
		return fmt.Sprintf("Category '%s' filter removed. No filter is active, so **all** categories stream.", category)
	}

	names := make([]string, 0, len(active))
	on := false
	for _, c := range active {
		names = append(names, string(c))
		if string(c) == category {
			on = true
		}
	}
	state := "OFF"
	if on {
		state = "ON"
	}
	return fmt.Sprintf("Category '%s' turned %s.\n\nStreaming only: %s\n\n(Toggle every category off to return to the full stream.)",
		category, state, strings.Join(names, ", "))
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
	sb.WriteString("- Scrollback mode: **FULL STREAM** (all events → chat)\n")

	sb.WriteString("\n### Categories\n")
	active := map[transparency.GlassBoxCategory]bool{}
	if m.glassBoxEventBus != nil {
		for _, c := range m.glassBoxEventBus.Categories() {
			active[c] = true
		}
	}
	filtered := len(active) > 0
	for _, c := range transparency.AllCategories() {
		mark := "streaming"
		if filtered && !active[c] {
			mark = "filtered out"
		}
		sb.WriteString(fmt.Sprintf("- `%s` (%s): %s\n", c, mark, categoryDescription(c)))
	}
	if !filtered {
		sb.WriteString("\nNo category filter is active — every category streams.\n")
	}

	sb.WriteString("\n### Keybindings\n")
	// Alt+D is the toggle; Alt+G cycles pane modes (model_key_handler.go:358/429).
	// This line said Alt+G for years and sent people to the wrong key.
	sb.WriteString("- `Alt+D`: Toggle Glass Box on/off\n")
	sb.WriteString("- `/glassbox verbose`: Toggle detailed output + immediate emit\n")
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

	// Glass Box is ON by default — full stream into chat scrollback.
	// User can still toggle off via Alt+G or `/glassbox`. We only
	// respect an *explicit* config opt-out; missing/nil config keeps
	// default-on behavior.
	enabled := true
	verbose := true // default verbose when debug streaming is desired
	var categories []string
	if m.Config != nil && m.Config.Transparency != nil {
		tc := m.Config.Transparency
		// Explicit disabled wins only when enabled is not also true.
		if tc.GlassBoxDisabled && !tc.GlassBoxEnabled {
			enabled = false
		} else {
			enabled = tc.GlassBoxEnabled || !tc.GlassBoxDisabled
		}
		verbose = tc.GlassBoxVerbose || tc.GlassBoxEnabled // verbose with full debug
		categories = tc.GlassBoxCategories
	}
	m.glassBoxEnabled = enabled
	if bus != nil {
		if enabled {
			bus.Enable()
		}
		bus.SetVerbose(verbose)
		if len(categories) > 0 {
			cats := make([]transparency.GlassBoxCategory, 0, len(categories))
			for _, c := range categories {
				if transparency.ValidCategory(c) {
					cats = append(cats, transparency.GlassBoxCategory(c))
				}
			}
			if len(cats) > 0 {
				bus.SetCategories(cats)
			}
		}
	}
}

// emitGlassBoxEvent is a helper to emit events from the chat package.
// It's a convenience wrapper around the event bus.
func (m *Model) emitGlassBoxEvent(category transparency.GlassBoxCategory, summary string, details string) {
	if m.glassBoxEventBus == nil || !m.glassBoxEnabled {
		return
	}

	m.glassBoxEventBus.EmitImmediate(transparency.GlassBoxEvent{
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

// listenObserverAssessments returns a tea.Cmd that waits for background observer assessments.
func (m Model) listenObserverAssessments() tea.Cmd {
	if m.observerAssessmentChan == nil {
		return nil
	}

	assessChan := m.observerAssessmentChan
	return func() tea.Msg {
		assess, ok := <-assessChan
		if !ok {
			return nil // Channel closed
		}
		return observerAssessmentMsg(assess)
	}
}
