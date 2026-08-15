// Package transparency provides operation visibility for codeNERD.
// This file implements the Glass Box event bus for collecting and dispatching events.
package transparency

import (
	"io"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// GlassBoxEventBus collects events from multiple sources and dispatches to subscribers.
// It uses batching to reduce UI churn and sequence numbers for proper ordering.
type GlassBoxEventBus struct {
	mu          sync.RWMutex
	subscribers []chan<- GlassBoxEvent
	enabled     atomic.Bool

	// Batching configuration
	batchWindow time.Duration // Time window for collecting events before dispatch
	batchLimit  int           // Max events per batch

	// Event buffer for batching
	buffer     []GlassBoxEvent
	bufferMu   sync.Mutex
	flushTimer *time.Timer

	// Temporal ordering
	sequence atomic.Uint64

	// dropped counts events discarded because a subscriber channel was full.
	// Drop-on-full is deliberate (never block the executive), but a silent
	// drop is indistinguishable from a producer that never fired — which is
	// the failure mode operators actually hit when the TUI stalls.
	dropped atomic.Uint64

	// delivered counts successful sends to subscriber channels.
	delivered atomic.Uint64

	// Filtering
	categories map[GlassBoxCategory]bool // Empty means all allowed
	verbose    bool

	// sinks are non-channel observers (NDJSON export, etc.).
	sinks []EventSink
}

// EventSink receives every event the bus dispatches, in addition to channel
// subscribers. Implementations must not block.
//
// This is the extension point for machine-readable export. An OpenTelemetry
// bridge would be a sink too — deliberately not built here: nothing in this
// repo configures an otel TracerProvider, so a bridge would map categories
// onto no-op spans and produce exactly the "looks wired, does nothing" shape
// this package is trying to remove.
type EventSink interface {
	Write(event GlassBoxEvent)
}

// NewGlassBoxEventBus creates a new event bus with default settings.
// Subscriber channels are large enough to absorb multi-shard tool storms
// without silent drops during Glass Box full-stream mode.
func NewGlassBoxEventBus() *GlassBoxEventBus {
	b := &GlassBoxEventBus{
		batchWindow: 50 * time.Millisecond,
		batchLimit:  20,
		buffer:      make([]GlassBoxEvent, 0, 64),
		categories:  make(map[GlassBoxCategory]bool),
	}
	// Headless runs (nerd run / campaign) have no TUI subscriber; the env sink
	// is the only way their Glass Box stream survives the process.
	attachEnvNDJSONSink(b)
	adoptProcessBus(b)
	return b
}

// AddSink registers a non-channel observer. Safe to call at any time; the sink
// receives events dispatched after registration.
func (b *GlassBoxEventBus) AddSink(sink EventSink) {
	if sink == nil {
		return
	}
	b.mu.Lock()
	b.sinks = append(b.sinks, sink)
	b.mu.Unlock()
}

// dispatchLocked sends one event to every subscriber and sink.
// Caller must hold b.mu (read lock is sufficient).
func (b *GlassBoxEventBus) dispatchLocked(event GlassBoxEvent) {
	for _, sub := range b.subscribers {
		select {
		case sub <- event:
			b.delivered.Add(1)
		default:
			// Drop rather than block the producing goroutine.
			b.dropped.Add(1)
		}
	}
	for _, sink := range b.sinks {
		sink.Write(event)
	}
}

// Enable activates the event bus.
func (b *GlassBoxEventBus) Enable() {
	b.enabled.Store(true)
}

// Disable deactivates the event bus.
func (b *GlassBoxEventBus) Disable() {
	b.enabled.Store(false)
	// Flush any pending events
	b.Flush()
}

// IsEnabled returns true if the event bus is active.
func (b *GlassBoxEventBus) IsEnabled() bool {
	return b.enabled.Load()
}

// SetVerbose enables/disables verbose mode.
func (b *GlassBoxEventBus) SetVerbose(v bool) {
	b.mu.Lock()
	b.verbose = v
	b.mu.Unlock()
}

// IsVerbose returns true if verbose mode is enabled.
func (b *GlassBoxEventBus) IsVerbose() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.verbose
}

// SetCategories sets the allowed categories. Empty slice means all allowed.
func (b *GlassBoxEventBus) SetCategories(categories []GlassBoxCategory) {
	b.mu.Lock()
	b.categories = make(map[GlassBoxCategory]bool)
	for _, c := range categories {
		b.categories[c] = true
	}
	b.mu.Unlock()
}

// Categories returns the current allow-list, sorted for stable display.
// An empty result means no filter is set and every category is emitted.
func (b *GlassBoxEventBus) Categories() []GlassBoxCategory {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.categories) == 0 {
		return nil
	}
	// Iterate AllCategories rather than the map so the order is deterministic.
	out := make([]GlassBoxCategory, 0, len(b.categories))
	for _, c := range AllCategories() {
		if b.categories[c] {
			out = append(out, c)
		}
	}
	return out
}

// ToggleCategory flips one category in the allow-list and returns the resulting
// list (nil meaning "no filter, everything emitted").
//
// Because an empty allow-list means "all allowed", toggling a category ON from
// the unfiltered state restricts the stream to just that category, and toggling
// the last one back OFF returns to the unfiltered stream. That is what a user
// typing `/glassbox kernel` twice expects.
func (b *GlassBoxEventBus) ToggleCategory(c GlassBoxCategory) []GlassBoxCategory {
	b.mu.Lock()
	if b.categories == nil {
		b.categories = make(map[GlassBoxCategory]bool)
	}
	if b.categories[c] {
		delete(b.categories, c)
	} else {
		b.categories[c] = true
	}
	b.mu.Unlock()
	return b.Categories()
}

