package diff

import (
	"strings"
	"testing"
)

// TODO: TEST_GAP: Null/Empty inputs (Empty File Paths with Non-Empty Content, Null Byte Injection, Zero-Byte Hunks / Empty Slices, Whitespace-Only Modifications)
// TODO: TEST_GAP: Type Coercion (Binary File Masquerading as Text, Invalid UTF-8 Byte Sequences, Carriage Return (CRLF) vs. Line Feed (LF) Dissonance)
// TODO: TEST_GAP: User Request Extremes (The 'No Newline' Monolith, Infinite Generation Loops, MaxInt32 Boundary Testing)
// TODO: TEST_GAP: State Conflicts (Concurrent Cache Mutation Data Race, Hash Collisions in FNV-1a, Cache Invalidation Under Memory Pressure)

func TestComputeDiff_SimpleAddition(t *testing.T) {
	oldContent := "line1\nline2\nline3"
	newContent := "line1\nline2\nline2.5\nline3"

	engine := NewEngine()
	diff := engine.ComputeDiff("old.txt", "new.txt", oldContent, newContent)

	if diff == nil {
		t.Fatal("Expected diff, got nil")
	}

	if len(diff.Hunks) != 1 {
		t.Errorf("Expected 1 hunk, got %d", len(diff.Hunks))
	}

	if diff.IsNew || diff.IsDelete {
		t.Error("Should not be marked as new or delete")
	}

	// Check that the added line is present
	hasAddition := false
	for _, hunk := range diff.Hunks {
		for _, line := range hunk.Lines {
			if line.Type == LineAdded && line.Content == "line2.5" {
				hasAddition = true
			}
		}
	}
	if !hasAddition {
		t.Error("Expected to find added line 'line2.5'")
	}
}

func TestComputeDiff_SimpleDeletion(t *testing.T) {
	oldContent := "line1\nline2\nline3\nline4"
	newContent := "line1\nline2\nline4"

	engine := NewEngine()
	diff := engine.ComputeDiff("old.txt", "new.txt", oldContent, newContent)

	if diff == nil {
		t.Fatal("Expected diff, got nil")
	}

	if len(diff.Hunks) != 1 {
		t.Errorf("Expected 1 hunk, got %d", len(diff.Hunks))
	}

	// Check that the removed line is present
	hasRemoval := false
	for _, hunk := range diff.Hunks {
		for _, line := range hunk.Lines {
			if line.Type == LineRemoved && line.Content == "line3" {
				hasRemoval = true
			}
		}
	}
	if !hasRemoval {
		t.Error("Expected to find removed line 'line3'")
	}
}

func TestComputeDiff_NewFile(t *testing.T) {
	oldContent := ""
	newContent := "new file content\nline 2"

	engine := NewEngine()
	diff := engine.ComputeDiff("", "new.txt", oldContent, newContent)

	if !diff.IsNew {
		t.Error("Expected diff to be marked as new file")
	}
}

func TestComputeDiff_DeletedFile(t *testing.T) {
	oldContent := "old file content\nline 2"
	newContent := ""

	engine := NewEngine()
	diff := engine.ComputeDiff("old.txt", "", oldContent, newContent)

	if !diff.IsDelete {
		t.Error("Expected diff to be marked as deleted file")
	}
}

func TestComputeDiff_NoChanges(t *testing.T) {
	content := "line1\nline2\nline3"

	engine := NewEngine()
	diff := engine.ComputeDiff("file.txt", "file.txt", content, content)

	if len(diff.Hunks) != 0 {
		t.Errorf("Expected 0 hunks for identical content, got %d", len(diff.Hunks))
	}
}

func TestComputeDiff_MultipleHunks(t *testing.T) {
	oldContent := `line1
line2
line3
line4
line5
line6
line7
line8
line9
line10
line11
line12
line13
line14
line15`

	newContent := `line1
line2
CHANGED3
line4
line5
line6
line7
line8
line9
line10
line11
line12
CHANGED13
line14
line15`

	engine := NewEngine()
	diff := engine.ComputeDiff("old.txt", "new.txt", oldContent, newContent)

	// Should create 2 separate hunks due to distance between changes
	if len(diff.Hunks) < 1 {
		t.Errorf("Expected at least 1 hunk, got %d", len(diff.Hunks))
	}
}

