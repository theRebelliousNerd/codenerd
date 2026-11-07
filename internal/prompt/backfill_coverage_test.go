package prompt

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/embedding"
	_ "github.com/mattn/go-sqlite3"
)

type flipCtx struct {
	context.Context
	calls  int
	failOn int
	err    error
}

func (f *flipCtx) Err() error {
	f.calls++
	if f.calls >= f.failOn {
		if f.err != nil {
			return f.err
		}
		return context.Canceled
	}
	return nil
}

type errBatchEngine struct {
	stubPromptEngine
	batchErr error
}

func (e *errBatchEngine) EmbedBatchWithTask(ctx context.Context, texts []string, task string) ([][]float32, error) {
	e.calls += len(texts)
	if e.batchErr != nil {
		return nil, e.batchErr
	}
	return e.stubPromptEngine.EmbedBatchWithTask(ctx, texts, task)
}

type taskOnlyEngine struct {
	name       string
	calls      int
	perItemErr map[string]error
	emptyFor   map[string]bool
}

func (e *taskOnlyEngine) Name() string    { return e.name }
func (e *taskOnlyEngine) Dimensions() int { return 3 }
func (e *taskOnlyEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	e.calls++
	return []float32{1, 2, 3}, nil
}
func (e *taskOnlyEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	e.calls += len(texts)
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{7, 8, 9}
	}
	return out, nil
}
func (e *taskOnlyEngine) EmbedWithTask(ctx context.Context, text, task string) ([]float32, error) {
	e.calls++
	if err, ok := e.perItemErr[text]; ok && err != nil {
		return nil, err
	}
	if e.emptyFor != nil && e.emptyFor[text] {
		return []float32{}, nil
	}
	return []float32{4, 5, 6}, nil
}

var _ embedding.TaskTypeAwareEngine = (*taskOnlyEngine)(nil)

type plainEngine struct {
	name     string
	calls    int
	batchErr error
	vecs     [][]float32
}

func (e *plainEngine) Name() string    { return e.name }
func (e *plainEngine) Dimensions() int { return 3 }
func (e *plainEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	e.calls++
	return []float32{1, 2, 3}, nil
}
func (e *plainEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	e.calls += len(texts)
	if e.batchErr != nil {
		return nil, e.batchErr
	}
	if len(e.vecs) > 0 {
		out := make([][]float32, len(texts))
		for i := range texts {
			out[i] = e.vecs[i%len(e.vecs)]
		}
		return out, nil
	}
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{9, 9, 9}
	}
	return out, nil
}

var _ embedding.EmbeddingEngine = (*plainEngine)(nil)

func TestBackfillCoversNilDB(t *testing.T) {
	n, err := BackfillMissingPromptVectors(context.Background(), nil, "x", &stubPromptEngine{name: "m"}, 10)
	if err != nil || n != 0 {
		t.Fatalf("nil db = %d,%v want 0,nil", n, err)
	}
}

func TestBackfillCoversCancelledAtEntry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	db := openBackfillTestDB(t, context.Background())
	n, err := BackfillMissingPromptVectors(ctx, db, "x", &stubPromptEngine{name: "m"}, 10)
	if !errors.Is(err, context.Canceled) || n != 0 {
		t.Fatalf("cancelled entry = %d,%v want 0,canceled", n, err)
	}
}

func TestBackfillCoversDefaultMaxAtoms(t *testing.T) {
	for _, max := range []int{0, -5} {
		ctx := context.Background()
		db := openBackfillTestDB(t, ctx)
		seedPromptRow(t, ctx, db, "m1", "hello", "hello", nil, nil, nil)
		n, err := BackfillMissingPromptVectors(ctx, db, "x", &stubPromptEngine{name: "m"}, max)
		if err != nil || n != 1 {
			t.Fatalf("max=%d filled=%d err=%v want 1", max, n, err)
		}
	}
}

func TestBackfillCoversQueryError(t *testing.T) {
	ctx := context.Background()
	db, _ := sql.Open("sqlite3", ":memory:")
	_ = db.Close()
	_, err := BackfillMissingPromptVectors(ctx, db, "x", &stubPromptEngine{name: "m"}, 10)
	if err == nil {
		t.Fatalf("closed db query should error")
	}
}

