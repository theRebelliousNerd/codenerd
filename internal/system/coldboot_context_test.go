package system

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/store"
)

type coldBootOutcome struct {
	cortex *Cortex
	err    error
}

type coldBootAdmissionContext struct {
	context.Context
	observed chan struct{}
	once     sync.Once
}

func (caller *coldBootAdmissionContext) Done() <-chan struct{} {
	caller.once.Do(func() { close(caller.observed) })
	return caller.Context.Done()
}

func awaitSystemBootEvent[Value any](test *testing.T, events <-chan Value) Value {
	test.Helper()
	select {
	case value := <-events:
		return value
	case <-time.After(30 * time.Second):
		test.Fatal("system bootstrap event did not finish")
		var zero Value
		return zero
	}
}

func TestColdBoot_DefaultBootstrapCancelsBlockedEmbeddingHTTP(test *testing.T) {
	if err := perception.ClosePerceptionLayer(); err != nil {
		test.Fatal(err)
	}
	entered := make(chan struct{})
	joined := make(chan struct{})
	var enteredOnce, joinedOnce sync.Once
	var embeddings atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/tags":
			_, _ = response.Write([]byte(`{"models":[{"name":"coldboot-model"}]}`))
		case "/api/embeddings":
			_, _ = io.Copy(io.Discard, request.Body)
			embeddings.Add(1)
			enteredOnce.Do(func() { close(entered) })
			<-request.Context().Done()
			joinedOnce.Do(func() { close(joined) })
		default:
			http.NotFound(response, request)
		}
	}))
	defer func() {
		server.CloseClientConnections()
		server.Close()
	}()
	workspace := test.TempDir()
	configuration := &config.UserConfig{Provider: "ollama", Engine: "api", Model: "coldboot-model"}
	configuration.Embedding = &config.EmbeddingConfig{Provider: "ollama", OllamaEndpoint: server.URL, OllamaModel: "coldboot-model", Dimensions: 2}
	encoded, err := json.Marshal(configuration)
	if err != nil {
		test.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, ".nerd"), 0o755); err != nil {
		test.Fatal(err)
	}
	configPath := filepath.Join(workspace, ".nerd", "config.json")
	if err := os.WriteFile(configPath, encoded, 0o600); err != nil {
		test.Fatal(err)
	}
	loaded, err := config.LoadUserConfig(configPath)
	if err != nil {
		test.Fatalf("production config loader rejected coldboot fixture: %v", err)
	}
	if loaded.Provider != "ollama" || loaded.GetEngine() != "api" || loaded.Model != "coldboot-model" {
		test.Fatal("production config loader changed the coldboot main provider settings")
	}
	if loaded.Embedding == nil || loaded.Embedding.Provider != "ollama" ||
		loaded.Embedding.OllamaEndpoint != server.URL || loaded.Embedding.OllamaModel != "coldboot-model" ||
		loaded.Embedding.Dimensions != 2 {
		test.Fatal("production config loader changed the coldboot embedding settings")
	}
	parent, cancel := context.WithCancel(context.Background())
	result := make(chan coldBootOutcome, 1)
	bootDone := make(chan struct{})
	var bootResult coldBootOutcome
	defer func() {
		cancel()
		<-bootDone
		if bootResult.cortex != nil {
			if err := bootResult.cortex.Close(); err != nil {
				test.Errorf("close unexpected production boot result: %v", err)
			}
		}
	}()
	go func() {
		defer close(bootDone)
		cortex, bootErr := BootCortex(parent, workspace, "", nil)
		bootResult = coldBootOutcome{cortex: cortex, err: bootErr}
		result <- bootResult
	}()
	admissionTimer := time.NewTimer(30 * time.Second)
	defer admissionTimer.Stop()
	select {
	case <-entered:
	case outcome := <-result:
		test.Fatalf("production boot ended before embedding HTTP admission: cortex=%v err=%v", outcome.cortex, outcome.err)
	case <-admissionTimer.C:
		test.Fatal("production boot did not admit embedding HTTP within the fixture watchdog")
	}
	cancel()
	outcome := awaitSystemBootEvent(test, result)
	if outcome.cortex != nil || !errors.Is(outcome.err, context.Canceled) || !strings.Contains(outcome.err.Error(), "boot kernel") {
		test.Fatalf("production boot returned (%v, %v)", outcome.cortex, outcome.err)
	}
	awaitSystemBootEvent(test, joined)
	if embeddings.Load() != 1 || perception.SharedSemanticClassifier != nil {
		test.Fatalf("embeddings=%d shared classifier=%v", embeddings.Load(), perception.SharedSemanticClassifier)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".nerd", "prompts", "corpus.db")); !os.IsNotExist(err) {
		test.Fatalf("later intelligence phase created corpus: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(workspace, ".nerd")); err != nil {
		test.Fatalf("partial production bootstrap retained workspace handles: %v", err)
	}
}

