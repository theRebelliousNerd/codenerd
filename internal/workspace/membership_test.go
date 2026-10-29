package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/config"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	argv := append([]string{
		"-c", "user.email=test@example.com",
		"-c", "user.name=test",
		"-c", "core.quotepath=false",
		"-c", "commit.gpgsign=false",
	}, args...)
	cmd := exec.Command("git", argv...)
	cmd.Dir = dir
	cmd.Env = gitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMembershipGit(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".gitignore"), ""+
		"ignored_dir/\n"+
		"*.log\n"+
		"**/secret/**\n"+
		"!keep.log\n")
	writeFile(t, filepath.Join(root, "keep.go"), "package keep\n")
	writeFile(t, filepath.Join(root, "foo.log"), "x\n")
	writeFile(t, filepath.Join(root, "keep.log"), "y\n")
	writeFile(t, filepath.Join(root, "ignored_dir", "a.go"), "package a\n")
	writeFile(t, filepath.Join(root, "secret", "a", "b.txt"), "s\n")
	writeFile(t, filepath.Join(root, "nested", "inner", ".gitignore"), "nested_skip.dat\n")
	writeFile(t, filepath.Join(root, "nested", "inner", "tracked.go"), "package t\n")
	writeFile(t, filepath.Join(root, "nested", "inner", "nested_skip.dat"), "n\n")
	writeFile(t, filepath.Join(root, "plain", "a.go"), "package p\n")
	runGit(t, root, "init")
	runGit(t, root, "add", "keep.go", ".gitignore", "nested/inner/tracked.go", "nested/inner/.gitignore")
	runGit(t, root, "commit", "-m", "init")

	// No extra user patterns: gitignore is the whole answer, plus .git/.nerd.
	m, err := Open(root, []string{})
	if err != nil {
		t.Fatal(err)
	}
	if !m.Includes("keep.go") || !m.Includes("keep.log") || !m.Includes("plain/a.go") {
		t.Fatalf("members missing: %v", m.Files())
	}
	if m.Includes("foo.log") || m.Includes("ignored_dir/a.go") || m.Includes("secret/a/b.txt") || m.Includes("nested/inner/nested_skip.dat") {
		t.Fatalf("ignored paths were members: %v", m.Files())
	}
	if m.IncludesDir("ignored_dir") {
		t.Fatal("ignored_dir/ should not be opened")
	}
	if !m.IncludesDir("plain") || !m.IncludesDir("nested/inner") {
		t.Fatal("directories of member files should be members")
	}
	if m.Includes(".git/HEAD") || m.IncludesDir(".git") || m.IncludesDir(".nerd") {
		t.Fatal(".git and .nerd are always excluded")
	}
	if m.Includes("") || !m.IncludesDir("") || !m.IncludesDir(".") {
		t.Fatal("root is a directory, not a file")
	}
	if m.Includes("../outside") || m.IncludesDir("../outside") {
		t.Fatal("paths outside the root are not members")
	}

	for _, f := range m.Files() {
		if strings.Contains(f, "\\") {
			t.Fatalf("Files path %q is not slash-separated", f)
		}
		switch f {
		case "foo.log", "ignored_dir/a.go", "secret/a/b.txt", "nested/inner/nested_skip.dat":
			t.Fatalf("Files listed ignored %s", f)
		}
	}

	// A file created after the snapshot is admitted by check-ignore, and is
	// absent from Files until Refresh.
	writeFile(t, filepath.Join(root, "later.go"), "package later\n")
	if !m.Includes("later.go") {
		t.Fatal("later.go should be admitted by check-ignore")
	}
	for _, f := range m.Files() {
		if f == "later.go" {
			t.Fatal("Files is the snapshot until Refresh")
		}
	}
	if err := m.Refresh(); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range m.Files() {
		if f == "later.go" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Refresh should list later.go, got %v", m.Files())
	}

	// Adding the file moves the index; the next Includes refreshes on its own.
	runGit(t, root, "add", "later.go")
	writeFile(t, filepath.Join(root, "after_add.go"), "package after\n")
	if !m.Includes("later.go") || !m.Includes("after_add.go") {
		t.Fatal("index change should refresh, and the new file should be admitted")
	}

	var walked []string
	var opened []string
	m.readDir = func(p string) ([]os.DirEntry, error) {
		rel, _ := filepath.Rel(root, p)
		opened = append(opened, filepath.ToSlash(rel))
		return os.ReadDir(p)
	}
	if err := m.Walk(context.Background(), func(rel string, d os.DirEntry) error {
		if rel != "" {
			walked = append(walked, rel)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, rel := range walked {
		if rel == "foo.log" || strings.HasPrefix(rel, "ignored_dir") || strings.HasPrefix(rel, "secret") {
			t.Fatalf("walk yielded %s", rel)
		}
	}
	for _, rel := range opened {
		if rel == "ignored_dir" || strings.HasPrefix(rel, "ignored_dir/") || rel == "secret" || strings.HasPrefix(rel, "secret/") || rel == ".git" || strings.HasPrefix(rel, ".git/") {
			t.Fatalf("walk opened %s (all: %v)", rel, opened)
		}
	}
	joined := strings.Join(walked, "\n")
	if !strings.Contains(joined, "keep.go") || !strings.Contains(joined, "later.go") {
		t.Fatalf("walk missed members: %v", walked)
	}
}

func TestWalkNeverEntersBigIgnoredDir(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "big", "nest", "a", "f.txt"), "x\n")
	writeFile(t, filepath.Join(root, "keep.txt"), "k\n")
	m, err := Open(root, []string{"big"})
	if err != nil {
		t.Fatal(err)
	}
	var opened []string
	m.readDir = func(p string) ([]os.DirEntry, error) {
		rel, _ := filepath.Rel(root, p)
		opened = append(opened, filepath.ToSlash(rel))
		return os.ReadDir(p)
	}
	if err := m.Walk(context.Background(), func(rel string, d os.DirEntry) error { return nil }); err != nil {
		t.Fatal(err)
	}
	for _, rel := range opened {
		if rel == "big" || strings.HasPrefix(rel, "big/") {
			t.Fatalf("entered %s; opened %v", rel, opened)
		}
	}
	if m.Includes("big/nest/a/f.txt") || m.IncludesDir("big") || m.IncludesDir("big/nest") {
		t.Fatal("pattern big should drop the tree")
	}
	if !m.Includes("keep.txt") {
		t.Fatal("keep.txt should be a member")
	}
}

