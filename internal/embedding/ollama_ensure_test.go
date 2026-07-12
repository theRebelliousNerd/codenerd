package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestResolveInstalledModel_Prefers300m(t *testing.T) {
	installed := []string{"embeddinggemma:300m", "nomic-embed-text:latest"}
	got := resolveInstalledModel("embeddinggemma", installed)
	if got != "embeddinggemma:300m" {
		t.Fatalf("got %q want embeddinggemma:300m", got)
	}
	if resolveInstalledModel("nomic-embed-text", installed) != "nomic-embed-text:latest" {
		t.Fatalf("nomic resolve failed")
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

func TestEnsureModel_RemapsBareNameToInstalledTag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(ollamaTagsResponse{
				Models: []ollamaTagModel{
					{Name: "embeddinggemma:300m"},
					{Name: "nomic-embed-text:latest"},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	engine, err := NewOllamaEngine(server.URL, "embeddinggemma")
	if err != nil {
		t.Fatal(err)
	}
	// Constructor already normalized bare → :300m; reset to bare for this unit.
	engine.model = "embeddinggemma"

	if err := engine.EnsureModel(context.Background()); err != nil {
		t.Fatalf("EnsureModel: %v", err)
	}
	if engine.Model() != "embeddinggemma:300m" {
		t.Fatalf("model = %q want embeddinggemma:300m", engine.Model())
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

	engine, err := NewOllamaEngine(server.URL, "embeddinggemma:300m")
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

	engine, err := NewOllamaEngine(server.URL, "embeddinggemma:300m")
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

func TestPullTargetFor(t *testing.T) {
	if pullTargetFor("embeddinggemma") != defaultOllamaEmbedModel {
		t.Fatal(pullTargetFor("embeddinggemma"))
	}
	if pullTargetFor("") != defaultOllamaEmbedModel {
		t.Fatal("empty")
	}
	if !strings.HasPrefix(pullTargetFor("nomic-embed-text"), "nomic") {
		t.Fatal(pullTargetFor("nomic-embed-text"))
	}
}
