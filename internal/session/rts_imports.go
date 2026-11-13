package session

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Static regression test selection, part 1b: an import change.
//
// An import changes nothing a call edge can see: what it can change is which
// packages a binary links and the order they initialize in, and a package's
// initialization (its package-level vars, its init functions) runs before
// every test of every binary that links it -- a registry an init fills, a
// flag it defines, a default it sets. So an import change is body-only
// exactly when every binary that links the written package links the same
// packages and initializes them in the same order as before.
//
// Both hold when every package-level import edge the turn added or removed is
// redundant: its target is reachable from the package through another of its
// direct imports. Then the transitive closure -- the set of packages linked --
// is unchanged, and so is the initialization order, because a package is
// initialized once all its direct imports are, and a redundant import is
// initialized before another direct import that already reaches it. (Removing
// redundant edges of a DAG one at a time keeps the rest redundant: a path that
// relied on one would close a cycle.) An import of a package nothing linked
// before, whose initialization may have side effects, is not redundant; the
// gate then runs every importer test, as for any other change it cannot
// follow.
//
// Live 2026-11-12 adding runtime, sort, sync, weak and a second mangle-go
// package to internal/core/kernel_query.go -- all already linked by
// internal/core -- ran the tests of all 32 importer packages.
//
// Names are the other half. A file that both removes and adds imports can
// give a name its body uses a different package without changing a byte of
// the body ("a/rand" for "b/rand"), and a dot import puts names in scope no
// selector shows; either is refused here. A pure addition or removal cannot:
// the compiler refuses a body whose name lost its import or collides with a
// new one, and the gate runs after the build passed.

// importSpec is one import of a file: its local name ("" when the package's
// own name is used, "_" or "." for blank and dot imports) and its path.
type importSpec struct {
	name, path string
}

// importEdit is a written file's import change: the import paths before and
// after the turn.
type importEdit struct {
	// File is the absolute, symlink-resolved path, as bodyChanges keys files.
	File          string
	Before, After []string
}

// parseImportSpec reads one entry of goDecls.imports ("alias path" or "path").
func parseImportSpec(entry string) importSpec {
	if name, path, ok := strings.Cut(entry, " "); ok {
		return importSpec{name: name, path: path}
	}
	return importSpec{path: entry}
}

// diffImports compares a file's imports before and after the turn: nil when
// they are the same, the change when it is a pure addition or removal of
// non-dot imports, or why it cannot be body-only.
func diffImports(before, after []string) (*importEdit, string) {
	if slices.Equal(before, after) {
		return nil, ""
	}
	old := map[string]bool{}
	for _, e := range before {
		old[e] = true
	}
	cur := map[string]bool{}
	for _, e := range after {
		cur[e] = true
	}
	var added, removed bool
	for _, e := range after {
		if !old[e] {
			added = true
			if parseImportSpec(e).name == "." {
				return nil, "the imports changed: a dot import was added"
			}
		}
	}
	for _, e := range before {
		if !cur[e] {
			removed = true
			if parseImportSpec(e).name == "." {
				return nil, "the imports changed: a dot import was removed"
			}
		}
	}
	if added && removed {
		return nil, "the imports changed: imports were both removed and added, so a name the code uses can now mean another package"
	}
	edit := &importEdit{}
	for _, e := range before {
		edit.Before = append(edit.Before, parseImportSpec(e).path)
	}
	for _, e := range after {
		edit.After = append(edit.After, parseImportSpec(e).path)
	}
	return edit, ""
}

// listedPackage is what `go list -deps -json` says of one package.
type listedPackage struct {
	ImportPath string
	Dir        string
	GoFiles    []string
	Imports    []string
	Error      *struct{ Err string }
}

