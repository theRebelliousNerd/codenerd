package world

import (
	"os"
	"path"

	"codenerd/internal/types"
)

// Path identity contract for the world model.
//
// Every fact the world subsystem emits, and every LocalStore row it keys,
// identifies a file by its CANONICAL path: workspace-relative, forward-slash
// separated, cleaned (types.CanonicalPath). Absolute paths are never fact
// identities — they make the knowledge store machine- and checkout-location
// dependent, so a restored session or a repo moved to another machine matches
// nothing.
//
// The producers must agree on that identity or the same file acquires two
// identities and nothing joins:
//
//	full scan       (Scanner.ScanDirectory)
//	incremental     (Scanner.ScanWorkspaceIncremental)
//	deep scan       (EnsureDeepFacts / Cartographer)
//	chat scans      (/scan-path, /scan-dir via ASTParser.ParseAs)
//	CodeDOM scope   (FileScope.ScopeFacts)
//
// They disagreed before: the incremental scanner passed the raw walk path
// (absolute) to the AST parsers while emitting file_topology with the relative
// path, so symbol_graph/dependency_link facts keyed an absolute path that no
// file_topology row ever matched; and it keyed LocalStore rows by the absolute
// path while the full scan keyed them by the relative one, so the retraction
// lookup on the next delta always missed and stale facts accumulated forever.
// The property test in canonical_path_test.go fails if a producer drifts from
// the single definition again.

// canonicalScanPath is the scanner-facing spelling of types.CanonicalPath. It
// exists so scan call sites read as "canonical path of this walk entry".
func canonicalScanPath(root, p string) string {
	return types.CanonicalPath(root, p)
}

// canonicalDir returns the canonical directory identity of a canonical file
// path. file_dir is a join key against directory/file_topology, so it has to be
// produced by the same slash-only rules rather than by filepath.Dir, which
// splits on os.PathSeparator.
func canonicalDir(canonical string) string {
	d := path.Dir(types.SlashClean(canonical))
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
