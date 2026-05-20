package retrieval

import (
	"os"
	"path/filepath"
	"testing"
)

// --- DefaultTieredContextConfig ---

func TestDefaultTieredContextConfig_ShouldReturnSensibleDefaults(t *testing.T) {
	cfg := DefaultTieredContextConfig("/some/dir")
	if cfg.WorkDir != "/some/dir" {
		t.Errorf("expected WorkDir='/some/dir', got %q", cfg.WorkDir)
	}
	if cfg.Tier1Budget != 0.30 {
		t.Errorf("expected Tier1Budget=0.30, got %f", cfg.Tier1Budget)
	}
	if cfg.Tier2Budget != 0.40 {
		t.Errorf("expected Tier2Budget=0.40, got %f", cfg.Tier2Budget)
	}
	if cfg.Tier3Budget != 0.20 {
		t.Errorf("expected Tier3Budget=0.20, got %f", cfg.Tier3Budget)
	}
	if cfg.Tier4Budget != 0.10 {
		t.Errorf("expected Tier4Budget=0.10, got %f", cfg.Tier4Budget)
	}
	if cfg.MaxTotal != 50 {
		t.Errorf("expected MaxTotal=50, got %d", cfg.MaxTotal)
	}
}

// --- NewTieredContextBuilder ---

func TestNewTieredContextBuilder_WhenNilConfig_ShouldUseDefaults_Coverage(t *testing.T) {
	builder := NewTieredContextBuilder(nil)
	if builder == nil {
		t.Fatal("expected non-nil builder")
	}
	if builder.tier1Budget != 0.30 {
		t.Errorf("expected tier1Budget=0.30, got %f", builder.tier1Budget)
	}
}

func TestNewTieredContextBuilder_WhenCustomConfig_ShouldApply_Coverage(t *testing.T) {
	cfg := &TieredContextConfig{
		WorkDir:     "/test",
		Tier1Budget: 0.5,
		Tier2Budget: 0.3,
		Tier3Budget: 0.1,
		Tier4Budget: 0.1,
		MaxTotal:    20,
	}
	builder := NewTieredContextBuilder(cfg)
	if builder.tier1Budget != 0.5 {
		t.Errorf("expected tier1Budget=0.5, got %f", builder.tier1Budget)
	}
	if builder.maxTier1 != 10 { // 20 * 0.5
		t.Errorf("expected maxTier1=10, got %d", builder.maxTier1)
	}
	if builder.maxTier2 != 6 { // 20 * 0.3
		t.Errorf("expected maxTier2=6, got %d", builder.maxTier2)
	}
}

func TestNewTieredContextBuilder_WhenZeroMaxTotal_ShouldDefault50_Coverage(t *testing.T) {
	cfg := &TieredContextConfig{
		WorkDir:     "/test",
		Tier1Budget: 0.30,
		MaxTotal:    0,
	}
	builder := NewTieredContextBuilder(cfg)
	if builder.maxTier1 != 15 { // 50 * 0.30
		t.Errorf("expected maxTier1=15 from default 50, got %d", builder.maxTier1)
	}
}

// --- TieredContext.GetFilesByTier ---

func TestGetFilesByTier_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	tc := &TieredContext{Files: []ContextFile{}}
	tier1 := tc.GetFilesByTier(1)
	if len(tier1) != 0 {
		t.Errorf("expected 0 files for empty context, got %d", len(tier1))
	}
}

func TestGetFilesByTier_WhenMixed_ShouldFilterCorrectly(t *testing.T) {
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
	tier2 := tc.GetFilesByTier(2)
	if len(tier2) != 1 {
		t.Errorf("expected 1 tier-2 file, got %d", len(tier2))
	}
	tier4 := tc.GetFilesByTier(4)
	if len(tier4) != 0 {
		t.Errorf("expected 0 tier-4 files, got %d", len(tier4))
	}
}

// --- TieredContext.GetTopFiles ---

