package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
func workspaceRoot() (string, error) {
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
func resolveWorkspacePath(root, p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("path is required")
	}
	if root == "" {
		var err error
		root, err = workspaceRoot()
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

	absPath, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("invalid path %q: %w", p, err)
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
