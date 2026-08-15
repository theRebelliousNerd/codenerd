package retrieval

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// =============================================================================
// TIER 3: GO IMPORT EXPANDER
// =============================================================================
//
// The import tier resolved Python `import x` / `from x import y` only. Applied
// to a Go repository — including this one — it resolved nothing, so Tier 3 was
// permanently empty and its 20% of the context budget went unspent while the
// files that actually define the symbols under discussion sat one edge away.
//
// A Go import names a package directory, not a file, so resolution expands to
// the package's source files. Only imports inside the current module are
// followed: the standard library and third-party modules live outside the
// workspace and cannot be edited, which is what this context is for.

// maxGoPackageFiles caps how many files one imported package contributes. A
// package like internal/core has dozens of files; pulling all of them in for a
// single import would spend the entire tier budget on one edge.
const maxGoPackageFiles = 3

// goModulePathPattern matches the module directive at the top of a go.mod.
var goModulePathPattern = regexp.MustCompile(`(?m)^\s*module\s+(\S+)`)

// goImportNeighbors returns workspace files belonging to the packages imported
// by a Go source file, nearest-package-first.
func (b *TieredContextBuilder) goImportNeighbors(filePath string) []string {
	imports := parseGoImports(filePath)
	if len(imports) == 0 {
		return nil
	}

	module := b.goModulePath()
	var out []string
	seen := make(map[string]bool)

	for _, imp := range imports {
		dir := b.resolveGoImportDir(imp, module)
		if dir == "" {
			continue
		}
		for _, f := range goPackageFiles(dir) {
			if seen[f] || f == filePath {
				continue
			}
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}

// parseGoImports reads the import block with the stdlib parser. ImportsOnly
// stops at the end of the import declaration, so this stays cheap even on very
// large files, and it is exact where a regex would trip over grouped imports,
// blank/dot aliases and import strings inside comments.
func parseGoImports(filePath string) []string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
	if err != nil || f == nil {
		return nil
	}
	out := make([]string, 0, len(f.Imports))
	for _, spec := range f.Imports {
		if spec.Path == nil {
			continue
		}
		p, uerr := strconv.Unquote(spec.Path.Value)
		if uerr != nil || p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

// goModulePath returns the module path declared by the workspace's go.mod, or ""
// when there is none. Cached under b.mu: every Tier 3 file would otherwise
// re-read go.mod.
func (b *TieredContextBuilder) goModulePath() string {
	const cacheKey = "\x00go.mod:module"

	b.mu.RLock()
	cached, ok := b.findCache[cacheKey]
	b.mu.RUnlock()
	if ok {
		return cached
	}

	module := ""
	if data, err := os.ReadFile(filepath.Join(b.workDir, "go.mod")); err == nil {
		if m := goModulePathPattern.FindSubmatch(data); len(m) > 1 {
			module = string(m[1])
		}
	}

	b.mu.Lock()
	if b.findCache == nil {
		b.findCache = make(map[string]string)
	}
	b.findCache[cacheKey] = module
	b.mu.Unlock()
	return module
}

// resolveGoImportDir maps an import path to a directory inside the workspace, or
// "" when the import is external.
func (b *TieredContextBuilder) resolveGoImportDir(importPath, module string) string {
	rel := ""
	switch {
	case module != "" && importPath == module:
		rel = "."
	case module != "" && strings.HasPrefix(importPath, module+"/"):
		rel = strings.TrimPrefix(importPath, module+"/")
	case !strings.Contains(firstSegment(importPath), "."):
		// No module line, or a path that is not module-qualified: a repo laid
		// out as a plain GOPATH tree still has the package under the root. The
		// dot test keeps "github.com/x/y" and other hosted modules out.
		rel = importPath
	default:
		return ""
	}

	dir := filepath.Join(b.workDir, filepath.FromSlash(rel))
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return ""
	}
	return dir
}

func firstSegment(importPath string) string {
	if i := strings.IndexByte(importPath, '/'); i >= 0 {
		return importPath[:i]
	}
	return importPath
}

// goPackageFiles lists a package directory's non-test Go sources, smallest
// first — the small files in a Go package are usually the type and interface
// declarations, which is what an import edge is worth reading for.
func goPackageFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	type sized struct {
		path string
		size int64
	}
	var candidates []sized
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		info, ierr := e.Info()
		if ierr != nil {
			continue
		}
		candidates = append(candidates, sized{filepath.Join(dir, name), info.Size()})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].size != candidates[j].size {
			return candidates[i].size < candidates[j].size
		}
		return candidates[i].path < candidates[j].path
	})

	if len(candidates) > maxGoPackageFiles {
		candidates = candidates[:maxGoPackageFiles]
	}
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, c.path)
	}
	return out
}
