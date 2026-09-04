package config

import (
	"codenerd/internal/core"
	"fmt"
	"strings"
)

// EffectiveAgentRuntimeConfig defines the configuration for a JIT-driven dynamic agent.
// This struct maps the output of the JIT compiler to the Universal Executor.
//
// YAML tags use snake_case so specialist config files at
// .nerd/agents/<name>/config.yaml can use the natural YAML convention.
type EffectiveAgentRuntimeConfig struct {
	IdentityPrompt string   `yaml:"identity_prompt" json:"identity_prompt"`
	IntentVerb     string   `yaml:"intent_verb" json:"intent_verb"`
	Persona        string   `yaml:"persona" json:"persona"`
	AllowedTools   []string `yaml:"allowed_tools" json:"allowed_tools"`
	Policies       []string `yaml:"policies" json:"policies"`
}

// Validate ensures the configuration is complete and usable.
//
// A valid config MUST have:
//   - A non-empty IdentityPrompt (after trimming whitespace). Without an
//     identity prompt the runtime has no persona to ground the LLM.
//   - At least one canonical embedded entry in Policies. Policies anchor the
//     Mangle kernel's executive layer; aliases, traversal, missing modules, and
//     duplicates are rejected against core's live policy inventory.
//
// AllowedTools and Persona are intentionally NOT validated here. They have
// safe zero values: an empty allowlist fails closed at execution, and Persona
// is descriptive metadata resolved from the intent verb.
func (c EffectiveAgentRuntimeConfig) Validate() error {
	if strings.TrimSpace(c.IdentityPrompt) == "" {
		return fmt.Errorf("config validation failed: identity_prompt is required")
	}
	if len(c.Policies) == 0 {
		return fmt.Errorf("config validation failed: at least one policy file is required")
	}
	seenPolicies := make(map[string]struct{}, len(c.Policies))
	for _, policy := range c.Policies {
		if !core.IsDefaultPolicyFile(policy) {
			return fmt.Errorf("config validation failed: policy %q is not a canonical embedded policy reference", policy)
		}
		if _, duplicate := seenPolicies[policy]; duplicate {
			return fmt.Errorf("config validation failed: duplicate policy reference %q", policy)
		}
		seenPolicies[policy] = struct{}{}
	}
	return nil
}
