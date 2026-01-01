package prompt

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"sync"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
)

// CompilerVectorSearcher is the default VectorSearcher for JIT prompts.
// It searches prompt_atoms embeddings across the compiler's registered DBs.
type CompilerVectorSearcher struct {
	mu       sync.RWMutex
	compiler *JITPromptCompiler
	engine   embedding.EmbeddingEngine
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

// Search performs semantic search over prompt_atoms embeddings.
func (s *CompilerVectorSearcher) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	s.mu.RLock()
	compiler := s.compiler
	engine := s.engine
	s.mu.RUnlock()

	if compiler == nil || engine == nil || query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}

	// Embed query (prefer RETRIEVAL_QUERY if supported).
	var queryEmbedding []float32
	var err error
	taskType := embedding.SelectTaskType(embedding.ContentTypeQuery, true)
	if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok && taskType != "" {
		queryEmbedding, err = taskAware.EmbedWithTask(ctx, query, taskType)
	} else {
		queryEmbedding, err = engine.Embed(ctx, query)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	dbs := compiler.snapshotPromptDBs()
	if len(dbs) == 0 {
		return nil, nil
	}

	bestScores := make(map[string]float64)
	for _, db := range dbs {
		if db == nil {
			continue
		}

		rows, err := db.QueryContext(ctx, "SELECT atom_id, embedding FROM prompt_atoms WHERE embedding IS NOT NULL")
		if err != nil {
			// Non-fatal; some DBs may not yet have embeddings or schema.
			logging.Get(logging.CategoryContext).Debug("Vector search skipped DB (prompt_atoms query failed): %v", err)
			continue
		}

		for rows.Next() {
			var atomID string
			var blob []byte
			if err := rows.Scan(&atomID, &blob); err != nil {
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
		return candidates[i].score > candidates[j].score
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

// snapshotPromptDBs returns a stable slice of DBs registered with the compiler.
func (c *JITPromptCompiler) snapshotPromptDBs() []*sql.DB {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	dbs := make([]*sql.DB, 0, 1+len(c.shardDBs))
	if c.projectDB != nil {
		dbs = append(dbs, c.projectDB)
	}
	for _, db := range c.shardDBs {
		if db != nil {
			dbs = append(dbs, db)
		}
	}
	return dbs
}

func decodeFloat32Slice(blob []byte) []float32 {
	if len(blob) == 0 || len(blob)%4 != 0 {
		return nil
	}
	n := len(blob) / 4
	vec := make([]float32, n)
	for i := 0; i < n; i++ {
		bits := binary.LittleEndian.Uint32(blob[i*4 : (i+1)*4])
		vec[i] = math.Float32frombits(bits)
	}
	return vec
}
