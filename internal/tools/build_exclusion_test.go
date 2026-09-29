package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func pinExclusionEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GOFLAGS", "")
	t.Setenv("CGO_ENABLED", "1")
}

// The constraint evaluator, pinned directly: tag gating under explicit tag
// sets, version tags, negation, malformed lines (fail closed), and the
// header rules (no package clause, no blank line) that keep inert text from
// counting as a constraint.
func TestBuildExclusion_Evaluator(t *testing.T) {
	t.Setenv("CGO_ENABLED", "1")
	other := "windows"
	if runtime.GOOS == "windows" {
		other = "linux"
	}
	tests := []struct {
		name     string
		goLines  []string
		plus     []string
		tags     map[string]bool
		excluded bool
	}{
		{"no lines included", nil, nil, map[string]bool{}, false},
		{"ignore excluded", []string{"ignore"}, nil, map[string]bool{}, true},
		{"ignore excluded even with tags", []string{"ignore"}, nil, map[string]bool{"x": true}, true},
		{"current goos included", []string{runtime.GOOS}, nil, map[string]bool{}, false},
		{"other goos excluded", []string{other}, nil, map[string]bool{}, true},
		{"tag gated out without the tag", []string{"sqlite_vec"}, nil, map[string]bool{}, true},
		{"tag gated in with the tag", []string{"sqlite_vec"}, nil, map[string]bool{"sqlite_vec": true}, false},
		{"conjunction needs every tag", []string{"a && b"}, nil, map[string]bool{"a": true}, true},
		{"negation holds", []string{"!" + other}, nil, map[string]bool{}, false},
		{"malformed fails closed", []string{"&&"}, nil, map[string]bool{}, true},
		{"bare line fails closed", []string{""}, nil, map[string]bool{}, true},
		{"plus ignore excluded", nil, []string{"ignore"}, map[string]bool{}, true},
		{"plus current goos included", nil, []string{runtime.GOOS}, map[string]bool{}, false},
		{"plus negation holds", nil, []string{"!" + other}, map[string]bool{}, false},
		{"plus conjunction needs every term", nil, []string{runtime.GOOS + ",missing_tag"}, map[string]bool{}, true},
		{"plus alternative saves", nil, []string{"missing_tag " + runtime.GOOS}, map[string]bool{}, false},
		{"go1.0 holds everywhere", []string{"go1.0"}, nil, map[string]bool{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildConstraintsExclude(tt.goLines, tt.plus, tt.tags); got != tt.excluded {
				t.Errorf("buildConstraintsExclude(%v, %v, %v) = %v, want %v",
					tt.goLines, tt.plus, tt.tags, got, tt.excluded)
			}
		})
	}
}

func TestBuildExclusion_HeaderLines(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		goLines   []string
		plusLines []string
	}{
		{"plain file", "package p\n", nil, nil},
		{"go build line", "//go:build linux\n\npackage p\n", []string{"linux"}, nil},
		{"plus build line", "// +build linux\n\npackage p\n", nil, []string{"linux"}},
		{"no blank line is inert", "//go:build ignore\npackage p\n", nil, nil},
		{"no package clause is nothing", "//go:build ignore\n", nil, nil},
		{"post-package mention inert", "package p\n\n// //go:build ignore\n", nil, nil},
		{"prose comment inert", "// adds //go:build ignore handling\n\npackage p\n", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goLines, plusLines := headerBuildConstraints([]byte(tt.content))
			if strings.Join(goLines, "|") != strings.Join(tt.goLines, "|") ||
				strings.Join(plusLines, "|") != strings.Join(tt.plusLines, "|") {
				t.Errorf("headerBuildConstraints(%q) = (%v, %v), want (%v, %v)",
					tt.content, goLines, plusLines, tt.goLines, tt.plusLines)
			}
		})
	}
}

