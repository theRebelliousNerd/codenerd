package chat

import (
	"codenerd/internal/store"
	"codenerd/internal/types"
)

// =============================================================================
// LEARNING STORE ADAPTER
// =============================================================================
// Adapts store.LearningStore to implement core.LearningStore interface for
// shard autopoiesis. Used by the live boot in session_shared_boot.go.

// coreLearningStoreAdapter wraps store.LearningStore to implement core.LearningStore.
type coreLearningStoreAdapter struct {
	store *store.LearningStore
}

func (a *coreLearningStoreAdapter) Save(shardType, factPredicate string, factArgs []any, sourceCampaign string) error {
	if a.store == nil {
		return nil
	}
	return a.store.Save(shardType, factPredicate, factArgs, sourceCampaign)
}

func (a *coreLearningStoreAdapter) SaveBatch(shardType string, learnings []types.ShardLearning, sourceCampaign string) error {
	if a.store == nil {
		return nil
	}
	return a.store.SaveBatch(shardType, learnings, sourceCampaign)
}

func (a *coreLearningStoreAdapter) Load(shardType string) ([]types.ShardLearning, error) {
	if a.store == nil {
		return nil, nil
	}
	// store.LearningStore.Load already returns []types.ShardLearning
	return a.store.Load(shardType)
}

func (a *coreLearningStoreAdapter) LoadByPredicate(shardType, predicate string) ([]types.ShardLearning, error) {
	if a.store == nil {
		return nil, nil
	}
	// store.LearningStore.LoadByPredicate already returns []types.ShardLearning
	return a.store.LoadByPredicate(shardType, predicate)
}

func (a *coreLearningStoreAdapter) DecayConfidence(shardType string, decayFactor float64) error {
	if a.store == nil {
		return nil
	}
	return a.store.DecayConfidence(shardType, decayFactor)
}

func (a *coreLearningStoreAdapter) Close() error {
	if a.store == nil {
		return nil
	}
	return a.store.Close()
}