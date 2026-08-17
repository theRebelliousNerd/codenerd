package prompt

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"codenerd/internal/core"
	"codenerd/internal/jit/config"
	"codenerd/internal/logging"
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

// TODO: [User Request Extremes] Optimize slice capacity allocation to prevent OOM when `input` slice is extremely large, rather than allocating a slice matching `len(input)` unconditionally.
func uniqueStrings(input []string) []string {
	const MaxItems = 1000 // Prevent massive DoS

	keys := make(map[string]bool)
	list := make([]string, 0, len(input)) // pre-allocate capacity

	for _, entry := range input {
		if len(list) >= MaxItems {
			break
		}
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry) // copies the string reference, safe
		}
	}
	return list
}

// Clone creates a deep copy of the ConfigAtom to prevent data races.
func (c ConfigAtom) Clone() ConfigAtom {
	tools := make([]string, len(c.Tools))
	copy(tools, c.Tools)

	policies := make([]string, len(c.Policies))
	copy(policies, c.Policies)

	return ConfigAtom{
		Tools:    tools,
		Policies: policies,
		Priority: c.Priority,
	}
}

// ConfigAtomProvider defines the interface for retrieving config atoms.
type ConfigAtomProvider interface {
	GetAtom(intent string) (ConfigAtom, bool)
}

// ConfigFactory generates EffectiveAgentRuntimeConfig objects.
type ConfigFactory struct {
	provider ConfigAtomProvider
}

// NewConfigFactory creates a new ConfigFactory.
func NewConfigFactory(provider ConfigAtomProvider) *ConfigFactory {
	return &ConfigFactory{
		provider: provider,
	}
}

// Generate creates an EffectiveAgentRuntimeConfig based on the intents and compilation result.
// It merges config atoms for all provided intents.
// TODO: [Null/Undefined/Empty] Missing panic prevention when f.provider is nil.
func (f *ConfigFactory) Generate(ctx context.Context, result *CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error) {
	if result == nil {
		return nil, fmt.Errorf("compilation result cannot be nil")
	}
	if len(intents) == 0 {
		return nil, fmt.Errorf("no intents provided")
	}
	if f.provider == nil {
		return nil, fmt.Errorf("config provider cannot be nil")
	}
	var finalAtom ConfigAtom
	found := false

	for _, rawIntent := range intents {
		intent := strings.TrimSpace(rawIntent)
		if atom, ok := f.provider.GetAtom(intent); ok {
			finalAtom = finalAtom.Merge(atom)
			found = true
			continue
		}
		// An unregistered intent falls back to /general so the agent still gets
		// a read-only tool set rather than running with zero capability.
		//
		// This used to apply only to "/consult/<persona>" specialists, so every
		// OTHER unregistered verb produced AllowedTools == nil. That is a worse
		// failure than it looks: the caller logs a WARN, keeps the empty config,
		// and proceeds to answer from an empty tool catalog -- the agent has no
		// way to read a file and no way to say so. Degrade loudly, don't
		// silently disarm.
		if atom, ok := f.provider.GetAtom("/general"); ok {
			logging.Get(logging.CategoryContext).Warn(
				"No config atom for intent %q; falling back to /general read-only tools. "+
					"Canonical verbs belong in NewDefaultConfigAtomProvider.", intent)
			finalAtom = finalAtom.Merge(atom)
			found = true
		}
	}

	if !found {
		return nil, fmt.Errorf("no config atoms found for intents: %v", intents)
	}

	// Determine primary intent for the config
	primaryIntent := intents[0]

	cfg := &config.EffectiveAgentRuntimeConfig{
		IdentityPrompt: result.Prompt,
		IntentVerb:     primaryIntent,
		AllowedTools:   finalAtom.Tools,
		Policies:       finalAtom.Policies,
		ToolLoop: config.ToolLoopConfig{
			MaxIterations:   5,
			MaxTotalCalls:   50,
			FailOnToolError: false,
		},
		Safety: config.SafetyConfig{
			RequirePolicyEnforcement: true,
		},
	}

	return cfg, nil
}

