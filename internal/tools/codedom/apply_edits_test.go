package codedom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

func ctxForDir(dir string) context.Context {
	return context.WithValue(context.Background(), tools.CtxKeyWorkspaceRoot, dir)
}

func writeFixture(t *testing.T, dir, rel, content string) string {
	t.Helper()
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	return abs
}

func readFile(t *testing.T, abs string) string {
	t.Helper()
	b, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read %s: %v", abs, err)
	}
	return string(b)
}

func TestApplyEditsTool_Definition(t *testing.T) {
	t.Parallel()
	tool := ApplyEditsTool()
	if tool.Name != "apply_edits" {
		t.Fatalf("Name = %q, want apply_edits", tool.Name)
	}
	if tool.Execute == nil {
		t.Fatal("Execute nil")
	}
	if len(tool.Schema.Required) != 1 || tool.Schema.Required[0] != "edits" {
		t.Fatalf("schema required = %v, want [edits]", tool.Schema.Required)
	}
	prop, ok := tool.Schema.Properties["edits"]
	if !ok {
		t.Fatal("edits property missing")
	}
	if prop.Type != "array" {
		t.Fatalf("edits type = %q, want array", prop.Type)
	}
}

func TestApplyEdits_TwoFileSuccess(t *testing.T) {
	dir := t.TempDir()
	aRel := "a.txt"
	bRel := "sub/b.txt"
	aAbs := writeFixture(t, dir, aRel, "one\ntwo\nthree\n")
	bAbs := writeFixture(t, dir, bRel, "alpha\nbeta\ngamma\n")
	ctx := ctxForDir(dir)
	res, err := executeApplyEdits(ctx, map[string]any{
		"edits": []any{
			map[string]any{"operation": "edit_lines", "path": aRel, "start_line": 2, "end_line": 2, "new_content": "TWO"},
			map[string]any{"operation": "insert_lines", "path": bRel, "after_line": 1, "content": "INSERTED"},
		},
	})
	if err != nil {
		t.Fatalf("apply_edits failed: %v", err)
	}
	// Verify JSON compact
	var out map[string]any
	if err := json.Unmarshal([]byte(res), &out); err != nil {
		t.Fatalf("result not JSON: %v; raw=%s", err, res)
	}
	if strings.Contains(res, "\n  ") {
		t.Errorf("result not compact JSON: %q", res)
	}
	changed, _ := out["changed"].([]any)
	if len(changed) != 2 {
		t.Fatalf("changed len = %v, want 2, raw %s", changed, res)
	}
	// Verify files mutated
	aGot := readFile(t, aAbs)
	if !strings.Contains(aGot, "TWO") || strings.Contains(aGot, "\ntwo\n") {
		t.Errorf("a.txt not edited correctly: %q", aGot)
	}
	bGot := readFile(t, bAbs)
	if !strings.Contains(bGot, "INSERTED") {
		t.Errorf("b.txt not inserted correctly: %q", bGot)
	}
	lines := strings.Split(strings.TrimSpace(bGot), "\n")
	if len(lines) != 4 {
		t.Errorf("b.txt lines = %d, want 4, got %q", len(lines), bGot)
	}
}

func TestApplyEdits_PreflightFailureChangesNeitherFile(t *testing.T) {
	dir := t.TempDir()
	aRel := "a.txt"
	bRel := "b.txt"
	aAbs := writeFixture(t, dir, aRel, "one\ntwo\nthree\n")
	bAbs := writeFixture(t, dir, bRel, "alpha\nbeta\n")
	aBefore := readFile(t, aAbs)
	bBefore := readFile(t, bAbs)
	ctx := ctxForDir(dir)
	// Second edit out of range -> preflight should fail before any commit
	_, err := executeApplyEdits(ctx, map[string]any{
		"edits": []any{
			map[string]any{"operation": "edit_lines", "path": aRel, "start_line": 1, "end_line": 1, "new_content": "ONE"},
			map[string]any{"operation": "delete_lines", "path": bRel, "start_line": 99, "end_line": 100},
		},
	})
	if err == nil {
		t.Fatal("expected preflight error")
	}
	aAfter := readFile(t, aAbs)
	bAfter := readFile(t, bAbs)
	if aAfter != aBefore {
		t.Errorf("preflight failure mutated a.txt: before %q after %q", aBefore, aAfter)
	}
	if bAfter != bBefore {
		t.Errorf("preflight failure mutated b.txt: before %q after %q", bBefore, bAfter)
	}
}

