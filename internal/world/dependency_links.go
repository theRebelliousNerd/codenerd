package world

import (
	"os"
	"path"
	"sort"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// Import-edge resolution.
//
// The AST walkers emit a raw import fact per import statement:
//
//	dependency_link(File, "pkg:codenerd/internal/core", "codenerd/internal/core")
//	dependency_link(File, "mod:./widget",               "./widget")
//
// The second argument is a synthetic token, never a path, so every consumer
// rule of the shape
//
//	impacted(X)             :- dependency_link(X, Y, _), modified(Y).
//	coder_block_write(F, _) :- pending_edit(F, _), dependency_link(D, F, _), ...
//
// joined against nothing: `modified`/`pending_edit` hold file paths and no file
// is ever named "pkg:...". The entire impact/activation cascade — cross-package
// impact warnings, uncovered-impact write blocking, context spreading — was
// therefore dormant, while dependency_link sat in the world replace-set
// retracting and re-adding facts no rule could use.
//
// Resolution gives those tokens something to join with, without naming a
// path that is not one. Every source file belongs to an importable UNIT and
// every in-workspace import names a unit:
//
//	file_unit(File, Unit)      File belongs to Unit   (the member's own fact)
//	import_unit(File, Unit)    File imports Unit      (the importer's own fact)
//
// A Go file's unit is its package: the import path that reaches its directory.
// A file of a language whose import names one file (a Python module, a JS/TS
// relative specifier, a Rust module) is its own unit. "X depends on the file
// Y" is then a join the rules write themselves:
//
//	import_unit(X, U), file_unit(Y, U)
//
// Granularity is still package-level for Go: importing a package depends on
// every non-test file of it, which over-approximates in the safe direction for
// a predicate that gates writes.
//
// This used to be stored as the product: one dependency_link row, and one
// file_imports row beside it, from each importer to EVERY file of the imported
// package. The product grows with files times package size -- 46k rows when it
// was introduced, 225k of each on this repository a few months later, 78% of
// the world facts the kernel loads -- and it has to be rebuilt for every
// importer when a package gains a file (652 files rescanned for one new file
// in internal/core). The factors are about 20k import rows and one row per
// source file, and a new file is one new row. External imports keep only their
// token form in dependency_link: a truthful record of a third-party dependency
// that no rule joins.

// repoFileIndex maps the workspace file set into the shapes import resolution
// needs. Built once per scan; canonical (workspace-relative, slash) paths only.
type repoFileIndex struct {
	// goFilesByDir maps a package directory to its non-test .go files.
	goFilesByDir map[string][]string
	// all is the canonical file set, for exact-file resolution.
	all map[string]struct{}
	// goModule is the module path from go.mod, "" when absent.
	goModule string
}

func newRepoFileIndex(root string, canonicalFiles []string) *repoFileIndex {
	idx := &repoFileIndex{
		goFilesByDir: make(map[string][]string),
		all:          make(map[string]struct{}, len(canonicalFiles)),
		goModule:     ReadGoModulePath(root),
	}
	for _, f := range canonicalFiles {
		c := types.SlashClean(f)
		if c == "" || c == "." {
			continue
		}
		idx.all[c] = struct{}{}
		if strings.HasSuffix(c, ".go") && !strings.HasSuffix(c, "_test.go") {
			d := canonicalDir(c)
			idx.goFilesByDir[d] = append(idx.goFilesByDir[d], c)
		}
	}
	for d := range idx.goFilesByDir {
		sort.Strings(idx.goFilesByDir[d])
	}
	return idx
}

// ReadGoModulePath extracts the module path from <root>/go.mod, "" without one.
// Without it, Go import paths cannot be mapped to workspace directories, so
// resolution falls back to treating the import path as a literal directory
// (which is what a GOPATH-less multi-module checkout looks like).
func ReadGoModulePath(root string) string {
	data, err := os.ReadFile(types.ResolveWorkspacePath(root, "go.mod"))
	if err != nil {
		return ""
	}
	for line := range strings.Lines(string(data)) {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "module"); ok {
			mod := strings.TrimSpace(rest)
			mod = strings.Trim(mod, "\"")
			if i := strings.Index(mod, "//"); i >= 0 {
				mod = strings.TrimSpace(mod[:i])
			}
			return mod
		}
	}
	return ""
}

// ResolveDependencyLinks derives the import-graph rows (import_unit,
// imports_project_file, and file_unit for each file with a file_topology row)
// from the facts of a scan. The workspace file set is taken from the
// file_topology facts in facts, so this is the full-scan entry point; the
// incremental scanner uses resolveDependencyLinksWithIndex because its fact
// slice only covers changed files.
func ResolveDependencyLinks(root string, facts []core.Fact) []core.Fact {
	files := make([]string, 0, len(facts))
	for _, f := range facts {
		if f.Predicate == "file_topology" && len(f.Args) > 0 {
			if p, ok := f.Args[0].(string); ok {
				files = append(files, p)
			}
		}
	}
	return ResolveDependencyLinksAgainst(root, files, facts)
}

