package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"codenerd/internal/embedding"
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

// Recall must upgrade persisted legacy indexes without waiting for a reflection
// update: canonical embeddings can already be current, so sync may never run.
func TestLearningRecall_UpgradesLegacyVectorIndex(t *testing.T) {
	for _, tc := range []struct {
		name     string
		metadata string
	}{
		{name: "unversioned"},
		{name: "missing_model", metadata: ", embedding_task TEXT, handle_hash TEXT"},
		{name: "missing_task", metadata: ", embedding_model_id TEXT, handle_hash TEXT"},
		{name: "missing_hash", metadata: ", embedding_model_id TEXT, embedding_task TEXT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			ls, err := NewLearningStore(base)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := ls.Close(); err != nil {
					t.Error(err)
				}
			})
			if err := ls.Save("coder", "learned_pattern", []any{"persisted vectored fact"}, "c"); err != nil {
				t.Fatal(err)
			}
			db, err := ls.getDB("coder")
			if err != nil {
				t.Fatal(err)
			}
			var vecVersion string
			if err := db.QueryRow("SELECT vec_version()").Scan(&vecVersion); err != nil {
				t.Skipf("sqlite-vec unavailable: %v", err)
			}
			if _, err := db.Exec("CREATE VIRTUAL TABLE learnings_vec USING vec0(embedding float[2], learning_id INTEGER" + tc.metadata + ")"); err != nil {
				t.Fatal(err)
			}
			var id int64
			if err := db.QueryRow("SELECT id FROM learnings").Scan(&id); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`UPDATE learnings SET embedding = ?, embedding_model_id = 'engine-b', embedding_dim = 2, embedding_task = 'RETRIEVAL_DOCUMENT', handle_hash = 'current' WHERE id = ?`, encodeFloat32Slice([]float32{0, 1}), id); err != nil {
				t.Fatal(err)
			}
			// Reopen the persisted shard; do not call ensure or sync in the test.
			if err := ls.Close(); err != nil {
				t.Fatal(err)
			}
			ls, err = NewLearningStore(base)
			if err != nil {
				t.Fatal(err)
			}
			query := QueryEmbedding{Vector: []float32{0, 1}, ModelID: "engine-b"}
			for attempt := 0; attempt < 2; attempt++ {
				hits, err := ls.RecallLearningsByEmbeddingContext(t.Context(), query, 5)
				if err != nil || len(hits) != 1 || hits[0].LearningID != id || hits[0].Score < 0.99 {
					t.Fatalf("recall %d did not recover canonical vector from legacy index: %+v (err %v)", attempt, hits, err)
				}
			}
		})
	}
}

func TestLearningRecall_LegacyIndexMigrationFailure(t *testing.T) {
	for _, tc := range []struct {
		name     string
		canceled bool
	}{
		{name: "read_only"},
		{name: "canceled", canceled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ls, err := NewLearningStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := ls.Close(); err != nil {
					t.Error(err)
				}
			})
			if err := ls.Save("coder", "learned_pattern", []any{"canonical fact"}, "c"); err != nil {
				t.Fatal(err)
			}
			db, err := ls.getDB("coder")
			if err != nil {
				t.Fatal(err)
			}
			db.SetMaxOpenConns(1)
			var vecVersion string
			if err := db.QueryRow("SELECT vec_version()").Scan(&vecVersion); err != nil {
				t.Skipf("sqlite-vec unavailable: %v", err)
			}
			if _, err := db.Exec(`CREATE VIRTUAL TABLE learnings_vec USING vec0(embedding float[2], learning_id INTEGER)`); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.canceled {
				cancel()
			} else if _, err := db.Exec("PRAGMA query_only = ON"); err != nil {
				t.Fatal(err)
			}
			hits, err := ls.recallLearningsInShard(ctx, QueryEmbedding{Vector: []float32{0, 1}, ModelID: "engine-b"}, "coder", 5)
			if err == nil || len(hits) != 0 {
				t.Fatalf("failed migration returned hits or no error: %+v, %v", hits, err)
			}
			if tc.canceled {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation not preserved: %v", err)
				}
			} else if !strings.Contains(err.Error(), "ensure learning vector index for shard") {
				t.Fatalf("migration failure lost its context: %v", err)
			}
			if !tableExists(db, "learnings_vec") {
				t.Fatal("failed migration removed the legacy index")
			}
			var n int
			if err := db.QueryRow("SELECT COUNT(*) FROM learnings").Scan(&n); err != nil || n != 1 {
				t.Fatalf("canonical learning lost after failed migration: count %d, err %v", n, err)
			}
		})
	}
}

