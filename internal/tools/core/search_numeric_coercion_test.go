package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file is the regression for the silent-limit bug in search.go:
// bare args["max_results"].(int) always failed because JSON numbers arrive as
// float64 (and Mangle as int64), so grep was capped at 50 and glob at 100.
// It asserts on the resolved limit actually used via executeGlob/executeGrep so
// a future bare assertion at the call site fails the test.

// helper: count glob results (one per line, empty means 0)
func countGlobResults(result string) int {
	if strings.Contains(result, "No files found") {
		return 0
	}
	result = strings.TrimSpace(result)
	if result == "" {
		return 0
	}
	return len(strings.Split(result, "\n"))
}

// helper: count grep matches (one primary line per match, context lines are indented)
func countGrepMatches(result string) int {
	if strings.Contains(result, "No matches found") {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(result), "\n")
	n := 0
	for _, l := range lines {
		// primary match lines are "file:line: content" without leading spaces;
		// context lines are prefixed with two spaces.
		if strings.HasPrefix(l, "  ") {
			continue
		}
		if strings.TrimSpace(l) == "" {
			continue
		}
		n++
	}
	return n
}


func itoa(n int) string {
	// minimal itoa without fmt to keep helper pure; handles 0-999
	if n == 0 {
		return "0"
	}
	var buf [10]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}

func createGrepFileWithMatches(t *testing.T, dir string, matches int) string {
	t.Helper()
	path := filepath.Join(dir, "matches.txt")
	var sb strings.Builder
	for i := 0; i < matches; i++ {
		sb.WriteString("hello world line " + itoa(i) + "\n")
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("write grep file: %v", err)
	}
	return path
}

// Tests for argInt helper directly (sanity), but primary assertions are via executeGlob/Grep below.

func TestArgInt_AcceptsExpectedTypes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		val  any
		want int
		ok   bool
	}{
		{"int", int(7), 7, true},
		{"int64 mangle-sourced", int64(7), 7, true},
		{"float64 JSON production", float64(7), 7, true},
		{"json.Number", json.Number("7"), 7, true},
		{"json.Number float string", json.Number("7.9"), 7, true},
		{"missing", nil, 0, false},
		{"wrong type string", "7", 0, false},
		{"bool", true, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := map[string]any{}
			if tc.val != nil {
				args["k"] = tc.val
			}
			got, ok := argInt(args, "k")
			if got != tc.want || ok != tc.ok {
				t.Fatalf("argInt(%#v) = (%d,%v) want (%d,%v)", tc.val, got, ok, tc.want, tc.ok)
			}
		})
	}
}

// =============================================================================
// GLOB: max_results coercion
// =============================================================================

func TestGlob_MaxResults_Float64OverridesDefault_SmallLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(filepath.Join(dir, "file_"+itoa(i)+".go"), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
	}
	result, err := executeGlob(context.Background(), map[string]any{
		"pattern":     "*.go",
		"base_path":   dir,
		"max_results": float64(2),
	})
	if err != nil {
		t.Fatalf("executeGlob: %v", err)
	}
	if got := countGlobResults(result); got != 2 {
		t.Fatalf("float64 max_results=2 should yield 2 results, got %d: %q", got, result)
	}
}

func TestGlob_MaxResults_IntOverrides(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		os.WriteFile(filepath.Join(dir, "file_"+itoa(i)+".go"), []byte(""), 0644)
	}
	result, err := executeGlob(context.Background(), map[string]any{
		"pattern":     "*.go",
		"base_path":   dir,
		"max_results": int(2),
	})
	if err != nil {
		t.Fatalf("executeGlob: %v", err)
	}
	if got := countGlobResults(result); got != 2 {
		t.Fatalf("int max_results=2 should yield 2, got %d: %q", got, result)
	}
}

func TestGlob_MaxResults_Int64Overrides(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		os.WriteFile(filepath.Join(dir, "file_"+itoa(i)+".go"), []byte(""), 0644)
	}
	result, err := executeGlob(context.Background(), map[string]any{
		"pattern":     "*.go",
		"base_path":   dir,
		"max_results": int64(2),
	})
	if err != nil {
		t.Fatalf("executeGlob: %v", err)
	}
	if got := countGlobResults(result); got != 2 {
		t.Fatalf("int64 max_results=2 should yield 2, got %d: %q", got, result)
	}
}

func TestGlob_MaxResults_JsonNumberOverrides(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		os.WriteFile(filepath.Join(dir, "file_"+itoa(i)+".go"), []byte(""), 0644)
	}
	result, err := executeGlob(context.Background(), map[string]any{
		"pattern":     "*.go",
		"base_path":   dir,
		"max_results": json.Number("2"),
	})
	if err != nil {
		t.Fatalf("executeGlob: %v", err)
	}
	if got := countGlobResults(result); got != 2 {
		t.Fatalf("json.Number max_results=2 should yield 2, got %d: %q", got, result)
	}
}

