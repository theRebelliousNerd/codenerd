package workspace

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestMembershipPreservesPathWhitespace(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	writeFile(t, filepath.Join(root, " leading.go"), "package leading\n")
	writeFile(t, filepath.Join(root, ".gitignore"), " ignored.go\n")
	writeFile(t, filepath.Join(root, " ignored.go"), "package ignored\n")
	m, err := Open(root, []string{})
	if err != nil {
		t.Fatal(err)
	}
	if !m.Includes(" leading.go") || m.Includes(" ignored.go") {
		t.Fatalf("whitespace paths changed membership: %v", m.Files())
	}
	writeFile(t, filepath.Join(root, " later.go"), "package later\n")
	if !m.Includes(" later.go") {
		t.Fatal("dynamic path lost its leading space")
	}
	for _, raw := range []string{" leading.go", "trailing.go "} {
		got, _, ok := acceptGitPath(raw)
		if !ok || got != raw {
			t.Fatalf("acceptGitPath(%q) = %q, %v", raw, got, ok)
		}
	}
	if m.Includes("bad\x00name") || m.IncludesDir("bad\x00name") {
		t.Fatal("NUL path admitted")
	}
}

func TestMembershipConcurrentRefreshAndDynamicChecks(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	writeFile(t, filepath.Join(root, ".gitignore"), "ignored/\n*.log\n")
	writeFile(t, filepath.Join(root, "keep.go"), "package keep\n")
	m, err := Open(root, []string{})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "later.go"), "package later\n")
	writeFile(t, filepath.Join(root, "later.log"), "ignored\n")
	var wg sync.WaitGroup
	for n := 0; n < 4; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 4; i++ {
				if err := m.Refresh(); err != nil {
					t.Error(err)
					return
				}
				if !m.Includes("later.go") || !m.Includes("future.go") || m.Includes("later.log") || m.IncludesDir("ignored") {
					t.Error("refresh and dynamic checks disagree")
				}
			}
		}()
	}
	wg.Wait()
}

func TestMembershipRootAliasSharesAuthority(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "real")
	writeFile(t, filepath.Join(root, "keep.go"), "package keep\n")
	writeFile(t, filepath.Join(root, "ignored", "skip.go"), "package skip\n")
	alias := filepath.Join(parent, "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	m, err := Open(root, []string{"ignored"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := Open(alias, []string{"ignored"})
	if err != nil {
		t.Fatal(err)
	}
	if m != other {
		t.Fatal("aliased root created a separate membership cache")
	}
	for _, path := range []string{alias, filepath.Join(alias, "keep.go")} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if ok, err := m.Admit(path, info.IsDir()); err != nil || !ok {
			t.Fatalf("Admit(%s) = %v, %v", path, ok, err)
		}
	}
	var got []string
	if err := other.Walk(context.Background(), func(rel string, entry os.DirEntry) error {
		if !entry.IsDir() {
			got = append(got, rel)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"keep.go"}) {
		t.Fatalf("alias walk = %v", got)
	}
}

