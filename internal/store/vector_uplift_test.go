package store

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"unicode/utf8"
)

func openVectorTestStore(t *testing.T) *LocalStore {
	t.Helper()
	s, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	s.SetEmbeddingEngine(&MockEmbeddingEngine{})
	return s
}

func vecIndexCount(t *testing.T, s *LocalStore) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM vec_index").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// ANN reads vec_index, not vectors: a metadata delete that forgets the index
// leaves ghosts that keep surfacing in search results.
func TestVectorDelete_PurgesANNGhosts(t *testing.T) {
	s := openVectorTestStore(t)
	ctx := context.Background()
	if err := s.waitForVecBackfill(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.StoreVectorWithEmbedding(ctx, "alpha document", map[string]any{"kind": "keep"}); err != nil {
		t.Fatal(err)
	}
	if err := s.StoreVectorWithEmbedding(ctx, "beta document", map[string]any{"kind": "drop"}); err != nil {
		t.Fatal(err)
	}
	deleted, err := s.DeleteVectorsByMetadata("kind", "drop")
	if err != nil {
		t.Fatalf("DeleteVectorsByMetadata: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	results, err := s.VectorRecallSemantic(ctx, "document", 10)
	if err != nil {
		t.Fatalf("VectorRecallSemantic: %v", err)
	}
	if len(results) != 1 || results[0].Content != "alpha document" {
		t.Fatalf("results = %+v, want only the survivor", results)
	}
}

// Force re-embed must replace vec_index rows by rowid. The old force loop
// omitted rowid, so every pass appended stale twins and the index grew
// without bound while ANN scored dead embeddings.
func TestVectorForceReembed_ReplacesIndexRows(t *testing.T) {
	s := openVectorTestStore(t)
	ctx := context.Background()
	if err := s.waitForVecBackfill(ctx); err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{"one", "two", "three"} {
		if err := s.StoreVectorWithEmbedding(ctx, c, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.ReembedAllVectorsForce(ctx); err != nil {
		t.Fatalf("first force re-embed: %v", err)
	}
	if _, err := s.ReembedAllVectorsForce(ctx); err != nil {
		t.Fatalf("second force re-embed: %v", err)
	}
	if got := vecIndexCount(t, s); got != 3 {
		t.Errorf("vec_index rows = %d after two force passes, want 3", got)
	}
}

// Filtered brute-force search must find numeric metadata values. The old LIKE
// prefilter ('%"n":"5"%') never matches a JSON number, so numeric rows
// vanished from brute-force results that ANN found.
func TestVectorBruteForceFiltered_FindsNumericMetadata(t *testing.T) {
	s, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	s.SetEmbeddingEngine(&MockEmbeddingEngine{})
	ctx := context.Background()
	if err := s.waitForVecBackfill(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.StoreVectorWithEmbedding(ctx, "numeric row", map[string]any{"n": 5}); err != nil {
		t.Fatal(err)
	}
	// Force the brute-force path: drop the ANN index and mark it absent.
	s.mu.Lock()
	if _, err := s.db.Exec("DROP TABLE vec_index"); err != nil {
		s.mu.Unlock()
		t.Fatal(err)
	}
	s.vectorExt = false
	s.mu.Unlock()

	results, err := s.VectorRecallSemanticFiltered(ctx, "numeric", 10, "n", 5)
	if err != nil {
		t.Fatalf("VectorRecallSemanticFiltered: %v", err)
	}
	if len(results) != 1 || results[0].Content != "numeric row" {
		t.Fatalf("results = %+v, want the numeric row", results)
	}
}

// Trace ANN sync is incremental: syncing one trace must not wipe the rest of
// the index. The old ensure dropped and recreated the table on every call —
// every 45s reflection cycle and every force batch.
func TestTraceVecSync_IsIncremental(t *testing.T) {
	s := openVectorTestStore(t)
	if err := s.ensureTraceVecTable(4); err != nil {
		t.Fatalf("ensureTraceVecTable: %v", err)
	}
	seed := encodeFloat32Slice([]float32{1, 0, 0, 0})
	if _, err := s.db.Exec("INSERT INTO reasoning_traces_vec (trace_id, embedding) VALUES (?, ?)", "trace-keep", seed); err != nil {
		t.Fatal(err)
	}
	err := s.syncTraceVectorIndex([]TraceEmbeddingUpdate{
		{ID: "trace-new", Embedding: seed},
	}, 4)
	if err != nil {
		t.Fatalf("syncTraceVectorIndex: %v", err)
	}
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM reasoning_traces_vec").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("reasoning_traces_vec rows = %d, want 2 (kept + new)", n)
	}
}

// Same incrementality contract for the learning ANN index.
func TestLearningVecSync_IsIncremental(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := ensureLearningVecTable(db, 4); err != nil {
		t.Fatalf("ensureLearningVecTable: %v", err)
	}
	seed := encodeFloat32Slice([]float32{1, 0, 0, 0})
	if _, err := db.Exec("INSERT INTO learnings_vec (learning_id, embedding) VALUES (?, ?)", 7, seed); err != nil {
		t.Fatal(err)
	}
	err = syncLearningVectorIndex(db, []LearningEmbeddingUpdate{
		{ID: 8, Embedding: seed},
	}, 4)
	if err != nil {
		t.Fatalf("syncLearningVectorIndex: %v", err)
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM learnings_vec").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("learnings_vec rows = %d, want 2 (kept + new)", n)
	}
}

// A re-embed issued immediately after SetEmbeddingEngine must converge with
// the background backfill instead of racing it: final index holds exactly
// one row per vector.
func TestVectorBackfill_ReembedConverges(t *testing.T) {
	s, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	// Keyword rows with hand-written JSON embeddings: backfill has real work.
	for _, c := range []string{"backfill a", "backfill b"} {
		if _, err := s.db.Exec(
			"INSERT INTO vectors (content, embedding, metadata) VALUES (?, ?, ?)",
			c, `[0.1,0.2,0.3,0.4]`, `{}`,
		); err != nil {
			t.Fatal(err)
		}
	}
	s.SetEmbeddingEngine(&MockEmbeddingEngine{})
	ctx := context.Background()
	if _, err := s.ReembedAllVectorsForce(ctx); err != nil {
		t.Fatalf("ReembedAllVectorsForce: %v", err)
	}
	if got := vecIndexCount(t, s); got != 2 {
		t.Errorf("vec_index rows = %d, want exactly 2", got)
	}
}

// Stats on a broken table must error, not report the zeros of a healthy
// empty store. See LearningStore.GetStats for the same contract.
func TestVectorStats_BrokenTableIsReported(t *testing.T) {
	s, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if _, err := s.db.Exec("DROP TABLE vectors"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetVectorStats(); err == nil {
		t.Error("expected an error for a missing vectors table, got nil")
	}
}

// Atom text truncation must not split a multi-byte rune.
func TestTextForAtomEmbedding_RuneSafe(t *testing.T) {
	out := textForAtomEmbedding("", strings.Repeat("é", 600))
	if len([]rune(out)) != 500 {
		t.Errorf("truncated to %d runes, want 500", len([]rune(out)))
	}
	if !utf8.ValidString(out) {
		t.Error("truncated text is not valid UTF-8")
	}
}
