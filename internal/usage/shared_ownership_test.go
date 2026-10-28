package usage

import (
	"codenerd/internal/types"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// These tests own the reference-counting half of Shared: not "do two owners see
// the same aggregates" (shared_tracker_test.go covers that) but "does one
// owner's Close affect only that owner".
//
// The distinction is what the original refs-on-shared-state design got wrong.
// Every owner held the same *Tracker pointer, so a decrement could not be
// attributed to a handle, and `defer tracker.Close()` alongside an explicit
// shutdown — an ordinary, correct-looking pairing — consumed a reference
// belonging to a different subsystem.

func readPersisted(t *testing.T, ws string) AggregatedStats {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(ws, ".nerd", "usage.json"))
	if err != nil {
		t.Fatalf("read usage.json: %v", err)
	}
	var d struct {
		Aggregate AggregatedStats `json:"aggregate"`
	}
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatalf("unmarshal usage.json: %v", err)
	}
	return d.Aggregate
}

// TestShared_WhenOneOwnerClosesTwice_ShouldNotStealThePeersReference is the
// regression test for the refcount bug.
//
// Owner A does what a Go caller does by default: defers a Close and also closes
// explicitly on its own shutdown path. Under the old design the two decrements
// took the count from 2 to 0, the second one ran the final-close branch, and
// owner B — still very much alive — went on calling Track into a closed tracker
// where every call was dropped on the floor. The symptom in production is not a
// crash but a billing ledger that silently stops updating partway through a
// session.
func TestShared_WhenOneOwnerClosesTwice_ShouldNotStealThePeersReference(t *testing.T) {
	ws := t.TempDir()

	a, err := Shared(ws)
	if err != nil {
		t.Fatalf("Shared (owner A): %v", err)
	}
	b, err := Shared(ws)
	if err != nil {
		t.Fatalf("Shared (owner B): %v", err)
	}
	if a.tracker != b.tracker {
		t.Fatal("Shared must hand both owners the same underlying tracker")
	}

	if err := a.Close(); err != nil {
		t.Fatalf("owner A first Close: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("owner A second Close: %v", err)
	}

	b.Record(context.Background(), Report{Model: "m", Provider: "p", Operation: "op", Usage: types.UsageMetadata{InputTokens: 100, OutputTokens: 50}})
	if got := b.Stats().TotalProject.Total; got != 150 {
		t.Fatalf("owner B stopped metering after its peer double-closed: total=%d, want 150", got)
	}

	if err := b.Close(); err != nil {
		t.Fatalf("owner B Close: %v", err)
	}
	if got := readPersisted(t, ws).TotalProject.Total; got != 150 {
		t.Fatalf("persisted total=%d, want 150", got)
	}
}

// TestShared_WhenOwnersCloseInEitherOrder_ShouldPersistEverything checks that
// the *last* handle to close is the one that shuts down, whichever owner that
// turns out to be — the close order between Cortex and the chat model is not
// fixed, and neither ordering may drop the other's tokens.
func TestShared_WhenOwnersCloseInEitherOrder_ShouldPersistEverything(t *testing.T) {
	for _, order := range []string{"A-then-B", "B-then-A"} {
		t.Run(order, func(t *testing.T) {
			ws := t.TempDir()
			a, err := Shared(ws)
			if err != nil {
				t.Fatalf("Shared: %v", err)
			}
			b, err := Shared(ws)
			if err != nil {
				t.Fatalf("Shared: %v", err)
			}

			ctx := context.Background()
			a.Record(ctx, Report{Model: "m", Provider: "p", Operation: "opA", Usage: types.UsageMetadata{InputTokens: 10, OutputTokens: 0}})
			b.Record(ctx, Report{Model: "m", Provider: "p", Operation: "opB", Usage: types.UsageMetadata{InputTokens: 0, OutputTokens: 7}})

			first, second := a, b
			if order == "B-then-A" {
				first, second = b, a
			}
			if err := first.Close(); err != nil {
				t.Fatalf("first Close: %v", err)
			}
			// The survivor must still be metering after its peer let go.
			second.Record(ctx, Report{Model: "m", Provider: "p", Operation: "opC", Usage: types.UsageMetadata{InputTokens: 3, OutputTokens: 0}})
			if err := second.Close(); err != nil {
				t.Fatalf("second Close: %v", err)
			}

			got := readPersisted(t, ws).TotalProject
			if got.Input != 13 || got.Output != 7 {
				t.Fatalf("persisted in=%d out=%d, want 13/7", got.Input, got.Output)
			}
		})
	}
}

// TestShared_WhenAllOwnersClose_ShouldNotLeakTheRegistryEntry keeps the
// process-wide map from pinning a tracker (and its loaded aggregates) for every
// workspace ever opened.
func TestShared_WhenAllOwnersClose_ShouldNotLeakTheRegistryEntry(t *testing.T) {
	sharedMu.Lock()
	before := len(sharedTrackers)
	sharedMu.Unlock()

	ws := t.TempDir()
	a, err := Shared(ws)
	if err != nil {
		t.Fatalf("Shared: %v", err)
	}
	b, err := Shared(ws)
	if err != nil {
		t.Fatalf("Shared: %v", err)
	}
	if err := a.Close(); err != nil {
		t.Fatalf("Close a: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("Close b: %v", err)
	}

	sharedMu.Lock()
	after := len(sharedTrackers)
	sharedMu.Unlock()
	if after != before {
		t.Fatalf("registry leaked an entry: %d before, %d after", before, after)
	}
}

// TestShared_WhenAnOwnerReleases_ShouldFlushBothOwnersWork covers the case
// where one owner never closes at all (a crash, or a shutdown path that misses
// it). The releasing owner's Close is the last chance to get the *other*
// owner's tokens onto disk, because Track only arms a 5s debounce timer.
func TestShared_WhenAnOwnerReleases_ShouldFlushBothOwnersWork(t *testing.T) {
	ws := t.TempDir()
	a, err := Shared(ws)
	if err != nil {
		t.Fatalf("Shared: %v", err)
	}
	b, err := Shared(ws)
	if err != nil {
		t.Fatalf("Shared: %v", err)
	}
	defer b.Close()

	ctx := context.Background()
	a.Record(ctx, Report{Model: "m", Provider: "p", Operation: "opA", Usage: types.UsageMetadata{InputTokens: 1000, OutputTokens: 0}})
	b.Record(ctx, Report{Model: "m", Provider: "p", Operation: "opB", Usage: types.UsageMetadata{InputTokens: 0, OutputTokens: 2000}})

	// Non-final close: a handle release, but it must still flush.
	if err := a.Close(); err != nil {
		t.Fatalf("Close a: %v", err)
	}

	got := readPersisted(t, ws).TotalProject
	if got.Input != 1000 || got.Output != 2000 {
		t.Fatalf("a non-final Close did not flush both owners: in=%d out=%d, want 1000/2000",
			got.Input, got.Output)
	}
}
