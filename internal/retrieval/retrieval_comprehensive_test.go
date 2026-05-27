package retrieval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// DefaultSparseRetrieverConfig
// =============================================================================

func TestDefaultSparseRetrieverConfig_ShouldHaveSensibleDefaults(t *testing.T) {
	t.Parallel()
	cfg := DefaultSparseRetrieverConfig("/test/dir")

	if cfg.WorkDir != "/test/dir" {
		t.Errorf("WorkDir = %q, want /test/dir", cfg.WorkDir)
	}
	if cfg.MaxResults != 100 {
		t.Errorf("MaxResults = %d, want 100", cfg.MaxResults)
	}
	if cfg.SearchTimeout != 30*time.Second {
		t.Errorf("SearchTimeout = %v, want 30s", cfg.SearchTimeout)
	}
	if cfg.Parallelism != 4 {
		t.Errorf("Parallelism = %d, want 4", cfg.Parallelism)
	}
	if cfg.CacheSize != 1000 {
		t.Errorf("CacheSize = %d, want 1000", cfg.CacheSize)
	}
	if cfg.CacheTTL != 5*time.Minute {
		t.Errorf("CacheTTL = %v, want 5m", cfg.CacheTTL)
	}
	if len(cfg.ExcludePatterns) == 0 {
		t.Error("expected non-empty ExcludePatterns")
	}
}

// =============================================================================
// NewSparseRetriever
// =============================================================================

func TestNewSparseRetriever_WhenNilConfig_ShouldUseDefaults(t *testing.T) {
	t.Parallel()
	r := NewSparseRetriever(nil)
	if r == nil {
		t.Fatal("expected non-nil retriever")
	}
	if r.workDir != "." {
		t.Errorf("workDir = %q, want '.'", r.workDir)
	}
	if r.maxResults != 100 {
		t.Errorf("maxResults = %d, want 100", r.maxResults)
	}
}

func TestNewSparseRetriever_WhenCustomConfig_ShouldApplySettings(t *testing.T) {
	t.Parallel()
	cfg := &SparseRetrieverConfig{
		WorkDir:    "/custom",
		MaxResults: 50,
		CacheSize:  100,
		CacheTTL:   1 * time.Minute,
	}
	r := NewSparseRetriever(cfg)
	if r.maxResults != 50 {
		t.Errorf("maxResults = %d, want 50", r.maxResults)
	}
}

// =============================================================================
// ExtractKeywords - additional cases
// =============================================================================

func TestExtractKeywords_WhenClassDefinition_ShouldExtractAsPrimary(t *testing.T) {
	t.Parallel()
	kw := ExtractKeywords("class MyHandler is broken")

	found := false
	for _, p := range kw.Primary {
		if p == "MyHandler" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected MyHandler in Primary, got %v", kw.Primary)
	}
}

func TestExtractKeywords_WhenMultipleFilePaths_ShouldExtractAll(t *testing.T) {
	t.Parallel()
	kw := ExtractKeywords("errors in main.go and utils.py at line 42")

	if len(kw.MentionedFiles) < 2 {
		t.Errorf("expected at least 2 files, got %v", kw.MentionedFiles)
	}
}

func TestExtractKeywords_WhenErrorTypes_ShouldExtractAsPrimaryAndSymbols(t *testing.T) {
	t.Parallel()
	kw := ExtractKeywords("raised ValueError and TypeError in handler")

	primaries := strings.Join(kw.Primary, ",")
	if !strings.Contains(primaries, "ValueError") {
		t.Errorf("expected ValueError in Primary, got %v", kw.Primary)
	}
	if !strings.Contains(primaries, "TypeError") {
		t.Errorf("expected TypeError in Primary, got %v", kw.Primary)
	}
}

func TestExtractKeywords_WhenQuotedIdentifiers_ShouldExtractAsTertiary(t *testing.T) {
	t.Parallel()
	kw := ExtractKeywords(`the "specialConfig" variable is wrong`)

	found := false
	for _, ter := range kw.Tertiary {
		if ter == "specialConfig" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected specialConfig in Tertiary, got %v", kw.Tertiary)
	}
}

func TestExtractKeywords_WhenDuplicateSymbols_ShouldDeduplicate(t *testing.T) {
	t.Parallel()
	kw := ExtractKeywords("FooError and FooError again, FooError once more")
	count := 0
	for _, p := range kw.Primary {
		if p == "FooError" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected FooError exactly once in Primary, got %d times", count)
	}
}

