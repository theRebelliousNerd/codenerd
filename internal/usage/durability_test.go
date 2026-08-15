package usage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func newTestTracker(t *testing.T, opts ...Option) (*Tracker, string) {
	t.Helper()
	ws := t.TempDir()
	tr, err := NewTracker(ws, opts...)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	return tr, filepath.Join(ws, ".nerd", "usage.json")
}

// =============================================================================
// Atomic save
// =============================================================================

// TestSave_ShouldLeaveNoTempFiles verifies the temp+rename path cleans up after
// itself, so .nerd never fills with .usage-*.json debris.
func TestSave_ShouldLeaveNoTempFiles(t *testing.T) {
	tr, path := newTestTracker(t)
	ctx := WithShardContext(context.Background(), "coder-1", "coder", "sess-1")

	for i := 0; i < 5; i++ {
		tr.Track(ctx, "gpt-4o", "openai", 100, 50, "chat")
		if err := tr.Save(); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".usage-") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

// TestSave_ShouldProduceParseableFile checks the written file round-trips, which
// is the property the old truncate-in-place write could not guarantee.
func TestSave_ShouldProduceParseableFile(t *testing.T) {
	tr, path := newTestTracker(t)
	ctx := WithShardContext(context.Background(), "coder-1", "coder", "sess-1")
	tr.Track(ctx, "gpt-4o", "openai", 1000, 500, "chat")

	if err := tr.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var data UsageData
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("saved file does not parse: %v", err)
	}
	if data.Aggregate.TotalProject.Total != 1500 {
		t.Errorf("TotalProject.Total = %d, want 1500", data.Aggregate.TotalProject.Total)
	}
}

// TestSave_ShouldReplaceRatherThanAppend guards against a shorter payload
// leaving trailing bytes from a previous longer one — the classic in-place
// truncation bug.
func TestSave_ShouldReplaceRatherThanAppend(t *testing.T) {
	tr, path := newTestTracker(t)
	ctx := WithShardContext(context.Background(), "c", "coder", "sess-1")

	for i := 0; i < 50; i++ {
		tr.Track(ctx, fmt.Sprintf("model-%d", i), "openai", 10, 5, "chat")
	}
	if err := tr.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	big, _ := os.ReadFile(path)

	// Reset to a much smaller payload and save over the top.
	tr.mu.Lock()
	tr.data = newUsageData()
	tr.mu.Unlock()
	if err := tr.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	small, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if len(small) >= len(big) {
		t.Fatalf("expected smaller payload to shrink the file: %d -> %d", len(big), len(small))
	}
	var data UsageData
	if err := json.Unmarshal(small, &data); err != nil {
		t.Fatalf("rewritten file has trailing garbage: %v", err)
	}
}

