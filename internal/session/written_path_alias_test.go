package session

import (
	"os"
	"path/filepath"
	"testing"
)

// A write target spelled through an alias of the workspace -- a symlink here,
// an 8.3 short name on Windows -- is the same file as its resolved spelling,
// and the turn's gates key packages by the workspace-relative path. Left
// absolute, it became the package pattern "./<absolute>" and every build and
// test gate of the turn failed to set up (CI's path-alias run: 12 session
// tests, "CreateFile ...\001\C:\Users\RUNNER~1\...: The filename, directory
// name, or volume label syntax is incorrect").
func TestCanonicalizeWrittenPath_AnAliasedTargetIsWorkspaceRelative(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.MkdirAll(filepath.Join(real, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "pkg", "x.go"), []byte("package pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Skipf("cannot create a symlink here: %v", err)
	}
	workspace, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}

	cases := map[string]string{
		filepath.Join(alias, "pkg", "x.go"):        "pkg/x.go",     // exists
		filepath.Join(alias, "pkg", "new", "y.go"): "pkg/new/y.go", // not written yet
		filepath.Join(workspace, "pkg", "x.go"):    "pkg/x.go",     // already resolved
	}
	for target, want := range cases {
		if got := canonicalizeWrittenPath(target, workspace); got != want {
			t.Errorf("canonicalizeWrittenPath(%q) = %q, want %q", target, got, want)
		}
	}
	if got := packagesForPaths([]string{canonicalizeWrittenPath(filepath.Join(alias, "pkg", "x.go"), workspace)}); len(got) != 1 || got[0] != "./pkg" {
		t.Errorf("packagesForPaths = %v, want [./pkg]", got)
	}

	// Outside the workspace stays absolute: resolving an alias must not pull
	// a foreign file in.
	outside := filepath.Join(base, "elsewhere.go")
	if got := canonicalizeWrittenPath(outside, workspace); !filepath.IsAbs(filepath.FromSlash(got)) {
		t.Errorf("canonicalizeWrittenPath(%q) = %q, want it left absolute", outside, got)
	}
}
