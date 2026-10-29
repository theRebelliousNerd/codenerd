package world

import (
	"codenerd/internal/store"
	"context"
	"go/token"
)

// Thin wrappers over live entry points that only tests call. Production passes
// its workspace root (EnsureDeepFactsInRoot) and a request context.

// EnsureDeepFacts ensures deep world facts for the given file paths.
// Cached deep facts (depth="deep") are reused when fingerprints match.
//
// Paths are resolved against the process working directory. Callers that know
// the workspace root should use EnsureDeepFactsInRoot: deep facts must carry the
// same canonical file identity as the fast scan or code_defines/code_calls join
// against no file_topology row at all.
func EnsureDeepFacts(ctx context.Context, paths []string, db *store.LocalStore, workers int) (*DeepResult, error) {
	return EnsureDeepFactsInRoot(ctx, "", paths, db, workers)
}

// buildGoContext builds package-level context for Go files.
func (h *HolographicProvider) buildGoContext(ctx *HolographicContext, filePath string) error {
	return h.buildGoContextWithContext(context.Background(), ctx, filePath)
}

// extractGoSignatures parses one Go file directly into a HolographicContext.
//
// Kept as the single-file entry point for callers that hold a context rather
// than a package parse; it is a thin adapter over parseGoFileInto so the two
// paths cannot drift in what they extract.
func (h *HolographicProvider) extractGoSignatures(ctx *HolographicContext, fset *token.FileSet, filePath string) error {
	p := &packageParse{
		imports: make(map[string][]string, 1),
		pkgName: make(map[string]string, 1),
	}
	if err := h.parseGoFileInto(p, fset, filePath); err != nil {
		return err
	}
	ctx.PackageSignatures = append(ctx.PackageSignatures, p.signatures...)
	ctx.PackageTypes = append(ctx.PackageTypes, p.types...)
	ctx.PackageConstants = append(ctx.PackageConstants, p.constants...)
	if ctx.PackageImports == nil {
		ctx.PackageImports = make(map[string][]string, len(p.imports))
	}
	for k, v := range p.imports {
		ctx.PackageImports[k] = v
	}
	return nil
}

// WorldPredicateSet returns a map form for fast membership checks.
func WorldPredicateSet() map[string]struct{} {
	return predicateSet(WorldPredicates)
}
