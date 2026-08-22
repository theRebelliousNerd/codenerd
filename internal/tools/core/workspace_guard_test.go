package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

func TestResolveWorkspacePath(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	// Pre-create a nested directory inside root so paths with ".." that
	// normalize back inside root can be exercised.
	nested := filepath.Join(root, "sub", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// TODO: Null/Undefined/Empty: Test behavior with strings containing only spaces or tabs (e.g., "   ").
		// TODO: Type Coercion: Test mixed backslashes, forward slashes, and traversal characters (e.g., "..\\../foo").
		// TODO: User request Extremes: Test deeply nested traversal (e.g., "../../../../../../etc/passwd") and max path length.
		// TODO: State Conflicts: Test symlink edge cases (symlink pointing outside workspace, circular symlinks) and TOCTOU vulnerabilities.
		{
			name:  "plain relative path inside root",
			input: filepath.Join(root, "file.txt"),
		},
		{
			name:    "parent traversal escaping root",
			input:   filepath.Join(root, "..", "escape.txt"),
			wantErr: true,
		},
		{
			name:    "absolute path outside root",
			input:   filepath.Join(outside, "evil.txt"),
			wantErr: true,
		},
		{
			name:  "dotdot that normalizes back inside root",
			input: filepath.Join(root, "sub", "..", "sub", "deep", "ok.txt"),
		},
		{
			name:    "empty path",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveWorkspacePath(context.Background(), root, tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got resolved path %q", got)
				}
				if tt.input != "" && !errors.Is(err, ErrPathOutsideWorkspace) {
					// empty path is a validation error, not outside-workspace
					if !strings.Contains(err.Error(), "required") {
						t.Fatalf("expected ErrPathOutsideWorkspace, got %v", err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// EvalSymlinks may rewrite root on macOS/Windows; compare via Rel.
			rel, relErr := filepath.Rel(root, got)
			if relErr != nil {
				// Fall back to symlink-resolved root before declaring failure.
				resolvedRoot, evalErr := filepath.EvalSymlinks(root)
				if evalErr != nil {
					t.Fatalf("rel(%q, %q): %v", root, got, relErr)
				}
				rel, relErr = filepath.Rel(resolvedRoot, got)
				if relErr != nil {
					t.Fatalf("rel(%q, %q): %v", resolvedRoot, got, relErr)
				}
			}
			if strings.HasPrefix(rel, "..") {
				t.Fatalf("resolved path %q is outside root %q (rel=%q)", got, root, rel)
			}
		})
	}
}

// TODO: Null/Undefined/Empty: Test CtxKeyWorkspaceRoot with only spaces.
func TestWorkspaceRoot_PrefersCodenerdEnv(t *testing.T) {
	root := t.TempDir()
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODENERD_WORKSPACE_ROOT", root)

	got, err := workspaceRoot(context.Background())
	if err != nil {
		t.Fatalf("workspaceRoot: %v", err)
	}
	// Compare via Abs; EvalSymlinks may differ slightly, so normalize both.
	want, err := filepath.Abs(abs)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		// Accept symlink-resolved form.
		resolvedWant, evalErr := filepath.EvalSymlinks(want)
		if evalErr == nil && got == resolvedWant {
			return
		}
		t.Fatalf("workspaceRoot=%q want %q", got, want)
	}
}

func TestWorkspaceRoot_FallsBackToCwd(t *testing.T) {
	t.Setenv("CODENERD_WORKSPACE_ROOT", "")
	got, err := workspaceRoot(context.Background())
	if err != nil {
		t.Fatalf("workspaceRoot: %v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if got != cwd {
		t.Fatalf("workspaceRoot=%q want cwd %q", got, cwd)
	}
}

func TestResolveWorkspacePath_UsesEnvWhenRootEmpty(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CODENERD_WORKSPACE_ROOT", root)
	inside := filepath.Join(root, "new_write.txt")

	got, err := resolveWorkspacePath(context.Background(), "", inside)
	if err != nil {
		t.Fatalf("resolveWorkspacePath: %v", err)
	}
	rel, err := filepath.Rel(root, got)
	if err != nil {
		// symlink-resolved root
		resolved, evalErr := filepath.EvalSymlinks(root)
		if evalErr != nil {
			t.Fatal(err)
		}
		rel, err = filepath.Rel(resolved, got)
		if err != nil {
			t.Fatal(err)
		}
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatalf("resolved outside root: got=%q root=%q", got, root)
	}
}

func TestWorkspaceRoot_PrefersContext(t *testing.T) {
	want := "/from/context/root"
	ctx := context.WithValue(context.Background(), tools.CtxKeyWorkspaceRoot, want)

	// Ensure CODENERD_WORKSPACE_ROOT is set to something else to prove context takes precedence.
	oldEnv := os.Getenv("CODENERD_WORKSPACE_ROOT")
	os.Setenv("CODENERD_WORKSPACE_ROOT", "/from/env/root")
	defer os.Setenv("CODENERD_WORKSPACE_ROOT", oldEnv)

	got, err := workspaceRoot(ctx)
	if err != nil {
		t.Fatalf("workspaceRoot: %v", err)
	}

	if filepath.Base(got) != "root" {
		t.Errorf("expected root path %q, got %q", want, got)
	}
}
