package prompt

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"
)

// blockingPromptEngine embeds nothing until its context ends, so a test can
// prove the compiler's vector job is stopped and joined by Close.
type blockingPromptEngine struct {
	stubPromptEngine
	entered chan struct{}
}

func (b *blockingPromptEngine) EmbedBatchWithTask(ctx context.Context, texts []string, task string) ([][]float32, error) {
	select {
	case b.entered <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

func countPromptVectors(t *testing.T, db *sql.DB) (stamped, missing int) {
	t.Helper()
	err := db.QueryRow(`SELECT
		COALESCE(SUM(CASE WHEN embedding IS NOT NULL AND embedding_model IS NOT NULL THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN embedding IS NULL THEN 1 ELSE 0 END), 0)
		FROM prompt_atoms`).Scan(&stamped, &missing)
	if err != nil {
		t.Fatalf("count vectors: %v", err)
	}
	return stamped, missing
}

// The compiler's vector job drains every batch, not just the first, and
// leaves an atom with no text alone instead of selecting it forever.
func TestStartPromptVectorBackfillDrainsEveryBatch(t *testing.T) {
	ctx := context.Background()
	db := openBackfillTestDB(t, ctx)
	want := BackfillPromptLimit + 7
	for i := 0; i < want; i++ {
		id := fmt.Sprintf("atom-%03d", i)
		seedPromptRow(t, ctx, db, id, "text "+id, "text "+id, nil, nil, nil)
	}
	seedPromptRow(t, ctx, db, "blank", "", "  ", nil, nil, nil)

	compiler, err := NewJITPromptCompiler(WithProjectDB(db))
	if err != nil {
		t.Fatalf("compiler: %v", err)
	}
	if !compiler.StartPromptVectorBackfill(&stubPromptEngine{name: "test-model"}) {
		t.Fatal("backfill did not start")
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		stamped, _ := countPromptVectors(t, db)
		if stamped == want {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("stamped %d of %d atoms before the deadline", stamped, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
	var blank []byte
	if err := db.QueryRow(`SELECT embedding FROM prompt_atoms WHERE atom_id = 'blank'`).Scan(&blank); err != nil {
		t.Fatalf("blank row: %v", err)
	}
	if len(blank) != 0 {
		t.Fatal("an atom with no text was embedded")
	}
	if err := compiler.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

// Close cancels a running vector job and waits for it before closing the
// database the job writes to.
func TestStartPromptVectorBackfillStoppedAndJoinedByClose(t *testing.T) {
	ctx := context.Background()
	db := openBackfillTestDB(t, ctx)
	seedPromptRow(t, ctx, db, "a", "hi", "hi", nil, nil, nil)
	compiler, err := NewJITPromptCompiler(WithProjectDB(db))
	if err != nil {
		t.Fatalf("compiler: %v", err)
	}
	engine := &blockingPromptEngine{stubPromptEngine: stubPromptEngine{name: "m"}, entered: make(chan struct{}, 1)}
	if !compiler.StartPromptVectorBackfill(engine) {
		t.Fatal("backfill did not start")
	}
	select {
	case <-engine.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("job never reached the engine")
	}
	closed := make(chan error, 1)
	go func() { closed <- compiler.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("close: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Close did not stop the vector job")
	}
	if compiler.StartPromptVectorBackfill(&stubPromptEngine{name: "m"}) {
		t.Fatal("a closed compiler started a vector job")
	}
}

func TestStartPromptVectorBackfillRefusesWithoutEngineOrDB(t *testing.T) {
	compiler, err := NewJITPromptCompiler()
	if err != nil {
		t.Fatalf("compiler: %v", err)
	}
	defer compiler.Close()
	if compiler.StartPromptVectorBackfill(&stubPromptEngine{name: "m"}) {
		t.Fatal("started with no project database")
	}
	db := openBackfillTestDB(t, context.Background())
	withDB, err := NewJITPromptCompiler(WithProjectDB(db))
	if err != nil {
		t.Fatalf("compiler: %v", err)
	}
	defer withDB.Close()
	if withDB.StartPromptVectorBackfill(nil) {
		t.Fatal("started with no engine")
	}
}