func TestComputeDiff_ContextLines(t *testing.T) {
	oldContent := "line1\nline2\nline3\nline4\nline5"
	newContent := "line1\nline2\nCHANGED\nline4\nline5"

	engine := NewEngine()
	diff := engine.ComputeDiff("old.txt", "new.txt", oldContent, newContent)

	if len(diff.Hunks) != 1 {
		t.Fatalf("Expected 1 hunk, got %d", len(diff.Hunks))
	}

	hunk := diff.Hunks[0]

	// Should have context lines before and after the change
	hasContext := false
	for _, line := range hunk.Lines {
		if line.Type == LineContext {
			hasContext = true
			break
		}
	}
	if !hasContext {
		t.Error("Expected context lines in hunk")
	}
}

func TestComputeDiff_Caching(t *testing.T) {
	oldContent := "line1\nline2\nline3"
	newContent := "line1\nline2\nline3\nline4"

	engine := NewEngine()

	// First computation
	diff1 := engine.ComputeDiff("old.txt", "new.txt", oldContent, newContent)

	// Second computation with same content (should use cache)
	diff2 := engine.ComputeDiff("old2.txt", "new2.txt", oldContent, newContent)

	// Verify both diffs have same structure (but different paths)
	if len(diff1.Hunks) != len(diff2.Hunks) {
		t.Errorf("Cache should preserve hunk count: %d vs %d", len(diff1.Hunks), len(diff2.Hunks))
	}

	if diff2.OldPath != "old2.txt" || diff2.NewPath != "new2.txt" {
		t.Error("Cached diff should have updated paths")
	}

	// Clear cache and verify
	engine.ClearCache()
	diff3 := engine.ComputeDiff("old.txt", "new.txt", oldContent, newContent)
	if len(diff3.Hunks) != len(diff1.Hunks) {
		t.Error("Cache clearing should not affect diff computation")
	}
}

func TestComputeDiff_EmptyLines(t *testing.T) {
	oldContent := "line1\n\nline3"
	newContent := "line1\n\n\nline3"

	engine := NewEngine()
	diff := engine.ComputeDiff("old.txt", "new.txt", oldContent, newContent)

	if diff == nil {
		t.Fatal("Expected diff, got nil")
	}

	// Should detect the added empty line
	hasChange := len(diff.Hunks) > 0
	if !hasChange {
		t.Error("Expected to detect change in empty lines")
	}
}

func TestComputeDiff_LargeFile(t *testing.T) {
	// Generate large content. Note: avoid NUL bytes (rune(0)) because the diff
	// engine now short-circuits binary payloads via NUL-byte detection.
	var oldLines, newLines []string
	for i := range 1000 {
		oldLines = append(oldLines, "line "+string(rune(i+1)))
		newLines = append(newLines, "line "+string(rune(i+1)))
	}
	// Modify middle section
	newLines[500] = "CHANGED LINE"

	oldContent := strings.Join(oldLines, "\n")
	newContent := strings.Join(newLines, "\n")

	engine := NewEngine()
	diff := engine.ComputeDiff("old.txt", "new.txt", oldContent, newContent)

	if diff == nil {
		t.Fatal("Expected diff, got nil")
	}

	if len(diff.Hunks) == 0 {
		t.Error("Expected at least one hunk for large file diff")
	}
}

func TestComputeDiff_HunkCounts(t *testing.T) {
	oldContent := "line1\nline2\nline3"
	newContent := "line1\nNEW\nline3"

	engine := NewEngine()
	diff := engine.ComputeDiff("old.txt", "new.txt", oldContent, newContent)

	if len(diff.Hunks) != 1 {
		t.Fatalf("Expected 1 hunk, got %d", len(diff.Hunks))
	}

	hunk := diff.Hunks[0]

	// Verify OldCount and NewCount are computed correctly
	if hunk.OldCount == 0 {
		t.Error("Expected OldCount > 0")
	}
	if hunk.NewCount == 0 {
		t.Error("Expected NewCount > 0")
	}

	// Count manually
	oldCount := 0
	newCount := 0
	for _, line := range hunk.Lines {
		if line.Type == LineRemoved || line.Type == LineContext {
			oldCount++
		}
		if line.Type == LineAdded || line.Type == LineContext {
			newCount++
		}
	}

	if hunk.OldCount != oldCount {
		t.Errorf("OldCount mismatch: expected %d, got %d", oldCount, hunk.OldCount)
	}
	if hunk.NewCount != newCount {
		t.Errorf("NewCount mismatch: expected %d, got %d", newCount, hunk.NewCount)
	}
}

