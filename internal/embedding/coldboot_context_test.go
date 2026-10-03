package embedding

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func awaitEmbeddingBootEvent[Value any](test *testing.T, events <-chan Value) Value {
	test.Helper()
	select {
	case value := <-events:
		return value
	case <-time.After(10 * time.Second):
		test.Fatal("embedding bootstrap event did not finish")
		var zero Value
		return zero
	}
}

func TestColdBootEmbedding_ParentCancellationStopsStartupHTTP(test *testing.T) {
	entered := make(chan struct{}, 1)
	joined := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		entered <- struct{}{}
		<-request.Context().Done()
		joined <- struct{}{}
	}))
	defer server.Close()
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		engine, err := NewEngineWithContext(parent, Config{Provider: "ollama", OllamaEndpoint: server.URL, OllamaModel: "coldboot-model", Dimensions: 2})
		if engine != nil {
			result <- errors.New("canceled constructor returned an engine")
			return
		}
		result <- err
	}()
	awaitEmbeddingBootEvent(test, entered)
	cancel()
	if err := awaitEmbeddingBootEvent(test, result); !errors.Is(err, context.Canceled) {
		test.Fatalf("constructor error = %v, want cancellation", err)
	}
	awaitEmbeddingBootEvent(test, joined)
}

func TestColdBootEmbedding_OptionalStartupCapDoesNotPoisonRetry(test *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		switch request.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(response).Encode(ollamaTagsResponse{Models: []ollamaTagModel{{Name: "coldboot-model"}}})
		case "/api/embeddings":
			_ = json.NewEncoder(response).Encode(ollamaEmbedResponse{Embedding: []float32{1, 2}})
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	parent := context.Background()
	engine, err := newEngineWithStartupTimeout(parent, Config{Provider: "ollama", OllamaEndpoint: server.URL, OllamaModel: "coldboot-model", Dimensions: 2}, 0)
	if err != nil || engine == nil {
		test.Fatalf("optional cap returned (%v, %v)", engine, err)
	}
	if requests.Load() != 0 {
		test.Fatal("expired startup cap admitted HTTP")
	}
	vector, err := engine.Embed(parent, "retry after optional startup cap")
	if err != nil || len(vector) != 2 || requests.Load() != 2 {
		test.Fatalf("retry vector=%v error=%v requests=%d", vector, err, requests.Load())
	}
}

func TestColdBootEmbedding_AliveParentOutageRemainsOptional(test *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	engine, err := NewEngineWithContext(context.Background(), Config{Provider: "ollama", OllamaEndpoint: server.URL, OllamaModel: "coldboot-model", Dimensions: 2})
	if err != nil || engine == nil {
		test.Fatalf("optional outage returned (%v, %v)", engine, err)
	}
}

func TestColdBootEmbedding_PreCanceledConstructorsPreserveCause(test *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	if engine, err := NewEngineWithContext(parent, Config{Provider: "ollama"}); engine != nil || !errors.Is(err, context.Canceled) {
		test.Fatalf("engine constructor returned (%v, %v)", engine, err)
	}
	if engine, err := NewGenAIEngineWithContext(parent, "", "", ""); engine != nil || !errors.Is(err, context.Canceled) {
		test.Fatalf("GenAI constructor returned (%v, %v)", engine, err)
	}
}

func TestColdBootEmbedding_ExpiredParentDeadlinePreservesCause(test *testing.T) {
	parent, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer cancel()
	if engine, err := NewEngineWithContext(parent, Config{Provider: "ollama"}); engine != nil || !errors.Is(err, context.DeadlineExceeded) {
		test.Fatalf("expired engine constructor returned (%v, %v)", engine, err)
	}
}
