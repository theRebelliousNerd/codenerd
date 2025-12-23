// Package transparency provides operation visibility for codeNERD.
// This file defines Glass Box event types for inline debug display.
package transparency

import (
	"fmt"
	"strings"
	"time"
)

// GlassBoxCategory identifies the source subsystem of an event.
type GlassBoxCategory string

const (
	CategoryPerception GlassBoxCategory = "perception" // Intent parsing, entity resolution
	CategoryKernel     GlassBoxCategory = "kernel"     // Fact assertions, rule derivations
	CategoryJIT        GlassBoxCategory = "jit"        // Prompt atom selection, compilation
	CategoryShard      GlassBoxCategory = "shard"      // Shard spawn, phase transitions
	CategoryControl    GlassBoxCategory = "control"    // Control packets from LLM responses
	CategoryRouting    GlassBoxCategory = "routing"    // Tool routing and execution
)

// String returns the display name for the category.
func (c GlassBoxCategory) String() string {
	return string(c)
}

// DisplayPrefix returns the bracketed prefix for inline display.
func (c GlassBoxCategory) DisplayPrefix() string {
	return fmt.Sprintf("[%s]", strings.ToUpper(string(c)))
}

// GlassBoxEvent represents a single event for Glass Box display.
type GlassBoxEvent struct {
	// ID is a sequence number for ordering across async sources
	ID uint64

	// Timestamp when the event occurred
	Timestamp time.Time

	// Category identifies the source subsystem
	Category GlassBoxCategory

	// Summary is a one-line description for inline display
	Summary string

	// Details provides expanded information (shown in verbose mode)
	Details string

	// TurnID associates the event with a conversation turn
	TurnID int

	// Duration for timed operations (optional)
	Duration time.Duration

	// Source identifies the specific component (e.g., shard ID)
	Source string
}

// String returns a formatted string for display.
func (e GlassBoxEvent) String() string {
	prefix := e.Category.DisplayPrefix()
	result := fmt.Sprintf("%s %s", prefix, e.Summary)
	if e.Duration > 0 {
		result += fmt.Sprintf(" (%.1fms)", float64(e.Duration.Microseconds())/1000)
	}
	return result
}

// HasDetails returns true if the event has expanded details.
func (e GlassBoxEvent) HasDetails() bool {
	return e.Details != ""
}

// AllCategories returns all valid Glass Box categories.
func AllCategories() []GlassBoxCategory {
	return []GlassBoxCategory{
		CategoryPerception,
		CategoryKernel,
		CategoryJIT,
		CategoryShard,
		CategoryControl,
		CategoryRouting,
	}
}

// ValidCategory returns true if the category string is valid.
func ValidCategory(s string) bool {
	for _, c := range AllCategories() {
		if string(c) == s {
			return true
		}
	}
	return false
}

// ToolEvent represents a tool execution notification for the main chat.
// Unlike GlassBoxEvent, ToolEvent is ALWAYS displayed regardless of debug mode.
type ToolEvent struct {
	ToolName  string        // Name of the tool executed
	Result    string        // Result or error message (truncated for display)
	Success   bool          // True if tool succeeded
	Duration  time.Duration // Execution duration
	Timestamp time.Time     // When the tool executed
}

// ToolEventBus provides a simple channel for tool execution notifications.
// Unlike GlassBoxEventBus, this is always active - tool events always show in chat.
type ToolEventBus struct {
	ch chan ToolEvent
}

// NewToolEventBus creates a new tool event bus.
func NewToolEventBus() *ToolEventBus {
	return &ToolEventBus{
		ch: make(chan ToolEvent, 50), // Buffered to prevent blocking
	}
}

// Emit sends a tool event. This is always active, never gated.
func (b *ToolEventBus) Emit(event ToolEvent) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	select {
	case b.ch <- event:
	default:
		// Drop if channel full (shouldn't happen with buffer)
	}
}

// Subscribe returns the channel to receive tool events.
func (b *ToolEventBus) Subscribe() <-chan ToolEvent {
	return b.ch
}

// Close shuts down the tool event bus.
func (b *ToolEventBus) Close() {
	close(b.ch)
}
