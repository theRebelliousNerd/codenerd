package config

import (
	"fmt"
	"strings"
)

// EffectiveAgentRuntimeConfig defines the configuration for a JIT-driven dynamic agent.
// This struct maps the output of the JIT compiler to the Universal Executor.
//
// YAML tags use snake_case so specialist config files at
// .nerd/agents/<name>/config.yaml can use the natural YAML convention.
type EffectiveAgentRuntimeConfig struct {
	IdentityPrompt string           `yaml:"identity_prompt" json:"identity_prompt"`
	IntentVerb     string           `yaml:"intent_verb" json:"intent_verb"`
	Persona        string           `yaml:"persona" json:"persona"`
	AllowedTools   []string         `yaml:"allowed_tools" json:"allowed_tools"`
	Policies       []string         `yaml:"policies" json:"policies"`
	Model          string           `yaml:"model" json:"model"`
	ToolLoop       ToolLoopConfig   `yaml:"tool_loop" json:"tool_loop"`
	Safety         SafetyConfig     `yaml:"safety" json:"safety"`
	Workspace      WorkspaceConfig  `yaml:"workspace" json:"workspace"`
}

type ToolLoopConfig struct {
	MaxIterations   int  `yaml:"max_iterations" json:"max_iterations"`
	MaxTotalCalls   int  `yaml:"max_total_calls" json:"max_total_calls"`
	FailOnToolError bool `yaml:"fail_on_tool_error" json:"fail_on_tool_error"`
}

type SafetyConfig struct {
	RequirePolicyEnforcement bool `yaml:"require_policy_enforcement" json:"require_policy_enforcement"`
}

type WorkspaceConfig struct {
	RootPath string `yaml:"root_path" json:"root_path"`
}

// Validate ensures the configuration is complete and usable.
//
// A valid config MUST have:
//   - A non-empty IdentityPrompt (after trimming whitespace). Without an
//     identity prompt the runtime has no persona to ground the LLM.
//   - At least one entry in Policies. Policies anchor the Mangle kernel's
//     executive layer; an agent with zero policy files has no constitutional
//     safety net and is rejected by the JIT compiler.
//
// AllowedTools, Model, ToolLoop, Safety, and Workspace are intentionally
// NOT validated here. They have safe zero values or are populated by
// downstream layers (e.g. session executor defaults, ConfigFactory).
func (c EffectiveAgentRuntimeConfig) Validate() error {
	if strings.TrimSpace(c.IdentityPrompt) == "" {
		return fmt.Errorf("config validation failed: identity_prompt is required")
	}
	if len(c.Policies) == 0 {
		return fmt.Errorf("config validation failed: at least one policy file is required")
	}
	return nil
}
