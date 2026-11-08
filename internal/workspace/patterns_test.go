package workspace

import (
	"runtime"
	"testing"
)

func TestPatterns(t *testing.T) {
	t.Parallel()
	ps := compilePatterns([]string{
		"node_modules",
		"build/",
		"/rooted/",
		"*.log",
		"src/gen/**",
		"seed/**/tmp",
		"*.go",
		"!keep.go",
		"vendor/*",
		"[abc].txt",
		"bad[",
		"",
		"!",
		"foo//bar",
	})

	cases := []struct {
		rel   string
		isDir bool
		out   bool
	}{
		{"node_modules", true, true},
		{"pkg/node_modules/a.js", false, true},
		{"not_node_modules/a.js", false, false},
		{"build", true, true},
		{"build", false, false},
		{"src/build/main.go", false, true},
		{"rooted", true, true},
		{"src/rooted/x", false, false},
		{"dir/a.log", false, true},
		{"src/gen/a/b.go", false, true},
		{"seed/a/tmp", true, true},
		{"seed/tmp", true, true},
		{"other.go", false, true},
		{"keep.go", false, false},
		{"dir/keep.go", false, false},
		{"vendor/a.go", false, true},
		{"vendor/sub/a.go", false, true},
		{"vendor", true, false},
		{"a.txt", false, true},
		{"d.txt", false, false},
		{"bad[/x", false, false},
	}
	for _, tc := range cases {
		if got := ps.excluded(tc.rel, tc.isDir); got != tc.out {
			t.Errorf("excluded(%q, dir=%v) = %v, want %v", tc.rel, tc.isDir, got, tc.out)
		}
	}

	// build/ excludes the directory, but a later negation names a file inside
	// it, so the directory has to be opened. node_modules has no such negation.
	re := compilePatterns([]string{"build/", "!build/keep.go", "node_modules"})
	if re.blocksDir("build") {
		t.Error("build should be entered so !build/keep.go can match")
	}
	if !re.excluded("build/other.go", false) {
		t.Error("build/other.go should stay excluded")
	}
	if re.excluded("build/keep.go", false) {
		t.Error("build/keep.go should be re-included")
	}
	if !re.blocksDir("node_modules") {
		t.Error("node_modules has no later negation and should not be entered")
	}

	// An unanchored negation can match in any directory, so none of the
	// excluded directories can be skipped.
	any := compilePatterns([]string{"node_modules", "!keep.go"})
	if any.blocksDir("node_modules") {
		t.Error("unanchored !keep.go can match inside node_modules")
	}
	if any.blocksDir("other") {
		t.Error("other is not excluded")
	}

	if runtime.GOOS == "windows" {
		fold := compilePatterns([]string{"node_modules"})
		if !fold.excluded("Node_Modules/a", false) {
			t.Error("windows patterns are case-folded")
		}
	} else if compilePatterns([]string{"node_modules"}).excluded("Node_Modules/a", false) {
		t.Error("patterns are case-sensitive off windows")
	}
}

func TestBlocksDirAnchor(t *testing.T) {
	t.Parallel()
	ps := compilePatterns([]string{"generated/", "!src/generated/keep.go"})
	if ps.blocksDir("src/generated") {
		t.Error("src/generated holds a negated file")
	}
	if !ps.blocksDir("other/generated") {
		t.Error("other/generated is excluded and the negation is anchored elsewhere")
	}
}

func TestPatternHelpersPinEdgeCases(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"", "!", "foo//bar", "bad[", "/"} {
		if _, ok := compilePattern(raw); ok {
			t.Errorf("compilePattern(%q) accepted an invalid pattern", raw)
		}
	}
	if p, ok := compilePattern("build/"); !ok || !p.dirOnly || p.neg {
		t.Errorf("compilePattern(build/) = %+v, %v; want dirOnly non-negated", p, ok)
	}
	if p, ok := compilePattern("!keep.go"); !ok || !p.neg || !p.unanchored {
		t.Errorf("compilePattern(!keep.go) = %+v, %v; want negated unanchored", p, ok)
	}
	if !bracketsBalanced("a[b]c") || !bracketsBalanced("plain") {
		t.Error("balanced brackets reported unbalanced")
	}
	if bracketsBalanced("a[b") {
		t.Error("unbalanced [ reported balanced")
	}
	if !matchName("*.go", "a.go", false) || matchName("*.go", "a.txt", false) {
		t.Error("matchName glob mismatch")
	}
	if !matchName("exact.go", "exact.go", false) || matchName("exact.go", "other.go", false) {
		t.Error("matchName literal mismatch")
	}
	if got := ancestorPaths("a/b/c"); len(got) != 2 || got[0] != "a/b" || got[1] != "a" {
		t.Errorf("ancestorPaths(a/b/c) = %v", got)
	}
	if len(ancestorPaths("single")) != 0 {
		t.Error("ancestorPaths(single) must be empty")
	}
	if !strictAncestor("a", "a/b", false) || strictAncestor("a/b", "a", false) || strictAncestor("a", "a", false) {
		t.Error("strictAncestor direction mismatch")
	}
	if !strictAncestor("", "a", false) {
		t.Error("root is a strict ancestor of every path")
	}
	if got := GenerationOf(nil); got != "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("GenerationOf(nil) = %q", got)
	}
	ordered := GenerationOf([]string{"b", "a"})
	if ordered != GenerationOf([]string{"a", "b"}) {
		t.Error("GenerationOf must sort its input")
	}
}