// TestSave_ShouldReplaceTheInodeRatherThanWriteThrough is the guard that
// actually holds saveLocked to its temp+rename contract.
//
// The three tests above do not: every one of them passes with saveLocked
// reverted to a plain truncating os.WriteFile, because they only ever read the
// file back *after* the write returned, when a truncating write and an atomic
// one produce identical bytes. What separates them is what a concurrent reader
// or a crash sees in the middle, and the only way to observe that from a test
// is through the file's identity.
//
// So: stat before and after, and hold a descriptor open across the save. A
// rename swaps the directory entry to a new inode, which leaves the old one
// intact and still readable through the descriptor. A truncating write reuses
// the inode, so os.SameFile reports the same file and the descriptor sees the
// replacement contents — which in the real failure is a zero-length or
// half-written usage.json and the whole billing history gone.
func TestSave_ShouldReplaceTheInodeRatherThanWriteThrough(t *testing.T) {
	tr, path := newTestTracker(t)
	ctx := WithShardContext(context.Background(), "coder-1", "coder", "sess-1")

	// A large first payload so a truncating rewrite is unambiguously shorter.
	for i := 0; i < 50; i++ {
		tr.Track(ctx, fmt.Sprintf("model-%d", i), "openai", 10, 5, "chat")
	}
	if err := tr.Save(); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat before: %v", err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}

	// Open the current contents and keep the handle across the save.
	oldHandle, err := os.Open(path)
	if err != nil {
		t.Fatalf("open before: %v", err)
	}
	defer oldHandle.Close()

	// Rewrite with a much smaller payload — the shape that makes a truncating
	// write visibly destructive.
	tr.mu.Lock()
	tr.data = newUsageData()
	tr.mu.Unlock()
	if err := tr.Save(); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if os.SameFile(before, after) {
		t.Error("save wrote through the existing usage.json; a torn write would have " +
			"destroyed the only copy of the accounting history")
	}

	survived, err := io.ReadAll(oldHandle)
	if err != nil {
		t.Fatalf("read through the pre-save handle: %v", err)
	}
	if !bytes.Equal(survived, original) {
		t.Errorf("the pre-save contents were mutated underneath an open reader: "+
			"%d bytes before, %d after", len(original), len(survived))
	}
	var previous UsageData
	if err := json.Unmarshal(survived, &previous); err != nil {
		t.Errorf("a reader holding usage.json open across a save saw unparseable JSON: %v", err)
	}
	if previous.Aggregate.TotalProject.Total != 50*15 {
		t.Errorf("previous usage.json lost data during the save: Total = %d, want %d",
			previous.Aggregate.TotalProject.Total, 50*15)
	}
}

// =============================================================================
// Flush / Close
// =============================================================================

// TestClose_ShouldPersistPendingMutations is the regression test for usage lost
// on exit: Track only arms a 5s timer, so without a flush on shutdown the last
// turns of every session vanished.
func TestClose_ShouldPersistPendingMutations(t *testing.T) {
	tr, path := newTestTracker(t)
	ctx := WithShardContext(context.Background(), "coder-1", "coder", "sess-1")
	tr.Track(ctx, "gpt-4o", "openai", 700, 300, "chat")

	// Nothing has hit disk yet: only the debounce timer is armed.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected no file before flush, stat err = %v", err)
	}

	if err := tr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("usage.json missing after Close: %v", err)
	}
	var data UsageData
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if data.Aggregate.TotalProject.Total != 1000 {
		t.Errorf("Total = %d, want 1000", data.Aggregate.TotalProject.Total)
	}
}

// TestClose_ShouldBeIdempotent allows hosts to call Close on several shutdown paths.
func TestClose_ShouldBeIdempotent(t *testing.T) {
	tr, _ := newTestTracker(t)
	tr.Track(context.Background(), "gpt-4o", "openai", 10, 5, "chat")

	for i := 0; i < 3; i++ {
		if err := tr.Close(); err != nil {
			t.Fatalf("Close #%d: %v", i, err)
		}
	}
}

// TestTrack_AfterClose_ShouldBeIgnored keeps a late client callback from
// resurrecting a flushed tracker with data that will never be written.
func TestTrack_AfterClose_ShouldBeIgnored(t *testing.T) {
	tr, _ := newTestTracker(t)
	tr.Track(context.Background(), "gpt-4o", "openai", 100, 50, "chat")
	if err := tr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	tr.Track(context.Background(), "gpt-4o", "openai", 999, 999, "chat")
	if got := tr.Stats().TotalProject.Total; got != 150 {
		t.Errorf("Total = %d, want 150 (post-close Track must be dropped)", got)
	}
}

// TestFlush_WhenClean_ShouldBeNoOp avoids rewriting the file on every idle tick.
func TestFlush_WhenClean_ShouldBeNoOp(t *testing.T) {
	tr, path := newTestTracker(t)
	if err := tr.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("clean Flush wrote a file; stat err = %v", err)
	}
}

