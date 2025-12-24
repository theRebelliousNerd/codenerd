// Package transparency provides operation visibility for codeNERD.
// This file implements the Glass Box event bus for collecting and dispatching events.
package transparency

import (
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

	// Filtering
	categories map[GlassBoxCategory]bool // Empty means all allowed
	verbose    bool
}

// NewGlassBoxEventBus creates a new event bus with default settings.
func NewGlassBoxEventBus() *GlassBoxEventBus {
	return &GlassBoxEventBus{
		batchWindow: 100 * time.Millisecond,
		batchLimit:  10,
		buffer:      make([]GlassBoxEvent, 0, 20),
		categories:  make(map[GlassBoxCategory]bool),
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

// Subscribe returns a channel that will receive events.
// The channel is buffered to prevent blocking emitters.
func (b *GlassBoxEventBus) Subscribe() <-chan GlassBoxEvent {
	ch := make(chan GlassBoxEvent, 50)
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
func (b *GlassBoxEventBus) Emit(event GlassBoxEvent) {
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
	for _, sub := range b.subscribers {
		select {
		case sub <- event:
		default: // Drop if channel full
		}
	}
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
	for _, sub := range b.subscribers {
		for _, event := range b.buffer {
			select {
			case sub <- event:
			default: // Drop if channel full
			}
		}
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
}
