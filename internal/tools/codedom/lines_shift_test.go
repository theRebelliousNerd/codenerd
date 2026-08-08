package codedom

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The defect these cover (F-DOM-3, observed live): the line-mutation tools
// returned "Replaced lines 42-55 with 3 new lines in x.go" and nothing else.
// The tool result is the ONLY feedback an LLM gets between two edits to the
// same file, so the model reused line numbers from the get_elements it ran
// before the first edit — and the second edit landed 11 lines off, duplicating
// declarations instead of replacing them. Every mutation must now report where
// the coordinate system moved.

func writeTempLines(t *testing.T, n int) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.go")

	lines := make([]string, n)
	for i := range lines {
		lines[i] = "line"
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	return path
}

func TestEditLines_ReportsShrinkAndStaleWarning(t *testing.T) {
	path := writeTempLines(t, 20)

	// Replace 5 lines (10-14) with 2 -> file shrinks by 3.
	out, err := executeEditLines(context.Background(), map[string]any{
		"path":        path,
		"start_line":  10,
		"end_line":    14,
		"new_content": "a\nb",
	})
	if err != nil {
		t.Fatalf("edit_lines: %v", err)
	}

	if !strings.Contains(out, "now 17 lines") {
		t.Errorf("result does not report the new total; a model cannot bound its next edit:\n%s", out)
	}
	if !strings.Contains(out, "(-3)") {
		t.Errorf("result does not report the delta:\n%s", out)
	}
	if !strings.Contains(out, "STALE") {
		t.Errorf("result does not warn that earlier line numbers are invalid:\n%s", out)
	}
	if !strings.Contains(out, "subtract 3") {
		t.Errorf("result does not give the correction offset:\n%s", out)
	}
	if !strings.Contains(out, "at or after 10") {
		t.Errorf("result does not say from which line the shift applies:\n%s", out)
	}
}

func TestInsertLines_ReportsGrowth(t *testing.T) {
	path := writeTempLines(t, 20)

	out, err := executeInsertLines(context.Background(), map[string]any{
		"path":       path,
		"after_line": 5,
		"content":    "a\nb\nc\nd",
	})
	if err != nil {
		t.Fatalf("insert_lines: %v", err)
	}

	if !strings.Contains(out, "now 24 lines") || !strings.Contains(out, "(+4)") {
		t.Errorf("result does not report the new total and delta:\n%s", out)
	}
	// Insertion after line 5 means line 6 onward moved; line 5 did not.
	if !strings.Contains(out, "at or after 6") {
		t.Errorf("shift origin is wrong: inserting after line 5 moves line 6, not line 5:\n%s", out)
	}
	if !strings.Contains(out, "add 4") {
		t.Errorf("result does not give the correction offset:\n%s", out)
	}
}

func TestDeleteLines_ReportsShrink(t *testing.T) {
	path := writeTempLines(t, 20)

	out, err := executeDeleteLines(context.Background(), map[string]any{
		"path":       path,
		"start_line": 3,
		"end_line":   7,
	})
	if err != nil {
		t.Fatalf("delete_lines: %v", err)
	}

	if !strings.Contains(out, "now 15 lines") || !strings.Contains(out, "(-5)") {
		t.Errorf("result does not report the new total and delta:\n%s", out)
	}
	if !strings.Contains(out, "at or after 3") || !strings.Contains(out, "subtract 5") {
		t.Errorf("result does not give the shift origin and correction:\n%s", out)
	}
}

// A same-size replacement must NOT cry wolf — a warning on every edit trains
// the model to ignore the warning that matters.
func TestEditLines_SameSizeReplacementReportsNoShift(t *testing.T) {
	path := writeTempLines(t, 20)

	out, err := executeEditLines(context.Background(), map[string]any{
		"path":        path,
		"start_line":  4,
		"end_line":    6,
		"new_content": "a\nb\nc",
	})
	if err != nil {
		t.Fatalf("edit_lines: %v", err)
	}

	if strings.Contains(out, "STALE") {
		t.Errorf("a same-size replacement shifts nothing but warned anyway:\n%s", out)
	}
	if !strings.Contains(out, "still 20 lines") {
		t.Errorf("result should confirm line numbers are still valid:\n%s", out)
	}
}

// The reported total must match the file actually on disk, or the correction
// arithmetic sends the next edit somewhere worse than it started.
func TestLineShiftNotice_TotalMatchesDisk(t *testing.T) {
	path := writeTempLines(t, 30)

	if _, err := executeDeleteLines(context.Background(), map[string]any{
		"path":       path,
		"start_line": 10,
		"end_line":   19,
	}); err != nil {
		t.Fatalf("delete_lines: %v", err)
	}

	out, err := executeInsertLines(context.Background(), map[string]any{
		"path":       path,
		"after_line": 1,
		"content":    "x",
	})
	if err != nil {
		t.Fatalf("insert_lines: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	actual := len(strings.Split(string(content), "\n"))

	if !strings.Contains(out, "now 21 lines") {
		t.Errorf("reported total disagrees with disk (%d lines):\n%s", actual, out)
	}
	if actual != 21 {
		t.Errorf("file has %d lines, expected 21", actual)
	}
}
