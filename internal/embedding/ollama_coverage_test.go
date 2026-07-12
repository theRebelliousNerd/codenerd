package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// =============================================================================
// NewOllamaEngine Tests
// =============================================================================

func TestNewOllamaEngine_WhenDefaultParams_ShouldUseDefaults(t *testing.T) {
	engine, err := NewOllamaEngine("", "")
	if err != nil {
		t.Fatalf("NewOllamaEngine returned error: %v", err)
	}
	if engine == nil {
		t.Fatal("NewOllamaEngine returned nil")
	}
	if engine.model != defaultOllamaEmbedModel {
		t.Errorf("default model = %q, want %q", engine.model, defaultOllamaEmbedModel)
	}
	if engine.endpoint != "http://localhost:11434" {
		t.Errorf("endpoint = %q, want %q", engine.endpoint, "http://localhost:11434")
	}
}

func TestNewOllamaEngine_WhenCustomParams_ShouldRetainValues(t *testing.T) {
	engine, err := NewOllamaEngine("http://custom:9999", "custom-model")
	if err != nil {
		t.Fatalf("NewOllamaEngine returned error: %v", err)
	}
	if engine.endpoint != "http://custom:9999" {
		t.Errorf("endpoint = %q, want %q", engine.endpoint, "http://custom:9999")
	}
	if engine.model != "custom-model" {
		t.Errorf("model = %q, want %q", engine.model, "custom-model")
	}
}

func TestOllamaEngine_Dimensions_ShouldReturn768(t *testing.T) {
	engine, err := NewOllamaEngine("", "")
	if err != nil {
		t.Fatalf("NewOllamaEngine returned error: %v", err)
	}
	if engine.Dimensions() != 768 {
		t.Errorf("Dimensions() = %d, want 768", engine.Dimensions())
	}
}

func TestOllamaEngine_Name_ShouldIncludeModel(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected string
	}{
		// bare embeddinggemma is normalized to the tagged default
		{"default model", "embeddinggemma", "ollama:" + defaultOllamaEmbedModel},
		{"custom model", "nomic-embed-text", "ollama:nomic-embed-text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := NewOllamaEngine("", tt.model)
			if err != nil {
				t.Fatalf("NewOllamaEngine returned error: %v", err)
			}
			if engine.Name() != tt.expected {
				t.Errorf("Name() = %q, want %q", engine.Name(), tt.expected)
			}
		})
	}
}

// =============================================================================
// Ollama Embed Tests (with httptest mock server)
// =============================================================================

// skipEnsure marks the engine as model-ready so unit tests that only mock
// /api/embeddings are not forced through /api/tags + /api/pull.
func skipEnsure(e *OllamaEngine) *OllamaEngine {
	e.modelReady = true
	return e
}

func TestOllamaEngine_Embed_WhenServerReturnsEmbedding_ShouldSucceed(t *testing.T) {
	expectedEmb := []float32{0.1, 0.2, 0.3, 0.4, 0.5}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embeddings" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Verify request body
		var req ollamaEmbedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if req.Model != "test-model" {
			t.Errorf("request model = %q, want %q", req.Model, "test-model")
		}
		if req.Prompt != "hello world" {
			t.Errorf("request prompt = %q, want %q", req.Prompt, "hello world")
		}

		resp := ollamaEmbedResponse{Embedding: expectedEmb}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	engine, err := NewOllamaEngine(server.URL, "test-model")
	if err != nil {
		t.Fatalf("NewOllamaEngine returned error: %v", err)
	}
	skipEnsure(engine)

	emb, err := engine.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("Embed returned error: %v", err)
	}
	if len(emb) != len(expectedEmb) {
		t.Fatalf("Embed returned %d dims, want %d", len(emb), len(expectedEmb))
	}
	for i, v := range emb {
		if v != expectedEmb[i] {
			t.Errorf("emb[%d] = %f, want %f", i, v, expectedEmb[i])
		}
	}
}

