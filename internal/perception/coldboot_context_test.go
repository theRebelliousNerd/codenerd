package perception

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/embedding"
)

type coldClassifierOutcome struct {
	classifier *SemanticClassifier
	err        error
}

type coldClassifierAdmissionContext struct {
	context.Context
	observed chan struct{}
	once     sync.Once
}

func (caller *coldClassifierAdmissionContext) Done() <-chan struct{} {
	caller.once.Do(func() { close(caller.observed) })
	return caller.Context.Done()
}

func awaitPerceptionBootEvent[Value any](test *testing.T, events <-chan Value) Value {
	test.Helper()
	select {
	case value := <-events:
		return value
	case <-time.After(10 * time.Second):
		test.Fatal("perception bootstrap event did not finish")
		var zero Value
		return zero
	}
}

func coldClassifierWorkspace(test *testing.T) string {
	test.Helper()
	previous := SharedTaxonomy
	workspace := test.TempDir()
	SharedTaxonomy = &TaxonomyEngine{workspaceRoot: workspace}
	test.Cleanup(func() { SharedTaxonomy = previous })
	return workspace
}

func coldClassifierKernel(count int) *mockKernel {
	facts := make([]core.Fact, count)
	for index := range facts {
		facts[index] = core.Fact{Predicate: "intent_definition", Args: []any{fmt.Sprintf("cold intent %d", index), "/review", ""}}
	}
	return &mockKernel{assertedFacts: facts}
}

func TestColdBootPerception_CompletedChunksSurviveCanceledConstructorAndRetry(test *testing.T) {
	workspace := coldClassifierWorkspace(test)
	entered := make(chan struct{}, 1)
	joined := make(chan struct{}, 1)
	var embeds atomic.Int64
	var retry atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/tags":
			_, _ = response.Write([]byte(`{"models":[{"name":"coldboot-model"}]}`))
		case "/api/embeddings":
			_, _ = io.Copy(io.Discard, request.Body)
			if embeds.Add(1) == int64(intentEmbedChunkSize+1) && !retry.Load() {
				entered <- struct{}{}
				<-request.Context().Done()
				joined <- struct{}{}
				return
			}
			_, _ = response.Write([]byte(`{"embedding":[1,2]}`))
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	configuration := &config.UserConfig{Embedding: &config.EmbeddingConfig{Provider: "ollama", OllamaEndpoint: server.URL, OllamaModel: "coldboot-model", Dimensions: 2}}
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	kernel := coldClassifierKernel(intentEmbedChunkSize + 8)
	result := make(chan coldClassifierOutcome, 1)
	go func() {
		classifier, err := NewSemanticClassifierFromConfigWithContext(parent, kernel, configuration)
		result <- coldClassifierOutcome{classifier: classifier, err: err}
	}()
	awaitPerceptionBootEvent(test, entered)
	cancel()
	outcome := awaitPerceptionBootEvent(test, result)
	if outcome.classifier != nil || !errors.Is(outcome.err, context.Canceled) {
		test.Fatalf("canceled constructor returned (%v, %v)", outcome.classifier, outcome.err)
	}
	awaitPerceptionBootEvent(test, joined)
	cachePath := filepath.Join(workspace, ".nerd", "intent_embeddings.db")
	reopened, err := NewEmbeddedCorpusStoreWithCacheWithContext(context.Background(), 2, cachePath)
	if err != nil {
		test.Fatal(err)
	}
	var persisted int
	if err := reopened.cacheDB.QueryRow("SELECT COUNT(*) FROM embedding_cache").Scan(&persisted); err != nil {
		test.Fatal(err)
	}
	if err := reopened.Close(); err != nil {
		test.Fatal(err)
	}
	if persisted != intentEmbedChunkSize {
		test.Fatalf("persisted entries=%d, want completed chunk %d", persisted, intentEmbedChunkSize)
	}
	retry.Store(true)
	beforeRetry := embeds.Load()
	classifier, err := NewSemanticClassifierFromConfigWithContext(context.Background(), kernel, configuration)
	if err != nil {
		test.Fatal(err)
	}
	if len(classifier.embeddedStore.entries) != intentEmbedChunkSize+8 || embeds.Load()-beforeRetry != 8 {
		test.Fatalf("retry entries=%d requests=%d", len(classifier.embeddedStore.entries), embeds.Load()-beforeRetry)
	}
	if err := classifier.Close(); err != nil {
		test.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(workspace, ".nerd")); err != nil {
		test.Fatalf("constructor/retry retained cache or learned-store handles: %v", err)
	}
}

type coldClassifierClosingEngine struct {
	entered  chan struct{}
	closed   atomic.Int64
	closeErr error
}

func (engine *coldClassifierClosingEngine) Dimensions() int {
	return 2
}

func (engine *coldClassifierClosingEngine) Name() string {
	return "coldboot-owned"
}

