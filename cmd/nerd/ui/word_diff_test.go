package ui

import (
	"codenerd/internal/diff"
	"strings"
	"testing"
)

func TestWordLevelDiff_Toggle(t *testing.T) {
	styles := DefaultStyles()
	view := NewDiffApprovalView(styles, 100, 50)

	// Default should be enabled
	if !view.WordLevelDiff {
		t.Error("Expected word-level diff to be enabled by default")
	}

	// Toggle off
	view.ToggleWordLevelDiff()
	if view.WordLevelDiff {
		t.Error("Expected word-level diff to be disabled after toggle")
	}

	// Toggle back on
	view.ToggleWordLevelDiff()
	if !view.WordLevelDiff {
		t.Error("Expected word-level diff to be enabled after second toggle")
	}
}

func TestWordLevelDiff_BasicRendering(t *testing.T) {
	styles := DefaultStyles()
	view := NewDiffApprovalView(styles, 100, 50)

	// Create test diff
	oldContent := "The quick brown fox"
	newContent := "The quick red fox"

	fileDiff := diff.ComputeDiff("old.txt", "new.txt", oldContent, newContent)

	mutation := &PendingMutation{
		ID:          "test1",
		Description: "Test word diff",
		FilePath:    "test.txt",
		Diff:        fileDiff,
	}

	view.AddMutation(mutation)

	// Get rendered content
	rendered := view.View()

	// Should contain both lines
	if !strings.Contains(rendered, "quick") {
		t.Error("Expected rendered content to contain 'quick'")
	}
}

func TestWordLevelDiff_MultiplePairs(t *testing.T) {
	styles := DefaultStyles()
	view := NewDiffApprovalView(styles, 100, 50)

	oldContent := "line 1\nline 2 old\nline 3"
	newContent := "line 1\nline 2 new\nline 3"

	fileDiff := diff.ComputeDiff("old.txt", "new.txt", oldContent, newContent)

	mutation := &PendingMutation{
		ID:          "test2",
		Description: "Test multiple changes",
		FilePath:    "test.txt",
		Diff:        fileDiff,
	}

	view.AddMutation(mutation)

	// Should handle multiple change pairs
	if view.CurrentIndex != 0 {
		t.Errorf("Expected current index to be 0, got %d", view.CurrentIndex)
	}
}

func TestWordLevelDiff_DisabledRendering(t *testing.T) {
	styles := DefaultStyles()
	view := NewDiffApprovalView(styles, 100, 50)

	// Disable word-level diff
	view.WordLevelDiff = false

	oldContent := "The quick brown fox"
	newContent := "The quick red fox"

	fileDiff := diff.ComputeDiff("old.txt", "new.txt", oldContent, newContent)

	mutation := &PendingMutation{
		ID:          "test3",
		Description: "Test disabled word diff",
		FilePath:    "test.txt",
		Diff:        fileDiff,
	}

	view.AddMutation(mutation)

	// Should still render, just without word-level highlighting
	rendered := view.View()
	if rendered == "" {
		t.Error("Expected non-empty rendered content")
	}
}

func TestWordLevelDiff_EmptyLines(t *testing.T) {
	styles := DefaultStyles()
	view := NewDiffApprovalView(styles, 100, 50)

	oldContent := ""
	newContent := "new line"

	fileDiff := diff.ComputeDiff("old.txt", "new.txt", oldContent, newContent)

	mutation := &PendingMutation{
		ID:          "test4",
		Description: "Test empty old content",
		FilePath:    "test.txt",
		Diff:        fileDiff,
	}

	view.AddMutation(mutation)

	// Should handle empty lines gracefully
	rendered := view.View()
	if !strings.Contains(rendered, "new line") {
		t.Error("Expected rendered content to contain 'new line'")
	}
}

func TestWordLevelDiff_NoChanges(t *testing.T) {
	styles := DefaultStyles()
	view := NewDiffApprovalView(styles, 100, 50)

	content := "unchanged content"

	fileDiff := diff.ComputeDiff("old.txt", "new.txt", content, content)

	mutation := &PendingMutation{
		ID:          "test5",
		Description: "Test no changes",
		FilePath:    "test.txt",
		Diff:        fileDiff,
	}

	view.AddMutation(mutation)

	// Should handle no-change case
	if fileDiff == nil {
		t.Error("Expected non-nil diff for identical content")
	}
}