// TestFlush_ShouldClearDirtyState verifies a second Flush does not rewrite.
func TestFlush_ShouldClearDirtyState(t *testing.T) {
	tr, _ := newTestTracker(t)
	tr.Track(context.Background(), "gpt-4o", "openai", 10, 5, "chat")

	if err := tr.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	tr.mu.Lock()
	dirty := tr.dirty
	timer := tr.autoSaveTimer
	tr.mu.Unlock()

	if dirty {
		t.Error("dirty still set after Flush")
	}
	if timer != nil {
		t.Error("auto-save timer still armed after Flush")
	}
}

// =============================================================================
// Round trip
// =============================================================================

// TestTracker_RoundTrip_ShouldRestoreAggregates covers boot -> track -> close ->
// reload, the integration path the corpus asked for.
func TestTracker_RoundTrip_ShouldRestoreAggregates(t *testing.T) {
	ws := t.TempDir()

	first, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	ctx := WithShardContext(context.Background(), "coder-1", "specialist", "sess-A")
	first.Track(ctx, "claude-sonnet-4", "anthropic", 1_000_000, 200_000, "chat")
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	second, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("reopen NewTracker: %v", err)
	}
	stats := second.Stats()

	if stats.TotalProject.Input != 1_000_000 || stats.TotalProject.Output != 200_000 {
		t.Errorf("restored totals = %+v", stats.TotalProject)
	}
	if stats.ByShardName["coder-1"].Total != 1_200_000 {
		t.Errorf("ByShardName[coder-1] = %+v", stats.ByShardName["coder-1"])
	}
	if stats.BySession["sess-A"].Total != 1_200_000 {
		t.Errorf("BySession[sess-A] = %+v", stats.BySession["sess-A"])
	}
	// 1M input at $3/Mtok + 200k output at $15/Mtok = $3.00 + $3.00.
	if got := stats.TotalProject.Cost; got < 5.99 || got > 6.01 {
		t.Errorf("restored cost = %f, want ~6.00", got)
	}
}

// TestNewTracker_WhenFileCorrupt_ShouldStartEmpty keeps a bad usage.json from
// taking down the session.
func TestNewTracker_WhenFileCorrupt_ShouldStartEmpty(t *testing.T) {
	ws := t.TempDir()
	nerd := filepath.Join(ws, ".nerd")
	if err := os.MkdirAll(nerd, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nerd, "usage.json"), []byte("{not json"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	tr, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("NewTracker should tolerate a corrupt file: %v", err)
	}
	if got := tr.Stats().TotalProject.Total; got != 0 {
		t.Errorf("Total = %d, want 0", got)
	}
	// And the maps must be usable, not nil.
	tr.Track(context.Background(), "gpt-4o", "openai", 5, 5, "chat")
	if got := tr.Stats().TotalProject.Total; got != 10 {
		t.Errorf("Total after Track = %d, want 10", got)
	}
}

// =============================================================================
// Input validation
// =============================================================================

func TestTrack_ShouldRejectNegativeTokens(t *testing.T) {
	tr, _ := newTestTracker(t)
	ctx := WithShardContext(context.Background(), "c", "coder", "s")

	tr.Track(ctx, "gpt-4o", "openai", 100, 50, "chat")
	tr.Track(ctx, "gpt-4o", "openai", -500, 50, "chat")
	tr.Track(ctx, "gpt-4o", "openai", 100, -50, "chat")
	tr.Track(ctx, "gpt-4o", "openai", -1, -1, "chat")

	if got := tr.Stats().TotalProject.Total; got != 150 {
		t.Errorf("Total = %d, want 150 (negatives must be rejected)", got)
	}
}

func TestTrack_ShouldIgnoreZeroTokenCalls(t *testing.T) {
	tr, _ := newTestTracker(t)
	tr.Track(context.Background(), "gpt-4o", "openai", 0, 0, "chat")

	if got := len(tr.Stats().ByModel); got != 0 {
		t.Errorf("ByModel has %d entries, want 0 for a zero-token call", got)
	}
}

// =============================================================================
// Bounds
// =============================================================================

