package session

import (
	"context"
	"testing"

	"codenerd/internal/usage"
)

// turn_cost is read from the tracker carried by the turn context. The campaign
// path never tagged its context, so every spawned-shard turn logged
// prompt=0 completion=0 even after the LLM adapter started metering
// (campaign 5a2f4c8d, 2026-09-04). The executor now owns the tracker and tags
// its own context.
func TestExecutorMeteredContext_UsesOwnedTracker(t *testing.T) {
	tr, err := usage.NewTracker(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e := &Executor{kernel: &MockKernel{}, config: DefaultExecutorConfig()}
	e.SetSessionID("s1")
	e.SetUsageTracker(tr)

	tr.Track(usage.WithSessionID(context.Background(), "s1"), "m", "meta", 100, 7, "chat")

	ctx := e.meteredContext(usage.WithSessionID(context.Background(), "s1"))
	got := snapshotTurnUsage(ctx, "s1")
	if got.prompt != 100 || got.completion != 7 {
		t.Fatalf("snapshotTurnUsage = %+v, want prompt 100 completion 7", got)
	}
}

func TestExecutorMeteredContext_KeepsExistingTracker(t *testing.T) {
	owned, _ := usage.NewTracker(t.TempDir())
	other, _ := usage.NewTracker(t.TempDir())
	e := &Executor{kernel: &MockKernel{}, config: DefaultExecutorConfig()}
	e.SetUsageTracker(owned)

	ctx := e.meteredContext(usage.NewContext(context.Background(), other))
	if usage.FromContext(ctx) != other {
		t.Fatal("a tracker already on the context must win over the executor's own")
	}
}

func TestExecutorMeteredContext_NilTrackerLeavesContext(t *testing.T) {
	e := &Executor{kernel: &MockKernel{}, config: DefaultExecutorConfig()}
	ctx := context.Background()
	if e.meteredContext(ctx) != ctx {
		t.Fatal("with no tracker the context must be returned unchanged")
	}
}

func TestExecutorCloneForTask_CopiesUsageTracker(t *testing.T) {
	tr, _ := usage.NewTracker(t.TempDir())
	e := &Executor{kernel: &MockKernel{}, config: DefaultExecutorConfig()}
	e.SetUsageTracker(tr)
	if e.CloneForTask().usageTracker != tr {
		t.Fatal("CloneForTask must carry the usage tracker to the clone")
	}
}
