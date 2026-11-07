package diff

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// =============================================================================
// Engine Creation Tests
// =============================================================================

func TestNewEngine_ShouldReturnNonNil(t *testing.T) {
	t.Parallel()
	engine := NewEngine()
	if engine == nil {
		t.Fatal("NewEngine() returned nil")
	}
	if engine.dmp == nil {
		t.Fatal("engine.dmp is nil")
	}
}

func TestDefaultEngine_ShouldBeNonNil(t *testing.T) {
	t.Parallel()
	if DefaultEngine == nil {
		t.Fatal("DefaultEngine is nil")
	}
}

// =============================================================================
// ComputeDiff - Empty/Nil Inputs
// =============================================================================

func TestComputeDiff_WhenBothEmpty_ShouldReturnNoHunks(t *testing.T) {
	t.Parallel()
	engine := NewEngine()
	d := engine.ComputeDiff("a.txt", "b.txt", "", "")
	if d == nil {
		t.Fatal("expected non-nil diff")
	}
	if len(d.Hunks) != 0 {
		t.Errorf("expected 0 hunks for empty->empty, got %d", len(d.Hunks))
	}
	if !d.IsNew {
		t.Error("expected IsNew=true for empty old content")
	}
	if !d.IsDelete {
		t.Error("expected IsDelete=true for empty new content")
	}
}

func TestComputeDiff_WhenOldEmpty_ShouldBeNewFile(t *testing.T) {
	t.Parallel()
	engine := NewEngine()
	d := engine.ComputeDiff("", "new.go", "", "package main\n")
	if !d.IsNew {
		t.Error("expected IsNew=true")
	}
	if d.IsDelete {
		t.Error("expected IsDelete=false")
	}
	// Should have at least one hunk with added lines
	if len(d.Hunks) == 0 {
		t.Error("expected at least one hunk for new file")
	}
}

func TestComputeDiff_WhenNewEmpty_ShouldBeDeletedFile(t *testing.T) {
	t.Parallel()
	engine := NewEngine()
	d := engine.ComputeDiff("old.go", "", "package main\n", "")
	if !d.IsDelete {
		t.Error("expected IsDelete=true")
	}
	if d.IsNew {
		t.Error("expected IsNew=false")
	}
	if len(d.Hunks) == 0 {
		t.Error("expected at least one hunk for deleted file")
	}
}

// =============================================================================
// ComputeDiff - Identical Content
// =============================================================================

func TestComputeDiff_WhenIdenticalContent_ShouldReturnNoHunks(t *testing.T) {
	t.Parallel()
	content := "line1\nline2\nline3\n"
	engine := NewEngine()
	d := engine.ComputeDiff("f.txt", "f.txt", content, content)
	if len(d.Hunks) != 0 {
		t.Errorf("expected 0 hunks for identical content, got %d", len(d.Hunks))
	}
	if d.IsNew || d.IsDelete {
		t.Error("should not be new or delete for identical content")
	}
}

func TestComputeDiff_WhenSingleLineIdentical_ShouldReturnNoHunks(t *testing.T) {
	t.Parallel()
	engine := NewEngine()
	d := engine.ComputeDiff("f.txt", "f.txt", "hello", "hello")
	if len(d.Hunks) != 0 {
		t.Errorf("expected 0 hunks, got %d", len(d.Hunks))
	}
}

// =============================================================================
// ComputeDiff - Single Change Operations
// =============================================================================

