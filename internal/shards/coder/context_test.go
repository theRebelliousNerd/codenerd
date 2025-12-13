package coder

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
)

func TestReadFileContext_InjectsFactsViaVirtualStore(t *testing.T) {
	workspace := t.TempDir()
	filename := "sample.go"
	absPath := filepath.Join(workspace, filename)
	content := "// Package main\npackage main\n\nfunc main() {}\n"
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}

	vsCfg := core.DefaultVirtualStoreConfig()
	vsCfg.WorkingDir = workspace
	vs := core.NewVirtualStoreWithConfig(nil, vsCfg)
	vs.SetKernel(kernel)

	coderCfg := DefaultCoderConfig()
	coderCfg.WorkingDir = workspace
	c := NewCoderShardWithConfig(coderCfg)
	c.SetParentKernel(kernel)
	c.SetVirtualStore(vs)

	got, err := c.readFileContext(context.Background(), filename)
	if err != nil {
		t.Fatalf("readFileContext: %v", err)
	}
	if got != content {
		t.Fatalf("readFileContext content mismatch; got len=%d want len=%d", len(got), len(content))
	}

	facts, err := kernel.Query("file_content")
	if err != nil {
		t.Fatalf("Query(file_content): %v", err)
	}
	found := false
	for _, f := range facts {
		if len(f.Args) < 2 {
			continue
		}
		p, _ := f.Args[0].(string)
		c, _ := f.Args[1].(string)
		if p == absPath {
			if !strings.HasPrefix(c, "// Package") {
				t.Fatalf("file_content not preserved, got prefix=%q", c[:min(len(c), 16)])
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected file_content fact for %s", absPath)
	}
}

func TestIsTestFile_DetectsTestDirectoriesCrossPlatform(t *testing.T) {
	path := filepath.Join("a", "tests", "b.go")
	if !isTestFile(path) {
		t.Fatalf("expected %q to be detected as test file", path)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
