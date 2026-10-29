package workspace

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
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

	m, err := Open(root, append([]string(nil), config.DefaultWorldConfig().IgnorePatterns...))
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
