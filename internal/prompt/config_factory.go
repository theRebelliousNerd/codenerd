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

	// Code DOM tools for semantic code operations
	codeDomTools := []string{
		"get_elements",
		"get_element",
		"edit_lines",
		"insert_lines",
		"delete_lines",
	}

	// Test impact analysis tools
	testImpactTools := []string{
		"get_impacted_tests",
		"run_impacted_tests",
	}

	// Helper to copy slice and avoid aliasing
	copyTools := func(base []string, more ...string) []string {
		result := make([]string, 0, len(base)+len(more))
		result = append(result, base...)
		result = append(result, more...)
		return result
	}

	// Coder persona tools
	coderTools := copyTools(coreTools,
		"write_file",
		"edit_file",
		"delete_file",
		"run_build",
		"git_operation",
		"run_command",
		"bash",
	)
	coderTools = append(coderTools, codeDomTools...)
	coderTools = append(coderTools, testImpactTools...)

	// Tester persona tools
	testerTools := copyTools(coreTools,
		"run_tests",
		"run_command",
		"bash",
		"write_file", // Can write test files
		"edit_file",
	)
	testerTools = append(testerTools, codeDomTools...)
	testerTools = append(testerTools, testImpactTools...)

	// Reviewer persona tools (read-heavy, includes Code DOM for inspection)
	reviewerTools := copyTools(coreTools,
		"git_diff",
		"git_log",
		"run_command", // For running static analysis tools
	)
	reviewerTools = append(reviewerTools, codeDomTools...)

	// Researcher persona tools
	researcherTools := copyTools(coreTools,
		"context7_fetch", // LLM-optimized documentation
		"web_search",
		"web_fetch",
		"browser_navigate",
		"browser_extract",
		"research_cache_get",
		"research_cache_set",
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

	// Nemesis/adversarial intents (attack persona)
	nemesisTools := copyTools(coreTools,
		"run_command", // For running attack programs
		"bash",
		"run_build",
		"run_tests",
		"write_file", // For writing attack code
	)
	nemesisTools = append(nemesisTools, codeDomTools...)
	for _, intent := range []string{"/attack", "/break", "/exploit", "/fuzz", "/pentest", "/nemesis"} {
		provider.atoms[intent] = ConfigAtom{
			Tools:    nemesisTools,
			Priority: 85, // Higher priority than reviewer
		}
	}

	// Tool generator intents
	toolGenTools := copyTools(coreTools,
		"write_file",
		"run_build",
		"run_tests",
		"run_command",
	)
	for _, intent := range []string{"/generate", "/generate-tool", "/tool_generator", "/create_tool"} {
		provider.atoms[intent] = ConfigAtom{
			Tools:    toolGenTools,
			Priority: 75,
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