func TestOllamaEngine_Embed_WhenServerReturns500_ShouldRetryAndFail(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	engine, err := NewOllamaEngine(server.URL, "test-model")
	if err != nil {
		t.Fatalf("NewOllamaEngine returned error: %v", err)
	}
	skipEnsure(engine)

	emb, err := engine.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("Embed should return error on 500 response")
	}
	if emb != nil {
		t.Errorf("Embed should return nil embedding on error, got %v", emb)
	}
	// Should have retried 3 times (maxRetries = 3)
	if callCount != 3 {
		t.Errorf("Expected 3 retries, got %d calls", callCount)
	}
}

func TestOllamaEngine_Embed_WhenServerReturns400_ShouldNotRetry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	engine, err := NewOllamaEngine(server.URL, "test-model")
	if err != nil {
		t.Fatalf("NewOllamaEngine returned error: %v", err)
	}
	skipEnsure(engine)

	emb, err := engine.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("Embed should return error on 400 response")
	}
	if emb != nil {
		t.Errorf("Embed should return nil embedding on error")
	}
	// 400 is not retryable, should only call once
	if callCount != 1 {
		t.Errorf("Expected 1 call (no retry for 400), got %d", callCount)
	}
}

func TestOllamaEngine_Embed_WhenContextCancelled_ShouldReturnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow handler - should be cancelled
		select {
		case <-r.Context().Done():
			return
		}
	}))
	defer server.Close()

	engine, err := NewOllamaEngine(server.URL, "test-model")
	if err != nil {
		t.Fatalf("NewOllamaEngine returned error: %v", err)
	}
	skipEnsure(engine)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	emb, err := engine.Embed(ctx, "test")
	if err == nil {
		t.Fatal("Embed with cancelled context should return error")
	}
	if emb != nil {
		t.Errorf("Embed should return nil on cancelled context")
	}
}

func TestOllamaEngine_Embed_WhenInvalidJSON_ShouldRetryAndFail(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not valid json{{{"))
	}))
	defer server.Close()

	engine, err := NewOllamaEngine(server.URL, "test-model")
	if err != nil {
		t.Fatalf("NewOllamaEngine returned error: %v", err)
	}
	skipEnsure(engine)

	emb, err := engine.Embed(context.Background(), "test")
	if err == nil {
		t.Fatal("Embed with invalid JSON response should return error")
	}
	if emb != nil {
		t.Errorf("Embed should return nil on decode error")
	}
	// Invalid JSON is retried
	if callCount != 3 {
		t.Errorf("Expected 3 retries for decode error, got %d", callCount)
	}
}

func TestOllamaEngine_Embed_WhenEmptyText_ShouldStillCallServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaEmbedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if req.Prompt != "" {
			t.Errorf("expected empty prompt, got %q", req.Prompt)
		}

		resp := ollamaEmbedResponse{Embedding: []float32{0.0, 0.0}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	engine, err := NewOllamaEngine(server.URL, "test-model")
	if err != nil {
		t.Fatalf("NewOllamaEngine returned error: %v", err)
	}
	skipEnsure(engine)

	emb, err := engine.Embed(context.Background(), "")
	if err != nil {
		t.Fatalf("Embed with empty text returned error: %v", err)
	}
	if len(emb) != 2 {
		t.Errorf("Expected 2-dim embedding, got %d", len(emb))
	}
}

// =============================================================================
// Ollama EmbedBatch Tests
// =============================================================================

func TestOllamaEngine_EmbedBatch_WhenEmpty_ShouldReturnNil(t *testing.T) {
	engine, err := NewOllamaEngine("http://localhost:11434", "test-model")
	if err != nil {
		t.Fatalf("NewOllamaEngine returned error: %v", err)
	}

	result, err := engine.EmbedBatch(context.Background(), []string{})
	if err != nil {
		t.Fatalf("EmbedBatch(empty) returned error: %v", err)
	}
	if result != nil {
		t.Errorf("EmbedBatch(empty) should return nil, got %v", result)
	}
}

func TestOllamaEngine_EmbedBatch_WhenMultipleTexts_ShouldCallEmbedForEach(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embeddings" {
			return
		}
		callCount++
		resp := ollamaEmbedResponse{Embedding: []float32{float32(callCount), 0.0}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	engine, err := NewOllamaEngine(server.URL, "test-model")
	if err != nil {
		t.Fatalf("NewOllamaEngine returned error: %v", err)
	}
	skipEnsure(engine)

	texts := []string{"text1", "text2", "text3"}
	results, err := engine.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedBatch returned error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("EmbedBatch returned %d results, want 3", len(results))
	}
	if callCount != 3 {
		t.Errorf("Expected 3 server calls, got %d", callCount)
	}
}