// ResolveDependencyLinksAgainst is ResolveDependencyLinks with the workspace
// file set given explicitly, as canonical paths. It is the entry point for a
// scan that covers only part of the workspace (a chat /scan-path or /scan-dir):
// an import from a file in hand into a package that was not rescanned still
// has to resolve, so the index must span every file the kernel knows, not just
// the ones whose facts are in the slice.
func ResolveDependencyLinksAgainst(root string, workspaceFiles []string, facts []core.Fact) []core.Fact {
	idx := newRepoFileIndex(root, workspaceFiles)
	return append(resolveDependencyLinksWithIndex(idx, facts), fileUnitFacts(idx, facts)...)
}

// resolveDependencyLinksWithIndex returns the importer-owned rows for the raw
// import facts in facts: one import_unit row per in-workspace unit a file
// imports, and imports_project_file once per file that has any.
func resolveDependencyLinksWithIndex(idx *repoFileIndex, facts []core.Fact) []core.Fact {
	type edge struct{ from, unit string }
	var edges []edge
	seen := make(map[edge]struct{})

	for _, f := range facts {
		if f.Predicate != "dependency_link" || len(f.Args) < 3 {
			continue
		}
		from, ok := f.Args[0].(string)
		if !ok || from == "" {
			continue
		}
		token, ok := f.Args[1].(string)
		if !ok {
			continue
		}
		imp, ok := f.Args[2].(string)
		if !ok || imp == "" {
			continue
		}
		for _, unit := range idx.importedUnits(from, token, imp) {
			e := edge{from: from, unit: unit}
			if _, dup := seen[e]; dup {
				continue
			}
			seen[e] = struct{}{}
			edges = append(edges, e)
		}
	}

	// Sorted so a full scan and an incremental scan emit the same rows in the
	// same order. Both paths share this function; an unstable order would make
	// their replace-sets disagree about a fact that both derived.
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].from != edges[j].from {
			return edges[i].from < edges[j].from
		}
		return edges[i].unit < edges[j].unit
	})

	// imports_project_file marks each importer once (import_cycles.go): a
	// per-file measurement, stored and retracted with the file's imports.
	out := make([]core.Fact, 0, len(edges)+len(edges)/4)
	for i, e := range edges {
		if i == 0 || edges[i-1].from != e.from {
			out = append(out, core.Fact{
				Predicate: "imports_project_file",
				Args:      []any{e.from},
			})
		}
		out = append(out, core.Fact{
			Predicate: "import_unit",
			Args:      []any{e.from, e.unit},
		})
	}
	return out
}

// fileUnitFact is the member-owned row for file: the unit it belongs to. A
// file that no import can name -- a Go test file, a document, a file of a
// language the resolver does not read -- belongs to none.
func fileUnitFact(idx *repoFileIndex, file string) (core.Fact, bool) {
	unit := idx.unitOfFile(types.SlashClean(file))
	if unit == "" {
		return core.Fact{}, false
	}
	return core.Fact{Predicate: "file_unit", Args: []any{types.SlashClean(file), unit}}, true
}

// fileUnitFacts is fileUnitFact for every file that has a file_topology row
// in facts: the files a scan has in hand.
func fileUnitFacts(idx *repoFileIndex, facts []core.Fact) []core.Fact {
	var out []core.Fact
	for _, f := range facts {
		if f.Predicate != "file_topology" || len(f.Args) == 0 {
			continue
		}
		file, ok := f.Args[0].(string)
		if !ok || file == "" {
			continue
		}
		if row, has := fileUnitFact(idx, file); has {
			out = append(out, row)
		}
	}
	return out
}

// goUnitOfDir is the import path that reaches the Go package in dir: the
// module path joined with the directory, or the bare directory when go.mod
// names no module (the form resolveGoPackage accepts then).
func (idx *repoFileIndex) goUnitOfDir(dir string) string {
	dir = types.SlashClean(dir)
	switch {
	case idx.goModule == "":
		return dir
	case dir == "" || dir == ".":
		return idx.goModule
	default:
		return idx.goModule + "/" + dir
	}
}

// exactUnitExtensions are the languages whose import names one file, so the
// file is its own unit.
var exactUnitExtensions = map[string]struct{}{
	".py": {}, ".js": {}, ".jsx": {}, ".ts": {}, ".tsx": {}, ".mjs": {}, ".cjs": {}, ".rs": {},
}

// unitOfFile is the unit a canonical file path belongs to, "" for none.
func (idx *repoFileIndex) unitOfFile(file string) string {
	if strings.HasSuffix(file, ".go") {
		if strings.HasSuffix(file, "_test.go") {
			return ""
		}
		return idx.goUnitOfDir(canonicalDir(file))
	}
	if _, exact := exactUnitExtensions[path.Ext(file)]; exact {
		return file
	}
	return ""
}

