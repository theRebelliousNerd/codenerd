package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/tools"
)

// ErrPathOutsideWorkspace is returned when a tool is asked to operate on a
// path that resolves outside the configured workspace root.
var ErrPathOutsideWorkspace = errors.New("path escapes workspace root")

// workspaceRoot returns the canonical workspace root for file-tool path
// containment checks. It prefers the CODENERD_WORKSPACE_ROOT environment
// variable (set by session startup when WorkspaceConfig.RootPath is
// configured) and falls back to the current working directory.
//
// TODO: thread the workspace root through the tool registry context so
// tools no longer rely on process-global state.
func workspaceRoot(ctx context.Context) (string, error) {
	if ctx != nil {
		if val, ok := ctx.Value(tools.CtxKeyWorkspaceRoot).(string); ok && val != "" {
			abs, err := filepath.Abs(val)
			if err != nil {
				return "", fmt.Errorf("workspace root %q from context is not a valid path: %w", val, err)
			}
			return abs, nil
		}
	}

	if root := strings.TrimSpace(os.Getenv("CODENERD_WORKSPACE_ROOT")); root != "" {
		abs, err := filepath.Abs(root)
		if err != nil {
			return "", fmt.Errorf("workspace root %q is not a valid path: %w", root, err)
		}
		return abs, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to determine workspace root: %w", err)
	}
	return cwd, nil
}

// resolveWorkspacePath cleans p and ensures the absolute result lives inside
// root. If p references a path that does not yet exist (typical for writes),
// the closest existing parent is symlink-resolved instead. Returns the
// absolute, cleaned path on success.
func resolveWorkspacePath(ctx context.Context, root, p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("path is required")
	}
	if root == "" {
		var err error
		root, err = workspaceRoot(ctx)
		if err != nil {
			return "", err
		}
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("invalid workspace root %q: %w", root, err)
	}
	// Resolve symlinks on the root when possible so comparisons are stable.
	if resolved, err := filepath.EvalSymlinks(absRoot); err == nil {
		absRoot = resolved
	}

	// A relative tool path is workspace-relative, not CWD-relative.
	//
	// filepath.Abs resolves against the process working directory. That is the
	// same thing only when the workspace IS the working directory, which is the
	// default but not the contract: -w/--workspace sets the root without
	// chdir'ing (only the dom subcommands chdir, in cmd/nerd/dom_cmd.go). So
	// `nerd -w D:\project read_file internal/foo.go` from another directory
	// resolved to <cwd>/internal/foo.go and was then rejected for escaping
	// D:\project — every relative path failed, for the flag's entire lifetime.
	var absPath string
	if filepath.IsAbs(p) {
		absPath = filepath.Clean(p)
	} else {
		absPath = filepath.Join(absRoot, p)
	}

	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// Path may not exist yet (e.g. write to a new file). Walk up to the
		// closest existing ancestor and resolve symlinks on that, then
		// re-attach the missing suffix so containment is still enforced
		// against a real, symlink-resolved parent.
		parent := absPath
		missing := ""
		for {
			next := filepath.Dir(parent)
			if next == parent {
				return "", fmt.Errorf("cannot resolve path %q: %w", p, err)
			}
			if _, statErr := os.Stat(next); statErr == nil {
				realParent, evalErr := filepath.EvalSymlinks(next)
				if evalErr != nil {
					return "", fmt.Errorf("cannot resolve parent of %q: %w", p, evalErr)
				}
				missing = strings.TrimPrefix(absPath, parent)
				resolved = filepath.Join(realParent, filepath.Base(parent)+missing)
				break
			}
			parent = next
		}
	}

	rel, err := filepath.Rel(absRoot, resolved)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrPathOutsideWorkspace, p)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("%w: %s", ErrPathOutsideWorkspace, p)
	}

	return resolved, nil
}
