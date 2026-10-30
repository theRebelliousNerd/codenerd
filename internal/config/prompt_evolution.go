package config

import "fmt"

// PromptEvolutionConfig is the `prompt_evolution` section of
// .nerd/config.json: the bounds the System Prompt Learning loop
// (internal/autopoiesis/prompt_evolution) spends and retains within.
//
// Whether automatic cycles run at all is the prompt_evolution feature flag,
// not this section. These knobs bound what a cycle may buy, how much
// recorded work may wait for persistence, and how a reviewed trial of a
// quarantined candidate is limited.
type PromptEvolutionConfig struct {
	// RecordQueueCapacity is how many finished turns may wait for the
	// recording worker. A turn arriving at a full queue is rejected and
	// counted, never silently dropped and never an unbounded goroutine.
	RecordQueueCapacity int `json:"record_queue_capacity,omitempty"`

	// RecordDrainTimeoutSeconds bounds how long Close waits for accepted
	// records to persist before abandoning the remainder (reported as an
	// incomplete shutdown, with the abandoned count).
	RecordDrainTimeoutSeconds int `json:"record_drain_timeout_seconds,omitempty"`

	// MaxCycleJudgeCalls caps LLM-as-Judge calls in one cycle.
	MaxCycleJudgeCalls int `json:"max_cycle_judge_calls,omitempty"`

	// MaxCycleScreeningCalls caps logical shared-cause calls, including failures.
	MaxCycleScreeningCalls int `json:"max_cycle_screening_calls,omitempty"`

	// EvidenceCohortSize bounds selected evidence, within the screening protocol.
	EvidenceCohortSize int `json:"evidence_cohort_size,omitempty"`

	// MaxCycleGenerationCalls caps atom-generation and strategy-refinement
	// calls in one cycle, across every failure group.
	MaxCycleGenerationCalls int `json:"max_cycle_generation_calls,omitempty"`

	// MaxCycleAtoms caps candidate atoms stored by one cycle, across every
	// failure group. The per-group cap (EvolverConfig.MaxAtomsPerEvolution)
	// still applies inside it.
	MaxCycleAtoms int `json:"max_cycle_atoms,omitempty"`

	// ClaimLeaseSeconds is how long a generation claim stays owned by the
	// worker that took it. A claim older than this may be taken over (with a
	// new fence) by another cycle or process.
	ClaimLeaseSeconds int `json:"claim_lease_seconds,omitempty"`

	// TrialMaxExposures is the default exposure budget of an approved trial:
	// how many recorded turns may carry the candidate before admission stops.
	TrialMaxExposures int `json:"trial_max_exposures,omitempty"`

	// TrialTTLHours is the default lifetime of an approved trial.
	TrialTTLHours int `json:"trial_ttl_hours,omitempty"`
}

// DefaultPromptEvolutionConfig is the section with every field written down.
// MaxCycleJudgeCalls matches the twenty unevaluated records a cycle has
// always judged; MaxCycleGenerationCalls and MaxCycleAtoms make the old
// per-group cap of three a per-cycle cap as well.
func DefaultPromptEvolutionConfig() PromptEvolutionConfig {
	return PromptEvolutionConfig{
		RecordQueueCapacity:       256,
		RecordDrainTimeoutSeconds: 5,
		MaxCycleJudgeCalls:        20,
		MaxCycleScreeningCalls:    3,
		EvidenceCohortSize:        12,
		MaxCycleGenerationCalls:   3,
		MaxCycleAtoms:             3,
		ClaimLeaseSeconds:         600,
		TrialMaxExposures:         20,
		TrialTTLHours:             72,
	}
}

// GetPromptEvolutionConfig returns the section with every absent field
// defaulted. A nil receiver is the defaults.
func (c *UserConfig) GetPromptEvolutionConfig() PromptEvolutionConfig {
	if c == nil || c.PromptEvolution == nil {
		return DefaultPromptEvolutionConfig()
	}
	return c.PromptEvolution.WithDefaults()
}

// WithDefaults fills every absent (zero) field from the defaults. Every knob
// is a positive bound, so zero is never a meaningful explicit value.
func (c PromptEvolutionConfig) WithDefaults() PromptEvolutionConfig {
	d := DefaultPromptEvolutionConfig()
	fill := func(v *int, def int) {
		if *v == 0 {
			*v = def
		}
	}
	fill(&c.RecordQueueCapacity, d.RecordQueueCapacity)
	fill(&c.RecordDrainTimeoutSeconds, d.RecordDrainTimeoutSeconds)
	fill(&c.MaxCycleJudgeCalls, d.MaxCycleJudgeCalls)
	fill(&c.MaxCycleScreeningCalls, d.MaxCycleScreeningCalls)
	fill(&c.EvidenceCohortSize, d.EvidenceCohortSize)
	fill(&c.MaxCycleGenerationCalls, d.MaxCycleGenerationCalls)
	fill(&c.MaxCycleAtoms, d.MaxCycleAtoms)
	fill(&c.ClaimLeaseSeconds, d.ClaimLeaseSeconds)
	fill(&c.TrialMaxExposures, d.TrialMaxExposures)
	fill(&c.TrialTTLHours, d.TrialTTLHours)
	return c
}

// Check reports the contradictions in a prompt_evolution section, addressed
// under prefix ("prompt_evolution").
func (c PromptEvolutionConfig) Check(prefix string) []Problem {
	c = c.WithDefaults()
	var out []Problem
	positive := func(key string, v int) {
		if v < 1 {
			out = append(out, Problem{
				Severity: SeverityError,
				Path:     prefix + "." + key,
				Message:  fmt.Sprintf("%d is not a positive bound", v),
				Fix:      "a value of 1 or more, or remove the key for the default",
			})
		}
	}
	positive("record_queue_capacity", c.RecordQueueCapacity)
	positive("record_drain_timeout_seconds", c.RecordDrainTimeoutSeconds)
	positive("max_cycle_judge_calls", c.MaxCycleJudgeCalls)
	positive("max_cycle_screening_calls", c.MaxCycleScreeningCalls)
	positive("evidence_cohort_size", c.EvidenceCohortSize)
	positive("max_cycle_generation_calls", c.MaxCycleGenerationCalls)
	positive("max_cycle_atoms", c.MaxCycleAtoms)
	positive("claim_lease_seconds", c.ClaimLeaseSeconds)
	positive("trial_max_exposures", c.TrialMaxExposures)
	positive("trial_ttl_hours", c.TrialTTLHours)
	return out
}