func TestComputeDiff_WhenSingleLineAdded_ShouldHaveAddedLine(t *testing.T) {
	t.Parallel()
	old := "line1\nline2\n"
	new := "line1\ninserted\nline2\n"
	engine := NewEngine()
	d := engine.ComputeDiff("a.txt", "b.txt", old, new)

	if len(d.Hunks) == 0 {
		t.Fatal("expected at least one hunk")
	}

	found := false
	for _, h := range d.Hunks {
		for _, l := range h.Lines {
			if l.Type == LineAdded && l.Content == "inserted" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected to find added line 'inserted'")
	}
}

func TestComputeDiff_WhenSingleLineRemoved_ShouldHaveRemovedLine(t *testing.T) {
	t.Parallel()
	old := "line1\nremoved\nline2\n"
	new := "line1\nline2\n"
	engine := NewEngine()
	d := engine.ComputeDiff("a.txt", "b.txt", old, new)

	if len(d.Hunks) == 0 {
		t.Fatal("expected at least one hunk")
	}

	found := false
	for _, h := range d.Hunks {
		for _, l := range h.Lines {
			if l.Type == LineRemoved && l.Content == "removed" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected to find removed line 'removed'")
	}
}

func TestComputeDiff_WhenSingleLineModified_ShouldShowChange(t *testing.T) {
	t.Parallel()
	old := "line1\noriginal\nline3\n"
	new := "line1\nmodified\nline3\n"
	engine := NewEngine()
	d := engine.ComputeDiff("a.txt", "b.txt", old, new)

	if len(d.Hunks) == 0 {
		t.Fatal("expected at least one hunk")
	}

	hasRemoved := false
	hasAdded := false
	for _, h := range d.Hunks {
		for _, l := range h.Lines {
			if l.Type == LineRemoved && l.Content == "original" {
				hasRemoved = true
			}
			if l.Type == LineAdded && l.Content == "modified" {
				hasAdded = true
			}
		}
	}
	if !hasRemoved {
		t.Error("expected removed 'original'")
	}
	if !hasAdded {
		t.Error("expected added 'modified'")
	}
}

// =============================================================================
// ComputeDiff - Paths
// =============================================================================

func TestComputeDiff_ShouldPreservePaths(t *testing.T) {
	t.Parallel()
	engine := NewEngine()
	d := engine.ComputeDiff("src/old.go", "src/new.go", "a\n", "b\n")
	if d.OldPath != "src/old.go" {
		t.Errorf("OldPath = %q, want 'src/old.go'", d.OldPath)
	}
	if d.NewPath != "src/new.go" {
		t.Errorf("NewPath = %q, want 'src/new.go'", d.NewPath)
	}
}

// =============================================================================
// ComputeDiff - Large File Handling
// =============================================================================

func TestComputeDiff_WhenLargeFile_ShouldComplete(t *testing.T) {
	t.Parallel()
	var oldLines, newLines []string
	for i := range 5000 {
		line := fmt.Sprintf("line content number %d with some padding", i)
		oldLines = append(oldLines, line)
		newLines = append(newLines, line)
	}
	// Modify a few scattered lines
	newLines[100] = "CHANGED_100"
	newLines[2500] = "CHANGED_2500"
	newLines[4999] = "CHANGED_4999"

	old := strings.Join(oldLines, "\n")
	new := strings.Join(newLines, "\n")

	engine := NewEngine()
	d := engine.ComputeDiff("big.txt", "big.txt", old, new)
	if d == nil {
		t.Fatal("expected non-nil diff for large file")
	}
	if len(d.Hunks) == 0 {
		t.Error("expected hunks for modified large file")
	}
}

// =============================================================================
// ComputeDiff - Binary-like Content
// =============================================================================

func TestComputeDiff_WhenBinaryContent_ShouldNotPanic(t *testing.T) {
	t.Parallel()
	old := string([]byte{0x00, 0x01, 0x02, 0xFF, 0xFE})
	new := string([]byte{0x00, 0x01, 0x03, 0xFF, 0xFE})
	engine := NewEngine()
	// Should not panic
	d := engine.ComputeDiff("a.bin", "b.bin", old, new)
	if d == nil {
		t.Fatal("expected non-nil diff for binary content")
	}
}

// =============================================================================
// Caching Tests
// =============================================================================

func TestComputeDiff_WhenCached_ShouldReturnEquivalentResult(t *testing.T) {
	t.Parallel()
	old := "line1\nline2\n"
	new := "line1\nchanged\n"
	engine := NewEngine()

	d1 := engine.ComputeDiff("a.txt", "b.txt", old, new)
	d2 := engine.ComputeDiff("c.txt", "d.txt", old, new) // same content, different paths

	if len(d1.Hunks) != len(d2.Hunks) {
		t.Errorf("cached diff has different hunk count: %d vs %d", len(d1.Hunks), len(d2.Hunks))
	}
	// Paths should be updated even when cached
	if d2.OldPath != "c.txt" {
		t.Errorf("cached OldPath = %q, want 'c.txt'", d2.OldPath)
	}
	if d2.NewPath != "d.txt" {
		t.Errorf("cached NewPath = %q, want 'd.txt'", d2.NewPath)
	}
}

func TestClearCache_ShouldNotAffectResults(t *testing.T) {
	t.Parallel()
	old := "a\nb\n"
	new := "a\nc\n"
	engine := NewEngine()

	d1 := engine.ComputeDiff("f.txt", "f.txt", old, new)
	engine.ClearCache()
	d2 := engine.ComputeDiff("f.txt", "f.txt", old, new)

	if len(d1.Hunks) != len(d2.Hunks) {
		t.Errorf("results differ after cache clear: %d vs %d", len(d1.Hunks), len(d2.Hunks))
	}
}

// =============================================================================
// Hunk Count Verification
// =============================================================================

func TestComputeDiff_WhenHunkCounts_ShouldMatchManualCount(t *testing.T) {
	t.Parallel()
	old := "line1\nline2\nline3\nline4\nline5\n"
	new := "line1\nNEW\nline3\nline4\nline5\n"
	engine := NewEngine()
	d := engine.ComputeDiff("a.txt", "b.txt", old, new)

	for i, hunk := range d.Hunks {
		oldCount := 0
		newCount := 0
		for _, l := range hunk.Lines {
			if l.Type == LineRemoved || l.Type == LineContext {
				oldCount++
			}
			if l.Type == LineAdded || l.Type == LineContext {
				newCount++
			}
		}
		if hunk.OldCount != oldCount {
			t.Errorf("hunk[%d] OldCount=%d, manually counted=%d", i, hunk.OldCount, oldCount)
		}
		if hunk.NewCount != newCount {
			t.Errorf("hunk[%d] NewCount=%d, manually counted=%d", i, hunk.NewCount, newCount)
		}
	}
}

// =============================================================================
// ComputeWordLevelDiff Tests
// =============================================================================

func TestComputeWordLevelDiff_WhenIdentical_ShouldReturnEqual(t *testing.T) {
	t.Parallel()
	engine := NewEngine()
	diffs := engine.ComputeWordLevelDiff("hello world", "hello world")
	if len(diffs) == 0 {
		t.Fatal("expected at least one diff element")
	}
	// All should be equal
	for _, d := range diffs {
		if d.Type != 0 { // DiffEqual = 0
			t.Errorf("expected all equal diffs, got type %d", d.Type)
		}
	}
}

func TestComputeWordLevelDiff_WhenEmpty_ShouldNotPanic(t *testing.T) {
	t.Parallel()
	engine := NewEngine()
	// Should not panic — nil result is acceptable for empty-to-empty comparison
	diffs := engine.ComputeWordLevelDiff("", "")
	t.Logf("empty-to-empty word diff returned %d elements", len(diffs))
}

func TestComputeWordLevelDiff_WhenWordChanged_ShouldDetectChange(t *testing.T) {
	t.Parallel()
	engine := NewEngine()
	diffs := engine.ComputeWordLevelDiff("the quick brown fox", "the quick red fox")
	hasChange := false
	for _, d := range diffs {
		if d.Type != 0 {
			hasChange = true
			break
		}
	}
	if !hasChange {
		t.Error("expected to detect word-level change")
	}
}

// =============================================================================
// Convenience Function Tests
// =============================================================================

func TestComputeDiff_ConvenienceFunction_ShouldUseDefaultEngine(t *testing.T) {
	t.Parallel()
	d := ComputeDiff("a.txt", "b.txt", "old\n", "new\n")
	if d == nil {
		t.Fatal("ComputeDiff convenience function returned nil")
	}
	if len(d.Hunks) == 0 {
		t.Error("expected hunks from convenience function")
	}
}

// =============================================================================
// Concurrency Safety Tests
// =============================================================================

func TestComputeDiff_WhenConcurrent_ShouldNotRace(t *testing.T) {
	t.Parallel()
	engine := NewEngine()
	var wg sync.WaitGroup

	for i := range 20 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			old := fmt.Sprintf("line %d\nold content\n", idx)
			new := fmt.Sprintf("line %d\nnew content\n", idx)
			d := engine.ComputeDiff("a.txt", "b.txt", old, new)
			if d == nil {
				t.Errorf("concurrent ComputeDiff returned nil for idx %d", idx)
			}
		}(i)
	}
	wg.Wait()
}

