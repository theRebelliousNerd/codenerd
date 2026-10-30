package prompt

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"maps"
	"math"
	"sort"
	"strings"
	"sync"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"codenerd/internal/store"
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

// EmbedQuery embeds the query with the engine captured for this call and
// stamps the vector with that engine's identity.
func (s *CompilerVectorSearcher) EmbedQuery(ctx context.Context, query string) (store.QueryEmbedding, error) {
	s.mu.RLock()
	engine := s.engine
	s.mu.RUnlock()

	if engine == nil || query == "" {
		return store.QueryEmbedding{}, fmt.Errorf("no engine or query")
	}

	var vec []float32
	var err error
	taskType := embedding.SelectTaskType(embedding.ContentTypeQuery, true)
	if taskAware, ok := engine.(embedding.TaskTypeAwareEngine); ok && taskType != "" {
		vec, err = taskAware.EmbedWithTask(ctx, query, taskType)
	} else {
		vec, err = engine.Embed(ctx, query)
	}
	if err != nil {
		return store.QueryEmbedding{}, err
	}
	return store.QueryEmbedding{Vector: vec, ModelID: engine.Name()}, nil
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
	if limit <= 0 {
		limit = 10
	}

	// Embed query (prefer RETRIEVAL_QUERY if supported).
	queryEmbedding, err := s.EmbedQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}
	// The identity of the engine that produced this vector, not a second
	// read of the field (an engine swap between the two would mismatch).
	modelName := queryEmbedding.ModelID

	dbs, release := compiler.snapshotPromptDBs()
	defer release()
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
			if len(vec) != len(queryEmbedding.Vector) {
				continue
			}
			sim, err := embedding.CosineSimilarity(queryEmbedding.Vector, vec)
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
	prevSkipped := s.lastSkipped
	s.lastSkipped = skipped
	s.mu.Unlock()

	if warnSkipped(prevSkipped, skipped) {
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

// snapshotPromptDBs pins a stable set of shard pools for the vector scan.
func (c *JITPromptCompiler) snapshotPromptDBs() ([]*sql.DB, func()) {
	if c == nil {
		return nil, func() {}
	}
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()
	if c.closed {
		return nil, func() {}
	}
	c.dbMu.RLock()
	projectDB := c.projectDB
	c.dbMu.RUnlock()
	c.shardMu.Lock()
	defer c.shardMu.Unlock()
	dbs := make([]*sql.DB, 0, 1+len(c.shardDBs))
	if projectDB != nil {
		dbs = append(dbs, projectDB)
	}
	var registrations []*shardDBRegistration
	for _, registration := range c.shardDBs {
		if registration.db != nil {
			dbs = append(dbs, registration.db)
			registrations = append(registrations, registration)
		}
	}
	return dbs, c.pinShardReadersLocked(registrations)
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

// warnSkipped reports whether a search's skipped vectors are worth a warning:
// some were skipped, and the set differs from the previous search's. The same
// unstamped atoms were otherwise warned about on every compile.
func warnSkipped(prev, cur map[string]int) bool {
	return len(cur) > 0 && !maps.Equal(prev, cur)
}