// GenerateFallback creates a minimal config for when JIT compilation fails.
func (f *ConfigFactory) GenerateFallback(ctx context.Context, intent string, fallbackIdentity string) *config.EffectiveAgentRuntimeConfig {
	// Prevent OOM from massive fallback strings
	const MaxFallbackLength = 1024 * 1024 // 1MB limit
	if len(fallbackIdentity) > MaxFallbackLength {
		// TODO: [Type Coercion] Truncating by bytes can slice a multibyte UTF-8 character in half, resulting in invalid UTF-8. It should truncate on rune boundaries.
		fallbackIdentity = fallbackIdentity[:MaxFallbackLength]
	}

	intent = strings.TrimSpace(intent)
	var finalAtom ConfigAtom
	if f.provider != nil {
		if atom, ok := f.provider.GetAtom(intent); ok {
			finalAtom = atom
		} else if atom, ok := f.provider.GetAtom("/general"); ok {
			finalAtom = atom
		}
	}

	return &config.EffectiveAgentRuntimeConfig{
		IdentityPrompt: fallbackIdentity,
		IntentVerb:     intent,
		AllowedTools:   finalAtom.Tools,
		Policies:       finalAtom.Policies,
		ToolLoop: config.ToolLoopConfig{
			MaxIterations:   5,
			MaxTotalCalls:   50,
			FailOnToolError: false,
		},
		Safety: config.SafetyConfig{
			RequirePolicyEnforcement: true,
		},
	}
}

// =============================================================================
// DEFAULT CONFIG ATOM PROVIDER
// =============================================================================
// Provides built-in config atoms for common intents. This maps intent verbs
// to allowed tools and policies.

// DefaultConfigAtomProvider provides built-in config atoms.
type DefaultConfigAtomProvider struct {
	atoms map[string]ConfigAtom
	mu    sync.RWMutex
}

func mustDefaultPolicySet(setID string) []string {
	files, ok := core.DefaultAgentPolicySetFiles(setID)
	if !ok {
		panic(fmt.Sprintf("unknown default agent policy set %q", setID))
	}
	return files
}

