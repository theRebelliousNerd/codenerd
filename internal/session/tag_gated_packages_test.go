package session

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSplitTagGatedPackages(t *testing.T) {
	workspace := t.TempDir()

	if err := os.WriteFile(filepath.Join(workspace, "go.mod"), []byte("module example.com/m\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	files := []struct {
		dir     string
		name    string
		content string
	}{
		{"a", "a.go", "package a\n"},
		{"a", "a_integration_test.go", "//go:build integration\n\npackage a\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n"},
		{"e2e", "x_test.go", "//go:build integration && !race\n\npackage e2e\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n"},
		{"multi", "f1_test.go", "//go:build integration\n\npackage multi\n\nimport \"testing\"\n\nfunc TestF1(t *testing.T) {}\n"},
		{"multi", "f2_test.go", "//go:build linux && e2e\n\npackage multi\n\nimport \"testing\"\n\nfunc TestF2(t *testing.T) {}\n"},
		{"ign", "z.go", "//go:build ignore\n\npackage ign\n"},
		{"orsplit", "o_test.go", "//go:build (integration || smoke) && go1.21\n\npackage orsplit\n\nimport \"testing\"\n\nfunc TestO(t *testing.T) {}\n"},
	}
	for _, f := range files {
		dir := filepath.Join(workspace, f.dir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", f.dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, f.name), []byte(f.content), 0o644); err != nil {
			t.Fatalf("write %s/%s: %v", f.dir, f.name, err)
		}
	}

	packages := []string{"./a", "./e2e", "./multi", "./ign", "./orsplit", "./missing", "."}
	runnable, gated := splitTagGatedPackages(workspace, packages)

	wantRunnable := []string{"./a", "./missing", "."}
	if !reflect.DeepEqual(runnable, wantRunnable) {
		t.Fatalf("runnable = %q, want %q", runnable, wantRunnable)
	}
	if _, ok := gated["./a"]; ok {
		t.Fatalf("./a should not be gated, got %q", gated["./a"])
	}

	// ./ign normalises nil vs empty slice: key must be present with len 0.
	ignTags, ok := gated["./ign"]
	if !ok {
		t.Fatalf("./ign missing from gated map: %v", gated)
	}
	if len(ignTags) != 0 {
		t.Fatalf("./ign tags = %q, want empty", ignTags)
	}

	cases := []struct {
		pkg  string
		want []string
	}{
		{"./e2e", []string{"integration"}},
		{"./multi", []string{"e2e", "integration"}},
		{"./orsplit", []string{"integration", "smoke"}},
	}
	for _, tc := range cases {
		t.Run(tc.pkg, func(t *testing.T) {
			got, ok := gated[tc.pkg]
			if !ok {
				t.Fatalf("%s missing from gated map: %v", tc.pkg, gated)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("%s tags = %q, want %q", tc.pkg, got, tc.want)
			}
		})
	}

	if len(gated) != 4 {
		t.Fatalf("gated map has %d entries, want 4: %v", len(gated), gated)
	}
}

func TestTagsForFile_StopsAtPackageClause(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "late.go")
	content := "package late\n\n//go:build nope\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write late.go: %v", err)
	}
	if got := tagsForFile(path); len(got) != 0 {
		t.Fatalf("tagsForFile after package clause = %q, want empty", got)
	}
}