// TestBySession_ShouldBePrunedWhenUnbounded verifies a long-lived workspace
// cannot grow usage.json without limit, and that pruning preserves totals.
func TestBySession_ShouldBePrunedWhenUnbounded(t *testing.T) {
	tr, _ := newTestTracker(t)

	const sessions = maxSessions + 200
	for i := 0; i < sessions; i++ {
		ctx := WithShardContext(context.Background(), "c", "coder", fmt.Sprintf("sess-%04d", i))
		tr.Track(ctx, "gpt-4o", "openai", 10, 10, "chat")
	}

	stats := tr.Stats()
	if len(stats.BySession) > maxSessions {
		t.Errorf("BySession has %d entries, want <= %d", len(stats.BySession), maxSessions)
	}

	// Pruning folds rather than discards, so the per-session sum must still
	// reconcile with the project total.
	var sum int64
	for _, v := range stats.BySession {
		sum += v.Total
	}
	if sum != stats.TotalProject.Total {
		t.Errorf("BySession sum = %d, TotalProject = %d; pruning lost tokens", sum, stats.TotalProject.Total)
	}
}

// TestEvents_ShouldBeOffByDefault documents that aggregates are the default record.
func TestEvents_ShouldBeOffByDefault(t *testing.T) {
	tr, _ := newTestTracker(t)
	tr.Track(context.Background(), "gpt-4o", "openai", 10, 5, "chat")

	if got := tr.Events(); len(got) != 0 {
		t.Errorf("Events() returned %d entries, want 0 without WithEventLog", len(got))
	}
}

// TestEvents_ShouldBeBoundedRing verifies WithEventLog retains recent history
// without unbounded growth.
func TestEvents_ShouldBeBoundedRing(t *testing.T) {
	tr, _ := newTestTracker(t, WithEventLog())
	ctx := WithShardContext(context.Background(), "c", "coder", "s")

	for i := 0; i < maxEvents+250; i++ {
		tr.Track(ctx, "gpt-4o", "openai", 1, 1, fmt.Sprintf("op-%d", i))
	}

	events := tr.Events()
	if len(events) != maxEvents {
		t.Fatalf("Events() = %d, want %d", len(events), maxEvents)
	}
	// The ring must keep the newest, not the oldest.
	last := events[len(events)-1]
	if want := fmt.Sprintf("op-%d", maxEvents+249); last.OperationType != want {
		t.Errorf("newest event op = %q, want %q", last.OperationType, want)
	}
}

// TestEvents_ShouldReturnACopy keeps a caller from mutating tracker state.
func TestEvents_ShouldReturnACopy(t *testing.T) {
	tr, _ := newTestTracker(t, WithEventLog())
	tr.Track(context.Background(), "gpt-4o", "openai", 10, 5, "chat")

	got := tr.Events()
	if len(got) == 0 {
		t.Fatal("expected an event")
	}
	got[0].Model = "MUTATED"

	if again := tr.Events(); again[0].Model == "MUTATED" {
		t.Error("Events() exposed internal storage")
	}
}

// =============================================================================
// Concurrency
// =============================================================================

// TestTracker_ConcurrentTrackAndFlush_ShouldNotRace exercises the lock discipline
// around the debounce timer. Run with -race.
func TestTracker_ConcurrentTrackAndFlush_ShouldNotRace(t *testing.T) {
	tr, _ := newTestTracker(t, WithEventLog())
	var wg sync.WaitGroup

	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			ctx := WithShardContext(context.Background(), fmt.Sprintf("shard-%d", w), "coder", fmt.Sprintf("sess-%d", w))
			for i := 0; i < 200; i++ {
				tr.Track(ctx, "gpt-4o", "openai", 1, 1, "chat")
			}
		}(w)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = tr.Flush()
			_ = tr.Stats()
			_ = tr.Events()
		}
	}()

	wg.Wait()
	if err := tr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if got := tr.Stats().TotalProject.Total; got != 8*200*2 {
		t.Errorf("Total = %d, want %d", got, 8*200*2)
	}
}
