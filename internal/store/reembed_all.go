package store

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
)

// ReembedResult summarizes a force re-embed run across multiple DBs.
type ReembedResult struct {
	DBCount     int
	VectorsDone int
	AtomsDone   int
	Skipped     []string
	Duration    time.Duration
}

// ReembedProgressFn is an optional progress callback.
type ReembedProgressFn func(msg string)

// ReembedAllDBsForce scans all *.db files under the given roots and force re-embeds
// vectors and prompt_atoms tables using the provided embedding engine.
// It skips DBs that can't be opened as LocalStore or don't have relevant tables.
func ReembedAllDBsForce(ctx context.Context, roots []string, engine embedding.EmbeddingEngine, progress ReembedProgressFn) (ReembedResult, error) {
	start := time.Now()
	var result ReembedResult

	if engine == nil {
		return result, fmt.Errorf("no embedding engine configured")
	}

	logging.Store("Starting force re-embed across %d root(s) with engine=%s dims=%d",
		len(roots), engine.Name(), engine.Dimensions())

	seen := make(map[string]struct{})
	var dbPaths []string
	for _, root := range roots {
		if root == "" {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || d == nil || d.IsDir() {
				return nil
			}
			if strings.HasSuffix(strings.ToLower(d.Name()), ".db") {
				if _, ok := seen[path]; !ok {
					seen[path] = struct{}{}
					dbPaths = append(dbPaths, path)
				}
			}
			return nil
		})
	}

	if len(dbPaths) == 0 {
		logging.StoreDebug("No .db files found under roots: %v", roots)
		result.Duration = time.Since(start)
		return result, nil
	}

	logging.Store("Discovered %d database(s) to re-embed", len(dbPaths))

	totalVectors := 0
	totalAtoms := 0
	dbCount := 0
	var skipped []string

	for i, dbPath := range dbPaths {
		if progress != nil {
			progress(fmt.Sprintf("Re-embedding %d/%d: %s", i+1, len(dbPaths), dbPath))
		}
		logging.Store("Re-embedding DB %d/%d: %s", i+1, len(dbPaths), dbPath)

		ls, openErr := NewLocalStore(dbPath)
		if openErr != nil {
			logging.Get(logging.CategoryStore).Warn("Skipping DB (open failed): %s: %v", dbPath, openErr)
			skipped = append(skipped, fmt.Sprintf("%s: %v", dbPath, openErr))
			continue
		}
		ls.SetEmbeddingEngine(engine)

		vecs, vecErr := ls.ReembedAllVectorsForce(ctx)
		if vecErr != nil {
			logging.Get(logging.CategoryStore).Warn("Vectors force re-embed failed for %s: %v", dbPath, vecErr)
			skipped = append(skipped, fmt.Sprintf("%s vectors: %v", dbPath, vecErr))
		}
		atoms, atomErr := ls.ReembedAllPromptAtomsForce(ctx)
		if atomErr != nil {
			logging.Get(logging.CategoryStore).Warn("Prompt atoms force re-embed failed for %s: %v", dbPath, atomErr)
			skipped = append(skipped, fmt.Sprintf("%s prompt_atoms: %v", dbPath, atomErr))
		}

		totalVectors += vecs
		totalAtoms += atoms
		dbCount++
		logging.Store("Finished DB: %s (vectors=%d, prompt_atoms=%d)", dbPath, vecs, atoms)
		_ = ls.Close()
	}

	result.DBCount = dbCount
	result.VectorsDone = totalVectors
	result.AtomsDone = totalAtoms
	result.Skipped = skipped
	result.Duration = time.Since(start)

	logging.Store("ReembedAllDBsForce complete: dbs=%d vectors=%d atoms=%d skipped=%d duration=%s",
		result.DBCount, result.VectorsDone, result.AtomsDone, len(result.Skipped), result.Duration)

	return result, nil
}
