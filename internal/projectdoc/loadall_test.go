package projectdoc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const minimalValid = `---
schema: nerd/v1
project: test
---
Body
`

func writeValidNerdMd(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(minimalValid), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestLoadAll_RootOnly(t *testing.T) {
	dir := t.TempDir()
	writeValidNerdMd(t, filepath.Join(dir, FileName))
	docs, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d docs, want 1", len(docs))
	}
	if docs[0].Path != "nerd.md" {
		t.Errorf("Path = %q, want %q", docs[0].Path, "nerd.md")
	}
	if strings.Contains(docs[0].Path, "\\") {
		t.Errorf("Path %q contains backslash, want POSIX", docs[0].Path)
	}
	if filepath.IsAbs(docs[0].Path) {
		t.Errorf("Path %q is absolute, want workspace-relative", docs[0].Path)
	}
}

func TestLoadAll_RootPlusTwoModulesSorted(t *testing.T) {
	dir := t.TempDir()
	writeValidNerdMd(t, filepath.Join(dir, FileName))
	writeValidNerdMd(t, filepath.Join(dir, "internal", "b", FileName))
	writeValidNerdMd(t, filepath.Join(dir, "internal", "a", FileName))
	docs, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(docs) != 3 {
		t.Fatalf("got %d docs, want 3", len(docs))
	}
	// Root first.
	if docs[0].Path != "nerd.md" {
		t.Errorf("docs[0].Path = %q, want %q (root first)", docs[0].Path, "nerd.md")
	}
	// Modules sorted by path.
	want := []string{"internal/a/nerd.md", "internal/b/nerd.md"}
	for i, w := range want {
		got := docs[i+1].Path
		if got != w {
			t.Errorf("docs[%d].Path = %q, want %q", i+1, got, w)
		}
		if strings.Contains(got, "\\") {
			t.Errorf("Path %q contains backslash, want POSIX", got)
		}
		if filepath.IsAbs(got) {
			t.Errorf("Path %q is absolute, want workspace-relative", got)
		}
	}
	// Ensure sorted order.
	if docs[1].Path > docs[2].Path {
		t.Errorf("modules not sorted: %q > %q", docs[1].Path, docs[2].Path)
	}
}

func TestLoadAll_HiddenDirSkipped(t *testing.T) {
	dir := t.TempDir()
	writeValidNerdMd(t, filepath.Join(dir, FileName))
	writeValidNerdMd(t, filepath.Join(dir, ".hidden", FileName))
	docs, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d docs, want 1 (.hidden should be skipped), paths: %v", len(docs), pathsOf(docs))
	}
	if docs[0].Path != "nerd.md" {
		t.Errorf("Path = %q, want %q", docs[0].Path, "nerd.md")
	}
}

func TestLoadAll_NodeModulesSkipped(t *testing.T) {
	dir := t.TempDir()
	writeValidNerdMd(t, filepath.Join(dir, FileName))
	writeValidNerdMd(t, filepath.Join(dir, "node_modules", "pkg", FileName))
	writeValidNerdMd(t, filepath.Join(dir, "node_modules", FileName))
	docs, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d docs, want 1 (node_modules should be skipped), paths: %v", len(docs), pathsOf(docs))
	}
}

func TestLoadAll_VendorSkipped(t *testing.T) {
	dir := t.TempDir()
	writeValidNerdMd(t, filepath.Join(dir, FileName))
	writeValidNerdMd(t, filepath.Join(dir, "vendor", "pkg", FileName))
	docs, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d docs, want 1 (vendor should be skipped), paths: %v", len(docs), pathsOf(docs))
	}
}

func TestLoadAll_GitAndNerdDirsSkipped(t *testing.T) {
	dir := t.TempDir()
	writeValidNerdMd(t, filepath.Join(dir, FileName))
	writeValidNerdMd(t, filepath.Join(dir, ".git", FileName))
	writeValidNerdMd(t, filepath.Join(dir, ".nerd", "sub", FileName))
	docs, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d docs, want 1 (.git and .nerd should be skipped), paths: %v", len(docs), pathsOf(docs))
	}
}

func TestLoadAll_InvalidModuleError(t *testing.T) {
	dir := t.TempDir()
	writeValidNerdMd(t, filepath.Join(dir, FileName))
	invalid := filepath.Join(dir, "internal", "broken", FileName)
	if err := os.MkdirAll(filepath.Dir(invalid), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Invalid schema version.
	if err := os.WriteFile(invalid, []byte("---\nschema: nerd/v9\n---\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := LoadAll(dir)
	if err == nil {
		t.Fatal("expected error for invalid module nerd.md, got nil")
	}
	if !strings.Contains(err.Error(), "internal/broken/nerd.md") {
		t.Errorf("error %q must name offending path %q", err.Error(), "internal/broken/nerd.md")
	}
}

func TestLoadAll_EmptyWorkspace(t *testing.T) {
	dir := t.TempDir()
	docs, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll on empty workspace: %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("got %d docs, want 0 for empty workspace, paths: %v", len(docs), pathsOf(docs))
	}
	if docs != nil && len(docs) != 0 {
		t.Errorf("expected empty slice, got %v", docs)
	}
}

func TestLoadAll_RootOnlyPathIsPOSIX(t *testing.T) {
	dir := t.TempDir()
	writeValidNerdMd(t, filepath.Join(dir, FileName))
	docs, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d docs, want 1", len(docs))
	}
	if docs[0].Path != filepath.ToSlash(docs[0].Path) {
		t.Errorf("Path %q is not POSIX", docs[0].Path)
	}
}

func pathsOf(docs []*Document) []string {
	out := make([]string, len(docs))
	for i, d := range docs {
		out[i] = d.Path
	}
	return out
}
