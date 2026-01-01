package store

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
)

// TraceRecallHit represents a semantic recall hit for a trace descriptor.
type TraceRecallHit struct {
	TraceID   string
	Score     float64
	Summary   string
	Outcome   string
	ShardType string
	CreatedAt time.Time
	EmbeddingModelID string
	EmbeddingDim     int
	EmbeddingTask    string
}

// LearningRecallHit represents a semantic recall hit for a learning handle.
type LearningRecallHit struct {
	LearningID int64
	Score      float64
	Predicate  string
	Summary    string
	ShardType  string
	LearnedAt  time.Time
	EmbeddingModelID string
	EmbeddingDim     int
	EmbeddingTask    string
}

// RecallTracesByEmbedding returns top trace hits for a query embedding.
func (s *LocalStore) RecallTracesByEmbedding(query []float32, limit int) ([]TraceRecallHit, error) {
	timer := logging.StartTimer(logging.CategoryStore, "RecallTracesByEmbedding")
	defer timer.Stop()

	if s == nil || s.db == nil || len(query) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}

	if s.vectorExt && tableExists(s.db, "reasoning_traces_vec") {
		hits, err := s.recallTraceVec(query, limit)
		if err == nil {
			return hits, nil
		}
	}

	return s.recallTraceBruteForce(query, limit)
}

// RecallTracesLexical falls back to keyword search on descriptors.
func (s *LocalStore) RecallTracesLexical(query string, limit int) ([]TraceRecallHit, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}

	keywords := extractKeywords(query, 4)
	if len(keywords) == 0 {
		return nil, nil
	}

	var conditions []string
	var args []interface{}
	for _, kw := range keywords {
		conditions = append(conditions, "LOWER(summary_descriptor) LIKE ?")
		args = append(args, "%"+strings.ToLower(kw)+"%")
	}
		querySQL := fmt.Sprintf(`
			SELECT id, COALESCE(summary_descriptor, ''), shard_type, success, created_at,
			       COALESCE(embedding_model_id, ''), COALESCE(embedding_dim, 0), COALESCE(embedding_task, '')
			FROM reasoning_traces
			WHERE %s
			ORDER BY created_at DESC
			LIMIT ?`, strings.Join(conditions, " OR "))
	args = append(args, limit*3)

	rows, err := s.db.Query(querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hits []TraceRecallHit
	for rows.Next() {
		var hit TraceRecallHit
		var success bool
		var createdAt sql.NullTime
		if err := rows.Scan(&hit.TraceID, &hit.Summary, &hit.ShardType, &success, &createdAt, &hit.EmbeddingModelID, &hit.EmbeddingDim, &hit.EmbeddingTask); err != nil {
			continue
		}
		if createdAt.Valid {
			hit.CreatedAt = createdAt.Time
		}
		if success {
			hit.Outcome = "success"
		} else {
			hit.Outcome = "failure"
		}
		hit.Score = lexicalScore(hit.Summary, keywords)
		hits = append(hits, hit)
	}

	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

// RecallLearningsByEmbedding returns top learning hits for a query embedding across shards.
func (ls *LearningStore) RecallLearningsByEmbedding(query []float32, limit int) ([]LearningRecallHit, error) {
	timer := logging.StartTimer(logging.CategoryStore, "RecallLearningsByEmbedding")
	defer timer.Stop()

	if ls == nil || len(query) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}

	shards := ls.listShardTypes()
	var all []LearningRecallHit
	for _, shardType := range shards {
		hits, err := ls.recallLearningsInShard(query, shardType, limit)
		if err != nil {
			continue
		}
		all = append(all, hits...)
	}

	if len(all) == 0 {
		return nil, nil
	}

	sort.Slice(all, func(i, j int) bool { return all[i].Score > all[j].Score })
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

// RecallLearningsLexical falls back to keyword search on semantic handles.
func (ls *LearningStore) RecallLearningsLexical(query string, limit int) ([]LearningRecallHit, error) {
	if ls == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}
	keywords := extractKeywords(query, 4)
	if len(keywords) == 0 {
		return nil, nil
	}

	shards := ls.listShardTypes()
	var all []LearningRecallHit
	for _, shardType := range shards {
		hits, err := ls.recallLearningsLexicalInShard(shardType, keywords, limit)
		if err != nil {
			continue
		}
		all = append(all, hits...)
	}

	if len(all) == 0 {
		return nil, nil
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Score > all[j].Score })
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