func TestApplyEdits_PreflightDelimiterBalanceChangesNeitherFile(t *testing.T) {
	dir := t.TempDir()
	// Go file where second edit drops a brace, triggering delimiter balance check in staging
	goRel := "demo.go"
	txtRel := "notes.txt"
	goSrc := "package demo\n\ntype Config struct {\n\tA string\n\tB string\n}\n\nfunc Helper() {}\n"
	goAbs := writeFixture(t, dir, goRel, goSrc)
	txtAbs := writeFixture(t, dir, txtRel, "one\ntwo\nthree\n")
	goBefore := readFile(t, goAbs)
	txtBefore := readFile(t, txtAbs)
	ctx := ctxForDir(dir)
	_, err := executeApplyEdits(ctx, map[string]any{
		"edits": []any{
			map[string]any{"operation": "edit_lines", "path": txtRel, "start_line": 1, "end_line": 1, "new_content": "ONE"},
			map[string]any{"operation": "edit_lines", "path": goRel, "start_line": 3, "end_line": 6, "new_content": "type Config struct {\n\tA string\n\tB string\n\tC string"},
		},
	})
	if err == nil {
		t.Fatal("expected delimiter balance preflight failure")
	}
	if !strings.Contains(err.Error(), "delimiter balance") {
		t.Fatalf("wrong error: %v", err)
	}
	if readFile(t, goAbs) != goBefore {
		t.Error("go file mutated despite preflight failure")
	}
	if readFile(t, txtAbs) != txtBefore {
		t.Error("txt file mutated despite preflight failure")
	}
}

