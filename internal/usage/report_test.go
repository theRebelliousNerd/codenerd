package usage

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"codenerd/internal/types"
)

type recordingObserver struct {
	mu      sync.Mutex
	reports []Report
}

func (o *recordingObserver) Observed(r Report) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.reports = append(o.reports, r)
}

// The tracker and the in-flight observer must receive the same report: before
// the Report type, the observer got input/output only and the broker receipt
// lost every cache count the tracker recorded.
func TestRecordFromContext_DeliversTheSameReportToTrackerAndObserver(t *testing.T) {
	tracker, err := NewTracker(t.TempDir())
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	defer tracker.Close()
	obs := &recordingObserver{}
	ctx := WithObserver(NewContext(context.Background(), tracker), obs)

	want := Report{
		Model: "claude-opus-5-5", Provider: "anthropic", Operation: "chat",
		Usage: types.UsageMetadata{
			InputTokens: 1000, OutputTokens: 300, CachedContentTokens: 600, CacheWriteTokens: 100, ThinkingTokens: 120,
			Known: types.UsageTokens | types.UsageCacheRead | types.UsageCacheWrite | types.UsageReasoning,
		},
	}
	RecordFromContext(ctx, want)

	if len(obs.reports) != 1 || obs.reports[0] != want {
		t.Fatalf("observer got %+v, want exactly %+v", obs.reports, want)
	}
	tot := tracker.Stats().TotalProject
	if tot.CacheRead != 600 || tot.CacheWrite != 100 || tot.Reasoning != 120 {
		t.Fatalf("tracker sub-counts = read %d write %d reasoning %d, want 600/100/120", tot.CacheRead, tot.CacheWrite, tot.Reasoning)
	}
	if tot.Total != 1300 {
		t.Fatalf("Total = %d, want 1300: no sub-count may be added again", tot.Total)
	}
}

func TestRecord_PartialReportIsCountedAndVisible(t *testing.T) {
	tracker, err := NewTracker(t.TempDir(), WithEventLog())
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	defer tracker.Close()
	tracker.Record(context.Background(), Report{
		Model: "m", Provider: "openai", Operation: "chat",
		Usage: types.UsageMetadata{InputTokens: 40, Known: types.UsageTokens, Partial: true},
	})
	if got := tracker.Stats().TotalProject.PartialReports; got != 1 {
		t.Fatalf("PartialReports = %d, want 1", got)
	}
	events := tracker.Events()
	if len(events) != 1 || !events[0].Partial || events[0].Known != types.UsageTokens {
		t.Fatalf("event = %+v, want one partial event with only token totals known", events)
	}
}

// A save that merges with another writer's file used to drop the cache
// sub-counts: the per-field diff and add knew only input, output, total and
// cost, so the first cross-process save erased cache traffic from usage.json.
func TestSave_MergePreservesCacheAndReasoningSubCounts(t *testing.T) {
	ws := t.TempDir()
	tr, err := NewTracker(ws)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	defer tr.Close()
	report := Report{Model: "claude-opus-5-5", Provider: "anthropic", Operation: "chat",
		Usage: types.UsageMetadata{InputTokens: 100, OutputTokens: 50, CachedContentTokens: 60, CacheWriteTokens: 20, ThinkingTokens: 10}}
	tr.Record(context.Background(), report)
	if err := tr.Save(); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	path := filepath.Join(ws, ".nerd", "usage.json")
	ud := readUsageData(t, path)
	addOtherWriterAggregate(&ud.Aggregate)
	writeUsageData(t, path, ud)

	tr.Record(context.Background(), report)
	if err := tr.Save(); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	got := readUsageData(t, path).Aggregate.ByProvider["anthropic"]
	if got.CacheRead != 120 || got.CacheWrite != 40 || got.Reasoning != 20 {
		t.Fatalf("merged anthropic row = %+v, want cache read 120, write 40, reasoning 20", got)
	}
}
