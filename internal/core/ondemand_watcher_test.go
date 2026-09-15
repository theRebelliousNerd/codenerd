package core

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// waitForSubscription polls until the watcher's goroutine has subscribed.
// Publish-before-subscribe loses the event by design (pub/sub has no
// backlog), so every test that publishes must rendezvous here first.
func waitForSubscription(t *testing.T, bus *FactEventBus) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for bus.SubscriberCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if bus.SubscriberCount() == 0 {
		t.Fatal("watcher never subscribed within 5s")
	}
}

// TestOnDemandWatcherFiresOnTrigger proves the runtime half of on-demand
// activation: a trigger predicate on the fact bus reaches the ensure func.
// The debounce window is 1s, so the test polls with a hard deadline instead
// of sleeping a fixed guess.
func TestOnDemandWatcherFiresOnTrigger(t *testing.T) {
	bus := NewFactEventBus()
	var calls atomic.Int64
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := StartOnDemandWatcher(ctx, bus, func(context.Context) []string {
		calls.Add(1)
		return nil
	})
	defer stop()
	waitForSubscription(t, bus)

	bus.Publish("modified")

	deadline := time.Now().Add(5 * time.Second)
	for calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if calls.Load() == 0 {
		t.Fatal("publishing 'modified' never invoked ensure within 5s")
	}
}

// TestOnDemandWatcherIgnoresUnrelatedPredicates proves the watcher subscribes
// narrowly: predicates outside the trigger vocabulary must not wake it. The
// fallback sweep is 30s out, so a quiet 1.5s window is conclusive.
func TestOnDemandWatcherIgnoresUnrelatedPredicates(t *testing.T) {
	bus := NewFactEventBus()
	var calls atomic.Int64
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := StartOnDemandWatcher(ctx, bus, func(context.Context) []string {
		calls.Add(1)
		return nil
	})
	defer stop()
	waitForSubscription(t, bus)

	bus.Publish("some_unrelated_predicate")
	time.Sleep(1500 * time.Millisecond)

	if got := calls.Load(); got != 0 {
		t.Fatalf("unrelated predicate woke the watcher %d times", got)
	}
}

// TestOnDemandWatcherStopUnsubscribes proves lifecycle hygiene: after stop,
// the bus holds no subscription (no leak) and further triggers invoke
// nothing. Stop is also idempotent — a double stop must not panic or hang.
func TestOnDemandWatcherStopUnsubscribes(t *testing.T) {
	bus := NewFactEventBus()
	var calls atomic.Int64

	stop := StartOnDemandWatcher(context.Background(), bus, func(context.Context) []string {
		calls.Add(1)
		return nil
	})
	waitForSubscription(t, bus)
	if bus.SubscriberCount() != 1 {
		t.Fatalf("watcher must hold exactly 1 subscription, bus has %d", bus.SubscriberCount())
	}

	stop()
	stop()

	if n := bus.SubscriberCount(); n != 0 {
		t.Fatalf("stop must unsubscribe, bus still has %d subscribers", n)
	}
	bus.Publish("modified")
	time.Sleep(1500 * time.Millisecond)
	if got := calls.Load(); got != 0 {
		t.Fatalf("stopped watcher still fired %d times", got)
	}
}

// TestOnDemandWatcherNilDepsIsNoop proves the constructor fails closed: nil
// bus or nil ensure yields a stop func that does nothing, never a panic and
// never a leaked goroutine.
func TestOnDemandWatcherNilDepsIsNoop(t *testing.T) {
	stop := StartOnDemandWatcher(context.Background(), nil, func(context.Context) []string { return nil })
	stop()
	stop()

	bus := NewFactEventBus()
	stop = StartOnDemandWatcher(context.Background(), bus, nil)
	stop()
	if n := bus.SubscriberCount(); n != 0 {
		t.Fatalf("nil-ensure watcher must not subscribe, bus has %d", n)
	}
}
