package retrieval

import (
	"context"
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

// --- TieredContext.GetTopFiles ---

// --- TieredContext.GetFilePaths ---

// --- TieredContext.LoadContent ---

// --- findFile ---

func TestFindFile_WhenExactPath_ShouldReturn(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "src")
	os.MkdirAll(subDir, 0755)
	testFile := filepath.Join(subDir, "main.go")
	os.WriteFile(testFile, []byte("package main"), 0644)

	builder := NewTieredContextBuilder(&TieredContextConfig{WorkDir: dir})
	found := builder.findFile(context.Background(), filepath.Join("src", "main.go"))
	if found == "" {
		t.Error("expected file to be found by exact path")
	}
}

func TestFindFile_WhenNonExistent_ShouldReturnEmpty(t *testing.T) {
	dir := t.TempDir()
	builder := NewTieredContextBuilder(&TieredContextConfig{WorkDir: dir})
	found := builder.findFile(context.Background(), "nonexistent_file.go")
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