func TestGitlinkAndNestedRepoAreBoundaries(t *testing.T) {
	parent := t.TempDir()
	runGit(t, parent, "init")
	writeFile(t, filepath.Join(parent, "top.go"), "package top\n")
	runGit(t, parent, "add", "top.go")
	runGit(t, parent, "commit", "-m", "top")

	sub := filepath.Join(parent, "sub")
	writeFile(t, filepath.Join(sub, "inside.go"), "package inside\n")
	runGit(t, sub, "init")
	runGit(t, sub, "add", "inside.go")
	runGit(t, sub, "commit", "-m", "sub")
	sha := strings.TrimSpace(runGit(t, sub, "rev-parse", "HEAD"))
	runGit(t, parent, "update-index", "--add", "--cacheinfo", "160000,"+sha+",sub")

	// A nested repo that is not a gitlink is listed by the parent as
	// "nestedrepo/" — one directory entry, not the files inside it.
	// check-ignore would call those files unignored; the listing is what
	// closes the directory.
	nested := filepath.Join(parent, "nestedrepo")
	writeFile(t, filepath.Join(nested, "f.go"), "package f\n")
	runGit(t, nested, "init")
	runGit(t, nested, "add", "f.go")
	runGit(t, nested, "commit", "-m", "nested")

	m, err := Open(parent, []string{})
	if err != nil {
		t.Fatal(err)
	}
	if !m.Includes("top.go") {
		t.Fatal("top.go")
	}
	if !m.Includes("sub") || m.IncludesDir("sub") || m.Includes("sub/inside.go") {
		t.Fatalf("gitlink: Includes(sub)=%v IncludesDir=%v child=%v", m.Includes("sub"), m.IncludesDir("sub"), m.Includes("sub/inside.go"))
	}
	if !m.Includes("nestedrepo") || m.IncludesDir("nestedrepo") || m.Includes("nestedrepo/f.go") {
		t.Fatalf("nested repo: Includes=%v dir=%v child=%v", m.Includes("nestedrepo"), m.IncludesDir("nestedrepo"), m.Includes("nestedrepo/f.go"))
	}
	for _, f := range m.Files() {
		if f == "sub" || f == "nestedrepo" || strings.HasPrefix(f, "sub/") || strings.HasPrefix(f, "nestedrepo/") {
			t.Fatalf("Files listed boundary %s in %v", f, m.Files())
		}
	}
	var opened []string
	m.readDir = func(p string) ([]os.DirEntry, error) {
		rel, _ := filepath.Rel(parent, p)
		opened = append(opened, filepath.ToSlash(rel))
		return os.ReadDir(p)
	}
	if err := m.Walk(context.Background(), func(string, os.DirEntry) error { return nil }); err != nil {
		t.Fatal(err)
	}
	for _, rel := range opened {
		if rel == "sub" || strings.HasPrefix(rel, "sub/") || rel == "nestedrepo" || strings.HasPrefix(rel, "nestedrepo/") {
			t.Fatalf("entered boundary %s", rel)
		}
	}
}

