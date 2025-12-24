package transparency

import (
	"strings"
	"testing"
	"time"
)

func TestGlassBoxEventString(t *testing.T) {
	event := GlassBoxEvent{
		Category: CategoryShard,
		Summary:  "started",
		Duration: 1500 * time.Microsecond,
	}
	text := event.String()
	if !strings.Contains(text, "[SHARD]") {
		t.Fatalf("expected shard prefix")
	}
	if !strings.Contains(text, "started") {
		t.Fatalf("expected summary in string")
	}
	if !strings.Contains(text, "ms") {
		t.Fatalf("expected duration in string")
	}
}

func TestValidCategory(t *testing.T) {
	if !ValidCategory("kernel") {
		t.Fatalf("expected kernel to be valid")
	}
	if ValidCategory("unknown") {
		t.Fatalf("expected unknown to be invalid")
	}
}

func TestToolEventBus(t *testing.T) {
	bus := NewToolEventBus()
	ch := bus.Subscribe()
	defer bus.Close()

	bus.Emit(ToolEvent{ToolName: "tool", Result: "ok"})

	select {
	case evt := <-ch:
		if evt.Timestamp.IsZero() {
			t.Fatalf("expected timestamp to be set")
		}
		if evt.ToolName != "tool" {
			t.Fatalf("unexpected tool name")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected tool event")
	}
}