func TestMembershipNilGuardsAndPureHelpers(t *testing.T) {
	t.Parallel()
	var nilM *Membership
	if nilM.Includes("a.go") || nilM.IncludesDir("a") {
		t.Fatal("nil membership must admit nothing")
	}
	if nilM.Files() != nil {
		t.Fatal("nil membership Files must be nil")
	}
	if got := nilM.Gate("/tmp/x.go", false); got == nil {
		t.Fatal("nil membership Gate must fail")
	}
	if ok, err := nilM.Admit("/tmp/x.go", false); err != nil || ok {
		t.Fatalf("nil membership Admit = %v, %v; want false, nil", ok, err)
	}
	if err := nilM.Walk(t.Context(), func(string, os.DirEntry) error { return nil }); err == nil {
		t.Fatal("nil membership Walk must fail")
	}
	if got := pathClean("a/./b"); got != "a/b" {
		t.Errorf("pathClean(a/./b) = %q, want a/b", got)
	}
	if got := pathClean("a//b"); got != "a/b" {
		t.Errorf("pathClean(a//b) = %q, want a/b", got)
	}
	if got := pathClean("a/../b"); got != "b" {
		t.Errorf("pathClean(a/../b) = %q, want b", got)
	}
	if got := splitNUL(nil); got != nil {
		t.Errorf("splitNUL(nil) = %v, want nil", got)
	}
	if got := splitNUL([]byte("a\x00\x00b\x00")); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("splitNUL skips empty sections: %q", got)
	}
	if mode, rel, ok := parseStageRecord("100644 e69de29bb2d1d6434b8b29ae775ad8c2e48c5391 0\ta.go"); !ok || mode != "100644" || rel != "a.go" {
		t.Errorf("parseStageRecord = %q %q %v", mode, rel, ok)
	}
	if _, _, ok := parseStageRecord("no-tab-here"); ok {
		t.Error("parseStageRecord without a tab must fail")
	}
	if _, _, ok := parseStageRecord("100644 abc 0\t../outside.go"); ok {
		t.Error("parseStageRecord must refuse escapes")
	}
	bounds := map[string]struct{}{"sub": {}}
	if !underBoundary("sub/file.go", bounds) || !underBoundary("sub", bounds) || underBoundary("other.go", bounds) {
		t.Error("underBoundary must match self and descendants only")
	}
	if _, _, ok := acceptGitPath(""); ok {
		t.Error("empty git path must be refused")
	}
	if got, dirEntry, ok := acceptGitPath("a//b"); !ok || dirEntry || got != "a/b" {
		t.Errorf("acceptGitPath(a//b) = %q %v %v", got, dirEntry, ok)
	}
	for _, tc := range []struct {
		rel  string
		want string
		ok   bool
	}{
		{"", "", true},
		{".", "", true},
		{"a/./b", "a/b", true},
		{"../outside", "", false},
		{"/abs", "", false},
		{"bad\x00name", "", false},
	} {
		if got, ok := cleanRel(tc.rel); got != tc.want || ok != tc.ok {
			t.Errorf("cleanRel(%q) = %q %v, want %q %v", tc.rel, got, ok, tc.want, tc.ok)
		}
	}
	if !alwaysExcluded("a/.git/b") || !alwaysExcluded(".nerd") || alwaysExcluded("a/b") {
		t.Error("alwaysExcluded must match .git and .nerd segments only")
	}
	if parentRel("a/b/c") != "a/b" || parentRel("a") != "" || parentRel("") != "" {
		t.Error("parentRel mismatch")
	}
	dirs := ancestorDirs(map[string]struct{}{"a/b/c": {}})
	if _, ok := dirs["a/b"]; !ok {
		t.Error("ancestorDirs must contain a/b")
	}
	if _, ok := dirs["a"]; !ok {
		t.Error("ancestorDirs must contain a")
	}
	if decideKey("a", true) == decideKey("a", false) {
		t.Error("file and directory decisions must use distinct keys")
	}
	if st := statStamp(""); st.exists {
		t.Error("empty path must not stamp")
	}
	if got := cacheKey("root", nil); got == cacheKey("root", []string{}) {
		t.Error("nil and explicit empty patterns must use distinct cache keys")
	}
	if joinRel("a", "b") != "a/b" || joinRel("", "b") != "b" {
		t.Error("joinRel mismatch")
	}
	found := false
	for _, e := range gitEnv() {
		if e == "GIT_OPTIONAL_LOCKS=0" {
			found = true
		}
	}
	if !found {
		t.Error("gitEnv must force GIT_OPTIONAL_LOCKS=0")
	}
}

func TestMembershipWalkGuards(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "keep.go"), "package keep\n")
	m, err := Open(root, []string{})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Walk(t.Context(), nil); err == nil {
		t.Fatal("Walk with nil fn must fail")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := m.Walk(ctx, func(string, os.DirEntry) error { return nil }); err == nil {
		t.Fatal("Walk with a cancelled context must fail")
	}
	if m.Includes("") {
		t.Error("root is a directory, not a file")
	}
	if !m.IncludesDir("") {
		t.Error("root must be walkable")
	}
	if _, ok := m.relOf(filepath.Join(string(filepath.Separator), "definitely-outside-workspace-12345", "file.go")); ok {
		t.Error("far outside path must not resolve into the workspace")
	}
}