func TestBackfillCoversScanError(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE prompt_atoms (atom_id TEXT, description TEXT, content TEXT, embedding BLOB, embedding_task TEXT, embedding_model TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO prompt_atoms (atom_id, description, content, embedding) VALUES (NULL, 'd', 'c', NULL)`); err != nil {
		t.Fatalf("seed null id: %v", err)
	}
	_, err = BackfillMissingPromptVectors(ctx, db, "x", &stubPromptEngine{name: "m"}, 10)
	if err == nil {
		t.Fatalf("null atom_id scan should error")
	}
}

func TestBackfillCoversDescFallbackAndBlankSkip(t *testing.T) {
	ctx := context.Background()
	db := openBackfillTestDB(t, ctx)
	seedPromptRow(t, ctx, db, "fallback", "body-text", "", nil, nil, nil)
	seedPromptRow(t, ctx, db, "blank", "", "   ", nil, nil, nil)
	engine := &stubPromptEngine{name: "m"}
	n, err := BackfillMissingPromptVectors(ctx, db, "x", engine, 10)
	if err != nil || n != 1 {
		t.Fatalf("fallback/blank = %d,%v want 1", n, err)
	}
	var task sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT embedding_task FROM prompt_atoms WHERE atom_id='fallback'`).Scan(&task); err != nil {
		t.Fatal(err)
	}
}

func TestBackfillCoversEmptyWork(t *testing.T) {
	ctx := context.Background()
	db := openBackfillTestDB(t, ctx)
	seedPromptRow(t, ctx, db, "blankonly", "", "  ", nil, nil, nil)
	n, err := BackfillMissingPromptVectors(ctx, db, "x", &stubPromptEngine{name: "m"}, 10)
	if err != nil || n != 0 {
		t.Fatalf("blank only = %d,%v want 0", n, err)
	}
	db2 := openBackfillTestDB(t, ctx)
	n, err = BackfillMissingPromptVectors(ctx, db2, "x", &stubPromptEngine{name: "m"}, 10)
	if err != nil || n != 0 {
		t.Fatalf("no rows = %d,%v want 0", n, err)
	}
}

func TestBackfillCoversSecondCtxCheck(t *testing.T) {
	ctx := context.Background()
	db := openBackfillTestDB(t, ctx)
	seedPromptRow(t, ctx, db, "a", "hi", "hi", nil, nil, nil)
	fc := &flipCtx{Context: context.Background(), failOn: 2, err: context.Canceled}
	_, err := BackfillMissingPromptVectors(fc, db, "x", &stubPromptEngine{name: "m"}, 10)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("second ctx check err=%v want canceled", err)
	}
}

func TestBackfillCoversBatchError(t *testing.T) {
	ctx := context.Background()
	db := openBackfillTestDB(t, ctx)
	seedPromptRow(t, ctx, db, "a", "hi", "hi", nil, nil, nil)
	eng := &errBatchEngine{batchErr: errors.New("batch boom")}
	eng.name = "m"
	_, err := BackfillMissingPromptVectors(ctx, db, "x", eng, 10)
	if err == nil || !strings.Contains(err.Error(), "batch boom") {
		t.Fatalf("batch err=%v want boom", err)
	}
}

func TestBackfillCoversTaskOnlyWithPerItemErrorAndEmpty(t *testing.T) {
	ctx := context.Background()
	db := openBackfillTestDB(t, ctx)
	seedPromptRow(t, ctx, db, "good", "good-text", "good-text", nil, nil, nil)
	seedPromptRow(t, ctx, db, "bad", "bad-text", "bad-text", nil, nil, nil)
	seedPromptRow(t, ctx, db, "emptyvec", "empty-text", "empty-text", nil, nil, nil)
	eng := &taskOnlyEngine{name: "m", perItemErr: map[string]error{"bad-text": errors.New("one fail")}, emptyFor: map[string]bool{"empty-text": true}}
	n, err := BackfillMissingPromptVectors(ctx, db, "x", eng, 10)
	if err != nil {
		t.Fatalf("taskOnly err=%v", err)
	}
	if n != 1 {
		t.Fatalf("taskOnly filled=%d want 1 (good only)", n)
	}
	var goodBlob []byte
	if err := db.QueryRowContext(ctx, `SELECT embedding FROM prompt_atoms WHERE atom_id='good'`).Scan(&goodBlob); err != nil || len(goodBlob) == 0 {
		t.Fatalf("good not filled")
	}
	var badBlob []byte
	_ = db.QueryRowContext(ctx, `SELECT embedding FROM prompt_atoms WHERE atom_id='bad'`).Scan(&badBlob)
	if len(badBlob) != 0 {
		t.Fatalf("bad should stay empty")
	}
}

