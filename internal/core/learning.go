package core

// ShardLearning represents a learned pattern or preference.
type ShardLearning struct {
	FactPredicate string  `json:"fact_predicate"`
	FactArgs      []any   `json:"fact_args"`
	Confidence    float64 `json:"confidence"`
}

// LearningStore defines the interface for persisting shard learnings.
type LearningStore interface {
	Save(shardType, factPredicate string, factArgs []any, sourceCampaign string) error
	Load(shardType string) ([]ShardLearning, error)
	LoadByPredicate(shardType, predicate string) ([]ShardLearning, error)
	DecayConfidence(shardType string, decayFactor float64) error
	Close() error
}
