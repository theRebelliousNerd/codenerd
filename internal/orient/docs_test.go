package orient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
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
	mu         sync.Mutex
	calls      int
	batchSizes []int
	attempts   map[string]int
	failFirst  int
}

func (f *fakeEmbed) Embed(ctx context.Context, text string) ([]float32, error) {
	f.mu.Lock()
	f.calls++
	if f.attempts == nil {
		f.attempts = map[string]int{}
	}
	f.attempts[text]++
	transient := f.attempts[text] <= f.failFirst
	f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.Contains(text, "FAIL") {
		return nil, fmt.Errorf("embed failed")
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("embedding: the text is empty or whitespace")
	}
	if transient {
		return nil, fmt.Errorf("transient embedding failure")
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
	f.mu.Lock()
	f.batchSizes = append(f.batchSizes, len(texts))
	f.mu.Unlock()
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
	if len(factsNamed(failed.Facts, "doc_similar")) != 1 || len(factsNamed(failed.Facts, "doc_cluster")) != 2 {
		t.Fatalf("successful neighbour graph lost: %+v", failed.Facts)
	}
	if failed.EmbeddingFailed != 1 {
		t.Fatalf("embedding failures %d, want one", failed.EmbeddingFailed)
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
		"no embedding engine was passed",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q\n%s", want, text)
		}
	}
	if len(rep.Origins) != 0 {
		t.Fatalf("peripheral initial import was classified as origin: %+v", rep.Origins)
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

func TestCollectDocs_WhitespaceChunksAndPartialBatch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	git(t, root, time.Time{}, "init")
	first := strings.Repeat("alpha ", 42) + "alph"
	writeFile(t, root, "a.md", first+strings.Repeat(" ", 256)+"alpha")
	writeFile(t, root, "b.md", "alpha")
	writeFile(t, root, "bad.md", "FAIL")
	writeFile(t, root, "blank.md", " \n\t")
	git(t, root, time.Time{}, "add", ".")
	cfg := config.DefaultOrientConfig()
	cfg.EmbeddingChunkBytes, cfg.EmbeddingBatchSize = 256, 4
	emb := &fakeEmbed{}
	stderr := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = writer
	t.Cleanup(func() { os.Stderr = stderr; _ = reader.Close(); _ = writer.Close() })
	scan, scanErr := CollectDocs(context.Background(), root, cfg, emb)
	os.Stderr = stderr
	_ = writer.Close()
	notice, readErr := io.ReadAll(reader)
	if scanErr != nil {
		t.Fatal(scanErr)
	}
	if readErr != nil {
		t.Fatal(readErr)
	}
	if scan.Embedded != 2 || scan.Empty != 1 || scan.EmbeddingFailed != 1 || len(factsNamed(scan.Facts, "doc_similar")) != 1 {
		t.Fatalf("partial scan lost useful documents: %+v", scan)
	}
	for _, want := range []string{"excluded 2", "bad.md /error", "blank.md /empty", "embed failed"} {
		if !strings.Contains(string(notice), want) {
			t.Fatalf("stderr missing %q: %s", want, notice)
		}
	}
	emb.mu.Lock()
	for text := range emb.attempts {
		if strings.TrimSpace(text) == "" {
			t.Error("whitespace chunk reached embedding")
		}
	}
	for _, size := range emb.batchSizes {
		if size > cfg.EmbeddingBatchSize {
			t.Errorf("oversized batch %d", size)
		}
	}
	emb.mu.Unlock()
	facts := append(scan.Facts, DocTieFacts([]string{"a.md", "b.md", "bad.md", "blank.md"})...)
	e := evalFacts(t, &cfg, facts)
	rep, err := buildReport(e)
	if err != nil {
		t.Fatal(err)
	}
	rep.SimilarityNote = scan.SimilarityNote
	text := rep.Text()
	if strings.Index(text, "similarity exclusions:") > strings.Index(text, "span:") {
		t.Fatal("embedding exclusions were buried at the report tail")
	}
	raw, err := rep.JSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"bad.md", "blank.md", "embed failed", "embedding_omitted"} {
		if !strings.Contains(text+string(raw), want) {
			t.Fatalf("report missing %q", want)
		}
	}
}

func TestEmbeddingRequestsRetryAndExcludeOnlyExhaustedDocuments(t *testing.T) {
	for _, tc := range []struct {
		name, body                string
		failFirst, calls, omitted int
	}{
		{"transient", "alpha", 2, 3, 0},
		{"exhausted", "FAIL", 0, 3, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.DefaultOrientConfig()
			cfg.EmbeddingBatchSize, cfg.EmbeddingRetryAttempts = 1, 3
			emb := &fakeEmbed{failFirst: tc.failFirst}
			scan := embedDocs(context.Background(), t.TempDir(), cfg, emb, map[string]string{"doc.md": tc.body})
			if emb.Calls() != tc.calls || len(scan.omitted) != tc.omitted {
				t.Fatalf("calls=%d omitted=%+v", emb.Calls(), scan.omitted)
			}
			if tc.omitted == 0 && len(scan.vectors) != 1 {
				t.Fatal("successful retry lost its vector")
			}
		})
	}
}

