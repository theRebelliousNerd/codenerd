package usage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// =============================================================================
// TokenCounts - extended
// =============================================================================

func TestTokenCounts_Add_WhenMultipleSequentialAdds_ShouldAccumulate(t *testing.T) {
	t.Parallel()
	tc := TokenCounts{}
	for i := 0; i < 10; i++ {
		tc.Add(100, 50)
	}
	if tc.Input != 1000 {
		t.Errorf("Input = %d, want 1000", tc.Input)
	}
	if tc.Output != 500 {
		t.Errorf("Output = %d, want 500", tc.Output)
	}
	if tc.Total != 1500 {
		t.Errorf("Total = %d, want 1500", tc.Total)
	}
}

// =============================================================================
// Tracker.Track - comprehensive
// =============================================================================

func TestTracker_Track_WhenMultipleProviders_ShouldSegregateByProvider(t *testing.T) {
	ws := t.TempDir()
	tracker, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	tracker.dirty = true // prevent autosave

	ctx := WithShardContext(context.Background(), "coder-1", "coder", "sess-1")
	tracker.Track(ctx, "gpt-4", "openai", 100, 50, "chat")
	tracker.Track(ctx, "claude-3", "anthropic", 200, 100, "chat")
	tracker.Track(ctx, "gpt-4", "openai", 50, 25, "embedding")

	stats := tracker.Stats()

	if stats.ByProvider["openai"].Total != 225 {
		t.Errorf("ByProvider[openai].Total = %d, want 225", stats.ByProvider["openai"].Total)
	}
	if stats.ByProvider["anthropic"].Total != 300 {
		t.Errorf("ByProvider[anthropic].Total = %d, want 300", stats.ByProvider["anthropic"].Total)
	}
}

func TestTracker_Track_WhenMultipleModels_ShouldSegregateByModel(t *testing.T) {
	ws := t.TempDir()
	tracker, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	tracker.dirty = true

	ctx := WithShardContext(context.Background(), "coder-1", "coder", "sess-1")
	tracker.Track(ctx, "gpt-4", "openai", 100, 50, "chat")
	tracker.Track(ctx, "gpt-3.5", "openai", 80, 40, "chat")

	stats := tracker.Stats()
	if stats.ByModel["gpt-4"].Total != 150 {
		t.Errorf("ByModel[gpt-4].Total = %d, want 150", stats.ByModel["gpt-4"].Total)
	}
	if stats.ByModel["gpt-3.5"].Total != 120 {
		t.Errorf("ByModel[gpt-3.5].Total = %d, want 120", stats.ByModel["gpt-3.5"].Total)
	}
}

func TestTracker_Track_WhenMultipleOperations_ShouldSegregateByOperation(t *testing.T) {
	ws := t.TempDir()
	tracker, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	tracker.dirty = true

	ctx := WithShardContext(context.Background(), "coder-1", "coder", "sess-1")
	tracker.Track(ctx, "gpt-4", "openai", 100, 50, "chat")
	tracker.Track(ctx, "gpt-4", "openai", 10, 5, "embedding")

	stats := tracker.Stats()
	if stats.ByOperation["chat"].Total != 150 {
		t.Errorf("ByOperation[chat].Total = %d, want 150", stats.ByOperation["chat"].Total)
	}
	if stats.ByOperation["embedding"].Total != 15 {
		t.Errorf("ByOperation[embedding].Total = %d, want 15", stats.ByOperation["embedding"].Total)
	}
}

func TestTracker_Track_WhenNoShardContext_ShouldUseUnknown(t *testing.T) {
	ws := t.TempDir()
	tracker, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	tracker.dirty = true

	// Plain context without shard metadata
	ctx := context.Background()
	tracker.Track(ctx, "gpt-4", "openai", 10, 5, "chat")

	stats := tracker.Stats()
	if stats.ByShardType["unknown"].Total != 15 {
		t.Errorf("ByShardType[unknown].Total = %d, want 15", stats.ByShardType["unknown"].Total)
	}
	if stats.BySession["unknown"].Total != 15 {
		t.Errorf("BySession[unknown].Total = %d, want 15", stats.BySession["unknown"].Total)
	}
}

func TestTracker_Track_WhenMultipleSessions_ShouldSegregateBySession(t *testing.T) {
	ws := t.TempDir()
	tracker, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	tracker.dirty = true

	ctx1 := WithShardContext(context.Background(), "c1", "coder", "session-A")
	ctx2 := WithShardContext(context.Background(), "c2", "coder", "session-B")

	tracker.Track(ctx1, "gpt-4", "openai", 100, 50, "chat")
	tracker.Track(ctx2, "gpt-4", "openai", 200, 100, "chat")

	stats := tracker.Stats()
	if stats.BySession["session-A"].Total != 150 {
		t.Errorf("BySession[session-A].Total = %d, want 150", stats.BySession["session-A"].Total)
	}
	if stats.BySession["session-B"].Total != 300 {
		t.Errorf("BySession[session-B].Total = %d, want 300", stats.BySession["session-B"].Total)
	}
}

// =============================================================================
// Tracker.Save / Load round-trip
// =============================================================================

func TestTracker_SaveAndReload_ShouldPreserveData(t *testing.T) {
	ws := t.TempDir()
	tracker, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	tracker.dirty = true

	ctx := WithShardContext(context.Background(), "c1", "coder", "sess-1")
	tracker.Track(ctx, "gpt-4", "openai", 100, 50, "chat")
	tracker.Track(ctx, "claude-3", "anthropic", 200, 100, "embedding")

	if err := tracker.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Create a new tracker from the same workspace
	tracker2, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("NewTracker2: %v", err)
	}

	stats := tracker2.Stats()
	if stats.TotalProject.Total != 450 {
		t.Errorf("TotalProject.Total after reload = %d, want 450", stats.TotalProject.Total)
	}
	if stats.ByProvider["openai"].Total != 150 {
		t.Errorf("ByProvider[openai] after reload = %d, want 150", stats.ByProvider["openai"].Total)
	}
}

