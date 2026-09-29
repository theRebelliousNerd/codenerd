package orient

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

// fakeEmbed is a deterministic stand-in for an embedding engine. Vectors
// are a function of the text so cosine is a property of the fixture, and
// the mutex is there because CollectDocs embeds several documents at once.
type fakeEmbed struct {
	mu    sync.Mutex
	calls int
}

func (f *fakeEmbed) Embed(ctx context.Context, text string) ([]float32, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.Contains(text, "FAIL") {
		return nil, fmt.Errorf("embed failed")
	}
	switch {
	case strings.Contains(text, "alpha"):
		return []float32{1, 0}, nil
	case strings.Contains(text, "beta"):
		return []float32{0, 1}, nil
	default:
		return []float32{1, 1}, nil
	}
}

func (f *fakeEmbed) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		v, err := f.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func (f *fakeEmbed) Dimensions() int { return 2 }

func (f *fakeEmbed) Name() string { return "fake:test" }

func (f *fakeEmbed) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func TestCollectDocs_SimilarityCacheAndFailure(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	git(t, root, time.Time{}, "init")
	writeFile(t, root, "a-alpha.md", "alpha subject one\n")
	writeFile(t, root, "m-alpha.md", "alpha subject two\n")
	writeFile(t, root, "beta.md", "beta other\n")
	writeFile(t, root, "blank.md", "   \n")
	when := time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC)
	git(t, root, when, "add", ".")
	git(t, root, when, "commit", "-m", "docs")

	cfg := config.DefaultOrientConfig()
	cfg.SimilarityFloorPermille = 700
	cfg.SimilarTopK = 5
	emb := &fakeEmbed{}
	scan, err := CollectDocs(context.Background(), root, cfg, emb)
	if err != nil {
		t.Fatal(err)
	}
	if scan.Empty != 1 {
		t.Fatalf("empty documents %d", scan.Empty)
	}
	sims := factsNamed(scan.Facts, "doc_similar")
	if len(sims) != 1 {
		t.Fatalf("similar %+v", sims)
	}
	if types.ExtractString(sims[0].Args[0]) != "a-alpha.md" || types.ExtractString(sims[0].Args[1]) != "m-alpha.md" {
		t.Fatalf("pair order %+v", sims[0].Args)
	}
	pm, _ := types.ExtractInt64(sims[0].Args[2])
	if pm != 1000 {
		t.Fatalf("permille %d", pm)
	}
	clusters := map[string]string{}
	for _, f := range factsNamed(scan.Facts, "doc_cluster") {
		clusters[types.ExtractString(f.Args[0])] = types.ExtractString(f.Args[1])
	}
	if clusters["a-alpha.md"] != "a-alpha.md" || clusters["m-alpha.md"] != "a-alpha.md" {
		t.Fatalf("clusters %+v", clusters)
	}
	if _, ok := clusters["beta.md"]; ok {
		t.Fatal("isolate was clustered")
	}
	calls := emb.Calls()
	if calls == 0 {
		t.Fatal("embedder was not called")
	}

	again, err := CollectDocs(context.Background(), root, cfg, emb)
	if err != nil {
		t.Fatal(err)
	}
	if emb.Calls() != calls {
		t.Fatalf("cache miss: calls %d -> %d", calls, emb.Calls())
	}
	if again.CacheHits < 3 {
		t.Fatalf("cache hits %d", again.CacheHits)
	}

	cfg.EmbeddingChunkBytes = 256
	if _, err := CollectDocs(context.Background(), root, cfg, emb); err != nil {
		t.Fatal(err)
	}
	if emb.Calls() == calls {
		t.Fatal("chunk-size change reused the cache")
	}

	writeFile(t, root, "bad.md", "FAIL this body\n")
	git(t, root, when, "add", "bad.md")
	git(t, root, when, "commit", "-m", "bad")
	failed, err := CollectDocs(context.Background(), root, cfg, emb)
	if err != nil {
		t.Fatal(err)
	}
	if failed.SimilarityError == "" {
		t.Fatal("embed error was swallowed")
	}
	if n := len(factsNamed(failed.Facts, "doc_similar")) + len(factsNamed(failed.Facts, "doc_cluster")); n != 0 {
		t.Fatalf("partial neighbour graph asserted %d rows", n)
	}
}

