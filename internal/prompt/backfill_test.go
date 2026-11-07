package prompt

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"codenerd/internal/embedding"
	_ "github.com/mattn/go-sqlite3"
)

type stubPromptEngine struct {
	name  string
	vecs  [][]float32
	calls int
	err   error
}

func (s *stubPromptEngine) Name() string    { return s.name }
func (s *stubPromptEngine) Dimensions() int { return 3 }
func (s *stubPromptEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if len(s.vecs) > 0 {
		return s.vecs[0], nil
	}
	return []float32{1, 2, 3}, nil
}
func (s *stubPromptEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	s.calls += len(texts)
	if s.err != nil {
		return nil, s.err
	}
	out := make([][]float32, len(texts))
	for i := range texts {
		if len(s.vecs) > 0 {
			out[i] = s.vecs[i%len(s.vecs)]
		} else {
			out[i] = []float32{1, 2, 3}
		}
	}
	return out, nil
}
func (s *stubPromptEngine) EmbedWithTask(ctx context.Context, text, task string) ([]float32, error) {
	return s.Embed(ctx, text)
}
func (s *stubPromptEngine) EmbedBatchWithTask(ctx context.Context, texts []string, task string) ([][]float32, error) {
	return s.EmbedBatch(ctx, texts)
}

var _ embedding.EmbeddingEngine = (*stubPromptEngine)(nil)
var _ embedding.TaskTypeAwareEngine = (*stubPromptEngine)(nil)
var _ embedding.TaskTypeBatchAwareEngine = (*stubPromptEngine)(nil)

func openBackfillTestDB(t *testing.T, ctx context.Context) *sql.DB {
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

func TestBackfillMissingPromptVectors_FillsOnlyMissing(t *testing.T) {
	ctx := context.Background()
	db := openBackfillTestDB(t, ctx)
	seedPromptRow(t, ctx, db, "missing", "backfill me", "backfill me", nil, nil, nil)
	seedPromptRow(t, ctx, db, "stamped", "keep", "keep", []byte{9, 9, 9, 9}, "RETRIEVAL_DOCUMENT", "test-model")

	engine := &stubPromptEngine{name: "test-model"}
	n, err := BackfillMissingPromptVectors(ctx, db, "test.db", engine, 10)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if n != 1 {
		t.Fatalf("filled = %d, want 1", n)
	}
	var blob []byte
	var task, model sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT embedding, embedding_task, embedding_model FROM prompt_atoms WHERE atom_id = 'missing'`).Scan(&blob, &task, &model); err != nil {
		t.Fatalf("missing row: %v", err)
	}
	if len(blob) == 0 || !task.Valid || !model.Valid || model.String != "test-model" {
		t.Fatalf("missing not stamped: blob=%d task=%q model=%q", len(blob), task.String, model.String)
	}
	var kept []byte
	if err := db.QueryRowContext(ctx, `SELECT embedding FROM prompt_atoms WHERE atom_id = 'stamped'`).Scan(&kept); err != nil {
		t.Fatalf("stamped row: %v", err)
	}
	if len(kept) == 0 {
		t.Fatalf("stamped row lost embedding")
	}
	if engine.calls == 0 {
		t.Fatalf("engine was not called")
	}
}

func TestBackfillMissingPromptVectors_BoundedAndCancellable(t *testing.T) {
	ctx := context.Background()
	db := openBackfillTestDB(t, ctx)
	for _, id := range []string{"a", "b", "c"} {
		seedPromptRow(t, ctx, db, id, "text "+id, "text "+id, nil, nil, nil)
	}
	engine := &stubPromptEngine{name: "test-model"}

	n, err := BackfillMissingPromptVectors(ctx, db, "test.db", engine, 1)
	if err != nil {
		t.Fatalf("bounded backfill: %v", err)
	}
	if n != 1 {
		t.Fatalf("bounded filled = %d, want 1", n)
	}
	var remaining int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM prompt_atoms WHERE embedding IS NULL`).Scan(&remaining); err != nil {
		t.Fatalf("count remaining: %v", err)
	}
	if remaining != 2 {
		t.Fatalf("remaining = %d, want 2", remaining)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := BackfillMissingPromptVectors(cancelled, db, "test.db", engine, 10); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled err = %v, want context.Canceled", err)
	}
}

func TestBackfillMissingPromptVectors_NilEngineNoop(t *testing.T) {
	ctx := context.Background()
	db := openBackfillTestDB(t, ctx)
	seedPromptRow(t, ctx, db, "missing", "text", "text", nil, nil, nil)
	n, err := BackfillMissingPromptVectors(ctx, db, "test.db", nil, 10)
	if err != nil {
		t.Fatalf("nil engine: %v", err)
	}
	if n != 0 {
		t.Fatalf("nil engine filled = %d, want 0", n)
	}
}
