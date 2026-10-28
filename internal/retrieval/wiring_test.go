package retrieval

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Tier 3: Go imports
// =============================================================================

func goImportWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", []byte("module example.com/proj\n\ngo 1.26\n"))
	writeFile(t, dir, "internal/alpha/alpha.go", []byte(
		"package alpha\n\nimport (\n\t\"fmt\"\n\t\"example.com/proj/internal/beta\"\n)\n\nfunc Run() { fmt.Println(beta.Name) }\n"))
	writeFile(t, dir, "internal/beta/beta.go", []byte("package beta\n\nconst Name = \"beta\"\n"))
	writeFile(t, dir, "internal/beta/beta_test.go", []byte("package beta\n"))
	return dir
}

// TestGoImportNeighbors_ShouldFollowIntraModuleImports covers the tier that was
// dead on every Go repository: the expander understood Python syntax only, so
// Tier 3 never produced a single file here.
func TestGoImportNeighbors_ShouldFollowIntraModuleImports(t *testing.T) {
	dir := goImportWorkspace(t)
	b := NewTieredContextBuilder(DefaultTieredContextConfig(dir))

	got := b.importNeighbors(filepath.Join(dir, "internal", "alpha", "alpha.go"))
	if len(got) == 0 {
		t.Fatal("no import neighbors resolved for a Go file")
	}

	var bases []string
	for _, g := range got {
		bases = append(bases, filepath.Base(g))
	}
	sort.Strings(bases)
	if !reflect.DeepEqual(bases, []string{"beta.go"}) {
		t.Errorf("neighbors = %v, want exactly [beta.go] (stdlib excluded, tests excluded)", bases)
	}
}

func TestResolveGoImportDir_ShouldRejectExternalModules(t *testing.T) {
	dir := goImportWorkspace(t)
	b := NewTieredContextBuilder(DefaultTieredContextConfig(dir))

	if got := b.resolveGoImportDir("github.com/spf13/cobra", "example.com/proj"); got != "" {
		t.Errorf("external module resolved to %q, want empty", got)
	}
	if got := b.resolveGoImportDir("fmt", "example.com/proj"); got != "" {
		t.Errorf("stdlib package resolved to %q, want empty", got)
	}
	if got := b.resolveGoImportDir("example.com/proj/internal/beta", "example.com/proj"); got == "" {
		t.Error("intra-module import did not resolve")
	}
}

func TestGoModulePath_ShouldReadAndCacheModuleDirective(t *testing.T) {
	dir := goImportWorkspace(t)
	b := NewTieredContextBuilder(DefaultTieredContextConfig(dir))

	if got := b.goModulePath(); got != "example.com/proj" {
		t.Fatalf("module path = %q, want example.com/proj", got)
	}
	// Second call must come from the cache, not the (now removed) file.
	if err := removeFile(filepath.Join(dir, "go.mod")); err != nil {
		t.Fatalf("removing go.mod: %v", err)
	}
	if got := b.goModulePath(); got != "example.com/proj" {
		t.Errorf("cached module path = %q, want example.com/proj", got)
	}
}

// =============================================================================
// Tier 4: semantic injection + fallback
// =============================================================================

type stubSemantic struct {
	matches []SemanticMatch
	err     error
	calls   int
}

func (s *stubSemantic) SimilarFiles(_ context.Context, _ string, _ int) (SemanticResult, error) {
	s.calls++
	return SemanticResult{Matches: s.matches, Eligible: len(s.matches), Covered: len(s.matches)}, s.err
}

func TestSemanticExpansion_WhenSearcherConfigured_ShouldUseVectorMatches(t *testing.T) {
	dir := t.TempDir()
	target := writeFile(t, dir, "widget.py", []byte("class Widget:\n    pass\n"))

	cfg := DefaultTieredContextConfig(dir)
	cfg.Semantic = &stubSemantic{matches: []SemanticMatch{{FilePath: target, Score: 0.87}}}
	b := NewTieredContextBuilder(cfg)

	files, outcome := b.semanticExpansion(context.Background(), "something about widgets", &IssueKeywords{}, map[string]bool{})
	if outcome.Used != BackendEmbeddings || outcome.Status != StatusComplete {
		t.Errorf("outcome = %s, want complete via embeddings", outcome)
	}
	if len(files) != 1 {
		t.Fatalf("got %d tier 4 files, want 1", len(files))
	}
	if files[0].Tier != 4 || files[0].RelevanceScore != 0.87 {
		t.Errorf("tier 4 entry = %+v, want tier 4 score 0.87", files[0])
	}
}