func TestNestedGitignoreRefresh(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "nested", "seen.dat"), "a\n")
	writeFile(t, filepath.Join(root, "keep.go"), "package k\n")
	runGit(t, root, "init")
	runGit(t, root, "add", "keep.go")
	runGit(t, root, "commit", "-m", "init")
	m, err := Open(root, []string{})
	if err != nil {
		t.Fatal(err)
	}
	if !m.Includes("nested/seen.dat") {
		t.Fatal("seen.dat starts untracked and unignored")
	}
	// A nested gitignore edit does not move the index or the root .gitignore.
	// The file already in the snapshot stays a member until Refresh. A file
	// that was never in the snapshot is decided by live check-ignore.
	writeFile(t, filepath.Join(root, "nested", ".gitignore"), "seen.dat\nnew.dat\n")
	writeFile(t, filepath.Join(root, "nested", "new.dat"), "b\n")
	if !m.Includes("nested/seen.dat") {
		t.Fatal("snapshot member stays until Refresh when only a nested gitignore changes")
	}
	if m.Includes("nested/new.dat") {
		t.Fatal("new.dat was never snapshotted; check-ignore should see the nested gitignore")
	}
	if err := m.Refresh(); err != nil {
		t.Fatal(err)
	}
	if m.Includes("nested/seen.dat") {
		t.Fatal("Refresh should drop seen.dat")
	}
}

func TestUserPatternsOnTopOfGit(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "keep.go"), "package k\n")
	writeFile(t, filepath.Join(root, "vendor", "v.go"), "package v\n")
	writeFile(t, filepath.Join(root, ".nerd", "agents", "a.md"), "x\n")
	runGit(t, root, "init")
	runGit(t, root, "add", "-f", "keep.go", "vendor/v.go", ".nerd/agents/a.md")
	runGit(t, root, "commit", "-m", "init")

	// Exercise a caller-supplied exclusion, not a filled-in default list.
	m, err := Open(root, []string{"vendor"})
	if err != nil {
		t.Fatal(err)
	}
	if !m.Includes("keep.go") {
		t.Fatal("keep.go")
	}
	if m.Includes("vendor/v.go") || m.IncludesDir("vendor") {
		t.Fatal("tracked vendor is still excluded by the user pattern")
	}
	if m.Includes(".nerd/agents/a.md") || m.IncludesDir(".nerd") || m.IncludesDir(".nerd/agents") {
		t.Fatal(".nerd stays excluded even when it was force-added")
	}

	// A negation cannot bring .git or .nerd back, and cannot override gitignore.
	writeFile(t, filepath.Join(root, ".gitignore"), "hidden.go\n")
	writeFile(t, filepath.Join(root, "hidden.go"), "package h\n")
	neg, err := Open(root, []string{"!.nerd", "!.nerd/**", "!.git", "!hidden.go"})
	if err != nil {
		t.Fatal(err)
	}
	// Root .gitignore changed, but this is a new Membership. hidden.go is
	// gitignored; the user negation must not re-include it.
	if neg.Includes("hidden.go") || neg.IncludesDir(".nerd") || neg.Includes(".nerd/agents/a.md") || neg.IncludesDir(".git") {
		t.Fatal("user negation must not override gitignore or .git/.nerd")
	}
}

func TestPatternFallbackOutsideGit(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "keep.go"), "package k\n")
	writeFile(t, filepath.Join(root, "node_modules", "a.js"), "x\n")
	writeFile(t, filepath.Join(root, ".hidden", "h.go"), "package h\n")
	writeFile(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/main\n")
	writeFile(t, filepath.Join(root, ".nerd", "config.json"), "{}\n")

	// A HEAD-only .git is not a work tree. Membership falls back to patterns
	// instead of failing the scan.
	m, err := For(root)
	if err != nil {
		t.Fatal(err)
	}
	if m.git {
		t.Fatal("fake .git should not be treated as a work tree")
	}
	if !m.Includes("keep.go") || !m.Includes(".hidden/h.go") {
		t.Fatal("hidden directories are members when no pattern names them")
	}
	if m.Includes("node_modules/a.js") || m.IncludesDir("node_modules") {
		t.Fatal("default patterns should drop node_modules")
	}
	if m.IncludesDir(".git") || m.Includes(".git/HEAD") || m.Includes(".nerd/config.json") {
		t.Fatal(".git and .nerd are always excluded outside git too")
	}

	empty, err := Open(root, []string{})
	if err != nil {
		t.Fatal(err)
	}
	if !empty.Includes("node_modules/a.js") || !empty.IncludesDir("node_modules") {
		t.Fatal("an explicit empty pattern list does not apply the defaults")
	}
	if empty.IncludesDir(".git") || empty.IncludesDir(".nerd") {
		t.Fatal("empty patterns still exclude .git and .nerd")
	}

	files := m.Files()
	sawKeep, sawHidden, sawNode := false, false, false
	for _, f := range files {
		switch f {
		case "keep.go":
			sawKeep = true
		case ".hidden/h.go":
			sawHidden = true
		case "node_modules/a.js":
			sawNode = true
		}
	}
	if !sawKeep || !sawHidden || sawNode {
		t.Fatalf("Files = %v", files)
	}
}

