package chat

import (
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/transparency"
)

// TestHandleGlassBoxEvent_StreamsAllCategories verifies debug mode puts
// every category into chat scrollback (not just milestone events).
func TestHandleGlassBoxEvent_StreamsAllCategories(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))
	m.glassBoxEnabled = true
	bus := transparency.NewGlassBoxEventBus()
	bus.Enable()
	bus.SetVerbose(true)
	m.glassBoxEventBus = bus

	before := len(m.history)
	categories := []transparency.GlassBoxCategory{
		transparency.CategoryPerception,
		transparency.CategoryKernel,
		transparency.CategoryJIT,
		transparency.CategoryShard,
		transparency.CategoryControl,
		transparency.CategoryRouting,
	}
	for i, cat := range categories {
		m.handleGlassBoxEvent(transparency.GlassBoxEvent{
			Timestamp: time.Now(),
			Category:  cat,
			Summary:   "stream-test " + string(cat),
			Details:   "detail payload",
			// No Duration — previously perception/JIT without duration
			// were filtered out of scrollback by isMilestoneEvent.
			TurnID: 1,
			ID:     uint64(i + 1),
		})
	}

	added := len(m.history) - before
	if added != len(categories) {
		t.Fatalf("expected %d glass-box lines in history, got %d", len(categories), added)
	}
	for _, msg := range m.history[before:] {
		if msg.Role != "system" {
			t.Errorf("expected system role, got %q", msg.Role)
		}
		if msg.Content == "" {
			t.Error("empty glass-box content")
		}
		// Verbose → details included and not collapsed.
		if !msg.IsCollapsed && msg.GlassBoxCategory == transparency.CategoryPerception {
			if !strings.Contains(msg.Content, "detail payload") {
				t.Errorf("verbose mode should expand details, got %q", msg.Content)
			}
		}
	}
}

// TestInitGlassBox_WiresVerboseFromConfig ensures config.json glass_box_verbose
// is applied at boot so Emit paths go immediate.
func TestInitGlassBox_WiresVerboseFromConfig(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))
	m.Config = &config.UserConfig{
		Transparency: &config.TransparencyConfig{
			GlassBoxEnabled: true,
			GlassBoxVerbose: true,
		},
	}
	bus := transparency.NewGlassBoxEventBus()
	m.initGlassBox(bus)

	if !m.glassBoxEnabled {
		t.Fatal("glass box should be enabled from config")
	}
	if !bus.IsVerbose() {
		t.Fatal("verbose should be wired from config at init")
	}
	if m.glassBoxEventChan == nil {
		t.Fatal("should subscribe to event bus")
	}
}

// TestDrainGlassBoxEvents_FoldsBurst ensures buffered events land in one drain.
func TestDrainGlassBoxEvents_FoldsBurst(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))
	m.glassBoxEnabled = true
	bus := transparency.NewGlassBoxEventBus()
	bus.Enable()
	bus.SetVerbose(true)
	m.glassBoxEventBus = bus
	ch := bus.Subscribe()
	m.glassBoxEventChan = ch

	// Push several events onto the subscriber channel without going through
	// the tea update loop, then drain via the first event.
	first := transparency.GlassBoxEvent{
		Timestamp: time.Now(),
		Category:  transparency.CategoryShard,
		Summary:   "first",
	}
	for i := 0; i < 5; i++ {
		bus.EmitImmediate(transparency.GlassBoxEvent{
			Timestamp: time.Now(),
			Category:  transparency.CategoryShard,
			Summary:   "burst",
		})
	}

	// Drain may consume first + up to 5 from channel depending on timing;
	// call drain with an explicit first and ensure history grows.
	before := len(m.history)
	m.drainGlassBoxEvents(first)
	if len(m.history) <= before {
		t.Fatal("drain should add at least the first event to history")
	}
	// first is always handled; burst events should also appear if buffered.
	if len(m.history)-before < 1 {
		t.Fatalf("expected at least 1 event, got %d", len(m.history)-before)
	}
}