func TestGetTopFiles_WhenNGreaterThanLength_ShouldReturnAll(t *testing.T) {
	tc := &TieredContext{
		Files: []ContextFile{
			{FilePath: "a.go", RelevanceScore: 0.5},
			{FilePath: "b.go", RelevanceScore: 0.9},
		},
	}
	top := tc.GetTopFiles(10)
	if len(top) != 2 {
		t.Errorf("expected 2, got %d", len(top))
	}
}

func TestGetTopFiles_ShouldSortByRelevance(t *testing.T) {
	tc := &TieredContext{
		Files: []ContextFile{
			{FilePath: "low.go", RelevanceScore: 0.1},
			{FilePath: "high.go", RelevanceScore: 0.9},
			{FilePath: "mid.go", RelevanceScore: 0.5},
		},
	}
	top := tc.GetTopFiles(2)
	if len(top) != 2 {
		t.Fatalf("expected 2, got %d", len(top))
	}
	if top[0].FilePath != "high.go" {
		t.Errorf("expected first file 'high.go', got %q", top[0].FilePath)
	}
	if top[1].FilePath != "mid.go" {
		t.Errorf("expected second file 'mid.go', got %q", top[1].FilePath)
	}
}

func TestGetTopFiles_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	tc := &TieredContext{Files: []ContextFile{}}
	top := tc.GetTopFiles(5)
	if len(top) != 0 {
		t.Errorf("expected 0 files, got %d", len(top))
	}
}

// --- TieredContext.GetFilePaths ---

func TestGetFilePaths_ShouldReturnPaths(t *testing.T) {
	tc := &TieredContext{
		Files: []ContextFile{
			{FilePath: "a.go"},
			{FilePath: "b.go"},
			{FilePath: "c.go"},
		},
	}
	paths := tc.GetFilePaths()
	if len(paths) != 3 {
		t.Fatalf("expected 3 paths, got %d", len(paths))
	}
	if paths[0] != "a.go" || paths[1] != "b.go" || paths[2] != "c.go" {
		t.Errorf("unexpected paths: %v", paths)
	}
}

func TestGetFilePaths_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	tc := &TieredContext{Files: []ContextFile{}}
	paths := tc.GetFilePaths()
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %d", len(paths))
	}
}

// --- TieredContext.LoadContent ---

func TestLoadContent_ShouldLoadFiles(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "file1.txt")
	f2 := filepath.Join(dir, "file2.txt")
	os.WriteFile(f1, []byte("hello"), 0644)
	os.WriteFile(f2, []byte("world"), 0644)

	tc := &TieredContext{
		Files: []ContextFile{
			{FilePath: f1},
			{FilePath: f2},
		},
	}
	err := tc.LoadContent(1 << 20) // 1MB budget
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tc.Files[0].Content != "hello" {
		t.Errorf("expected 'hello', got %q", tc.Files[0].Content)
	}
	if tc.Files[1].Content != "world" {
		t.Errorf("expected 'world', got %q", tc.Files[1].Content)
	}
}

func TestLoadContent_WhenBudgetExceeded_ShouldStopLoading(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "big.txt")
	f2 := filepath.Join(dir, "small.txt")
	os.WriteFile(f1, []byte("12345678901234567890"), 0644) // 20 bytes
	os.WriteFile(f2, []byte("abc"), 0644)

	tc := &TieredContext{
		Files: []ContextFile{
			{FilePath: f1},
			{FilePath: f2},
		},
	}
	err := tc.LoadContent(20) // exactly enough for first file
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tc.Files[0].Content == "" {
		t.Error("expected first file to be loaded")
	}
	if tc.Files[1].Content != "" {
		t.Error("expected second file to NOT be loaded (budget exceeded)")
	}
}