type boundedBatchEmbed struct {
	mu              sync.Mutex
	started         chan int
	release         chan struct{}
	active, maximum int
	sizes           []int
}

func (e *boundedBatchEmbed) Embed(context.Context, string) ([]float32, error) {
	return nil, fmt.Errorf("single-text path must not be used")
}
func (e *boundedBatchEmbed) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	e.mu.Lock()
	e.active++
	if e.active > e.maximum {
		e.maximum = e.active
	}
	e.sizes = append(e.sizes, len(texts))
	e.mu.Unlock()
	defer func() { e.mu.Lock(); e.active--; e.mu.Unlock() }()
	select {
	case e.started <- len(texts):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case <-e.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	out := make([][]float32, len(texts))
	for i := range out {
		out[i] = []float32{1, 0}
	}
	return out, nil
}
func (*boundedBatchEmbed) Dimensions() int { return 2 }
func (*boundedBatchEmbed) Name() string    { return "bounded-batch" }

func TestEmbeddingBatchConcurrencyAndCancellation(t *testing.T) {
	for _, cancelRun := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelRun), func(t *testing.T) {
			cfg := config.DefaultOrientConfig()
			cfg.EmbeddingBatchSize, cfg.EmbeddingConcurrency = 3, 2
			emb := &boundedBatchEmbed{started: make(chan int, 16), release: make(chan struct{})}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			bodies := map[string]string{}
			for i := 0; i < 10; i++ {
				bodies[fmt.Sprintf("d%d.md", i)] = fmt.Sprintf("alpha %d", i)
			}
			done := make(chan embeddingScan, 1)
			root := t.TempDir()
			go func() { done <- embedDocs(ctx, root, cfg, emb, bodies) }()
			for i := 0; i < cfg.EmbeddingConcurrency; i++ {
				select {
				case <-emb.started:
				case <-time.After(5 * time.Second):
					t.Fatal("batch worker did not start")
				}
			}
			if cancelRun {
				cancel()
			} else {
				close(emb.release)
			}
			select {
			case scan := <-done:
				if !cancelRun && len(scan.vectors) != len(bodies) {
					t.Fatalf("vectors=%d", len(scan.vectors))
				}
			case <-time.After(5 * time.Second):
				t.Fatal("batch workers did not drain")
			}
			emb.mu.Lock()
			defer emb.mu.Unlock()
			if emb.maximum != cfg.EmbeddingConcurrency || emb.active != 0 {
				t.Fatalf("concurrency maximum=%d active=%d", emb.maximum, emb.active)
			}
			for _, size := range emb.sizes {
				if size > cfg.EmbeddingBatchSize {
					t.Fatalf("batch size %d", size)
				}
			}
		})
	}
}

func TestInspect_SparseOneDayCohortElectsLinkedMember(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	git(t, root, time.Time{}, "init")
	jan := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	// The denominator is unrelated documents born over a different window.
	for i := 0; i < 108; i++ {
		writeFile(t, root, fmt.Sprintf("other/d%03d/p.md", i), fmt.Sprintf("other %d", i))
	}
	git(t, root, jan, "add", ".")
	git(t, root, jan, "commit", "-m", "existing documentation")
	for i := 0; i < 12; i++ {
		body := fmt.Sprintf("packet member %d", i)
		if i != 11 {
			body += "\n[entry](p11.md)"
		}
		writeFile(t, root, fmt.Sprintf("packet/p%02d.md", i), body)
	}
	git(t, root, jan.AddDate(0, 0, 10), "add", "packet")
	git(t, root, jan.AddDate(0, 0, 10), "commit", "-m", "cohesive documentation act")
	cfg := config.DefaultOrientConfig()
	rep, err := Inspect(context.Background(), root, &cfg, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.CohortGroups) != 1 || len(rep.CohortGroups[0].Members) != 12 || !rep.CohortGroups[0].Burst || rep.CohortGroups[0].Representative != "packet/p11.md" {
		t.Fatalf("cohort groups %+v", rep.CohortGroups)
	}
	count := 0
	for _, candidate := range rep.ReadCandidates {
		if strings.HasPrefix(candidate.Path, "packet/") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("packet read slots %d", count)
	}
}