func TestExtractKeywords_WhenCommonWords_ShouldFilter(t *testing.T) {
	t.Parallel()
	kw := ExtractKeywords("the error class is not found")

	all := kw.AllKeywords()
	for _, word := range all {
		lower := strings.ToLower(word)
		if lower == "the" || lower == "is" || lower == "not" {
			t.Errorf("common word %q should be filtered", word)
		}
	}
}

// =============================================================================
// IssueKeywords.AllKeywords
// =============================================================================

func TestIssueKeywords_AllKeywords_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()
	kw := &IssueKeywords{}
	if len(kw.AllKeywords()) != 0 {
		t.Errorf("expected empty AllKeywords, got %v", kw.AllKeywords())
	}
}

func TestIssueKeywords_AllKeywords_ShouldMaintainPriorityOrder(t *testing.T) {
	t.Parallel()
	kw := &IssueKeywords{
		Primary:   []string{"P1"},
		Secondary: []string{"S1"},
		Tertiary:  []string{"T1"},
	}
	all := kw.AllKeywords()
	if len(all) != 3 || all[0] != "P1" || all[1] != "S1" || all[2] != "T1" {
		t.Errorf("expected [P1 S1 T1], got %v", all)
	}
}

// =============================================================================
// KeywordHitCache - additional
// =============================================================================

func TestKeywordHitCache_Get_WhenMissing_ShouldReturnFalse(t *testing.T) {
	t.Parallel()
	cache := NewKeywordHitCache(10, time.Hour)
	_, ok := cache.Get("missing")
	if ok {
		t.Error("expected miss for non-existent key")
	}
}

func TestKeywordHitCache_Set_WhenValidTTL_ShouldBeRetrievable(t *testing.T) {
	t.Parallel()
	cache := NewKeywordHitCache(10, time.Hour)
	hits := []KeywordHit{{FilePath: "a.go", Keyword: "test"}}
	cache.Set("key", hits)

	got, ok := cache.Get("key")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if len(got) != 1 || got[0].FilePath != "a.go" {
		t.Errorf("unexpected cache result: %v", got)
	}
}

func TestKeywordHitCache_Clear_ShouldEmptyCache(t *testing.T) {
	t.Parallel()
	cache := NewKeywordHitCache(10, time.Hour)
	cache.Set("k1", []KeywordHit{{FilePath: "a.go"}})
	cache.Set("k2", []KeywordHit{{FilePath: "b.go"}})

	cache.Clear()

	if _, ok := cache.Get("k1"); ok {
		t.Error("expected miss after clear")
	}
	if _, ok := cache.Get("k2"); ok {
		t.Error("expected miss after clear")
	}
}

func TestKeywordHitCache_Set_WhenAtCapacity_ShouldEvictOldest(t *testing.T) {
	t.Parallel()
	cache := NewKeywordHitCache(2, time.Hour)
	cache.Set("first", []KeywordHit{{FilePath: "1.go"}})

	// Ensure "first" is oldest
	cache.mu.Lock()
	cache.entries["first"].timestamp = time.Now().Add(-1 * time.Hour)
	cache.mu.Unlock()

	cache.Set("second", []KeywordHit{{FilePath: "2.go"}})
	cache.Set("third", []KeywordHit{{FilePath: "3.go"}}) // Should evict "first"

	if _, ok := cache.Get("first"); ok {
		t.Error("expected 'first' to be evicted")
	}
	if _, ok := cache.Get("second"); !ok {
		t.Error("expected 'second' to remain")
	}
	if _, ok := cache.Get("third"); !ok {
		t.Error("expected 'third' to remain")
	}
}

// =============================================================================
// RankFiles - additional
// =============================================================================

