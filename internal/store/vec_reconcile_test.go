package store

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// newVecStore is a file-backed store with the sqlite-vec index built and its
// backfill settled. Skips when the build carries no sqlite-vec.
func newVecStore(t *testing.T) *LocalStore {
	t.Helper()
	s, err := NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	s.SetEmbeddingEngine(&mockSimpleEngine{})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.waitForVecBackfill(ctx); err != nil {
		t.Fatalf("waitForVecBackfill: %v", err)
	}
	if !s.vectorExt.Load() {
		t.Skip("sqlite-vec not available in this build")
	}
	return s
}

func storeProbes(t *testing.T, s *LocalStore, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := s.StoreVectorWithEmbedding(context.Background(), fmt.Sprintf("probe-%d", i), map[string]any{"i": i}); err != nil {
			t.Fatalf("store %d: %v", i, err)
		}
	}
}

func statOf(t *testing.T, s *LocalStore, key string) (int64, bool) {
	t.Helper()
	stats, err := s.GetStats()
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	v, ok := stats[key]
	return v, ok
}

// Failure injection: a vec_index insert that fails leaves its vectors row and
// logs ANN drift; the drift is measured in GetStats, and ReconcileVecIndex --
// run by the maintenance cycle -- puts the rows back so ANN search finds them.
// Before 2026-09-25 nothing measured the drift and only the next engine
// attach, which rebuilds the whole index, healed it.
func TestReconcileVecIndex_HealsTheDriftAFailedInsertLeaves(t *testing.T) {
	s := newVecStore(t)
	storeProbes(t, s, 2)
	if n, ok := statOf(t, s, StatVecIndexMissing); !ok || n != 0 {
		t.Fatalf("%s = %d (present=%v) on a healthy index, want 0", StatVecIndexMissing, n, ok)
	}

	// Inject the failure: the index is gone, so the next insert into it fails.
	if _, err := s.db.Exec("DROP TABLE vec_index"); err != nil {
		t.Fatal(err)
	}
	if err := s.StoreVectorWithEmbedding(context.Background(), "written-while-the-index-failed", nil); err != nil {
		t.Fatalf("a failed index insert failed the store itself: %v", err)
	}
	// The index comes back empty (what a rebuild that lost its backfill looks like).
	if _, err := s.db.Exec("CREATE VIRTUAL TABLE vec_index USING vec0(embedding float[4], content TEXT, metadata TEXT)"); err != nil {
		t.Fatal(err)
	}

	if n, _ := statOf(t, s, StatVecIndexMissing); n != 3 {
		t.Fatalf("%s = %d after the index lost every row, want 3", StatVecIndexMissing, n)
	}
	stats, err := s.MaintenanceCleanup(MaintenanceConfig{ReconcileVecIndex: true})
	if err != nil {
		t.Fatalf("MaintenanceCleanup: %v", err)
	}
	if stats.VecIndexHealed != 3 {
		t.Fatalf("maintenance healed %d rows, want 3", stats.VecIndexHealed)
	}
	if n, _ := statOf(t, s, StatVecIndexMissing); n != 0 {
		t.Fatalf("%s = %d after reconcile, want 0", StatVecIndexMissing, n)
	}
	var indexed int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM vec_index").Scan(&indexed); err != nil {
		t.Fatal(err)
	}
	if indexed != 3 {
		t.Fatalf("vec_index holds %d rows after reconcile, want 3", indexed)
	}
	if healed, err := s.ReconcileVecIndex(context.Background()); err != nil || healed != 0 {
		t.Fatalf("a second reconcile healed %d (err %v), want 0", healed, err)
	}
}

// A row embedded by an earlier model has another dimension: it is a re-embed's
// job, not drift, and the reconcile must neither count nor index it.
func TestReconcileVecIndex_IgnoresRowsOfAnotherDimension(t *testing.T) {
	s := newVecStore(t)
	if _, err := s.db.Exec("INSERT INTO vectors (content, embedding, metadata) VALUES (?, ?, ?)", "old-model", "[0.1,0.2,0.3]", "{}"); err != nil {
		t.Fatal(err)
	}
	if n, _ := statOf(t, s, StatVecIndexMissing); n != 0 {
		t.Fatalf("a 3-dimension row counted as drift of a 4-dimension index: %d", n)
	}
	if healed, err := s.ReconcileVecIndex(context.Background()); err != nil || healed != 0 {
		t.Fatalf("reconcile indexed %d wrong-dimension rows (err %v)", healed, err)
	}
}

// GetStats counts every tier the store keeps, not the first ten tables, and
// reports the reflection backlog once an engine is attached.
func TestGetStats_CountsEveryTierAndTheReflectionBacklog(t *testing.T) {
	s, err := NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	defer s.Close()

	stats, err := s.GetStats()
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"reasoning_traces", "prompt_atoms", "task_verifications", "review_findings", "archived_facts"} {
		if _, ok := stats[table]; !ok {
			t.Errorf("GetStats has no count for %s", table)
		}
	}
	if _, ok := stats[StatReflectionTraceBacklog]; ok {
		t.Errorf("%s reported with no engine to measure it against", StatReflectionTraceBacklog)
	}

	s.SetEmbeddingEngine(&mockSimpleEngine{})
	if _, ok := statOf(t, s, StatReflectionTraceBacklog); !ok {
		t.Errorf("GetStats has no %s with an engine attached", StatReflectionTraceBacklog)
	}
}
