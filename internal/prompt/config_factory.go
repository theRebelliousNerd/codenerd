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

// =============================================================================
// DEFAULT CONFIG ATOM PROVIDER
// =============================================================================
// Provides built-in config atoms for common intents. This maps intent verbs
// to allowed tools and policies.

// DefaultConfigAtomProvider provides built-in config atoms.
type DefaultConfigAtomProvider struct {
	atoms map[string]ConfigAtom
}

// NewDefaultConfigAtomProvider creates a new default config provider.
func NewDefaultConfigAtomProvider() *DefaultConfigAtomProvider {
	provider := &DefaultConfigAtomProvider{
		atoms: make(map[string]ConfigAtom),
	}

	// Core tools available to all personas
	coreTools := []string{
		"read_file",
		"search_code",
		"list_files",
		"glob",
		"grep",
	}

	// Coder persona tools
	coderTools := append(coreTools,
		"write_file",
		"edit_file",
		"create_file",
		"run_build",
		"git_operation",
		"run_command",
	)

	// Tester persona tools
	testerTools := append(coreTools,
		"run_tests",
		"coverage_report",
		"write_file", // Can write test files
	)

	// Reviewer persona tools (read-heavy)
	reviewerTools := append(coreTools,
		"git_diff",
		"git_log",
		"security_scan",
	)

	// Researcher persona tools
	researcherTools := append(coreTools,
		"web_search",
		"web_fetch",
		"write_file", // Can write documentation
	)

	// Register coder intents
	for _, intent := range []string{"/fix", "/implement", "/refactor", "/create", "/modify", "/add", "/update"} {
		provider.atoms[intent] = ConfigAtom{
			Tools:    coderTools,
			Priority: 100,
		}
	}

	// Register tester intents
	for _, intent := range []string{"/test", "/cover", "/verify", "/validate"} {
		provider.atoms[intent] = ConfigAtom{
			Tools:    testerTools,
			Priority: 90,
		}
	}

	// Register reviewer intents
	for _, intent := range []string{"/review", "/audit", "/check", "/analyze", "/inspect"} {
		provider.atoms[intent] = ConfigAtom{
			Tools:    reviewerTools,
			Priority: 80,
		}
	}

	// Register researcher intents
	for _, intent := range []string{"/research", "/learn", "/document", "/understand", "/explore", "/find"} {
		provider.atoms[intent] = ConfigAtom{
			Tools:    researcherTools,
			Priority: 70,
		}
	}

	// General/fallback intent
	provider.atoms["/general"] = ConfigAtom{
		Tools:    coreTools,
		Priority: 50,
	}

	return provider
}

// GetAtom returns the config atom for an intent.
func (p *DefaultConfigAtomProvider) GetAtom(intent string) (ConfigAtom, bool) {
	atom, ok := p.atoms[intent]
	return atom, ok
}

// RegisterAtom adds or updates a config atom for an intent.
func (p *DefaultConfigAtomProvider) RegisterAtom(intent string, atom ConfigAtom) {
	p.atoms[intent] = atom
}

// NewDefaultConfigFactory creates a ConfigFactory with the default provider.
func NewDefaultConfigFactory() *ConfigFactory {
	return NewConfigFactory(NewDefaultConfigAtomProvider())
}
