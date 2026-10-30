package campaign

import "errors"

var (
	// ErrDecompositionFailed indicates plan decomposition could not produce a valid campaign.
	ErrDecompositionFailed = errors.New("campaign decomposition failed")

	// ErrCheckpointFailed indicates a checkpoint gate failed.
	ErrCheckpointFailed = errors.New("campaign checkpoint failed")

	// ErrReplanExhausted indicates replanning could not recover campaign progress.
	ErrReplanExhausted = errors.New("campaign replan exhausted")

	// ErrNilDependency indicates required orchestrator dependencies are missing.
	ErrNilDependency = errors.New("campaign missing required dependency")

	// ErrInvalidConfig indicates invalid orchestrator configuration values.
	ErrInvalidConfig = errors.New("campaign invalid configuration")

	// ErrNilCampaign indicates a required campaign pointer was nil.
	ErrNilCampaign = errors.New("campaign is nil")

	// ErrNilKernel indicates a required kernel dependency was nil.
	ErrNilKernel = errors.New("campaign kernel is nil")

	// ErrEmptyRequirement indicates a requirement string was empty after trimming.
	ErrEmptyRequirement = errors.New("campaign requirement is empty")

	// ErrEmptyGoal indicates a decomposition goal was empty after trimming.
	ErrEmptyGoal = errors.New("campaign goal is empty")
)