func TestRankFiles_WhenEmptyHits_ShouldReturnNil(t *testing.T) {
	t.Parallel()
	r := &SparseRetriever{}
	result := r.RankFiles(nil, &IssueKeywords{Weights: map[string]float64{}}, 10)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestRankFiles_WhenLimitApplied_ShouldTruncate(t *testing.T) {
	t.Parallel()
	r := &SparseRetriever{}
	kw := &IssueKeywords{Weights: map[string]float64{"k1": 1.0, "k2": 0.5}}

	hits := []KeywordHit{
		{FilePath: "a.go", Keyword: "k1"},
		{FilePath: "b.go", Keyword: "k1"},
		{FilePath: "c.go", Keyword: "k2"},
		{FilePath: "d.go", Keyword: "k2"},
	}

	result := r.RankFiles(hits, kw, 2)
	if len(result) != 2 {
		t.Errorf("expected 2 results with limit=2, got %d", len(result))
	}
}

func TestRankFiles_WhenZeroLimit_ShouldReturnAll(t *testing.T) {
	t.Parallel()
	r := &SparseRetriever{}
	kw := &IssueKeywords{Weights: map[string]float64{"k1": 1.0}}

	hits := []KeywordHit{
		{FilePath: "a.go", Keyword: "k1"},
		{FilePath: "b.go", Keyword: "k1"},
	}

	result := r.RankFiles(hits, kw, 0)
	if len(result) != 2 {
		t.Errorf("expected 2 results with limit=0, got %d", len(result))
	}
}

func TestRankFiles_ShouldSortByDescendingScore(t *testing.T) {
	t.Parallel()
	r := &SparseRetriever{}
	kw := &IssueKeywords{
		Weights: map[string]float64{"high": 1.0, "low": 0.1},
	}

	hits := []KeywordHit{
		{FilePath: "low.go", Keyword: "low"},
		{FilePath: "high.go", Keyword: "high"},
	}

	result := r.RankFiles(hits, kw, 0)
	if len(result) < 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	if result[0].FilePath != "high.go" {
		t.Errorf("expected high.go first, got %s", result[0].FilePath)
	}
}

// =============================================================================
// determineTier
// =============================================================================

func TestDetermineTier_WhenMentionedFile_ShouldReturnTier1(t *testing.T) {
	t.Parallel()
	r := &SparseRetriever{}
	kw := &IssueKeywords{
		MentionedFiles: []string{"src/handler.go"},
		Weights:        map[string]float64{},
	}
	tier := r.determineTier("repo/src/handler.go", 0.5, kw)
	if tier != 1 {
		t.Errorf("tier = %d, want 1", tier)
	}
}

func TestDetermineTier_WhenHighScore_ShouldReturnTier2(t *testing.T) {
	t.Parallel()
	r := &SparseRetriever{}
	kw := &IssueKeywords{Weights: map[string]float64{}}
	tier := r.determineTier("file.go", 2.5, kw)
	if tier != 2 {
		t.Errorf("tier = %d, want 2", tier)
	}
}

func TestDetermineTier_WhenMediumScore_ShouldReturnTier3(t *testing.T) {
	t.Parallel()
	r := &SparseRetriever{}
	kw := &IssueKeywords{Weights: map[string]float64{}}
	tier := r.determineTier("file.go", 1.5, kw)
	if tier != 3 {
		t.Errorf("tier = %d, want 3", tier)
	}
}

func TestDetermineTier_WhenLowScore_ShouldReturnTier4(t *testing.T) {
	t.Parallel()
	r := &SparseRetriever{}
	kw := &IssueKeywords{Weights: map[string]float64{}}
	tier := r.determineTier("file.go", 0.5, kw)
	if tier != 4 {
		t.Errorf("tier = %d, want 4", tier)
	}
}

// =============================================================================
// parseRipgrepJSON - edge cases
// =============================================================================

func TestParseRipgrepJSON_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()
	r := &SparseRetriever{}
	hits := r.parseRipgrepJSON(nil, "kw")
	if len(hits) != 0 {
		t.Errorf("expected 0 hits for empty output, got %d", len(hits))
	}
}

func TestParseRipgrepJSON_WhenPartialLine_ShouldSkip(t *testing.T) {
	t.Parallel()
	r := &SparseRetriever{}
	output := []byte("file.go:1:2\n") // Not valid JSON — should be skipped
	hits := r.parseRipgrepJSON(output, "kw")
	if len(hits) != 0 {
		t.Errorf("expected 0 hits for non-JSON line, got %d", len(hits))
	}
}

func TestParseRipgrepJSON_WhenContentContainsColons_ShouldHandleGracefully(t *testing.T) {
	t.Parallel()
	r := &SparseRetriever{}
	output := []byte(rgMatchJSON("file.go", 10, 5, 11, `map[string]int{"a": 1, "b": 2}`) + "\n")
	hits := r.parseRipgrepJSON(output, "kw")
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	if !strings.Contains(hits[0].Context, "map[string]int") {
		t.Errorf("context should contain content with embedded colons: %q", hits[0].Context)
	}
}

// =============================================================================
// isCommonWord
// =============================================================================

func TestIsCommonWord_WhenCommon_ShouldReturnTrue(t *testing.T) {
	t.Parallel()
	tests := []string{"the", "is", "def", "class", "return", "test", "error"}
	for _, w := range tests {
		if !isCommonWord(w) {
			t.Errorf("expected %q to be common", w)
		}
	}
}

