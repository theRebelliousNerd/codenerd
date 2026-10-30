package store

import (
	"fmt"
	"strings"
	"testing"
)

// A healthy query from engine B was ranked against an unmigrated index of
// engine A when both had the same dimension: the cosine between two spaces
// ranked A's rows first and the merge displaced comparable facts. Semantic
// recall now ranks only rows of the query's space, filtering before each
// shard's top-k, so foreign rows cannot starve a comparable lower-ranked one;
// a query that names no engine is refused.
func TestLearningSemanticRecallRanksOnlyTheQuerysSpace(t *testing.T) {
	ls, err := NewLearningStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ls.Close()
	const foreign = 7
	for i := 0; i < foreign; i++ {
		if err := ls.Save("coder", "learned_pattern", []any{fmt.Sprintf("old-space fact %d", i)}, "c"); err != nil {
			t.Fatal(err)
		}
	}
	if err := ls.Save("coder", "learned_pattern", []any{"new-space fact"}, "c"); err != nil {
		t.Fatal(err)
	}
	db, err := ls.getDB("coder")
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureLearningVecTable(db, 2); err != nil {
		t.Skipf("sqlite-vec unavailable: %v", err)
	}
	rows, err := db.Query(`SELECT id, fact_args FROM learnings ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		id   int64
		args string
	}
	var all []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.args); err != nil {
			t.Fatal(err)
		}
		all = append(all, r)
	}
	rows.Close()
	var compatibleID int64
	for _, r := range all {
		model, vec := "engine-a", []float32{1, 0} // the old space, aligned with the query
		if strings.Contains(r.args, "new-space") {
			model, vec, compatibleID = "engine-b", []float32{0, 1}, r.id
		}
		if _, err := db.Exec(`UPDATE learnings SET embedding_model_id = ?, embedding_dim = 2, embedding_task = 'RETRIEVAL_DOCUMENT' WHERE id = ?`, model, r.id); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO learnings_vec (learning_id, embedding, embedding_model_id, embedding_task, handle_hash)
			SELECT id, ?, embedding_model_id, embedding_task, COALESCE(handle_hash, '') FROM learnings WHERE id = ?`, encodeFloat32Slice(vec), r.id); err != nil {
			t.Fatal(err)
		}
	}

	query := QueryEmbedding{Vector: []float32{1, 0}, ModelID: "engine-b"}
	hits, err := ls.RecallLearningsByEmbeddingContext(t.Context(), query, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].LearningID != compatibleID || hits[0].EmbeddingModelID != "engine-b" {
		t.Fatalf("engine-b query ranked %+v; want only the engine-b row %d (the %d same-dimension engine-a rows are another space)", hits, compatibleID, foreign)
	}

	if _, err := ls.RecallLearningsByEmbeddingContext(t.Context(), QueryEmbedding{Vector: []float32{1, 0}}, 5); err == nil {
		t.Fatal("a query embedding naming no engine was ranked")
	}

	// The old space stays reachable lexically.
	lex, err := ls.RecallLearningsLexicalContext(t.Context(), "old-space fact", 5)
	if err != nil || len(lex) == 0 {
		t.Fatalf("old-space facts lost their lexical path: %v %v", lex, err)
	}
}
