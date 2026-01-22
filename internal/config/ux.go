package config

// UIConfig holds user interface configuration.
type UIConfig struct {
	// SplitPaneRatio is the default left:right ratio (0.0-1.0, left pane percentage)
	// Default is 0.67 (2/3 chat, 1/3 logic)
	SplitPaneRatio float64 `json:"split_pane_ratio" yaml:"split_pane_ratio"`

	// LogicPaneWidth is an alternative fixed width for the logic pane (0 = use ratio)
	LogicPaneWidth int `json:"logic_pane_width,omitempty" yaml:"logic_pane_width,omitempty"`
}

// DefaultUIConfig returns sensible UI defaults.
func DefaultUIConfig() *UIConfig {
	return &UIConfig{
		SplitPaneRatio: 0.67, // 2/3 chat, 1/3 logic pane
		LogicPaneWidth: 0,    // Use ratio by default
	}
}

// ExperienceLevel tracks user familiarity with codeNERD.
type ExperienceLevel string

const (
	ExperienceBeginner     ExperienceLevel = "beginner"     // First-time or new users
	ExperienceIntermediate ExperienceLevel = "intermediate" // Familiar with basics
	ExperienceAdvanced     ExperienceLevel = "advanced"     // Power users
	ExperienceExpert       ExperienceLevel = "expert"       // Full access, minimal guidance
)

// OnboardingState tracks user onboarding progress.
type OnboardingState struct {
	// SetupComplete indicates if the user has completed initial setup
	SetupComplete bool `json:"setup_complete"`

	// ExperienceLevel affects how much guidance is shown
	ExperienceLevel ExperienceLevel `json:"experience_level,omitempty"`

	// FirstRunAt records when the user first used codeNERD
	FirstRunAt string `json:"first_run_at,omitempty"`

	// Milestones tracks completed onboarding milestones
	Milestones []string `json:"milestones,omitempty"`

	// CommandsUsed tracks command usage for progressive unlock
	CommandsUsed map[string]int `json:"commands_used,omitempty"`

	// ShowTips enables/disables contextual tips
	ShowTips bool `json:"show_tips"`

	// TourStep tracks current position in the guided tour
	TourStep int `json:"tour_step,omitempty"`

	// TourComplete indicates if the guided tour is finished
	TourComplete bool `json:"tour_complete,omitempty"`
}

// TransparencyConfig controls visibility into internal operations.
type TransparencyConfig struct {
	// Enabled is the master toggle for transparency features
	Enabled bool `json:"enabled"`

	// ShardPhases shows which phase each shard is in during execution
	ShardPhases bool `json:"shard_phases"`

	// StreamReasoning streams LLM reasoning in real-time (verbose)
	StreamReasoning bool `json:"stream_reasoning"`

	// SafetyExplanations shows why constitutional gates blocked actions
	SafetyExplanations bool `json:"safety_explanations"`

	// JITExplain shows JIT atom selection in /jit explain
	JITExplain bool `json:"jit_explain"`

	// OperationSummaries shows post-operation summaries
	OperationSummaries bool `json:"operation_summaries"`

	// VerboseErrors shows categorized errors with remediation
	VerboseErrors bool `json:"verbose_errors"`

	// Glass Box Debug Mode - shows system internals inline in chat
	GlassBoxEnabled bool `json:"glass_box_enabled"`

	// GlassBoxCategories filters which event categories are shown
	// Valid values: "perception", "kernel", "jit", "shard", "control"
	// Empty means all categories are shown
	GlassBoxCategories []string `json:"glass_box_categories,omitempty"`

	// GlassBoxVerbose shows expanded details instead of summaries
	GlassBoxVerbose bool `json:"glass_box_verbose"`
}

// GuidanceLevel controls how much help/guidance is shown.
type GuidanceLevel string

const (
	GuidanceVerbose GuidanceLevel = "verbose" // Maximum guidance (learning users)
	GuidanceNormal  GuidanceLevel = "normal"  // Standard guidance
	GuidanceMinimal GuidanceLevel = "minimal" // Minimal guidance (productive users)
	GuidanceNone    GuidanceLevel = "none"    // No guidance (power users)
)

// GuidanceConfig controls contextual help and tips.
type GuidanceConfig struct {
	// Level controls overall guidance verbosity
	Level GuidanceLevel `json:"level,omitempty"`

	// ShowHints enables inline hints after commands
	ShowHints bool `json:"show_hints"`

	// ShowWhyExplanations enables automatic explanations
	ShowWhyExplanations bool `json:"show_why_explanations"`

	// AutoSuggestHelp triggers help when user struggles
	AutoSuggestHelp bool `json:"auto_suggest_help"`
}

// DefaultOnboardingState returns sensible defaults for new users.
func DefaultOnboardingState() *OnboardingState {
	return &OnboardingState{
		SetupComplete:   false,
		ExperienceLevel: ExperienceBeginner,
		ShowTips:        true,
		CommandsUsed:    make(map[string]int),
	}
}

// DefaultTransparencyConfig returns sensible defaults.
func DefaultTransparencyConfig() *TransparencyConfig {
	return &TransparencyConfig{
		Enabled:            false,
		ShardPhases:        true,
		StreamReasoning:    false,
		SafetyExplanations: true,
		JITExplain:         false,
		OperationSummaries: true,
		VerboseErrors:      true,
	}
}

// DefaultGuidanceConfig returns sensible defaults.
func DefaultGuidanceConfig() *GuidanceConfig {
	return &GuidanceConfig{
		Level:               GuidanceNormal,
		ShowHints:           true,
		ShowWhyExplanations: true,
		AutoSuggestHelp:     true,
	}
}
