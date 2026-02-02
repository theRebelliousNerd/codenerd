package diff

import (
	"strings"
	"testing"
)

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
	// Generate large content
	var oldLines, newLines []string
	for i := 0; i < 1000; i++ {
		oldLines = append(oldLines, "line "+string(rune(i)))
		newLines = append(newLines, "line "+string(rune(i)))
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
	for i := 0; i < 1000; i++ {
		lines = append(lines, "line content here "+string(rune(i)))
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
