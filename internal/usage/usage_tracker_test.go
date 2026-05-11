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

func TestNewTracker_Success(t *testing.T) {
	ws := t.TempDir()
	tracker, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("NewTracker failed: %v", err)
	}

	if tracker == nil {
		t.Fatalf("tracker is nil")
	}

	// Ensure maps are initialized
	if tracker.data.Aggregate.ByProvider == nil {
		t.Errorf("ByProvider map not initialized")
	}
	if tracker.data.Aggregate.ByModel == nil {
		t.Errorf("ByModel map not initialized")
	}
	if tracker.data.Aggregate.ByShardType == nil {
		t.Errorf("ByShardType map not initialized")
	}
	if tracker.data.Aggregate.ByOperation == nil {
		t.Errorf("ByOperation map not initialized")
	}
	if tracker.data.Aggregate.BySession == nil {
		t.Errorf("BySession map not initialized")
	}

	// Ensure .nerd dir exists
	nerdDir := filepath.Join(ws, ".nerd")
	info, err := os.Stat(nerdDir)
	if err != nil {
		t.Errorf(".nerd dir not created: %v", err)
	} else if !info.IsDir() {
		t.Errorf(".nerd is not a directory")
	}
}

func TestNewTracker_MkdirError(t *testing.T) {
	ws := t.TempDir()
	nerdPath := filepath.Join(ws, ".nerd")

	// Create a file where the directory should be to force MkdirAll to fail
	if err := os.WriteFile(nerdPath, []byte("not a dir"), 0644); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	tracker, err := NewTracker(ws)
	if err == nil {
		t.Errorf("expected error when MkdirAll fails, got nil")
	}
	if tracker != nil {
		t.Errorf("expected nil tracker on error, got %v", tracker)
	}
}

func TestNewTracker_LoadsExistingData(t *testing.T) {
	ws := t.TempDir()
	nerdDir := filepath.Join(ws, ".nerd")
	if err := os.MkdirAll(nerdDir, 0755); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	existingData := []byte(`{"version":"1.0","aggregate":{"total_project":{"input":100,"output":50,"total":150}}}`)
	if err := os.WriteFile(filepath.Join(nerdDir, "usage.json"), existingData, 0644); err != nil {
		t.Fatalf("failed to write existing data: %v", err)
	}

	tracker, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("NewTracker failed: %v", err)
	}

	if tracker.data.Aggregate.TotalProject.Input != 100 {
		t.Errorf("expected input to be 100, got %d", tracker.data.Aggregate.TotalProject.Input)
	}
}

func TestNewTracker_CorruptData(t *testing.T) {
	ws := t.TempDir()
	nerdDir := filepath.Join(ws, ".nerd")
	if err := os.MkdirAll(nerdDir, 0755); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Write invalid JSON
	if err := os.WriteFile(filepath.Join(nerdDir, "usage.json"), []byte("{bad-json"), 0644); err != nil {
		t.Fatalf("failed to write corrupt data: %v", err)
	}

	// NewTracker should swallow the json parse error and return a valid empty tracker
	tracker, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("NewTracker failed on corrupt data, expected it to swallow error: %v", err)
	}

	if tracker == nil {
		t.Fatalf("tracker is nil")
	}

	// Should still have maps initialized
	if tracker.data.Aggregate.ByProvider == nil {
		t.Errorf("ByProvider map not initialized after corrupt data load")
	}
}
