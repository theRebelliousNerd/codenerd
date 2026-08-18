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
	ctx = context.WithValue(ctx, "shard_type", 42)            // int, not string
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

func TestFromContext(t *testing.T) {
	// Test empty context
	ctx := context.Background()
	if got := FromContext(ctx); got != nil {
		t.Errorf("FromContext(empty ctx) = %v, want nil", got)
	}

	// Test nil context
	if got := FromContext(nil); got != nil {
		t.Errorf("FromContext(nil) = %v, want nil", got)
	}

	// Test populated context
	tracker, err := NewTracker(t.TempDir())
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}

	ctx = NewContext(ctx, tracker)
	if got := FromContext(ctx); got != tracker {
		t.Errorf("FromContext(populated ctx) = %v, want %v", got, tracker)
	}
}

func TestWithShardContext(t *testing.T) {
	ctx := context.Background()
	ctx = WithShardContext(ctx, "test-shard", "test-type", "test-session")

	if got := ctx.Value(shardNameKey); got != "test-shard" {
		t.Errorf("shardNameKey = %v, want %v", got, "test-shard")
	}
	if got := ctx.Value(shardTypeKey); got != "test-type" {
		t.Errorf("shardTypeKey = %v, want %v", got, "test-type")
	}
	if got := ctx.Value(sessionIDKey); got != "test-session" {
		t.Errorf("sessionIDKey = %v, want %v", got, "test-session")
	}
}

func TestTrackFromContext(t *testing.T) {
	// 1. Context without a tracker: should be a no-op, shouldn't panic.
	ctx := context.Background()
	TrackFromContext(ctx, "gpt-4", "openai", 10, 20, "chat")

	// 2. Context with a tracker: should record the usage.
	tracker, err := NewTracker(t.TempDir())
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}

	ctxWithTracker := NewContext(ctx, tracker)
	TrackFromContext(ctxWithTracker, "gpt-4", "openai", 10, 20, "chat")

	stats := tracker.Stats()
	if stats.TotalProject.Input != 10 {
		t.Errorf("expected input to be 10, got %d", stats.TotalProject.Input)
	}
	if stats.TotalProject.Output != 20 {
		t.Errorf("expected output to be 20, got %d", stats.TotalProject.Output)
	}
	if stats.TotalProject.Total != 30 {
		t.Errorf("expected total to be 30, got %d", stats.TotalProject.Total)
	}
}