func TestForReadsIgnorePatterns(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "seed_corpus", "a.txt"), "s\n")
	writeFile(t, filepath.Join(root, "node_modules", "a.js"), "n\n")
	writeFile(t, filepath.Join(root, "keep.txt"), "k\n")
	writeFile(t, filepath.Join(root, ".nerd", "config.json"), `{"world":{"ignore_patterns":["seed_corpus"]},"api_key":"should-not-matter"}`)

	m, err := For(root)
	if err != nil {
		t.Fatal(err)
	}
	if m.Includes("seed_corpus/a.txt") || m.IncludesDir("seed_corpus") {
		t.Fatal("seed_corpus should be excluded by the workspace file")
	}
	// A non-empty list replaces the defaults; it is not merged with them.
	// .git and .nerd are still always excluded.
	if !m.Includes("node_modules/a.js") {
		t.Fatal("node_modules is not in the user's list, so it is a member outside git")
	}
	if !m.Includes("keep.txt") {
		t.Fatal("keep.txt")
	}

	again, err := For(root)
	if err != nil {
		t.Fatal(err)
	}
	if again != m {
		t.Fatal("For should return the cached membership for the same root and patterns")
	}

	writeFile(t, filepath.Join(root, ".nerd", "config.json"), `{`)
	broken := t.TempDir()
	writeFile(t, filepath.Join(broken, ".nerd", "config.json"), `{`)
	writeFile(t, filepath.Join(broken, "node_modules", "a.js"), "n\n")
	b, err := For(broken)
	if err != nil {
		t.Fatal(err)
	}
	if b.Includes("node_modules/a.js") {
		t.Fatal("invalid JSON should use the default list")
	}
}

func TestAcceptGitPathDropsEscapes(t *testing.T) {
	t.Parallel()
	if _, _, ok := acceptGitPath("../outside.go"); ok {
		t.Fatal("../outside.go")
	}
	if _, _, ok := acceptGitPath("/abs.go"); ok {
		t.Fatal("/abs.go")
	}
	got, dir, ok := acceptGitPath("nestedrepo/")
	if !ok || !dir || got != "nestedrepo" {
		t.Fatalf("nestedrepo/: %q dir=%v ok=%v", got, dir, ok)
	}
	got, dir, ok = acceptGitPath("plain/a.go")
	if !ok || dir || got != "plain/a.go" {
		t.Fatalf("plain/a.go: %q dir=%v ok=%v", got, dir, ok)
	}
}

func TestGateAndAdmit(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "keep.txt"), "k\n")
	writeFile(t, filepath.Join(root, "node_modules", "a.js"), "n\n")
	m, err := Open(root, []string{"node_modules"})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Gate(filepath.Join(root, "keep.txt"), false); err != nil {
		t.Fatal(err)
	}
	if err := m.Gate(filepath.Join(root, "node_modules"), true); err != fs.SkipDir {
		t.Fatalf("Gate dir: %v", err)
	}
	ok, err := m.Admit(filepath.Join(root, "node_modules", "a.js"), false)
	if err != nil || ok {
		t.Fatalf("Admit file: ok=%v err=%v", ok, err)
	}
	ok, err = m.Admit(filepath.Join(root, "node_modules"), true)
	if ok || err != fs.SkipDir {
		t.Fatalf("Admit dir: ok=%v err=%v", ok, err)
	}
}

