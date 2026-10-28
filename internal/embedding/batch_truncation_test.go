package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

// fakeTruncatingOllama is Ollama's documented embedding behavior at a model
// window of `window` characters: /api/embed truncates an oversized input to the
// window unless the request says "truncate": false, in which case it refuses
// the request; /api/embeddings always refuses an oversized prompt. A vector is
// [count of 'a', count of 'z', 1] of whatever text the model actually saw, so a
// vector with no 'z' component is the vector of a text whose tail was cut.
func fakeTruncatingOllama(t *testing.T, window int) (srv *httptest.Server, batchBodies *[]string) {
	t.Helper()
	var mu sync.Mutex
	var bodies []string
	vecOf := func(text string) []float32 {
		return []float32{float32(strings.Count(text, "a")), float32(strings.Count(text, "z")), 1}
	}
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(ollamaTagsResponse{Models: []ollamaTagModel{{Name: "trunc-model:latest"}}})
		case "/api/embed":
			raw := make([]byte, 0, 1024)
			buf := make([]byte, 1024)
			for {
				n, err := r.Body.Read(buf)
				raw = append(raw, buf[:n]...)
				if err != nil {
					break
				}
			}
			mu.Lock()
			bodies = append(bodies, string(raw))
			mu.Unlock()
			var req struct {
				Input    []string `json:"input"`
				Truncate *bool    `json:"truncate"`
			}
			if err := json.Unmarshal(raw, &req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			truncate := req.Truncate == nil || *req.Truncate // the documented default
			out := ollamaBatchEmbedResponse{}
			for _, in := range req.Input {
				if utf8.RuneCountInString(in) > window {
					if !truncate {
						http.Error(w, `{"error":"the input length exceeds the context length"}`, http.StatusBadRequest)
						return
					}
					in = string([]rune(in)[:window])
				}
				out.Embeddings = append(out.Embeddings, vecOf(in))
			}
			_ = json.NewEncoder(w).Encode(out)
		case "/api/embeddings":
			var req ollamaEmbedRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if utf8.RuneCountInString(req.Prompt) > window {
				http.Error(w, `{"error":"the input length exceeds the context length"}`, http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(ollamaEmbedResponse{Embedding: vecOf(req.Prompt)})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &bodies
}

// An oversized text in a native batch used to come back as a valid vector of
// its opening: /api/embed truncates by default, so the batch succeeded, the
// split-and-pool path in Embed never ran, and evidence that lives only in the
// tail was missing from the vector with nothing to say so.
func TestOllamaEmbedBatch_OversizedInputCoversTheTailNotTheOpening(t *testing.T) {
	srv, bodies := fakeTruncatingOllama(t, 200)
	e, err := NewOllamaEngine(srv.URL, "trunc-model", 3)
	if err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("a ", 150) + "\n\n" + strings.Repeat("z ", 150) // 'z' only in the second half
	texts := []string{"short a text", long}

	vecs, err := e.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("got %d vectors for 2 texts", len(vecs))
	}
	if vecs[1][1] == 0 {
		t.Fatalf("the long text's vector has no 'z' component %v: it embeds the truncated opening, not the whole input", vecs[1])
	}

	if len(*bodies) == 0 {
		t.Fatal("native batch endpoint was never used")
	}
	for _, body := range *bodies {
		if !strings.Contains(body, `"truncate":false`) {
			t.Fatalf("batch request %s does not set truncate:false: the server default silently cuts oversized input", body)
		}
	}
}

// Single and batch must agree on coverage: the same long text embeds to the
// same vector through Embed and through EmbedBatch.
func TestOllamaEmbed_SingleAndBatchCoverTheSameInput(t *testing.T) {
	srv, _ := fakeTruncatingOllama(t, 200)
	e, err := NewOllamaEngine(srv.URL, "trunc-model", 3)
	if err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("a ", 150) + "\n\n" + strings.Repeat("z ", 150)
	single, err := e.Embed(context.Background(), long)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	batch, err := e.EmbedBatch(context.Background(), []string{"x a", long})
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	for i := range single {
		if d := single[i] - batch[1][i]; d > 1e-5 || d < -1e-5 {
			t.Fatalf("single %v and batch %v vectors of the same text differ", single, batch[1])
		}
	}
}
