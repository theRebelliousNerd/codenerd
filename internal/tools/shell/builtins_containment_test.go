package shell

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Containment for the builtin fallback: every operand must resolve inside the
// workspace root. A read-only builtin that honors absolute paths, "..", or
// symlinks pointing out is a bypass around the file-tool guard wearing a
// safe label — the permission gate approved a workspace search, not the rest
// of the disk.

func containmentTree(t *testing.T) (root, ws string) {
	t.Helper()
	root = t.TempDir()
	ws = filepath.Join(root, "ws")
	sub := filepath.Join(ws, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(p, content string) {
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "secret.txt"), "TOP-SECRET-OUTSIDE\n")
	write(filepath.Join(ws, "inner.txt"), "inside-content\nsecond line\nthird line\n")
	write(filepath.Join(sub, "nested.txt"), "nested-content\n")
	return root, ws
}

func runBuiltin(t *testing.T, argv []string, root, dir string) string {
	t.Helper()
	out, handled := runBuiltinFallback(context.Background(), argv, root, dir)
	if !handled {
		t.Fatalf("%v not handled", argv)
	}
	return out
}

func TestBuiltinContain_AbsoluteOutsideRejected(t *testing.T) {
	root, ws := containmentTree(t)
	outside := filepath.Join(root, "secret.txt")
	for _, argv := range [][]string{
		{"cat", outside},
		{"head", outside},
		{"tail", outside},
		{"wc", outside},
		{"ls", outside},
		{"rg", "TOP-SECRET", outside},
	} {
		out := runBuiltin(t, argv, ws, ws)
		if strings.Contains(out, "TOP-SECRET") {
			t.Fatalf("%v leaked outside content: %q", argv, out)
		}
		if !strings.Contains(out, "secret.txt") {
			t.Errorf("%v must name the rejected operand: %q", argv, out)
		}
	}
}

func TestBuiltinContain_DotDotEscapeRejected(t *testing.T) {
	_, ws := containmentTree(t)
	out := runBuiltin(t, []string{"cat", "../secret.txt"}, ws, ws)
	if strings.Contains(out, "TOP-SECRET") {
		t.Fatalf("dotdot escape leaked: %q", out)
	}
	out = runBuiltin(t, []string{"rg", "TOP-SECRET", ".."}, ws, ws)
	if strings.Contains(out, "TOP-SECRET") {
		t.Fatalf("grep over .. leaked: %q", out)
	}
}

func TestBuiltinContain_SymlinkOutsideRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevation on Windows")
	}
	root, ws := containmentTree(t)
	if err := os.Symlink(filepath.Join(root, "secret.txt"), filepath.Join(ws, "link.txt")); err != nil {
		t.Fatal(err)
	}
	out := runBuiltin(t, []string{"cat", "link.txt"}, ws, ws)
	if strings.Contains(out, "TOP-SECRET") {
		t.Fatalf("symlink escape leaked via cat: %q", out)
	}
	// The walk is lexical: a linked file inside the tree must not be opened
	// when it resolves outside, and the skip stays silent.
	out = runBuiltin(t, []string{"rg", "TOP-SECRET"}, ws, ws)
	if strings.Contains(out, "TOP-SECRET") {
		t.Fatalf("symlink escape leaked via walk: %q", out)
	}
}

func TestBuiltinContain_SymlinkInsideAllowed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevation on Windows")
	}
	_, ws := containmentTree(t)
	if err := os.Symlink(filepath.Join(ws, "inner.txt"), filepath.Join(ws, "oklink.txt")); err != nil {
		t.Fatal(err)
	}
	out := runBuiltin(t, []string{"cat", "oklink.txt"}, ws, ws)
	if !strings.Contains(out, "inside-content") {
		t.Fatalf("in-workspace symlink must stay readable: %q", out)
	}
}

func TestBuiltinContain_SubdirWorkingDirKeepsShellSemantics(t *testing.T) {
	_, ws := containmentTree(t)
	sub := filepath.Join(ws, "sub")
	// Relative operands join onto the working dir first, then containment
	// checks the result against the root: ../inner.txt from sub/ is fine.
	out := runBuiltin(t, []string{"cat", "../inner.txt"}, ws, sub)
	if !strings.Contains(out, "inside-content") {
		t.Fatalf("in-workspace relative read broke: %q", out)
	}
	out = runBuiltin(t, []string{"cat", "../../secret.txt"}, ws, sub)
	if strings.Contains(out, "TOP-SECRET") {
		t.Fatalf("dotdot escape from subdir leaked: %q", out)
	}
	out = runBuiltin(t, []string{"ls", ".."}, ws, sub)
	if !strings.Contains(out, "inner.txt") {
		t.Fatalf("ls .. from subdir broke: %q", out)
	}
}