func TestBackfillCoversTaskOnlyCtxCancelInsideLoop(t *testing.T) {
	ctx := context.Background()
	db := openBackfillTestDB(t, ctx)
	seedPromptRow(t, ctx, db, "a", "ta", "ta", nil, nil, nil)
	seedPromptRow(t, ctx, db, "b", "tb", "tb", nil, nil, nil)
	eng := &taskOnlyEngine{name: "m"}
	fc := &flipCtx{Context: context.Background(), failOn: 3, err: context.Canceled}
	_, err := BackfillMissingPromptVectors(fc, db, "x", eng, 10)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("inner cancel err=%v want canceled", err)
	}
}

func TestBackfillCoversFallbackBatchAndError(t *testing.T) {
	ctx := context.Background()
	db := openBackfillTestDB(t, ctx)
	seedPromptRow(t, ctx, db, "a", "hi", "hi", nil, nil, nil)
	plain := &plainEngine{name: "m"}
	n, err := BackfillMissingPromptVectors(ctx, db, "x", plain, 10)
	if err != nil || n != 1 {
		t.Fatalf("fallback ok = %d,%v want 1", n, err)
	}
	db2 := openBackfillTestDB(t, ctx)
	seedPromptRow(t, ctx, db2, "a", "hi", "hi", nil, nil, nil)
	bad := &plainEngine{name: "m", batchErr: errors.New("fallback boom")}
	_, err = BackfillMissingPromptVectors(ctx, db2, "x", bad, 10)
	if err == nil {
		t.Fatalf("fallback err should boom")
	}
}

func TestBackfillCoversFillLoopCancelAndEmptyVec(t *testing.T) {
	ctx := context.Background()
	db := openBackfillTestDB(t, ctx)
	seedPromptRow(t, ctx, db, "a", "ta", "ta", nil, nil, nil)
	eng := &plainEngine{name: "m", vecs: [][]float32{{}}}
	_ = eng
	emptyEng := &stubPromptEngine{name: "m", vecs: [][]float32{{}}}
	n, err := BackfillMissingPromptVectors(ctx, db, "x", emptyEng, 10)
	if err != nil || n != 0 {
		t.Fatalf("empty vec = %d,%v want 0", n, err)
	}
	db2 := openBackfillTestDB(t, ctx)
	seedPromptRow(t, ctx, db2, "a", "ta", "ta", nil, nil, nil)
	fc := &flipCtx{Context: context.Background(), failOn: 3, err: context.Canceled}
	_, err = BackfillMissingPromptVectors(fc, db2, "x", &stubPromptEngine{name: "m"}, 10)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("fill cancel err=%v want canceled", err)
	}
}

func TestBackfillCoversExecError(t *testing.T) {
	ctx := context.Background()
	db := openBackfillTestDB(t, ctx)
	seedPromptRow(t, ctx, db, "a", "hi", "hi", nil, nil, nil)
	if _, err := db.Exec(`CREATE TRIGGER fail_update BEFORE UPDATE ON prompt_atoms BEGIN SELECT RAISE(ABORT, 'nope'); END;`); err != nil {
		t.Fatal(err)
	}
	_, err := BackfillMissingPromptVectors(ctx, db, "x", &stubPromptEngine{name: "m"}, 10)
	if err == nil {
		t.Fatalf("trigger should fail update")
	}
}

func TestBackfillCoversEmptyLabel(t *testing.T) {
	ctx := context.Background()
	db := openBackfillTestDB(t, ctx)
	seedPromptRow(t, ctx, db, "a", "hi", "hi", nil, nil, nil)
	n, err := BackfillMissingPromptVectors(ctx, db, "", &stubPromptEngine{name: "m"}, 10)
	if err != nil || n != 1 {
		t.Fatalf("empty label = %d,%v want 1", n, err)
	}
}

func TestNullableBackfillStringCoversBoth(t *testing.T) {
	if nullableBackfillString("") != nil {
		t.Fatalf("empty should be nil")
	}
	if nullableBackfillString("   ") != nil {
		t.Fatalf("whitespace should be nil")
	}
	v := nullableBackfillString("RETRIEVAL_DOCUMENT")
	s, ok := v.(string)
	if !ok || s != "RETRIEVAL_DOCUMENT" {
		t.Fatalf("stamped = %v want string", v)
	}
}