func TestDefaultIgnoresPreserveTrackedPackages(t *testing.T) {
	root := t.TempDir()
	sources := []string{"internal/build/no_hardcoded_models_test.go", "build/source.go", "src/dist/source.go", "src/target/source.go", "src/bin/source.go", "src/obj/source.go", "src/node_modules/source.go", "src/vendor/source.go"}
	outputs := []string{"build/output/generated.go", "dist/generated.go", "nested/build/generated.go", "target/generated.go", "bin/generated.go", "obj/generated.go", "node_modules/generated.go"}
	for _, rel := range append(append([]string(nil), sources...), outputs...) {
		writeFile(t, filepath.Join(root, filepath.FromSlash(rel)), "package source\n")
	}
	runGit(t, root, "init")
	runGit(t, root, append([]string{"add"}, sources...)...)
	m, err := For(root)
	if err != nil {
		t.Fatal(err)
	}
	files := m.Files()
	for _, rel := range sources {
		t.Run(rel, func(t *testing.T) {
			if !m.Includes(rel) {
				t.Errorf("tracked source %s excluded by implicit defaults", rel)
			}
			if !m.IncludesDir(filepath.ToSlash(filepath.Dir(rel))) {
				t.Errorf("tracked source directory %s pruned", rel)
			}
			if !slices.Contains(files, rel) {
				t.Errorf("Files omitted tracked source %s: %v", rel, files)
			}
		})
	}
	var opened, walked []string
	m.readDir = func(p string) ([]os.DirEntry, error) {
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil, err
		}
		opened = append(opened, filepath.ToSlash(rel))
		return os.ReadDir(p)
	}
	if err := m.Walk(context.Background(), func(rel string, d os.DirEntry) error {
		if !d.IsDir() {
			walked = append(walked, rel)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, rel := range sources {
		if !slices.Contains(walked, rel) {
			t.Errorf("Walk omitted tracked source %s", rel)
		}
	}
	for _, rel := range outputs {
		if m.Includes(rel) || slices.Contains(files, rel) || slices.Contains(walked, rel) {
			t.Errorf("output %s was admitted", rel)
		}
		dir := filepath.ToSlash(filepath.Dir(rel))
		if m.IncludesDir(dir) || slices.Contains(opened, dir) {
			t.Errorf("opened output directory %s", dir)
		}
	}
	// A filled-in copy of the defaults is still implicit unless the operator
	// actually wrote the exclusions in the workspace configuration.
	copied, err := Open(root, append([]string(nil), config.DefaultWorldConfig().IgnorePatterns...))
	if err != nil {
		t.Fatal(err)
	}
	if !copied.Includes(sources[0]) || !copied.IncludesDir("internal/build") {
		t.Fatal("copied defaults excluded a tracked source package")
	}
	// A newly tracked package must stop being hidden without reopening m.
	later := "nested/build/generated.go"
	runGit(t, root, "add", later)
	if err := m.Refresh(); err != nil {
		t.Fatal(err)
	}
	if !m.Includes(later) || !m.IncludesDir("nested/build") || !slices.Contains(m.Files(), later) {
		t.Fatal("refresh did not update tracked exceptions")
	}
	runGit(t, root, "rm", "--cached", later)
	if err := m.Refresh(); err != nil {
		t.Fatal(err)
	}
	if m.Includes(later) || m.IncludesDir("nested/build") {
		t.Fatal("refresh retained an untracked exception")
	}
}

func TestMembershipDefaultPatternProvenance(t *testing.T) {
	defaults := config.DefaultWorldConfig().IgnorePatterns
	encoded, err := json.Marshal(map[string]any{"world": map[string]any{"ignore_patterns": defaults}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		body     string
		explicit bool
	}{
		{name: "no config file"},
		{name: "world absent", body: `{}`},
		{name: "patterns absent", body: `{"world":{}}`},
		{name: "patterns null", body: `{"world":{"ignore_patterns":null}}`},
		{name: "patterns empty", body: `{"world":{"ignore_patterns":[]}}`},
		{name: "operator build exclusion", body: `{"world":{"ignore_patterns":["build"]}}`, explicit: true},
		{name: "operator wrote default list", body: string(encoded), explicit: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			const source = "internal/build/env.go"
			const keep = "internal/source.go"
			outputs := []string{"build/output/generated.go", "nested/build/generated.go"}
			for _, rel := range append([]string{source, keep}, outputs...) {
				writeFile(t, filepath.Join(root, filepath.FromSlash(rel)), "package source\n")
			}
			configPath := filepath.Join(root, ".nerd", "config.json")
			if tc.body != "" {
				writeFile(t, configPath, tc.body)
			}
			runGit(t, root, "init")
			runGit(t, root, "add", source, keep)
			cfg, err := config.LoadUserConfig(configPath)
			if err != nil {
				t.Fatal(err)
			}
			loaded := cfg.GetWorldConfig().IgnorePatterns
			if !tc.explicit && !slices.Equal(loaded, defaults) {
				t.Fatalf("config loader defaults = %v, want %v", loaded, defaults)
			}
			for _, entry := range []struct {
				name string
				open func() (*Membership, error)
			}{
				{name: "Open copied defaults", open: func() (*Membership, error) {
					return Open(root, append([]string(nil), defaults...))
				}},
				{name: "For", open: func() (*Membership, error) { return For(root) }},
				{name: "Open nil", open: func() (*Membership, error) { return Open(root, nil) }},
				{name: "Open loaded config", open: func() (*Membership, error) {
					return Open(root, append([]string(nil), loaded...))
				}},
			} {
				t.Run(entry.name, func(t *testing.T) {
					m, err := entry.open()
					if err != nil {
						t.Fatal(err)
					}
					wantSource := !tc.explicit
					if got := m.Includes(source); got != wantSource {
						t.Errorf("Includes(%q) = %t, want %t", source, got, wantSource)
					}
					if got := m.IncludesDir("internal/build"); got != wantSource {
						t.Errorf("IncludesDir(internal/build) = %t, want %t", got, wantSource)
					}
					files := m.Files()
					if got := slices.Contains(files, source); got != wantSource {
						t.Errorf("Files contains tracked source = %t, want %t", got, wantSource)
					}
					abs := filepath.Join(root, filepath.FromSlash(source))
					gateErr := m.Gate(abs, false)
					if wantSource && gateErr != nil {
						t.Errorf("Gate rejected tracked source: %v", gateErr)
					} else if !wantSource && !errors.Is(gateErr, ErrNotMember) {
						t.Errorf("Gate error = %v, want ErrNotMember", gateErr)
					}
					if admitted, err := m.Admit(abs, false); err != nil || admitted != wantSource {
						t.Errorf("Admit = (%t, %v), want (%t, nil)", admitted, err, wantSource)
					}
					var opened, walked []string
					originalReadDir := m.readDir
					t.Cleanup(func() { m.readDir = originalReadDir })
					m.readDir = func(p string) ([]os.DirEntry, error) {
						rel, err := filepath.Rel(root, p)
						if err != nil {
							return nil, err
						}
						opened = append(opened, filepath.ToSlash(rel))
						return os.ReadDir(p)
					}
					if err := m.Walk(context.Background(), func(rel string, d os.DirEntry) error {
						if !d.IsDir() {
							walked = append(walked, rel)
						}
						return nil
					}); err != nil {
						t.Fatal(err)
					}
					if got := slices.Contains(walked, source); got != wantSource {
						t.Errorf("Walk contains tracked source = %t, want %t", got, wantSource)
					}
					if !m.Includes(keep) || !slices.Contains(files, keep) || !slices.Contains(walked, keep) {
						t.Error("unrelated tracked source was excluded")
					}
					for _, rel := range outputs {
						if m.Includes(rel) || slices.Contains(files, rel) || slices.Contains(walked, rel) {
							t.Errorf("build output %s was admitted", rel)
						}
						dir := filepath.ToSlash(filepath.Dir(rel))
						if m.IncludesDir(dir) || slices.Contains(opened, dir) {
							t.Errorf("build output directory %s was opened", dir)
						}
					}
				})
			}
		})
	}
}

func TestIgnorePatternFallbackPreservesTrackedSource(t *testing.T) {
	for _, tc := range []struct {
		name     string
		body     string
		explicit bool
	}{
		{name: "world is an array", body: `{"world":[]}`},
		{name: "world is a string", body: `{"world":"invalid"}`},
		{name: "patterns missing", body: `{"world":{}}`},
		{name: "patterns null", body: `{"world":{"ignore_patterns":null}}`},
		{name: "patterns is an object", body: `{"world":{"ignore_patterns":{}}}`},
		{name: "patterns contains a number", body: `{"world":{"ignore_patterns":["build",7]}}`},
		{name: "patterns empty", body: `{"world":{"ignore_patterns":[]}}`},
		{name: "explicit exclusion", body: `{"world":{"ignore_patterns":["build"]}}`, explicit: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			const source = "internal/build/source.go"
			writeFile(t, filepath.Join(root, filepath.FromSlash(source)), "package build\n")
			writeFile(t, filepath.Join(root, "build", "generated.go"), "package generated\n")
			writeFile(t, filepath.Join(root, ".nerd", "config.json"), tc.body)
			runGit(t, root, "init")
			runGit(t, root, "add", source)
			patterns := loadIgnorePatterns(root)
			if tc.explicit {
				if !slices.Equal(patterns, []string{"build"}) {
					t.Fatalf("explicit patterns = %v, want [build]", patterns)
				}
			} else if patterns != nil {
				t.Fatalf("fallback patterns = %#v, want nil to preserve implicit-default provenance", patterns)
			}
			m, err := For(root)
			if err != nil {
				t.Fatal(err)
			}
			wantSource := !tc.explicit
			if got := m.Includes(source); got != wantSource {
				t.Errorf("Includes(%q) = %t, want %t", source, got, wantSource)
			}
			if got := m.IncludesDir("internal/build"); got != wantSource {
				t.Errorf("IncludesDir(internal/build) = %t, want %t", got, wantSource)
			}
			if got := slices.Contains(m.Files(), source); got != wantSource {
				t.Errorf("Files contains tracked source = %t, want %t", got, wantSource)
			}
			if m.Includes("build/generated.go") || m.IncludesDir("build") {
				t.Error("fallback admitted an untracked build-output tree")
			}
		})
	}
}

func TestSnapshotDropsTrackedDescendantsOfConflictedGitlink(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "keep.go"), "package keep\n")
	runGit(t, root, "init")
	runGit(t, root, "add", "keep.go")
	runGit(t, root, "commit", "-m", "fixture")
	commit := strings.TrimSpace(runGit(t, root, "rev-parse", "HEAD"))
	blob := strings.TrimSpace(runGit(t, root, "rev-parse", "HEAD:keep.go"))
	// An unmerged directory/file conflict can retain a gitlink at one stage
	// and a tracked descendant at another. Neither source admission nor its
	// ancestor exceptions may cross that repository boundary.
	index := "160000 " + commit + " 1\tbuild\n100644 " + blob + " 2\tbuild/source.go\n"
	out, code, err := gitRun(root, []byte(index), "update-index", "--index-info")
	if err != nil {
		t.Fatalf("create conflicted index: %v", err)
	}
	if code != 0 {
		t.Fatalf("create conflicted index exited %d: %s", code, out)
	}
	staged := runGit(t, root, "ls-files", "--stage")
	if !strings.Contains(staged, "160000 "+commit+" 1\tbuild\n") || !strings.Contains(staged, "100644 "+blob+" 2\tbuild/source.go\n") {
		t.Fatalf("fixture does not contain the cross-stage boundary conflict: %s", staged)
	}
	snap, err := loadSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := snap.boundaries["build"]; !ok {
		t.Fatal("gitlink did not establish a boundary")
	}
	if _, ok := snap.tracked["build/source.go"]; ok {
		t.Fatal("tracked descendant survived repository boundary filtering")
	}
	if _, ok := snap.files["build/source.go"]; ok {
		t.Fatal("member descendant survived repository boundary filtering")
	}
	if _, ok := snap.tracked["keep.go"]; !ok {
		t.Fatal("unrelated tracked source was discarded")
	}
	m, err := For(root)
	if err != nil {
		t.Fatal(err)
	}
	if m.Includes("build/source.go") || m.IncludesDir("build") || slices.Contains(m.Files(), "build/source.go") {
		t.Fatal("implicit-default exceptions exposed a descendant of the gitlink")
	}
	m.mu.Lock()
	_, leakedAncestor := m.trackedDirs["build"]
	m.mu.Unlock()
	if leakedAncestor {
		t.Fatal("boundary descendant established a tracked-directory exception")
	}
	if !m.Includes("keep.go") {
		t.Fatal("unrelated source is not readable")
	}
}