func (s *LocalStore) recallTraceVec(query []float32, limit int) ([]TraceRecallHit, error) {
	queryBlob := encodeFloat32Slice(query)
	rows, err := s.db.Query(`
		SELECT rt.id, COALESCE(rt.summary_descriptor, ''), rt.shard_type, rt.success, rt.created_at,
		       COALESCE(rt.embedding_model_id, ''), COALESCE(rt.embedding_dim, 0), COALESCE(rt.embedding_task, ''),
		       vec_distance_cosine(v.embedding, ?) AS distance
		FROM reasoning_traces_vec v
		JOIN reasoning_traces rt ON rt.id = v.trace_id
		ORDER BY distance ASC
		LIMIT ?`, queryBlob, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hits []TraceRecallHit
	for rows.Next() {
		var hit TraceRecallHit
		var distance sql.NullFloat64
		var createdAt sql.NullTime
		var success bool
		if err := rows.Scan(&hit.TraceID, &hit.Summary, &hit.ShardType, &success, &createdAt, &hit.EmbeddingModelID, &hit.EmbeddingDim, &hit.EmbeddingTask, &distance); err != nil {
			continue
		}
		if createdAt.Valid {
			hit.CreatedAt = createdAt.Time
		}
		if success {
			hit.Outcome = "success"
		} else {
			hit.Outcome = "failure"
		}
		if distance.Valid {
			hit.Score = clampScore(1 - distance.Float64)
		}
		hits = append(hits, hit)
	}
	return hits, nil
}

func (s *LocalStore) recallTraceBruteForce(query []float32, limit int) ([]TraceRecallHit, error) {
	rows, err := s.db.Query(`
		SELECT id, COALESCE(summary_descriptor, ''), shard_type, success, created_at,
		       embedding, COALESCE(embedding_model_id, ''), COALESCE(embedding_dim, 0), COALESCE(embedding_task, '')
		FROM reasoning_traces
		WHERE embedding IS NOT NULL AND length(embedding) > 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hits []TraceRecallHit
	for rows.Next() {
		var hit TraceRecallHit
		var success bool
		var createdAt sql.NullTime
		var blob []byte
		if err := rows.Scan(&hit.TraceID, &hit.Summary, &hit.ShardType, &success, &createdAt, &blob, &hit.EmbeddingModelID, &hit.EmbeddingDim, &hit.EmbeddingTask); err != nil {
			continue
		}
		if createdAt.Valid {
			hit.CreatedAt = createdAt.Time
		}
		if success {
			hit.Outcome = "success"
		} else {
			hit.Outcome = "failure"
		}
		vec := decodeFloat32SliceFromBlob(blob)
		if len(vec) == 0 || len(vec) != len(query) {
			continue
		}
		score, err := embedding.CosineSimilarity(query, vec)
		if err != nil {
			continue
		}
		hit.Score = clampScore(score)
		hits = append(hits, hit)
	}

	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func (ls *LearningStore) recallLearningsInShard(query []float32, shardType string, limit int) ([]LearningRecallHit, error) {
	db, err := ls.getDB(shardType)
	if err != nil {
		return nil, err
	}

	if tableExists(db, "learnings_vec") {
		queryBlob := encodeFloat32Slice(query)
		rows, err := db.Query(`
			SELECT l.id, COALESCE(l.semantic_handle, ''), l.fact_predicate, l.shard_type, l.learned_at,
			       COALESCE(l.embedding_model_id, ''), COALESCE(l.embedding_dim, 0), COALESCE(l.embedding_task, ''),
			       vec_distance_cosine(v.embedding, ?) AS distance
			FROM learnings_vec v
			JOIN learnings l ON l.id = v.learning_id
			ORDER BY distance ASC
			LIMIT ?`, queryBlob, limit)
		if err == nil {
			defer rows.Close()
			var hits []LearningRecallHit
			for rows.Next() {
					var hit LearningRecallHit
					var distance sql.NullFloat64
					if err := rows.Scan(&hit.LearningID, &hit.Summary, &hit.Predicate, &hit.ShardType, &hit.LearnedAt, &hit.EmbeddingModelID, &hit.EmbeddingDim, &hit.EmbeddingTask, &distance); err != nil {
						continue
					}
				if distance.Valid {
					hit.Score = clampScore(1 - distance.Float64)
				}
				hits = append(hits, hit)
			}
			return hits, nil
		}
	}

	rows, err := db.Query(`
		SELECT id, shard_type, fact_predicate, COALESCE(semantic_handle, ''), learned_at,
		       embedding, COALESCE(embedding_model_id, ''), COALESCE(embedding_dim, 0), COALESCE(embedding_task, '')
		FROM learnings
		WHERE embedding IS NOT NULL AND length(embedding) > 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hits []LearningRecallHit
	for rows.Next() {
		var hit LearningRecallHit
		var blob []byte
		if err := rows.Scan(&hit.LearningID, &hit.ShardType, &hit.Predicate, &hit.Summary, &hit.LearnedAt, &blob, &hit.EmbeddingModelID, &hit.EmbeddingDim, &hit.EmbeddingTask); err != nil {
			continue
		}
		vec := decodeFloat32SliceFromBlob(blob)
		if len(vec) == 0 || len(vec) != len(query) {
			continue
		}
		score, err := embedding.CosineSimilarity(query, vec)
		if err != nil {
			continue
		}
		hit.Score = clampScore(score)
		hits = append(hits, hit)
	}

	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func (ls *LearningStore) recallLearningsLexicalInShard(shardType string, keywords []string, limit int) ([]LearningRecallHit, error) {
	db, err := ls.getDB(shardType)
	if err != nil {
		return nil, err
	}

	var conditions []string
	var args []interface{}
	for _, kw := range keywords {
		conditions = append(conditions, "LOWER(semantic_handle) LIKE ?")
		args = append(args, "%"+strings.ToLower(kw)+"%")
	}
		querySQL := fmt.Sprintf(`
			SELECT id, shard_type, fact_predicate, COALESCE(semantic_handle, ''), learned_at,
			       COALESCE(embedding_model_id, ''), COALESCE(embedding_dim, 0), COALESCE(embedding_task, '')
			FROM learnings
			WHERE %s
			ORDER BY learned_at DESC
			LIMIT ?`, strings.Join(conditions, " OR "))
	args = append(args, limit*3)

	rows, err := db.Query(querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hits []LearningRecallHit
	for rows.Next() {
		var hit LearningRecallHit
		if err := rows.Scan(&hit.LearningID, &hit.ShardType, &hit.Predicate, &hit.Summary, &hit.LearnedAt, &hit.EmbeddingModelID, &hit.EmbeddingDim, &hit.EmbeddingTask); err != nil {
			continue
		}
		hit.Score = lexicalScore(hit.Summary, keywords)
		hits = append(hits, hit)
	}
	return hits, nil
}

func (ls *LearningStore) listShardTypes() []string {
	ls.mu.RLock()
	seen := make(map[string]struct{})
	for shardType := range ls.dbs {
		seen[shardType] = struct{}{}
	}
	ls.mu.RUnlock()
	for _, shardType := range ls.listShardTypesFromDisk() {
		if shardType == "" {
			continue
		}
		seen[shardType] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	shards := make([]string, 0, len(seen))
	for shardType := range seen {
		shards = append(shards, shardType)
	}
	sort.Strings(shards)
	return shards
}

func (ls *LearningStore) listShardTypesFromDisk() []string {
	if ls.basePath == "" {
		return nil
	}
	entries, err := os.ReadDir(ls.basePath)
	if err != nil {
		return []string{"coder", "tester", "reviewer", "researcher"}
	}
	var shards []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, "_learnings.db") {
			shardType := strings.TrimSuffix(name, "_learnings.db")
			if shardType != "" {
				shards = append(shards, shardType)
			}
		}
	}
	if len(shards) == 0 {
		shards = []string{"coder", "tester", "reviewer", "researcher"}
	}
	return shards
}

func extractKeywords(text string, max int) []string {
	if max <= 0 {
		max = 4
	}
	words := strings.Fields(strings.ToLower(text))
	var keywords []string
	for _, word := range words {
		word = strings.Trim(word, ".,:;()[]{}<>\"'")
		if len(word) < 4 {
			continue
		}
		keywords = append(keywords, word)
		if len(keywords) >= max {
			break
		}
	}
	return keywords
}

func lexicalScore(text string, keywords []string) float64 {
	if len(keywords) == 0 {
		return 0
	}
	textLower := strings.ToLower(text)
	matches := 0
	for _, kw := range keywords {
		if strings.Contains(textLower, kw) {
			matches++
		}
	}
	score := float64(matches) / float64(len(keywords))
	if score < 0.3 {
		score = 0.3
	}
	return clampScore(score)
}