func TestApplyEdits_DuplicateCanonicalRejected(t *testing.T) {
	dir := t.TempDir()
	aRel := "a.txt"
	writeFixture(t, dir, aRel, "one\ntwo\n")
	ctx := ctxForDir(dir)
	_, err := executeApplyEdits(ctx, map[string]any{
		"edits": []any{
			map[string]any{"operation": "edit_lines", "path": aRel, "start_line": 1, "end_line": 1, "new_content": "ONE"},
			map[string]any{"operation": "edit_lines", "path": aRel, "start_line": 2, "end_line": 2, "new_content": "TWO"},
		},
	})
	if err == nil {
		t.Fatal("expected duplicate error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		t.Fatalf("expected duplicate mention, got %v", err)
	}
}

func TestApplyEdits_DuplicateViaSymlink(t *testing.T) {
	dir := t.TempDir()
	aRel := "a.txt"
	aAbs := writeFixture(t, dir, aRel, "one\ntwo\n")
	linkRel := "link.txt"
	linkAbs := filepath.Join(dir, linkRel)
	if err := os.Symlink(aAbs, linkAbs); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	ctx := ctxForDir(dir)
	_, err := executeApplyEdits(ctx, map[string]any{
		"edits": []any{
			map[string]any{"operation": "edit_lines", "path": aRel, "start_line": 1, "end_line": 1, "new_content": "ONE"},
			map[string]any{"operation": "edit_lines", "path": linkRel, "start_line": 1, "end_line": 1, "new_content": "ONE2"},
		},
	})
	if err == nil {
		t.Fatal("expected duplicate via symlink")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		t.Fatalf("expected duplicate, got %v", err)
	}
	_ = aAbs
}

func TestApplyEdits_SymlinkEscapeRejected(t *testing.T) {
	dir := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkRel := "evil.txt"
	linkAbs := filepath.Join(dir, linkRel)
	if err := os.Symlink(outsideFile, linkAbs); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
	// Need at least 2 edits, so create a real file too
	realRel := "real.txt"
	writeFixture(t, dir, realRel, "real\n")
	ctx := ctxForDir(dir)
	_, err := executeApplyEdits(ctx, map[string]any{
		"edits": []any{
			map[string]any{"operation": "edit_lines", "path": realRel, "start_line": 1, "end_line": 1, "new_content": "REAL"},
			map[string]any{"operation": "edit_lines", "path": linkRel, "start_line": 1, "end_line": 1, "new_content": "EVIL"},
		},
	})
	if err == nil {
		t.Fatal("expected symlink escape rejection")
	}
	if !strings.Contains(err.Error(), "escapes workspace root") && !strings.Contains(strings.ToLower(err.Error()), "outside") {
		t.Fatalf("expected escape error, got %v", err)
	}
}

func TestApplyEdits_OutsideWorkspaceRejected(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	insideRel := "inside.txt"
	writeFixture(t, dir, insideRel, "inside\n")
	ctx := ctxForDir(dir)
	_, err := executeApplyEdits(ctx, map[string]any{
		"edits": []any{
			map[string]any{"operation": "edit_lines", "path": insideRel, "start_line": 1, "end_line": 1, "new_content": "HI"},
			map[string]any{"operation": "edit_lines", "path": outside, "start_line": 1, "end_line": 1, "new_content": "HI"},
		},
	})
	if err == nil {
		t.Fatal("expected outside workspace error")
	}
	if !strings.Contains(err.Error(), "escapes workspace") {
		t.Fatalf("expected escapes workspace, got %v", err)
	}
}

func TestApplyEdits_OptimisticConflict(t *testing.T) {
	dir := t.TempDir()
	aRel := "a.txt"
	bRel := "b.txt"
	aAbs := writeFixture(t, dir, aRel, "one\ntwo\n")
	bAbs := writeFixture(t, dir, bRel, "alpha\nbeta\n")
	aBefore := readFile(t, aAbs)
	bBefore := readFile(t, bAbs)
	ctx := ctxForDir(dir)
	// Hook mutates a.txt after snapshot but before commit's optimistic check
	applyEditsBeforeCommitHook = func() {
		_ = os.WriteFile(aAbs, []byte("tampered\n"), 0o644)
	}
	t.Cleanup(func() { applyEditsBeforeCommitHook = nil })
	_, err := executeApplyEdits(ctx, map[string]any{
		"edits": []any{
			map[string]any{"operation": "edit_lines", "path": aRel, "start_line": 1, "end_line": 1, "new_content": "ONE"},
			map[string]any{"operation": "edit_lines", "path": bRel, "start_line": 1, "end_line": 1, "new_content": "ALPHA"},
		},
	})
	if err == nil {
		t.Fatal("expected optimistic conflict")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "optimistic conflict") {
		t.Fatalf("expected optimistic conflict, got %v", err)
	}
	// b must be unchanged (bBefore) because commit never happened
	if got := readFile(t, bAbs); got != bBefore {
		t.Errorf("b changed despite conflict: got %q want %q", got, bBefore)
	}
	// a is tampered by hook, not by transaction's planned bytes
	if got := readFile(t, aAbs); got == aBefore {
		t.Error("a should remain tampered, not original")
	}
	if got := readFile(t, aAbs); got == "ONE\ntwo\n" {
		t.Error("a was incorrectly overwritten with transaction data despite conflict")
	}
}

func TestApplyEdits_RollbackOnWriteFailure(t *testing.T) {
	dir := t.TempDir()
	aRel := "a.txt"
	bRel := "b.txt"
	aAbs := writeFixture(t, dir, aRel, "one\ntwo\n")
	bAbs := writeFixture(t, dir, bRel, "alpha\nbeta\n")
	aBefore := readFile(t, aAbs)
	bBefore := readFile(t, bAbs)
	origWrite := applyEditsWriteFile
	call := 0
	applyEditsWriteFile = func(path string, data []byte, perm os.FileMode) error {
		call++
		if call == 2 {
			return errors.New("injected write failure")
		}
		return os.WriteFile(path, data, perm)
	}
	t.Cleanup(func() { applyEditsWriteFile = origWrite })
	ctx := ctxForDir(dir)
	_, err := executeApplyEdits(ctx, map[string]any{
		"edits": []any{
			map[string]any{"operation": "edit_lines", "path": aRel, "start_line": 1, "end_line": 1, "new_content": "ONE"},
			map[string]any{"operation": "edit_lines", "path": bRel, "start_line": 1, "end_line": 1, "new_content": "ALPHA"},
		},
	})
	if err == nil {
		t.Fatal("expected injected write failure")
	}
	// Both should be rolled back (a was first, so rolled back; b never written)
	if got := readFile(t, aAbs); got != aBefore {
		t.Errorf("a not rolled back: got %q want %q", got, aBefore)
	}
	if got := readFile(t, bAbs); got != bBefore {
		t.Errorf("b should be unchanged: got %q want %q", got, bBefore)
	}
}

func TestApplyEdits_RollbackConflictReported(t *testing.T) {
	dir := t.TempDir()
	aRel := "a.txt"
	bRel := "b.txt"
	aAbs := writeFixture(t, dir, aRel, "one\ntwo\n")
	bAbs := writeFixture(t, dir, bRel, "alpha\nbeta\n")
	aBefore := readFile(t, aAbs)
	bBefore := readFile(t, bAbs)
	origWrite := applyEditsWriteFile
	call := 0
	applyEditsWriteFile = func(path string, data []byte, perm os.FileMode) error {
		call++
		if call == 1 {
			// First write succeeds (a)
			if err := os.WriteFile(path, data, perm); err != nil {
				return err
			}
			// Immediately taint it externally so rollback will see planned != current
			_ = os.WriteFile(path, []byte("external taint\n"), 0o644)
			return nil
		}
		if call == 2 {
			return errors.New("second write fails")
		}
		// rollback attempt for a will be call 3, but file is tainted so rollback should detect conflict
		return os.WriteFile(path, data, perm)
	}
	t.Cleanup(func() { applyEditsWriteFile = origWrite })
	ctx := ctxForDir(dir)
	_, err := executeApplyEdits(ctx, map[string]any{
		"edits": []any{
			map[string]any{"operation": "edit_lines", "path": aRel, "start_line": 1, "end_line": 1, "new_content": "ONE"},
			map[string]any{"operation": "edit_lines", "path": bRel, "start_line": 1, "end_line": 1, "new_content": "ALPHA"},
		},
	})
	if err == nil {
		t.Fatal("expected failure with rollback conflict")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "rollback") {
		t.Fatalf("expected rollback conflict mention, got %v", err)
	}
	// a should remain tainted, not restored to aBefore, because rollback detected conflict and skipped
	if got := readFile(t, aAbs); got != "external taint\n" {
		t.Errorf("a should remain tainted due to rollback conflict, got %q", got)
	}
	if got := readFile(t, bAbs); got != bBefore {
		t.Errorf("b should be unchanged, got %q", got)
	}
	_ = aBefore
}

func TestApplyEdits_Bounds(t *testing.T) {
	dir := t.TempDir()
	ctx := ctxForDir(dir)
	files := make([]string, 17)
	for i := range files {
		rel := filepath.ToSlash(filepath.Join("bounds", fmt.Sprintf("f%02d.txt", i)))
		writeFixture(t, dir, rel, "hello\n")
		files[i] = rel
	}
	edits16 := make([]any, 16)
	for i, f := range files[:16] {
		edits16[i] = map[string]any{"operation": "edit_lines", "path": f, "start_line": 1, "end_line": 1, "new_content": "HI"}
	}
	if _, err := executeApplyEdits(ctx, map[string]any{"edits": edits16}); err != nil {
		t.Fatalf("16 edits should succeed: %v", err)
	}
	edits17 := make([]any, 17)
	copy(edits17, edits16)
	edits17[16] = map[string]any{"operation": "edit_lines", "path": files[16], "start_line": 1, "end_line": 1, "new_content": "HI"}
	if _, err := executeApplyEdits(ctx, map[string]any{"edits": edits17}); err == nil {
		t.Fatal("17 edits should fail")
	} else if !strings.Contains(err.Error(), "at most 16") {
		t.Fatalf("expected at most 16 error, got %v", err)
	}
	// 1 should fail
	_, err := executeApplyEdits(ctx, map[string]any{
		"edits": []any{
			map[string]any{"operation": "edit_lines", "path": files[0], "start_line": 1, "end_line": 1, "new_content": "HI"},
		},
	})
	if err == nil {
		t.Fatal("1 edit should fail (at least 2)")
	}
	if !strings.Contains(err.Error(), "at least 2") {
		t.Fatalf("expected at least 2, got %v", err)
	}
}

func TestApplyEdits_OversizeAggregate(t *testing.T) {
	dir := t.TempDir()
	aRel := "a.txt"
	bRel := "b.txt"
	writeFixture(t, dir, aRel, "one\n")
	writeFixture(t, dir, bRel, "two\n")
	ctx := ctxForDir(dir)
	large := strings.Repeat("x", 600*1024) // 600k each -> 1.2M total > 1M limit
	_, err := executeApplyEdits(ctx, map[string]any{
		"edits": []any{
			map[string]any{"operation": "edit_lines", "path": aRel, "start_line": 1, "end_line": 1, "new_content": large},
			map[string]any{"operation": "edit_lines", "path": bRel, "start_line": 1, "end_line": 1, "new_content": large},
		},
	})
	if err == nil {
		t.Fatal("expected oversize error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "aggregate") {
		t.Fatalf("expected aggregate mention, got %v", err)
	}
	// Ensure neither file changed
	if got := readFile(t, filepath.Join(dir, aRel)); got != "one\n" {
		t.Error("oversize should not mutate files")
	}
}

func TestApplyEdits_LineEndingPreservation(t *testing.T) {
	dir := t.TempDir()
	crlfRel := "crlf.txt"
	lfRel := "lf.txt"
	crlfAbs := writeFixture(t, dir, crlfRel, "one\r\ntwo\r\nthree\r\n")
	lfAbs := writeFixture(t, dir, lfRel, "alpha\nbeta\n")
	ctx := ctxForDir(dir)
	if _, err := executeApplyEdits(ctx, map[string]any{
		"edits": []any{
			map[string]any{"operation": "edit_lines", "path": crlfRel, "start_line": 2, "end_line": 2, "new_content": "TWO"},
			map[string]any{"operation": "edit_lines", "path": lfRel, "start_line": 1, "end_line": 1, "new_content": "ALPHA"},
		},
	}); err != nil {
		t.Fatalf("apply_edits CRLF test failed: %v", err)
	}
	crlfData, _ := os.ReadFile(crlfAbs)
	lfCount := strings.Count(string(crlfData), "\n")
	crlfCount := strings.Count(string(crlfData), "\r\n")
	if crlfCount == 0 || lfCount != crlfCount {
		t.Fatalf("CRLF not preserved: lf=%d crlf=%d data=%q", lfCount, crlfCount, string(crlfData))
	}
	lfData, _ := os.ReadFile(lfAbs)
	if strings.Contains(string(lfData), "\r\n") {
		t.Fatalf("LF file incorrectly got CRLF: %q", string(lfData))
	}
}

func TestApplyEdits_Cancellation(t *testing.T) {
	dir := t.TempDir()
	aRel := "a.txt"
	bRel := "b.txt"
	aAbs := writeFixture(t, dir, aRel, "one\ntwo\n")
	bAbs := writeFixture(t, dir, bRel, "alpha\nbeta\n")
	aBefore := readFile(t, aAbs)
	bBefore := readFile(t, bAbs)
	ctx, cancel := context.WithCancel(ctxForDir(dir))
	cancel()
	_, err := executeApplyEdits(ctx, map[string]any{
		"edits": []any{
			map[string]any{"operation": "edit_lines", "path": aRel, "start_line": 1, "end_line": 1, "new_content": "ONE"},
			map[string]any{"operation": "edit_lines", "path": bRel, "start_line": 1, "end_line": 1, "new_content": "ALPHA"},
		},
	})
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if !errors.Is(err, context.Canceled) && !strings.Contains(strings.ToLower(err.Error()), "canceled") && !strings.Contains(strings.ToLower(err.Error()), "cancelled") {
		t.Fatalf("expected canceled, got %v", err)
	}
	if readFile(t, aAbs) != aBefore || readFile(t, bAbs) != bBefore {
		t.Error("cancellation should not mutate files")
	}
}

func TestApplyEdits_NeverCreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	aRel := "a.txt"
	writeFixture(t, dir, aRel, "one\n")
	ctx := ctxForDir(dir)
	missingRel := "missing.txt"
	_, err := executeApplyEdits(ctx, map[string]any{
		"edits": []any{
			map[string]any{"operation": "edit_lines", "path": aRel, "start_line": 1, "end_line": 1, "new_content": "ONE"},
			map[string]any{"operation": "edit_lines", "path": missingRel, "start_line": 1, "end_line": 1, "new_content": "NEW"},
		},
	})
	if err == nil {
		t.Fatal("expected missing file error")
	}
	if _, statErr := os.Stat(filepath.Join(dir, missingRel)); !os.IsNotExist(statErr) {
		t.Error("apply_edits must never create new files")
	}
}

func TestApplyEdits_MalformedShapes(t *testing.T) {
	dir := t.TempDir()
	aRel := "a.txt"
	bRel := "b.txt"
	writeFixture(t, dir, aRel, "one\n")
	writeFixture(t, dir, bRel, "two\n")
	ctx := ctxForDir(dir)
	cases := []struct {
		name string
		args map[string]any
	}{
		{"missing operation", map[string]any{"edits": []any{
			map[string]any{"path": aRel, "start_line": 1, "end_line": 1, "new_content": "x"},
			map[string]any{"operation": "edit_lines", "path": bRel, "start_line": 1, "end_line": 1, "new_content": "y"},
		}}},
		{"unknown operation", map[string]any{"edits": []any{
			map[string]any{"operation": "unknown_op", "path": aRel, "start_line": 1, "end_line": 1},
			map[string]any{"operation": "edit_lines", "path": bRel, "start_line": 1, "end_line": 1, "new_content": "y"},
		}}},
		{"missing path", map[string]any{"edits": []any{
			map[string]any{"operation": "edit_lines", "start_line": 1, "end_line": 1, "new_content": "x"},
			map[string]any{"operation": "edit_lines", "path": bRel, "start_line": 1, "end_line": 1, "new_content": "y"},
		}}},
		{"missing start_line for edit", map[string]any{"edits": []any{
			map[string]any{"operation": "edit_lines", "path": aRel, "end_line": 1, "new_content": "x"},
			map[string]any{"operation": "edit_lines", "path": bRel, "start_line": 1, "end_line": 1, "new_content": "y"},
		}}},
		{"missing content for insert", map[string]any{"edits": []any{
			map[string]any{"operation": "insert_lines", "path": aRel, "after_line": 0},
			map[string]any{"operation": "edit_lines", "path": bRel, "start_line": 1, "end_line": 1, "new_content": "y"},
		}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := executeApplyEdits(ctx, tc.args)
			if err == nil {
				t.Fatalf("%s: expected error", tc.name)
			}
			if !strings.Contains(strings.ToLower(err.Error()), "malformed") {
				t.Fatalf("%s: expected malformed mention, got %v", tc.name, err)
			}
		})
	}
}

func TestApplyCoerceIntRejectsUnsafeNumbers(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value any
	}{
		{name: "fractional float64", value: 1.5},
		{name: "fractional float32", value: float32(2.25)},
		{name: "positive infinity", value: math.Inf(1)},
		{name: "not a number", value: math.NaN()},
		{name: "upper overflow", value: float64(uint64(math.MaxInt) + 1)},
		{name: "unsigned overflow", value: uint64(math.MaxInt) + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, ok := applyCoerceInt(tc.value); ok {
				t.Fatalf("applyCoerceInt(%v) = %d, true; want rejection", tc.value, got)
			}
		})
	}

	for _, value := range []any{int(1), int64(math.MaxInt), float64(2), float32(3)} {
		if _, ok := applyCoerceInt(value); !ok {
			t.Fatalf("applyCoerceInt(%T(%v)) rejected an integral in-range value", value, value)
		}
	}
}

