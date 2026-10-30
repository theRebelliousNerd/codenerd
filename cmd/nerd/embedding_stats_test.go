package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/store"
)

// statsEngine is a deterministic embedding engine for the fixture.
type statsEngine struct{}

func (statsEngine) Embed(_ context.Context, text string) ([]float32, error) {
	return []float32{float32(len(text)), 1, 0, 0}, nil
}
func (e statsEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i], _ = e.Embed(ctx, t)
	}
	return out, nil
}
func (statsEngine) Dimensions() int { return 4 }
func (statsEngine) Name() string    { return "fixture:stats-4" }

func vecIndexRows(t *testing.T, ls *store.LocalStore) int64 {
	t.Helper()
	var n int64
	if err := ls.GetDB().QueryRow(`SELECT COUNT(*) FROM vec_index`).Scan(&n); err != nil {
		t.Fatalf("count vec_index: %v", err)
	}
	return n
}

// `nerd embedding stats` attached the configured engine to describe the
// table. Attachment drops and rebuilds vec_index and runs the provider's
// readiness check (an Ollama model list, and a pull when the model is
// missing); the command then closed the store, possibly before the rebuild,
// so asking for statistics could leave the index emptier than it found it
// (audit embedding-stats-observation-boundary). Stats reads; it attaches
// nothing and calls no provider.
func TestEmbeddingStats_ObservesWithoutAttachingAnEngine(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(ws, ".nerd", "knowledge.db")
	ls, err := store.NewLocalStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	ls.SetEmbeddingEngine(statsEngine{})
	ctx := context.Background()
	for _, c := range []string{"alpha", "beta", "gamma"} {
		if err := ls.StoreVectorWithEmbedding(ctx, c, map[string]any{"path": c}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ls.ReconcileVecIndex(ctx); err != nil {
		t.Fatal(err)
	}
	indexed := vecIndexRows(t, ls)
	if indexed != 3 {
		_ = ls.Close()
		t.Skipf("vec_index holds %d rows; this build has no working sqlite-vec index", indexed)
	}
	_ = ls.Close()

	var provider atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provider.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer srv.Close()
	cfg := `{"embedding": {"provider": "ollama", "ollama_endpoint": "` + srv.URL + `", "ollama_model": "other-model", "dimensions": 8}}`
	if err := os.WriteFile(filepath.Join(ws, ".nerd", "config.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runEmbeddingStats(&out, ws); err != nil {
		t.Fatal(err)
	}
	if n := provider.Load(); n != 0 {
		t.Fatalf("stats made %d request(s) to the embedding provider", n)
	}
	reopened, err := store.NewLocalStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if got := vecIndexRows(t, reopened); got != indexed {
		t.Fatalf("vec_index went from %d to %d rows: stats rebuilt the index", indexed, got)
	}
	text := out.String()
	for _, want := range []string{"Total Vectors: 3", "fixture:stats-4: 3", "other-model"} {
		if !strings.Contains(text, want) {
			t.Fatalf("stats output lacks %q:\n%s", want, text)
		}
	}
}
