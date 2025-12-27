package prompt

import (
	"context"
	"fmt"

	"codenerd/internal/jit/config"
)

// ConfigAtom represents a configuration fragment associated with an intent.
type ConfigAtom struct {
	Tools    []string
	Policies []string
	Priority int
}

// Merge combines two ConfigAtoms.
// Tools and Policies are merged and deduplicated.
// The higher priority is kept.
func (c ConfigAtom) Merge(other ConfigAtom) ConfigAtom {
	merged := ConfigAtom{
		Tools:    uniqueStrings(append(c.Tools, other.Tools...)),
		Policies: uniqueStrings(append(c.Policies, other.Policies...)),
		Priority: c.Priority,
	}

	if other.Priority > c.Priority {
		merged.Priority = other.Priority
	}

	return merged
}

func uniqueStrings(input []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range input {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

// ConfigAtomProvider defines the interface for retrieving config atoms.
type ConfigAtomProvider interface {
	GetAtom(intent string) (ConfigAtom, bool)
}

// ConfigFactory generates AgentConfig objects.
type ConfigFactory struct {
	provider ConfigAtomProvider
}

// NewConfigFactory creates a new ConfigFactory.
func NewConfigFactory(provider ConfigAtomProvider) *ConfigFactory {
	return &ConfigFactory{
		provider: provider,
	}
}

// Generate creates an AgentConfig based on the intents and compilation result.
// It merges config atoms for all provided intents.
func (f *ConfigFactory) Generate(ctx context.Context, result *CompilationResult, intents ...string) (*config.AgentConfig, error) {
	var finalAtom ConfigAtom
	found := false

	for _, intent := range intents {
		if atom, ok := f.provider.GetAtom(intent); ok {
			finalAtom = finalAtom.Merge(atom)
			found = true
		}
	}

	if !found {
		return nil, fmt.Errorf("no config atoms found for intents: %v", intents)
	}

	cfg := &config.AgentConfig{
		IdentityPrompt: result.Prompt,
		Tools: config.ToolSet{
			AllowedTools: finalAtom.Tools,
		},
		Policies: config.PolicySet{
			Files: finalAtom.Policies,
		},
	}

	return cfg, nil
}
