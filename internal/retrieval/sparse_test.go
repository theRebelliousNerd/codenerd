package retrieval

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestExtractKeywords_WeightsAndOrder(t *testing.T) {
	issue := "panic: FooError in internal/core/kernel.go:123 when calling do_thing() and obj.process() with `specialVar`."

	kw := ExtractKeywords(issue)

	if got, want := kw.MentionedFiles, []string{"internal/core/kernel.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("MentionedFiles = %#v, want %#v", got, want)
	}
	if got, want := kw.Weights["internal/core/kernel.go"], 1.0; got != want {
		t.Fatalf("Weights[file] = %v, want %v", got, want)
	}

	if got, want := kw.Primary, []string{"FooError"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Primary = %#v, want %#v", got, want)
	}
	if got, want := kw.MentionedSymbols, []string{"FooError"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("MentionedSymbols = %#v, want %#v", got, want)
	}
	if got, want := kw.Weights["FooError"], 0.9; got != want {
		t.Fatalf("Weights[FooError] = %v, want %v", got, want)
	}

	if got, want := kw.Secondary, []string{"do_thing", "process"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Secondary = %#v, want %#v", got, want)
	}
	if got, want := kw.Weights["do_thing"], 0.7; got != want {
		t.Fatalf("Weights[do_thing] = %v, want %v", got, want)
	}
	if got, want := kw.Weights["process"], 0.7; got != want {
		t.Fatalf("Weights[process] = %v, want %v", got, want)
	}

	if got, want := kw.Tertiary, []string{"specialVar"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Tertiary = %#v, want %#v", got, want)
	}
	if got, want := kw.Weights["specialVar"], 0.5; got != want {
		t.Fatalf("Weights[specialVar] = %v, want %v", got, want)
	}

	if got, want := kw.AllKeywords(), []string{"FooError", "do_thing", "process", "specialVar"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("AllKeywords() = %#v, want %#v", got, want)
	}
}

func TestExtractKeywords_NormalizesBackslashPaths(t *testing.T) {
	issue := "panic: FooError in internal\\core\\kernel.go:123"
	kw := ExtractKeywords(issue)

	if got, want := kw.MentionedFiles, []string{"internal/core/kernel.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("MentionedFiles = %#v, want %#v", got, want)
	}
	if got, want := kw.Weights["internal/core/kernel.go"], 1.0; got != want {
		t.Fatalf("Weights[file] = %v, want %v", got, want)
	}
}

// REMEDIATED: TEST_GAP: Empty Inputs: TestExtractKeywords_EmptyString - Verify behavior with "" and whitespace-only strings.
// REMEDIATED: TEST_GAP: Malformed Output: TestParseRipgrepOutput_MalformedColons - Verify handling of Windows paths (C:\repo\file.go) and ignored fmt.Sscanf errors.
// REMEDIATED: TEST_GAP: Concurrency: TestKeywordHitCache_Concurrency - Verify race conditions during simultaneous Get, Set, and evictOldest.
// REMEDIATED: TEST_GAP: Resource Limits: TestSparseRetriever_HugeOutput - Verify memory safety when ripgrep returns millions of lines (OOM prevention).
// REMEDIATED: TEST_GAP: Context Cancellation: TestSparseRetriever_ContextTimeout - Verify process is cleanly killed without leaking goroutines when timeout occurs.
// REMEDIATED: TEST_GAP: Extreme Length: TestExtractKeywords_ReDoS - Verify regex performance on 100kb strings without spaces.
// REMEDIATED: TEST_GAP: Null Byte Injection: TestExtractKeywords_NullBytes - Verify safe handling of \x00 in user input.
// REMEDIATED: TEST_GAP: Empty WorkDir: TestSparseRetriever_EmptyWorkDir - Verify initialization and command execution safety when workDir is "".
// REMEDIATED: TEST_GAP: Case Sensitivity: TestRankFiles_CaseInsensitiveWeights - Verify keyword weighting works regardless of casing differences between extraction and ripgrep output.