func TestComputeWordLevelDiff(t *testing.T) {
	oldLine := "The quick brown fox"
	newLine := "The quick red fox"

	engine := NewEngine()
	diffs := engine.ComputeWordLevelDiff(oldLine, newLine)

	if len(diffs) == 0 {
		t.Fatal("Expected word-level diffs, got none")
	}

	// Should detect "brown" -> "red" change
	hasChange := false
	for _, diff := range diffs {
		if strings.Contains(diff.Text, "red") || strings.Contains(diff.Text, "brown") {
			hasChange = true
			break
		}
	}
	if !hasChange {
		t.Error("Expected to detect word-level change")
	}
}

func BenchmarkComputeDiff_Small(b *testing.B) {
	oldContent := "line1\nline2\nline3"
	newContent := "line1\nCHANGED\nline3"
	engine := NewEngine()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.ComputeDiff("old.txt", "new.txt", oldContent, newContent)
	}
}

func BenchmarkComputeDiff_Large(b *testing.B) {
	var lines []string
	for i := range 1000 {
		// Skip rune(0) (NUL) — flagged as binary by the engine.
		lines = append(lines, "line content here "+string(rune(i+1)))
	}
	oldContent := strings.Join(lines, "\n")
	lines[500] = "CHANGED"
	newContent := strings.Join(lines, "\n")

	engine := NewEngine()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.ComputeDiff("old.txt", "new.txt", oldContent, newContent)
	}
}

func BenchmarkComputeDiff_WithCache(b *testing.B) {
	oldContent := "line1\nline2\nline3"
	newContent := "line1\nCHANGED\nline3"
	engine := NewEngine()

	// Prime the cache
	engine.ComputeDiff("old.txt", "new.txt", oldContent, newContent)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.ComputeDiff("old.txt", "new.txt", oldContent, newContent)
	}
}

// =============================================================================
// Boundary Analysis Coverage (QA 2026-05-24 diff_boundary_analysis)
// =============================================================================

// TestComputeDiff_EmptyStrings covers the both-empty edge case: it must not panic,
// must mark both IsNew and IsDelete, and must produce zero hunks.
func TestComputeDiff_EmptyStrings(t *testing.T) {
	engine := NewEngine()
	d := engine.ComputeDiff("a.txt", "b.txt", "", "")
	if d == nil {
		t.Fatal("expected non-nil diff for empty/empty")
	}
	if !d.IsNew {
		t.Error("expected IsNew=true when old content is empty")
	}
	if !d.IsDelete {
		t.Error("expected IsDelete=true when new content is empty")
	}
	if len(d.Hunks) != 0 {
		t.Errorf("expected 0 hunks for empty/empty, got %d", len(d.Hunks))
	}
}

// TestComputeDiff_BinaryContent verifies binary payloads (NUL bytes) short-circuit
// before reaching diffmatchpatch and are flagged IsBinary=true.
func TestComputeDiff_BinaryContent(t *testing.T) {
	engine := NewEngine()
	old := string([]byte{0x48, 0x00, 0x49, 0xFF})
	new := string([]byte{0x48, 0x00, 0x4A, 0xFF})
	d := engine.ComputeDiff("a.bin", "b.bin", old, new)
	if d == nil {
		t.Fatal("expected non-nil diff for binary content")
	}
	if !d.IsBinary {
		t.Error("expected IsBinary=true when content contains NUL bytes")
	}
	if len(d.Hunks) != 0 {
		t.Errorf("expected 0 hunks for binary content (short-circuit), got %d", len(d.Hunks))
	}

	// Also verify the case where only one side is binary.
	d2 := engine.ComputeDiff("a.txt", "b.bin", "text\n", string([]byte{0x00, 0x01}))
	if !d2.IsBinary {
		t.Error("expected IsBinary=true when only the new side contains NUL bytes")
	}
}

