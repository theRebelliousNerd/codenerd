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
