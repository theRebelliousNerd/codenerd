package prompt

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func openUnstampedTestDB(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := NewAtomLoader(nil).EnsureSchema(ctx, db); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	return db
}

func seedPromptRow(t *testing.T, ctx context.Context, db *sql.DB, id, content, desc string, embedding []byte, task, model any) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `INSERT INTO prompt_atoms (atom_id, version, content, token_count, content_hash, description, content_concise, content_min, category, subcategory, importance, is_mandatory, is_exclusive, embedding, embedding_task, embedding_model, source_file) VALUES (?, 1, ?, 1, 'h', ?, NULL, NULL, 'general', NULL, NULL, 0, NULL, ?, ?, ?, 'embedded')`, id, content, desc, embedding, task, model); err != nil {
		t.Fatalf("seed %s: %v", id, err)
	}
}

func TestReconcilePreservesExactTasksThenDropUnstamped(t *testing.T) {
	ctx := context.Background()
	db := openUnstampedTestDB(t, ctx)

	stampedBlob := []byte{1, 2, 3, 4}
	unstampedBlob := []byte{5, 6, 7, 8}
	seedPromptRow(t, ctx, db, "stamped", "hello", "hello", stampedBlob, "RETRIEVAL_DOCUMENT", "test-model")
	seedPromptRow(t, ctx, db, "unstamped-task", "world", "world", unstampedBlob, nil, nil)
	seedPromptRow(t, ctx, db, "unstamped-model", "other", "other", unstampedBlob, "RETRIEVAL_DOCUMENT", nil)
	if _, err := db.ExecContext(ctx, `INSERT INTO vec_prompt_atoms (atom_id, embedding) VALUES ('stamped', ?), ('unstamped-task', ?), ('unstamped-model', ?)`, stampedBlob, unstampedBlob, unstampedBlob); err != nil {
		// vec table may not exist on this schema path; reconciler must still succeed.
		t.Logf("vec seed skipped: %v", err)
	}

	stamped := NewPromptAtom("stamped", AtomCategory("general"), "hello")
	stamped.Description = "hello"
	unstampedTask := NewPromptAtom("unstamped-task", AtomCategory("general"), "world")
	unstampedTask.Description = "world"
	unstampedModel := NewPromptAtom("unstamped-model", AtomCategory("general"), "other")
	unstampedModel.Description = "other"

	counts, err := ReconcilePromptCorpus(ctx, db, []*PromptAtom{stamped, unstampedTask, unstampedModel})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if counts.RetainedEmbeddings != 3 {
		t.Fatalf("retained = %d, want 3 (exact task preservation)", counts.RetainedEmbeddings)
	}
	if counts.ClearedEmbeddings != 0 {
		t.Fatalf("cleared = %d, want 0 before drop", counts.ClearedEmbeddings)
	}

	dropped, err := DropUnstampedPromptEmbeddings(ctx, db)
	if err != nil {
		t.Fatalf("drop unstamped: %v", err)
	}
	if dropped != 2 {
		t.Fatalf("dropped = %d, want 2 (both unstamped)", dropped)
	}

	var keptBlob []byte
	var keptTask, keptModel sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT embedding, embedding_task, embedding_model FROM prompt_atoms WHERE atom_id = 'stamped'`).Scan(&keptBlob, &keptTask, &keptModel); err != nil {
		t.Fatalf("kept row: %v", err)
	}
	if len(keptBlob) == 0 || keptTask.String != "RETRIEVAL_DOCUMENT" || keptModel.String != "test-model" {
		t.Fatalf("stamped row lost stamp: blob=%d task=%q model=%q", len(keptBlob), keptTask.String, keptModel.String)
	}
	for _, id := range []string{"unstamped-task", "unstamped-model"} {
		var blob []byte
		var task, model sql.NullString
		if err := db.QueryRowContext(ctx, `SELECT embedding, embedding_task, embedding_model FROM prompt_atoms WHERE atom_id = ?`, id).Scan(&blob, &task, &model); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if len(blob) != 0 || task.Valid || model.Valid {
			t.Fatalf("%s not dropped: blob=%d task=%q valid=%v model=%q valid=%v", id, len(blob), task.String, task.Valid, model.String, model.Valid)
		}
		if _, err := db.ExecContext(ctx, `SELECT COUNT(*) FROM vec_prompt_atoms WHERE atom_id = ?`); err == nil {
			var n int
			if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM vec_prompt_atoms WHERE atom_id = ?`, id).Scan(&n); err == nil && n != 0 {
				t.Fatalf("%s vec row not dropped: %d", id, n)
			}
		}
	}
}