func TestRefuseAddedBuildExclusion_EditFile(t *testing.T) {
	pinExclusionEnv(t)
	dir := t.TempDir()
	compiled := "package p\n\nfunc A() int { return 1 }\n"
	path := filepath.Join(dir, "a.go")
	if err := os.WriteFile(path, []byte(compiled), 0o644); err != nil {
		t.Fatal(err)
	}
	err := RefuseAddedBuildExclusion("edit_file", map[string]any{
		"path": "a.go", "old_text": "package p\n", "new_text": "//go:build ignore\n\npackage p\n",
	}, dir)
	if err == nil || !strings.Contains(err.Error(), "excludes") {
		t.Fatalf("err = %v, want the added ignore refused", err)
	}
	if data, _ := os.ReadFile(path); string(data) != compiled {
		t.Fatalf("the projection wrote: %q", data)
	}

	ignored := "//go:build ignore\n\n" + compiled
	if err := os.WriteFile(path, []byte(ignored), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RefuseAddedBuildExclusion("edit_file", map[string]any{
		"path": "a.go", "old_text": "return 1", "new_text": "return 2",
	}, dir); err != nil {
		t.Fatalf("a file that already excluded was refused: %v", err)
	}

	if err := os.WriteFile(path, []byte(compiled), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RefuseAddedBuildExclusion("edit_file", map[string]any{
		"path":     "a.go",
		"old_text": "func A() int { return 1 }",
		"new_text": "func A() int {\n\t// Never add //go:build ignore here.\n\treturn 1\n}",
	}, dir); err != nil {
		t.Fatalf("a mention after the package clause was refused: %v", err)
	}
}

// "package p" occurs twice in the file and once in the header. The old
// projection skipped any old that was not unique in the file, so this edit
// reached the tool.
func TestRefuseAddedBuildExclusion_HeaderElement(t *testing.T) {
	pinExclusionEnv(t)
	dir := t.TempDir()
	body := "package p\n\nfunc A() int { return 1 }\n\nfunc B() {\n\t// package p\n}\n"
	path := filepath.Join(dir, "a.go")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	err := RefuseAddedBuildExclusion("edit_element", map[string]any{
		"path": "a.go", "ref": "header", "old": "package p", "new": "//go:build ignore\n\npackage p",
	}, dir)
	if err == nil || !strings.Contains(err.Error(), "excludes") {
		t.Fatalf("err = %v, want the header edit refused", err)
	}

	err = RefuseAddedBuildExclusion("replace_element", map[string]any{
		"path": "a.go", "ref": "header", "source": "//go:build ignore\n\npackage p",
	}, dir)
	if err == nil || !strings.Contains(err.Error(), "excludes") {
		t.Fatalf("err = %v, want the header replacement refused", err)
	}

	two := "package p\n\nfunc A() int { return 1 }\n\nfunc B() int { return 1 }\n"
	if err := os.WriteFile(path, []byte(two), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RefuseAddedBuildExclusion("edit_element", map[string]any{
		"path": "a.go", "ref": "A", "old": "return 1", "new": "return 2",
	}, dir); err != nil {
		t.Fatalf("a non-unique old that is unique in its element was refused: %v", err)
	}
}

func TestRefuseAddedBuildExclusion_Uncertain(t *testing.T) {
	pinExclusionEnv(t)
	dir := t.TempDir()
	body := "package p\n\nfunc A() int { return 1 }\n"
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	err := RefuseAddedBuildExclusion("edit_element", map[string]any{
		"path": "a.go", "ref": "nope", "old": "return 1", "new": "return 2",
	}, dir)
	if err == nil || !strings.Contains(err.Error(), "cannot be checked") || strings.Contains(err.Error(), "excludes") {
		t.Fatalf("err = %v, want an unresolved ref refused without an exclusion verdict", err)
	}
	err = RefuseAddedBuildExclusion("edit_file", map[string]any{
		"path": "a.go", "old_text": "package p", "new_text": "//go:build ignore\n\npackage p",
	}, "")
	if err == nil || !strings.Contains(err.Error(), "cannot be checked") {
		t.Fatalf("err = %v, want a relative path with no workspace refused", err)
	}
	if err := RefuseAddedBuildExclusion("apply_patch", map[string]any{"path": "a.go"}, dir); err == nil || !strings.Contains(err.Error(), "cannot be checked") {
		t.Fatalf("err = %v, want an unprojected write tool refused", err)
	}
	if err := RefuseAddedBuildExclusion("delete_file", map[string]any{"path": "a.go"}, dir); err != nil {
		t.Fatalf("delete_file was judged: %v", err)
	}
	if err := RefuseAddedBuildExclusion("insert_element", map[string]any{
		"path": "a.go", "anchor": "header", "position": "before", "source": "func C() int { return 3 }",
	}, dir); err != nil {
		t.Fatalf("insert before the header is the tool's own refusal, got %v", err)
	}
}
