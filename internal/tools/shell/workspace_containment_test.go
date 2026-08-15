package shell

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

// shellWsCtx returns a context whose workspace root is dir.
//
// The shell tools now contain working_dir. Fixtures live under t.TempDir(),
// which is outside the process working directory, so a test that does not
// declare its workspace is correctly refused — that refusal is the fix.
func shellWsCtx(dir string) context.Context {
	return tools.WithWorkspaceRoot(context.Background(), dir)
}

// shellWorkspace builds a workspace with a marker file inside it and a
// directory outside it holding a marker of its own.
func shellWorkspace(t *testing.T) (root, outside string) {
	t.Helper()
	base := t.TempDir()
	root = filepath.Join(base, "ws")
	outside = filepath.Join(base, "outside")
	for _, d := range []string{root, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "inside.marker"), []byte("in\n"), 0o600); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outside, "outside.marker"), []byte("out\n"), 0o600); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	return root, outside
}

// escapeCases are the working_dir shapes that must never be accepted.
func escapeCases(outside string) map[string]string {
	return map[string]string{
		"absolute path outside root": outside,
		"dotdot traversal":           filepath.Join("..", "outside"),
		"backslash traversal":        `..\outside`,
		"filesystem root":            string(filepath.Separator),
	}
}

func TestRunCommand_WhenWorkingDirEscapesWorkspace_ShouldRefuse(t *testing.T) {
	t.Parallel()
	root, outside := shellWorkspace(t)

	for name, dir := range escapeCases(outside) {
		t.Run(name, func(t *testing.T) {
			out, err := executeRunCommand(shellWsCtx(root), map[string]any{
				"command":     "echo hello",
				"working_dir": dir,
			})
			if err == nil {
				t.Fatalf("run_command ran with working_dir=%q: %q", dir, out)
			}
			if !errors.Is(err, tools.ErrPathOutsideWorkspace) {
				t.Fatalf("expected ErrPathOutsideWorkspace for %q, got %v", dir, err)
			}
		})
	}
}

func TestBash_WhenWorkingDirEscapesWorkspace_ShouldRefuse(t *testing.T) {
	t.Parallel()
	root, outside := shellWorkspace(t)

	if _, err := executeBash(shellWsCtx(root), map[string]any{
		"script":      "pwd",
		"working_dir": outside,
	}); !errors.Is(err, tools.ErrPathOutsideWorkspace) {
		t.Fatalf("expected bash working_dir to be refused, got %v", err)
	}
}

func TestRunBuild_WhenWorkingDirEscapesWorkspace_ShouldRefuse(t *testing.T) {
	t.Parallel()
	root, outside := shellWorkspace(t)

	if _, err := executeRunBuild(shellWsCtx(root), map[string]any{
		"working_dir": outside,
		"command":     "echo build",
	}); !errors.Is(err, tools.ErrPathOutsideWorkspace) {
		t.Fatalf("expected run_build working_dir to be refused, got %v", err)
	}
}

func TestRunTests_WhenWorkingDirEscapesWorkspace_ShouldRefuse(t *testing.T) {
	t.Parallel()
	root, outside := shellWorkspace(t)

	if _, err := executeRunTests(shellWsCtx(root), map[string]any{
		"working_dir": outside,
		"command":     "echo test",
	}); !errors.Is(err, tools.ErrPathOutsideWorkspace) {
		t.Fatalf("expected run_tests working_dir to be refused, got %v", err)
	}
}

func TestGitTools_WhenWorkingDirEscapesWorkspace_ShouldRefuse(t *testing.T) {
	t.Parallel()
	root, outside := shellWorkspace(t)
	ctx := shellWsCtx(root)

	cases := map[string]func() (string, error){
		"git_diff": func() (string, error) {
			return executeGitDiff(ctx, map[string]any{"working_dir": outside})
		},
		"git_log": func() (string, error) {
			return executeGitLog(ctx, map[string]any{"working_dir": outside})
		},
		"git_operation": func() (string, error) {
			return executeGitOperation(ctx, map[string]any{"operation": "status", "working_dir": outside})
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := run(); !errors.Is(err, tools.ErrPathOutsideWorkspace) {
				t.Fatalf("%s ran outside the workspace: %v", name, err)
			}
		})
	}
}

func TestGitTools_WhenPathspecEscapesWorkspace_ShouldRefuse(t *testing.T) {
	t.Parallel()
	root, outside := shellWorkspace(t)
	ctx := shellWsCtx(root)

	if _, err := executeGitDiff(ctx, map[string]any{"path": outside}); !errors.Is(err, tools.ErrPathOutsideWorkspace) {
		t.Fatalf("git_diff accepted an out-of-workspace pathspec: %v", err)
	}
	if _, err := executeGitLog(ctx, map[string]any{"path": outside}); !errors.Is(err, tools.ErrPathOutsideWorkspace) {
		t.Fatalf("git_log accepted an out-of-workspace pathspec: %v", err)
	}
}

func TestRunCommand_WhenWorkingDirIsSiblingSharingPrefix_ShouldRefuse(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	root := filepath.Join(base, "ws")
	sibling := filepath.Join(base, "ws-evil")
	for _, d := range []string{root, sibling} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}

	// "/ws-evil" has "/ws" as a plain string prefix; only a boundary-aware
	// comparison rejects it.
	if _, err := executeRunCommand(shellWsCtx(root), map[string]any{
		"command":     "echo hello",
		"working_dir": sibling,
	}); !errors.Is(err, tools.ErrPathOutsideWorkspace) {
		t.Fatalf("sibling directory sharing a name prefix was accepted: %v", err)
	}
}

func TestRunCommand_WhenWorkingDirIsSymlinkOutside_ShouldRefuse(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevation on Windows")
	}
	t.Parallel()
	root, outside := shellWorkspace(t)

	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := executeRunCommand(shellWsCtx(root), map[string]any{
		"command":     "echo hello",
		"working_dir": "escape",
	}); !errors.Is(err, tools.ErrPathOutsideWorkspace) {
		t.Fatalf("symlinked working_dir escaped containment: %v", err)
	}
}

func TestRunCommand_WhenWorkingDirOmitted_ShouldRunInWorkspaceRootNotCwd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("relies on a POSIX shell for pwd")
	}
	t.Parallel()
	root, _ := shellWorkspace(t)

	// An omitted working_dir used to leave Cmd.Dir empty, i.e. the process
	// working directory. That is only the workspace when -w/--workspace was not
	// used, so an agent's "ls" listed a directory it was not confined to.
	out, err := executeRunCommand(shellWsCtx(root), map[string]any{"command": "pwd"})
	if err != nil {
		t.Fatalf("executeRunCommand: %v (out=%s)", err, out)
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	if strings.TrimSpace(out) != resolved {
		t.Fatalf("command ran in %q, want workspace root %q", strings.TrimSpace(out), resolved)
	}
}

func TestRunCommand_WhenWorkingDirIsInsideWorkspace_ShouldRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("relies on a POSIX shell")
	}
	t.Parallel()
	root, _ := shellWorkspace(t)
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	out, err := executeRunCommand(shellWsCtx(root), map[string]any{
		"command":     "pwd",
		"working_dir": "sub",
	})
	if err != nil {
		t.Fatalf("containment rejected a legitimate in-workspace dir: %v (out=%s)", err, out)
	}
	if !strings.HasSuffix(strings.TrimSpace(out), "sub") {
		t.Fatalf("unexpected working directory: %q", out)
	}
}
