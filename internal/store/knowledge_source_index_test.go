package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// Semantic recall reads vec_index, which no trigger can track, and the JIT
// cache keys knowledge on KnowledgeSourceGeneration. Two index-only changes
// left that token alone: a backfill still building the index (recall serves
// a partial index as healthy) and a repair that puts drifted rows back. A
// prompt compiled from the partial or drifted index stayed cached after the
// index recovered (audit knowledge-index-repair-cache-freshness).

// While the backfill is building the index, the knowledge generation is not
// readable, so a compile in that window is not cached as healthy.
func TestKnowledgeSourceGeneration_UnreadableWhileTheIndexIsBuilding(t *testing.T) {
	hold := make(chan struct{})
	backfillStartHold = hold
	s, err := NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		backfillStartHold = nil
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	s.SetEmbeddingEngine(&mockSimpleEngine{})
	backfillStartHold = nil
	if !s.vectorExt.Load() {
		close(hold)
		t.Skip("sqlite-vec not available in this build")
	}
	ctx := context.Background()
	if tok, err := s.KnowledgeSourceGeneration(ctx); err == nil {
		close(hold)
		t.Fatalf("generation %q read while the index backfill is pending: a partial index looked complete", tok)
	}
	close(hold)
	wctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := s.waitForVecBackfill(wctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.KnowledgeSourceGeneration(ctx); err != nil {
		t.Fatalf("generation after the backfill: %v", err)
	}
}

// Re-indexing drifted rows changes what recall returns and nothing a trigger
// sees: the generation has to move with it.
func TestKnowledgeSourceGeneration_MovesWhenTheIndexIsRepaired(t *testing.T) {
	s := newVecStore(t)
	storeProbes(t, s, 2)
	var id int64
	if err := s.db.QueryRow(`SELECT id FROM vectors ORDER BY id LIMIT 1`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DELETE FROM vec_index WHERE rowid = ?`, id); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	drifted, err := s.KnowledgeSourceGeneration(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if healed, err := s.ReconcileVecIndex(ctx); err != nil || healed != 1 {
		t.Fatalf("ReconcileVecIndex = %d, %v; want the one drifted row", healed, err)
	}
	repaired, err := s.KnowledgeSourceGeneration(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if repaired == drifted {
		t.Fatalf("generation %q did not move when the index was repaired", repaired)
	}
	again, err := s.KnowledgeSourceGeneration(ctx)
	if err != nil || again != repaired {
		t.Fatalf("an unchanged store gave %q then %q (%v)", repaired, again, err)
	}
}
