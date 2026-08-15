package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrPathOutsideWorkspace is returned when a tool is asked to operate on a
// path that resolves outside the configured workspace root.
var ErrPathOutsideWorkspace = errors.New("path escapes workspace root")

// WorkspaceRoot returns the canonical workspace root for file-tool path
// containment checks, in descending order of authority:
//
//  1. the value carried on the context (CtxKeyWorkspaceRoot), which
//     Registry.ExecuteTool injects from the registry's configured root;
//  2. the CODENERD_WORKSPACE_ROOT environment variable;
//  3. the process working directory.
//
// (1) exists because the env variable is process-global: two registries in the
// same process — the VirtualStore one and tools.Global() — could not be given
// different roots, and any code that shelled out or reassigned the variable
// silently moved the containment boundary for every tool at once. Registry
// .SetWorkspaceRoot makes the root a property of the registry that owns the
// tool, and the context is how it reaches an ExecuteFunc that only receives
// (ctx, args). The env fallback is retained for callers that reach a tool's
// Execute directly without going through a registry.
func WorkspaceRoot(ctx context.Context) (string, error) {
	if ctx != nil {
		if val, ok := ctx.Value(CtxKeyWorkspaceRoot).(string); ok && val != "" {
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

// WithWorkspaceRoot returns a context carrying root as the workspace root for
// every tool executed with it. Prefer Registry.SetWorkspaceRoot; this is for
// call sites that invoke a Tool.Execute directly.
func WithWorkspaceRoot(ctx context.Context, root string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(root) == "" {
		return ctx
	}
	return context.WithValue(ctx, CtxKeyWorkspaceRoot, root)
}

// normalizeSeparators rewrites backslashes to forward slashes on every
// platform before a path is used in a containment decision.
//
// filepath.ToSlash and filepath.Clean are the trap here: both are separator
// aware, which means on Linux they treat a backslash as an ordinary filename
// character and leave it alone. `..\..\etc\passwd` therefore survives Clean
// intact, is not recognized as traversal, and lands as a single oddly named
// file inside the workspace — or, for a gate that pattern-matches path
// segments (nerd.md's write protection), as a segment that never matches
// ".nerd" and so is never protected. That exact no-op let `.nerd\config.json`
// through the write gate on Linux.
//
// Rewriting unconditionally cannot open an escape: containment is decided
// after this, by resolving to absolute and comparing against the root. The
// only thing it gives up is addressing a file whose name genuinely contains a
// backslash on a POSIX filesystem, which is not a case any tool caller has.
func normalizeSeparators(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

// containedIn reports whether path (already absolute and cleaned) is root
// itself or lives beneath it.
//
// The trailing separator is what makes this correct: a bare
// strings.HasPrefix(path, root) accepts "/ws-evil/secret" as inside "/ws",
// because the string "/ws" is a prefix of it. Comparing against root plus a
// separator forces the match to land on a path boundary.
func containedIn(root, path string) bool {
	if root == path {
		return true
	}
	sep := string(filepath.Separator)
	prefix := strings.TrimSuffix(root, sep) + sep
	return strings.HasPrefix(path, prefix)
}

// ResolveWorkspacePath cleans p and ensures the absolute result lives inside
// root. If p references a path that does not yet exist (typical for writes),
// the closest existing parent is symlink-resolved instead. Returns the
// absolute, cleaned path on success.
func ResolveWorkspacePath(ctx context.Context, root, p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("path is required")
	}
	if root == "" {
		var err error
		root, err = WorkspaceRoot(ctx)
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
	//
	// The separator rewrite happens before the IsAbs test on purpose: on Linux
	// `C:\Users\x` and `..\..\etc` are not absolute and not traversal until the
	// backslashes are normalized, so both would otherwise be joined onto the
	// root as literal filenames and accepted.
	slashed := normalizeSeparators(p)
	// On Windows a leading separator means the root of the current drive and
	// filepath.IsAbs does not report it as absolute, so without this the
	// path would be joined onto the workspace root and silently accepted.
	if !filepath.IsAbs(slashed) && strings.HasPrefix(slashed, "/") {
		return "", fmt.Errorf("%w: %q is rooted at the filesystem root, not the workspace", ErrPathOutsideWorkspace, p)
	}
	var absPath string
	if filepath.IsAbs(slashed) {
		absPath = filepath.Clean(slashed)
	} else {
		absPath = filepath.Join(absRoot, slashed)
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

	resolved = filepath.Clean(resolved)

	// Two independent containment checks. Rel answers "how do I walk from root
	// to path", which is the precise question; containedIn is a literal
	// boundary-aware prefix test. They should never disagree, and requiring
	// both means a bug in either one fails closed rather than open.
	rel, err := filepath.Rel(absRoot, resolved)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrPathOutsideWorkspace, p)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("%w: %s", ErrPathOutsideWorkspace, p)
	}
	if !containedIn(absRoot, resolved) {
		return "", fmt.Errorf("%w: %s", ErrPathOutsideWorkspace, p)
	}

	return resolved, nil
}

// ResolveWorkspaceDir is ResolveWorkspacePath for a directory argument that is
// allowed to be empty or "." — both mean "the workspace root". Tools that take
// a working directory or a search base use it so that omitting the argument
// lands inside the workspace instead of wherever the process happens to be
// chdir'd.
func ResolveWorkspaceDir(ctx context.Context, root, p string) (string, error) {
	if root == "" {
		var err error
		root, err = WorkspaceRoot(ctx)
		if err != nil {
			return "", err
		}
	}
	if trimmed := strings.TrimSpace(normalizeSeparators(p)); trimmed == "" || trimmed == "." || trimmed == "./" {
		abs, err := filepath.Abs(root)
		if err != nil {
			return "", fmt.Errorf("invalid workspace root %q: %w", root, err)
		}
		if evaluated, evalErr := filepath.EvalSymlinks(abs); evalErr == nil {
			abs = evaluated
		}
		return abs, nil
	}
	return ResolveWorkspacePath(ctx, root, p)
}
