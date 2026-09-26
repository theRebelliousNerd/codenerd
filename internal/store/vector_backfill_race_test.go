package store

import (
	"context"
	"sync"
	"testing"
	"time"
)

// B3: the vec backfill runs in a background goroutine spawned by
// SetEmbeddingEngine. It must never touch s.vectorExt itself (that unlocked
// read raced init/test writers; CI run 35130570857), and a superseding engine
// swap must stop the older generation instead of letting it write into the
// recreated vec_index.

// The generation token mechanism, pinned deterministically: after a second
// SetEmbeddingEngine the first generation is no longer current.
func TestBackfill_SupersededGenerationNotCurrent(t *testing.T) {
	s, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	defer s.Close()
	// Hold both generations until they are inspected: an empty store's
	// backfill finishes, and clears its token, as soon as it starts.
	hold := make(chan struct{})
	var release sync.Once
	backfillStartHold = hold
	defer func() {
		release.Do(func() { close(hold) })
		backfillStartHold = nil
	}()

	s.SetEmbeddingEngine(&mockSimpleEngine{})
	s.backfillMu.Lock()
	done1 := s.backfillDone
	s.backfillMu.Unlock()
	if done1 == nil {
		t.Fatal("expected a pending backfill generation after SetEmbeddingEngine")
	}

	s.SetEmbeddingEngine(&mockSimpleEngine{})
	if s.isCurrentBackfill(done1) {
		t.Fatal("superseded backfill generation still reports current")
	}
	s.backfillMu.Lock()
	done2 := s.backfillDone
	s.backfillMu.Unlock()
	if !s.isCurrentBackfill(done2) {
		t.Fatal("newest backfill generation does not report current")
	}
	release.Do(func() { close(hold) })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.waitForVecBackfill(ctx); err != nil {
		t.Fatalf("waitForVecBackfill: %v", err)
	}
}

// Engine swap over a live backfill: race-clean, settles, stays queryable.
// initVecIndex drops vec_index on every SetEmbeddingEngine, so the older
// generation must abort on its currency check rather than inserting into the
// recreated table. Run with -race -count=3 or more; overlap is timing-shaped,
// the abort mechanism itself is pinned by SupersededGenerationNotCurrent.
func TestBackfill_EngineSwapRaceClean(t *testing.T) {
	s, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	s.SetEmbeddingEngine(&mockSimpleEngine{})
	const rows = 300
	for i := 0; i < rows; i++ {
		if err := s.StoreVectorWithEmbedding(ctx, "swap-probe-content", map[string]any{"i": i}); err != nil {
			t.Fatalf("store %d: %v", i, err)
		}
	}

	// Swap immediately: the first backfill is still scanning/inserting.
	s.SetEmbeddingEngine(&mockSimpleEngine{})

	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := s.waitForVecBackfill(waitCtx); err != nil {
		t.Fatalf("waitForVecBackfill: %v", err)
	}
	s.backfillMu.Lock()
	pending := s.backfillDone
	s.backfillMu.Unlock()
	if pending != nil {
		t.Fatal("backfillDone still set after generations settled")
	}
	if _, err := s.VectorRecallSemanticFiltered(ctx, "swap-probe", 5, "i", 0); err != nil {
		t.Fatalf("recall after swap: %v", err)
	}
}

// Close during a live backfill must not panic, hang, or race: backfill
// database calls fail cleanly on the closed handle and the generation
// channel still closes.
func TestBackfill_CloseDuringBackfillSafe(t *testing.T) {
	s, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	ctx := context.Background()

	s.SetEmbeddingEngine(&mockSimpleEngine{})
	for i := 0; i < 100; i++ {
		if err := s.StoreVectorWithEmbedding(ctx, "close-probe-content", map[string]any{"i": i}); err != nil {
			t.Fatalf("store %d: %v", i, err)
		}
	}
	s.SetEmbeddingEngine(&mockSimpleEngine{})

	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := s.waitForVecBackfill(waitCtx); err != nil {
		t.Fatalf("backfill did not settle after Close: %v", err)
	}
}
