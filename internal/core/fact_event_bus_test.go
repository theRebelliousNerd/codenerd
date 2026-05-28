package core

import (
	"sync"
	"testing"
	"time"
)

func TestFactEventBus_Subscribe_WhenMatchingPredicatePublished_ShouldReceiveEvent(t *testing.T) {
	bus := NewFactEventBus()
	ch := bus.Subscribe([]string{"user_intent"})
	defer bus.Unsubscribe(ch)

	bus.Publish("user_intent")

	select {
	case event := <-ch:
		if event.Predicate != "user_intent" {
			t.Errorf("expected predicate 'user_intent', got %q", event.Predicate)
		}
		if event.Timestamp.IsZero() {
			t.Error("expected non-zero timestamp")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestFactEventBus_Subscribe_WhenNonMatchingPredicatePublished_ShouldNotReceive(t *testing.T) {
	bus := NewFactEventBus()
	ch := bus.Subscribe([]string{"user_intent"})
	defer bus.Unsubscribe(ch)

	bus.Publish("pending_action") // different predicate

	select {
	case event := <-ch:
		t.Fatalf("should not have received event, got predicate=%q", event.Predicate)
	case <-time.After(50 * time.Millisecond):
		// Expected: no event received
	}
}

func TestFactEventBus_Subscribe_WhenMultiplePredicates_ShouldReceiveAll(t *testing.T) {
	bus := NewFactEventBus()
	ch := bus.Subscribe([]string{"user_intent", "pending_action", "next_action"})
	defer bus.Unsubscribe(ch)

	bus.Publish("pending_action")
	bus.Publish("next_action")

	received := make(map[string]bool)
	for range 2 {
		select {
		case event := <-ch:
			received[event.Predicate] = true
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timeout waiting for event")
		}
	}

	if !received["pending_action"] {
		t.Error("expected to receive pending_action event")
	}
	if !received["next_action"] {
		t.Error("expected to receive next_action event")
	}
}

func TestFactEventBus_Unsubscribe_WhenUnsubscribed_ShouldStopReceiving(t *testing.T) {
	bus := NewFactEventBus()
	ch := bus.Subscribe([]string{"user_intent"})

	bus.Unsubscribe(ch)

	// Channel should be closed
	_, ok := <-ch
	if ok {
		t.Fatal("expected channel to be closed after unsubscribe")
	}

	if bus.SubscriberCount() != 0 {
		t.Errorf("expected 0 subscribers after unsubscribe, got %d", bus.SubscriberCount())
	}
}

func TestFactEventBus_Unsubscribe_WhenCalledTwice_ShouldBeNoOp(t *testing.T) {
	bus := NewFactEventBus()
	ch := bus.Subscribe([]string{"user_intent"})

	bus.Unsubscribe(ch)
	// Second call should not panic
	bus.Unsubscribe(ch)
}

func TestFactEventBus_Publish_WhenChannelFull_ShouldNotBlock(t *testing.T) {
	bus := NewFactEventBus()
	ch := bus.Subscribe([]string{"user_intent"})
	defer bus.Unsubscribe(ch)

	// Fill the channel buffer (capacity 16)
	for range 20 {
		bus.Publish("user_intent")
	}

	// Should not have blocked — drain what we can
	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			goto done
		}
	}
done:
	if count != 16 {
		t.Errorf("expected 16 buffered events (channel capacity), got %d", count)
	}
}

func TestFactEventBus_MultipleSubscribers_WhenPublished_ShouldAllReceive(t *testing.T) {
	bus := NewFactEventBus()
	ch1 := bus.Subscribe([]string{"user_intent"})
	ch2 := bus.Subscribe([]string{"user_intent"})
	ch3 := bus.Subscribe([]string{"user_intent"})
	defer bus.Unsubscribe(ch1)
	defer bus.Unsubscribe(ch2)
	defer bus.Unsubscribe(ch3)

	bus.Publish("user_intent")

	for i, ch := range []chan FactEvent{ch1, ch2, ch3} {
		select {
		case event := <-ch:
			if event.Predicate != "user_intent" {
				t.Errorf("subscriber %d: expected 'user_intent', got %q", i, event.Predicate)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("subscriber %d: timeout", i)
		}
	}

	if bus.PredicateSubscriberCount("user_intent") != 3 {
		t.Errorf("expected 3 subscribers for user_intent, got %d", bus.PredicateSubscriberCount("user_intent"))
	}
}

func TestFactEventBus_ConcurrentPublishSubscribe_ShouldNotRace(t *testing.T) {
	bus := NewFactEventBus()
	var wg sync.WaitGroup

	// Spawn 10 concurrent subscribers
	channels := make([]chan FactEvent, 10)
	for i := range 10 {
		channels[i] = bus.Subscribe([]string{"user_intent", "pending_action"})
	}

	// Spawn 10 concurrent publishers
	for range 10 {
		wg.Go(func() {
			for range 100 {
				bus.Publish("user_intent")
				bus.Publish("pending_action")
			}
		})
	}

	wg.Wait()

	// Unsubscribe all
	for _, ch := range channels {
		bus.Unsubscribe(ch)
	}

	if bus.SubscriberCount() != 0 {
		t.Errorf("expected 0 subscribers after cleanup, got %d", bus.SubscriberCount())
	}
}

func TestFactEventBus_SubscriberCount_ShouldTrackCorrectly(t *testing.T) {
	bus := NewFactEventBus()

	if bus.SubscriberCount() != 0 {
		t.Fatalf("expected 0 initial subscribers, got %d", bus.SubscriberCount())
	}

	ch1 := bus.Subscribe([]string{"a"})
	ch2 := bus.Subscribe([]string{"b", "c"})

	if bus.SubscriberCount() != 2 {
		t.Errorf("expected 2 subscribers, got %d", bus.SubscriberCount())
	}

	bus.Unsubscribe(ch1)
	if bus.SubscriberCount() != 1 {
		t.Errorf("expected 1 subscriber after unsubscribe, got %d", bus.SubscriberCount())
	}

	bus.Unsubscribe(ch2)
	if bus.SubscriberCount() != 0 {
		t.Errorf("expected 0 subscribers after full cleanup, got %d", bus.SubscriberCount())
	}
}
