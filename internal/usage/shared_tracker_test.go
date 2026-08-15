package usage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestShared_WhenCalledTwiceForSameWorkspace_ShouldReturnSameTracker(t *testing.T) {
	ws := t.TempDir()

	a, err := Shared(ws)
	if err != nil {
		t.Fatalf("Shared: %v", err)
	}
	b, err := Shared(ws)
	if err != nil {
		t.Fatalf("Shared: %v", err)
	}
	defer func() { _ = a.Close(); _ = b.Close() }()

	if a != b {
		t.Fatal("Shared handed out two trackers for one workspace; each would clobber the other's usage.json")
	}
}

func TestShared_WhenWorkspacesDiffer_ShouldReturnDistinctTrackers(t *testing.T) {
	a, err := Shared(t.TempDir())
	if err != nil {
		t.Fatalf("Shared: %v", err)
	}
	defer a.Close()
	b, err := Shared(t.TempDir())
	if err != nil {
		t.Fatalf("Shared: %v", err)
	}
	defer b.Close()

	if a == b {
		t.Fatal("distinct workspaces must not share a tracker")
	}
}

func TestShared_WhenOneOwnerCloses_ShouldKeepTrackingForTheOther(t *testing.T) {
	ws := t.TempDir()

	cortex, err := Shared(ws)
	if err != nil {
		t.Fatalf("Shared: %v", err)
	}
	chat, err := Shared(ws)
	if err != nil {
		t.Fatalf("Shared: %v", err)
	}

	ctx := context.Background()
	cortex.Track(ctx, "m", "zai", 10, 5, "chat")

	// The first Close is a handle release, not a shutdown: the other owner is
	// still metering.
	if err := chat.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	cortex.Track(ctx, "m", "zai", 1, 1, "chat")

	if total := cortex.Stats().TotalProject.Total; total != 17 {
		t.Fatalf("total=%d, want 17 — tracking stopped when the first owner closed", total)
	}

	if err := cortex.Close(); err != nil {
		t.Fatalf("final Close: %v", err)
	}

	// The final Close flushes everything both owners recorded.
	data, err := os.ReadFile(filepath.Join(ws, ".nerd", "usage.json"))
	if err != nil {
		t.Fatalf("read usage.json: %v", err)
	}
	var stored UsageData
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("parse usage.json: %v", err)
	}
	if stored.Aggregate.TotalProject.Total != 17 {
		t.Fatalf("persisted total=%d, want 17", stored.Aggregate.TotalProject.Total)
	}
}

func TestShared_AfterFinalClose_ShouldCreateAFreshUsableTracker(t *testing.T) {
	ws := t.TempDir()

	first, err := Shared(ws)
	if err != nil {
		t.Fatalf("Shared: %v", err)
	}
	first.Track(context.Background(), "m", "zai", 3, 4, "chat")
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	second, err := Shared(ws)
	if err != nil {
		t.Fatalf("Shared after close: %v", err)
	}
	defer second.Close()

	if second == first {
		t.Fatal("Shared returned a closed tracker; it would silently drop every Track")
	}
	second.Track(context.Background(), "m", "zai", 1, 1, "chat")
	// The replacement loads what the first owner persisted, so totals accumulate
	// rather than restart.
	if total := second.Stats().TotalProject.Total; total != 9 {
		t.Fatalf("total=%d, want 9", total)
	}
}

func TestShared_WhenCalledConcurrently_ShouldStillYieldOneTracker(t *testing.T) {
	ws := t.TempDir()

	const owners = 16
	var wg sync.WaitGroup
	trackers := make([]*Tracker, owners)
	for i := range owners {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tr, err := Shared(ws)
			if err != nil {
				t.Errorf("Shared: %v", err)
				return
			}
			trackers[i] = tr
		}()
	}
	wg.Wait()

	for _, tr := range trackers {
		if tr == nil {
			t.Fatal("nil tracker")
		}
		if tr != trackers[0] {
			t.Fatal("concurrent Shared produced more than one tracker for one workspace")
		}
	}
	for _, tr := range trackers {
		if err := tr.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}
}

func TestNewTracker_ShouldCloseOnFirstClose(t *testing.T) {
	// A privately constructed tracker has exactly one owner: Close must shut it
	// down, not merely decrement a count.
	tr, err := NewTracker(t.TempDir())
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	if err := tr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	tr.Track(context.Background(), "m", "zai", 5, 5, "chat")
	if total := tr.Stats().TotalProject.Total; total != 0 {
		t.Fatalf("closed tracker recorded %d tokens", total)
	}
}

func TestWithShardContext_ShouldUseTypedKeysInvisibleToStringLookups(t *testing.T) {
	// The typed keys are the point: a package that happens to store its own
	// "shard_name" string in the context must not be able to read, or be read
	// as, shard attribution. Raw-string keys are still honored on read for
	// callers that predate WithShardContext, but nothing writes them.
	ctx := WithShardContext(context.Background(), "planner", "specialist", "sess-1")

	//nolint:staticcheck // deliberately probing the legacy raw-string key.
	if v := ctx.Value("shard_name"); v != nil {
		t.Errorf(`ctx.Value("shard_name")=%v, want nil — WithShardContext must not write raw string keys`, v)
	}

	tr, err := NewTracker(t.TempDir())
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	defer tr.Close()
	tr.Track(ctx, "m", "zai", 2, 3, "chat")

	stats := tr.Stats()
	if stats.ByShardName["planner"].Total != 5 {
		t.Errorf("ByShardName=%v, want planner=5", stats.ByShardName)
	}
	if stats.ByShardType["specialist"].Total != 5 {
		t.Errorf("ByShardType=%v, want specialist=5", stats.ByShardType)
	}
	if stats.BySession["sess-1"].Total != 5 {
		t.Errorf("BySession=%v, want sess-1=5", stats.BySession)
	}
}