// TestSemanticExpansion_WhenSearcherFails_ShouldFallBackToDefinitionScan: an
// embedding backend that is down must not empty the tier.
func TestSemanticExpansion_WhenSearcherFails_ShouldFallBackToDefinitionScan(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "widget.py", []byte("class Widget:\n    pass\n"))

	stub := &stubSemantic{err: errors.New("embedding endpoint unreachable")}
	cfg := DefaultTieredContextConfig(dir)
	cfg.Semantic = stub
	b := NewTieredContextBuilder(cfg)

	keywords := &IssueKeywords{MentionedSymbols: []string{"Widget"}}
	files, outcome := b.semanticExpansion(context.Background(), "Widget is broken", keywords, map[string]bool{})
	// Fixture 6: the attempt and the fallback are reported separately.
	if outcome.Attempted != BackendEmbeddings || outcome.Used != BackendDefinitionScan || outcome.Status != StatusPartial || outcome.Reason != ReasonBackendError {
		t.Errorf("outcome = %s, want embeddings attempted, definition scan used, partial (backend_error)", outcome)
	}

	if stub.calls != 1 {
		t.Errorf("semantic searcher called %d times, want 1", stub.calls)
	}
	if len(files) == 0 {
		t.Fatal("fallback produced no files; a failing backend emptied tier 4")
	}
	if !strings.HasSuffix(files[0].FilePath, "widget.py") {
		t.Errorf("fallback returned %q, want the definition file", files[0].FilePath)
	}
}

func TestNewEmbeddingSemanticSearcher_WhenNoEngine_ShouldReturnNil(t *testing.T) {
	if s := NewEmbeddingSemanticSearcher(nil, "."); s != nil {
		t.Errorf("got %v, want a nil searcher so the builder falls back", s)
	}
}

// fakeEmbedder returns a deterministic vector whose first component is the count
// of the letter 'z' in the text, so similarity is controllable without a live
// endpoint.
type fakeEmbedder struct{ batches int }

func (f *fakeEmbedder) vector(text string) []float32 {
	return []float32{float32(strings.Count(text, "zebra")), 1}
}

func (f *fakeEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	return f.vector(text), nil
}

func (f *fakeEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	f.batches++
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = f.vector(t)
	}
	return out, nil
}

func TestEmbeddingSemanticSearcher_ShouldRankAndCacheVectors(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "match.go", []byte("package main\n// zebra zebra zebra\n"))
	writeFile(t, dir, "other.go", []byte("package main\n// unrelated\n"))

	engine := &fakeEmbedder{}
	s := NewEmbeddingSemanticSearcher(engine, dir)

	result, err := s.SimilarFiles(context.Background(), "zebra zebra zebra problem", 5)
	if err != nil {
		t.Fatalf("SimilarFiles: %v", err)
	}
	matches := result.Matches
	if result.Eligible != 2 || result.Covered != 2 {
		t.Errorf("coverage = %d of %d, want 2 of 2", result.Covered, result.Eligible)
	}
	if len(matches) == 0 {
		t.Fatal("no matches")
	}
	if !strings.HasSuffix(matches[0].FilePath, "match.go") {
		t.Errorf("top match = %q, want match.go", matches[0].FilePath)
	}

	// A second query must reuse the cached file vectors.
	if _, err := s.SimilarFiles(context.Background(), "zebra again", 5); err != nil {
		t.Fatalf("second SimilarFiles: %v", err)
	}
	if engine.batches != 1 {
		t.Errorf("embedded the corpus %d times, want 1 (vectors are cached)", engine.batches)
	}
}

// partialEmbedder fails its second batch, simulating a seed budget expiring
// mid-corpus: the first chunk's vectors are already paid for and must be both
// ranked now and kept for the next call.
type partialEmbedder struct {
	batches    int
	failSecond bool
}

func (f *partialEmbedder) vector(text string) []float32 {
	return []float32{float32(strings.Count(text, "zebra")), 1}
}

func (f *partialEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	return f.vector(text), nil
}

func (f *partialEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	f.batches++
	if f.failSecond && f.batches == 2 {
		return nil, context.DeadlineExceeded
	}
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = f.vector(t)
	}
	return out, nil
}

