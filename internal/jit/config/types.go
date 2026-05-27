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
