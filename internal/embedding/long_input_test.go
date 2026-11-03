package embedding

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/go-cmp/cmp"
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

func TestOllamaEmbed_LongUnicodeInputPoolsByRuneCount(t *testing.T) {
	for _, tt := range []struct {
		name                   string
		left, separator, right string
		window                 int
		rightVector            []float32
		want                   []float64
	}{
		{
			name: "minimum splittable length",
			left: strings.Repeat("界", 64), right: strings.Repeat("a", 64), window: 64,
			rightVector: []float32{0, 1}, want: []float64{1 / math.Sqrt(2), 1 / math.Sqrt(2)},
		},
		{
			name: "odd rune midpoint without a boundary",
			left: strings.Repeat("界", 65), right: strings.Repeat("a", 66), window: 66,
			rightVector: []float32{0, 1}, want: []float64{65 / math.Sqrt(8581), 66 / math.Sqrt(8581)},
		},
		{
			name: "unequal paragraphs with different byte widths",
			left: strings.Repeat("界", 100), separator: "\n\n", right: strings.Repeat("a", 200), window: 200,
			rightVector: []float32{0, 1}, want: []float64{1 / math.Sqrt(5), 2 / math.Sqrt(5)},
		},
		{
			name: "opposite vectors cancel without nonfinite output",
			left: strings.Repeat("界", 64), right: strings.Repeat("a", 64), window: 64,
			rightVector: []float32{-1, 0}, want: []float64{0, 0},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			text := tt.left + tt.separator + tt.right
			var mu sync.Mutex
			var prompts []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/tags":
					if err := json.NewEncoder(w).Encode(ollamaTagsResponse{Models: []ollamaTagModel{{Name: "unicode-model:latest"}}}); err != nil {
						t.Errorf("encode model list: %v", err)
					}
				case "/api/embeddings":
					var req ollamaEmbedRequest
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						t.Errorf("decode embedding request: %v", err)
						http.Error(w, "invalid request", http.StatusBadRequest)
						return
					}
					mu.Lock()
					prompts = append(prompts, req.Prompt)
					mu.Unlock()
					if utf8.RuneCountInString(req.Prompt) > tt.window {
						http.Error(w, "input exceeds the context length", http.StatusInternalServerError)
						return
					}
					var vector []float32
					switch req.Prompt {
					case tt.left:
						vector = []float32{1, 0}
					case tt.right:
						vector = tt.rightVector
					default:
						t.Errorf("unexpected split piece %q", req.Prompt)
						http.Error(w, "unexpected piece", http.StatusBadRequest)
						return
					}
					if err := json.NewEncoder(w).Encode(ollamaEmbedResponse{Embedding: vector}); err != nil {
						t.Errorf("encode embedding: %v", err)
					}
				default:
					t.Errorf("unexpected endpoint %q", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(srv.Close)
			engine, err := NewOllamaEngine(srv.URL, "unicode-model", 2)
			if err != nil {
				t.Fatal(err)
			}
			got, err := engine.Embed(t.Context(), text)
			if err != nil {
				t.Fatalf("Embed() error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("Embed() = %v, want %v", got, tt.want)
			}
			for i, value := range got {
				if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) || math.Abs(float64(value)-tt.want[i]) > 1e-6 {
					t.Errorf("Embed()[%d] = %v, want %v", i, value, tt.want[i])
				}
			}
			mu.Lock()
			gotPrompts := append([]string(nil), prompts...)
			mu.Unlock()
			if diff := cmp.Diff([]string{text, tt.left, tt.right}, gotPrompts); diff != "" {
				t.Errorf("provider prompts mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEmbedLong_PropagatesPieceErrorsWithoutPartialVector(t *testing.T) {
	left, right := strings.Repeat("l", 100), strings.Repeat("r", 100)
	text := left + "\n\n" + right
	pieceErr := errors.New("piece embedding unavailable")
	for _, tt := range []struct {
		name        string
		failedPiece string
		wantCalls   []string
	}{
		{name: "left stops before right", failedPiece: left, wantCalls: []string{text, left}},
		{name: "right discards successful left", failedPiece: right, wantCalls: []string{text, left, right}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var ceiling inputCeiling
			var calls []string
			got, err := embedLong(t.Context(), text, &ceiling, func(_ context.Context, piece string) ([]float32, error) {
				calls = append(calls, piece)
				switch piece {
				case text:
					return nil, &InputTooLongError{Chars: utf8.RuneCountInString(text), Model: "test-model"}
				case tt.failedPiece:
					return nil, fmt.Errorf("embed piece: %w", pieceErr)
				default:
					return []float32{1, 0}, nil
				}
			})
			if !errors.Is(err, pieceErr) {
				t.Fatalf("embedLong() error = %v, want wrapped piece error", err)
			}
			if got != nil {
				t.Errorf("embedLong() returned partial vector %v on error", got)
			}
			if diff := cmp.Diff(tt.wantCalls, calls); diff != "" {
				t.Errorf("piece calls mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