func TestEmbeddingSemanticSearcher_WhenBudgetExpiresMidCorpus_ShouldRankPartialAndKeepProgress(t *testing.T) {
	dir := t.TempDir()
	for i := range semanticEmbedChunkSize + 8 {
		body := "package p\n// filler\n"
		if i == 0 {
			body = "package p\n// zebra\n"
		}
		if i == semanticEmbedChunkSize+7 {
			body = "package p\n// zebra zebra zebra\n"
		}
		writeFile(t, dir, fmt.Sprintf("f%02d.go", i), []byte(body))
	}

	engine := &partialEmbedder{failSecond: true}
	s := NewEmbeddingSemanticSearcher(engine, dir)

	result, err := s.SimilarFiles(context.Background(), "zebra zebra zebra", 5)
	if err != nil {
		t.Fatalf("SimilarFiles with partial corpus: %v", err)
	}
	matches := result.Matches
	if result.Incomplete == "" || result.Covered != semanticEmbedChunkSize || result.Eligible != semanticEmbedChunkSize+8 {
		t.Errorf("a partly embedded corpus must say so: %+v", result)
	}
	if len(matches) == 0 || !strings.HasSuffix(matches[0].FilePath, "f00.go") {
		t.Fatalf("partial matches = %v, want f00.go ranked from the completed chunk", matches)
	}

	engine.failSecond = false
	result, err = s.SimilarFiles(context.Background(), "zebra zebra zebra", 5)
	if err != nil {
		t.Fatalf("second SimilarFiles: %v", err)
	}
	matches = result.Matches
	if len(matches) == 0 || !strings.HasSuffix(matches[0].FilePath, fmt.Sprintf("f%02d.go", semanticEmbedChunkSize+7)) {
		t.Fatalf("completed matches = %v, want the final file ranked first", matches)
	}
	if engine.batches != 3 {
		t.Errorf("batches = %d, want 3 (chunk 1, failed chunk 2, retried remainder only)", engine.batches)
	}
}

// =============================================================================
// Keyword extraction: file extensions
// =============================================================================

func TestExtractKeywords_ShouldRecognizeModernFileExtensions(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"crash in src/Button.tsx during render":     "src/Button.tsx",
		"see components/Nav.vue for the markup":     "components/Nav.vue",
		"MainActivity.kt throws on launch":          "MainActivity.kt",
		"broken in web/app.svelte, check the store": "web/app.svelte",
		"schemas_knowledge.mg lost a Decl":          "schemas_knowledge.mg",
		"my-pkg/deep.util.test.ts fails":            "my-pkg/deep.util.test.ts",
		"regression traced to main.go.":             "main.go",
	}
	for issue, want := range cases {
		kw := ExtractKeywords(issue)
		if !containsString(kw.MentionedFiles, want) {
			t.Errorf("ExtractKeywords(%q).MentionedFiles = %v, want to contain %q", issue, kw.MentionedFiles, want)
		}
	}
}

// TestExtractKeywords_ShouldNotMatchPrefixExtensionsGreedily guards the
// alternation ordering: with "ts" listed before "tsx", nothing matched at all
// because the stray "x" broke the trailing delimiter.
func TestExtractKeywords_ShouldNotMatchPrefixExtensionsGreedily(t *testing.T) {
	t.Parallel()
	kw := ExtractKeywords("edit Button.tsx and Nav.jsx and lib.cc and impl.mm")
	for _, want := range []string{"Button.tsx", "Nav.jsx", "lib.cc", "impl.mm"} {
		if !containsString(kw.MentionedFiles, want) {
			t.Errorf("missing %q in %v", want, kw.MentionedFiles)
		}
	}
}

// =============================================================================
// Cancellation
// =============================================================================

// TestSearchSingleKeyword_WhenCanceled_ShouldReturnPromptly is the regression
// test for a deadlock: the workers exited on cancellation while the directory
// walk kept sending into a full channel, so close(files) was never reached and
// the search hung its caller forever with its goroutines leaked.
func TestSearchSingleKeyword_WhenCanceled_ShouldReturnPromptly(t *testing.T) {
	dir := t.TempDir()
	// Comfortably more files than the 1000-slot hand-off buffer.
	for i := 0; i < 2500; i++ {
		writeFile(t, dir, filepath.Join("pkg", "f"+itoa(i)+".go"), []byte("package p // needle\n"))
	}

	r := newBoundedRetriever(t, dir)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = r.searchSingleKeyword(ctx, "needle")
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("searchSingleKeyword did not return after cancellation; the walk/worker deadlock is back")
	}
}

