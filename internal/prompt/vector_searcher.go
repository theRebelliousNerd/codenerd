package prompt

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
)

// CompilerVectorSearcher is the default VectorSearcher for JIT prompts.
// It searches prompt_atoms embeddings across the compiler's registered DBs.
type CompilerVectorSearcher struct {
	mu          sync.RWMutex
	compiler    *JITPromptCompiler
	engine      embedding.EmbeddingEngine
	lastSkipped map[string]int
}

// NewCompilerVectorSearcher creates a default vector searcher backed by prompt_atoms embeddings.
func NewCompilerVectorSearcher(engine embedding.EmbeddingEngine) *CompilerVectorSearcher {
	return &CompilerVectorSearcher{engine: engine}
}

// SetCompiler attaches the JIT compiler so the searcher can access registered DBs.
func (s *CompilerVectorSearcher) SetCompiler(c *JITPromptCompiler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.compiler = c
}

// EmbedQuery generates an embedding vector for the given query.
func (s *CompilerVectorSearcher) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	s.mu.RLock()
	engine := s.engine
	s.mu.RUnlock()

	if engine == nil || query == "" {
		return nil, fmt.Errorf("no engine or query")
	}

	taskType := embedding.SelectTaskType(embedding.ContentTypeQuery, true)
	if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok && taskType != "" {
		return taskAware.EmbedWithTask(ctx, query, taskType)
	}
	return engine.Embed(ctx, query)
}

// Search performs semantic search over prompt_atoms embeddings.
func (s *CompilerVectorSearcher) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	s.mu.RLock()
	compiler := s.compiler
	engine := s.engine
	s.mu.RUnlock()

	if compiler == nil || query == "" {
		return nil, nil
	}
	if engine == nil {
		return nil, nil
	}
	modelName := engine.Name()
	if limit <= 0 {
		limit = 10
	}

	// Embed query (prefer RETRIEVAL_QUERY if supported).
	queryEmbedding, err := s.EmbedQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	dbs := compiler.snapshotPromptDBs()
	if len(dbs) == 0 {
		return nil, nil
	}

	bestScores := make(map[string]float64)
	skipped := make(map[string]int)
	for _, db := range dbs {
		if db == nil {
			continue
		}

		rows, err := db.QueryContext(ctx, "SELECT atom_id, embedding, COALESCE(embedding_model, '') FROM prompt_atoms WHERE embedding IS NOT NULL")
		legacy := false
		if err != nil {
			// Fall back for databases created before the embedding_model column existed.
			rows, err = db.QueryContext(ctx, "SELECT atom_id, embedding FROM prompt_atoms WHERE embedding IS NOT NULL")
			if err != nil {
				// Non-fatal; some DBs may not yet have embeddings or schema.
				logging.Get(logging.CategoryContext).Debug("Vector search skipped DB (prompt_atoms query failed): %v", err)
				continue
			}
			legacy = true
		}

		for rows.Next() {
			var atomID string
			var blob []byte
			var storedModel string
			if legacy {
				if err := rows.Scan(&atomID, &blob); err != nil {
					continue
				}
				storedModel = ""
			} else {
				if err := rows.Scan(&atomID, &blob, &storedModel); err != nil {
					continue
				}
			}
			if storedModel != modelName {
				key := storedModel
				if key == "" {
					key = "unstamped"
				}
				skipped[key]++
				continue
			}
			vec := decodeFloat32Slice(blob)
			if len(vec) != len(queryEmbedding) {
				continue
			}
			sim, err := embedding.CosineSimilarity(queryEmbedding, vec)
			if err != nil {
				continue
			}
			if sim > bestScores[atomID] {
				bestScores[atomID] = sim
			}
		}
		rows.Close()
	}

	s.mu.Lock()
	s.lastSkipped = skipped
	s.mu.Unlock()

	if len(skipped) > 0 {
		names := make([]string, 0, len(skipped))
		total := 0
		for name, count := range skipped {
			names = append(names, name)
			total += count
		}
		sort.Strings(names)
		pairs := make([]string, 0, len(names))
		for _, name := range names {
			pairs = append(pairs, fmt.Sprintf("%s=%d", name, skipped[name]))
		}
		logging.Get(logging.CategoryJIT).Warn("Vector search skipped %d atom vector(s) not embedded by %s (%s); run `nerd embedding reembed`", total, modelName, strings.Join(pairs, ", "))
	}

	if len(bestScores) == 0 {
		return nil, nil
	}

	type scored struct {
		id    string
		score float64
	}
	candidates := make([]scored, 0, len(bestScores))
	for id, score := range bestScores {
		candidates = append(candidates, scored{id: id, score: score})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].id < candidates[j].id
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	results := make([]SearchResult, len(candidates))
	for i, c := range candidates {
		results[i] = SearchResult{AtomID: c.id, Score: c.score}
	}

	return results, nil
}

// LastSkipped returns a copy of the per-model skipped counts from the last Search call.
func (s *CompilerVectorSearcher) LastSkipped() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]int, len(s.lastSkipped))
	for k, v := range s.lastSkipped {
		out[k] = v
	}
	return out
}

// snapshotPromptDBs returns a stable slice of DBs registered with the compiler.
func (c *JITPromptCompiler) snapshotPromptDBs() []*sql.DB {
	if c == nil {
		return nil
	}
	c.shardMu.RLock()
	shardDBsCopy := make([]*sql.DB, 0, len(c.shardDBs))
	for _, db := range c.shardDBs {
		if db != nil {
			shardDBsCopy = append(shardDBsCopy, db)
		}
	}
	c.shardMu.RUnlock()

	c.dbMu.RLock()
	var projectDB *sql.DB
	if c.projectDB != nil {
		projectDB = c.projectDB
	}
	c.dbMu.RUnlock()

	dbs := make([]*sql.DB, 0, 1+len(shardDBsCopy))
	if projectDB != nil {
		dbs = append(dbs, projectDB)
	}
	dbs = append(dbs, shardDBsCopy...)
	return dbs
}

func decodeFloat32Slice(blob []byte) []float32 {
	if len(blob) == 0 || len(blob)%4 != 0 {
		return nil
	}
	n := len(blob) / 4
	vec := make([]float32, n)
	for i := range n {
		bits := binary.LittleEndian.Uint32(blob[i*4 : (i+1)*4])
		vec[i] = math.Float32frombits(bits)
	}
	return vec
}
