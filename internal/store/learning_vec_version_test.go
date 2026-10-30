package store

import (
	"testing"
)

// The learning index held only a learning id beside its bytes, so a canonical
// row re-embedded into a new space -- with the index sync after it failing --
// was ranked by its old bytes under its new metadata. Index rows now carry the
// model, task and handle hash their bytes came from, recall ranks a row only
// while they equal the canonical row's, and an unversioned index is rebuilt
// from the canonical rows.
func TestLearningIndex_AStaleRowIsNotRankedAndAnUnversionedIndexIsRebuilt(t *testing.T) {
	ls, err := NewLearningStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ls.Close()
	if err := ls.Save("coder", "learned_pattern", []any{"vectored fact"}, "c"); err != nil {
		t.Fatal(err)
	}
	db, err := ls.getDB("coder")
	if err != nil {
		t.Fatal(err)
	}
	// The pre-versioning index, holding bytes that agree with nothing.
	if _, err := db.Exec(`CREATE VIRTUAL TABLE learnings_vec USING vec0(embedding float[2], learning_id INTEGER)`); err != nil {
		t.Skipf("sqlite-vec unavailable: %v", err)
	}
	var id int64
	if err := db.QueryRow(`SELECT id FROM learnings`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO learnings_vec (learning_id, embedding) VALUES (?, ?)`, id, encodeFloat32Slice([]float32{1, 0})); err != nil {
		t.Fatal(err)
	}
	// The canonical row: embedded by engine-b along {0, 1}.
	if _, err := db.Exec(`UPDATE learnings SET embedding = ?, embedding_model_id = 'engine-b', embedding_dim = 2, embedding_task = 'RETRIEVAL_DOCUMENT' WHERE id = ?`,
		encodeFloat32Slice([]float32{0, 1}), id); err != nil {
		t.Fatal(err)
	}

	if err := ensureLearningVecTable(db, 2); err != nil {
		t.Fatalf("ensureLearningVecTable: %v", err)
	}
	if versioned, err := learningVecVersioned(db); err != nil || !versioned {
		t.Fatalf("the unversioned index was not rebuilt (versioned=%v, err=%v)", versioned, err)
	}
	query := QueryEmbedding{Vector: []float32{0, 1}, ModelID: "engine-b"}
	hits, err := ls.RecallLearningsByEmbeddingContext(t.Context(), query, 5)
	if err != nil || len(hits) != 1 || hits[0].Score < 0.99 {
		t.Fatalf("the rebuilt index does not rank the canonical bytes: %+v (err %v)", hits, err)
	}

	// The canonical row moves to engine-c; the sync that would follow never
	// runs. Its index row still holds engine-b bytes and is not ranked.
	if _, err := db.Exec(`UPDATE learnings SET embedding = ?, embedding_model_id = 'engine-c' WHERE id = ?`, encodeFloat32Slice([]float32{1, 0}), id); err != nil {
		t.Fatal(err)
	}
	hits, err = ls.RecallLearningsByEmbeddingContext(t.Context(), QueryEmbedding{Vector: []float32{0, 1}, ModelID: "engine-c"}, 5)
	if err != nil || len(hits) != 0 {
		t.Fatalf("a stale index row was ranked under the canonical row's new space: %+v (err %v)", hits, err)
	}
}
