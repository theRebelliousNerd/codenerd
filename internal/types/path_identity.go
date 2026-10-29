package types

import (
	"path"
	"path/filepath"
	"strings"
)

// Path identity of a file inside a Fact.
//
// Every fact that names a file — file_topology, symbol_graph, dependency_link,
// the deep code_* predicates, modified, active_file, file_in_scope,
// code_element, and every diagnostic keyed by a file — identifies it by its
// CANONICAL path: workspace-relative, forward-slash separated, cleaned.
// Absolute paths are never fact identities: they make the knowledge store
// machine- and checkout-location dependent, and two producers that spell the
// same file differently give it two identities that no rule can join.
//
// The helpers live here, in the leaf package every producer already imports,
// so the world scanners, the CodeDOM scope, the file editor and the
// VirtualStore all use one definition. They used to live in internal/world,
// which internal/core and internal/tactile cannot import, and those two
// packages kept emitting absolute paths for exactly that reason.

// CanonicalPath returns the canonical (workspace-relative, forward-slash)
// identity of p.
//
// p may be absolute or already workspace-relative. A relative path is
// interpreted as ALREADY being relative to root — that is the form facts and
// store rows carry, so re-canonicalizing an identity is idempotent, which is
// what lets callers apply it defensively at any layer.
//
// If p is absolute and lies outside root, the absolute path is returned with
// separators normalized. That is still strictly more portable than a
// backslash-laden Windows path, and callers that care (the scanners) never
// walk outside root anyway. The relativisation runs on the slash-normalised
// forms: on a POSIX host a backslash is an ordinary character, so a
// Windows-shaped root and path would otherwise never share a prefix.
func CanonicalPath(root, p string) string {
	s := toSlashAlways(p)
	if s == "" {
		return ""
	}
	if !isAbsSlash(s) {
		return cleanSlash(s)
	}
	if rel, err := filepath.Rel(toSlashAlways(root), s); err == nil {
		relSlash := toSlashAlways(rel)
		if !strings.HasPrefix(relSlash, "../") && relSlash != ".." {
			return cleanSlash(relSlash)
		}
	}
	return cleanSlash(s)
}

// ResolveWorkspacePath returns a filesystem path usable for os.Stat/os.ReadFile
// from a canonical (workspace-relative) identity. Canonical identities are not
// openable unless the process happens to be chdir'd into the workspace, which
// is exactly the assumption that made deep-scan caching silently no-op when it
// was not true. An absolute input is returned in the host's separator form.
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

// SlashClean normalises separators to forward slashes on every platform and
// cleans the result without letting the host's separator rules back in. It is
// the spelling test for an identity that is already canonical: a path is
// canonical iff SlashClean leaves it unchanged and it is not absolute.
func SlashClean(p string) string {
	return cleanSlash(toSlashAlways(p))
}

// RelabelPathArgs rewrites every string argument equal to from into to, on a
// copy of facts. A parser or extractor is given a path it can open and has no
// notion of workspace-relative identity; the producer that does relabels its
// output so the facts key the same file as everything emitted beside them.
func RelabelPathArgs(facts []Fact, from, to string) []Fact {
	if from == to || len(facts) == 0 {
		return facts
	}
	out := make([]Fact, 0, len(facts))
	for _, f := range facts {
		args := make([]any, len(f.Args))
		copy(args, f.Args)
		for i, a := range args {
			if s, ok := a.(string); ok && s == from {
				args[i] = to
			}
		}
		out = append(out, Fact{Predicate: f.Predicate, Args: args})
	}
	return out
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