func TestMembershipCacheSeparatesImplicitDefaultsAndExplicitEmptyPatterns(t *testing.T) {
	for _, implicitFirst := range []bool{true, false} {
		name := "explicit first"
		if implicitFirst {
			name = "implicit first"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "build", "generated.go"), "package generated\n")
			var implicit, explicit *Membership
			openImplicit := func() {
				t.Helper()
				var err error
				implicit, err = For(root)
				if err != nil {
					t.Fatal(err)
				}
			}
			openExplicit := func() {
				t.Helper()
				var err error
				explicit, err = Open(root, []string{})
				if err != nil {
					t.Fatal(err)
				}
			}
			if implicitFirst {
				openImplicit()
				openExplicit()
			} else {
				openExplicit()
				openImplicit()
			}
			if implicit == explicit {
				t.Fatal("implicit defaults and an explicit empty list share cached authority")
			}
			if implicit.Includes("build/generated.go") || implicit.IncludesDir("build") {
				t.Error("implicit defaults admitted build output")
			}
			if !explicit.Includes("build/generated.go") || !explicit.IncludesDir("build") {
				t.Error("explicit empty exclusions inherited implicit build exclusions")
			}
			for _, tc := range []struct {
				patterns []string
				want     *Membership
			}{{nil, implicit}, {[]string{}, explicit}} {
				again, err := Open(root, tc.patterns)
				if err != nil {
					t.Fatal(err)
				}
				if again != tc.want {
					t.Errorf("Open(%#v) did not reuse its own cached authority", tc.patterns)
				}
			}
		})
	}
}

