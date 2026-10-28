package retrieval

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// =============================================================================
// Cache maintenance (kernel-driven, query-level)
// =============================================================================

func writeEvent(t *testing.T, k *core.RealKernel, path, hash string, ts int64) {
	t.Helper()
	if err := k.LoadFacts([]core.Fact{{
		Predicate: "file_written",
		Args:      []any{types.MangleString(path), types.MangleString(hash), types.MangleString("/session_1"), ts},
	}}); err != nil {
		t.Fatalf("LoadFacts file_written: %v", err)
	}
}

func searchOne(t *testing.T, r *SparseRetriever, keyword string) ([]string, TierOutcome) {
	t.Helper()
	hits, outcome := r.SearchKeywords(context.Background(), &IssueKeywords{Primary: []string{keyword}})
	files := map[string]bool{}
	for _, h := range hits {
		files[filepath.Base(h.FilePath)] = true
	}
	var out []string
	for f := range files {
		out = append(out, f)
	}
	sort.Strings(out)
	return out, outcome
}

// R1, fixture 1: an empty cached result is a claim that no member holds the
// keyword. A write that adds it must surface the file before the TTL, and
// without a cold rescan of the workspace.
func TestCacheDelta_EmptyResultLearnsANewMatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "alpha.go", []byte("package alpha\n"))
	r := newBoundedRetriever(t, dir)
	k := newSeedKernel(t)

	if files, _ := searchOne(t, r, "quasar"); len(files) != 0 {
		t.Fatalf("no file holds quasar yet, got %v", files)
	}
	writeFile(t, dir, "beta.go", []byte("package beta // quasar lives here\n"))
	writeEvent(t, k, filepath.Join(dir, "beta.go"), "h1", 1755000000)
	if n := r.InvalidateFromKernel(k); n != 1 {
		t.Fatalf("applied %d change events, want 1", n)
	}

	walked := r.Metrics().FilesWalked
	files, outcome := searchOne(t, r, "quasar")
	if strings.Join(files, ",") != "beta.go" {
		t.Fatalf("the new match is missing: got %v", files)
	}
	if outcome.CacheHits != 1 || r.Metrics().FilesWalked != walked {
		t.Errorf("the result was not maintained in the cache (cache hits %d, walked %d -> %d)", outcome.CacheHits, walked, r.Metrics().FilesWalked)
	}
}

// R1, fixture 2: a hit in A, then a match added to B. Dropping only the
// results that name a changed file -- or only empty results -- keeps the
// A-only answer.
func TestCacheDelta_SecondMatchJoinsAnExistingHit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.go", []byte("package a // needle\n"))
	writeFile(t, dir, "b.go", []byte("package b\n"))
	r := newBoundedRetriever(t, dir)
	k := newSeedKernel(t)

	if files, _ := searchOne(t, r, "needle"); strings.Join(files, ",") != "a.go" {
		t.Fatalf("setup: %v", files)
	}
	writeFile(t, dir, "b.go", []byte("package b // needle too\n"))
	// The kernel records the writer's workspace-relative spelling.
	writeEvent(t, k, "b.go", "h1", 1755000000)
	r.InvalidateFromKernel(k)
	files, outcome := searchOne(t, r, "needle")
	if strings.Join(files, ",") != "a.go,b.go" {
		t.Fatalf("both files must match after the write, got %v", files)
	}
	if outcome.CacheHits != 1 {
		t.Errorf("want the maintained cache entry, got %d cache hits", outcome.CacheHits)
	}
}

// R1, fixture 3: deleting the only matching file empties the result, and an
// edit that removes the keyword takes its file out.
func TestCacheDelta_DeleteAndEditRemoveHits(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.go", []byte("package a // needle\n"))
	writeFile(t, dir, "b.go", []byte("package b // needle\n"))
	r := newBoundedRetriever(t, dir)
	k := newSeedKernel(t)
	searchOne(t, r, "needle")

	if err := os.Remove(filepath.Join(dir, "a.go")); err != nil {
		t.Fatal(err)
	}
	if err := k.LoadFacts([]core.Fact{{Predicate: "file_deleted", Args: []any{types.MangleString(filepath.Join(dir, "a.go"))}}}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "b.go", []byte("package b\n"))
	writeEvent(t, k, filepath.Join(dir, "b.go"), "h2", 1755000001)
	if n := r.InvalidateFromKernel(k); n != 2 {
		t.Fatalf("applied %d change events, want the delete and the write", n)
	}
	if files, _ := searchOne(t, r, "needle"); len(files) != 0 {
		t.Fatalf("deleted and edited files still match: %v", files)
	}
	// A timestampless event is applied once while it stands in the kernel.
	if n := r.InvalidateFromKernel(k); n != 0 {
		t.Fatalf("replayed %d events, want 0", n)
	}
}