func TestLearningVecVersioned_PropagatesSchemaInspectionError(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	// A closed database makes the real schema query fail, rather than
	// returning an empty column set that could be mistaken for a legacy index.
	_, cause := migrationTableColumns(db, "learnings_vec")
	if cause == nil {
		t.Fatal("schema inspection on a closed database unexpectedly succeeded")
	}
	versioned, err := learningVecVersioned(db)
	if versioned {
		t.Fatal("an unreadable index was reported as versioned")
	}
	if !errors.Is(err, cause) {
		t.Fatalf("schema inspection failure was not preserved: got %v, want cause %v", err, cause)
	}
	if !strings.Contains(err.Error(), "inspect learning vector index columns") {
		t.Fatalf("schema inspection failure lost its context: %v", err)
	}
}

// Stale identity must be filtered before top-k, even when its bytes would
// otherwise outrank the current row. Matching empty hashes remain comparable.
func TestLearningRecall_IndexIdentityFiltersBeforeTopK(t *testing.T) {
	for _, tc := range []struct {
		name          string
		canonicalHash any
		indexModel    string
		indexTask     string
		indexHash     string
		wantBest      bool
	}{
		{name: "matching_identity", canonicalHash: "current", indexModel: "engine-b", indexTask: "RETRIEVAL_DOCUMENT", indexHash: "current", wantBest: true},
		{name: "stale_model", canonicalHash: "current", indexModel: "engine-a", indexTask: "RETRIEVAL_DOCUMENT", indexHash: "current"},
		{name: "stale_task", canonicalHash: "current", indexModel: "engine-b", indexTask: "RETRIEVAL_QUERY", indexHash: "current"},
		{name: "stale_hash", canonicalHash: "current", indexModel: "engine-b", indexTask: "RETRIEVAL_DOCUMENT", indexHash: "old"},
		{name: "empty_stale_hash", canonicalHash: "current", indexModel: "engine-b", indexTask: "RETRIEVAL_DOCUMENT"},
		{name: "matching_empty_hash", canonicalHash: "", indexModel: "engine-b", indexTask: "RETRIEVAL_DOCUMENT", wantBest: true},
		{name: "matching_null_canonical_hash", indexModel: "engine-b", indexTask: "RETRIEVAL_DOCUMENT", wantBest: true},
		{name: "stale_hash_against_null", indexModel: "engine-b", indexTask: "RETRIEVAL_DOCUMENT", indexHash: "old"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ls, err := NewLearningStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := ls.Close(); err != nil {
					t.Error(err)
				}
			})
			for _, predicate := range []string{"best_match", "current_fallback"} {
				if err := ls.Save("coder", predicate, []any{"identity test"}, "c"); err != nil {
					t.Fatal(err)
				}
			}
			db, err := ls.getDB("coder")
			if err != nil {
				t.Fatal(err)
			}
			var bestID, fallbackID int64
			if err := db.QueryRow("SELECT id FROM learnings WHERE fact_predicate = 'best_match'").Scan(&bestID); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow("SELECT id FROM learnings WHERE fact_predicate = 'current_fallback'").Scan(&fallbackID); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`UPDATE learnings SET embedding = ?, embedding_model_id = 'engine-b', embedding_dim = 2, embedding_task = 'RETRIEVAL_DOCUMENT', handle_hash = 'fallback'`, encodeFloat32Slice([]float32{3, 4})); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`UPDATE learnings SET embedding = ?, handle_hash = ? WHERE id = ?`, encodeFloat32Slice([]float32{1, 0}), tc.canonicalHash, bestID); err != nil {
				t.Fatal(err)
			}
			if err := ensureLearningVecTable(db, 2); err != nil {
				t.Fatalf("create versioned index: %v", err)
			}
			if _, err := db.Exec(`INSERT INTO learnings_vec (learning_id, embedding, embedding_model_id, embedding_task, handle_hash)
				SELECT id, embedding, embedding_model_id, embedding_task, handle_hash FROM learnings WHERE id = ?`, fallbackID); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`INSERT INTO learnings_vec (learning_id, embedding, embedding_model_id, embedding_task, handle_hash) VALUES (?, ?, ?, ?, ?)`, bestID, encodeFloat32Slice([]float32{1, 0}), tc.indexModel, tc.indexTask, tc.indexHash); err != nil {
				t.Fatal(err)
			}

			hits, err := ls.RecallLearningsByEmbeddingContext(t.Context(), QueryEmbedding{Vector: []float32{1, 0}, ModelID: "engine-b"}, 1)

			if err != nil {
				t.Fatal(err)
			}
			wantID, wantScore := fallbackID, 0.6
			if tc.wantBest {
				wantID, wantScore = bestID, 1
			}
			if len(hits) != 1 || hits[0].LearningID != wantID {
				t.Fatalf("recall = %+v; want only learning %d", hits, wantID)
			}
			if math.Abs(hits[0].Score-wantScore) > 1e-6 {
				t.Errorf("score = %v, want %v", hits[0].Score, wantScore)
			}
		})
	}
}