// Subscribe returns a channel that will receive events.
// The channel is buffered large enough that full-stream debug mode can
// keep up with concurrent shard/tool bursts without dropping events.
func (b *GlassBoxEventBus) Subscribe() <-chan GlassBoxEvent {
	ch := make(chan GlassBoxEvent, 512)
	b.mu.Lock()
	b.subscribers = append(b.subscribers, ch)
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber channel.
func (b *GlassBoxEventBus) Unsubscribe(ch <-chan GlassBoxEvent) {
	if ch == nil {
		return
	}
	target := reflect.ValueOf(ch).Pointer()
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, sub := range b.subscribers {
		if reflect.ValueOf(sub).Pointer() == target {
			b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)
			close(sub)
			break
		}
	}
}

// Emit sends an event to all subscribers (with batching).
// This is safe to call from any goroutine.
// When verbose (full debug stream) is on, events dispatch immediately so
// the chat shows live activity with no batch delay.
func (b *GlassBoxEventBus) Emit(event GlassBoxEvent) {
	if !b.enabled.Load() {
		return
	}

	// Verbose full-stream: skip batching — user wants live chat telemetry.
	if b.IsVerbose() {
		b.EmitImmediate(event)
		return
	}

	// Apply category filter
	b.mu.RLock()
	if len(b.categories) > 0 && !b.categories[event.Category] {
		b.mu.RUnlock()
		return
	}
	b.mu.RUnlock()

	// Assign sequence number for ordering
	event.ID = b.sequence.Add(1)
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	b.bufferMu.Lock()
	b.buffer = append(b.buffer, event)

	// Flush if batch limit reached, else start timer
	if len(b.buffer) >= b.batchLimit {
		b.flushLocked()
	} else if b.flushTimer == nil {
		b.flushTimer = time.AfterFunc(b.batchWindow, func() {
			b.bufferMu.Lock()
			b.flushLocked()
			b.bufferMu.Unlock()
		})
	}
	b.bufferMu.Unlock()
}

// EmitImmediate sends an event immediately without batching.
// Use for high-priority events that should appear instantly.
func (b *GlassBoxEventBus) EmitImmediate(event GlassBoxEvent) {
	if !b.enabled.Load() {
		return
	}

	// Apply category filter
	b.mu.RLock()
	if len(b.categories) > 0 && !b.categories[event.Category] {
		b.mu.RUnlock()
		return
	}
	b.mu.RUnlock()

	// Assign sequence number
	event.ID = b.sequence.Add(1)
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Dispatch directly
	b.mu.RLock()
	b.dispatchLocked(event)
	b.mu.RUnlock()
}

// Flush dispatches all buffered events immediately.
func (b *GlassBoxEventBus) Flush() {
	b.bufferMu.Lock()
	b.flushLocked()
	b.bufferMu.Unlock()
}

// flushLocked sends buffered events (must hold bufferMu).
func (b *GlassBoxEventBus) flushLocked() {
	if len(b.buffer) == 0 {
		return
	}

	if b.flushTimer != nil {
		b.flushTimer.Stop()
		b.flushTimer = nil
	}

	// Sort by sequence number for proper ordering
	sort.Slice(b.buffer, func(i, j int) bool {
		return b.buffer[i].ID < b.buffer[j].ID
	})

	b.mu.RLock()
	for _, event := range b.buffer {
		b.dispatchLocked(event)
	}
	b.mu.RUnlock()

	// Clear buffer
	b.buffer = b.buffer[:0]
}

// ClearTurn removes events from a specific turn.
// Useful for cleaning up after turn completion.
func (b *GlassBoxEventBus) ClearTurn(turnID int) {
	b.bufferMu.Lock()
	defer b.bufferMu.Unlock()

	filtered := b.buffer[:0]
	for _, e := range b.buffer {
		if e.TurnID != turnID {
			filtered = append(filtered, e)
		}
	}
	b.buffer = filtered
}

// Close shuts down the event bus and all subscriber channels.
func (b *GlassBoxEventBus) Close() {
	b.Disable()

	b.mu.Lock()
	defer b.mu.Unlock()

	for _, sub := range b.subscribers {
		close(sub)
	}
	b.subscribers = nil

	// Sinks own file handles; a headless run that never closes them loses the
	// tail of its buffered NDJSON.
	for _, sink := range b.sinks {
		if closer, ok := sink.(io.Closer); ok {
			_ = closer.Close()
		}
	}
	b.sinks = nil
}

// Stats returns current event bus statistics.
func (b *GlassBoxEventBus) Stats() GlassBoxBusStats {
	b.mu.RLock()
	b.bufferMu.Lock()
	defer b.bufferMu.Unlock()
	defer b.mu.RUnlock()

	return GlassBoxBusStats{
		Enabled:         b.enabled.Load(),
		SubscriberCount: len(b.subscribers),
		BufferedEvents:  len(b.buffer),
		TotalEmitted:    b.sequence.Load(),
		CategoryCount:   len(b.categories),
		Verbose:         b.verbose,
		Delivered:       b.delivered.Load(),
		Dropped:         b.dropped.Load(),
		SinkCount:       len(b.sinks),
	}
}

// GlassBoxBusStats holds event bus statistics.
type GlassBoxBusStats struct {
	Enabled         bool
	SubscriberCount int
	BufferedEvents  int
	TotalEmitted    uint64
	CategoryCount   int
	Verbose         bool
	// Delivered counts events successfully handed to a subscriber channel
	// (counted once per subscriber).
	Delivered uint64
	// Dropped counts events discarded because a subscriber channel was full.
	// Non-zero means the TUI is behind and scrollback is incomplete.
	Dropped uint64
	// SinkCount is the number of registered non-channel sinks.
	SinkCount int
}