func (engine *coldClassifierClosingEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
func (engine *coldClassifierClosingEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	engine.entered <- struct{}{}
	<-ctx.Done()
	return nil, ctx.Err()
}
func (engine *coldClassifierClosingEngine) Close() error {
	engine.closed.Add(1)
	return engine.closeErr
}

func TestColdBootPerception_CanceledConstructorClosesOwnedEngineAndJoinsFailure(test *testing.T) {
	workspace := coldClassifierWorkspace(test)
	closeFailure := errors.New("classifier engine close failed")
	engine := &coldClassifierClosingEngine{entered: make(chan struct{}, 1), closeErr: closeFailure}
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan coldClassifierOutcome, 1)
	go func() {
		classifier, err := newSemanticClassifierFromFactory(parent, coldClassifierKernel(1), &config.UserConfig{}, intentHydrateTimeout,
			func(context.Context, embedding.Config) (embedding.EmbeddingEngine, error) { return engine, nil })
		result <- coldClassifierOutcome{classifier: classifier, err: err}
	}()
	awaitPerceptionBootEvent(test, engine.entered)
	cancel()
	outcome := awaitPerceptionBootEvent(test, result)
	if outcome.classifier != nil || !errors.Is(outcome.err, context.Canceled) || !errors.Is(outcome.err, closeFailure) || engine.closed.Load() != 1 {
		test.Fatalf("constructor=(%v, %v) engine closes=%d", outcome.classifier, outcome.err, engine.closed.Load())
	}
	if err := os.RemoveAll(filepath.Join(workspace, ".nerd")); err != nil {
		test.Fatalf("failed constructor retained cache handles: %v", err)
	}
}

func TestColdBootPerception_AliveParentHydrationCapKeepsOwnedClassifier(test *testing.T) {
	coldClassifierWorkspace(test)
	engine := &coldClassifierClosingEngine{entered: make(chan struct{}, 1)}
	classifier, err := newSemanticClassifierFromFactory(context.Background(), coldClassifierKernel(1), &config.UserConfig{}, 0,
		func(parent context.Context, configuration embedding.Config) (embedding.EmbeddingEngine, error) {
			if _, bounded := parent.Deadline(); bounded {
				return nil, errors.New("constructor added a parent deadline")
			}
			return engine, nil
		})
	if err != nil || classifier == nil || classifier.learnedStore == nil || engine.closed.Load() != 0 {
		test.Fatalf("optional hydration cap returned (%v, %v), closes=%d", classifier, err, engine.closed.Load())
	}
	if err := classifier.Close(); err != nil || engine.closed.Load() != 1 {
		test.Fatalf("owned classifier cleanup error=%v closes=%d", err, engine.closed.Load())
	}
}

func TestColdBootPerception_SharedAdmissionObservesCallerCancellation(test *testing.T) {
	if err := sharedClassifierMu.Acquire(context.Background(), 1); err != nil {
		test.Fatal(err)
	}
	defer sharedClassifierMu.Release(1)
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	caller := &coldClassifierAdmissionContext{Context: parent, observed: make(chan struct{})}
	result := make(chan error, 1)
	go func() {
		result <- InitSemanticClassifierWithContext(caller, coldClassifierKernel(1), &config.UserConfig{})
	}()
	awaitPerceptionBootEvent(test, caller.observed)
	cancel()
	if err := awaitPerceptionBootEvent(test, result); !errors.Is(err, context.Canceled) {
		test.Fatalf("blocked shared admission error=%v", err)
	}
}

func TestColdBootPerception_PreCanceledConstructionDoesNotCreateCache(test *testing.T) {
	cachePath := filepath.Join(test.TempDir(), "uncreated", "cache.db")
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	if corpus, err := NewEmbeddedCorpusStoreWithCacheWithContext(parent, 2, cachePath); corpus != nil || !errors.Is(err, context.Canceled) {
		test.Fatalf("cache constructor returned (%v, %v)", corpus, err)
	}
	if _, err := os.Stat(filepath.Dir(cachePath)); !os.IsNotExist(err) {
		test.Fatalf("canceled constructor touched cache directory: %v", err)
	}
	if classifier, err := NewSemanticClassifierFromConfigWithContext(parent, nil, nil); classifier != nil || !errors.Is(err, context.Canceled) {
		test.Fatalf("classifier constructor returned (%v, %v)", classifier, err)
	}
}

func TestColdBootPerception_CacheSQLAdmissionCancelsBeforeEmbedding(test *testing.T) {
	corpus, err := NewEmbeddedCorpusStoreWithCacheWithContext(context.Background(), 4, filepath.Join(test.TempDir(), "cache.db"))
	if err != nil {
		test.Fatal(err)
	}
	defer corpus.Close()
	corpus.cacheDB.SetMaxOpenConns(1)
	connection, err := corpus.cacheDB.Conn(context.Background())
	if err != nil {
		test.Fatal(err)
	}
	defer connection.Close()
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	caller := &coldClassifierAdmissionContext{Context: parent, observed: make(chan struct{})}
	engine := &mockBatchEmbedEngine{dims: 4}
	result := make(chan error, 1)
	go func() {
		result <- corpus.LoadFromKernelWithContext(caller, coldClassifierKernel(1), engine)
	}()
	awaitPerceptionBootEvent(test, caller.observed)
	cancel()
	if err := awaitPerceptionBootEvent(test, result); !errors.Is(err, context.Canceled) || engine.callCount != 0 {
		test.Fatalf("cache SQL admission error=%v embedding calls=%d", err, engine.callCount)
	}
}

func TestColdBootPerception_AliveParentUnavailableEngineDegrades(test *testing.T) {
	classifier, err := NewSemanticClassifierFromConfigWithContext(context.Background(), coldClassifierKernel(1), &config.UserConfig{Embedding: &config.EmbeddingConfig{Provider: "coldboot-unavailable"}})
	if err != nil || classifier == nil || classifier.embedEngine != nil {
		test.Fatalf("optional engine failure returned (%v, %v)", classifier, err)
	}
	if err := classifier.Close(); err != nil {
		test.Fatal(err)
	}
}