func TestLearningIndex_LegacyRebuildCopiesOnlyCanonicalVectors(t *testing.T) {
	for _, tc := range []struct {
		name     string
		vector   []byte
		dim      any
		nullTags bool
		wantRow  bool
	}{
		{name: "current_vector", vector: encodeFloat32Slice([]float32{0, 1}), dim: 2, wantRow: true},
		{name: "null_identity_normalized", vector: encodeFloat32Slice([]float32{0, 1}), dim: 2, nullTags: true, wantRow: true},
		{name: "null_embedding", dim: 2},
		{name: "empty_embedding", vector: []byte{}, dim: 2},
		{name: "other_dimension", vector: encodeFloat32Slice([]float32{0, 1, 0}), dim: 3},
		{name: "zero_dimension", vector: encodeFloat32Slice([]float32{0, 1}), dim: 0},
		{name: "null_dimension", vector: encodeFloat32Slice([]float32{0, 1})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ls, err := NewLearningStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := ls.Close(); err != nil {
					t.Error(err)
				}
			})
			for _, predicate := range []string{"control", "candidate"} {
				if err := ls.Save("coder", predicate, []any{"canonical vector"}, "c"); err != nil {
					t.Fatal(err)
				}
			}
			db, err := ls.getDB("coder")
			if err != nil {
				t.Fatal(err)
			}
			db.SetMaxOpenConns(1)
			var controlID, candidateID int64
			if err := db.QueryRow("SELECT id FROM learnings WHERE fact_predicate = 'control'").Scan(&controlID); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow("SELECT id FROM learnings WHERE fact_predicate = 'candidate'").Scan(&candidateID); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`UPDATE learnings SET embedding = ?, embedding_model_id = 'engine-b', embedding_dim = 2, embedding_task = 'RETRIEVAL_DOCUMENT', handle_hash = 'control' WHERE id = ?`, encodeFloat32Slice([]float32{1, 0}), controlID); err != nil {
				t.Fatal(err)
			}
			var model, task, hash any = "engine-c", "RETRIEVAL_DOCUMENT", "candidate"
			if tc.nullTags {
				model, task, hash = nil, nil, nil
			}
			if _, err := db.Exec(`UPDATE learnings SET embedding = ?, embedding_model_id = ?, embedding_dim = ?, embedding_task = ?, handle_hash = ? WHERE id = ?`, tc.vector, model, tc.dim, task, hash, candidateID); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`CREATE VIRTUAL TABLE learnings_vec USING vec0(embedding float[2], learning_id INTEGER)`); err != nil {
				t.Fatalf("create legacy index: %v", err)
			}
			if _, err := db.Exec(`INSERT INTO learnings_vec (learning_id, embedding) VALUES (?, ?)`, candidateID, encodeFloat32Slice([]float32{-1, 0})); err != nil {
				t.Fatal(err)
			}

			if err := ensureLearningVecTable(db, 2); err != nil {
				t.Fatalf("rebuild legacy index: %v", err)
			}

			type indexRow struct {
				id                int64
				vector            []byte
				model, task, hash string
			}
			want := []indexRow{{id: controlID, vector: encodeFloat32Slice([]float32{1, 0}), model: "engine-b", task: "RETRIEVAL_DOCUMENT", hash: "control"}}
			if tc.wantRow {
				r := indexRow{id: candidateID, vector: tc.vector, model: "engine-c", task: "RETRIEVAL_DOCUMENT", hash: "candidate"}
				if tc.nullTags {
					r.model, r.task, r.hash = "", "", ""
				}
				want = append(want, r)
			}
			rows, err := db.Query(`SELECT learning_id, embedding, embedding_model_id, embedding_task, handle_hash FROM learnings_vec ORDER BY learning_id`)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			var got []indexRow
			for rows.Next() {
				var r indexRow
				if err := rows.Scan(&r.id, &r.vector, &r.model, &r.task, &r.hash); err != nil {
					t.Fatal(err)
				}
				got = append(got, r)
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("rebuilt index = %+v, want canonical rows %+v", got, want)
			}
		})
	}
}