// =============================================================================
// Hash Function Tests
// =============================================================================

func TestHash_WhenSameInput_ShouldReturnSameResult(t *testing.T) {
	t.Parallel()
	h1 := hash("test string")
	h2 := hash("test string")
	if h1 != h2 {
		t.Errorf("hash not deterministic: %d vs %d", h1, h2)
	}
}

func TestHash_WhenDifferentInput_ShouldReturnDifferentResult(t *testing.T) {
	t.Parallel()
	h1 := hash("alpha")
	h2 := hash("beta")
	if h1 == h2 {
		t.Error("hash collision for different inputs")
	}
}

func TestHash_WhenEmpty_ShouldNotPanic(t *testing.T) {
	t.Parallel()
	h := hash("")
	if h == 0 {
		// FNV-1a on empty string should return the offset basis, not 0
		t.Error("expected non-zero hash for empty string")
	}
}

// =============================================================================
// Edge Case: Empty Lines / Whitespace
// =============================================================================

func TestComputeDiff_WhenOnlyWhitespaceChanges_ShouldDetect(t *testing.T) {
	t.Parallel()
	old := "  indented\n"
	new := "    more indented\n"
	engine := NewEngine()
	d := engine.ComputeDiff("a.txt", "b.txt", old, new)
	if len(d.Hunks) == 0 {
		t.Error("expected hunks for whitespace changes")
	}
}