// R1, fixture 3 (membership): an ignore-file change is a new domain, so
// every cached result goes; so does a change to the member set's revision.
func TestCacheDelta_IgnoreChangeEmptiesTheCache(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.go", []byte("package a // needle\n"))
	r := newBoundedRetriever(t, dir)
	searchOne(t, r, "needle")
	if r.cache.Len() != 1 {
		t.Fatal("setup: nothing cached")
	}
	if dropped := r.ApplyFileChanges(filepath.Join(dir, ".gitignore")); dropped != 1 || r.cache.Len() != 0 {
		t.Fatalf("an ignore change kept %d results (dropped %d)", r.cache.Len(), dropped)
	}

	searchOne(t, r, "needle")
	domain, err := r.searchDomain()
	if err != nil {
		t.Fatal(err)
	}
	epoch := r.cache.Begin(domain)
	if r.cache.Begin("another domain") == epoch || r.cache.Len() != 0 {
		t.Fatal("a new domain kept the old domain's results")
	}
}

// R1, fixture 4: a scan that started before a change cannot publish after
// it. The change moves the epoch; the late result is refused.
func TestCacheDelta_InFlightScanCannotOverwriteAChange(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.go", []byte("package a // needle\n"))
	r := newBoundedRetriever(t, dir)
	searchOne(t, r, "needle")

	domain, err := r.searchDomain()
	if err != nil {
		t.Fatal(err)
	}
	stamped := r.cache.Begin(domain)
	stale := keywordResult{hits: []KeywordHit{{FilePath: filepath.Join(dir, "a.go"), Keyword: "needle"}}}
	writeFile(t, dir, "a.go", []byte("package a\n"))
	r.ApplyFileChanges(filepath.Join(dir, "a.go"))
	if r.cache.Set("needle", stale, stamped) {
		t.Fatal("a result read before the change was published after it")
	}
	if files, _ := searchOne(t, r, "needle"); len(files) != 0 {
		t.Fatalf("the pre-change result came back: %v", files)
	}
}

// An overflowed result cannot be maintained exactly: a change drops it.
func TestCacheDelta_OverflowedResultIsDroppedOnChange(t *testing.T) {
	r := newBoundedRetriever(t, t.TempDir())
	r.cache.Set("kw", keywordResult{hits: []KeywordHit{{FilePath: "x.go"}}, overflow: true}, r.cache.Begin(""))
	r.cache.Set("other", keywordResult{}, r.cache.Begin(""))
	r.ApplyFileChanges("y.go")
	if _, ok := r.cache.Get("kw"); ok {
		t.Error("an overflowed result survived a change")
	}
	if _, ok := r.cache.Get("other"); !ok {
		t.Error("a whole result was dropped instead of maintained")
	}
}

// The write cursor: a distinct same-second write is a new event, an exact
// replay is not, and a same-hash write to another path is distinct.
func TestInvalidateFromKernel_CursorSeesEachWriteOnce(t *testing.T) {
	dir := t.TempDir()
	r := newBoundedRetriever(t, dir)
	k := newSeedKernel(t)
	const ts int64 = 1755000000
	target := filepath.Join(dir, "alpha.go")

	writeEvent(t, k, target, "first-hash", ts)
	if n := r.InvalidateFromKernel(k); n != 1 {
		t.Fatalf("first write: %d events, want 1", n)
	}
	if n := r.InvalidateFromKernel(k); n != 0 {
		t.Fatalf("replay: %d events, want 0", n)
	}
	writeEvent(t, k, target, "second-hash", ts)
	if n := r.InvalidateFromKernel(k); n != 1 {
		t.Fatalf("same-second write: %d events, want 1", n)
	}
	writeEvent(t, k, filepath.Join(dir, "beta.go"), "second-hash", ts)
	if n := r.InvalidateFromKernel(k); n != 1 {
		t.Fatalf("same hash, other path: %d events, want 1", n)
	}
	if n := r.InvalidateFromKernel(k); n != 0 {
		t.Fatalf("full replay: %d events, want 0", n)
	}
	if n := r.InvalidateFromKernel(nil); n != 0 {
		t.Fatalf("nil source: %d events", n)
	}
}