func TestGlob_MaxResults_Float64OverridesDefault_LargeLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	const total = 110
	for i := 0; i < total; i++ {
		os.WriteFile(filepath.Join(dir, "file_"+itoa(i)+".go"), []byte(""), 0644)
	}
	result, err := executeGlob(context.Background(), map[string]any{
		"pattern":     "*.go",
		"base_path":   dir,
		"max_results": float64(500),
	})
	if err != nil {
		t.Fatalf("executeGlob: %v", err)
	}
	if got := countGlobResults(result); got != total {
		t.Fatalf("float64 max_results=500 should override default 100 and return %d, got %d: %q", total, got, result)
	}
}

func TestGlob_MaxResults_ZeroDoesNotOverride(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	const total = 110
	for i := 0; i < total; i++ {
		os.WriteFile(filepath.Join(dir, "file_"+itoa(i)+".go"), []byte(""), 0644)
	}
	result, err := executeGlob(context.Background(), map[string]any{
		"pattern":     "*.go",
		"base_path":   dir,
		"max_results": float64(0),
	})
	if err != nil {
		t.Fatalf("executeGlob: %v", err)
	}
	if got := countGlobResults(result); got != 100 {
		t.Fatalf("zero max_results should not override default 100, expected 100 got %d", got)
	}
	// also int zero
	result, _ = executeGlob(context.Background(), map[string]any{
		"pattern":     "*.go",
		"base_path":   dir,
		"max_results": int(0),
	})
	if got := countGlobResults(result); got != 100 {
		t.Fatalf("int zero should not override, got %d", got)
	}
}

func TestGlob_MaxResults_NegativeDoesNotOverride(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	const total = 110
	for i := 0; i < total; i++ {
		os.WriteFile(filepath.Join(dir, "file_"+itoa(i)+".go"), []byte(""), 0644)
	}
	result, err := executeGlob(context.Background(), map[string]any{
		"pattern":     "*.go",
		"base_path":   dir,
		"max_results": float64(-5),
	})
	if err != nil {
		t.Fatalf("executeGlob: %v", err)
	}
	if got := countGlobResults(result); got != 100 {
		t.Fatalf("negative max_results should not override default 100, got %d", got)
	}
	result, _ = executeGlob(context.Background(), map[string]any{
		"pattern":     "*.go",
		"base_path":   dir,
		"max_results": int(-10),
	})
	if got := countGlobResults(result); got != 100 {
		t.Fatalf("int negative should not override, got %d", got)
	}
}

func TestGlob_MaxResults_AbsentLeavesDefault(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	const total = 110
	for i := 0; i < total; i++ {
		os.WriteFile(filepath.Join(dir, "file_"+itoa(i)+".go"), []byte(""), 0644)
	}
	result, err := executeGlob(context.Background(), map[string]any{
		"pattern":   "*.go",
		"base_path": dir,
	})
	if err != nil {
		t.Fatalf("executeGlob: %v", err)
	}
	if got := countGlobResults(result); got != 100 {
		t.Fatalf("absent max_results should leave default 100, got %d", got)
	}
}

// =============================================================================
// GREP: max_results coercion
// =============================================================================

func TestGrep_MaxResults_Float64OverridesDefault_SmallLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := createGrepFileWithMatches(t, dir, 10)
	result, err := executeGrep(context.Background(), map[string]any{
		"pattern":     "hello",
		"path":        path,
		"max_results": float64(2),
	})
	if err != nil {
		t.Fatalf("executeGrep: %v", err)
	}
	if got := countGrepMatches(result); got != 2 {
		t.Fatalf("float64 max_results=2 should yield 2, got %d: %q", got, result)
	}
}

func TestGrep_MaxResults_IntOverrides(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := createGrepFileWithMatches(t, dir, 10)
	result, err := executeGrep(context.Background(), map[string]any{
		"pattern":     "hello",
		"path":        path,
		"max_results": int(2),
	})
	if err != nil {
		t.Fatalf("executeGrep: %v", err)
	}
	if got := countGrepMatches(result); got != 2 {
		t.Fatalf("int max_results=2 should yield 2, got %d: %q", got, result)
	}
}

func TestGrep_MaxResults_Int64Overrides(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := createGrepFileWithMatches(t, dir, 10)
	result, err := executeGrep(context.Background(), map[string]any{
		"pattern":     "hello",
		"path":        path,
		"max_results": int64(2),
	})
	if err != nil {
		t.Fatalf("executeGrep: %v", err)
	}
	if got := countGrepMatches(result); got != 2 {
		t.Fatalf("int64 max_results=2 should yield 2, got %d: %q", got, result)
	}
}

func TestGrep_MaxResults_JsonNumberOverrides(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := createGrepFileWithMatches(t, dir, 10)
	result, err := executeGrep(context.Background(), map[string]any{
		"pattern":     "hello",
		"path":        path,
		"max_results": json.Number("3"),
	})
	if err != nil {
		t.Fatalf("executeGrep: %v", err)
	}
	if got := countGrepMatches(result); got != 3 {
		t.Fatalf("json.Number max_results=3 should yield 3, got %d: %q", got, result)
	}
}

