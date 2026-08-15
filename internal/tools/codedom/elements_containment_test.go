package codedom

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

// get_elements and get_element return each element's file, name, signature and
// extent. An uncontained read here is an arbitrary file disclosure — the line
// tools next door were guarded and these two were not.
func TestElementTools_WhenPathEscapesWorkspace_ShouldRefuse(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	workspace := filepath.Join(base, "ws")
	outside := filepath.Join(base, "outside")
	for _, d := range []string{workspace, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	victim := filepath.Join(outside, "secret.go")
	if err := os.WriteFile(victim, []byte("package secret\n\nfunc ApiKey() string { return \"sk-live\" }\n"), 0o600); err != nil {
		t.Fatalf("seed victim: %v", err)
	}

	ctx := tools.WithWorkspaceRoot(t.Context(), workspace)

	cases := map[string]string{
		"absolute path outside root": victim,
		"dotdot traversal":           filepath.Join("..", "outside", "secret.go"),
		"backslash traversal":        `..\outside\secret.go`,
	}

	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := executeGetElements(ctx, map[string]any{"path": path})
			if err == nil {
				t.Fatalf("get_elements read outside the workspace: %q", out)
			}
			if !errors.Is(err, tools.ErrPathOutsideWorkspace) {
				t.Fatalf("expected ErrPathOutsideWorkspace, got %v", err)
			}
			if strings.Contains(out, "ApiKey") {
				t.Fatalf("get_elements leaked out-of-workspace content: %q", out)
			}

			out, err = executeGetElement(ctx, map[string]any{"path": path, "name": "ApiKey"})
			if err == nil {
				t.Fatalf("get_element read outside the workspace: %q", out)
			}
			if !errors.Is(err, tools.ErrPathOutsideWorkspace) {
				t.Fatalf("expected ErrPathOutsideWorkspace, got %v", err)
			}
		})
	}
}

func TestElementTools_WhenPathIsInsideWorkspace_ShouldSucceed(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	target := filepath.Join(workspace, "pkg", "svc.go")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte("package pkg\n\nfunc Handler() {}\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	ctx := tools.WithWorkspaceRoot(t.Context(), workspace)

	// Workspace-relative, which is how the model addresses files.
	out, err := executeGetElements(ctx, map[string]any{"path": "pkg/svc.go"})
	if err != nil {
		t.Fatalf("containment rejected a legitimate in-workspace path: %v", err)
	}
	if !strings.Contains(out, "Handler") {
		t.Fatalf("expected to find Handler, got %q", out)
	}
}

func TestElementTools_WhenPathIsSymlinkOutside_ShouldRefuse(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevation on Windows")
	}
	t.Parallel()
	base := t.TempDir()
	workspace := filepath.Join(base, "ws")
	outside := filepath.Join(base, "outside")
	for _, d := range []string{workspace, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	victim := filepath.Join(outside, "secret.go")
	if err := os.WriteFile(victim, []byte("package secret\n\nfunc ApiKey() {}\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.Symlink(victim, filepath.Join(workspace, "link.go")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	ctx := tools.WithWorkspaceRoot(t.Context(), workspace)
	if _, err := executeGetElements(ctx, map[string]any{"path": "link.go"}); !errors.Is(err, tools.ErrPathOutsideWorkspace) {
		t.Fatalf("a symlink inside the workspace read a file outside it: %v", err)
	}
}
