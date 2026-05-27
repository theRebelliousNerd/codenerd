package config

import (
	"fmt"
	"strings"
)

// EffectiveAgentRuntimeConfig defines the configuration for a JIT-driven dynamic agent.
// This struct maps the output of the JIT compiler to the Universal Executor.
type EffectiveAgentRuntimeConfig struct {
	IdentityPrompt string
	IntentVerb     string
	Persona        string
	AllowedTools   []string
	Policies       []string
	Model          string
	ToolLoop       ToolLoopConfig
	Safety         SafetyConfig
	Workspace      WorkspaceConfig
}

type ToolLoopConfig struct {
	MaxIterations   int
	MaxTotalCalls   int
	FailOnToolError bool
}

type SafetyConfig struct {
	RequirePolicyEnforcement bool
}

type WorkspaceConfig struct {
	RootPath string
}

// Validate ensures the configuration is complete and usable.
func (c EffectiveAgentRuntimeConfig) Validate() error {
	if strings.TrimSpace(c.IdentityPrompt) == "" {
		return fmt.Errorf("config validation failed: identity_prompt is required")
	}
	return nil
}