func TestColdBoot_CancellationAroundNilReturningStagesRollsBack(test *testing.T) {
	for _, canceledBefore := range []bool{true, false} {
		test.Run(map[bool]string{true: "before", false: "after"}[canceledBefore], func(test *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			if canceledBefore {
				cancel()
			}
			var database *store.LocalStore
			var admitted, later bool
			steps := []bootStep{
				{name: "cancel boundary", run: func(boot *bootContext) error {
					admitted = true
					var err error
					database, err = store.NewLocalStore(filepath.Join(test.TempDir(), "owned.db"))
					boot.localDB = database
					cancel()
					return err
				}},
				{name: "never admitted", run: func(boot *bootContext) error { later = true; return nil }},
			}
			cortex, err := bootCortexWithSteps(parent, BootConfig{}, steps)
			if cortex != nil || !errors.Is(err, context.Canceled) || admitted == canceledBefore || later {
				test.Fatalf("cortex=%v error=%v admitted=%v later=%v", cortex, err, admitted, later)
			}
			if database != nil && database.GetDB().Ping() == nil {
				test.Fatal("canceled nil-returning stage left its database open")
			}
		})
	}
}

type coldBootClosingEngine struct {
	closed   atomic.Int64
	closeErr error
}

func (engine *coldBootClosingEngine) Embed(context.Context, string) ([]float32, error) {
	return nil, nil
}

func (engine *coldBootClosingEngine) EmbedBatch(context.Context, []string) ([][]float32, error) {
	return nil, nil
}

func (engine *coldBootClosingEngine) Dimensions() int {
	return 2
}

func (engine *coldBootClosingEngine) Name() string {
	return "coldboot-closing"
}

func (engine *coldBootClosingEngine) Close() error {
	engine.closed.Add(1)
	return engine.closeErr
}

func TestColdBoot_CancellationJoinsRollbackFailure(test *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	closeFailure := errors.New("owned engine close failure")
	engine := &coldBootClosingEngine{closeErr: closeFailure}
	steps := []bootStep{{name: "cancel with owned engine", run: func(boot *bootContext) error {
		boot.embeddingEngine = engine
		cancel()
		return nil
	}}}
	cortex, err := bootCortexWithSteps(parent, BootConfig{}, steps)
	if cortex != nil || !errors.Is(err, context.Canceled) || !errors.Is(err, closeFailure) || engine.closed.Load() != 1 {
		test.Fatalf("cortex=%v error=%v closes=%d", cortex, err, engine.closed.Load())
	}
}

func TestColdBoot_CacheAdmissionObservesCallerCancellation(test *testing.T) {
	if err := cortexCacheMu.Acquire(context.Background(), 1); err != nil {
		test.Fatal(err)
	}
	defer cortexCacheMu.Release(1)
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	caller := &coldBootAdmissionContext{Context: parent, observed: make(chan struct{})}
	result := make(chan coldBootOutcome, 1)
	var admitted atomic.Bool
	workspace := test.TempDir()
	go func() {
		cortex, err := getOrBootCortex(caller, workspace, "", nil, func(context.Context, string, string, []string) (*Cortex, error) {
			admitted.Store(true)
			return &Cortex{}, nil
		})
		result <- coldBootOutcome{cortex: cortex, err: err}
	}()
	awaitSystemBootEvent(test, caller.observed)
	cancel()
	outcome := awaitSystemBootEvent(test, result)
	if outcome.cortex != nil || !errors.Is(outcome.err, context.Canceled) || admitted.Load() {
		test.Fatalf("blocked admission returned (%v, %v), admitted=%v", outcome.cortex, outcome.err, admitted.Load())
	}
}

func TestColdBoot_CanceledResultIsNotPublishedAndRetryReusesCache(test *testing.T) {
	workspace := test.TempDir()
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	closeFailure := errors.New("canceled result close failure")
	engine := &coldBootClosingEngine{closeErr: closeFailure}
	canceledCortex := &Cortex{EmbeddingEngine: engine}
	cortex, err := getOrBootCortex(parent, workspace, "", nil, func(context.Context, string, string, []string) (*Cortex, error) {
		cancel()
		return canceledCortex, nil
	})
	if cortex != nil || !errors.Is(err, context.Canceled) || !errors.Is(err, closeFailure) || engine.closed.Load() != 1 || canceledCortex.maintenanceDone != nil {
		test.Fatalf("canceled cache boot returned (%v, %v), closes=%d", cortex, err, engine.closed.Load())
	}
	var bootCalls int
	boot := func(caller context.Context, workspace string, apiKey string, disabled []string) (*Cortex, error) {
		if _, bounded := caller.Deadline(); bounded {
			return nil, errors.New("cache added a deadline")
		}
		bootCalls++
		return &Cortex{Workspace: workspace}, nil
	}
	retried, err := getOrBootCortex(context.Background(), workspace, "", nil, boot)
	if err != nil {
		test.Fatal(err)
	}
	defer retried.Close()
	again, err := getOrBootCortex(context.Background(), workspace, "", nil, boot)
	if err != nil || again != retried || bootCalls != 1 {
		test.Fatalf("retry reuse cortex=%v error=%v boot calls=%d", again, err, bootCalls)
	}
	expired, expiredCancel := context.WithCancel(context.Background())
	expiredCancel()
	if cached, err := getOrBootCortex(expired, workspace, "", nil, boot); cached != nil || !errors.Is(err, context.Canceled) {
		test.Fatalf("canceled cache hit returned (%v, %v)", cached, err)
	}
}