func TestGrep_MaxResults_Float64OverridesDefault_LargeLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	const total = 60
	path := createGrepFileWithMatches(t, dir, total)
	result, err := executeGrep(context.Background(), map[string]any{
		"pattern":     "hello",
		"path":        path,
		"max_results": float64(500),
	})
	if err != nil {
		t.Fatalf("executeGrep: %v", err)
	}
	if got := countGrepMatches(result); got != total {
		t.Fatalf("float64 max_results=500 should override default 50 and return %d, got %d", total, got)
	}
}

func TestGrep_MaxResults_ZeroDoesNotOverride(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := createGrepFileWithMatches(t, dir, 60)
	result, err := executeGrep(context.Background(), map[string]any{
		"pattern":     "hello",
		"path":        path,
		"max_results": float64(0),
	})
	if err != nil {
		t.Fatalf("executeGrep: %v", err)
	}
	if got := countGrepMatches(result); got != 50 {
		t.Fatalf("zero max_results should not override default 50, got %d", got)
	}
	result, _ = executeGrep(context.Background(), map[string]any{
		"pattern":     "hello",
		"path":        path,
		"max_results": int(0),
	})
	if got := countGrepMatches(result); got != 50 {
		t.Fatalf("int zero should not override default 50, got %d", got)
	}
}

func TestGrep_MaxResults_NegativeDoesNotOverride(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := createGrepFileWithMatches(t, dir, 60)
	result, err := executeGrep(context.Background(), map[string]any{
		"pattern":     "hello",
		"path":        path,
		"max_results": float64(-5),
	})
	if err != nil {
		t.Fatalf("executeGrep: %v", err)
	}
	if got := countGrepMatches(result); got != 50 {
		t.Fatalf("negative max_results should not override default 50, got %d", got)
	}
	result, _ = executeGrep(context.Background(), map[string]any{
		"pattern":     "hello",
		"path":        path,
		"max_results": int(-10),
	})
	if got := countGrepMatches(result); got != 50 {
		t.Fatalf("int negative should not override, got %d", got)
	}
}

func TestGrep_MaxResults_AbsentLeavesDefault(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := createGrepFileWithMatches(t, dir, 60)
	result, err := executeGrep(context.Background(), map[string]any{
		"pattern": "hello",
		"path":    path,
	})
	if err != nil {
		t.Fatalf("executeGrep: %v", err)
	}
	if got := countGrepMatches(result); got != 50 {
		t.Fatalf("absent max_results should leave default 50, got %d", got)
	}
}

// =============================================================================
// GREP: context_lines coercion
// =============================================================================

func TestGrep_ContextLines_Float64Accepted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "ctx.txt")
	content := "line1\nline2\ntarget line\nline4\nline5\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	// Without context, result should not contain line2's content beyond the match line itself
	resultNoCtx, err := executeGrep(context.Background(), map[string]any{
		"pattern": "target",
		"path":    path,
	})
	if err != nil {
		t.Fatalf("executeGrep: %v", err)
	}
	if !strings.Contains(resultNoCtx, "target") {
		t.Fatalf("expected target in result")
	}
	// With float64 context_lines=1, preceding line should appear as context (indented)
	result, err := executeGrep(context.Background(), map[string]any{
		"pattern":       "target",
		"path":          path,
		"context_lines": float64(1),
	})
	if err != nil {
		t.Fatalf("executeGrep: %v", err)
	}
	if !strings.Contains(result, "line2") {
		t.Fatalf("float64 context_lines=1 should include preceding line as context, got: %q", result)
	}
}

func TestGrep_ContextLines_IntAccepted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "ctx.txt")
	os.WriteFile(path, []byte("a\nb\ntarget\nc\nd\n"), 0644)
	result, err := executeGrep(context.Background(), map[string]any{
		"pattern":       "target",
		"path":          path,
		"context_lines": int(1),
	})
	if err != nil {
		t.Fatalf("executeGrep: %v", err)
	}
	if !strings.Contains(result, "b") {
		t.Fatalf("int context_lines=1 should include context, got %q", result)
	}
}

func TestGrep_ContextLines_Int64Accepted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "ctx.txt")
	os.WriteFile(path, []byte("a\nb\ntarget\nc\nd\n"), 0644)
	result, err := executeGrep(context.Background(), map[string]any{
		"pattern":       "target",
		"path":          path,
		"context_lines": int64(1),
	})
	if err != nil {
		t.Fatalf("executeGrep: %v", err)
	}
	if !strings.Contains(result, "b") {
		t.Fatalf("int64 context_lines=1 should include context, got %q", result)
	}
}

func TestGrep_ContextLines_JsonNumberAccepted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "ctx.txt")
	os.WriteFile(path, []byte("a\nb\ntarget\nc\nd\n"), 0644)
	result, err := executeGrep(context.Background(), map[string]any{
		"pattern":       "target",
		"path":          path,
		"context_lines": json.Number("1"),
	})
	if err != nil {
		t.Fatalf("executeGrep: %v", err)
	}
	if !strings.Contains(result, "b") {
		t.Fatalf("json.Number context_lines=1 should include context, got %q", result)
	}
}
