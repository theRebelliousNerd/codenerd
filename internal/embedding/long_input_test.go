package embedding

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

// fakeWindowOllama refuses any prompt longer than window characters the way
// Ollama does (500, "exceeds the context length") and records every prompt.
func fakeWindowOllama(t *testing.T, model string, window int) (*httptest.Server, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var prompts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(ollamaTagsResponse{Models: []ollamaTagModel{{Name: model + ":latest"}}})
		case "/api/embeddings":
			var req ollamaEmbedRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			mu.Lock()
			prompts = append(prompts, req.Prompt)
			mu.Unlock()
			if utf8.RuneCountInString(req.Prompt) > window {
				http.Error(w, `{"error":"the input length exceeds the context length"}`, http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(ollamaEmbedResponse{Embedding: []float32{1, 0, 0}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &prompts
}

func TestOllamaEmbed_LongInputIsSplitAtParagraphsAndPooled(t *testing.T) {
	srv, prompts := fakeWindowOllama(t, "window-model", 300)
	e, err := NewOllamaEngine(srv.URL, "window-model", 3)
	if err != nil {
		t.Fatal(err)
	}
	para := strings.Repeat("word ", 50) // 250 chars
	text := para + "\n\n" + para + "\n\n" + para + "\n\n" + para

	vec, err := e.Embed(context.Background(), text)
	if err != nil {
		t.Fatalf("a text longer than the window must embed by splitting: %v", err)
	}
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	if len(vec) != 3 || math.Abs(norm-1) > 1e-5 {
		t.Fatalf("pooled vector = %v (norm %v), want a unit 3-vector", vec, norm)
	}
	for _, p := range *prompts {
		if utf8.RuneCountInString(p) <= 300 && strings.Contains(strings.TrimSpace(p), "\n\n") {
			t.Errorf("an accepted piece still spans a paragraph break the splitter should have used: %q", p)
		}
	}

	// The refusal taught the ceiling: an equally long text is split up front,
	// so no request at or above the refused length is sent again.
	*prompts = nil
	if _, err := e.Embed(context.Background(), text); err != nil {
		t.Fatal(err)
	}
	for _, p := range *prompts {
		if utf8.RuneCountInString(p) >= utf8.RuneCountInString(text) {
			t.Errorf("a text at the learned ceiling was sent whole again (%d chars)", utf8.RuneCountInString(p))
		}
	}
}

func TestOllamaEmbed_RefusalOfTinyPiecesIsAnError(t *testing.T) {
	srv, _ := fakeWindowOllama(t, "tiny-model", 10)
	e, err := NewOllamaEngine(srv.URL, "tiny-model", 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Embed(context.Background(), strings.Repeat("x", 500)); err == nil {
		t.Fatal("a model that refuses even the smallest piece must fail, not loop or invent a vector")
	}
}

func TestSplitForEmbeddingPrefersStrongBoundaries(t *testing.T) {
	a, b := splitForEmbedding("first sentence here. second sentence here.\n\nnext paragraph text here.")
	if !strings.HasSuffix(a, "second sentence here.") || !strings.HasPrefix(b, "next paragraph") {
		t.Fatalf("split = %q | %q, want the paragraph break", a, b)
	}
	a, b = splitForEmbedding(strings.Repeat("x", 100))
	if a == "" || b == "" || len(a)+len(b) != 100 {
		t.Fatalf("a text with no boundary must still split in two non-empty halves: %q | %q", a, b)
	}
}
