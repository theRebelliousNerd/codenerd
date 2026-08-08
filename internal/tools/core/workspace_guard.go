package core

import (
	"context"

	"codenerd/internal/tools"
)

// The containment implementation moved to internal/tools so every tool family
// can share exactly one copy.
//
// It used to live here, unexported, which meant it protected only the five
// tools in file_ops.go. codeNERD's own security review of that file found the
// consequence: internal/tools/codedom/lines.go — edit_lines, insert_lines,
// delete_lines — called raw os.ReadFile/os.WriteFile on a caller-supplied path
// at all six of its I/O sites, with no root, no symlink resolution and no
// escape check. An absolute path also defeats the constitution's
// path_traversal_protection rule, which greps for a literal "..".
//
// These aliases keep the existing call sites and their tests unchanged.

// ErrPathOutsideWorkspace is returned when a tool is asked to operate on a
// path that resolves outside the configured workspace root.
var ErrPathOutsideWorkspace = tools.ErrPathOutsideWorkspace

func workspaceRoot(ctx context.Context) (string, error) {
	return tools.WorkspaceRoot(ctx)
}

func resolveWorkspacePath(ctx context.Context, root, p string) (string, error) {
	return tools.ResolveWorkspacePath(ctx, root, p)
}