func TestMembershipExplicitPatternsDistinguishFilesAndTraversableDirectories(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "build"), "ordinary file, not a directory\n")
	writeFile(t, filepath.Join(root, "hidden", "keep.go"), "package keep\n")
	writeFile(t, filepath.Join(root, "hidden", "drop.go"), "package drop\n")
	m, err := Open(root, []string{"build/", "hidden", "!hidden/keep.go"})
	if err != nil {
		t.Fatal(err)
	}
	if !m.Includes("build") {
		t.Error("directory-only exclusion hid an ordinary file named build")
	}
	if !m.IncludesDir("hidden") {
		t.Error("negated descendant needs its excluded parent traversed")
	}
	if m.Includes("hidden/drop.go") {
		t.Error("traversable parent re-included an excluded sibling")
	}
	var files []string
	if err := m.Walk(context.Background(), func(rel string, d os.DirEntry) error {
		if !d.IsDir() {
			files = append(files, rel)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(files, []string{"build", "hidden/keep.go"}) {
		t.Fatalf("Walk = %v, want ordinary build file and only the re-included descendant", files)
	}
}

func TestWalkBatchesOnlyUndecidedNonExcludedPaths(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "seed.go"), "package seed\n")
	runGit(t, root, "init")
	runGit(t, root, "add", "seed.go")
	m, err := For(root)
	if err != nil {
		t.Fatal(err)
	}
	// These entries postdate the census. Walk must classify all unknown
	// siblings before delivering the first callback, but must not ask git
	// about paths already excluded by the default output heuristics.
	writeFile(t, filepath.Join(root, "a_new.go"), "package fresh\n")
	writeFile(t, filepath.Join(root, "z_new.go"), "package fresh\n")
	writeFile(t, filepath.Join(root, "fresh", "new.go"), "package fresh\n")
	writeFile(t, filepath.Join(root, "build", "generated.go"), "package generated\n")
	writeFile(t, filepath.Join(root, "dist"), "output file\n")
	var files []string
	checkedBatch := false
	if err := m.Walk(context.Background(), func(rel string, d os.DirEntry) error {
		if rel == "a_new.go" {
			checkedBatch = true
			m.mu.Lock()
			for _, sibling := range []struct {
				path string
				dir  bool
			}{{"a_new.go", false}, {"z_new.go", false}, {"fresh", true}} {
				admitted, decided := m.decided[decideKey(sibling.path, sibling.dir)]
				if !decided || !admitted {
					t.Errorf("sibling %s not batched before first callback", sibling.path)
				}
			}
			m.mu.Unlock()
		}
		if !d.IsDir() {
			files = append(files, rel)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !checkedBatch {
		t.Fatal("Walk did not reach the new file")
	}
	if !slices.Equal(files, []string{"a_new.go", "fresh/new.go", "seed.go", "z_new.go"}) {
		t.Errorf("Walk = %v, want all new source files and no output files", files)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, excluded := range []struct {
		path string
		dir  bool
	}{{"build", true}, {"dist", false}, {"build/generated.go", false}} {
		if _, asked := m.decided[decideKey(excluded.path, excluded.dir)]; asked {
			t.Errorf("Walk asked git about already-excluded path %s", excluded.path)
		}
	}
}

func TestNeedsAskUsesTrackedExceptionsWithoutACensusHit(t *testing.T) {
	root := t.TempDir()
	const source = "internal/build/source.go"
	writeFile(t, filepath.Join(root, filepath.FromSlash(source)), "package build\n")
	runGit(t, root, "init")
	runGit(t, root, "add", source)
	for _, tc := range []struct {
		name     string
		patterns []string
		wantAsk  bool
	}{
		{name: "implicit defaults yield to tracking evidence", wantAsk: true},
		{name: "explicit exclusions remain authoritative", patterns: []string{"build"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := newMembership(root, tc.patterns)
			if err != nil {
				t.Fatal(err)
			}
			// Isolate the query-admission decision from the census fast path.
			// Keep the independently loaded git tracking evidence, but make
			// these paths unknown to the member census and dynamic-answer cache.
			// With a census hit, needsAsk returns false in both implementations
			// and cannot distinguish default heuristics from explicit exclusions.
			m.files = map[string]struct{}{}
			m.dirs = map[string]struct{}{}
			for _, path := range []struct {
				rel string
				dir bool
			}{
				{source, false}, {"internal/build", true},
			} {
				if got := m.needsAsk(path.rel, path.dir); got != tc.wantAsk {
					t.Errorf("needsAsk(%q, %t) = %t, want %t: tracking evidence must exempt implicit defaults, not explicit exclusions", path.rel, path.dir, got, tc.wantAsk)
				}
			}
			// Tracking exceptions are path-specific, not permission to ask
			// git about every output path with the same directory name.
			for _, path := range []struct {
				rel string
				dir bool
			}{
				{"build/generated.go", false}, {"build", true},
			} {
				if m.needsAsk(path.rel, path.dir) {
					t.Errorf("needsAsk admitted excluded output %q", path.rel)
				}
			}
		})
	}
}