func TestCollectDocs_UnreadableTrackedFile(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	git(t, root, time.Time{}, "init")
	writeFile(t, root, "gone.md", "still tracked\n")
	when := time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC)
	git(t, root, when, "add", "gone.md")
	git(t, root, when, "commit", "-m", "add")
	if err := os.Remove(root + string(os.PathSeparator) + "gone.md"); err != nil {
		t.Fatal(err)
	}
	scan, err := CollectDocs(context.Background(), root, config.DefaultOrientConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(scan.Unreadable, ",") != "gone.md" {
		t.Fatalf("unreadable %v", scan.Unreadable)
	}
	for _, f := range factsNamed(scan.Facts, "doc_file") {
		if types.ExtractString(f.Args[0]) == "gone.md" {
			t.Fatal("unreadable file was asserted")
		}
	}
	if strings.Contains(scan.SimilarityNote, "config.json") {
		t.Fatal(scan.SimilarityNote)
	}
	if !strings.Contains(scan.SimilarityNote, "no embedding engine was passed") {
		t.Fatal(scan.SimilarityNote)
	}
}

func TestInspect_TimelineAndShallow(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	git(t, root, time.Time{}, "init")
	jan := time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC)
	mar := time.Date(2020, 3, 15, 0, 0, 0, 0, time.UTC)
	writeFile(t, root, "readme.md", "# readme\n")
	writeFile(t, root, "a.md", "alpha body\n")
	for i := 0; i < 8; i++ {
		writeFile(t, root, fmt.Sprintf("c%d.md", i), "cohort\n")
	}
	git(t, root, jan, "add", ".")
	git(t, root, jan, "commit", "-m", "birth")
	git(t, root, mar, "mv", "a.md", "b.md")
	git(t, root, mar, "commit", "-m", "rename")
	for i := 0; i < 4; i++ {
		when := mar.Add(time.Duration(i) * time.Hour)
		writeFile(t, root, "day.md", fmt.Sprintf("day %d\n", i))
		git(t, root, when, "add", "day.md")
		git(t, root, when, "commit", "-m", fmt.Sprintf("day %d", i))
	}

	cfg := config.DefaultOrientConfig()
	rep, err := Inspect(context.Background(), root, &cfg, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := rep.Text()
	for _, want := range []string{
		"0 /wave 2020-01..2020-01",
		"1 /lull 2020-02..2020-02",
		"2 /wave 2020-03..2020-03",
		"b.md /birth_era",
		"no embedding engine was passed",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "day.md") {
		t.Fatalf("day.md is not a burst or an origin\n%s", text)
	}
	if strings.Contains(text, "config.json") {
		t.Fatal(text)
	}
	raw, err := rep.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"kind": "/wave"`) {
		t.Fatalf("json %s", raw)
	}

	dst := t.TempDir()
	git(t, root, time.Time{}, "-c", "safe.directory=*", "clone", "--no-local", "--depth", "1", root, dst)
	shallow, err := Inspect(context.Background(), dst, &cfg, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(shallow.Text(), "History is shallow") {
		t.Fatal(shallow.Text())
	}
	if len(shallow.Eras) != 0 || len(shallow.Origins) != 0 {
		t.Fatalf("shallow drew eras=%d origins=%d", len(shallow.Eras), len(shallow.Origins))
	}
}

func factsNamed(facts []types.Fact, pred string) []types.Fact {
	var out []types.Fact
	for _, f := range facts {
		if f.Predicate == pred {
			out = append(out, f)
		}
	}
	return out
}
