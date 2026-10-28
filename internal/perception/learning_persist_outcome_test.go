package perception

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	storepkg "codenerd/internal/store"
)

// withSharedClassifier installs sc as the shared classifier for one test.
func withSharedClassifier(t *testing.T, sc *SemanticClassifier) {
	t.Helper()
	prev := SharedSemanticClassifier
	SharedSemanticClassifier = sc
	t.Cleanup(func() { SharedSemanticClassifier = prev })
}

const persistFact = `learned_exemplar("xyzzy batter", /document, "include the TUI", "", 0.95).`

// ctxEngine is an embedding engine that honors its context: an embed blocks
// until the test cancels it, and every call is counted.
type ctxEngine struct {
	dims    int
	calls   atomic.Int32
	entered chan struct{}
	block   bool
}

func (e *ctxEngine) Embed(ctx context.Context, _ string) ([]float32, error) {
	e.calls.Add(1)
	if e.block {
		e.entered <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	v := make([]float32, e.dims)
	v[0] = 1
	return v, nil
}

func (e *ctxEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for _, text := range texts {
		v, err := e.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}
func (e *ctxEngine) Dimensions() int { return e.dims }
func (e *ctxEngine) Name() string    { return "ctx-test" }

func memoryClassifier(t *testing.T, engine *ctxEngine) *SemanticClassifier {
	t.Helper()
	store, err := NewLearnedCorpusStore(nil, engine.dims, nil)
	if err != nil {
		t.Fatal(err)
	}
	return NewSemanticClassifier(&mockKernel{}, nil, store, engine)
}

func TestPersistLearnedFact_OutcomeCompleteWhenEveryBackendCommits(t *testing.T) {
	eng := testTaxonomyEngine(t)
	eng.SetWorkspace(t.TempDir())
	withSharedClassifier(t, memoryClassifier(t, &ctxEngine{dims: 3}))

	out, err := eng.PersistLearnedFact(context.Background(), persistFact)
	if err != nil {
		t.Fatalf("PersistLearnedFact: %v", err)
	}
	if out.Taxonomy != ProjectionInstalled || out.File != ProjectionInstalled || out.Semantic != ProjectionInstalled {
		t.Fatalf("outcome = %s, want taxonomy, file and semantic installed", out)
	}
	if out.Knowledge != ProjectionUnavailable {
		t.Fatalf("knowledge = %s, want unavailable (no taxonomy store attached)", out.Knowledge)
	}
	if !out.Complete() {
		t.Fatalf("outcome %s not Complete", out)
	}
}

// A failed semantic projection used to be a log line while the function
// returned success whenever the final file append worked.
func TestPersistLearnedFact_FailedSemanticProjectionIsReported(t *testing.T) {
	eng := testTaxonomyEngine(t)
	root := t.TempDir()
	eng.SetWorkspace(root)
	// A classifier with a learned store but no embedding engine cannot embed.
	store, err := NewLearnedCorpusStore(nil, 3, nil)
	if err != nil {
		t.Fatal(err)
	}
	withSharedClassifier(t, NewSemanticClassifier(&mockKernel{}, nil, store, nil))

	out, err := eng.PersistLearnedFact(context.Background(), persistFact)
	if err != nil {
		t.Fatalf("authoritative stages succeeded; err = %v, want nil", err)
	}
	if out.File != ProjectionInstalled || out.Semantic != ProjectionFailed {
		t.Fatalf("outcome = %s, want file installed and semantic failed", out)
	}
	if out.Complete() || out.SemanticErr == nil || out.Err() == nil {
		t.Fatalf("a failed projection must make the outcome incomplete with its error: %+v", out)
	}
}

func TestPersistLearnedFact_CancelledBeforeStartChangesNothing(t *testing.T) {
	eng := testTaxonomyEngine(t)
	root := t.TempDir()
	eng.SetWorkspace(root)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	out, err := eng.PersistLearnedFact(ctx, persistFact)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if out.Taxonomy != ProjectionNotAttempted || out.File != ProjectionNotAttempted {
		t.Fatalf("outcome = %s, want nothing attempted", out)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".nerd", "mangle", "learned_taxonomy.mg")); !os.IsNotExist(statErr) {
		t.Fatalf("cancelled persist wrote the taxonomy file (stat err = %v)", statErr)
	}
}

// The file is the durable record; when it cannot be written the derived
// projections must not run, so recall never knows what the file does not.
func TestPersistLearnedFact_FileFailureStopsProjections(t *testing.T) {
	eng := testTaxonomyEngine(t)
	root := t.TempDir()
	eng.SetWorkspace(root)
	// .nerd/mangle is a regular file, so MkdirAll fails.
	if err := os.MkdirAll(filepath.Join(root, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".nerd", "mangle"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	engine := &ctxEngine{dims: 3}
	withSharedClassifier(t, memoryClassifier(t, engine))

	out, err := eng.PersistLearnedFact(context.Background(), persistFact)
	if err == nil {
		t.Fatal("file failure returned nil error")
	}
	if out.Taxonomy != ProjectionInstalled || out.File != ProjectionFailed || out.Semantic != ProjectionNotAttempted {
		t.Fatalf("outcome = %s, want taxonomy installed, file failed, semantic not attempted", out)
	}
	if engine.calls.Load() != 0 {
		t.Fatalf("embedding ran %d times after the durable record failed", engine.calls.Load())
	}
}

// Cancelling the operation reaches the embedding call: the semantic stage
// fails with the cancellation, the committed file record stays, and the
// outcome says so.
func TestPersistLearnedFact_CancellationReachesSemanticEmbedding(t *testing.T) {
	eng := testTaxonomyEngine(t)
	eng.SetWorkspace(t.TempDir())
	engine := &ctxEngine{dims: 3, block: true, entered: make(chan struct{}, 1)}
	withSharedClassifier(t, memoryClassifier(t, engine))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var out PersistOutcome
	var err error
	go func() { out, err = eng.PersistLearnedFact(ctx, persistFact); close(done) }()
	<-engine.entered
	cancel()
	<-done

	if err != nil {
		t.Fatalf("err = %v, want nil: the authoritative record committed", err)
	}
	if out.File != ProjectionInstalled || out.Semantic != ProjectionFailed || !errors.Is(out.SemanticErr, context.Canceled) {
		t.Fatalf("outcome = %s (semantic err %v), want file installed, semantic failed by cancellation", out, out.SemanticErr)
	}
}

// openBackendStore opens the real SQLite learned store behind the perception
// adapter, with an engine that honors its context.
func openBackendStore(t *testing.T, engine *ctxEngine) *LearnedCorpusStore {
	t.Helper()
	backend, err := storepkg.NewLearnedCorpusStoreWithContext(context.Background(), filepath.Join(t.TempDir(), "learned.db"), engine)
	if err != nil {
		t.Fatalf("open backend: %v", err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	return &LearnedCorpusStore{backend: backend, dimensions: engine.dims, embeddings: map[string][]float32{}}
}

// The adapter used to replace the operation context with context.Background()
// on the durable path, so a cancelled learn still ran its embedding and write.
func TestLearnedCorpusStore_Add_DurablePathObservesCancellation(t *testing.T) {
	engine := &ctxEngine{dims: 4, block: true, entered: make(chan struct{}, 1)}
	store := openBackendStore(t, engine)

	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() {
		errc <- store.Add(ctx, CorpusEntry{TextContent: "fix it", Verb: "/fix", Confidence: 0.9}, nil)
	}()
	<-engine.entered
	cancel()
	if err := <-errc; !errors.Is(err, context.Canceled) {
		t.Fatalf("Add err = %v, want context.Canceled from the backend embedding", err)
	}
}

func TestLearnedCorpusStore_Add_PreCancelledStartsNoWork(t *testing.T) {
	engine := &ctxEngine{dims: 4}
	store := openBackendStore(t, engine)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.Add(ctx, CorpusEntry{TextContent: "fix it", Verb: "/fix", Confidence: 0.9}, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("Add err = %v, want context.Canceled", err)
	}
	if engine.calls.Load() != 0 {
		t.Fatalf("pre-cancelled Add embedded %d times", engine.calls.Load())
	}
}

func TestSemanticClassifier_AddLearnedPattern_PreCancelledStartsNoWork(t *testing.T) {
	engine := &ctxEngine{dims: 3}
	sc := memoryClassifier(t, engine)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sc.AddLearnedPattern(ctx, "fix it", "/fix", "", "", 0.9); !errors.Is(err, context.Canceled) {
		t.Fatalf("AddLearnedPattern err = %v, want context.Canceled", err)
	}
	if engine.calls.Load() != 0 {
		t.Fatalf("pre-cancelled AddLearnedPattern embedded %d times", engine.calls.Load())
	}
}
