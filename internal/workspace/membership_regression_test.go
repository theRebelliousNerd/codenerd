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