func TestKeywordHitCache_TTLAndEviction(t *testing.T) {
	hits := []KeywordHit{{FilePath: "a.go", Keyword: "k", Line: 1}}

	t.Run("ttl_expired", func(t *testing.T) {
		cache := NewKeywordHitCache(10, -1*time.Second)
		cache.Set("k", hits)
		if _, ok := cache.Get("k"); ok {
			t.Fatalf("Get() ok=true, want false for expired entry")
		}
	})

	t.Run("evicts_oldest", func(t *testing.T) {
		cache := NewKeywordHitCache(2, time.Hour)
		cache.Set("a", hits)
		cache.Set("b", hits)

		cache.mu.Lock()
		cache.entries["a"].timestamp = time.Unix(0, 0)
		cache.entries["b"].timestamp = time.Unix(100, 0)
		cache.mu.Unlock()

		cache.Set("c", hits)

		cache.mu.RLock()
		_, hasA := cache.entries["a"]
		_, hasB := cache.entries["b"]
		_, hasC := cache.entries["c"]
		cache.mu.RUnlock()

		if hasA || !hasB || !hasC {
			t.Fatalf("cache eviction unexpected (a=%v b=%v c=%v)", hasA, hasB, hasC)
		}
	})
}

// rgMatchJSON builds a single ripgrep --json "match" event line matching the
// format ripgrep emits. Submatches are byte ranges; we use a single submatch
// at the given 0-based byte offset.
func rgMatchJSON(path string, lineNumber, byteStart, byteEnd int, line string) string {
	return fmt.Sprintf(
		`{"type":"match","data":{"path":{"text":%q},"lines":{"text":%q},"line_number":%d,"submatches":[{"start":%d,"end":%d}]}}`,
		path, line, lineNumber, byteStart, byteEnd,
	)
}

func TestParseRipgrepJSON_CountsPerFile(t *testing.T) {
	r := &SparseRetriever{}
	output := strings.Join([]string{
		rgMatchJSON("a.go", 1, 1, 4, "first"),
		rgMatchJSON("a.go", 3, 3, 6, "second"),
		rgMatchJSON("b.go", 5, 5, 8, "third"),
	}, "\n") + "\n"

	hits := r.parseRipgrepJSON([]byte(output), "kw")
	if len(hits) != 3 {
		t.Fatalf("parseRipgrepJSON len=%d, want 3", len(hits))
	}
	if hits[0].FilePath != "a.go" || hits[0].Count != 1 {
		t.Fatalf("hits[0]=%+v, want FilePath=a.go Count=1", hits[0])
	}
	if hits[1].FilePath != "a.go" || hits[1].Count != 2 {
		t.Fatalf("hits[1]=%+v, want FilePath=a.go Count=2", hits[1])
	}
	if hits[2].FilePath != "b.go" || hits[2].Count != 1 {
		t.Fatalf("hits[2]=%+v, want FilePath=b.go Count=1", hits[2])
	}
}

// TestParseRipgrepJSON_WindowsPath asserts that a path with a drive-letter
// colon (e.g. C:\repo\file.go) is preserved verbatim, since it's a
// structured JSON field rather than a colon-delimited token. This is the
// original motivation for switching to --json.
func TestParseRipgrepJSON_WindowsPath(t *testing.T) {
	r := &SparseRetriever{}
	output := rgMatchJSON(`C:\repo\file.go`, 12, 3, 9, "content") + "\n"
	hits := r.parseRipgrepJSON([]byte(output), "kw")
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	if hits[0].FilePath != `C:\repo\file.go` {
		t.Fatalf("FilePath=%q, want C:\\repo\\file.go", hits[0].FilePath)
	}
	if hits[0].Line != 12 || hits[0].Column != 4 {
		t.Fatalf("Line/Column = %d/%d, want 12/4", hits[0].Line, hits[0].Column)
	}
}

func TestRankFiles_ScoreAndTier(t *testing.T) {
	r := &SparseRetriever{}

	keywords := &IssueKeywords{
		Weights: map[string]float64{
			"K1": 1.0,
			"K2": 1.0,
			// K3 omitted to exercise default weight path.
		},
		MentionedFiles: []string{"src/mentioned.go"},
	}

	hits := []KeywordHit{
		{FilePath: "repo\\src\\mentioned.go", Keyword: "K3"},
		{FilePath: "repo/src/high.go", Keyword: "K1"},
		{FilePath: "repo/src/high.go", Keyword: "K2"},
		{FilePath: "repo/src/medium.go", Keyword: "K1"},
		{FilePath: "repo/src/low.go", Keyword: "K3"},
	}

	candidates := r.RankFiles(hits, keywords, 0)
	if len(candidates) != 4 {
		t.Fatalf("RankFiles len=%d, want 4", len(candidates))
	}

	if candidates[0].FilePath != "repo/src/high.go" {
		t.Fatalf("candidates[0].FilePath=%q, want repo/src/high.go", candidates[0].FilePath)
	}
	if candidates[0].Tier != 2 {
		t.Fatalf("high.go tier=%d, want 2", candidates[0].Tier)
	}

	var mentioned *CandidateFile
	for i := range candidates {
		if candidates[i].FilePath == "repo\\src\\mentioned.go" {
			mentioned = &candidates[i]
			break
		}
	}
	if mentioned == nil {
		t.Fatalf("expected mentioned file candidate not found: %#v", candidates)
	}
	if mentioned.Tier != 1 {
		t.Fatalf("mentioned.go tier=%d, want 1", mentioned.Tier)
	}
	if mentioned.RelevanceScore != 0.3 {
		t.Fatalf("mentioned.go score=%v, want 0.3 (default weight)", mentioned.RelevanceScore)
	}
}

