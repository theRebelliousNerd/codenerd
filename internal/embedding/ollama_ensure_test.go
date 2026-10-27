package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolveInstalledModel_MatchesOnlyTheConfiguredModel(t *testing.T) {
	installed := []string{"embeddinggemma:300m", "nomic-embed-text:latest"}
	// A bare name is its :latest tag, as Ollama resolves it.
	if got := resolveInstalledModel("nomic-embed-text", installed); got != "nomic-embed-text:latest" {
		t.Fatalf("bare name: got %q want nomic-embed-text:latest", got)
	}
	if got := resolveInstalledModel("embeddinggemma:300m", installed); got != "embeddinggemma:300m" {
		t.Fatalf("exact tag: got %q", got)
	}
	// Another tag of the same family is a different model: no match.
	if got := resolveInstalledModel("embeddinggemma", installed); got != "" {
		t.Fatalf("bare name must not resolve to another tag, got %q", got)
	}
	if resolveInstalledModel("missing", installed) != "" {
		t.Fatal("expected empty for missing")
	}
}

func TestIsModelNotFoundStatus(t *testing.T) {
	body := `{"error":"model \"embeddinggemma\" not found, try pulling it first"}`
	if !isModelNotFoundStatus(404, body) {
		t.Fatal("expected 404 not-found body to match")
	}
	if isModelNotFoundStatus(500, "internal") {
		t.Fatal("500 should not match")
	}
}

func TestEnsureModel_BareNameResolvesToLatestOnly(t *testing.T) {
	var pulls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(ollamaTagsResponse{
				Models: []ollamaTagModel{
					{Name: "embeddinggemma:latest"},
					{Name: "nomic-embed-text:latest"},
				},
			})
			return
		}
		if r.URL.Path == "/api/pull" {
			pulls.Add(1)
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	engine, err := NewOllamaEngine(server.URL, "embeddinggemma", 768)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.EnsureModel(context.Background()); err != nil {
		t.Fatalf("EnsureModel: %v", err)
	}
	if engine.Model() != "embeddinggemma:latest" {
		t.Fatalf("model = %q want embeddinggemma:latest", engine.Model())
	}
	if pulls.Load() != 0 {
		t.Fatalf("an installed model must not be pulled, got %d pulls", pulls.Load())
	}
}

func TestEnsureModel_NeverSubstitutesAnotherModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(ollamaTagsResponse{
				Models: []ollamaTagModel{{Name: "nomic-embed-text:latest"}, {Name: "embeddinggemma:300m"}},
			})
			return
		}
		http.NotFound(w, r) // the pull fails
	}))
	defer server.Close()

	engine, err := NewOllamaEngine(server.URL, "embeddinggemma-2", 768)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.EnsureModel(context.Background()); err == nil {
		t.Fatal("a missing model whose pull fails must be an error, not a substitute")
	}
	if engine.Model() != "embeddinggemma-2" {
		t.Fatalf("model changed to %q", engine.Model())
	}
}

func TestEnsureModel_AutoPullsWhenMissing(t *testing.T) {
	var pulled atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			// First call empty; after pull, report installed.
			if pulled.Load() == 0 {
				_ = json.NewEncoder(w).Encode(ollamaTagsResponse{Models: nil})
				return
			}
			_ = json.NewEncoder(w).Encode(ollamaTagsResponse{
				Models: []ollamaTagModel{{Name: "embeddinggemma:300m"}},
			})
		case "/api/pull":
			pulled.Add(1)
			var req ollamaPullRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Name != "embeddinggemma:300m" {
				t.Errorf("pull name = %q", req.Name)
			}
			_ = json.NewEncoder(w).Encode(ollamaPullStatus{Status: "success"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	engine, err := NewOllamaEngine(server.URL, "embeddinggemma:300m", 768)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.EnsureModel(context.Background()); err != nil {
		t.Fatalf("EnsureModel: %v", err)
	}
	if pulled.Load() != 1 {
		t.Fatalf("expected 1 pull, got %d", pulled.Load())
	}
	if !engine.modelReady {
		t.Fatal("modelReady should be true")
	}
	// Second ensure is a no-op.
	if err := engine.EnsureModel(context.Background()); err != nil {
		t.Fatalf("second EnsureModel: %v", err)
	}
	if pulled.Load() != 1 {
		t.Fatalf("should not pull twice, got %d", pulled.Load())
	}
}

func TestEnsureModel_BootstrapDeadlineDoesNotPoisonLaterPull(t *testing.T) {
	var pullCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(ollamaTagsResponse{Models: nil})
		case "/api/pull":
			pullCalls.Add(1)
			if pullCalls.Load() == 1 {
				// Keep the first response beyond the bootstrap deadline. The
				// client must return on its context, while the bounded sleep lets
				// httptest.Server close deterministically on every platform.
				time.Sleep(100 * time.Millisecond)
				return
			}
			_ = json.NewEncoder(w).Encode(ollamaPullStatus{Status: "success"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	engine, err := NewOllamaEngine(server.URL, "test-model", 768)
	if err != nil {
		t.Fatal(err)
	}
	bootstrapCtx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := engine.EnsureModel(bootstrapCtx); err == nil {
		t.Fatal("EnsureModel bootstrap call succeeded; want deadline failure")
	}
	if engine.pullAttempted {
		t.Fatal("deadline-cancelled bootstrap pull permanently marked as attempted")
	}

	if err := engine.EnsureModel(context.Background()); err != nil {
		t.Fatalf("EnsureModel retry after bootstrap deadline: %v", err)
	}
	if !engine.modelReady {
		t.Fatal("retry did not mark model ready")
	}
	if pullCalls.Load() < 2 {
		t.Fatalf("pull calls = %d, want retry after cancelled bootstrap pull", pullCalls.Load())
	}
}

func TestEmbed_AutoPullsOn404(t *testing.T) {
	var embCalls, pullCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			if pullCalls.Load() == 0 {
				_ = json.NewEncoder(w).Encode(ollamaTagsResponse{Models: nil})
			} else {
				_ = json.NewEncoder(w).Encode(ollamaTagsResponse{
					Models: []ollamaTagModel{{Name: "embeddinggemma:300m"}},
				})
			}
		case "/api/pull":
			pullCalls.Add(1)
			_ = json.NewEncoder(w).Encode(ollamaPullStatus{Status: "success"})
		case "/api/embeddings":
			n := embCalls.Add(1)
			if n == 1 {
				// Simulate missing model on first embed (EnsureModel may have
				// already pulled; force a not-found path by failing first).
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":"model \"embeddinggemma:300m\" not found, try pulling it first"}`))
				return
			}
			_ = json.NewEncoder(w).Encode(ollamaEmbedResponse{Embedding: []float32{0.1, 0.2, 0.3}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	engine, err := NewOllamaEngine(server.URL, "embeddinggemma:300m", 3)
	if err != nil {
		t.Fatal(err)
	}
	// Skip preflight ready so first embed hits 404 path cleanly.
	engine.modelReady = true

	emb, err := engine.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Embed after auto-pull: %v", err)
	}
	if len(emb) != 3 {
		t.Fatalf("dims=%d", len(emb))
	}
	if pullCalls.Load() < 1 {
		t.Fatalf("expected pull on 404, got %d", pullCalls.Load())
	}
}