func TestLearningIndex_SyncKeepsBytesAndVersionAtomic(t *testing.T) {
	for _, tc := range []struct {
		name    string
		vector  []byte
		fail    bool
		wantRow bool
	}{
		{name: "replace_bytes_and_identity", vector: encodeFloat32Slice([]float32{0, 1}), wantRow: true},
		{name: "remove_null_embedding"},
		{name: "remove_empty_embedding", vector: []byte{}},
		{name: "later_invalid_vector_rolls_back_batch", vector: encodeFloat32Slice([]float32{0, 1}), fail: true, wantRow: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db, err := sql.Open("sqlite3", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			db.SetMaxOpenConns(1)
			t.Cleanup(func() {
				if err := db.Close(); err != nil {
					t.Error(err)
				}
			})
			if err := ensureLearningVecTable(db, 2); err != nil {
				t.Fatalf("create versioned index: %v", err)
			}
			oldBytes := encodeFloat32Slice([]float32{1, 0})
			for _, id := range []int64{7, 8} {
				if _, err := db.Exec(`INSERT INTO learnings_vec (learning_id, embedding, embedding_model_id, embedding_task, handle_hash) VALUES (?, ?, 'engine-a', 'RETRIEVAL_DOCUMENT', 'old')`, id, oldBytes); err != nil {
					t.Fatal(err)
				}
			}
			updates := []LearningEmbeddingUpdate{{ID: 7, Embedding: tc.vector, EmbeddingModelID: "engine-b", EmbeddingTask: "RETRIEVAL_QUERY", HandleHash: "new"}}
			if tc.fail {
				updates = append(updates, LearningEmbeddingUpdate{ID: 8, Embedding: encodeFloat32Slice([]float32{1, 0, 0}), EmbeddingModelID: "engine-b", EmbeddingTask: "RETRIEVAL_QUERY", HandleHash: "new"})
			}

			err = syncLearningVectorIndex(db, updates, 2)

			if (err != nil) != tc.fail {
				t.Fatalf("sync error = %v, want error %v", err, tc.fail)
			}
			for _, id := range []int64{7, 8} {
				var vector []byte
				var model, task, hash string
				err := db.QueryRow(`SELECT embedding, embedding_model_id, embedding_task, handle_hash FROM learnings_vec WHERE learning_id = ?`, id).Scan(&vector, &model, &task, &hash)
				if id == 7 && !tc.wantRow {
					if !errors.Is(err, sql.ErrNoRows) {
						t.Fatalf("removed vector lookup = %v, want sql.ErrNoRows", err)
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				wantBytes, wantModel, wantTask, wantHash := oldBytes, "engine-a", "RETRIEVAL_DOCUMENT", "old"
				if id == 7 && !tc.fail {
					wantBytes, wantModel, wantTask, wantHash = tc.vector, "engine-b", "RETRIEVAL_QUERY", "new"
				}
				if !bytes.Equal(vector, wantBytes) || model != wantModel || task != wantTask || hash != wantHash {
					t.Errorf("learning %d = (%v, %q, %q, %q), want (%v, %q, %q, %q)", id, vector, model, task, hash, wantBytes, wantModel, wantTask, wantHash)
				}
			}
			var count int
			if err := db.QueryRow("SELECT COUNT(*) FROM learnings_vec").Scan(&count); err != nil {
				t.Fatal(err)
			}
			wantCount := 2
			if !tc.wantRow {
				wantCount = 1
			}
			if count != wantCount {
				t.Errorf("index row count = %d, want %d", count, wantCount)
			}
		})
	}
}

func TestProcessLearningReflectionCycle_UpgradesLegacyVecTableWhenNoCandidatesNeedReembed(t *testing.T) {
	t.Parallel()
	ls, err := NewLearningStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := ls.Close(); err != nil {
			t.Error(err)
		}
	})

	ls.SetEmbeddingEngine(&spaceEngine{name: "engine-b", dim: 2})
	if err := ls.Save("coder", "learned_rule", []any{"zero-candidate migration"}, "c"); err != nil {
		t.Fatal(err)
	}

	db, err := ls.getDB("coder")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)

	expectedTask := embedding.SelectTaskType(embedding.ContentTypeKnowledgeAtom, false)
	handle := buildLearningHandle("coder", "learned_rule", []any{"zero-candidate migration"})
	hash := computeDescriptorHash(handle)
	vecBytes := encodeFloat32Slice([]float32{1, 0})

	var id int64
	if err := db.QueryRow("SELECT id FROM learnings WHERE fact_predicate = 'learned_rule'").Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`UPDATE learnings SET semantic_handle = ?, handle_version = ?, handle_hash = ?, embedding = ?, embedding_model_id = 'engine-b', embedding_dim = 2, embedding_task = ? WHERE id = ?`,
		handle, learningHandleVersion, hash, vecBytes, expectedTask, id,
	); err != nil {
		t.Fatal(err)
	}

	candidates, err := ls.ListLearningEmbeddingCandidates("coder", 16, "engine-b", 2, expectedTask)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected 0 embedding candidates before reflection cycle, got %d", len(candidates))
	}

	if _, err := db.Exec(`DROP TABLE IF EXISTS learnings_vec`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE VIRTUAL TABLE learnings_vec USING vec0(embedding float[2], learning_id INTEGER, handle_hash TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO learnings_vec (learning_id, embedding, handle_hash) VALUES (?, ?, ?)`, id, vecBytes, hash); err != nil {
		t.Fatal(err)
	}

	ls.processLearningReflectionCycle()

	versioned, err := learningVecVersioned(db)
	if err != nil {
		t.Fatalf("learningVecVersioned after cycle: %v", err)
	}
	if !versioned {
		t.Fatal("expected processLearningReflectionCycle to upgrade partial/legacy learnings_vec even when 0 candidates need re-embedding")
	}

	hits, err := ls.RecallLearningsByEmbeddingContext(t.Context(), QueryEmbedding{Vector: []float32{1, 0}, ModelID: "engine-b"}, 5)
	if err != nil {
		t.Fatalf("recall after cycle upgrade: %v", err)
	}
	if len(hits) != 1 || hits[0].LearningID != id {
		t.Fatalf("recall hits after cycle upgrade = %+v, want learning %d", hits, id)
	}
}