func TestEmbeddingIncompleteDocumentIsNeverCached(t *testing.T) {
	cfg := config.DefaultOrientConfig()
	cfg.EmbeddingChunkBytes = 256
	body := strings.Repeat("alpha ", 42) + "alph" + "FAIL"
	root := t.TempDir()
	emb := &fakeEmbed{}
	scan := embedDocs(context.Background(), root, cfg, emb, map[string]string{"mixed.md": body})
	if len(scan.vectors) != 0 || len(scan.omitted) != 1 || scan.omitted[0].Kind != "/error" {
		t.Fatalf("partial document was admitted: %+v", scan)
	}
	sum := sha256.Sum256([]byte(body))
	if _, err := os.Stat(cacheFile(root, emb.Name(), hex.EncodeToString(sum[:]))); !os.IsNotExist(err) {
		t.Fatalf("partial centroid cached: %v", err)
	}
}

func TestValidVectorRejectsUnusableEmbeddings(t *testing.T) {
	for _, tc := range []struct {
		name  string
		vec   []float32
		dims  int
		valid bool
	}{
		{"empty", nil, 2, false},
		{"wrong dimensions", []float32{1}, 2, false},
		{"not a number", []float32{float32(math.NaN()), 1}, 2, false},
		{"infinite", []float32{1, float32(math.Inf(1))}, 2, false},
		{"zero", []float32{0, 0}, 2, false},
		{"valid", []float32{1, 0}, 2, true},
		{"unknown dimensions", []float32{1, 0}, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validVector(tc.vec, tc.dims); (err == nil) != tc.valid {
				t.Fatalf("vector validity %v: %v", tc.valid, err)
			}
		})
	}
}

func TestInspect_RareInstructionBeatsSharedBurstInFixtureRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	git(t, root, time.Time{}, "init")
	when := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 100; i++ {
		writeFile(t, root, fmt.Sprintf("d%03d.md", i), fmt.Sprintf("initial %d", i))
	}
	git(t, root, when, "add", ".")
	git(t, root, when, "commit", "-m", "initial documents")
	for revision := 1; revision <= 5; revision++ {
		for i := 0; i < 90; i++ {
			writeFile(t, root, fmt.Sprintf("d%03d.md", i), fmt.Sprintf("revision %d document %d", revision, i))
		}
		git(t, root, when.Add(time.Duration(revision)*time.Minute), "add", ".")
		git(t, root, when.Add(time.Duration(revision)*time.Minute), "commit", "-m", fmt.Sprintf("revision %d", revision))
	}
	cfg := config.DefaultOrientConfig()
	cfg.ReadCandidateBudget = 1
	extra := []types.Fact{F("agent_source", "root", N("/codex"), N("/instructions"), "root", "d099.md", N("/yes"))}
	rep, err := Inspect(context.Background(), root, &cfg, nil, extra)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.ReadCandidates) != 1 || rep.ReadCandidates[0].Path != "d099.md" {
		t.Fatalf("saturated burst won: %+v", rep.ReadCandidates)
	}
}

func TestInspect_EarlyPeripheralDocumentsAreNotOrigins(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	git(t, root, time.Time{}, "init")
	jan := time.Date(2020, 1, 15, 12, 0, 0, 0, time.UTC)
	writeFile(t, root, "old.md", "old peripheral document")
	git(t, root, jan, "add", ".")
	git(t, root, jan, "commit", "-m", "early peripheral document")
	writeFile(t, root, "later.md", "unrelated later document")
	git(t, root, jan.AddDate(0, 2, 0), "add", ".")
	git(t, root, jan.AddDate(0, 2, 0), "commit", "-m", "later unrelated document")
	history, err := ScanHistory(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultOrientConfig()
	docs, err := CollectDocs(context.Background(), root, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	facts := append(history.Facts, docs.Facts...)
	facts = append(facts, DocTieFacts(tiePaths(facts))...)
	e := evalFacts(t, &cfg, facts)
	mustRow(t, e, "doc_generation", "old.md", "/early")
	refuseRow(t, e, "origin_source", "old.md", "/birth_era")
}

func TestInspect_QuietMonthIsRelativeToFixtureHistory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	git(t, root, time.Time{}, "init")
	for month, count := range []int{20, 1, 20} {
		for commit := 0; commit < count; commit++ {
			when := time.Date(2020, time.Month(month+1), 15, 12, commit, 0, 0, time.UTC)
			git(t, root, when, "commit", "--allow-empty", "-m", fmt.Sprintf("m%d c%d", month, commit))
		}
	}
	cfg := config.DefaultOrientConfig()
	rep, err := Inspect(context.Background(), root, &cfg, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Eras) != 3 || rep.Eras[1].Kind != "/lull" || rep.Eras[1].StartLabel != "2020-02" {
		t.Fatalf("quiet five-percent month was not a lull: %+v", rep.Eras)
	}
}
