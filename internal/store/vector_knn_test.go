package store

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"path/filepath"
	"sort"
	"testing"
)

// hashEngine embeds a text to a deterministic pseudo-random vector, so many
// texts spread over the space and rankings are not ties.
type hashEngine struct{ dim int }

func (e hashEngine) vec(text string) []float32 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(text))
	rng := rand.New(rand.NewSource(int64(h.Sum64())))
	v := make([]float32, e.dim)
	for i := range v {
		v[i] = float32(rng.NormFloat64())
	}
	return v
}
func (e hashEngine) Embed(_ context.Context, text string) ([]float32, error) { return e.vec(text), nil }
func (e hashEngine) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = e.vec(t)
	}
	return out, nil
}
func (e hashEngine) Dimensions() int { return e.dim }
func (e hashEngine) Name() string    { return "hash-engine" }

func cosineDistance(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	return 1 - dot/(math.Sqrt(na)*math.Sqrt(nb))
}

// The k-nearest-neighbour recall ranks exactly as a brute-force cosine scan
// of the eligible rows that pass the same filters: same rows, same order,
// same similarity, for metadata filters, path filters and no filter.
func TestVectorRecallVecRanksAsABruteForceCosineScan(t *testing.T) {
	s, err := NewLocalStore(filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if !s.vectorExt.Load() {
		t.Skip("sqlite-vec not available")
	}
	eng := hashEngine{dim: 16}
	s.SetEmbeddingEngine(eng)
	ctx := context.Background()
	if err := s.waitForVecBackfill(ctx); err != nil {
		t.Fatal(err)
	}
	type row struct {
		content string
		meta    map[string]any
	}
	var rows []row
	for i := 0; i < 240; i++ {
		meta := map[string]any{
			"content_type": []string{"knowledge_atom", "prompt_atom", "conversation"}[i%3],
			"path":         fmt.Sprintf("dir%d/f.go", i%5),
		}
		r := row{content: fmt.Sprintf("document %d about topic %d", i, i%17), meta: meta}
		if err := s.StoreVectorWithEmbedding(ctx, r.content, r.meta); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, r)
	}
	reference := func(query string, limit int, keep func(row) bool) []string {
		q := eng.vec(query)
		type scored struct {
			content string
			d       float64
		}
		var all []scored
		for _, r := range rows {
			if keep(r) {
				all = append(all, scored{r.content, cosineDistance(q, eng.vec(r.content))})
			}
		}
		sort.Slice(all, func(a, b int) bool { return all[a].d < all[b].d })
		var out []string
		for i := 0; i < limit && i < len(all); i++ {
			out = append(out, all[i].content)
		}
		return out
	}
	got := func(entries []VectorEntry) []string {
		var out []string
		for _, e := range entries {
			out = append(out, e.Content)
		}
		return out
	}
	for qi := 0; qi < 20; qi++ {
		query := fmt.Sprintf("query %d", qi)
		sq := semanticQuery{vec: eng.vec(query), space: eng.Name()}
		cases := []struct {
			name  string
			paths []string
			key   string
			value any
			keep  func(row) bool
		}{
			{"none", nil, "", nil, func(row) bool { return true }},
			{"content_type", nil, "content_type", "knowledge_atom", func(r row) bool { return r.meta["content_type"] == "knowledge_atom" }},
			{"paths", []string{"dir1/f.go", "dir3/f.go"}, "", nil, func(r row) bool { return r.meta["path"] == "dir1/f.go" || r.meta["path"] == "dir3/f.go" }},
		}
		for _, c := range cases {
			entries, err := s.vectorRecallVec(query, sq, 7, c.paths, c.key, c.value)
			if err != nil {
				t.Fatal(err)
			}
			want := reference(query, 7, c.keep)
			if fmt.Sprint(got(entries)) != fmt.Sprint(want) {
				t.Fatalf("query %d filter %s: KNN %v, brute force %v", qi, c.name, got(entries), want)
			}
			for _, e := range entries {
				sim, _ := e.Metadata["similarity"].(float64)
				if want := 1 - cosineDistance(sq.vec, eng.vec(e.Content)); math.Abs(sim-want) > 1e-4 {
					t.Fatalf("similarity %v, want %v", sim, want)
				}
			}
		}
	}
}