// The cache's relative and absolute spellings of one file match each other.
func TestApplyFileChanges_MatchesRelativeAndAbsoluteForms(t *testing.T) {
	dir := t.TempDir()
	r := newBoundedRetriever(t, dir)
	abs := filepath.Join(dir, "internal", "alpha.go")

	r.cache.Set("kw", keywordResult{hits: []KeywordHit{{FilePath: abs, Keyword: "kw"}}}, r.cache.Begin(""))
	if changed := r.ApplyFileChanges("internal/alpha.go"); changed != 1 {
		t.Fatalf("relative form changed %d results, want 1", changed)
	}
	r.cache.Set("kw", keywordResult{hits: []KeywordHit{{FilePath: "internal/alpha.go", Keyword: "kw"}}}, r.cache.Begin(""))
	if changed := r.ApplyFileChanges(abs); changed != 1 {
		t.Fatalf("absolute form changed %d results, want 1", changed)
	}
	if got, _ := r.cache.Get("kw"); len(got.hits) != 0 {
		t.Fatalf("hits of a file that no longer exists survived: %v", got.hits)
	}
	// A path outside the workspace is outside the domain.
	r.cache.Set("kw", keywordResult{hits: []KeywordHit{{FilePath: abs, Keyword: "kw"}}}, r.cache.Begin(""))
	if changed := r.ApplyFileChanges(filepath.Join(filepath.Dir(dir), "elsewhere.go")); changed != 0 {
		t.Fatalf("an outside path changed %d results", changed)
	}
}

// =============================================================================
// Keyword provenance (R3)
// =============================================================================

// failingBackend fails one keyword and answers the others.
type failingBackend struct{ fail string }

func (failingBackend) Name() string { return "fake" }
func (b failingBackend) Search(_ context.Context, root, keyword string, _ []string) (BackendResult, error) {
	if keyword == b.fail {
		return BackendResult{}, errors.New("backend exploded")
	}
	return BackendResult{Hits: []KeywordHit{{FilePath: filepath.Join(root, keyword+".go"), Keyword: keyword, Line: 1, Column: 1, Count: 1}}}, nil
}

// Fixture 5: one keyword's search fails while another succeeds. The outcome
// is partial, the valid hits are kept, and the failed keyword is named and
// not cached.
func TestSearchKeywords_OneFailedKeywordIsPartial(t *testing.T) {
	cfg := DefaultSparseRetrieverConfig(t.TempDir())
	cfg.Backend = failingBackend{fail: "broken"}
	r := NewSparseRetriever(cfg)
	hits, outcome := r.SearchKeywords(context.Background(), &IssueKeywords{Primary: []string{"good", "broken"}})
	if outcome.Status != StatusPartial || outcome.Reason != ReasonBackendError {
		t.Fatalf("outcome = %s, want partial (backend_error)", outcome)
	}
	if len(hits) != 1 || hits[0].Keyword != "good" {
		t.Fatalf("the valid hits were not kept: %+v", hits)
	}
	if strings.Join(outcome.Incomplete, ",") != "broken" {
		t.Fatalf("incomplete = %v, want [broken]", outcome.Incomplete)
	}
	if _, ok := r.cache.Get("broken"); ok {
		t.Fatal("a failed keyword's partial result was cached as whole")
	}
	if _, ok := r.cache.Get("good"); !ok {
		t.Fatal("the whole result was not cached")
	}

	_, all := r.SearchKeywords(context.Background(), &IssueKeywords{Primary: []string{"broken"}})
	if all.Status != StatusUnavailable {
		t.Fatalf("every keyword failed with no hits: %s, want unavailable", all)
	}
}

// A canceled walk searched nothing: unavailable, with the cancellation as
// its reason, and nothing cached.
func TestSearchKeywords_CanceledWalkIsUnavailable(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.go", []byte("package a // needle\n"))
	r := newBoundedRetriever(t, dir)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, outcome := r.SearchKeywords(ctx, &IssueKeywords{Primary: []string{"needle"}})
	if outcome.Status != StatusUnavailable || outcome.Reason != ReasonCanceled {
		t.Fatalf("outcome = %s, want unavailable (canceled)", outcome)
	}
	if r.cache.Len() != 0 {
		t.Fatal("an unfinished walk was cached")
	}
}

// A keyword that reaches the per-keyword cap is an overflow: partial, and
// reported again when the cached result is reused.
func TestSearchKeywords_OverflowIsPartial(t *testing.T) {
	r := newBoundedRetriever(t, t.TempDir())
	domain, err := r.searchDomain()
	if err != nil {
		t.Fatal(err)
	}
	r.cache.Set("kw", keywordResult{hits: []KeywordHit{{FilePath: "a.go"}}, overflow: true}, r.cache.Begin(domain))
	_, outcome := r.SearchKeywords(context.Background(), &IssueKeywords{Primary: []string{"kw"}})
	if outcome.Status != StatusPartial || outcome.Reason != ReasonOverflow || outcome.CacheHits != 1 {
		t.Fatalf("outcome = %s, want partial (overflow) from the cache", outcome)
	}
}
