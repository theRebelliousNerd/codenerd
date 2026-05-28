package core

import (
	"sync"
	"time"
)

// FactEvent is a lightweight notification that a predicate was mutated.
// Subscribers use this to wake up and query the kernel for full facts.
type FactEvent struct {
	Predicate string
	Timestamp time.Time
}

// FactEventBus is a thread-safe pub/sub for kernel fact mutations.
// When Assert() adds a new fact, the bus publishes to all subscribers
// interested in that predicate. This replaces polling-based system shard loops.
//
// Design decisions:
//   - Buffered channels (capacity 16) prevent Assert from blocking on slow consumers
//   - Non-blocking send: if a subscriber's buffer is full, the event is dropped
//     (the fallback heartbeat ticker in each shard will catch missed events)
//   - Read lock for Publish (hot path), write lock only for Subscribe/Unsubscribe
//   - Subscribers can listen on multiple predicates via a single channel
type FactEventBus struct {
	mu sync.RWMutex

	// subscribers maps predicate name -> list of subscriber channels.
	// A single channel may appear under multiple predicates.
	subscribers map[string][]chan FactEvent

	// channelPredicates tracks which predicates each channel is subscribed to,
	// enabling O(predicates) unsubscribe instead of scanning all predicates.
	channelPredicates map[chan FactEvent][]string
}

// NewFactEventBus creates a new event bus.
func NewFactEventBus() *FactEventBus {
	return &FactEventBus{
		subscribers:       make(map[string][]chan FactEvent),
		channelPredicates: make(map[chan FactEvent][]string),
	}
}

// Subscribe returns a channel that receives events when any of the listed
// predicates are asserted. The returned channel has a buffer of 16 to absorb
// bursts without blocking the kernel's Assert path.
//
// The caller MUST eventually call Unsubscribe to prevent resource leaks.
func (b *FactEventBus) Subscribe(predicates []string) chan FactEvent {
	ch := make(chan FactEvent, 16)

	b.mu.Lock()
	defer b.mu.Unlock()

	for _, pred := range predicates {
		b.subscribers[pred] = append(b.subscribers[pred], ch)
	}
	b.channelPredicates[ch] = predicates
	return ch
}

// Unsubscribe removes a subscription and closes the channel.
// Safe to call multiple times (subsequent calls are no-ops).
func (b *FactEventBus) Unsubscribe(ch chan FactEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	predicates, ok := b.channelPredicates[ch]
	if !ok {
		return // Already unsubscribed
	}

	// Remove channel from each predicate's subscriber list
	for _, pred := range predicates {
		subs := b.subscribers[pred]
		filtered := make([]chan FactEvent, 0, len(subs))
		for _, sub := range subs {
			if sub != ch {
				filtered = append(filtered, sub)
			}
		}
		if len(filtered) == 0 {
			delete(b.subscribers, pred)
		} else {
			b.subscribers[pred] = filtered
		}
	}

	delete(b.channelPredicates, ch)
	close(ch)
}

// Publish notifies all subscribers interested in the given predicate.
// This is called by Assert/AssertBatch after a fact is successfully added.
//
// Non-blocking: if a subscriber's channel is full, the event is silently dropped.
// This ensures Assert never blocks on a slow consumer. The subscriber's fallback
// heartbeat ticker will catch any missed events.
func (b *FactEventBus) Publish(predicate string) {
	// Hold the RLock for the duration of the send so a concurrent
	// Unsubscribe (write-lock) cannot close any ch while we're sending
	// to it. Without this, the previous snapshot-then-release pattern
	// raced with Unsubscribe → close(ch) and would panic on the next
	// `ch <- event` ("send on closed channel"). The bus is on the
	// kernel's Assert hot path, so the panic crashed the whole agent
	// non-deterministically under load.
	//
	// RLock allows many concurrent Publishers to proceed in parallel;
	// only Unsubscribe (Lock) serializes against them, which is the
	// correct trade-off for an Assert-heavy workload.
	b.mu.RLock()
	defer b.mu.RUnlock()

	subs := b.subscribers[predicate]
	if len(subs) == 0 {
		return
	}

	event := FactEvent{
		Predicate: predicate,
		Timestamp: time.Now(),
	}

	for _, ch := range subs {
		// Non-blocking send — drop if buffer full. Safe under RLock:
		// Unsubscribe cannot close ch concurrently.
		select {
		case ch <- event:
		default:
		}
	}
}

// SubscriberCount returns the number of active subscriber channels.
// Used for testing and diagnostics.
func (b *FactEventBus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.channelPredicates)
}

// PredicateSubscriberCount returns how many subscribers are listening
// for the given predicate. Used for testing and diagnostics.
func (b *FactEventBus) PredicateSubscriberCount(predicate string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers[predicate])
}