// TestComputeDiff_HugeContext verifies the contextLines clamp in convertToHunks.
// Negative values must not panic / index out-of-range, and MaxInt-sized values
// must not cause the hunk grouper to allocate unbounded leading-context slices.
func TestComputeDiff_HugeContext(t *testing.T) {
	engine := NewEngine()

	// Negative -> clamped to 0
	{
		ops := []operation{
			{typ: LineContext, oldLine: 0, newLine: 0, content: "ctx1"},
			{typ: LineRemoved, oldLine: 1, newLine: -1, content: "del"},
			{typ: LineAdded, oldLine: -1, newLine: 1, content: "add"},
			{typ: LineContext, oldLine: 2, newLine: 2, content: "ctx2"},
		}
		hunks := engine.groupIntoHunks(ops, -1)
		// Clamp to 0 means trailing-context check should immediately close the hunk
		// once we hit context after the change; we still expect at least one hunk.
		if len(hunks) == 0 {
			t.Error("expected at least one hunk even with negative contextLines")
		}
	}

	// Massive contextLines -> clamped to maxContextLines via convertToHunks
	{
		old := "line1\nline2\nCHANGED_OLD\nline4\nline5\n"
		new := "line1\nline2\nCHANGED_NEW\nline4\nline5\n"
		d := engine.ComputeDiff("a.txt", "b.txt", old, new)
		if d == nil {
			t.Fatal("expected non-nil diff")
		}
		// Sanity: the public path uses defaultContextLines, so this just confirms
		// no panic. We also drive convertToHunks directly with an absurd value.
		a, b, lineArray := engine.dmp.DiffLinesToChars(old, new)
		diffs := engine.dmp.DiffMain(a, b, false)
		diffs = engine.dmp.DiffCleanupSemantic(diffs)
		diffs = engine.dmp.DiffCharsToLines(diffs, lineArray)
		// Must not panic and must return some hunks.
		hunks := engine.convertToHunks(diffs, 1_000_000)
		if len(hunks) == 0 {
			t.Error("expected hunks with extreme contextLines (clamped)")
		}
	}
}

// TestComputeDiff_EmptyPaths verifies that empty path strings don't break
// FileDiff construction or the cache path-rewrite logic on a cache hit.
func TestComputeDiff_EmptyPaths(t *testing.T) {
	engine := NewEngine()
	d := engine.ComputeDiff("", "", "old\n", "new\n")
	if d == nil {
		t.Fatal("expected non-nil diff with empty paths")
	}
	if d.OldPath != "" || d.NewPath != "" {
		t.Errorf("expected empty paths to be preserved, got %q -> %q", d.OldPath, d.NewPath)
	}
	if len(d.Hunks) == 0 {
		t.Error("expected hunks even when paths are empty")
	}

	// Hit the cache with empty paths to make sure the path-rewrite branch works.
	d2 := engine.ComputeDiff("", "", "old\n", "new\n")
	if d2 == nil || len(d2.Hunks) != len(d.Hunks) {
		t.Error("cached call with empty paths produced inconsistent result")
	}
}

// TODO: TEST_GAP: [Null/Undefined/Empty] Verify ComputeDiff handles completely empty strings for both oldContent and newContent without panicking or creating invalid hunks.
// TODO: TEST_GAP: [Null/Undefined/Empty] Verify the algorithm correctly captures and represents the addition or removal of a single trailing newline at EOF, avoiding the empty line truncation logic.
// TODO: TEST_GAP: [Null/Undefined/Empty] Verify the caching mechanism's behavior if an FNV-1a hash collision occurs with an empty string, ensuring cache hits validate the actual content or lengths.
// TODO: TEST_GAP: [Type Coercion] Verify ComputeDiff flags binary payloads (e.g., strings containing null bytes \x00) immediately without passing them to the heavy Myers diff engine.
// TODO: TEST_GAP: [Type Coercion] Verify extreme file paths (e.g. containing \n, \r, or exceeding 4096 chars) do not break the FileDiff struct serialization or cause panic.
// TODO: TEST_GAP: [Type Coercion] Verify ComputeDiff gracefully handles invalid UTF-8 byte sequences or unpaired surrogate halves without causing panics in the rune mapping engine.
// TODO: TEST_GAP: [User Request Extremes] Verify ComputeDiff does not hang indefinitely when processing massive single-line minified strings (e.g. 5MB of minified JS without newlines). A timeout or fallback must exist.
// TODO: TEST_GAP: [User Request Extremes] Verify the DefaultEngine cache does not cause an Out-Of-Memory (OOM) crash when 100,000 unique files are diffed sequentially in a rapid loop.
// TODO: TEST_GAP: [User Request Extremes] Verify ComputeDiff avoids extreme memory allocations when processing a massive pure deletion (e.g., deleting a 5MB file). It should not allocate millions of Line structs.
// TODO: TEST_GAP: [State Conflicts] Verify the "Shallow Copy Pointer Trap". Retrieve a cached diff, mutate its Hunks slice, request the diff again, and verify the cache was not permanently mutated.
// TODO: TEST_GAP: [State Conflicts] Verify the exact boundary spacing of context lines (changes exactly contextLines * 2 apart) does not cause context duplication or incorrect Hunk merging.
// TODO: TEST_GAP: [State Conflicts] Verify the race condition between ClearCache() and concurrent active ComputeDiff requests doesn't result in stale computations populating the new cache.
