package retrieval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newBoundedRetriever(t *testing.T, dir string) *SparseRetriever {
	t.Helper()
	cfg := DefaultSparseRetrieverConfig(dir)
	cfg.SearchTimeout = 30 * time.Second
	return NewSparseRetriever(cfg)
}

func writeFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

// =============================================================================
// Resource bounds
// =============================================================================

// TestSearch_ShouldSkipOversizedFiles keeps the scanner from pulling a
// multi-megabyte artifact into memory (then copying it again for case folding).
func TestSearch_ShouldSkipOversizedFiles(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "small.go", []byte("package main\n// needle here\n"))

	// One file past the cutoff that also contains the keyword.
	big := make([]byte, maxScanFileSize+1024)
	for i := range big {
		big[i] = 'a'
	}
	copy(big, []byte("needle"))
	writeFile(t, dir, "huge.txt", big)

	r := newBoundedRetriever(t, dir)
	hits, err := r.searchSingleKeyword(context.Background(), "needle")
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	for _, h := range hits {
		if strings.HasSuffix(h.FilePath, "huge.txt") {
			t.Errorf("oversized file was scanned: %s", h.FilePath)
		}
	}
	if len(hits) == 0 {
		t.Error("expected the small file to still match")
	}
}

// TestSearch_ShouldSkipBinaryFiles verifies NUL-containing payloads are ignored.
func TestSearch_ShouldSkipBinaryFiles(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "real.go", []byte("package main\n// needle\n"))
	writeFile(t, dir, "blob.bin", []byte("needle\x00\x01\x02binary payload"))

	r := newBoundedRetriever(t, dir)
	hits, err := r.searchSingleKeyword(context.Background(), "needle")
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	for _, h := range hits {
		if strings.HasSuffix(h.FilePath, "blob.bin") {
			t.Errorf("binary file was scanned: %s", h.FilePath)
		}
	}
	if len(hits) == 0 {
		t.Error("expected the text file to still match")
	}
}

func TestIsBinaryContent(t *testing.T) {
	t.Parallel()
	if isBinaryContent([]byte("plain text, no nul")) {
		t.Error("plain text flagged as binary")
	}
	if !isBinaryContent([]byte("head\x00tail")) {
		t.Error("NUL byte not detected")
	}
	// A NUL far past the sniff window is deliberately not inspected.
	tail := append([]byte(strings.Repeat("x", binarySniffBytes+100)), 0x00)
	if isBinaryContent(tail) {
		t.Error("sniff window should not extend past binarySniffBytes")
	}
	if isBinaryContent(nil) {
		t.Error("empty content flagged as binary")
	}
}

// TestSearch_ShouldCapHitsPerFile stops one generated file from producing an
// unbounded hit list.
func TestSearch_ShouldCapHitsPerFile(t *testing.T) {
	dir := t.TempDir()

	var sb strings.Builder
	for i := 0; i < maxHitsPerFile*3; i++ {
		fmt.Fprintf(&sb, "needle line %d\n", i)
	}
	writeFile(t, dir, "generated.go", []byte(sb.String()))

	r := newBoundedRetriever(t, dir)
	hits, err := r.searchSingleKeyword(context.Background(), "needle")
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	if len(hits) > maxHitsPerFile {
		t.Errorf("got %d hits from one file, want <= %d", len(hits), maxHitsPerFile)
	}
	if len(hits) == 0 {
		t.Error("expected some hits")
	}
}

func TestWorkerBudget_ShouldDivideAcrossSearches(t *testing.T) {
	r := newBoundedRetriever(t, t.TempDir())
	r.parallelism = 8

	// No search in flight: the caller still gets the full budget.
	if got := r.workerBudget(); got != 8 {
		t.Errorf("idle budget = %d, want 8", got)
	}

	r.activeSearches.Store(4)
	if got := r.workerBudget(); got != 2 {
		t.Errorf("budget with 4 in flight = %d, want 2", got)
	}

	// More searches than workers must still yield a usable pool.
	r.activeSearches.Store(64)
	if got := r.workerBudget(); got != 1 {
		t.Errorf("oversubscribed budget = %d, want 1", got)
	}
}

func TestWorkerBudget_ShouldSurviveDegenerateConfig(t *testing.T) {
	r := newBoundedRetriever(t, t.TempDir())
	r.parallelism = 0
	if got := r.workerBudget(); got < 1 {
		t.Errorf("budget = %d, want >= 1", got)
	}
}

// =============================================================================
// Tier 4 definition search
// =============================================================================

// TestFindSymbolDefinitions_ShouldMatchRealDefinitions is the regression test
// for the T4 bug: patterns were built as regexes ("^class Foo") but consumed by
// a literal byte scanner, so the search looked for a caret in the source and
// matched nothing for any symbol in any language.
func TestFindSymbolDefinitions_ShouldMatchRealDefinitions(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "widget.py", []byte("import os\n\n\nclass Widget:\n    def build(self):\n        pass\n"))
	writeFile(t, dir, "widget.go", []byte("package main\n\nfunc Widget() error {\n\treturn nil\n}\n"))
	writeFile(t, dir, "widget.rs", []byte("pub fn Widget() -> u32 {\n    0\n}\n"))
	// A file that merely mentions the symbol must not be treated as defining it.
	writeFile(t, dir, "caller.go", []byte("package main\n\nfunc run() { _ = Widget() }\n"))

	b := NewTieredContextBuilder(DefaultTieredContextConfig(dir))
	files := b.findSymbolDefinitions(context.Background(), "Widget")

	if len(files) == 0 {
		t.Fatal("tier 4 found no definitions; the literal-vs-regex bug is back")
	}

	found := make(map[string]bool)
	for _, f := range files {
		found[filepath.Base(f)] = true
	}
	for _, want := range []string{"widget.py", "widget.go", "widget.rs"} {
		if !found[want] {
			t.Errorf("expected %s among definitions, got %v", want, files)
		}
	}
	if found["caller.go"] {
		t.Error("a call site was reported as a definition")
	}
}

func TestIsLineLeading(t *testing.T) {
	t.Parallel()
	tests := []struct {
		line    string
		pattern string
		want    bool
	}{
		{"class Widget:", "class Widget", true},
		{"    def build(self):", "def build", true},     // indented method
		{"pub fn Widget() -> u32 {", "fn Widget", true}, // Rust modifier
		{"export function Widget() {", "function Widget", true},
		{"async function Widget() {", "function Widget", true},
		{"CLASS Widget:", "class Widget", true}, // scanner folds case
		{"_ = Widget()", "func Widget", false},
		{"return class Widget", "class Widget", false}, // mid-expression
		{"", "class Widget", false},
		{"   ", "class Widget", false},
	}
	for _, tc := range tests {
		if got := isLineLeading(tc.line, tc.pattern); got != tc.want {
			t.Errorf("isLineLeading(%q, %q) = %v, want %v", tc.line, tc.pattern, got, tc.want)
		}
	}
}

// TestFindSymbolDefinitions_ShouldNotMatchUnrelatedSymbol guards against the
// keyword sweep returning every file that happens to contain "class".
func TestFindSymbolDefinitions_ShouldNotMatchUnrelatedSymbol(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "widget.py", []byte("class Widget:\n    pass\n"))

	b := NewTieredContextBuilder(DefaultTieredContextConfig(dir))
	if files := b.findSymbolDefinitions(context.Background(), "Gadget"); len(files) != 0 {
		t.Errorf("expected no definitions for an absent symbol, got %v", files)
	}
}
