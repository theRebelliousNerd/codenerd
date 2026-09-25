package embedding

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/genai"
)

// fakeGenAI stands in for Models.EmbedContent: it fails with each error in
// order, then answers with one vector per content.
type fakeGenAI struct {
	failures []error
	calls    atomic.Int32
}

func (f *fakeGenAI) embed(_ context.Context, _ string, contents []*genai.Content, _ *genai.EmbedContentConfig) (*genai.EmbedContentResponse, error) {
	n := int(f.calls.Add(1))
	if n <= len(f.failures) {
		return nil, f.failures[n-1]
	}
	resp := &genai.EmbedContentResponse{}
	for range contents {
		resp.Embeddings = append(resp.Embeddings, &genai.ContentEmbedding{Values: []float32{0.1, 0.2, 0.3}})
	}
	return resp, nil
}

func fakeGenAIEngine(f *fakeGenAI) *GenAIEngine {
	return &GenAIEngine{model: "gemini-embedding-001", taskType: "SEMANTIC_SIMILARITY", embed: f.embed}
}

func fastRetries(t *testing.T) {
	t.Helper()
	old := genaiRetryBackoff
	genaiRetryBackoff = time.Millisecond
	t.Cleanup(func() { genaiRetryBackoff = old })
}

// One 429 used to fail the embed: embedWithTask made one call and wrapped the
// failure.
func TestGenAIEngine_RetriesATransientFailure(t *testing.T) {
	fastRetries(t)
	f := &fakeGenAI{failures: []error{
		genai.APIError{Code: http.StatusTooManyRequests, Status: "429 Too Many Requests"},
		genai.APIError{Code: http.StatusServiceUnavailable, Status: "503 Service Unavailable"},
	}}
	vec, err := fakeGenAIEngine(f).Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("a 429 then a 503 failed the embed: %v", err)
	}
	if len(vec) != 3 || f.calls.Load() != 3 {
		t.Fatalf("got %d dims after %d calls, want 3 dims after 3 calls", len(vec), f.calls.Load())
	}

	// A batch chunk retries the same way.
	f = &fakeGenAI{failures: []error{genai.APIError{Code: http.StatusInternalServerError}}}
	vecs, err := fakeGenAIEngine(f).EmbedBatch(context.Background(), []string{"a", "b"})
	if err != nil || len(vecs) != 2 {
		t.Fatalf("a 500 failed the batch: %v (%d vectors)", err, len(vecs))
	}
}

func TestGenAIEngine_DoesNotRetryABadRequest(t *testing.T) {
	fastRetries(t)
	f := &fakeGenAI{failures: []error{genai.APIError{Code: http.StatusBadRequest}, nil}}
	if _, err := fakeGenAIEngine(f).Embed(context.Background(), "hello"); err == nil {
		t.Fatal("a 400 was retried into success; the request is what is wrong")
	}
	if n := f.calls.Load(); n != 1 {
		t.Errorf("a 400 was sent %d times, want 1", n)
	}
}

func TestGenAIEngine_GivesUpAfterItsAttempts(t *testing.T) {
	fastRetries(t)
	busy := genai.APIError{Code: http.StatusServiceUnavailable}
	f := &fakeGenAI{failures: []error{busy, busy, busy, busy}}
	if _, err := fakeGenAIEngine(f).Embed(context.Background(), "hello"); err == nil {
		t.Fatal("four 503s produced a vector")
	}
	if n := f.calls.Load(); n != genaiMaxAttempts {
		t.Errorf("made %d calls, want %d", n, genaiMaxAttempts)
	}
}

// The wait observes the caller's context (internal/embedding/agents.md).
func TestGenAIEngine_RetryWaitObservesTheContext(t *testing.T) {
	old := genaiRetryBackoff
	genaiRetryBackoff = time.Hour
	t.Cleanup(func() { genaiRetryBackoff = old })
	f := &fakeGenAI{failures: []error{genai.APIError{Code: http.StatusServiceUnavailable}}}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := fakeGenAIEngine(f).Embed(ctx, "hello")
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want the context's deadline", err)
	}
	if waited := time.Since(start); waited > 10*time.Second {
		t.Fatalf("the retry wait ignored the context: returned after %v", waited)
	}
}

