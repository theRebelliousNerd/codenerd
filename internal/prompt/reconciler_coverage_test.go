package prompt

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func mustCoverageDB(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := NewAtomLoader(nil).EnsureSchema(ctx, db); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	return db
}

func TestReconcileCoversNilDBAndCancelledCtx(t *testing.T) {
	if _, err := ReconcilePromptCorpus(context.Background(), nil, nil); err == nil {
		t.Fatalf("nil db should error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	db := mustCoverageDB(t, context.Background())
	if _, err := ReconcilePromptCorpus(ctx, db, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled ctx err=%v want canceled", err)
	}
	if _, err := DropUnstampedPromptEmbeddings(ctx, db); !errors.Is(err, context.Canceled) {
		t.Fatalf("drop cancelled err=%v", err)
	}
	if _, err := ReconcileEmbeddedCorpus(ctx, db); !errors.Is(err, context.Canceled) {
		t.Fatalf("embedded cancelled err=%v", err)
	}
}

func TestReconcileCoversBeginTxAndQueryErrors(t *testing.T) {
	ctx := context.Background()
	closed, _ := sql.Open("sqlite3", ":memory:")
	_ = closed.Close()
	if _, err := ReconcilePromptCorpus(ctx, closed, []*PromptAtom{NewPromptAtom("x", AtomCategory("general"), "hi")}); err == nil {
		t.Fatalf("closed db reconcile should error")
	}
	if _, err := DropUnstampedPromptEmbeddings(ctx, closed); err == nil {
		t.Fatalf("closed db drop should error")
	}
	// Missing prompt_atoms table.
	empty, _ := sql.Open("sqlite3", ":memory:")
	empty.SetMaxOpenConns(1)
	defer empty.Close()
	if _, err := ReconcilePromptCorpus(ctx, empty, []*PromptAtom{NewPromptAtom("x", AtomCategory("general"), "hi")}); err == nil {
		t.Fatalf("missing table reconcile should error")
	}
	if _, err := DropUnstampedPromptEmbeddings(ctx, empty); err == nil {
		t.Fatalf("missing table drop should error")
	}
}

func TestDropCoversEmptyBlobAndStampCombos(t *testing.T) {
	ctx := context.Background()
	db := mustCoverageDB(t, ctx)
	// Empty blob row is selected (embedding IS NOT NULL with zero bytes) but skipped.
	if _, err := db.Exec(`INSERT INTO prompt_atoms (atom_id, version, content, token_count, content_hash, category, embedding, embedding_task, embedding_model, source_file) VALUES ('emptyblob',1,'c',1,'h','general',X'', 'T', 'M', 'embedded')`); err != nil {
		t.Fatalf("seed empty blob: %v", err)
	}
	seedPromptRow(t, ctx, db, "task-null", "c1", "d1", []byte{1}, nil, nil)
	seedPromptRow(t, ctx, db, "task-empty", "c2", "d2", []byte{2}, "", "M")
	seedPromptRow(t, ctx, db, "task-ws", "c3", "d3", []byte{3}, "   ", "M")
	seedPromptRow(t, ctx, db, "model-null", "c4", "d4", []byte{4}, "T", nil)
	seedPromptRow(t, ctx, db, "model-empty", "c5", "d5", []byte{5}, "T", "")
	seedPromptRow(t, ctx, db, "model-ws", "c6", "d6", []byte{6}, "T", "  ")
	seedPromptRow(t, ctx, db, "stamped", "c7", "d7", []byte{7}, "T", "M")
	dropped, err := DropUnstampedPromptEmbeddings(ctx, db)
	if err != nil {
		t.Fatalf("drop: %v", err)
	}
	if dropped != 6 {
		t.Fatalf("dropped=%d want 6 (empty blob skipped, stamped kept)", dropped)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM prompt_atoms WHERE embedding IS NOT NULL`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("remaining vectors=%d err=%v want 2 (empty+stamped)", n, err)
	}
}

func TestDropCoversVecMissingTableAndUpdateError(t *testing.T) {
	ctx := context.Background()
	db := mustCoverageDB(t, ctx)
	seedPromptRow(t, ctx, db, "u1", "c", "d", []byte{9}, nil, "M")
	// Drop vec table to hit missing-table continue branch.
	_, _ = db.Exec(`DROP TABLE IF EXISTS vec_prompt_atoms`)
	dropped, err := DropUnstampedPromptEmbeddings(ctx, db)
	if err != nil || dropped != 1 {
		t.Fatalf("missing vec table drop=%d err=%v want 1,nil", dropped, err)
	}
	// Update error via trigger.
	db2 := mustCoverageDB(t, ctx)
	seedPromptRow(t, ctx, db2, "u2", "c", "d", []byte{9}, nil, "M")
	if _, err := db2.Exec(`CREATE TRIGGER fail_upd BEFORE UPDATE ON prompt_atoms BEGIN SELECT RAISE(ABORT, 'nope'); END;`); err != nil {
		t.Fatal(err)
	}
	if _, err := DropUnstampedPromptEmbeddings(ctx, db2); err == nil {
		t.Fatalf("trigger should fail update")
	}
}

func TestReconcileEmbeddedCoversSuccessAndDropError(t *testing.T) {
	ctx := context.Background()
	db := mustCoverageDB(t, ctx)
	counts, err := ReconcileEmbeddedCorpus(ctx, db)
	if err != nil {
		t.Fatalf("embedded reconcile: %v", err)
	}
	if counts.Upserted == 0 {
		t.Fatalf("embedded upserted=0 want >0")
	}
	// Closed db forces reconcile error path.
	closed, _ := sql.Open("sqlite3", ":memory:")
	_ = closed.Close()
	if _, err := ReconcileEmbeddedCorpus(ctx, closed); err == nil {
		t.Fatalf("closed embedded should error")
	}
}

func TestEnsureColumnsCoversOldSchemaAndNullable(t *testing.T) {
	ctx := context.Background()
	db := mustCoverageDB(t, ctx)
	// Simulate an old workspace DB that predates embedding_model: full schema
	// minus that one column. Reconcile must heal it via ensure.
	if _, err := db.Exec(`ALTER TABLE prompt_atoms DROP COLUMN embedding_model`); err != nil {
		t.Skipf("sqlite without DROP COLUMN: %v", err)
	}
	a := NewPromptAtom("old1", AtomCategory("general"), "hello")
	a.Description = "hello"
	if _, err := ReconcilePromptCorpus(ctx, db, []*PromptAtom{a}); err != nil {
		t.Fatalf("old schema reconcile: %v", err)
	}
	// ensure ran: model column now exists.
	var sqltext string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE name='prompt_atoms'`).Scan(&sqltext); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sqltext, "embedding_model") {
		t.Fatalf("migration missing embedding_model in %s", sqltext)
	}
	if nullableBytes(nil) != nil {
		t.Fatalf("nullableBytes(nil) should be nil")
	}
	if nullableBytes([]byte{1, 2}) == nil {
		t.Fatalf("nullableBytes(non-nil) should be non-nil")
	}
}
