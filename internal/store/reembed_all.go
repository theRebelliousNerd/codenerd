package store

import (
	"context"
	"errors"
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
//
// Complete is the machine-checkable answer to "did everything migrate": true
// only when no database was skipped and every table pass certified that all of
// its rows are in the target space. The counts say how much work was done;
// Stores says, table by table, what was left.
type ReembedResult struct {
	DBCount       int
	VectorsDone   int
	AtomsDone     int
	TracesDone    int
	LearningsDone int
	Skipped       []string
	Duration      time.Duration
	// Stores holds one outcome per database table the run touched.
	Stores []StoreMigration
	// Complete is true when Skipped is empty and every Stores entry is complete.
	Complete bool
}

// StoreMigration is the outcome of one table's re-embed pass.
type StoreMigration struct {
	Path     string // database or shards directory
	Table    string // "vectors", "prompt_atoms", "reasoning_traces", "learnings", or "*" for a database that could not be opened
	Done     int    // rows this pass embedded
	Total    int64  // rows the pass was responsible for; 0 when it could not say
	Failed   int64  // rows left outside the target space
	RunID    string // resumable run, when the table keeps one
	Complete bool
	Detail   string // why it is not complete
}

// migrationOf turns one table pass's result into its manifest entry. A pass
// returns nil only when it certifies the table, so a nil error is Complete; an
// *IncompleteReembedError says how much was left; anything else is a failure
// whose extent is unknown.
func migrationOf(path, table string, done int, err error) StoreMigration {
	m := StoreMigration{Path: path, Table: table, Done: done}
	if err == nil {
		m.Complete = true
		return m
	}
	m.Detail = err.Error()
	var inc *IncompleteReembedError
	if errors.As(err, &inc) {
		m.Total, m.Failed, m.RunID = inc.Total, inc.Failed, inc.RunID
	}
	return m
}

// finalize sets Complete from the outcomes gathered so far.
func (r *ReembedResult) finalize() {
	r.Complete = len(r.Skipped) == 0
	for _, m := range r.Stores {
		if !m.Complete {
			r.Complete = false
		}
	}
}

// ReembedProgressFn is an optional progress callback.
type ReembedProgressFn func(msg string)

// ReembedSearchRoots is where a force re-embed looks for databases: the
// workspace .nerd tree, and only that.
//
// It used to include the workspace internal/ tree as well. The databases there
// are build inputs -- internal/core/defaults/*_corpus.db are go:embed-ed into
// the binary and internal/shards/*_learnings.db are tracked -- so a runtime
// maintenance command was rewriting source files: measured 2026-09-17, one
// /embedding reembed dirtied five tracked databases and the next build shipped
// the rewritten copies. Their vectors are legacy either way (no embedding_model
// column, skipped by the searcher); the prompt loader re-embeds atoms from YAML
// into .nerd/prompts/corpus.db with the live engine.
func ReembedSearchRoots(workspace string) []string {
	return []string{filepath.Join(workspace, ".nerd")}
}

// ReembedAllDBsForce scans all *.db files under the given roots and force re-embeds
// vectors and prompt_atoms tables using the provided embedding engine.
// It skips DBs that can't be opened as LocalStore or don't have relevant tables.
func ReembedAllDBsForce(ctx context.Context, roots []string, engine embedding.EmbeddingEngine, progress ReembedProgressFn) (ReembedResult, error) {
	start := time.Now()
	var result ReembedResult

	if engine == nil {
		return result, fmt.Errorf("no embedding engine configured")
	}
	if err := ctx.Err(); err != nil {
		result.Duration = time.Since(start)
		return result, err
	}

	logging.Store("Starting force re-embed across %d root(s) with engine=%s dims=%d",
		len(roots), engine.Name(), engine.Dimensions())

	dbPaths := discoverDBPaths(roots)
	if len(dbPaths) == 0 {
		logging.StoreDebug("No .db files found under roots: %v", roots)
		result.Duration = time.Since(start)
		result.finalize() // nothing to migrate: nothing left behind
		return result, nil
	}

	logging.Store("Discovered %d database(s) to re-embed", len(dbPaths))

	for i, dbPath := range dbPaths {
		// Fail fast between databases: without this a cancelled run still
		// opens and re-embeds every remaining DB before noticing.
		if err := ctx.Err(); err != nil {
			result.Duration = time.Since(start)
			result.finalize()
			return result, err
		}
		if progress != nil {
			progress(fmt.Sprintf("Re-embedding %d/%d: %s", i+1, len(dbPaths), dbPath))
		}
		logging.Store("Re-embedding DB %d/%d: %s", i+1, len(dbPaths), dbPath)

		opened := processLocalDB(ctx, dbPath, engine, &result)
		if opened {
			result.DBCount++
		}
	}

	learningRoots := discoverLearningRoots(roots)
	for shardsDir := range learningRoots {
		if err := ctx.Err(); err != nil {
			result.Duration = time.Since(start)
			result.finalize()
			return result, err
		}
		processLearningStore(ctx, shardsDir, engine, &result)
	}

	result.Duration = time.Since(start)
	result.finalize()

	logging.Store("ReembedAllDBsForce complete=%v: dbs=%d vectors=%d atoms=%d traces=%d learnings=%d skipped=%d duration=%s",
		result.Complete, result.DBCount, result.VectorsDone, result.AtomsDone, result.TracesDone, result.LearningsDone, len(result.Skipped), result.Duration)

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

// processLocalDB re-embeds one database's tables and records each table's
// outcome in result. It reports whether the database could be opened at all.
func processLocalDB(ctx context.Context, dbPath string, engine embedding.EmbeddingEngine, result *ReembedResult) bool {
	ls, openErr := NewLocalStore(dbPath)
	if openErr != nil {
		logging.Get(logging.CategoryStore).Warn("Skipping DB (open failed): %s: %v", dbPath, openErr)
		result.Skipped = append(result.Skipped, fmt.Sprintf("%s: %v", dbPath, openErr))
		result.Stores = append(result.Stores, migrationOf(dbPath, "*", 0, openErr))
		return false
	}
	defer ls.Close()

	ls.SetEmbeddingEngine(engine)

	vecs, vecErr := ls.ReembedAllVectorsForce(ctx)
	if vecErr != nil {
		logging.Get(logging.CategoryStore).Warn("Vectors force re-embed failed for %s: %v", dbPath, vecErr)
		result.Skipped = append(result.Skipped, fmt.Sprintf("%s vectors: %v", dbPath, vecErr))
	}
	result.VectorsDone += vecs
	result.Stores = append(result.Stores, migrationOf(dbPath, "vectors", vecs, vecErr))

	atoms, atomErr := ls.ReembedAllPromptAtomsForce(ctx)
	if atomErr != nil {
		logging.Get(logging.CategoryStore).Warn("Prompt atoms force re-embed failed for %s: %v", dbPath, atomErr)
		result.Skipped = append(result.Skipped, fmt.Sprintf("%s prompt_atoms: %v", dbPath, atomErr))
	}
	result.AtomsDone += atoms
	result.Stores = append(result.Stores, migrationOf(dbPath, "prompt_atoms", atoms, atomErr))

	traces, traceErr := ls.ReembedAllTracesForce(ctx)
	if traceErr != nil {
		logging.Get(logging.CategoryStore).Warn("Trace force re-embed failed for %s: %v", dbPath, traceErr)
		result.Skipped = append(result.Skipped, fmt.Sprintf("%s traces: %v", dbPath, traceErr))
	}
	result.TracesDone += traces
	result.Stores = append(result.Stores, migrationOf(dbPath, "reasoning_traces", traces, traceErr))

	logging.Store("Finished DB: %s (vectors=%d, prompt_atoms=%d, traces=%d)", dbPath, vecs, atoms, traces)
	return true
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

func processLearningStore(ctx context.Context, shardsDir string, engine embedding.EmbeddingEngine, result *ReembedResult) {
	learningStore, err := NewLearningStore(shardsDir)
	if err != nil {
		logging.Get(logging.CategoryStore).Warn("Skipping learning store re-embed at %s: %v", shardsDir, err)
		result.Skipped = append(result.Skipped, fmt.Sprintf("%s learnings: %v", shardsDir, err))
		result.Stores = append(result.Stores, migrationOf(shardsDir, "learnings", 0, err))
		return
	}
	defer learningStore.Close()

	learningStore.SetEmbeddingEngine(engine)
	learnings, learnErr := learningStore.ReembedAllLearningsForce(ctx)
	if learnErr != nil {
		logging.Get(logging.CategoryStore).Warn("Learning force re-embed failed for %s: %v", shardsDir, learnErr)
		result.Skipped = append(result.Skipped, fmt.Sprintf("%s learnings: %v", shardsDir, learnErr))
	}
	result.LearningsDone += learnings
	result.Stores = append(result.Stores, migrationOf(shardsDir, "learnings", learnings, learnErr))
}