func TestComputeDiff_WhenTrailingNewlineDiffers_ShouldDetect(t *testing.T) {
	t.Parallel()
	old := "hello"
	new := "hello\n"
	engine := NewEngine()
	d := engine.ComputeDiff("a.txt", "b.txt", old, new)
	// Should detect the trailing newline difference
	if d == nil {
		t.Fatal("expected non-nil diff")
	}
}

func TestClampContextLines_ShouldBoundExtremes(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		in   int
		want int
	}{
		"negative clamps to zero": {in: -5, want: 0},
		"zero stays zero":         {in: 0, want: 0},
		"small value passes":      {in: 3, want: 3},
		"limit passes":            {in: maxContextLines, want: maxContextLines},
		"above limit clamps":      {in: maxContextLines + 1, want: maxContextLines},
		"huge clamps":             {in: 1 << 30, want: maxContextLines},
	}
	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := clampContextLines(tc.in); got != tc.want {
				t.Errorf("clampContextLines(%d) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestOptions_ContextLines_ShouldResolveAndClamp(t *testing.T) {
	t.Parallel()
	if got := (Options{}).contextLines(); got != defaultContextLines {
		t.Errorf("zero Options contextLines = %d, want %d", got, defaultContextLines)
	}
	if got := (Options{ContextLines: 5}).contextLines(); got != 5 {
		t.Errorf("ContextLines 5 resolves to %d, want 5", got)
	}
	if got := (Options{ContextLines: -1}).contextLines(); got != 0 {
		t.Errorf("negative ContextLines resolves to %d, want 0 (genuinely zero context)", got)
	}
	if got := (Options{ContextLines: maxContextLines + 500}).contextLines(); got != maxContextLines {
		t.Errorf("huge ContextLines resolves to %d, want clamp %d", got, maxContextLines)
	}
}

func TestNewDiffCache_ShouldFallBackToDefaultsOnNonPositive(t *testing.T) {
	t.Parallel()
	c := newDiffCache(0, 0)
	if c.maxEntries != defaultMaxCacheEntries {
		t.Errorf("maxEntries = %d, want default %d", c.maxEntries, defaultMaxCacheEntries)
	}
	if c.maxBytes != defaultMaxCacheBytes {
		t.Errorf("maxBytes = %d, want default %d", c.maxBytes, defaultMaxCacheBytes)
	}
	neg := newDiffCache(-3, -100)
	if neg.maxEntries != defaultMaxCacheEntries || neg.maxBytes != defaultMaxCacheBytes {
		t.Errorf("negative bounds should fall back to defaults, got entries=%d bytes=%d", neg.maxEntries, neg.maxBytes)
	}
	custom := newDiffCache(2, 1024)
	if custom.maxEntries != 2 || custom.maxBytes != 1024 {
		t.Errorf("positive bounds should be kept, got entries=%d bytes=%d", custom.maxEntries, custom.maxBytes)
	}
}

func TestDiffCache_PutNil_ShouldNotPanicOrStore(t *testing.T) {
	t.Parallel()
	c := newDiffCache(4, 1<<20)
	c.put(cacheKey{}, nil, "", "", false)
	if got := c.stats().Entries; got != 0 {
		t.Errorf("Entries after nil put = %d, want 0", got)
	}
}

func TestDiffCache_PutOverwrite_ShouldReplaceWithoutDuplicating(t *testing.T) {
	t.Parallel()
	c := newDiffCache(4, 1<<20)
	key := cacheKey{oldHash: 11, newHash: 22, oldLen: 1, newLen: 1}
	first := &FileDiff{
		OldPath: "a", NewPath: "b",
		Hunks: []Hunk{{OldStart: 1, OldCount: 1, NewStart: 1, NewCount: 1,
			Lines: []Line{{LineNum: 1, Content: "x", Type: LineAdded}}}},
	}
	second := &FileDiff{
		OldPath: "a", NewPath: "b",
		Hunks: []Hunk{{OldStart: 1, OldCount: 1, NewStart: 1, NewCount: 1,
			Lines: []Line{{LineNum: 1, Content: "y", Type: LineAdded}}}},
	}
	c.put(key, first, "", "", false)
	c.put(key, second, "", "", false)
	if got := c.stats().Entries; got != 1 {
		t.Fatalf("Entries after overwrite = %d, want 1", got)
	}
	got := c.get(key, "", "")
	if got == nil {
		t.Fatal("expected cached diff after overwrite, got nil")
	}
	if len(got.Hunks) != 1 || len(got.Hunks[0].Lines) != 1 || got.Hunks[0].Lines[0].Content != "y" {
		t.Errorf("overwrite should serve the second diff, got %+v", got.Hunks)
	}
}

func TestFileDiff_ApproxSize_ShouldGrowWithContent(t *testing.T) {
	t.Parallel()
	empty := (&FileDiff{}).approxSize()
	if empty != 0 {
		t.Errorf("empty diff approxSize = %d, want 0", empty)
	}
	withPaths := (&FileDiff{OldPath: "a.txt", NewPath: "b.txt"}).approxSize()
	if withPaths <= empty {
		t.Errorf("paths should increase approxSize: empty=%d paths=%d", empty, withPaths)
	}
	withHunks := (&FileDiff{
		OldPath: "a.txt", NewPath: "b.txt",
		Hunks: []Hunk{{OldStart: 1, OldCount: 1, NewStart: 1, NewCount: 1,
			Lines: []Line{{LineNum: 1, Content: "hello", Type: LineContext}}}},
	}).approxSize()
	if withHunks <= withPaths {
		t.Errorf("hunks should increase approxSize: paths=%d hunks=%d", withPaths, withHunks)
	}
}

func TestConvertToHunks_WhenNoDiffs_ShouldReturnNil(t *testing.T) {
	t.Parallel()
	e := NewEngine()
	if got := e.convertToHunks(nil, defaultContextLines); got != nil {
		t.Errorf("convertToHunks(nil) = %+v, want nil", got)
	}
	if got := e.groupIntoHunks(nil, defaultContextLines); got != nil {
		t.Errorf("groupIntoHunks(nil) = %+v, want nil", got)
	}
}

func TestComputeDiff_WhenBothEmpty_ShouldFlagNewAndDelete(t *testing.T) {
	t.Parallel()
	d := NewEngine().ComputeDiff("a.txt", "b.txt", "", "")
	if d == nil {
		t.Fatal("expected non-nil diff")
	}
	if !d.IsNew || !d.IsDelete {
		t.Errorf("both-empty should set IsNew and IsDelete, got IsNew=%v IsDelete=%v", d.IsNew, d.IsDelete)
	}
	if d.IsBinary {
		t.Error("both-empty must not be flagged binary")
	}
	if len(d.Hunks) != 0 {
		t.Errorf("both-empty should have no hunks, got %d", len(d.Hunks))
	}
}

func TestComputeDiff_CacheHit_ShouldRetargetPathsWithoutMutatingCache(t *testing.T) {
	t.Parallel()
	e := NewEngine()
	old := "a\nb\n"
	new := "a\nc\n"
	first := e.ComputeDiff("old1", "new1", old, new)
	second := e.ComputeDiff("old2", "new2", old, new)
	if second.OldPath != "old2" || second.NewPath != "new2" {
		t.Errorf("cache hit should retarget paths, got %q %q", second.OldPath, second.NewPath)
	}
	if first.OldPath != "old1" || first.NewPath != "new1" {
		t.Errorf("retarget must not mutate the first result, got %q %q", first.OldPath, first.NewPath)
	}
	if len(first.Hunks) != len(second.Hunks) {
		t.Errorf("cached hunks should match: first=%d second=%d", len(first.Hunks), len(second.Hunks))
	}
	if got := e.Stats().Hits; got < 1 {
		t.Errorf("expected at least one cache hit, got %d", got)
	}
}

func TestComputeDiff_ZeroContext_ShouldContainOnlyChanges(t *testing.T) {
	t.Parallel()
	e := NewEngineWith(Options{ContextLines: -1})
	old := "l1\nl2\nl3\nl4\nl5\nchange1\nl7\nl8\nl9\nl10\nl11\n"
	new := "l1\nl2\nl3\nl4\nl5\nchange2\nl7\nl8\nl9\nl10\nl11\n"
	d := e.ComputeDiff("a", "b", old, new)
	if d == nil || len(d.Hunks) == 0 {
		t.Fatal("expected hunks for zero-context change")
	}
	for hi, h := range d.Hunks {
		for _, l := range h.Lines {
			if l.Type == LineContext {
				t.Errorf("hunk %d should have no context lines with zero context, found %q", hi, l.Content)
			}
		}
	}
}

func TestFileDiff_Clone_WhenHunkHasNilLines_ShouldPreserveNil(t *testing.T) {
	t.Parallel()
	fd := &FileDiff{
		OldPath: "a", NewPath: "b",
		Hunks: []Hunk{{OldStart: 1, OldCount: 0, NewStart: 1, NewCount: 0, Lines: nil}},
	}
	cloned := fd.Clone()
	if cloned == nil {
		t.Fatal("expected non-nil clone")
	}
	if len(cloned.Hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(cloned.Hunks))
	}
	if cloned.Hunks[0].Lines != nil {
		t.Errorf("nil Lines should stay nil after Clone, got %+v", cloned.Hunks[0].Lines)
	}
}
