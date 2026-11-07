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
func TestIdentityNilCommandReturnsEmpty(t *testing.T) {
	dir, digest := Identity(nil)
	if dir != "" || digest != "" {
		t.Fatalf("Identity(nil) = %q/%q, want empty", dir, digest)
	}
}

func TestIdentityEmptyDirFallsBackToWorkingDirectory(t *testing.T) {
	cmd := exec.Command("cargo", "test")
	cmd.Dir = ""
	cmd.Env = []string{"A=1"}
	dir, digest := Identity(cmd)
	if dir == "" {
		t.Fatal("Identity with empty Dir returned empty dir")
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Skip("no working directory")
	}
	abs, err := filepath.Abs(wd)
	if err != nil {
		t.Fatalf("Abs(wd): %v", err)
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		abs = real
	}
	want := filepath.Clean(abs)
	if dir != want {
		t.Fatalf("empty Dir identity = %q, want working dir %q", dir, want)
	}
	if digest == "" {
		t.Fatal("empty Dir identity returned empty digest")
	}
}

func TestIdentityEnvOrderIndependent(t *testing.T) {
	// Same directory for both so only ordering varies.
	dir := t.TempDir()
	a := exec.Command("cargo", "test")
	a.Dir, a.Env = dir, []string{"A=1", "B=2"}
	b := exec.Command("cargo", "test")
	b.Dir, b.Env = dir, []string{"B=2", "A=1"}
	_, e1 := Identity(a)
	_, e2 := Identity(b)
	if e1 != e2 {
		t.Fatalf("env order changed digest: %s vs %s", e1, e2)
	}
}

func TestIdentityCleansTrailingSeparator(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	plain := exec.Command("cargo", "test")
	plain.Dir, plain.Env = nested, []string{"A=1"}
	messy := exec.Command("cargo", "test")
	messy.Dir, messy.Env = nested+string(filepath.Separator), []string{"A=1"}
	d1, _ := Identity(plain)
	d2, _ := Identity(messy)
	if d1 != d2 {
		t.Fatalf("trailing separator changed dir: %q vs %q", d1, d2)
	}
	if d1 != filepath.Clean(nested) {
		t.Fatalf("dir = %q, want cleaned %q", d1, filepath.Clean(nested))
	}
}

func TestIdentityMissingDirKeepsCleanedAbs(t *testing.T) {
	root := t.TempDir()
	ghost := filepath.Join(root, "ghost")
	cmd := exec.Command("cargo", "test")
	cmd.Dir, cmd.Env = ghost, []string{"A=1"}
	dir, digest := Identity(cmd)
	if dir == "" || digest == "" {
		t.Fatalf("missing dir identity = %q/%q, want non-empty", dir, digest)
	}
	abs, err := filepath.Abs(ghost)
	if err != nil {
		t.Fatalf("Abs(ghost): %v", err)
	}
	if want := filepath.Clean(abs); dir != want {
		t.Fatalf("missing dir = %q, want %q", dir, want)
	}
}
