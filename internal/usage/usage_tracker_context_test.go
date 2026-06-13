package usage

import (
	"context"
	"testing"
)

// TestTracker_Track_WhenNonStringContextValues_ShouldNotPanic guards the
// context-metadata extraction in Track. Context values are untyped (any); a
// caller that stores a non-string under these keys must degrade to "unknown"
// rather than panic the tracker via an unchecked type assertion.
func TestTracker_Track_WhenNonStringContextValues_ShouldNotPanic(t *testing.T) {
	tracker, err := NewTracker(t.TempDir())
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	tracker.dirty = true

	ctx := context.Background()
	ctx = context.WithValue(ctx, "shard_type", 42)           // int, not string
	ctx = context.WithValue(ctx, "shard_name", []string{"x"}) // slice, not string
	ctx = context.WithValue(ctx, "session_id", struct{}{})    // struct, not string

	// Before the fix this panicked on the first unchecked val.(string).
	tracker.Track(ctx, "gpt-4", "openai", 10, 5, "chat")

	stats := tracker.Stats()
	if stats.ByShardType["unknown"].Total != 15 {
		t.Errorf("non-string shard_type should fall back to 'unknown', got %+v", stats.ByShardType)
	}
	if stats.BySession["unknown"].Total != 15 {
		t.Errorf("non-string session_id should fall back to 'unknown', got %+v", stats.BySession)
	}
}