// importEditsKeepLinkSet reports why the turn's import changes may change
// what a binary of a written package links or the order it initializes in,
// or "" when every package-level edge they add or remove is redundant.
func importEditsKeepLinkSet(ctx context.Context, workspace string, env []string, edits []importEdit) string {
	if len(edits) == 0 {
		return ""
	}
	byDir := map[string][]importEdit{}
	var patterns []string
	for _, e := range edits {
		dir := filepath.Dir(e.File)
		if _, seen := byDir[dir]; !seen {
			patterns = append(patterns, dir)
		}
		byDir[dir] = append(byDir[dir], e)
	}
	slices.Sort(patterns)
	args := append([]string{"list", "-e", "-deps", "-json=ImportPath,Dir,GoFiles,Imports,Error"}, patterns...)
	out, outcome, reason := runVerificationCommand(ctx, goWorkspace(workspace), env, buildVerifyTimeout, "go", args, verifyBuildRunner)
	if outcome != VerifyPassed {
		return "the imports changed and their effect on what links could not be listed (" + string(outcome) + suffixed(reason) + ")"
	}
	graph := map[string]listedPackage{}
	byListedDir := map[string]listedPackage{}
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var p listedPackage
		if err := dec.Decode(&p); err != nil {
			return "the imports changed and go list -deps did not decode (" + err.Error() + ")"
		}
		graph[p.ImportPath] = p
		if p.Dir != "" {
			byListedDir[resolveSymlinks(p.Dir)] = p
		}
	}
	for _, dir := range patterns {
		pkg, ok := byListedDir[resolveSymlinks(dir)]
		if !ok || pkg.Error != nil {
			return "the imports changed in " + dir + ", which go list did not load"
		}
		why := redundantImportEdits(pkg, byDir[dir], graph)
		if why != "" {
			return why
		}
	}
	return ""
}

// redundantImportEdits checks one written package: the package-level import
// edges its edited files add or remove, each of which must be reachable
// through another of the package's direct imports after the turn.
func redundantImportEdits(pkg listedPackage, edits []importEdit, graph map[string]listedPackage) string {
	included := map[string]bool{}
	for _, f := range pkg.GoFiles {
		included[f] = true
	}
	edited := map[string]importEdit{}
	for _, e := range edits {
		edited[filepath.Base(e.File)] = e
	}
	before, after := map[string]bool{}, map[string]bool{}
	for _, f := range pkg.GoFiles {
		if e, ok := edited[f]; ok {
			for _, p := range e.Before {
				before[p] = true
			}
			for _, p := range e.After {
				after[p] = true
			}
			continue
		}
		paths, err := fileImportPaths(filepath.Join(pkg.Dir, f))
		if err != "" {
			return "the imports changed in " + pkg.ImportPath + " and " + f + " " + err
		}
		for _, p := range paths {
			before[p], after[p] = true, true
		}
	}
	// reaches is whether target is reachable from one of the package's
	// direct imports after the turn other than target itself.
	reaches := func(target string) bool {
		seen := map[string]bool{}
		var stack []string
		for d := range after {
			if d != target {
				stack = append(stack, d)
			}
		}
		for len(stack) > 0 {
			p := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if p == target {
				return true
			}
			if seen[p] {
				continue
			}
			seen[p] = true
			stack = append(stack, graph[p].Imports...)
		}
		return false
	}
	var changed []string
	for p := range after {
		if !before[p] {
			changed = append(changed, p)
		}
	}
	for p := range before {
		if !after[p] {
			changed = append(changed, p)
		}
	}
	slices.Sort(changed)
	for _, p := range changed {
		if _, known := graph[p]; !known && !after[p] {
			// A removed import nothing lists any more is linked no more.
			return fmt.Sprintf("the imports changed: %s no longer imports %s, which nothing else it imports links, so its initialization leaves every importer's binary", pkg.ImportPath, p)
		}
		if !reaches(p) {
			verb := "now imports"
			if !after[p] {
				verb = "no longer imports"
			}
			return fmt.Sprintf("the imports changed: %s %s %s, which none of its other imports links, so every importer's binary initializes a different set of packages", pkg.ImportPath, verb, p)
		}
	}
	return ""
}

// fileImportPaths is one source file's import paths, or why it cannot be read.
func fileImportPaths(path string) ([]string, string) {
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly|parser.SkipObjectResolution)
	if err != nil {
		return nil, "does not parse (" + err.Error() + ")"
	}
	var out []string
	for _, is := range f.Imports {
		p, err := strconv.Unquote(is.Path.Value)
		if err != nil {
			return nil, "has an unreadable import path " + is.Path.Value
		}
		out = append(out, p)
	}
	return out, ""
}
