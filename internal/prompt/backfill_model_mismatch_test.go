package prompt

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestBackfillMissingPromptVectors_RefillsModelMismatch(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := NewAtomLoader(nil).EnsureSchema(ctx, db); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	seedPromptRow(t, ctx, db, "mismatched", "backfill me", "backfill me", []byte{9, 9, 9, 9}, "RETRIEVAL_DOCUMENT", "old-model")
	seedPromptRow(t, ctx, db, "unstamped", "backfill me too", "backfill me too", []byte{8, 8, 8, 8}, nil, nil)

	engine := &stubPromptEngine{name: "test-model"}
	n, err := BackfillMissingPromptVectors(ctx, db, "test.db", engine, 10)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if n != 2 {
		t.Fatalf("filled = %d, want 2 (mismatched + unstamped)", n)
	}
	for _, id := range []string{"mismatched", "unstamped"} {
		var model sql.NullString
		var task sql.NullString
		var blob []byte
		if err := db.QueryRowContext(ctx, `SELECT embedding, embedding_task, embedding_model FROM prompt_atoms WHERE atom_id = ?`, id).Scan(&blob, &task, &model); err != nil {
			t.Fatalf("%s row: %v", id, err)
		}
		if len(blob) == 0 || !model.Valid || model.String != "test-model" {
			t.Fatalf("%s not restamped: blob=%d model=%q", id, len(blob), model.String)
		}
		if !task.Valid || task.String == "" {
			t.Fatalf("%s missing task stamp", id)
		}
	}
}

func TestBackfillMissingPromptVectors_EmptyModelNoop(t *testing.T) {
	ctx := context.Background()
	db := openBackfillTestDB(t, ctx)
	seedPromptRow(t, ctx, db, "missing", "text", "text", nil, nil, nil)
	engine := &stubPromptEngine{name: "  "}
	n, err := BackfillMissingPromptVectors(ctx, db, "test.db", engine, 10)
	if err != nil {
		t.Fatalf("empty model backfill: %v", err)
	}
	if n != 0 {
		t.Fatalf("empty model filled = %d, want 0", n)
	}
}
