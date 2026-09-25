package session

import (
	"path/filepath"
	"strings"

	"codenerd/internal/tools"
)

// The gates hand the go command paths -- the directory it runs in, the keys of
// an -overlay file -- and read paths back from what it prints. The go command
// does not use the workspace's spelling: on Unix it takes its working
// directory from the operating system, which has resolved every symlink on the
// way (macOS's /var is /private/var; a workspace reached through a link is its
// target), and on Windows it keeps the spelling it was started in, 8.3 short
// names included. An overlay key spelled through any other alias of the same
// file names a file go never opens: the overlay is silently ignored, the
// "before the turn" run measures the tree as it is now, and every failure the
// turn caused reads as pre-existing. That is how, under a symlinked temp
// directory, vet, the importer gate, test-failure attribution and the pinning
// gate all passed turns that broke something.
//
// So a gate speaks to the go command in one spelling only: the resolved
// workspace root (tools.CanonicalWorkspaceRoot, the same identity the tools'
// containment check uses), for the directory it runs in and for every path it
// is given.

// goWorkspace is workspace spelled the way the go command spells the files
// under it. It is idempotent, and returns workspace unchanged when it cannot
// be resolved, so a gate never loses its workspace to a failed lookup.
func goWorkspace(workspace string) string {
	ws := strings.TrimSpace(workspace)
	if ws == "" {
		return workspace
	}
	root, err := tools.CanonicalWorkspaceRoot(ws)
	if err != nil || root == "" {
		return workspace
	}
	return root
}

// goOverlayKey is the -overlay Replace key for p, a workspace-relative path
// (slash-separated) or an absolute one under any spelling of workspace: the
// file's path under the resolved root. An absolute path outside the workspace
// keeps its own spelling.
func goOverlayKey(workspace, p string) string {
	root := goWorkspace(workspace)
	if !filepath.IsAbs(p) {
		return filepath.Join(root, filepath.FromSlash(p))
	}
	if rel, ok := resolvedWorkspaceRel(root, p); ok {
		return filepath.Join(root, filepath.FromSlash(rel))
	}
	return filepath.Clean(p)
}

// pathSpellings is p and, when it differs, p with its symlinks resolved: the
// two names a printed path may carry for a file the gate created itself (an
// overlay stand-in under the temp directory).
func pathSpellings(p string) []string {
	out := []string{p}
	if resolved, err := filepath.EvalSymlinks(p); err == nil && resolved != p {
		out = append(out, resolved)
	}
	return out
}
