package usage

import (
	"codenerd/internal/types"
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

	// Bare string keys on purpose. This test simulates a foreign package
	// writing these literals into the context -- which is precisely the
	// collision that typed keys exist to prevent, and precisely what the
	// tracker must survive. Using typed keys here would test nothing.
	ctx := context.Background()
	//lint:ignore SA1029 simulating a foreign caller's string-keyed collision is the point of this test
	ctx = context.WithValue(ctx, "shard_type", 42) // int, not string
	//lint:ignore SA1029 see above
	ctx = context.WithValue(ctx, "shard_name", []string{"x"}) // slice, not string
	//lint:ignore SA1029 see above
	ctx = context.WithValue(ctx, "session_id", struct{}{}) // struct, not string

	// Before the fix this panicked on the first unchecked val.(string).
	tracker.Record(ctx, Report{Model: "gpt-4", Provider: "openai", Operation: "chat", Usage: types.UsageMetadata{InputTokens: 10, OutputTokens: 5}})

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
	if got := FromContext(context.Background()); got != nil {
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
	RecordFromContext(ctx, Report{Model: "gpt-4", Provider: "openai", Operation: "chat", Usage: types.UsageMetadata{InputTokens: 10, OutputTokens: 20}})

	// 2. Context with a tracker: should record the usage.
	tracker, err := NewTracker(t.TempDir())
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}

	ctxWithTracker := NewContext(ctx, tracker)
	RecordFromContext(ctxWithTracker, Report{Model: "gpt-4", Provider: "openai", Operation: "chat", Usage: types.UsageMetadata{InputTokens: 10, OutputTokens: 20}})

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

func TestCredentialUsagePersistsAndMergesAcrossWriters(t *testing.T) {
	dir := t.TempDir()
	first, err := NewTracker(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewTracker(dir)
	if err != nil {
		t.Fatal(err)
	}
	first.Record(WithCredentialAlias(context.Background(), "anthropic/alpha"), Report{Model: "model", Provider: "anthropic", Operation: "chat", Usage: types.UsageMetadata{InputTokens: 3, OutputTokens: 2}})
	second.Record(WithCredentialAlias(context.Background(), "anthropic/beta"), Report{Model: "model", Provider: "anthropic", Operation: "chat", Usage: types.UsageMetadata{InputTokens: 4, OutputTokens: 1}})
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	loaded, err := NewTracker(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer loaded.Close()
	stats := loaded.Stats()
	if stats.ByCredential["anthropic/alpha"].Total != 5 || stats.ByCredential["anthropic/beta"].Total != 5 || stats.TotalProject.Total != 10 {
		t.Fatalf("credential usage merge lost counts: %+v", stats)
	}
}

// Cache sub-counts ride along as part of Input: they are aggregated for
// visibility and priced at cache rates, but never double-counted in Total.
func TestTrackDetailedFromContext_CacheSubCounts(t *testing.T) {
	tracker, err := NewTracker(t.TempDir())
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	ctx := NewContext(context.Background(), tracker)
	ctx = WithSessionID(ctx, "session-42")
	// 1000 input of which 600 cache-read and 100 cache-write, 100 output,
	// on claude-opus-5-5 (input 4.00, output 20.00, read 0.40, write 5.00).
	RecordFromContext(ctx, Report{Model: "anthropic/claude-opus-5-5", Provider: "anthropic", Operation: "chat", Usage: types.UsageMetadata{InputTokens: 1000, OutputTokens: 100, CachedContentTokens: 600, CacheWriteTokens: 100}})

	stats := tracker.Stats()
	tot := stats.TotalProject
	if tot.Input != 1000 || tot.Output != 100 || tot.Total != 1100 {
		t.Errorf("totals = %+v, want input 1000 output 100 total 1100 (cache not double-counted)", tot)
	}
	if tot.CacheRead != 600 || tot.CacheWrite != 100 {
		t.Errorf("cache counts = read %d write %d, want 600/100", tot.CacheRead, tot.CacheWrite)
	}
	// 300 uncached*4 + 600 read*0.4 + 100 write*5 + 100 out*20, per MTok.
	wantCost := (300*4.00 + 600*0.40 + 100*5.00 + 100*20.00) / 1e6
	if diff := tot.Cost - wantCost; diff > 1e-12 || diff < -1e-12 {
		t.Errorf("cost = %v, want %v", tot.Cost, wantCost)
	}
	if got := stats.ByModel["anthropic/claude-opus-5-5"].CacheRead; got != 600 {
		t.Errorf("ByModel cache read = %d, want 600", got)
	}
	if got := stats.BySession["session-42"].Input; got != 1000 {
		t.Errorf("BySession[session-42].Input = %d, want 1000", got)
	}
}
