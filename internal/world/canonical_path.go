package world

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Path identity contract for the world model.
//
// Every fact the world subsystem emits, and every LocalStore row it keys,
// identifies a file by its CANONICAL path: workspace-relative, forward-slash
// separated, cleaned. Absolute paths are never fact identities — they make the
// knowledge store machine- and checkout-location dependent, so a restored
// session or a repo moved to another machine matches nothing.
//
// Three producers must agree on that identity or the same file acquires two
// identities and nothing joins:
//
//	full scan       (Scanner.ScanDirectory)
//	incremental     (Scanner.ScanWorkspaceIncremental)
//	deep scan       (EnsureDeepFacts / Cartographer)
//
// They disagreed before: the incremental scanner passed the raw walk path
// (absolute) to the AST parsers while emitting file_topology with the relative
// path, so symbol_graph/dependency_link facts keyed an absolute path that no
// file_topology row ever matched; and it keyed LocalStore rows by the absolute
// path while the full scan keyed them by the relative one, so the retraction
// lookup on the next delta always missed and stale facts accumulated forever.
// CanonicalPath is the single definition; the property test in
// canonical_path_test.go fails if a producer drifts from it again.

// CanonicalPath returns the canonical (workspace-relative, forward-slash)
// identity of path.
//
// path may be absolute or already workspace-relative. A relative path is
// interpreted as ALREADY being relative to root — that is the form facts and
// store rows carry, so re-canonicalizing an identity is idempotent, which is
// what lets callers apply it defensively at any layer.
//
// If path is absolute and lies outside root, the absolute path is returned with
// separators normalized. That is still strictly more portable than a
// backslash-laden Windows path, and callers that care (the scanners) never walk
// outside root anyway.
func CanonicalPath(root, path string) string {
	p := toSlashAlways(path)
	if p == "" {
		return ""
	}
	if !isAbsSlash(p) {
		return cleanSlash(p)
	}
	if rel, err := filepath.Rel(root, path); err == nil {
		relSlash := toSlashAlways(rel)
		if !strings.HasPrefix(relSlash, "../") && relSlash != ".." {
			return cleanSlash(relSlash)
		}
	}
	return cleanSlash(p)
}

// ResolveWorkspacePath returns a filesystem path usable for os.Stat/os.ReadFile
// from a canonical (workspace-relative) identity. Canonical identities are not
// openable unless the process happens to be chdir'd into the workspace, which
// is exactly the assumption that made deep-scan caching silently no-op when it
// was not true.
func ResolveWorkspacePath(root, canonical string) string {
	if canonical == "" {
		return root
	}
	if isAbsSlash(toSlashAlways(canonical)) {
		return filepath.FromSlash(canonical)
	}
	if root == "" {
		return filepath.FromSlash(canonical)
	}
	return filepath.Join(root, filepath.FromSlash(canonical))
}

// canonicalScanPath is the scanner-facing spelling of CanonicalPath. It exists
// so scan call sites read as "canonical path of this walk entry".
func canonicalScanPath(root, p string) string {
	return CanonicalPath(root, p)
}

// toSlashAlways normalizes path separators on every platform.
//
// Deliberately NOT filepath.ToSlash: that converts os.PathSeparator, so it is a
// no-op off Windows and a Windows-shaped path arriving on a Linux host (from a
// config file, a remote worker, or a restored session) kept its backslashes and
// produced a second fact identity for one file.
func toSlashAlways(p string) string {
	return strings.ReplaceAll(p, `\`, "/")
}

// isAbsSlash reports whether a slash-normalized path is absolute on either
// POSIX ("/x") or Windows ("C:/x", "//host/share").
func isAbsSlash(p string) bool {
	if strings.HasPrefix(p, "/") {
		return true
	}
	return len(p) >= 3 && p[1] == ':' && p[2] == '/'
}

// cleanSlash cleans a slash-separated path without letting filepath's
// OS-specific separator rules back in.
func cleanSlash(p string) string {
	c := path.Clean(p)
	if c == "." {
		return "."
	}
	return c
}

// canonicalDir returns the canonical directory identity of a canonical file
// path. file_dir is a join key against directory/file_topology, so it has to be
// produced by the same slash-only rules rather than by filepath.Dir, which
// splits on os.PathSeparator.
func canonicalDir(canonical string) string {
	d := path.Dir(cleanSlash(canonical))
	if d == "" {
		return "."
	}
	return d
}

// workspaceRootOrCwd returns root, defaulting to the process working directory.
// Deep-scan callers that predate the root-aware API pass no root; before this
// existed they implicitly depended on the process being chdir'd to the
// workspace, and produced non-canonical identities whenever it was not.
func workspaceRootOrCwd(root string) string {
	if root != "" {
		return root
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}
