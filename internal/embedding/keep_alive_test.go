package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// embedding.ollama_keep_alive reaches both request shapes, so Ollama keeps
// the model loaded between turns instead of reloading it after five minutes.
func TestOllamaRequestsCarryKeepAlive(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(ollamaTagsResponse{Models: []ollamaTagModel{{Name: "keep-model:latest"}}})
		case "/api/embeddings":
			var req ollamaEmbedRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			mu.Lock()
			seen["single"] = req.KeepAlive
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(ollamaEmbedResponse{Embedding: []float32{1, 0, 0}})
		case "/api/embed":
			var req ollamaBatchEmbedRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			mu.Lock()
			seen["batch"] = req.KeepAlive
			mu.Unlock()
			out := ollamaBatchEmbedResponse{}
			for range req.Input {
				out.Embeddings = append(out.Embeddings, []float32{1, 0, 0})
			}
			_ = json.NewEncoder(w).Encode(out)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	engine, err := NewEngine(Config{Provider: "ollama", OllamaEndpoint: srv.URL, OllamaModel: "keep-model", Dimensions: 3, OllamaKeepAlive: "45m"})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if _, err := engine.Embed(context.Background(), "one"); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if _, err := engine.EmbedBatch(context.Background(), []string{"a", "b"}); err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if seen["single"] != "45m" || seen["batch"] != "45m" {
		t.Fatalf("keep_alive sent: %v, want 45m on both request shapes", seen)
	}
}