// =============================================================================
// Tracker.Stats - copy safety
// =============================================================================

func TestTracker_Stats_ShouldReturnCopy(t *testing.T) {
	ws := t.TempDir()
	tracker, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	tracker.dirty = true

	ctx := WithShardContext(context.Background(), "c1", "coder", "sess-1")
	tracker.Track(ctx, "gpt-4", "openai", 100, 50, "chat")

	stats := tracker.Stats()

	// Modify the returned copy
	stats.ByProvider["openai"] = TokenCounts{Input: 999, Output: 999, Total: 999}

	// Original should not be affected
	origStats := tracker.Stats()
	if origStats.ByProvider["openai"].Total == 999 {
		t.Error("Stats() should return a copy, not a reference to internal data")
	}
}

// =============================================================================
// Context helpers
// =============================================================================

func TestFromContext_WhenNoTracker_ShouldReturnNil(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if FromContext(ctx) != nil {
		t.Error("expected nil from context without tracker")
	}
}

func TestWithShardContext_ShouldSetAllKeys(t *testing.T) {
	t.Parallel()
	ctx := WithShardContext(context.Background(), "coder-1", "coder", "sess-123")

	if val := ctx.Value("shard_name"); val != "coder-1" {
		t.Errorf("shard_name = %v, want coder-1", val)
	}
	if val := ctx.Value("shard_type"); val != "coder" {
		t.Errorf("shard_type = %v, want coder", val)
	}
	if val := ctx.Value("session_id"); val != "sess-123" {
		t.Errorf("session_id = %v, want sess-123", val)
	}
}

// =============================================================================
// UsageData JSON serialization
// =============================================================================

func TestUsageData_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	data := UsageData{
		Version: "1.0",
		Aggregate: AggregatedStats{
			TotalProject: TokenCounts{Input: 100, Output: 50, Total: 150},
			ByProvider:   map[string]TokenCounts{"openai": {Input: 100, Output: 50, Total: 150}},
			ByModel:      map[string]TokenCounts{"gpt-4": {Input: 100, Output: 50, Total: 150}},
			ByShardType:  map[string]TokenCounts{"coder": {Input: 100, Output: 50, Total: 150}},
			ByOperation:  map[string]TokenCounts{"chat": {Input: 100, Output: 50, Total: 150}},
			BySession:    map[string]TokenCounts{"sess-1": {Input: 100, Output: 50, Total: 150}},
		},
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded UsageData
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Version != "1.0" {
		t.Errorf("Version = %q, want 1.0", decoded.Version)
	}
	if decoded.Aggregate.TotalProject.Total != 150 {
		t.Errorf("TotalProject.Total = %d, want 150", decoded.Aggregate.TotalProject.Total)
	}
}

// =============================================================================
// Tracker edge case: corrupt file on disk
// =============================================================================

func TestTracker_NewTracker_WhenCorruptFileExists_ShouldStillCreateTracker(t *testing.T) {
	ws := t.TempDir()
	nerdDir := filepath.Join(ws, ".nerd")
	if err := os.MkdirAll(nerdDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Write corrupt JSON
	if err := os.WriteFile(filepath.Join(nerdDir, "usage.json"), []byte("{{{corrupt"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	tracker, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("expected no error (corrupt file is handled), got %v", err)
	}
	if tracker == nil {
		t.Fatal("expected non-nil tracker")
	}

	// Should have initialized maps despite corrupt load
	if tracker.data.Aggregate.ByProvider == nil {
		t.Error("ByProvider should be initialized even with corrupt file")
	}
}

// =============================================================================
// copyTokenCountsMap helper
// =============================================================================

func TestCopyTokenCountsMap_WhenNil_ShouldReturnNil(t *testing.T) {
	t.Parallel()
	result := copyTokenCountsMap(nil)
	if result != nil {
		t.Error("expected nil for nil input")
	}
}

func TestCopyTokenCountsMap_WhenNonEmpty_ShouldReturnDistinctCopy(t *testing.T) {
	t.Parallel()
	src := map[string]TokenCounts{
		"a": {Input: 10, Output: 20, Total: 30},
		"b": {Input: 40, Output: 50, Total: 90},
	}

	dst := copyTokenCountsMap(src)
	if len(dst) != 2 {
		t.Errorf("expected 2 entries, got %d", len(dst))
	}

	// Modify dst, src should be unaffected
	dst["a"] = TokenCounts{Input: 999}
	if src["a"].Input == 999 {
		t.Error("modifying copy should not affect source")
	}
}

// =============================================================================
// addToMap helper
// =============================================================================

func TestAddToMap_WhenNewKey_ShouldCreateEntry(t *testing.T) {
	t.Parallel()
	m := make(map[string]TokenCounts)
	addToMap(m, "new", 10, 20)

	if m["new"].Input != 10 || m["new"].Output != 20 || m["new"].Total != 30 {
		t.Errorf("expected {10 20 30}, got %+v", m["new"])
	}
}

func TestAddToMap_WhenExistingKey_ShouldAccumulate(t *testing.T) {
	t.Parallel()
	m := make(map[string]TokenCounts)
	addToMap(m, "key", 10, 20)
	addToMap(m, "key", 5, 3)

	if m["key"].Input != 15 || m["key"].Output != 23 || m["key"].Total != 38 {
		t.Errorf("expected {15 23 38}, got %+v", m["key"])
	}
}
