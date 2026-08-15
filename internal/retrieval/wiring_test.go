package retrieval

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// =============================================================================
// Cache invalidation (kernel-driven)
// =============================================================================

// TestInvalidateFromKernel_WhenFileWritten_ShouldDropCachedHits proves the
// invalidation signal is read out of the EDB. Before this, a five-minute cache
// TTL meant an agent that edited a file and then searched again was answered
// from hits computed before the edit.
func TestInvalidateFromKernel_WhenFileWritten_ShouldDropCachedHits(t *testing.T) {
	dir := t.TempDir()
	r := newBoundedRetriever(t, dir)

	target := filepath.Join(dir, "alpha.go")
	r.cache.Set("needle", []KeywordHit{{FilePath: target, Keyword: "needle"}})
	r.cache.Set("other", []KeywordHit{{FilePath: filepath.Join(dir, "beta.go"), Keyword: "other"}})

	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	if err := k.LoadFacts([]core.Fact{{
		Predicate: "file_written",
		Args:      []any{types.MangleString(target), types.MangleString("deadbeef"), types.MangleString("/session_1"), int64(1755000000)},
	}}); err != nil {
		t.Fatalf("LoadFacts: %v", err)
	}

	if dropped := r.InvalidateFromKernel(k); dropped != 1 {
		t.Fatalf("dropped %d cache entries, want 1", dropped)
	}
	if _, ok := r.cache.Get("needle"); ok {
		t.Error("cached hits for the written file survived invalidation")
	}
	if _, ok := r.cache.Get("other"); !ok {
		t.Error("an unrelated keyword's cache entry was dropped")
	}

	// The cursor must advance so a second pass over the same write log is a
	// no-op rather than re-dropping everything on every seed.
	r.cache.Set("needle", []KeywordHit{{FilePath: target, Keyword: "needle"}})
	if dropped := r.InvalidateFromKernel(k); dropped != 0 {
		t.Errorf("replayed %d already-seen writes, want 0", dropped)
	}
}

// TestInvalidateFiles_ShouldMatchRelativeAndAbsoluteForms: the cache holds the
// scanner's paths while the kernel holds whatever the writer used, so the two
// spellings of one file have to invalidate each other.
func TestInvalidateFiles_ShouldMatchRelativeAndAbsoluteForms(t *testing.T) {
	dir := t.TempDir()
	r := newBoundedRetriever(t, dir)
	abs := filepath.Join(dir, "internal", "alpha.go")

	r.cache.Set("kw", []KeywordHit{{FilePath: abs, Keyword: "kw"}})
	if dropped := r.InvalidateFiles("internal/alpha.go"); dropped != 1 {
		t.Fatalf("relative form dropped %d entries, want 1", dropped)
	}

	r.cache.Set("kw", []KeywordHit{{FilePath: "internal/alpha.go", Keyword: "kw"}})
	if dropped := r.InvalidateFiles(abs); dropped != 1 {
		t.Fatalf("absolute form dropped %d entries, want 1", dropped)
	}
}

func TestInvalidateAll_ShouldEmptyCacheAndResetCursor(t *testing.T) {
	r := newBoundedRetriever(t, t.TempDir())
	r.cache.Set("kw", []KeywordHit{{FilePath: "a.go"}})
	r.mu.Lock()
	r.lastWriteCursor = 99
	r.mu.Unlock()

	r.InvalidateAll()

	if _, ok := r.cache.Get("kw"); ok {
		t.Error("cache not emptied")
	}
	r.mu.RLock()
	cursor := r.lastWriteCursor
	r.mu.RUnlock()
	if cursor != 0 {
		t.Errorf("write cursor = %d, want 0", cursor)
	}
}

func TestInvalidateFromKernel_WhenSourceNil_ShouldBeNoOp(t *testing.T) {
	r := newBoundedRetriever(t, t.TempDir())
	if got := r.InvalidateFromKernel(nil); got != 0 {
		t.Errorf("nil source dropped %d entries", got)
	}
}

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

// TestBuildContext_ShouldPopulateTier3OnGoWorkspace is the end-to-end form: the
// builder must reach the imported package, not just the mentioned file.
func TestBuildContext_ShouldPopulateTier3OnGoWorkspace(t *testing.T) {
	dir := goImportWorkspace(t)
	b := NewTieredContextBuilder(DefaultTieredContextConfig(dir))

	tc, err := b.BuildContext(context.Background(), "Run() misbehaves in internal/alpha/alpha.go")
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if tc.Tier3Count == 0 {
		t.Fatalf("tier 3 empty; files=%v", tc.GetFilePaths())
	}
	for _, f := range tc.GetFilesByTier(3) {
		if strings.HasSuffix(f.FilePath, "beta.go") {
			return
		}
	}
	t.Errorf("tier 3 did not include the imported package: %v", tc.GetFilesByTier(3))
}

// =============================================================================
// Tier 4: semantic injection + fallback
// =============================================================================

type stubSemantic struct {
	matches []SemanticMatch
	err     error
	calls   int
}

func (s *stubSemantic) SimilarFiles(_ context.Context, _ string, _ int) ([]SemanticMatch, error) {
	s.calls++
	return s.matches, s.err
}

func TestSemanticExpansion_WhenSearcherConfigured_ShouldUseVectorMatches(t *testing.T) {
	dir := t.TempDir()
	target := writeFile(t, dir, "widget.py", []byte("class Widget:\n    pass\n"))

	cfg := DefaultTieredContextConfig(dir)
	cfg.Semantic = &stubSemantic{matches: []SemanticMatch{{FilePath: target, Score: 0.87}}}
	b := NewTieredContextBuilder(cfg)

	files := b.semanticExpansion(context.Background(), "something about widgets", &IssueKeywords{}, map[string]bool{})
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
	files := b.semanticExpansion(context.Background(), "Widget is broken", keywords, map[string]bool{})

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

	matches, err := s.SimilarFiles(context.Background(), "zebra zebra zebra problem", 5)
	if err != nil {
		t.Fatalf("SimilarFiles: %v", err)
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

	if _, err := r.SearchKeywords(context.Background(), kw); err != nil {
		t.Fatalf("SearchKeywords: %v", err)
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
	if _, err := r.SearchKeywords(context.Background(), kw); err != nil {
		t.Fatalf("SearchKeywords (cached): %v", err)
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
