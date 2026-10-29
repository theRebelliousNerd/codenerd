package core

import (
	"codenerd/internal/types"
)

// LearningStore defines the interface for persisting shard learnings.
type LearningStore interface {
	Save(shardType, factPredicate string, factArgs []any, sourceCampaign string) error
	SaveBatch(shardType string, learnings []types.ShardLearning, sourceCampaign string) error
	Load(shardType string) ([]types.ShardLearning, error)
	LoadByPredicate(shardType, predicate string) ([]types.ShardLearning, error)
	DecayConfidence(shardType string, decayFactor float64) error
	Close() error
}
