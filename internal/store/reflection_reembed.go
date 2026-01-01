package store

import (
	"context"
	"fmt"

	"codenerd/internal/embedding"
	"codenerd/internal/logging"
)

// ReembedAllTracesForce regenerates embeddings for all reasoning traces.
func (s *LocalStore) ReembedAllTracesForce(ctx context.Context) (int, error) {
	timer := logging.StartTimer(logging.CategoryStore, "ReembedAllTracesForce")
	defer timer.Stop()

	if s == nil || s.traceStore == nil {
		return 0, nil
	}

	s.mu.RLock()
	engine := s.embeddingEngine
	s.mu.RUnlock()
	if engine == nil {
		return 0, fmt.Errorf("no embedding engine configured")
	}

	expectedTask := embedding.SelectTaskType(embedding.ContentTypeDocumentation, false)
	expectedModel := engine.Name()
	expectedDim := engine.Dimensions()

	totalEmbedded := 0
	offset := 0
	for {
		candidates, err := s.traceStore.ListAllTraceEmbeddingCandidates(reflectionBatchSize, offset)
		if err != nil {
			return totalEmbedded, err
		}
		if len(candidates) == 0 {
			break
		}

		updates, embedCount, err := s.buildTraceEmbeddingUpdates(ctx, candidates, engine, expectedTask, expectedModel, expectedDim, true)
		if err != nil {
			return totalEmbedded, err
		}
		if len(updates) > 0 {
			if err := s.traceStore.ApplyTraceEmbeddingUpdates(updates); err != nil {
				return totalEmbedded, err
			}
			if err := s.syncTraceVectorIndex(updates, expectedDim); err != nil {
				logging.Get(logging.CategoryStore).Warn("Trace vector index update failed during re-embed: %v", err)
			}
		}
		totalEmbedded += embedCount
		offset += len(candidates)
	}

	return totalEmbedded, nil
}

// ReembedAllLearningsForce regenerates embeddings for all shard learnings.
func (ls *LearningStore) ReembedAllLearningsForce(ctx context.Context) (int, error) {
	timer := logging.StartTimer(logging.CategoryStore, "ReembedAllLearningsForce")
	defer timer.Stop()

	if ls == nil {
		return 0, nil
	}

	ls.mu.RLock()
	engine := ls.embeddingEngine
	ls.mu.RUnlock()
	if engine == nil {
		return 0, fmt.Errorf("no embedding engine configured")
	}

	expectedTask := embedding.SelectTaskType(embedding.ContentTypeKnowledgeAtom, false)
	expectedModel := engine.Name()
	expectedDim := engine.Dimensions()

	totalEmbedded := 0
	shardTypes := ls.listShardTypes()
	for _, shardType := range shardTypes {
		offset := 0
		for {
			candidates, err := ls.ListAllLearningEmbeddingCandidates(shardType, reflectionBatchSize, offset)
			if err != nil {
				return totalEmbedded, err
			}
			if len(candidates) == 0 {
				break
			}

			updates, embedCount, err := ls.buildLearningEmbeddingUpdates(ctx, candidates, engine, expectedTask, expectedModel, expectedDim, true)
			if err != nil {
				return totalEmbedded, err
			}
			if len(updates) > 0 {
				if err := ls.ApplyLearningEmbeddingUpdates(shardType, updates); err != nil {
					return totalEmbedded, err
				}
				db, err := ls.getDB(shardType)
				if err == nil {
					if err := syncLearningVectorIndex(db, updates, expectedDim); err != nil {
						logging.Get(logging.CategoryStore).Warn("Learning vector index update failed during re-embed for %s: %v", shardType, err)
					}
				}
			}

			totalEmbedded += embedCount
			offset += len(candidates)
		}
	}

	return totalEmbedded, nil
}
