package config

import (
	"fmt"
	"strings"
)

// AgentConfig defines the configuration for a JIT-driven dynamic agent.
// This struct maps the output of the JIT compiler to the Universal Executor.
type AgentConfig struct {
	// IdentityPrompt is the system prompt that defines the agent's persona and mission.
	IdentityPrompt string `json:"identity_prompt"`

	// Tools defines the set of tools this agent is permitted to use.
	Tools ToolSet `json:"tools"`

	// Policies defines the Mangle logic files that govern this agent's behavior.
	Policies PolicySet `json:"policies"`

	// Mode defines the execution mode (e.g., "SingleTurn", "Campaign").
	Mode string `json:"mode,omitempty"`
}

// ToolSet represents a collection of allowed tools.
type ToolSet struct {
	AllowedTools []string `json:"allowed_tools"`
}

// PolicySet represents a collection of Mangle policy files.
type PolicySet struct {
	Files []string `json:"files"`
}

// Validate ensures the configuration is complete and usable.
func (c AgentConfig) Validate() error {
	if strings.TrimSpace(c.IdentityPrompt) == "" {
		return fmt.Errorf("agent config validation failed: identity_prompt is required")
	}

	if len(c.Policies.Files) == 0 {
		return fmt.Errorf("agent config validation failed: at least one policy file is required")
	}

	return nil
}