func TestApplyEdits_NoOpRejectedBeforeCommit(t *testing.T) {
	dir := t.TempDir()
	aAbs := writeFixture(t, dir, "a.txt", "one\ntwo\n")
	bAbs := writeFixture(t, dir, "b.txt", "alpha\nbeta\n")
	aBefore := readFile(t, aAbs)
	bBefore := readFile(t, bAbs)

	_, err := executeApplyEdits(ctxForDir(dir), map[string]any{"edits": []any{
		map[string]any{"operation": "edit_lines", "path": "a.txt", "start_line": 1, "end_line": 1, "new_content": "one"},
		map[string]any{"operation": "edit_lines", "path": "b.txt", "start_line": 1, "end_line": 1, "new_content": "ALPHA"},
	}})
	if err == nil || !strings.Contains(err.Error(), "produces no change") {
		t.Fatalf("expected no-op rejection, got %v", err)
	}
	if got := readFile(t, aAbs); got != aBefore {
		t.Fatalf("no-op rejection changed a.txt: got %q want %q", got, aBefore)
	}
	if got := readFile(t, bAbs); got != bBefore {
		t.Fatalf("no-op rejection changed b.txt: got %q want %q", got, bBefore)
	}
}

func TestApplyEdits_InterWriteConflictRollsBackEarlierWrite(t *testing.T) {
	dir := t.TempDir()
	aAbs := writeFixture(t, dir, "a.txt", "one\ntwo\n")
	bAbs := writeFixture(t, dir, "b.txt", "alpha\nbeta\n")
	aBefore := readFile(t, aAbs)
	const externalB = "external change\n"

	origWrite := applyEditsWriteFile
	writes := 0
	applyEditsWriteFile = func(path string, data []byte, perm os.FileMode) error {
		writes++
		if err := os.WriteFile(path, data, perm); err != nil {
			return err
		}
		if writes == 1 {
			return os.WriteFile(bAbs, []byte(externalB), 0o644)
		}
		return nil
	}
	t.Cleanup(func() { applyEditsWriteFile = origWrite })

	_, err := executeApplyEdits(ctxForDir(dir), map[string]any{"edits": []any{
		map[string]any{"operation": "edit_lines", "path": "a.txt", "start_line": 1, "end_line": 1, "new_content": "ONE"},
		map[string]any{"operation": "edit_lines", "path": "b.txt", "start_line": 1, "end_line": 1, "new_content": "ALPHA"},
	}})
	if err == nil || !strings.Contains(err.Error(), "optimistic conflict") {
		t.Fatalf("expected inter-write optimistic conflict, got %v", err)
	}
	if got := readFile(t, aAbs); got != aBefore {
		t.Fatalf("earlier write was not rolled back: got %q want %q", got, aBefore)
	}
	if got := readFile(t, bAbs); got != externalB {
		t.Fatalf("external change was overwritten: got %q want %q", got, externalB)
	}
}

func TestApplyEdits_Registration(t *testing.T) {
	reg := tools.NewRegistry()
	if err := RegisterAll(reg); err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}
	if reg.Get("apply_edits") == nil {
		t.Fatal("apply_edits not registered")
	}
	// Idempotent second call
	if err := RegisterAll(reg); err != nil {
		t.Fatalf("second RegisterAll failed: %v", err)
	}
	if reg.Get("apply_edits") == nil {
		t.Fatal("apply_edits missing after second register")
	}
	// Direct registry Execute via tool name should also validate edits required
	_, err := reg.Execute(ctxForDir(t.TempDir()), "apply_edits", map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing edits via registry")
	}
}