func TestOllamaEngine_EmbedBatch_WhenOneTextFails_ShouldReturnError(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embeddings" {
			return
		}
		callCount++
		if callCount == 2 {
			// Fail on second call (non-retryable)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		resp := ollamaEmbedResponse{Embedding: []float32{0.1, 0.2}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	engine, err := NewOllamaEngine(server.URL, "test-model")
	if err != nil {
		t.Fatalf("NewOllamaEngine returned error: %v", err)
	}
	skipEnsure(engine)

	results, err := engine.EmbedBatch(context.Background(), []string{"a", "b", "c"})
	if err == nil {
		t.Fatal("EmbedBatch should return error when one embed fails")
	}
	if results != nil {
		t.Errorf("EmbedBatch should return nil on error, got %v", results)
	}
}

// =============================================================================
// Ollama HealthCheck Tests
// =============================================================================

func TestOllamaEngine_HealthCheck_WhenHealthy_ShouldReturnNil(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("HealthCheck called unexpected path: %s", r.URL.Path)
		}
		if r.Method != "GET" {
			t.Errorf("HealthCheck used unexpected method: %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"models":[]}`))
	}))
	defer server.Close()

	engine, err := NewOllamaEngine(server.URL, "test-model")
	if err != nil {
		t.Fatalf("NewOllamaEngine returned error: %v", err)
	}

	err = engine.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("HealthCheck returned error for healthy server: %v", err)
	}
}

func TestOllamaEngine_HealthCheck_WhenUnhealthy_ShouldReturnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	engine, err := NewOllamaEngine(server.URL, "test-model")
	if err != nil {
		t.Fatalf("NewOllamaEngine returned error: %v", err)
	}

	err = engine.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("HealthCheck should return error for unhealthy server")
	}
}

func TestOllamaEngine_HealthCheck_WhenUnreachable_ShouldReturnError(t *testing.T) {
	engine, err := NewOllamaEngine("http://127.0.0.1:1", "test-model")
	if err != nil {
		t.Fatalf("NewOllamaEngine returned error: %v", err)
	}

	err = engine.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("HealthCheck should return error for unreachable server")
	}
}

// =============================================================================
// Ollama HealthChecker Interface Compliance
// =============================================================================

func TestOllamaEngine_ImplementsHealthChecker(t *testing.T) {
	engine, err := NewOllamaEngine("", "")
	if err != nil {
		t.Fatalf("NewOllamaEngine returned error: %v", err)
	}
	var hc HealthChecker = engine
	if hc == nil {
		t.Fatal("OllamaEngine should implement HealthChecker")
	}
}

func TestOllamaEngine_ImplementsEmbeddingEngine(t *testing.T) {
	engine, err := NewOllamaEngine("", "")
	if err != nil {
		t.Fatalf("NewOllamaEngine returned error: %v", err)
	}
	var ee EmbeddingEngine = engine
	if ee == nil {
		t.Fatal("OllamaEngine should implement EmbeddingEngine")
	}
}

// =============================================================================
// Ollama Embed with "connection was forcibly closed" retry
// =============================================================================

func TestOllamaEngine_Embed_WhenForciblyClosedMessage_ShouldRetry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			// Return a 400 with "connection was forcibly closed" (special retryable case)
			http.Error(w, "connection was forcibly closed by remote host", http.StatusBadRequest)
			return
		}
		resp := ollamaEmbedResponse{Embedding: []float32{0.1, 0.2, 0.3}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	engine, err := NewOllamaEngine(server.URL, "test-model")
	if err != nil {
		t.Fatalf("NewOllamaEngine returned error: %v", err)
	}

	emb, err := engine.Embed(context.Background(), "test")
	if err != nil {
		t.Fatalf("Embed should succeed after retries: %v", err)
	}
	if len(emb) != 3 {
		t.Errorf("Expected 3-dim embedding, got %d", len(emb))
	}
	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}
}
