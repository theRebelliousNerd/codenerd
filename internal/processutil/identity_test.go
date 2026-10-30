package processutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Identity tells two invocations of one argv apart by directory and by
// environment, and resolves a symlinked directory to the one it names.
func TestIdentity(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	mk := func(dir string, env []string) *exec.Cmd {
		c := exec.Command("cargo", "test")
		c.Dir, c.Env = dir, env
		return c
	}
	d1, e1 := Identity(mk(root, []string{"A=1"}))
	d2, e2 := Identity(mk(nested, []string{"A=1"}))
	if d1 == d2 || e1 != e2 {
		t.Fatalf("root %q/%s vs nested %q/%s: want distinct dirs, one environment", d1, e1, d2, e2)
	}
	if _, e3 := Identity(mk(root, []string{"A=2"})); e3 == e1 {
		t.Fatal("two environments share a digest")
	}
	if _, e4 := Identity(mk(root, []string{"B=2", "A=1"})); e4 == e1 {
		t.Fatal("an extra variable did not change the digest")
	}
	if _, e5 := Identity(mk(root, []string{"A=1"})); e5 != e1 {
		t.Fatal("one environment gave two digests")
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err == nil {
		if d, _ := Identity(mk(link, nil)); d != d1 {
			t.Fatalf("symlinked dir = %q, want %q", d, d1)
		}
	}
}
