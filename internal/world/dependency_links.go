package world

import (
	"os"
	"path"
	"sort"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/logging"
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
// Resolution turns those tokens into real file->file edges for imports that
// land inside the workspace. External imports keep their token form: they are
// still a truthful record of a third-party dependency and no rule joins them.
//
// Granularity is deliberately package-level for Go: an importer gets an edge to
// every non-test file of the imported package. That over-approximates (a change
// to any file of a package marks its importers impacted) which is the safe
// direction for a predicate that gates writes — the same reasoning as
// test_file_for's conservative pairing. Languages whose imports name a single
// file (Python modules, JS/TS relative specifiers, Rust modules) resolve
// exactly, one edge per import.

// maxResolvedDependencyLinks bounds the file->file expansion. Package-level
// fan-out is quadratic-ish (importers x files-in-package): codeNERD itself
// yields ~46k edges from ~2.1k import statements, and a monorepo an order of
// magnitude larger would blow past the kernel's 250k EDB ceiling and evict
// facts that matter more. On overflow the tail is dropped (edges are emitted in
// sorted order so full and incremental scans drop the SAME tail and stay
// identical) and a warning names the count.
const maxResolvedDependencyLinks = 50000

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
		goModule:     readGoModulePath(root),
	}
	for _, f := range canonicalFiles {
		c := cleanSlash(toSlashAlways(f))
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

// readGoModulePath extracts the module path from <root>/go.mod.
// Without it, Go import paths cannot be mapped to workspace directories, so
// resolution falls back to treating the import path as a literal directory
// (which is what a GOPATH-less multi-module checkout looks like).
func readGoModulePath(root string) string {
	data, err := os.ReadFile(ResolveWorkspacePath(root, "go.mod"))
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

// ResolveDependencyLinks derives in-workspace file->file import edges from the
// raw import facts in facts. The workspace file set is taken from the
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
	return resolveDependencyLinksWithIndex(newRepoFileIndex(root, files), facts)
}

func resolveDependencyLinksWithIndex(idx *repoFileIndex, facts []core.Fact) []core.Fact {
	type edge struct{ from, to, imp string }
	var edges []edge
	seen := make(map[string]struct{})

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
		for _, target := range idx.resolve(from, token, imp) {
			if target == from {
				continue
			}
			key := from + "\x00" + target
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			edges = append(edges, edge{from: from, to: target, imp: imp})
		}
	}

	// Sorted so a truncated tail is the same tail on every scanner and every
	// run; an unstable truncation would make full and incremental scans
	// disagree about which edges exist.
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].from != edges[j].from {
			return edges[i].from < edges[j].from
		}
		return edges[i].to < edges[j].to
	})

	if len(edges) > maxResolvedDependencyLinks {
		logging.WorldWarn("dependency_link resolution truncated: %d edges exceed the %d cap; impact analysis under-reports for the dropped tail",
			len(edges), maxResolvedDependencyLinks)
		edges = edges[:maxResolvedDependencyLinks]
	}

	out := make([]core.Fact, 0, len(edges))
	for _, e := range edges {
		out = append(out, core.Fact{
			Predicate: "dependency_link",
			Args:      []any{e.from, e.to, e.imp},
		})
	}
	return out
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
	return idx.goFilesByDir[cleanSlash(dir)]
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
		base = cleanSlash(strings.TrimPrefix(source, "/"))
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
		c = cleanSlash(c)
		if c == "" || strings.HasPrefix(c, "..") {
			continue
		}
		if _, ok := idx.all[c]; ok {
			return []string{c}
		}
	}
	return nil
}
