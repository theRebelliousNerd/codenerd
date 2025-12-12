package campaign

// AssaultScope controls how targets are grouped for execution.
type AssaultScope string

const (
	AssaultScopeRepo      AssaultScope = "/repo"      // Whole repo as a single target
	AssaultScopeModule    AssaultScope = "/module"    // Coarse directory grouping (e.g., internal, cmd)
	AssaultScopeSubsystem AssaultScope = "/subsystem" // Medium grouping (e.g., internal/core, cmd/nerd)
	AssaultScopePackage   AssaultScope = "/package"   // Individual packages (e.g., codenerd/internal/core)
)

// AssaultStageKind is the kind of work performed for each target.
type AssaultStageKind string

const (
	AssaultStageGoTest        AssaultStageKind = "/go_test"
	AssaultStageGoTestRace    AssaultStageKind = "/go_test_race"
	AssaultStageGoVet         AssaultStageKind = "/go_vet"
	AssaultStageNemesisReview AssaultStageKind = "/nemesis_review"
	AssaultStageCommand       AssaultStageKind = "/command" // Custom command template
)

// AssaultStage defines one per-target action performed during the assault sweep.
type AssaultStage struct {
	Kind AssaultStageKind `json:"kind"`
	Name string          `json:"name"`

	// Command is used only when Kind == /command. It may include "{{target}}".
	Command string `json:"command,omitempty"`

	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
	Repeat         int `json:"repeat,omitempty"`
}

// AssaultConfig configures a long-horizon adversarial assault campaign.
type AssaultConfig struct {
	Scope AssaultScope `json:"scope"`

	// Optional path prefixes (workspace-relative). If provided, only matching targets are included.
	Include []string `json:"include,omitempty"`
	// Optional path prefixes to skip (workspace-relative).
	Exclude []string `json:"exclude,omitempty"`

	BatchSize             int           `json:"batch_size,omitempty"`
	Cycles                int           `json:"cycles,omitempty"`
	DefaultTimeoutSeconds int           `json:"default_timeout_seconds,omitempty"`
	Stages                []AssaultStage `json:"stages,omitempty"`

	// Limit for per-command captured output (also used for on-disk logs).
	LogMaxBytes int64 `json:"log_max_bytes,omitempty"`

	EnableNemesis       bool `json:"enable_nemesis,omitempty"`
	MaxRemediationTasks int  `json:"max_remediation_tasks,omitempty"`
}

func DefaultAssaultConfig() AssaultConfig {
	return AssaultConfig{
		Scope:                AssaultScopeSubsystem,
		BatchSize:            10,
		Cycles:               1,
		DefaultTimeoutSeconds: 900, // 15 minutes
		Stages: []AssaultStage{
			{Kind: AssaultStageGoTest, Name: "go test", Repeat: 1},
			{Kind: AssaultStageNemesisReview, Name: "nemesis review", Repeat: 1},
		},
		LogMaxBytes:          2 * 1024 * 1024, // 2MB
		EnableNemesis:        true,
		MaxRemediationTasks:  25,
	}
}

func (c AssaultConfig) Normalize() AssaultConfig {
	out := c
	if out.Scope == "" {
		out.Scope = DefaultAssaultConfig().Scope
	}
	if out.BatchSize <= 0 {
		out.BatchSize = DefaultAssaultConfig().BatchSize
	}
	if out.Cycles <= 0 {
		out.Cycles = DefaultAssaultConfig().Cycles
	}
	if out.DefaultTimeoutSeconds <= 0 {
		out.DefaultTimeoutSeconds = DefaultAssaultConfig().DefaultTimeoutSeconds
	}
	if out.LogMaxBytes <= 0 {
		out.LogMaxBytes = DefaultAssaultConfig().LogMaxBytes
	}
	if out.MaxRemediationTasks <= 0 {
		out.MaxRemediationTasks = DefaultAssaultConfig().MaxRemediationTasks
	}
	if len(out.Stages) == 0 {
		out.Stages = DefaultAssaultConfig().Stages
	}
	// Ensure stages have sane Repeat/Timeout defaults.
	for i := range out.Stages {
		if out.Stages[i].Repeat <= 0 {
			out.Stages[i].Repeat = 1
		}
		if out.Stages[i].TimeoutSeconds <= 0 {
			out.Stages[i].TimeoutSeconds = out.DefaultTimeoutSeconds
		}
	}
	return out
}

