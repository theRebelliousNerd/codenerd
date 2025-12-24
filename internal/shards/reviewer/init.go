// Package reviewer implements the Reviewer ShardAgent per §7.0 Sharding.
// This file contains constructor functions.
package reviewer

import (
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/types"
)

// =============================================================================
// CONSTRUCTORS
// =============================================================================

// NewReviewerShard creates a new Reviewer shard with default configuration.
func NewReviewerShard() *ReviewerShard {
	return NewReviewerShardWithConfig(DefaultReviewerConfig())
}

// NewReviewerShardWithConfig creates a reviewer shard with custom configuration.
func NewReviewerShardWithConfig(reviewerConfig ReviewerConfig) *ReviewerShard {
	shard := &ReviewerShard{
		config:              coreshards.DefaultSpecialistConfig("reviewer", ""),
		state:               types.ShardStateIdle,
		reviewerConfig:      reviewerConfig,
		findings:            make([]ReviewFinding, 0),
		severity:            ReviewSeverityClean,
		customRules:         make([]CustomRule, 0),
		approvedPatterns:    make(map[string]int),
		flaggedPatterns:     make(map[string]int),
		learnedAntiPatterns: make(map[string]string),
	}

	// Attempt to load custom rules if path is configured
	if reviewerConfig.CustomRulesPath != "" {
		_ = shard.LoadCustomRules(reviewerConfig.CustomRulesPath)
	}

	return shard
}
