package world

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/store"
)

// A manifest can be shared with scans of sibling subtrees. A completed census
// proves deletion only inside its own root, never for another scan's entries.
func pathWithinScanRoot(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Without prior facts, a full replacement is needed only when resolution
// changes or a removed importer/changed module prevents proving it unchanged.
// Ordinary additions (including non-importing tests) keep the delta contract.
func needsImportReplacement(root string, prior map[string]CacheEntry, facts []core.Fact, changed, added, deleted []string) bool {
	for _, paths := range [][]string{changed, added, deleted} {
		for _, p := range paths {
			if canonicalScanPath(root, p) == "go.mod" {
				return true
			}
		}
	}
	for _, p := range deleted {
		switch filepath.Ext(p) {
		case ".go", ".py", ".js", ".jsx", ".ts", ".tsx", ".rs":
			return true // The removed file's previous imports are unavailable.
		}
	}
	previousFiles := make([]string, 0, len(prior))
	for p := range prior {
		if pathWithinScanRoot(root, p) {
			previousFiles = append(previousFiles, canonicalScanPath(root, p))
		}
	}
	oldRows := resolveDependencyLinksWithIndex(newRepoFileIndex(root, previousFiles), facts)
	newRows := ResolveDependencyLinks(root, facts)
	asSet := func(rows []core.Fact) map[string]bool {
		set := make(map[string]bool, len(rows))
		for _, f := range rows {
			set[f.String()] = true
		}
		return set
	}
	return !maps.Equal(asSet(oldRows), asSet(newRows))
}

// importRefreshPaths finds unchanged admitted importers whose stored resolved
// edges differ under the current file census. Queue them through the ordinary
// per-file staging path so their old owner rows retract and new rows persist.
func importRefreshPaths(ctx context.Context, root string, db *store.LocalStore, current map[string]os.FileInfo, idx *repoFileIndex, scheduled []string) ([]string, error) {
	if db == nil {
		return nil, nil
	}
	// Borrow the store handle for a cancellable read of only import rows;
	// close the cursor, never the owning database.
	rows, err := db.GetDB().QueryContext(ctx, `SELECT predicate, args FROM world_facts
		WHERE depth = 'fast' AND predicate IN ('dependency_link', 'file_imports', 'imports_project_file')`)
	if err != nil {
		return nil, fmt.Errorf("load prior import facts: %w", err)
	}
	defer rows.Close()
	paths := make(map[string]string, len(current))
	for p := range current {
		paths[canonicalScanPath(root, p)] = p
	}
	for _, p := range scheduled {
		delete(paths, canonicalScanPath(root, p))
	}
	type imports struct {
		raw      []core.Fact
		resolved map[string]bool
	}
	byFile := map[string]*imports{}
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var row store.WorldFactInput
		var encoded []byte
		if err := rows.Scan(&row.Predicate, &encoded); err != nil {
			return nil, fmt.Errorf("read prior import facts: %w", err)
		}
		row.Args, err = store.DecodeFactArgsBinary(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode prior import facts: %w", err)
		}
		if len(row.Args) == 0 || (row.Predicate != "dependency_link" && row.Predicate != "file_imports" && row.Predicate != "imports_project_file") {
			continue
		}
		from, ok := row.Args[0].(string)
		if !ok || paths[from] == "" {
			continue
		}
		group := byFile[from]
		if group == nil {
			group = &imports{resolved: map[string]bool{}}
			byFile[from] = group
		}
		f := core.Fact{Predicate: row.Predicate, Args: row.Args}
		if row.Predicate == "dependency_link" && len(row.Args) >= 3 {
			if token, ok := row.Args[1].(string); ok && (strings.HasPrefix(token, "pkg:") || strings.HasPrefix(token, "mod:") || strings.HasPrefix(token, "crate:")) {
				group.raw = append(group.raw, f)
				continue
			}
		}
		group.resolved[f.String()] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var affected []string
	for from, group := range byFile {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		fresh := map[string]bool{}
		for _, f := range resolveDependencyLinksWithIndex(idx, group.raw) {
			fresh[f.String()] = true
		}
		if !maps.Equal(group.resolved, fresh) {
			affected = append(affected, paths[from])
		}
	}
	sort.Strings(affected)
	return affected, nil
}
