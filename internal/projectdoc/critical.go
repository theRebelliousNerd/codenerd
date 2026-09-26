package projectdoc

import (
	"path"
	"strings"
)

// CriticalPaths returns the critical: list of the workspace's root nerd.md:
// the paths whose loss or breakage is catastrophic for this project. A
// workspace without nerd.md declares none. A nerd.md that does not parse is
// an error, never an empty list: a safety list that silently vanished would
// read as "nothing here is critical".
func CriticalPaths(workspace string) ([]string, error) {
	doc, err := Load(workspace)
	if err != nil || doc == nil {
		return nil, err
	}
	return append([]string(nil), doc.Spec.Critical...), nil
}

// DocPaths returns the docs: list of the workspace's root nerd.md: the paths
// whose files are documentation. A workspace without nerd.md declares none.
func DocPaths(workspace string) ([]string, error) {
	doc, err := Load(workspace)
	if err != nil || doc == nil {
		return nil, err
	}
	return append([]string(nil), doc.Spec.Docs...), nil
}

// MatchPaths reports which entry of a nerd.md path list (critical:, docs:)
// covers p. An entry is a workspace-relative directory or file
// ("internal/core") that covers everything under it, matched on whole path
// segments so "internal/corex" is not "internal/core"; or a glob ("*.mg")
// matched against the base name and the whole path. Matching ignores case and
// separator style: a safety list a different spelling walks past is not a
// safety list. p may be absolute; a directory entry matches wherever its
// segments appear.
func MatchPaths(entries []string, p string) (string, bool) {
	norm := "/" + strings.Trim(path.Clean(slashed(p)), "/") + "/"
	base := strings.ToLower(path.Base(strings.TrimSuffix(norm, "/")))
	for _, entry := range entries {
		e := strings.Trim(slashed(strings.TrimSpace(entry)), "/")
		if e == "" {
			continue
		}
		if strings.ContainsAny(e, "*?[") {
			if ok, _ := path.Match(e, base); ok {
				return entry, true
			}
			if ok, _ := path.Match(e, strings.Trim(norm, "/")); ok {
				return entry, true
			}
			continue
		}
		if strings.Contains(norm, "/"+e+"/") {
			return entry, true
		}
	}
	return "", false
}

// slashed lower-cases p and treats both separators as "/" on every platform:
// a Windows path in a model's tool call is still that path on Linux.
func slashed(p string) string {
	return strings.ToLower(strings.ReplaceAll(p, `\`, "/"))
}