func TestFindFile_WhenCanceled_ShouldNotCacheTheMiss(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "deep/nested/alpha.go", []byte("package main\n"))
	b := NewTieredContextBuilder(DefaultTieredContextConfig(dir))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = b.findFile(ctx, "nested/alpha.go")

	b.mu.RLock()
	_, cached := b.findCache["nested/alpha.go"]
	b.mu.RUnlock()
	if cached {
		t.Error("a cancellation-induced miss was cached; the file would look absent for the whole session")
	}

	// With a live context the same lookup resolves.
	if got := b.findFile(context.Background(), "nested/alpha.go"); got == "" {
		t.Error("file not found with a live context")
	}
}

// =============================================================================
// Metrics
// =============================================================================

func TestMetrics_ShouldCountWalkScanAndCacheActivity(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.go", []byte("package main // needle\n"))
	writeFile(t, dir, "blob.bin", []byte("needle\x00binary"))

	r := newBoundedRetriever(t, dir)
	kw := &IssueKeywords{Primary: []string{"needle"}, Weights: map[string]float64{"needle": 1}}

	if _, outcome := r.SearchKeywords(context.Background(), kw); outcome.Status != StatusComplete {
		t.Fatalf("SearchKeywords: %s", outcome)
	}
	first := r.Metrics()
	if first.Searches != 1 {
		t.Errorf("Searches = %d, want 1", first.Searches)
	}
	if first.CacheMisses != 1 || first.CacheHits != 0 {
		t.Errorf("cache misses/hits = %d/%d, want 1/0", first.CacheMisses, first.CacheHits)
	}
	if first.FilesWalked < 2 {
		t.Errorf("FilesWalked = %d, want >= 2", first.FilesWalked)
	}
	if first.FilesScanned < 1 {
		t.Errorf("FilesScanned = %d, want >= 1", first.FilesScanned)
	}
	if first.FilesSkipped < 1 {
		t.Errorf("FilesSkipped = %d, want >= 1 (the binary blob)", first.FilesSkipped)
	}
	if first.Hits < 1 {
		t.Errorf("Hits = %d, want >= 1", first.Hits)
	}

	// Second identical search must be a cache hit and must not walk again.
	if _, outcome := r.SearchKeywords(context.Background(), kw); outcome.Status != StatusComplete || outcome.CacheHits != 1 {
		t.Fatalf("SearchKeywords (cached): %s, %d cache hits", outcome, outcome.CacheHits)
	}
	second := r.Metrics()
	if second.CacheHits != 1 {
		t.Errorf("CacheHits = %d, want 1", second.CacheHits)
	}
	if second.FilesWalked != first.FilesWalked {
		t.Errorf("cached search still walked the tree (%d -> %d)", first.FilesWalked, second.FilesWalked)
	}
	if rate := second.CacheHitRate(); rate != 0.5 {
		t.Errorf("CacheHitRate = %v, want 0.5", rate)
	}
	if second.String() == "" {
		t.Error("metrics rendered empty")
	}
}

func TestMetrics_WhenNothingRan_ShouldBeZeroSafe(t *testing.T) {
	t.Parallel()
	var m RetrieverMetrics
	if m.CacheHitRate() != 0 || m.MeanSearchTime() != 0 {
		t.Error("zero-value metrics must not divide by zero")
	}
}

// =============================================================================
// Ripgrep backend
// =============================================================================

