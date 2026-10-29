package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

// Fractional line bounds are refused, not truncated: start_line 1.5 silently
// becoming line 1 would read a region the caller did not ask for. Whole-number
// floats (how JSON decodes integers) keep working.
func TestReadFile_FractionalBoundsRefused(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODENERD_WORKSPACE_ROOT", dir)
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if _, err := executeReadFile(ctx, map[string]any{"path": "f.txt", "start_line": 1.5}); err == nil {
		t.Error("start_line 1.5 must be refused")
	} else if !strings.Contains(err.Error(), "integral") {
		t.Errorf("error %q must say integral", err)
	}
	if _, err := executeReadFile(ctx, map[string]any{"path": "f.txt", "end_line": 2.5}); err == nil {
		t.Error("end_line 2.5 must be refused")
	}
	out, err := executeReadFile(ctx, map[string]any{"path": "f.txt", "start_line": 2.0, "end_line": 2.0})
	if err != nil {
		t.Fatalf("whole-number floats must work: %v", err)
	}
	if !strings.Contains(out, "two") || strings.Contains(out, "three") {
		t.Errorf("bounded read returned wrong region: %q", out)
	}
}

// A minified single-line file trips bufio's 64KB default buffer and used to be
// silently skipped as a read error; matches past the old limit must surface.
func TestGrep_LongLineMatches(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODENERD_WORKSPACE_ROOT", dir)
	line := strings.Repeat("x", 100*1024) + "NEEDLE" + strings.Repeat("y", 100*1024) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "min.js"), []byte(line), 0644); err != nil {
		t.Fatal(err)
	}
	out, err := executeGrep(context.Background(), map[string]any{"pattern": "NEEDLE", "path": "."})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if !strings.Contains(out, "min.js") || !strings.Contains(out, "NEEDLE") {
		t.Errorf("grep missed the long-line match: %q", out)
	}
}

// Page addresses read through the canonical strict helper: a Mangle-sourced
// int64 offset is honored (the old local switch rejected it outright with
// "must be integral"), while a fractional offset is still refused.
func TestRecallContext_Int64OffsetHonoredFractionalRefused(t *testing.T) {
	recall := &fakeRecall{}
	ctx := tools.WithContextRecall(context.Background(), recall)
	tool := RecallContextTool()

	out, err := tool.Execute(ctx, map[string]any{"id": "abc", "offset": int64(7)})
	if err != nil {
		t.Fatalf("int64 offset must be honored: %v", err)
	}
	if !strings.Contains(out, "recall abc 7") {
		t.Errorf("offset not honored, got %q", out)
	}
	if _, err := tool.Execute(ctx, map[string]any{"id": "abc", "offset": 1.5}); err == nil {
		t.Error("fractional offset must be refused")
	} else if !strings.Contains(err.Error(), "must be integral") {
		t.Errorf("error %q must say must be integral", err)
	}
}
