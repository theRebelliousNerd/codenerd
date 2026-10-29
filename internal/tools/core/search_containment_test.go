package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

// wsCtx returns a context whose workspace root is dir. Search tests operate on
// t.TempDir() trees, which live outside the package directory the process is
// run from; before the containment guard existed they reached those trees
// because glob/grep imposed no root at all.
func wsCtx(dir string) context.Context {
	return tools.WithWorkspaceRoot(context.Background(), dir)
}

// seedWorkspace lays down a workspace containing one file, plus a sibling
// directory OUTSIDE the workspace holding a secret. Returns (root, outsideDir).
func seedWorkspace(t *testing.T) (string, string) {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "ws")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "inside.txt"), []byte("NEEDLE inside\n"), 0o600); err != nil {
		t.Fatalf("write inside: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("NEEDLE secret\n"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	return root, outside
}

func TestGrep_WhenPathEscapesWorkspace_ShouldRefuse(t *testing.T) {
	t.Parallel()
	root, outside := seedWorkspace(t)

	cases := map[string]string{
		"absolute path outside root": outside,
		"dotdot traversal":           filepath.Join("..", "outside"),
		"dotdot to a file":           filepath.Join("..", "outside", "secret.txt"),
		"backslash traversal":        `..\outside`,
		"root of filesystem":         string(filepath.Separator),
	}

	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := executeGrep(wsCtx(root), map[string]any{
				"pattern": "NEEDLE",
				"path":    path,
			})
			if err == nil {
				t.Fatalf("grep escaped the workspace via %q and returned: %q", path, out)
			}
			if !errors.Is(err, tools.ErrPathOutsideWorkspace) {
				t.Fatalf("expected ErrPathOutsideWorkspace for %q, got %v", path, err)
			}
			if strings.Contains(out, "secret") {
				t.Fatalf("grep leaked out-of-workspace content: %q", out)
			}
		})
	}
}

func TestGrep_WhenSiblingRootSharesPrefix_ShouldRefuse(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	root := filepath.Join(base, "ws")
	sibling := filepath.Join(base, "ws-evil")
	for _, d := range []string{root, sibling} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	if err := os.WriteFile(filepath.Join(sibling, "secret.txt"), []byte("NEEDLE evil\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	// "/ws-evil" has "/ws" as a string prefix. A prefix check without a
	// trailing separator accepts it as inside the workspace.
	out, err := executeGrep(wsCtx(root), map[string]any{
		"pattern": "NEEDLE",
		"path":    sibling,
	})
	if err == nil {
		t.Fatalf("sibling root sharing a name prefix was accepted as inside: %q", out)
	}
	if !errors.Is(err, tools.ErrPathOutsideWorkspace) {
		t.Fatalf("expected ErrPathOutsideWorkspace, got %v", err)
	}
}

func TestGrep_WhenSymlinkPointsOutside_ShouldNotFollow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevation on Windows")
	}
	t.Parallel()
	root, outside := seedWorkspace(t)

	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	// Walking the workspace must not descend through the link.
	out, err := executeGrep(wsCtx(root), map[string]any{"pattern": "NEEDLE"})
	if err != nil {
		t.Fatalf("executeGrep: %v", err)
	}
	if strings.Contains(out, "secret") {
		t.Fatalf("grep followed a symlink out of the workspace: %q", out)
	}
	if !strings.Contains(out, "inside.txt") {
		t.Fatalf("grep lost in-workspace matches: %q", out)
	}

	// And naming the link explicitly must be refused, since it resolves out.
	if _, err := executeGrep(wsCtx(root), map[string]any{
		"pattern": "NEEDLE",
		"path":    "escape",
	}); !errors.Is(err, tools.ErrPathOutsideWorkspace) {
		t.Fatalf("expected symlinked path to be refused, got %v", err)
	}
}

func TestGlob_WhenBasePathEscapesWorkspace_ShouldRefuse(t *testing.T) {
	t.Parallel()
	root, outside := seedWorkspace(t)

	for name, base := range map[string]string{
		"absolute outside":    outside,
		"dotdot":              filepath.Join("..", "outside"),
		"backslash traversal": `..\outside`,
	} {
		t.Run(name, func(t *testing.T) {
			out, err := executeGlob(wsCtx(root), map[string]any{
				"pattern":   "*.txt",
				"base_path": base,
			})
			if err == nil {
				t.Fatalf("glob escaped via base_path=%q: %q", base, out)
			}
			if !errors.Is(err, tools.ErrPathOutsideWorkspace) {
				t.Fatalf("expected ErrPathOutsideWorkspace, got %v", err)
			}
		})
	}
}

func TestGlob_WhenPatternPrefixEscapesWorkspace_ShouldRefuse(t *testing.T) {
	t.Parallel()
	root, _ := seedWorkspace(t)

	// The recursive branch splits on "**" and joins the prefix onto base_path,
	// which made the pattern itself a second, unchecked path argument.
	if _, err := executeGlob(wsCtx(root), map[string]any{
		"pattern": "../outside/**/*.txt",
	}); !errors.Is(err, tools.ErrPathOutsideWorkspace) {
		t.Fatalf("expected pattern prefix traversal to be refused, got %v", err)
	}
}

func TestGlob_WhenSimplePatternStraddlesBoundary_ShouldDropOutsideMatches(t *testing.T) {
	t.Parallel()
	root, _ := seedWorkspace(t)

	out, err := executeGlob(wsCtx(root), map[string]any{
		"pattern": "../outside/*.txt",
	})
	if err != nil {
		t.Fatalf("executeGlob: %v", err)
	}
	if strings.Contains(out, "secret") {
		t.Fatalf("glob returned out-of-workspace matches: %q", out)
	}
}

func TestGrep_WhenPathOmitted_ShouldSearchWorkspaceRootNotCwd(t *testing.T) {
	t.Parallel()
	root, _ := seedWorkspace(t)

	// The default used to be ".", i.e. the process working directory, which is
	// not the workspace whenever -w/--workspace is used without a chdir.
	out, err := executeGrep(wsCtx(root), map[string]any{"pattern": "NEEDLE"})
	if err != nil {
		t.Fatalf("executeGrep: %v", err)
	}
	if !strings.Contains(out, "inside.txt") {
		t.Fatalf("default path did not search the workspace root: %q", out)
	}
}

func TestGrep_WhenMatchFound_ShouldReportWorkspaceRelativePath(t *testing.T) {
	t.Parallel()
	root, _ := seedWorkspace(t)
	sub := filepath.Join(root, "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "svc.go"), []byte("// NEEDLE here\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	out, err := executeGrep(wsCtx(root), map[string]any{"pattern": "NEEDLE"})
	if err != nil {
		t.Fatalf("executeGrep: %v", err)
	}
	// Containment resolves every search root to an absolute path; echoing
	// those back would put the same long prefix on every result line.
	if !strings.Contains(out, "pkg/svc.go:") {
		t.Fatalf("expected a workspace-relative path in %q", out)
	}
	if strings.Contains(out, root) {
		t.Fatalf("output leaked the absolute workspace prefix: %q", out)
	}
}
