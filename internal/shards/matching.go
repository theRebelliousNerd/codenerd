// Package shards provides the shard-agent (specialist) classification
// shared by chat delegation, advisors and observers.
//
// Which shard agent a task needs is not decided here: it is expert_needed,
// derived by the kernel (internal/core/defaults/policy/shard_experts.mg) and
// read through internal/experts. A Go table of file-name, import and content
// substrings (CoreTechnologyPatterns) used to make that choice on two chat
// paths only; it is gone.
package shards

import (
	"strings"
)

// SpecialistMatch is a shard agent the kernel derived a delegated task needs,
// in the shape chat delegation spawns and reports it.
type SpecialistMatch struct {
	AgentName      string                    // Agent identity (lower-cased registry name)
	Files          []string                  // Task files the agent's domain covers
	Score          float64                   // expert_confidence / 100
	Reason         string                    // The technologies that tie the task to the agent
	Classification *SpecialistClassification // Classification info (executor/advisor/observer)
	ShouldExecute  bool                      // specialist_should_execute derived for this task
}

// =============================================================================
// SPECIALIST CLASSIFICATION SYSTEM
// =============================================================================

// SpecialistExecutionMode defines the execution capability of a specialist
type SpecialistExecutionMode string

const (
	// SpecialistModeExecutor - Can directly write/modify code in their domain
	SpecialistModeExecutor SpecialistExecutionMode = "/executor"

	// SpecialistModeAdvisor - Can only advise, not execute
	SpecialistModeAdvisor SpecialistExecutionMode = "/advisor"

	// SpecialistModeObserver - Background monitoring only
	SpecialistModeObserver SpecialistExecutionMode = "/observer"
)

// SpecialistKnowledgeTier defines the type of knowledge a specialist has
type SpecialistKnowledgeTier string

const (
	// TierTechnical - Implementation expertise (how to code)
	TierTechnical SpecialistKnowledgeTier = "/technical"

	// TierStrategic - Architectural/philosophical guidance (what to code)
	TierStrategic SpecialistKnowledgeTier = "/strategic"

	// TierDomain - Project-specific knowledge (why we code this way)
	TierDomain SpecialistKnowledgeTier = "/domain"
)

// SpecialistClassification holds the classification of a specialist
type SpecialistClassification struct {
	ExecutionMode       SpecialistExecutionMode
	KnowledgeTier       SpecialistKnowledgeTier
	CanExecute          bool
	CanAdvise           bool
	CanObserve          bool
	BackgroundCapable   bool
	CampaignIntegration string // /phase_executor, /plan_reviewer, /background_monitor
}

// DefaultSpecialistClassifications maps specialist names to their classifications
// These are parsed from the classification atoms in .nerd/agents/*/prompts.yaml
var DefaultSpecialistClassifications = map[string]SpecialistClassification{
	"goexpert": {
		ExecutionMode:       SpecialistModeExecutor,
		KnowledgeTier:       TierTechnical,
		CanExecute:          true,
		CanAdvise:           true,
		CanObserve:          true,
		BackgroundCapable:   true,
		CampaignIntegration: "/phase_executor",
	},
	"bubbleteaexpert": {
		ExecutionMode:       SpecialistModeExecutor,
		KnowledgeTier:       TierTechnical,
		CanExecute:          true,
		CanAdvise:           true,
		CanObserve:          true,
		BackgroundCapable:   true,
		CampaignIntegration: "/phase_executor",
	},
	"cobraexpert": {
		ExecutionMode:       SpecialistModeExecutor,
		KnowledgeTier:       TierTechnical,
		CanExecute:          true,
		CanAdvise:           true,
		CanObserve:          true,
		BackgroundCapable:   true,
		CampaignIntegration: "/phase_executor",
	},
	"rodexpert": {
		ExecutionMode:       SpecialistModeExecutor,
		KnowledgeTier:       TierTechnical,
		CanExecute:          true,
		CanAdvise:           true,
		CanObserve:          true,
		BackgroundCapable:   true,
		CampaignIntegration: "/phase_executor",
	},
	"mangleexpert": {
		ExecutionMode:       SpecialistModeExecutor,
		KnowledgeTier:       TierTechnical,
		CanExecute:          true,
		CanAdvise:           true,
		CanObserve:          true,
		BackgroundCapable:   true,
		CampaignIntegration: "/phase_executor",
	},
	"securityauditor": {
		ExecutionMode:       SpecialistModeAdvisor,
		KnowledgeTier:       TierStrategic,
		CanExecute:          false,
		CanAdvise:           true,
		CanObserve:          true,
		BackgroundCapable:   true,
		CampaignIntegration: "/plan_reviewer",
	},
	"testarchitect": {
		ExecutionMode:       SpecialistModeAdvisor,
		KnowledgeTier:       TierStrategic,
		CanExecute:          false,
		CanAdvise:           true,
		CanObserve:          true,
		BackgroundCapable:   true,
		CampaignIntegration: "/plan_reviewer",
	},
	"northstar": {
		ExecutionMode:       SpecialistModeObserver,
		KnowledgeTier:       TierStrategic,
		CanExecute:          false,
		CanAdvise:           true,
		CanObserve:          true,
		BackgroundCapable:   true,
		CampaignIntegration: "/alignment_guardian",
	},
}

// GetSpecialistClassification returns the classification for a specialist
func GetSpecialistClassification(name string) (SpecialistClassification, bool) {
	// Normalize name to lowercase
	lowerName := strings.ToLower(strings.TrimSpace(name))
	class, ok := DefaultSpecialistClassifications[lowerName]
	return class, ok
}
