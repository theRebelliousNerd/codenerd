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

// Generate creates an AgentConfig based on the intent and compilation result.
func (f *ConfigFactory) Generate(ctx context.Context, intent string, result *CompilationResult) (*config.AgentConfig, error) {
	atom, ok := f.provider.GetAtom(intent)
	if !ok {
		return nil, fmt.Errorf("no config atom found for intent: %s", intent)
	}

	cfg := &config.AgentConfig{
		IdentityPrompt: result.Prompt,
		Tools: config.ToolSet{
			AllowedTools: atom.Tools,
		},
		Policies: config.PolicySet{
			Files: atom.Policies,
		},
	}

	return cfg, nil
}
