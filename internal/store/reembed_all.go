package store

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
)

// ReembedResult summarizes a force re-embed run across multiple DBs.
type ReembedResult struct {
	DBCount       int
	VectorsDone   int
	AtomsDone     int
	TracesDone    int
	LearningsDone int
	Skipped       []string
	Duration      time.Duration
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

	dbPaths := discoverDBPaths(roots)
	if len(dbPaths) == 0 {
		logging.StoreDebug("No .db files found under roots: %v", roots)
		result.Duration = time.Since(start)
		return result, nil
	}

	logging.Store("Discovered %d database(s) to re-embed", len(dbPaths))

	totalVectors := 0
	totalAtoms := 0
	totalTraces := 0
	dbCount := 0
	var skipped []string

	for i, dbPath := range dbPaths {
		if progress != nil {
			progress(fmt.Sprintf("Re-embedding %d/%d: %s", i+1, len(dbPaths), dbPath))
		}
		logging.Store("Re-embedding DB %d/%d: %s", i+1, len(dbPaths), dbPath)

		vecs, atoms, traces, err := processLocalDB(ctx, dbPath, engine, &skipped)
		if err != nil {
			continue
		}

		totalVectors += vecs
		totalAtoms += atoms
		totalTraces += traces
		dbCount++
	}

	learningRoots := discoverLearningRoots(roots)
	totalLearnings := 0
	for shardsDir := range learningRoots {
		totalLearnings += processLearningStore(ctx, shardsDir, engine, &skipped)
	}

	result.DBCount = dbCount
	result.VectorsDone = totalVectors
	result.AtomsDone = totalAtoms
	result.TracesDone = totalTraces
	result.LearningsDone = totalLearnings
	result.Skipped = skipped
	result.Duration = time.Since(start)

	logging.Store("ReembedAllDBsForce complete: dbs=%d vectors=%d atoms=%d traces=%d learnings=%d skipped=%d duration=%s",
		result.DBCount, result.VectorsDone, result.AtomsDone, result.TracesDone, result.LearningsDone, len(result.Skipped), result.Duration)

	return result, nil
}

func discoverDBPaths(roots []string) []string {
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
			nameLower := strings.ToLower(d.Name())
			if strings.HasSuffix(nameLower, "_learnings.db") {
				return nil
			}
			if strings.HasSuffix(nameLower, ".db") {
				if _, ok := seen[path]; !ok {
					seen[path] = struct{}{}
					dbPaths = append(dbPaths, path)
				}
			}
			return nil
		})
	}
	return dbPaths
}

func processLocalDB(ctx context.Context, dbPath string, engine embedding.EmbeddingEngine, skipped *[]string) (int, int, int, error) {
	ls, openErr := NewLocalStore(dbPath)
	if openErr != nil {
		logging.Get(logging.CategoryStore).Warn("Skipping DB (open failed): %s: %v", dbPath, openErr)
		*skipped = append(*skipped, fmt.Sprintf("%s: %v", dbPath, openErr))
		return 0, 0, 0, openErr
	}
	defer ls.Close()

	ls.SetEmbeddingEngine(engine)

	vecs, vecErr := ls.ReembedAllVectorsForce(ctx)
	if vecErr != nil {
		logging.Get(logging.CategoryStore).Warn("Vectors force re-embed failed for %s: %v", dbPath, vecErr)
		*skipped = append(*skipped, fmt.Sprintf("%s vectors: %v", dbPath, vecErr))
	}
	atoms, atomErr := ls.ReembedAllPromptAtomsForce(ctx)
	if atomErr != nil {
		logging.Get(logging.CategoryStore).Warn("Prompt atoms force re-embed failed for %s: %v", dbPath, atomErr)
		*skipped = append(*skipped, fmt.Sprintf("%s prompt_atoms: %v", dbPath, atomErr))
	}

	traces, traceErr := ls.ReembedAllTracesForce(ctx)
	if traceErr != nil {
		logging.Get(logging.CategoryStore).Warn("Trace force re-embed failed for %s: %v", dbPath, traceErr)
		*skipped = append(*skipped, fmt.Sprintf("%s traces: %v", dbPath, traceErr))
	}

	logging.Store("Finished DB: %s (vectors=%d, prompt_atoms=%d, traces=%d)", dbPath, vecs, atoms, traces)
	return vecs, atoms, traces, nil
}

func discoverLearningRoots(roots []string) map[string]struct{} {
	learningRoots := make(map[string]struct{})
	for _, root := range roots {
		if root == "" {
			continue
		}
		shardsDir := filepath.Join(root, "shards")
		if info, err := os.Stat(shardsDir); err == nil && info.IsDir() {
			learningRoots[shardsDir] = struct{}{}
		}
	}
	return learningRoots
}

func processLearningStore(ctx context.Context, shardsDir string, engine embedding.EmbeddingEngine, skipped *[]string) int {
	learningStore, err := NewLearningStore(shardsDir)
	if err != nil {
		logging.Get(logging.CategoryStore).Warn("Skipping learning store re-embed at %s: %v", shardsDir, err)
		*skipped = append(*skipped, fmt.Sprintf("%s learnings: %v", shardsDir, err))
		return 0
	}
	defer learningStore.Close()

	learningStore.SetEmbeddingEngine(engine)
	learnings, learnErr := learningStore.ReembedAllLearningsForce(ctx)
	if learnErr != nil {
		logging.Get(logging.CategoryStore).Warn("Learning force re-embed failed for %s: %v", shardsDir, learnErr)
		*skipped = append(*skipped, fmt.Sprintf("%s learnings: %v", shardsDir, learnErr))
	}
	return learnings
}
