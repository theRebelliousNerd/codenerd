package shell

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// The workspace's build env sets GOFLAGS=-tags=feature_a and its go_flags
// add -tags=feature_b on the command line. go vet applies GOFLAGS first and
// the command line second, and a later -tags replaces the earlier set, so
// vet builds feature_b alone: the feature_a file, with its planted finding,
// is not checked. The receipt named both tags, so the model would call the
// feature_a file verified (audit run-lint-effective-build-tag-receipt).
func TestRunLint_BuildTagsAreTheTagsTheInvocationApplied(t *testing.T) {
	t.Setenv("GOFLAGS", "")
	root := t.TempDir()
	files := map[string]string{
		"go.mod":            "module tagprobe\n\ngo 1.22\n",
		"feat/a.go":         "//go:build feature_a\n\npackage feat\n\nimport \"fmt\"\n\n// A prints n.\nfunc A(n string) { fmt.Printf(\"%d\\n\", n) }\n",
		"feat/b.go":         "//go:build feature_b\n\npackage feat\n\n// B is 2.\nfunc B() int { return 2 }\n",
		"feat/doc.go":       "// Package feat is tagged.\npackage feat\n",
		".nerd/config.json": `{"build": {"env_vars": {"GOFLAGS": "-tags=feature_a"}, "go_flags": ["-tags=feature_b"]}}`,
	}
	for name, content := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r, err := runLint(t, root, map[string]any{"gate": "vet", "packages": []any{"./feat"}})
	if err != nil {
		t.Fatalf("run_lint: %v (%+v)", err, r)
	}
	if r.Status != LintClean {
		t.Fatalf("precondition: vet built feature_b only and should be clean, got %+v", r)
	}
	if !slices.Contains(r.Argv, "-tags=feature_b") || r.GoFlags != "-tags=feature_a" {
		t.Fatalf("precondition: argv %v goflags %q, want both inputs kept", r.Argv, r.GoFlags)
	}
	if !slices.Equal(r.BuildTags, []string{"feature_b"}) {
		t.Fatalf("build_tags = %v, want [feature_b]: the command line replaced GOFLAGS' tags", r.BuildTags)
	}
}

// The resolution, case by case: GOFLAGS then the command line, each -tags
// replacing the set before it; an explicit empty -tags is no custom tags;
// GOFLAGS is split the way the go command splits it (quoted words).
func TestEffectiveBuildTags(t *testing.T) {
	cases := []struct {
		name    string
		goflags string
		argv    []string
		want    []string
	}{
		{"env only", "-tags=sqlite_vec", []string{"go", "vet", "-json", "./..."}, []string{"sqlite_vec"}},
		{"argv only", "", []string{"go", "vet", "-tags", "a,b", "./..."}, []string{"a", "b"}},
		{"argv replaces env", "-tags=a", []string{"go", "vet", "-tags=b", "-json", "./p"}, []string{"b"}},
		{"later argv replaces earlier", "", []string{"go", "vet", "-tags=a", "--tags=c", "./p"}, []string{"c"}},
		{"explicit empty", "-tags=a", []string{"go", "vet", "-tags=", "./p"}, []string{}},
		{"quoted goflags", `"-tags=a b" -mod=mod`, []string{"go", "vet", "./p"}, []string{"a", "b"}},
		{"none", "", []string{"go", "vet", "./p"}, []string{}},
	}
	for _, c := range cases {
		got, unresolved := effectiveBuildTags("vet", c.goflags, c.argv, t.TempDir(), "")
		if unresolved != "" || !slices.Equal(got, c.want) {
			t.Errorf("%s: tags %v (unresolved %q), want %v", c.name, got, unresolved, c.want)
		}
	}

	// golangci-lint can set its own build tags in its config file: with one
	// present the receipt says the tags are unresolved instead of guessing.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".golangci.yml"), []byte("run:\n  build-tags: [x]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if tags, unresolved := effectiveBuildTags("golangci-lint", "-tags=a", []string{"golangci-lint", "run", "./..."}, dir, dir); unresolved == "" || len(tags) != 0 {
		t.Fatalf("golangci-lint with a config file: tags %v unresolved %q, want unresolved", tags, unresolved)
	}
	if tags, unresolved := effectiveBuildTags("golangci-lint", "-tags=a", []string{"golangci-lint", "run", "./..."}, t.TempDir(), ""); unresolved != "" || !slices.Equal(tags, []string{"a"}) {
		t.Fatalf("golangci-lint without a config file: tags %v unresolved %q, want [a]", tags, unresolved)
	}
}