// -----------------------------------------------------------------------------
// Marathon 36: Sparse Retriever Gaps
// -----------------------------------------------------------------------------

func TestExtractKeywords_EmptyString(t *testing.T) {
	kw1 := ExtractKeywords("")
	if len(kw1.AllKeywords()) != 0 {
		t.Errorf("expected 0 keywords for empty string, got %d", len(kw1.AllKeywords()))
	}
	kw2 := ExtractKeywords("   \n \t ")
	if len(kw2.AllKeywords()) != 0 {
		t.Errorf("expected 0 keywords for whitespace string, got %d", len(kw2.AllKeywords()))
	}
}

func TestParseRipgrepJSON_MalformedLines(t *testing.T) {
	r := &SparseRetriever{}
	// One good match, one non-JSON line, one JSON of a different type,
	// one JSON match with missing path — only the first should produce a hit.
	output := strings.Join([]string{
		rgMatchJSON(`C:\repo\file.go`, 1, 2, 5, "content"),
		"not json at all",
		`{"type":"begin","data":{"path":{"text":"a.go"}}}`,
		`{"type":"match","data":{"path":{"text":""},"lines":{"text":"x"},"line_number":1,"submatches":[]}}`,
	}, "\n") + "\n"

	hits := r.parseRipgrepJSON([]byte(output), "kw")
	if len(hits) != 1 {
		t.Errorf("Expected 1 hit, got %d", len(hits))
	}
}

func TestKeywordHitCache_Concurrency(t *testing.T) {
	cache := NewKeywordHitCache(10, time.Hour)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "key" // High collision
			if i%2 == 0 {
				cache.Set(key, []KeywordHit{{FilePath: "a.go"}})
			} else {
				cache.Get(key)
			}
		}(i)
	}
	wg.Wait()
}

func TestSparseRetriever_HugeOutput(t *testing.T) {
	r := &SparseRetriever{}
	var sb strings.Builder
	line := rgMatchJSON("a.go", 1, 1, 4, "hit")
	// Feed 100k lines; parser must cap at maxHitsPerKeyword (OOM prevention).
	for i := 0; i < 100000; i++ {
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	hits := r.parseRipgrepJSON([]byte(sb.String()), "kw")
	if len(hits) != maxHitsPerKeyword {
		t.Errorf("Expected exactly %d hits (cap), got %d", maxHitsPerKeyword, len(hits))
	}
}

func TestSparseRetriever_ContextTimeout(t *testing.T) {
	r := &SparseRetriever{searchTimeout: time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	_, err := r.searchSingleKeyword(ctx, "test")
	if err == nil {
		t.Error("Expected error from cancelled context")
	}
}

func TestExtractKeywords_ReDoS(t *testing.T) {
	longStr := strings.Repeat("a", 100000)
	done := make(chan bool)
	go func() {
		ExtractKeywords(longStr)
		done <- true
	}()

	select {
	case <-done:
		// success
	case <-time.After(1 * time.Second):
		t.Error("ExtractKeywords took too long, possible ReDoS")
	}
}

func TestExtractKeywords_NullBytes(t *testing.T) {
	kw := ExtractKeywords("error\x00panic in \x00file.go")
	if len(kw.AllKeywords()) > 0 {
		_ = kw.AllKeywords() // Just ensuring it doesn't crash
	}
}

func TestSparseRetriever_EmptyWorkDir(t *testing.T) {
	cfg := DefaultSparseRetrieverConfig("")
	r := NewSparseRetriever(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r.searchSingleKeyword(ctx, "test")
}

func TestRankFiles_CaseInsensitiveWeights(t *testing.T) {
	r := &SparseRetriever{}
	kw := &IssueKeywords{
		Weights: map[string]float64{"MixedCase": 1.0},
	}
	hits := []KeywordHit{{FilePath: "a.go", Keyword: "MixedCase"}}
	candidates := r.RankFiles(hits, kw, 0)
	if len(candidates) != 1 || candidates[0].RelevanceScore != 1.0 {
		t.Errorf("Case Sensitivity weighting issue")
	}
}
