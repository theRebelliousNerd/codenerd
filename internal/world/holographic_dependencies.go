package world

// =============================================================================
// DEPENDENCY DIMENSIONS
// =============================================================================
// HolographicContext declared DirectImports, DirectImporters and ExternalDeps
// from the day it was written and populated none of them: verified by grep,
// there was no assignment and no read of any of the three anywhere in the repo.
// The type that represents the system's "X-Ray Vision" was advertising a
// dependency dimension it never delivered.
//
// DirectImporters is the one that matters. What a file imports is visible by
// reading it; what depends on the file is not visible from anywhere the model
// can look, and it is the first thing a careful engineer wants before changing
// an exported symbol. It is the only one of the three rendered into the prompt.

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// applyImportDimensions splits the target file's parsed imports into
// first-party and external, and counts its TODO markers.
//
// Free: the imports were already extracted during the package parse, and the
// target file is read once. Neither is rendered into the prompt — a model
// editing a file can see its own import block — but both are part of the
// context struct that programmatic consumers read.
func (h *HolographicProvider) applyImportDimensions(hc *HolographicContext, filePath string) {
	if hc == nil {
		return
	}
	base := filepath.Base(filePath)
	for _, path := range hc.PackageImports[base] {
		hc.DirectImports = append(hc.DirectImports, ImportInfo{Path: path})
		if isExternalImport(path) {
			hc.ExternalDeps = append(hc.ExternalDeps, path)
		}
	}

	// CountTODOs has existed in holographic_formatting.go with zero callers.
	// One read of one file, on a path that already stats it.
	if data, err := os.ReadFile(filePath); err == nil {
		hc.TODOCount = CountTODOs(string(data))
	}
}

// isExternalImport reports whether an import path is third-party.
//
// The heuristic is the standard one: a path whose first segment contains a dot
// is a domain, so it came from outside this module and outside the standard
// library. Relative and single-segment paths ("fmt", "strings", "internal/x")
// are not.
func isExternalImport(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return strings.Contains(first, ".")
}

// applyDirectImporters fills DirectImporters from the world model.
//
// dependency_link(FilePath, "pkg:<importPath>", importPath) is emitted per
// import by the scanner (ast_treesitter.go:316), so the reverse edge is a
// single scan of that relation: every file whose CalleeID names this package.
//
// Requires a kernel and a completed scan. Absent either, the dimension stays
// empty rather than guessing — an empty importer list and an unscanned
// workspace must not look the same to a renderer, which is why PromptSection
// only emits the section when the list is non-empty.
func (h *HolographicProvider) applyDirectImporters(hc *HolographicContext, filePath string) {
	if h == nil || h.kernel == nil || hc == nil {
		return
	}
	pkgPath := h.packageImportPathInScope(filePath, hc.readScope)
	if pkgPath == "" {
		return
	}
	target := "pkg:" + pkgPath

	// Only the rows that import this package, from the argument index: the
	// relation has a row per import edge of the workspace (251k here) and this
	// runs for every file the model opens.
	facts, err := h.kernel.Query(core.BoundQuery("dependency_link", 3, map[int]string{1: target}))
	if err != nil {
		logging.WorldDebug("applyDirectImporters: dependency_link query failed: %v", err)
		return
	}

	self := types.CanonicalPath(h.workDir, filePath)
	selfDir := filepath.ToSlash(filepath.Dir(self))
	seen := make(map[string]struct{}, 16)
	var importers []string
	for _, f := range facts {
		if len(f.Args) < 2 {
			continue
		}
		if h.stringArg(f.Args[1]) != target {
			continue
		}
		importer := filepath.ToSlash(h.stringArg(f.Args[0]))
		identity := types.CanonicalPath(h.workDir, importer)
		if importer == "" || identity == self {
			continue
		}
		// A package importing itself is not an importer; the scanner records
		// the edge per file, and siblings share the package.
		if filepath.ToSlash(filepath.Dir(identity)) == selfDir {
			continue
		}
		if _, dup := seen[importer]; dup {
			continue
		}
		seen[importer] = struct{}{}
		importers = append(importers, importer)
	}

	// Sorted so the rendered section is byte-stable across turns; kernel query
	// order is not guaranteed, and a section that reshuffles costs a
	// prompt-cache hit for nothing.
	sort.Strings(importers)
	hc.DirectImporters = importers
}

// packageImportPath derives the target's module-qualified import path.
//
// Built from the module line in the workspace's go.mod plus the target's
// directory relative to the workspace, which is exactly the form the scanner
// records as the CalleeID of a dependency_link.
func (h *HolographicProvider) packageImportPath(filePath string) string {
	return h.packageImportPathInScope(filePath, nil)
}

func (h *HolographicProvider) packageImportPathInScope(filePath string, scope *holographicReadScope) string {
	if h == nil || h.workDir == "" {
		return ""
	}
	if scope == nil {
		_, fresh, err := h.readScope(context.Background(), filePath)
		if err != nil {
			return ""
		}
		scope = fresh
	}
	resolved, err := scope.resolve(context.Background(), filePath)
	if err != nil {
		return ""
	}
	module := h.moduleNameInScope(scope)
	if module == "" {
		return ""
	}
	rel, err := filepath.Rel(scope.root, filepath.Dir(resolved))
	if err != nil {
		return ""
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return module
	}
	if rel == ".." || strings.HasPrefix(rel, "../") {
		// Outside the workspace: not a package this module can name.
		return ""
	}
	return module + "/" + rel
}

// moduleName reads the module path from the workspace's go.mod.
//
// Not cached: it is one small read on a path already doing a package parse, and
// caching it would mean holding a value that goes stale when the workspace root
// changes under a long-lived provider.
func (h *HolographicProvider) moduleName() string {
	_, scope, err := h.readScope(context.Background(), "go.mod")
	if err != nil {
		return ""
	}
	return h.moduleNameInScope(scope)
}

func (h *HolographicProvider) moduleNameInScope(scope *holographicReadScope) string {
	path, err := scope.resolve(context.Background(), filepath.Join(scope.root, "go.mod"))
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}