// importedUnits returns the in-workspace units an import token refers to, nil
// when the import leaves the workspace. A Go import is its package's unit when
// that package has files here; any other import is the file it resolves to.
// A file never imports itself.
func (idx *repoFileIndex) importedUnits(from, token, imp string) []string {
	if strings.HasPrefix(token, "pkg:") {
		members := idx.resolveGoPackage(imp)
		if len(members) == 0 {
			return nil
		}
		unit := idx.unitOfFile(members[0])
		// A member that names its own package (not Go that compiles, but a
		// scan sees whatever is on disk) would join to itself.
		if unit == idx.unitOfFile(from) {
			return nil
		}
		return []string{unit}
	}
	var units []string
	for _, target := range idx.resolve(from, token, imp) {
		if target != from {
			units = append(units, target)
		}
	}
	return units
}

// resolve returns the workspace files an import token refers to, or nil when
// the import leaves the workspace.
func (idx *repoFileIndex) resolve(from, token, imp string) []string {
	switch {
	case strings.HasPrefix(token, "pkg:"):
		return idx.resolveGoPackage(imp)
	case strings.HasPrefix(token, "mod:"):
		if strings.HasSuffix(from, ".py") {
			return idx.resolvePythonModule(from, imp)
		}
		return idx.resolveJSModule(from, imp)
	case strings.HasPrefix(token, "crate:"):
		return idx.resolveRustPath(from, imp)
	}
	return nil
}

func (idx *repoFileIndex) resolveGoPackage(importPath string) []string {
	dir := ""
	switch {
	case idx.goModule != "" && importPath == idx.goModule:
		dir = "."
	case idx.goModule != "" && strings.HasPrefix(importPath, idx.goModule+"/"):
		dir = strings.TrimPrefix(importPath, idx.goModule+"/")
	default:
		// No module line (or a foreign module path): only accept the import
		// path if it names a real workspace package directory, so stdlib and
		// third-party imports do not accidentally match.
		if _, ok := idx.goFilesByDir[importPath]; !ok {
			return nil
		}
		dir = importPath
	}
	if dir == "" {
		dir = "."
	}
	return idx.goFilesByDir[types.SlashClean(dir)]
}

func (idx *repoFileIndex) resolvePythonModule(from, module string) []string {
	rel := strings.ReplaceAll(strings.Trim(module, "."), ".", "/")
	if rel == "" {
		return nil
	}
	fromDir := canonicalDir(from)
	candidates := []string{
		rel + ".py",
		rel + "/__init__.py",
		path.Join(fromDir, rel+".py"),
		path.Join(fromDir, rel, "__init__.py"),
	}
	return idx.firstExisting(candidates)
}

func (idx *repoFileIndex) resolveJSModule(from, source string) []string {
	if source == "" {
		return nil
	}
	// Bare specifiers are npm packages; only relative/rooted ones can name a
	// workspace file.
	if !strings.HasPrefix(source, ".") && !strings.HasPrefix(source, "/") {
		return nil
	}
	base := path.Join(canonicalDir(from), source)
	if strings.HasPrefix(source, "/") {
		base = types.SlashClean(strings.TrimPrefix(source, "/"))
	}
	exts := []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}
	candidates := []string{base}
	for _, e := range exts {
		candidates = append(candidates, base+e)
	}
	for _, e := range exts {
		candidates = append(candidates, path.Join(base, "index"+e))
	}
	return idx.firstExisting(candidates)
}

func (idx *repoFileIndex) resolveRustPath(from, usePath string) []string {
	segs := strings.Split(usePath, "::")
	// Drop the crate/self/super root and the trailing item name: `use
	// crate::a::b::Thing` names module a::b, not a file called Thing.rs.
	if len(segs) > 0 && (segs[0] == "crate" || segs[0] == "self" || segs[0] == "super") {
		segs = segs[1:]
	}
	var candidates []string
	fromDir := canonicalDir(from)
	for n := len(segs); n > 0; n-- {
		rel := path.Join(segs[:n]...)
		if rel == "" {
			continue
		}
		candidates = append(candidates,
			path.Join(fromDir, rel+".rs"),
			path.Join(fromDir, rel, "mod.rs"),
			path.Join("src", rel+".rs"),
			path.Join("src", rel, "mod.rs"),
		)
	}
	return idx.firstExisting(candidates)
}

func (idx *repoFileIndex) firstExisting(candidates []string) []string {
	for _, c := range candidates {
		c = types.SlashClean(c)
		if c == "" || strings.HasPrefix(c, "..") {
			continue
		}
		if _, ok := idx.all[c]; ok {
			return []string{c}
		}
	}
	return nil
}