func TestIsCommonWord_WhenNotCommon_ShouldReturnFalse(t *testing.T) {
	t.Parallel()
	tests := []string{"FooError", "handleRequest", "parseJSON", "validateInput"}
	for _, w := range tests {
		if isCommonWord(w) {
			t.Errorf("expected %q to not be common", w)
		}
	}
}

func TestIsCommonWord_WhenTooShort_ShouldReturnTrue(t *testing.T) {
	t.Parallel()
	if !isCommonWord("ab") {
		t.Error("expected 2-char word to be common")
	}
	if !isCommonWord("a") {
		t.Error("expected 1-char word to be common")
	}
}

// =============================================================================
// uniqueStrings
// =============================================================================

func TestUniqueStrings_WhenDuplicates_ShouldRemoveThem(t *testing.T) {
	t.Parallel()
	input := []string{"a", "b", "a", "c", "b"}
	result := uniqueStrings(input)
	if len(result) != 3 {
		t.Errorf("expected 3 unique strings, got %d: %v", len(result), result)
	}
}

func TestUniqueStrings_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()
	result := uniqueStrings(nil)
	if len(result) != 0 {
		t.Errorf("expected empty, got %v", result)
	}
}

func TestUniqueStrings_WhenAllUnique_ShouldPreserveOrder(t *testing.T) {
	t.Parallel()
	input := []string{"c", "a", "b"}
	result := uniqueStrings(input)
	if len(result) != 3 || result[0] != "c" || result[1] != "a" || result[2] != "b" {
		t.Errorf("expected [c a b], got %v", result)
	}
}

// =============================================================================
// normalizePathSeparators
// =============================================================================