func TestRenderWordDiffPair(t *testing.T) {
	styles := DefaultStyles()
	view := NewDiffApprovalView(styles, 100, 50)

	removedLine := diff.Line{
		LineNum: 1,
		Content: "The quick brown fox",
		Type:    diff.LineRemoved,
	}

	addedLine := diff.Line{
		LineNum: 1,
		Content: "The quick red fox",
		Type:    diff.LineAdded,
	}

	result := view.renderWordDiffPair(removedLine, addedLine)

	// Should contain both lines
	if !strings.Contains(result, "quick") {
		t.Error("Expected result to contain 'quick'")
	}

	// Should have prefix indicators
	if !strings.Contains(result, "-") && !strings.Contains(result, "+") {
		t.Error("Expected result to contain diff prefixes")
	}
}

func TestRenderHunkLines_WithWordDiff(t *testing.T) {
	styles := DefaultStyles()
	view := NewDiffApprovalView(styles, 100, 50)
	view.WordLevelDiff = true

	lines := []diff.Line{
		{LineNum: 1, Content: "context line", Type: diff.LineContext},
		{LineNum: 2, Content: "old value", Type: diff.LineRemoved},
		{LineNum: 2, Content: "new value", Type: diff.LineAdded},
		{LineNum: 3, Content: "another context", Type: diff.LineContext},
	}

	result := view.renderHunkLines(lines)

	// Should contain all lines
	if !strings.Contains(result, "context line") {
		t.Error("Expected result to contain context line")
	}

	// Should process the removed/added pair
	if !strings.Contains(result, "value") {
		t.Error("Expected result to contain 'value'")
	}
}

func TestRenderHunkLines_WithoutWordDiff(t *testing.T) {
	styles := DefaultStyles()
	view := NewDiffApprovalView(styles, 100, 50)
	view.WordLevelDiff = false

	lines := []diff.Line{
		{LineNum: 1, Content: "old value", Type: diff.LineRemoved},
		{LineNum: 1, Content: "new value", Type: diff.LineAdded},
	}

	result := view.renderHunkLines(lines)

	// Should still render both lines separately
	if result == "" {
		t.Error("Expected non-empty result")
	}
}

func TestRenderDiffLine_WithoutWordDiffs(t *testing.T) {
	styles := DefaultStyles()
	view := NewDiffApprovalView(styles, 100, 50)

	line := diff.Line{
		LineNum: 1,
		Content: "test content",
		Type:    diff.LineAdded,
	}

	result := view.renderDiffLine(line, nil)

	if !strings.Contains(result, "test content") {
		t.Error("Expected result to contain line content")
	}
}

func TestRenderLineWithWordHighlights_Removed(t *testing.T) {
	styles := DefaultStyles()
	view := NewDiffApprovalView(styles, 100, 50)

	line := diff.Line{
		LineNum: 1,
		Content: "removed text",
		Type:    diff.LineRemoved,
	}

	result := view.renderLineWithWordHighlights(line, nil, true)

	if !strings.Contains(result, "removed text") {
		t.Error("Expected result to contain line content")
	}
}

func TestRenderLineWithWordHighlights_Added(t *testing.T) {
	styles := DefaultStyles()
	view := NewDiffApprovalView(styles, 100, 50)

	line := diff.Line{
		LineNum: 1,
		Content: "added text",
		Type:    diff.LineAdded,
	}

	result := view.renderLineWithWordHighlights(line, nil, false)

	if !strings.Contains(result, "added text") {
		t.Error("Expected result to contain line content")
	}
}

func BenchmarkWordLevelDiff_Rendering(b *testing.B) {
	styles := DefaultStyles()
	view := NewDiffApprovalView(styles, 100, 50)

	oldContent := strings.Repeat("The quick brown fox jumps over the lazy dog\n", 100)
	newContent := strings.Repeat("The quick red fox jumps over the lazy cat\n", 100)

	fileDiff := diff.ComputeDiff("old.txt", "new.txt", oldContent, newContent)

	mutation := &PendingMutation{
		ID:          "bench",
		Description: "Benchmark test",
		FilePath:    "test.txt",
		Diff:        fileDiff,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		view.ClearMutations()
		view.AddMutation(mutation)
		_ = view.View()
	}
}

func BenchmarkWordLevelDiff_Disabled(b *testing.B) {
	styles := DefaultStyles()
	view := NewDiffApprovalView(styles, 100, 50)
	view.WordLevelDiff = false

	oldContent := strings.Repeat("The quick brown fox jumps over the lazy dog\n", 100)
	newContent := strings.Repeat("The quick red fox jumps over the lazy cat\n", 100)

	fileDiff := diff.ComputeDiff("old.txt", "new.txt", oldContent, newContent)

	mutation := &PendingMutation{
		ID:          "bench",
		Description: "Benchmark test",
		FilePath:    "test.txt",
		Diff:        fileDiff,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		view.ClearMutations()
		view.AddMutation(mutation)
		_ = view.View()
	}
}