func TestLoadContent_WhenFileMissing_ShouldSkipAndContinue(t *testing.T) {
	dir := t.TempDir()
	f2 := filepath.Join(dir, "exists.txt")
	os.WriteFile(f2, []byte("data"), 0644)

	tc := &TieredContext{
		Files: []ContextFile{
			{FilePath: filepath.Join(dir, "missing.txt")},
			{FilePath: f2},
		},
	}
	err := tc.LoadContent(1 << 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tc.Files[0].Content != "" {
		t.Error("expected missing file to have no content")
	}
	if tc.Files[1].Content != "data" {
		t.Errorf("expected 'data', got %q", tc.Files[1].Content)
	}
}

// --- findFile ---

func TestFindFile_WhenExactPath_ShouldReturn(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "src")
	os.MkdirAll(subDir, 0755)
	testFile := filepath.Join(subDir, "main.go")
	os.WriteFile(testFile, []byte("package main"), 0644)

	builder := NewTieredContextBuilder(&TieredContextConfig{WorkDir: dir})
	found := builder.findFile(nil, filepath.Join("src", "main.go"))
	if found == "" {
		t.Error("expected file to be found by exact path")
	}
}

func TestFindFile_WhenNonExistent_ShouldReturnEmpty(t *testing.T) {
	dir := t.TempDir()
	builder := NewTieredContextBuilder(&TieredContextConfig{WorkDir: dir})
	found := builder.findFile(nil, "nonexistent_file.go")
	if found != "" {
		t.Errorf("expected empty string for nonexistent file, got %q", found)
	}
}

// --- extractImports ---

func TestExtractImports_WhenPythonFile_ShouldExtract(t *testing.T) {
	dir := t.TempDir()
	pyFile := filepath.Join(dir, "test.py")
	content := `import os
import sys
from pathlib import Path
from collections import OrderedDict
`
	os.WriteFile(pyFile, []byte(content), 0644)

	builder := NewTieredContextBuilder(&TieredContextConfig{WorkDir: dir})
	imports := builder.extractImports(pyFile)
	if len(imports) != 4 {
		t.Fatalf("expected 4 imports, got %d: %v", len(imports), imports)
	}
}

func TestExtractImports_WhenNonExistent_ShouldReturnNil(t *testing.T) {
	builder := NewTieredContextBuilder(&TieredContextConfig{WorkDir: "."})
	imports := builder.extractImports("/nonexistent/file.py")
	if imports != nil {
		t.Errorf("expected nil for nonexistent file, got %v", imports)
	}
}

// --- resolveImport ---

func TestResolveImport_WhenRelativeFileExists_ShouldResolve(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "pkg")
	os.MkdirAll(subDir, 0755)
	targetFile := filepath.Join(subDir, "utils.py")
	os.WriteFile(targetFile, []byte("def helper(): pass"), 0644)

	builder := NewTieredContextBuilder(&TieredContextConfig{WorkDir: dir})
	resolved := builder.resolveImport("utils", filepath.Join(subDir, "main.py"))
	if resolved == "" {
		t.Error("expected import to resolve to utils.py")
	}
}

func TestResolveImport_WhenNotFound_ShouldReturnEmpty(t *testing.T) {
	dir := t.TempDir()
	builder := NewTieredContextBuilder(&TieredContextConfig{WorkDir: dir})
	resolved := builder.resolveImport("nonexistent_module", filepath.Join(dir, "main.py"))
	if resolved != "" {
		t.Errorf("expected empty string for unresolvable import, got %q", resolved)
	}
}

// --- ContextFile struct ---

func TestContextFile_ShouldHoldAllFields(t *testing.T) {
	cf := ContextFile{
		FilePath:        "/path/to/file.go",
		Tier:            2,
		RelevanceScore:  0.85,
		SelectionReason: "keyword match",
		Keywords:        []string{"auth", "login"},
		ImportedBy:      []string{"/path/to/other.go"},
		Content:         "package main",
	}
	if cf.FilePath != "/path/to/file.go" {
		t.Error("FilePath mismatch")
	}
	if cf.Tier != 2 {
		t.Error("Tier mismatch")
	}
	if cf.RelevanceScore != 0.85 {
		t.Error("RelevanceScore mismatch")
	}
	if len(cf.Keywords) != 2 {
		t.Error("Keywords length mismatch")
	}
}