func TestNormalizePathSeparators_ShouldConvertBackslashes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{`internal\core\kernel.go`, "internal/core/kernel.go"},
		{"already/forward/slashes", "already/forward/slashes"},
		{`C:\Users\test\file.go`, "C:/Users/test/file.go"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := normalizePathSeparators(tt.input)
			if got != tt.want {
				t.Errorf("normalizePathSeparators(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// =============================================================================
// TieredContextBuilder
// =============================================================================

func TestDefaultTieredContextConfig_ShouldHaveSensibleDefaults(t *testing.T) {
	t.Parallel()
	cfg := DefaultTieredContextConfig("/test")
	if cfg.WorkDir != "/test" {
		t.Errorf("WorkDir = %q, want /test", cfg.WorkDir)
	}
	if cfg.Tier1Budget != 0.30 {
		t.Errorf("Tier1Budget = %f, want 0.30", cfg.Tier1Budget)
	}
	if cfg.MaxTotal != 50 {
		t.Errorf("MaxTotal = %d, want 50", cfg.MaxTotal)
	}
}

func TestNewTieredContextBuilder_WhenNilConfig_ShouldUseDefaults(t *testing.T) {
	t.Parallel()
	builder := NewTieredContextBuilder(nil)
	if builder == nil {
		t.Fatal("expected non-nil builder")
	}
	if builder.workDir != "." {
		t.Errorf("workDir = %q, want '.'", builder.workDir)
	}
}

func TestNewTieredContextBuilder_WhenCustomConfig_ShouldApply(t *testing.T) {
	t.Parallel()
	cfg := &TieredContextConfig{
		WorkDir:     "/custom",
		Tier1Budget: 0.50,
		Tier2Budget: 0.30,
		Tier3Budget: 0.15,
		Tier4Budget: 0.05,
		MaxTotal:    100,
	}
	builder := NewTieredContextBuilder(cfg)
	if builder.maxTier1 != 50 {
		t.Errorf("maxTier1 = %d, want 50", builder.maxTier1)
	}
}

func TestNewTieredContextBuilder_WhenZeroMaxTotal_ShouldDefault50(t *testing.T) {
	t.Parallel()
	cfg := &TieredContextConfig{
		WorkDir:     "/test",
		Tier1Budget: 0.30,
		Tier2Budget: 0.40,
		MaxTotal:    0,
	}
	builder := NewTieredContextBuilder(cfg)
	if builder.maxTier1 != int(50*0.30) {
		t.Errorf("maxTier1 = %d, want %d", builder.maxTier1, int(50*0.30))
	}
}

// =============================================================================
// TieredContext helpers
// =============================================================================

func TestTieredContext_GetFilesByTier_ShouldFilterCorrectly(t *testing.T) {
	t.Parallel()
	tc := &TieredContext{
		Files: []ContextFile{
			{FilePath: "a.go", Tier: 1},
			{FilePath: "b.go", Tier: 2},
			{FilePath: "c.go", Tier: 1},
			{FilePath: "d.go", Tier: 3},
		},
	}

	tier1 := tc.GetFilesByTier(1)
	if len(tier1) != 2 {
		t.Errorf("expected 2 tier-1 files, got %d", len(tier1))
	}

	tier4 := tc.GetFilesByTier(4)
	if len(tier4) != 0 {
		t.Errorf("expected 0 tier-4 files, got %d", len(tier4))
	}
}

func TestTieredContext_GetTopFiles_ShouldSortByRelevance(t *testing.T) {
	t.Parallel()
	tc := &TieredContext{
		Files: []ContextFile{
			{FilePath: "low.go", RelevanceScore: 0.1},
			{FilePath: "high.go", RelevanceScore: 0.9},
			{FilePath: "mid.go", RelevanceScore: 0.5},
		},
	}

	top := tc.GetTopFiles(2)
	if len(top) != 2 {
		t.Fatalf("expected 2 files, got %d", len(top))
	}
	if top[0].FilePath != "high.go" {
		t.Errorf("expected high.go first, got %s", top[0].FilePath)
	}
	if top[1].FilePath != "mid.go" {
		t.Errorf("expected mid.go second, got %s", top[1].FilePath)
	}
}

func TestTieredContext_GetTopFiles_WhenNLargerThanFiles_ShouldReturnAll(t *testing.T) {
	t.Parallel()
	tc := &TieredContext{
		Files: []ContextFile{
			{FilePath: "a.go", RelevanceScore: 0.5},
		},
	}

	top := tc.GetTopFiles(100)
	if len(top) != 1 {
		t.Errorf("expected 1 file, got %d", len(top))
	}
}

func TestTieredContext_GetFilePaths_ShouldReturnAllPaths(t *testing.T) {
	t.Parallel()
	tc := &TieredContext{
		Files: []ContextFile{
			{FilePath: "a.go"},
			{FilePath: "b.py"},
			{FilePath: "c.rs"},
		},
	}

	paths := tc.GetFilePaths()
	if len(paths) != 3 {
		t.Errorf("expected 3 paths, got %d", len(paths))
	}
	if paths[0] != "a.go" || paths[1] != "b.py" || paths[2] != "c.rs" {
		t.Errorf("unexpected paths: %v", paths)
	}
}

func TestTieredContext_GetFilePaths_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()
	tc := &TieredContext{Files: []ContextFile{}}
	paths := tc.GetFilePaths()
	if len(paths) != 0 {
		t.Errorf("expected empty paths, got %v", paths)
	}
}

func TestTieredContext_LoadContent_ShouldRespectMaxBytes(t *testing.T) {
	// Create temp files with known content
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "a.go")
	file2 := filepath.Join(tmpDir, "b.go")
	os.WriteFile(file1, []byte("content1"), 0644)
	os.WriteFile(file2, []byte("content2"), 0644)

	tc := &TieredContext{
		Files: []ContextFile{
			{FilePath: file1},
			{FilePath: file2},
		},
	}

	// Set max to allow only first file
	if err := tc.LoadContent(8); err != nil {
		t.Fatalf("LoadContent: %v", err)
	}

	if tc.Files[0].Content != "content1" {
		t.Errorf("expected content1, got %q", tc.Files[0].Content)
	}
	if tc.Files[1].Content != "" {
		t.Errorf("expected empty content for second file (exceeded budget), got %q", tc.Files[1].Content)
	}
}

func TestTieredContext_LoadContent_WhenFileMissing_ShouldSkip(t *testing.T) {
	tc := &TieredContext{
		Files: []ContextFile{
			{FilePath: "/nonexistent/file.go"},
		},
	}

	if err := tc.LoadContent(1000); err != nil {
		t.Fatalf("LoadContent: %v", err)
	}

	if tc.Files[0].Content != "" {
		t.Error("expected empty content for missing file")
	}
}

// =============================================================================
// SearchKeywords nil/empty handling
// =============================================================================

func TestSearchKeywords_WhenNilKeywords_ShouldReturnNilNil(t *testing.T) {
	t.Parallel()
	r := NewSparseRetriever(DefaultSparseRetrieverConfig(t.TempDir()))

	hits, err := r.SearchKeywords(nil, nil)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if hits != nil {
		t.Errorf("expected nil hits, got %v", hits)
	}
}

func TestSearchKeywords_WhenEmptyKeywords_ShouldReturnNilNil(t *testing.T) {
	t.Parallel()
	r := NewSparseRetriever(DefaultSparseRetrieverConfig(t.TempDir()))

	kw := &IssueKeywords{}
	hits, err := r.SearchKeywords(nil, kw)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if hits != nil {
		t.Errorf("expected nil hits, got %v", hits)
	}
}