func copyPolicySet(setID string) []string {
	return append([]string(nil), mustDefaultPolicySet(setID)...)
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
		"browser_observe",
		"browser_act",
		"browser_mangle",
		"browser_wait",
		"browser_reason",
		"browser_evidence",
		"browser_specs",
		"browser_test",
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
		"grounded_web_search",
		"web_fetch",
		"browser_navigate",
		"browser_extract",
		"browser_observe",
		"browser_act",
		"browser_mangle",
		"browser_wait",
		"browser_reason",
		"browser_evidence",
		"browser_specs",
		"browser_test",
		"research_cache_get",
		"research_cache_set",
		"write_file", // Can write documentation
	)

	verificationTools := copyTools(testerTools, "grounded_web_search")

	// The intent lists below must cover every verb in
	// perception.DefaultTaxonomyData, and each verb belongs to the persona that
	// taxonomy declares as its ShardType. TestConfigAtoms_EveryTaxonomyVerbHasTools
	// pins that; read it before editing these lists.
	//
	// They drifted badly once. The taxonomy grew to 36 verbs while this file
	// still listed 9 of them plus a dozen synonyms that were never canonical
	// ("/implement", "/check", "/find", ...), leaving 27 verbs with NO config
	// atom. An unregistered verb resolved to zero AllowedTools, which made
	// buildToolCatalogForPiggyback emit nothing and buildToolDefinitions log
	// "no tools configured" -- so `nerd explain <file>` could not read the file
	// it was asked to explain. It answered "reading the file now..." and exited
	// 0. /explain is the highest-volume verb in the product.
	//
	// Non-canonical aliases are kept: they cost nothing and the perception layer
	// has historically emitted some of them.

	// Register coder intents
	for _, intent := range []string{
		"/fix", "/refactor", "/create", "/write", "/delete", "/debug",
		"/campaign", "/git", "/migrate", "/optimize", "/document",
		"/scaffold", "/format",
		// non-canonical aliases
		"/implement", "/modify", "/add", "/update",
	} {
		provider.atoms[intent] = ConfigAtom{
			Tools:    coderTools,
			Policies: copyPolicySet(core.PolicySetCoder),
			Priority: 100,
		}
	}

	// Register tester intents (ordinary)
	for _, intent := range []string{
		"/test", "/benchmark", "/profile",
		// non-canonical aliases
		"/cover",
	} {
		provider.atoms[intent] = ConfigAtom{
			Tools:    testerTools,
			Policies: copyPolicySet(core.PolicySetTester),
			Priority: 90,
		}
	}

	// Register verification intents (tester + grounded search)
	for _, intent := range []string{
		"/verify", "/validate",
	} {
		provider.atoms[intent] = ConfigAtom{
			Tools:    verificationTools,
			Policies: copyPolicySet(core.PolicySetTester),
			Priority: 90,
		}
	}

	// Register reviewer intents
	for _, intent := range []string{
		"/review", "/review_enhance", "/security", "/analyze", "/audit", "/lint",
		// non-canonical aliases
		"/check", "/inspect",
	} {
		provider.atoms[intent] = ConfigAtom{
			Tools:    reviewerTools,
			Policies: copyPolicySet(core.PolicySetReviewer),
			Priority: 80,
		}
	}

	// Register researcher intents
	for _, intent := range []string{
		"/explore", "/search", "/research", "/init",
		// non-canonical aliases
		"/learn", "/understand", "/find",
	} {
		provider.atoms[intent] = ConfigAtom{
			Tools:    researcherTools,
			Policies: copyPolicySet(core.PolicySetResearcher),
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
			Policies: copyPolicySet(core.PolicySetNemesis),
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
	for _, intent := range []string{
		"/generate_tool",
		// non-canonical aliases
		"/generate", "/generate-tool", "/tool_generator", "/create_tool",
	} {
		provider.atoms[intent] = ConfigAtom{
			Tools:    toolGenTools,
			Policies: copyPolicySet(core.PolicySetToolGenerator),
			Priority: 75,
		}
	}

	// General/fallback intent
	provider.atoms["/general"] = ConfigAtom{
		Tools:    coreTools,
		Policies: copyPolicySet(core.PolicySetBase),
		Priority: 50,
	}

	// Taxonomy verbs whose declared ShardType is /none. They route no persona,
	// but "no persona" is not "no hands" -- /explain and /read exist to describe
	// files, and answering from the filename alone is exactly the hallucination
	// the constitutional prompt forbids. Core tools are read-only, so this grants
	// the ability to look without the ability to change anything.
	for _, intent := range []string{
		"/explain", "/read", "/stats", "/knowledge", "/help", "/greet",
		"/configure", "/dream", "/shadow", "/assault",
	} {
		provider.atoms[intent] = ConfigAtom{
			Tools:    coreTools,
			Policies: copyPolicySet(core.PolicySetBase),
			Priority: 50,
		}
	}

	return provider
}

// GetAtom returns the config atom for an intent.
func (p *DefaultConfigAtomProvider) GetAtom(intent string) (ConfigAtom, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	atom, ok := p.atoms[intent]
	if ok {
		return atom.Clone(), true
	}
	return atom, false
}

// RegisterAtom adds or updates a config atom for an intent.
func (p *DefaultConfigAtomProvider) RegisterAtom(intent string, atom ConfigAtom) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.atoms[intent] = atom.Clone()
}

// NewDefaultConfigFactory creates a ConfigFactory with the default provider.
func NewDefaultConfigFactory() *ConfigFactory {
	return NewConfigFactory(NewDefaultConfigAtomProvider())
}