// An unknown task type is refused before the network, at construction for the
// configured one and at the call for a caller's.
func TestGenAIEngine_RefusesAnUnknownTaskType(t *testing.T) {
	if _, err := NewGenAIEngine("test-key", "gemini-embedding-001", "SEMANTIC_SIMILARTY"); err == nil ||
		!strings.Contains(err.Error(), "SEMANTIC_SIMILARTY") {
		t.Fatalf("a misspelled configured task type was accepted: %v", err)
	}
	f := &fakeGenAI{}
	e := fakeGenAIEngine(f)
	if _, err := e.EmbedWithTask(context.Background(), "hello", "retrieval_docs"); err == nil {
		t.Fatal("an unknown task type reached the provider")
	}
	if _, err := e.EmbedBatchWithTask(context.Background(), []string{"a"}, "nope"); err == nil {
		t.Fatal("an unknown batch task type reached the provider")
	}
	if n := f.calls.Load(); n != 0 {
		t.Errorf("the provider was called %d times for unknown task types", n)
	}
	// A known one, in any case, still works.
	if _, err := e.EmbedWithTask(context.Background(), "hello", "retrieval_document"); err != nil {
		t.Errorf("a known task type in lower case was refused: %v", err)
	}
}

// Empty text is refused before any request, by both engines.
func TestEngines_RejectEmptyTextBeforeTheProvider(t *testing.T) {
	f := &fakeGenAI{}
	e := fakeGenAIEngine(f)
	if _, err := e.Embed(context.Background(), "  \n"); !errors.Is(err, ErrEmptyText) {
		t.Errorf("GenAI Embed of whitespace: err = %v, want ErrEmptyText", err)
	}
	if _, err := e.EmbedBatch(context.Background(), []string{"a", ""}); !errors.Is(err, ErrEmptyText) || !strings.Contains(err.Error(), "text 1 of 2") {
		t.Errorf("GenAI EmbedBatch with an empty text: err = %v, want ErrEmptyText naming text 1", err)
	}
	if n := f.calls.Load(); n != 0 {
		t.Errorf("GenAI was called %d times for empty text", n)
	}

	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"embedding": []}`))
	}))
	defer srv.Close()
	o, err := NewOllamaEngine(srv.URL, "nomic-embed-text", 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := o.Embed(context.Background(), ""); !errors.Is(err, ErrEmptyText) {
		t.Errorf("Ollama Embed of empty text: err = %v, want ErrEmptyText", err)
	}
	if _, err := o.EmbedBatch(context.Background(), []string{"", "b"}); !errors.Is(err, ErrEmptyText) {
		t.Errorf("Ollama EmbedBatch with an empty text: err = %v, want ErrEmptyText", err)
	}
	if n := requests.Load(); n != 0 {
		t.Errorf("Ollama received %d requests for empty text", n)
	}
}

// A content_type the selector does not know is not trusted verbatim: the text
// decides. It used to become an unknown ContentType and select
// SEMANTIC_SIMILARITY without the text being looked at.
func TestDetectContentType_WhenMetadataNamesNoKnownType_ShouldDetectFromTheText(t *testing.T) {
	code := "package main\n\nfunc main() {\n\tx := 1 // comment\n}\n"
	if got := DetectContentType(code, map[string]any{"content_type": "sourcecode"}); got != ContentTypeCode {
		t.Errorf("unknown content_type: detected %q, want %q from the text", got, ContentTypeCode)
	}
	if got := DetectContentType("anything", map[string]any{"content_type": " Code "}); got != ContentTypeCode {
		t.Errorf("content_type %q: got %q, want the known %q", " Code ", got, ContentTypeCode)
	}
	if got := GetOptimalTaskType(code, map[string]any{"content_type": "sourcecode"}, false); got != "RETRIEVAL_DOCUMENT" {
		t.Errorf("task type for code with a misspelled content_type = %s, want RETRIEVAL_DOCUMENT", got)
	}
}