// TestRipgrepBackend_ShouldProduceTheSameHitsAsTheNativeScan is what makes
// parseRipgrepOutput live code again: it now parses the output of a process
// this package actually runs.
func TestRipgrepBackend_ShouldProduceTheSameHitsAsTheNativeScan(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("ripgrep not installed")
	}

	dir := t.TempDir()
	writeFile(t, dir, "alpha.go", []byte("package main\n\n// needle here\nfunc needle() {}\n"))
	writeFile(t, dir, "beta.go", []byte("package main\n\n// unrelated\n"))

	native := newBoundedRetriever(t, dir)
	nativeHits, err := native.searchSingleKeyword(context.Background(), "needle")
	if err != nil {
		t.Fatalf("native search: %v", err)
	}

	backend, err := NewRipgrepBackend()
	if err != nil {
		t.Fatalf("NewRipgrepBackend: %v", err)
	}
	if backend.Name() != "ripgrep" {
		t.Errorf("Name() = %q", backend.Name())
	}

	cfg := DefaultSparseRetrieverConfig(dir)
	cfg.Backend = backend
	rg := NewSparseRetriever(cfg)
	rgHits, err := rg.searchSingleKeyword(context.Background(), "needle")
	if err != nil {
		t.Fatalf("ripgrep search: %v", err)
	}

	if len(rgHits) != len(nativeHits) {
		t.Errorf("ripgrep found %d hits, native found %d", len(rgHits), len(nativeHits))
	}
	for _, h := range rgHits {
		if !strings.HasSuffix(h.FilePath, "alpha.go") {
			t.Errorf("unexpected hit file %q", h.FilePath)
		}
		if h.Keyword != "needle" || h.Line == 0 {
			t.Errorf("malformed hit %+v", h)
		}
	}
}

// TestRipgrepBackend_WhenNoMatch_ShouldReturnEmptyNotError: rg exits 1 on a
// miss, which is not a failure.
func TestRipgrepBackend_WhenNoMatch_ShouldReturnEmptyNotError(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("ripgrep not installed")
	}
	dir := t.TempDir()
	writeFile(t, dir, "alpha.go", []byte("package main\n"))

	backend, err := NewRipgrepBackend()
	if err != nil {
		t.Fatalf("NewRipgrepBackend: %v", err)
	}
	hits, err := backend.Search(context.Background(), dir, "definitely_absent_symbol", nil)
	if err != nil {
		t.Fatalf("a miss was reported as an error: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("got %d hits for an absent symbol", len(hits))
	}
}

func TestRipgrepBackend_WhenKeywordBlank_ShouldNotRunTheProcess(t *testing.T) {
	backend := &RipgrepBackend{Binary: "/nonexistent/rg"}
	hits, err := backend.Search(context.Background(), ".", "   ", nil)
	if err != nil || hits != nil {
		t.Errorf("blank keyword: hits=%v err=%v, want nil/nil", hits, err)
	}
}

// helpers ---------------------------------------------------------------------

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

func removeFile(path string) error { return os.Remove(path) }

// TestBuildContext_ShouldPopulateTier3OnGoWorkspace is the end-to-end form: the
// builder must reach the imported package, not just the mentioned file.
func TestBuildContext_ShouldPopulateTier3OnGoWorkspace(t *testing.T) {
	dir := goImportWorkspace(t)
	b := NewTieredContextBuilder(DefaultTieredContextConfig(dir))

	tc, err := b.BuildContext(context.Background(), "Run() misbehaves in internal/alpha/alpha.go")
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	var tier3 []string
	for _, f := range tc.Files {
		if f.Tier == 3 {
			tier3 = append(tier3, f.FilePath)
		}
	}
	if tc.Tier3Count == 0 {
		t.Fatalf("tier 3 empty; files=%v", tc.Files)
	}
	for _, p := range tier3 {
		if strings.HasSuffix(p, "beta.go") {
			return
		}
	}
	t.Errorf("tier 3 did not include the imported package: %v", tier3)
}

// Warm embeds the corpus ahead of the turn: the query that follows embeds only
// itself, so a turn ranks instead of spending its budget embedding files.
func TestEmbeddingSemanticSearcher_WarmMovesEmbeddingOffTheTurn(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "match.go", []byte("package main\n// zebra zebra zebra\n"))
	writeFile(t, dir, "other.go", []byte("package main\n// unrelated\n"))
	engine := &fakeEmbedder{}
	s := NewEmbeddingSemanticSearcher(engine, dir)
	if err := s.Warm(context.Background()); err != nil {
		t.Fatal(err)
	}
	warmed := engine.batches
	if warmed == 0 {
		t.Fatal("Warm embedded nothing")
	}
	result, err := s.SimilarFiles(context.Background(), "zebra problem", 5)
	if err != nil {
		t.Fatal(err)
	}
	if engine.batches != warmed || result.Covered != 2 {
		t.Fatalf("the query embedded corpus files again (%d batches after %d warm) or covered %d of 2", engine.batches, warmed, result.Covered)
	}
}
