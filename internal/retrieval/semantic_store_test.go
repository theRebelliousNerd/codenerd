package retrieval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func openTestVectorStore(t *testing.T, dir string) *FileVectorStore {
	t.Helper()
	st, err := OpenFileVectorStore(context.Background(), filepath.Join(dir, "vectors.db"))
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// A new process starts warm: vectors an earlier searcher embedded come back
// from the store, and only an edited file reaches the engine again. Before
// the store, every process embedded the whole corpus again.
func TestSemanticVectors_PersistAcrossSearchers(t *testing.T) {
	work, db := t.TempDir(), t.TempDir()
	for i := range 5 {
		writeFile(t, work, fmt.Sprintf("f%d.go", i), []byte(fmt.Sprintf("package f // file %d\n", i)))
	}
	ctx := context.Background()

	first := &countingEmbedder{}
	s1 := NewEmbeddingSemanticSearcher(first, work, openTestVectorStore(t, db))
	r1, err := s1.Warm(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Embedded != 5 || first.texts.Load() != 5 {
		t.Fatalf("first warm embedded %d (engine saw %d), want 5", r1.Embedded, first.texts.Load())
	}
	if err := s1.Close(); err != nil {
		t.Fatal(err)
	}

	writeFile(t, work, "f2.go", []byte("package f // edited zebra\n"))
	second := &countingEmbedder{}
	s2 := NewEmbeddingSemanticSearcher(second, work, openTestVectorStore(t, db))
	defer s2.Close()
	r2, err := s2.Warm(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Stored != 4 || r2.Embedded != 1 || second.texts.Load() != 1 {
		t.Fatalf("second warm stored %d, embedded %d (engine saw %d); want 4 from the store and only the edited file embedded", r2.Stored, r2.Embedded, second.texts.Load())
	}
	// The edited file's old vector no longer describes any file.
	if r2.Pruned != 1 {
		t.Fatalf("pruned %d rows, want the edited file's stale one", r2.Pruned)
	}
	result, err := s2.SimilarFiles(ctx, "zebra", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Matches) != 1 || filepath.Base(result.Matches[0].FilePath) != "f2.go" {
		t.Fatalf("the edited file was not ranked by its new text: %+v", result.Matches)
	}
}

// Vectors are shared only within one embedding space: a different model's
// searcher embeds everything itself instead of reading the other's rows.
func TestSemanticVectors_StoreIsPartitionedBySpace(t *testing.T) {
	work, db := t.TempDir(), t.TempDir()
	writeFile(t, work, "a.go", []byte("package a\n"))
	writeFile(t, work, "b.go", []byte("package b\n"))
	ctx := context.Background()

	st := openTestVectorStore(t, db)
	defer st.Close()
	if _, err := NewEmbeddingSemanticSearcher(&countingEmbedder{space: "model-a"}, work, st).Warm(ctx); err != nil {
		t.Fatal(err)
	}
	other := &countingEmbedder{space: "model-b"}
	r, err := NewEmbeddingSemanticSearcher(other, work, st).Warm(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if r.Stored != 0 || other.texts.Load() != 2 {
		t.Fatalf("model-b reused %d of model-a's vectors (engine saw %d)", r.Stored, other.texts.Load())
	}
	// Retain trims only its own space.
	again, err := NewEmbeddingSemanticSearcher(&countingEmbedder{space: "model-a"}, work, st).Warm(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if again.Stored != 2 {
		t.Fatalf("model-b's pass pruned model-a's rows: %+v", again)
	}
}

// The corpus is every eligible file. The searcher used to rank a sample of
// 256, so on a larger repository most files were out of every query's reach.
func TestSemanticTier_CoversTheWholeCorpus(t *testing.T) {
	work := t.TempDir()
	const n = 300
	for i := range n {
		writeFile(t, work, filepath.Join(fmt.Sprintf("pkg%02d", i%30), fmt.Sprintf("f%03d.go", i)), []byte(fmt.Sprintf("package p // %d\n", i)))
	}
	writeFile(t, work, "blank.go", []byte("   \n"))
	s := NewEmbeddingSemanticSearcher(&countingEmbedder{}, work, nil)
	result, err := s.SimilarFiles(context.Background(), "q", 5)
	if err != nil {
		t.Fatal(err)
	}
	// A blank file has nothing to represent: it is not counted as eligible,
	// so it cannot hold the tier at partial forever.
	if result.Eligible != n || result.Covered != n || result.Incomplete != "" {
		t.Fatalf("covered %d of %d (incomplete %q), want all %d", result.Covered, result.Eligible, result.Incomplete, n)
	}
}

// gatedEmbedder blocks corpus batches until released; query embeddings pass.
type gatedEmbedder struct {
	countingEmbedder
	gate    chan struct{}
	entered chan struct{}
}

func (e *gatedEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	select {
	case e.entered <- struct{}{}:
	default:
	}
	select {
	case <-e.gate:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return e.countingEmbedder.EmbedBatch(ctx, texts)
}

// A turn never waits on the warm-up: files the background pass is embedding
// are reported pending and the turn ranks what is already held. Policy keeps
// that out of the model's caveats; it clears when the warm-up lands.
func TestSemanticTier_TurnDoesNotWaitOnTheWarmUp(t *testing.T) {
	work := t.TempDir()
	writeFile(t, work, "held1.go", []byte("package h // zebra\n"))
	writeFile(t, work, "held2.go", []byte("package h // zebra zebra\n"))
	engine := &gatedEmbedder{gate: make(chan struct{}), entered: make(chan struct{}, 1)}
	s := NewEmbeddingSemanticSearcher(engine, work, nil)
	close(engine.gate)
	if _, err := s.Warm(context.Background()); err != nil {
		t.Fatal(err)
	}

	select { // the first pass's signal; the next one must be the second pass's
	case <-engine.entered:
	default:
	}
	engine.gate = make(chan struct{})
	for i := range 4 {
		writeFile(t, work, fmt.Sprintf("new%d.go", i), []byte("package h // new\n"))
	}
	warmDone := make(chan error, 1)
	go func() {
		_, err := s.Warm(context.Background())
		warmDone <- err
	}()
	<-engine.entered

	cfg := DefaultTieredContextConfig(work)
	cfg.Semantic = s
	b := NewTieredContextBuilder(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	start := time.Now()
	files, outcome := b.semanticExpansion(ctx, "zebra", &IssueKeywords{}, map[string]bool{})
	if time.Since(start) > time.Second {
		t.Fatalf("the turn waited %s on the warm-up", time.Since(start))
	}
	if outcome.Status != StatusPartial || outcome.Reason != ReasonEmbedPending || outcome.Eligible != 6 || outcome.Covered != 2 || len(files) == 0 {
		t.Fatalf("outcome = %s with %d files, want partial (embedding_pending), 2 of 6 ranked", outcome, len(files))
	}
	if !strings.Contains(outcome.Detail, "4 of 6") {
		t.Fatalf("detail %q does not say how many are pending", outcome.Detail)
	}

	close(engine.gate)
	if err := <-warmDone; err != nil {
		t.Fatal(err)
	}
	_, outcome = b.semanticExpansion(context.Background(), "zebra", &IssueKeywords{}, map[string]bool{})
	if outcome.Status != StatusComplete || outcome.Covered != 6 {
		t.Fatalf("after the warm-up: %s, want complete over 6", outcome)
	}
}

// A damaged row is a miss, not an error: the file is embedded again and the
// row replaced.
func TestFileVectorStore_DamagedRowIsAMiss(t *testing.T) {
	ctx := context.Background()
	st := openTestVectorStore(t, t.TempDir())
	defer st.Close()
	if err := st.Save(ctx, "s", map[string][]float32{"good": {1, 2}, "bad": {3, 4}}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.ExecContext(ctx, `UPDATE file_vectors SET vec = x'00' WHERE digest = 'bad'`); err != nil {
		t.Fatal(err)
	}
	got, err := st.Load(ctx, "s", []string{"good", "bad", "absent"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got["good"][1] != 2 {
		t.Fatalf("load = %v, want only the intact row", got)
	}
	if _, err := os.Stat(FileVectorStorePath(t.TempDir())); err == nil {
		t.Fatal("FileVectorStorePath must not create anything")
	}
}
