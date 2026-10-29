package transparency

import (
	"testing"
)

// The Glass Box category filter is the control you reach for when the debug
// stream is too noisy to read. `/glassbox <category>` used to validate the name
// and then report "filter toggled" without touching the bus, so the stream
// never changed. These pin the real behaviour.
func TestEventBus_ToggleCategory_FiltersTheStream(t *testing.T) {
	bus := NewGlassBoxEventBus()
	bus.Enable()
	sub := bus.Subscribe()

	// No filter set: everything is allowed.
	if got := bus.Categories(); len(got) != 0 {
		t.Fatalf("a fresh bus should have no category filter, got %v", got)
	}

	// Toggling one category ON from the unfiltered state restricts to just it.
	active := bus.ToggleCategory(CategoryKernel)
	if len(active) != 1 || active[0] != CategoryKernel {
		t.Fatalf("ToggleCategory(kernel) = %v, want [kernel]", active)
	}

	bus.EmitImmediate(GlassBoxEvent{Category: CategoryKernel, Summary: "kept"})
	bus.EmitImmediate(GlassBoxEvent{Category: CategoryShard, Summary: "dropped"})

	select {
	case ev := <-sub:
		if ev.Category != CategoryKernel {
			t.Errorf("first event category = %s, want kernel", ev.Category)
		}
	default:
		t.Fatal("kernel event was filtered out but should have passed")
	}
	select {
	case ev := <-sub:
		t.Errorf("shard event should have been filtered, got %+v", ev)
	default:
	}

	// Toggling the last category back OFF returns to the full stream.
	if active := bus.ToggleCategory(CategoryKernel); len(active) != 0 {
		t.Fatalf("toggling the last category off should clear the filter, got %v", active)
	}
	bus.EmitImmediate(GlassBoxEvent{Category: CategoryShard, Summary: "now kept"})
	select {
	case ev := <-sub:
		if ev.Category != CategoryShard {
			t.Errorf("event category = %s, want shard", ev.Category)
		}
	default:
		t.Error("with no filter active every category must stream")
	}
}

// Categories() must be stable across calls so the TUI does not print the filter
// list in a different order every time (Go map iteration is randomized).
func TestEventBus_Categories_IsDeterministicallyOrdered(t *testing.T) {
	bus := NewGlassBoxEventBus()
	bus.ToggleCategory(CategoryShard)
	bus.ToggleCategory(CategoryPerception)
	bus.ToggleCategory(CategoryKernel)

	first := bus.Categories()
	if len(first) != 3 {
		t.Fatalf("expected 3 active categories, got %v", first)
	}
	for range 20 {
		got := bus.Categories()
		for i := range got {
			if got[i] != first[i] {
				t.Fatalf("Categories() order is unstable: %v then %v", first, got)
			}
		}
	}

	// Order follows AllCategories(), not insertion or map order.
	var want []GlassBoxCategory
	for _, c := range AllCategories() {
		if c == CategoryShard || c == CategoryPerception || c == CategoryKernel {
			want = append(want, c)
		}
	}
	for i := range want {
		if first[i] != want[i] {
			t.Errorf("Categories() = %v, want AllCategories() order %v", first, want)
		}
	}
}
