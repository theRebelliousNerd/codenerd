package campaign

import "errors"

var (
	// ErrDecompositionFailed indicates plan decomposition could not produce a valid campaign.
	ErrDecompositionFailed = errors.New("campaign decomposition failed")

	// ErrTaskTimeout indicates an individual campaign task exceeded its timeout.
	ErrTaskTimeout = errors.New("campaign task timeout")

	// ErrCampaignTimeout indicates the campaign exceeded overall runtime limits.
	ErrCampaignTimeout = errors.New("campaign timeout")

	// ErrCheckpointFailed indicates a checkpoint gate failed.
	ErrCheckpointFailed = errors.New("campaign checkpoint failed")

	// ErrReplanExhausted indicates replanning could not recover campaign progress.
	ErrReplanExhausted = errors.New("campaign replan exhausted")

	// ErrNilDependency indicates required orchestrator dependencies are missing.
	ErrNilDependency = errors.New("campaign missing required dependency")

	// ErrInvalidConfig indicates invalid orchestrator configuration values.
	ErrInvalidConfig = errors.New("campaign invalid configuration")
)
