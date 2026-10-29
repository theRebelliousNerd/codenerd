package transparency

import (
	"testing"
	"time"
)

func TestGlassBoxEventBusEmitImmediate(t *testing.T) {
	bus := NewGlassBoxEventBus()
	bus.Enable()
	ch := bus.Subscribe()
	defer bus.Close()

	bus.EmitImmediate(GlassBoxEvent{
		Category: CategoryKernel,
		Summary:  "event",
	})

	select {
	case evt := <-ch:
		if evt.Summary != "event" {
			t.Fatalf("unexpected summary: %s", evt.Summary)
		}
		if evt.ID == 0 {
			t.Fatalf("expected sequence id")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected event to be delivered")
	}
}

func TestGlassBoxEventBusFilter(t *testing.T) {
	bus := NewGlassBoxEventBus()
	bus.Enable()
	bus.SetCategories([]GlassBoxCategory{CategoryKernel})
	ch := bus.Subscribe()
	defer bus.Close()

	bus.EmitImmediate(GlassBoxEvent{
		Category: CategoryPerception,
		Summary:  "filtered",
	})

	select {
	case <-ch:
		t.Fatalf("unexpected event for filtered category")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestGlassBoxEventBusFlush(t *testing.T) {
	bus := NewGlassBoxEventBus()
	bus.Enable()
	ch := bus.Subscribe()
	defer bus.Close()

	bus.Emit(GlassBoxEvent{
		Category: CategoryShard,
		Summary:  "buffered",
	})
	bus.Flush()

	select {
	case evt := <-ch:
		if evt.Summary != "buffered" {
			t.Fatalf("unexpected summary: %s", evt.Summary)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected buffered event to be delivered")
	}

	stats := bus.Stats()
	if stats.TotalEmitted == 0 {
		t.Fatalf("expected total emitted count")
	}
}

func TestGlassBoxEventBusUnsubscribe(t *testing.T) {
	bus := NewGlassBoxEventBus()
	bus.Enable()
	ch1 := bus.Subscribe()
	ch2 := bus.Subscribe()
	defer bus.Close()

	bus.Unsubscribe(ch2)

	select {
	case _, ok := <-ch2:
		if ok {
			t.Fatalf("expected unsubscribed channel to be closed")
		}
	default:
		t.Fatalf("expected unsubscribed channel to be closed")
	}

	bus.EmitImmediate(GlassBoxEvent{
		Category: CategoryKernel,
		Summary:  "alive",
	})

	select {
	case evt := <-ch1:
		if evt.Summary != "alive" {
			t.Fatalf("unexpected summary: %s", evt.Summary)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected event on remaining subscriber")
	}
}

func TestGlassBoxEventBusClearTurn(t *testing.T) {
	bus := NewGlassBoxEventBus()
	bus.batchWindow = time.Hour
	bus.Enable()
	ch := bus.Subscribe()
	defer bus.Close()

	bus.Emit(GlassBoxEvent{
		Category: CategoryShard,
		Summary:  "turn-1",
		TurnID:   1,
	})
	bus.Emit(GlassBoxEvent{
		Category: CategoryShard,
		Summary:  "turn-2",
		TurnID:   2,
	})

	bus.ClearTurn(1)
	bus.Flush()

	select {
	case evt := <-ch:
		if evt.Summary != "turn-2" {
			t.Fatalf("expected remaining event for turn 2, got %s", evt.Summary)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected event after flush")
	}
}
