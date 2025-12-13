package usage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTracker_TrackAggregatesAndPersists(t *testing.T) {
	ws := t.TempDir()
	tracker, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}

	// Avoid background autosave during the test (debounce uses AfterFunc).
	tracker.dirty = true

	ctx := WithShardContext(context.Background(), "coder-1", "coder", "sess_1")
	tracker.Track(ctx, "glm-4.6", "zai", 10, 5, "chat")
	tracker.Track(ctx, "glm-4.6", "zai", 2, 3, "chat")

	stats := tracker.Stats()
	if stats.TotalProject.Input != 12 || stats.TotalProject.Output != 8 || stats.TotalProject.Total != 20 {
		t.Fatalf("TotalProject=%+v, want input=12 output=8 total=20", stats.TotalProject)
	}
	if got := stats.ByProvider["zai"]; got.Total != 20 {
		t.Fatalf("ByProvider[zai]=%+v, want total=20", got)
	}
	if got := stats.ByModel["glm-4.6"]; got.Total != 20 {
		t.Fatalf("ByModel[glm-4.6]=%+v, want total=20", got)
	}
	if got := stats.ByShardType["coder"]; got.Total != 20 {
		t.Fatalf("ByShardType[coder]=%+v, want total=20", got)
	}
	if got := stats.ByOperation["chat"]; got.Total != 20 {
		t.Fatalf("ByOperation[chat]=%+v, want total=20", got)
	}
	if got := stats.BySession["sess_1"]; got.Total != 20 {
		t.Fatalf("BySession[sess_1]=%+v, want total=20", got)
	}

	if err := tracker.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(ws, ".nerd", "usage.json"))
	if err != nil {
		t.Fatalf("read usage.json: %v", err)
	}
	var persisted UsageData
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("unmarshal usage.json: %v", err)
	}
	if persisted.Aggregate.TotalProject.Total != 20 {
		t.Fatalf("persisted total=%d, want 20", persisted.Aggregate.TotalProject.Total)
	}
}

func TestTracker_ContextHelpers(t *testing.T) {
	ws := t.TempDir()
	tracker, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}

	ctx := NewContext(context.Background(), tracker)
	if got := FromContext(ctx); got == nil {
		t.Fatalf("FromContext returned nil")
	}
	if got := FromContext(ctx); got != tracker {
		t.Fatalf("FromContext mismatch")
	}
}
